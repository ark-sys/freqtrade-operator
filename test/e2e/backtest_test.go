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
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	freqtradev1beta1 "github.com/ark-sys/freqtrade-operator/api/v1beta1"
	"github.com/ark-sys/freqtrade-operator/test/utils"
	"k8s.io/apimachinery/pkg/types"
)

// backtestContext covers P5-3's "run a Backtest to completion" acceptance criterion: results
// land on the PVC - verified directly, by mounting it read-only from a throwaway probe pod, not
// just inferred from status.resultsPVCName being non-empty - and the extracted summary appears
// in status.results.
func backtestContext() {
	Context("Backtest runs to completion", Ordered, func() {
		const (
			name     = "e2e-backtest"
			strategy = "e2e-backtest-strategy"
			config   = "e2e-backtest-config"
			cachePVC = "e2e-backtest-cache"
			// Backtest.spec.exchange has no secretRef field of its own (it reuses the
			// TradeBotConfig referenced by configRef) - this Secret is the same shape as the
			// TradeBot dry-run test's, just under its own name to keep the two Contexts
			// independent (Ginkgo does not guarantee their relative ordering).
			exchangeSA = "e2e-backtest-exchange-creds"
		)
		ctx := context.Background()

		BeforeAll(func() {
			// Reuses tradingNamespace (created by tradeBotDryRunContext's own BeforeAll) if that
			// Context already ran; creates it otherwise - Ordered containers at the top level are
			// not ordered relative to each other, so this Context cannot assume it ran second.
			cmd := exec.Command("kubectl", "get", "ns", tradingNamespace)
			if _, err := utils.Run(cmd); err != nil {
				cmd = exec.Command("kubectl", "create", "ns", tradingNamespace)
				_, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred(), "Failed to create trading namespace")
				cmd = exec.Command("kubectl", "label", "--overwrite", "ns", tradingNamespace,
					"pod-security.kubernetes.io/enforce=restricted")
				_, err = utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred(), "Failed to label trading namespace")
			}

			By("creating the exchange Secret, Strategy, TradeBotConfig, data cache PVC, and Backtest")
			Expect(k8sClient.Create(ctx, newExchangeSecret(exchangeSA))).To(Succeed())
			Expect(k8sClient.Create(ctx, newStrategy(strategy))).To(Succeed())
			Expect(k8sClient.Create(ctx, newDryRunTradeBotConfig(config, exchangeSA))).To(Succeed())
			Expect(k8sClient.Create(ctx, newDataCachePVC(cachePVC, tradingNamespace))).To(Succeed())
			Expect(k8sClient.Create(ctx, newBacktest(name, tradingNamespace, config, strategy, cachePVC))).To(Succeed())
		})

		It("reaches Succeeded with results extracted", func() {
			var bt freqtradev1beta1.Backtest
			verifySucceeded := func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: tradingNamespace}, &bt)).To(Succeed())
				g.Expect(bt.Status.Phase).To(Equal("Succeeded"), "backtest status.message: %s", bt.Status.Message)
			}
			// Downloading real market data plus running the backtest genuinely takes longer than
			// the other specs' polls - a 3-day window timed out at the full 5m budget on a real
			// run, so recentTimerange() was cut to 1 day *and* the budget widened here, rather than
			// relying on either alone.
			Eventually(verifySucceeded, "8m").Should(Succeed())

			By("checking the results PVC was actually written to, not just referenced")
			Expect(bt.Status.ResultsPVCName).NotTo(BeEmpty())

			By("mounting the results PVC read-only from a throwaway pod and checking it's non-empty")
			probePod := "e2e-backtest-results-probe"
			cmd := exec.Command("kubectl", "run", probePod, "--restart=Never",
				"-n", tradingNamespace,
				"--image=busybox:latest",
				"--overrides", fmt.Sprintf(`{
					"spec": {
						"containers": [{
							"name": "%s",
							"image": "busybox:latest",
							"command": ["sh", "-c", "ls -A /results | grep -q ."],
							"volumeMounts": [{"name": "results", "mountPath": "/results", "readOnly": true}],
							"securityContext": {
								"allowPrivilegeEscalation": false,
								"capabilities": {"drop": ["ALL"]},
								"runAsNonRoot": true,
								"runAsUser": 1000,
								"seccompProfile": {"type": "RuntimeDefault"}
							}
						}],
						"securityContext": {"runAsUser": 1000, "runAsGroup": 1000, "fsGroup": 1000},
						"volumes": [{"name": "results", "persistentVolumeClaim": {"claimName": "%s"}}]
					}
				}`, probePod, bt.Status.ResultsPVCName),
			)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create the results-PVC probe pod")

			verifyProbeSucceeded := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "pod", probePod, "-n", tradingNamespace,
					"-o", "jsonpath={.status.phase}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Succeeded"), "results PVC was empty or unreadable")
			}
			Eventually(verifyProbeSucceeded, "1m").Should(Succeed())

			By("checking the extracted summary landed in status.results")
			Expect(bt.Status.Results).NotTo(BeNil())
			Expect(bt.Status.Results.TotalTrades).To(BeNumerically(">=", 0))
		})
	})
}
