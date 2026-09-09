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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// webhookRejectionContext exercises P5-3's "webhook rejection cases" against the real,
// cert-manager-issued webhook server this cluster deployed - a genuinely different code path
// from envtest's own webhook wiring (real CA injection, real TLS termination, not just
// envtest.CRDInstallOptions' automatic conversion patching), even though the underlying
// validation logic already has unit/envtest coverage of its own.
func webhookRejectionContext() {
	Context("Admission and conversion webhooks reject invalid resources", func() {
		ctx := context.Background()

		It("rejects a TradeBotConfig with plaintext exchange credentials and no opt-out annotation", func() {
			cfg := &freqtradev1alpha1.TradeBotConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "e2e-plaintext-creds-rejected", Namespace: tradingNamespace},
				Spec: freqtradev1alpha1.TradeBotConfigSpec{
					Bot: &freqtradev1alpha1.BotConfig{StakeCurrency: "USDT", StakeAmount: "100"},
					Exchange: &freqtradev1alpha1.ExchangeSpec{
						Name:   "binance",
						Key:    "plaintext-key-should-be-rejected",
						Secret: "plaintext-secret-should-be-rejected",
					},
				},
			}
			err := k8sClient.Create(ctx, cfg)
			Expect(err).To(HaveOccurred(), "expected the admission webhook to reject plaintext credentials")
			Expect(err.Error()).To(ContainSubstring("allow-plaintext-credentials"))
		})

		It("rejects a v1alpha1 TradeBot with a Job-mode freqtrade_command (v1beta1 is trade-only)", func() {
			const (
				strategy   = "e2e-jobmode-strategy"
				config     = "e2e-jobmode-config"
				exchangeSA = "e2e-jobmode-exchange-creds"
			)
			// The validating webhook (vtradebot.kb.io) checks spec.strategy/spec.config
			// existence before this conversion-webhook check is ever reached (see
			// TradeBotCustomValidator.validate) - both refs need to resolve to something real
			// first, or Create fails on that unrelated check instead of the one this It means to
			// exercise. Verified directly: a first pass using placeholder ref names failed on
			// exactly this, with a "Strategy ... not found" message instead of "trade-only".
			Expect(k8sClient.Create(ctx, newExchangeSecret(exchangeSA))).To(Succeed())
			Expect(k8sClient.Create(ctx, newStrategy(strategy))).To(Succeed())
			Expect(k8sClient.Create(ctx, newDryRunTradeBotConfig(config, exchangeSA))).To(Succeed())

			tb := &freqtradev1alpha1.TradeBot{
				ObjectMeta: metav1.ObjectMeta{Name: "e2e-jobmode-rejected", Namespace: tradingNamespace},
				Spec: freqtradev1alpha1.TradeBotSpec{
					Config:           config,
					Strategy:         strategy,
					FreqtradeCommand: "backtesting",
				},
			}
			// v1beta1 is the storage version (P6-5): the apiserver must convert this v1alpha1
			// object to v1beta1 to store it, and v1beta1 has no Job-mode representation at all -
			// api/v1alpha1/tradebot_conversion.go's ConvertTo rejects it, so this fails at Create,
			// not at some later read-back.
			err := k8sClient.Create(ctx, tb)
			Expect(err).To(HaveOccurred(), "expected the conversion webhook to reject a Job-mode TradeBot")
			Expect(err.Error()).To(ContainSubstring("trade-only"))
		})
	})
}
