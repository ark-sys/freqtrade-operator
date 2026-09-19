/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package e2e

import (
	"context"
	"fmt"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// exchangeReachable is set by exchangeReachabilityContext and read by skipUnlessExchangeReachable.
// Top-level Ordered containers inside the "Manager" Describe run in definition order (it is Ordered
// itself), so this always runs before any of the exchange-dependent Contexts.
var exchangeReachable bool

// exchangeReachabilityContext exists because freqtrade's failure mode on an unreachable exchange is
// deceptive: it logs "Could not load markets, therefore cannot start" but the process stays alive
// with its API port closed, so the pod just looks slow. Without this, a blocked exchange (see
// e2eExchange for the history - Binance 451, Bybit 403 from GitHub-hosted runners) costs every
// exchange-dependent Context its full multi-minute timeout and buries the cause in probe failures.
// This fails once, in seconds, naming the HTTP status, and lets those Contexts skip rather than
// each time out. It checks from the machine running the suite, not from a pod: measured on a
// GitHub-hosted runner, host and in-cluster results were identical for every endpoint tried.
func exchangeReachabilityContext() {
	Context("Exchange reachability", func() {
		It("can reach the exchange's public API (required by every dry-run bot and Backtest)", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, e2eExchangeProbeURL, http.NoBody)
			Expect(err).NotTo(HaveOccurred())

			resp, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred(), fmt.Sprintf(
				"could not reach %s at all (DNS/network egress): every spec that needs a live exchange "+
					"will be skipped", e2eExchangeProbeURL))
			defer func() { _ = resp.Body.Close() }()

			// 403/451 are how exchanges geo-block datacenter/US ranges. Say so - it is the failure
			// this spec exists for, and "status 451" alone sends the reader hunting through the cluster.
			Expect(resp.StatusCode).To(Equal(http.StatusOK), fmt.Sprintf(
				"%s returned HTTP %d. 403/451 means the exchange is geo/provider-blocking this network "+
					"(GitHub-hosted runners are US Azure IPs) - pick an exchange that allows it, see "+
					"e2eExchange in fixtures_test.go. Every spec that needs a live exchange will be skipped.",
				e2eExchangeProbeURL, resp.StatusCode))
			exchangeReachable = true
		})
	})
}

// skipUnlessExchangeReachable is called at the top of the BeforeAll of every Context that starts a
// freqtrade process, since freqtrade cannot start without loading the exchange's markets.
func skipUnlessExchangeReachable() {
	if !exchangeReachable {
		Skip("exchange unreachable from this network - see the failed 'Exchange reachability' spec")
	}
}
