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
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	freqtradev1beta1 "github.com/ark-sys/freqtrade-operator/api/v1beta1"
	"github.com/ark-sys/freqtrade-operator/test/utils"
)

// frequiGatewayContext covers G6-2's own acceptance criteria (GATEWAY-API-PLAN.md): a
// Gateway-mode FreqUI produces the expected set of HTTPRoutes with the expected hostnames and
// backends, dropping a TradeBotRef prunes that bot's own route, and switching back to Ingress
// mode deletes the HTTPRoutes and creates the Ingress instead. This deliberately does NOT
// install a full Gateway implementation (Envoy Gateway, Istio) to assert real traffic - that
// would be a large, slow, flaky addition on top of an already-slow suite - so
// status.conditions[ExposureReady]/Accepted=True is never asserted here; controllers/frequi's
// own envtest suite (G2-2/G4-1) already covers that condition's derivation logic directly. Free
// of any network dependency beyond the kind cluster and the CRD installs done in
// e2e_suite_test.go's BeforeSuite - that matters: the TradeBot dry-run e2e spec depends on a real
// exchange over the network (some, like Binance and Bybit, block CI runners; see e2eExchange in
// fixtures_test.go), and this spec must not repeat that mistake.
func frequiGatewayContext() {
	Context("FreqUI spec.exposure: Gateway (G6-2)", Ordered, func() {
		const (
			frequiName = "e2e-gw-frequi"
			strategy   = "e2e-gw-strategy"
			config     = "e2e-gw-config"
			exchangeSA = "e2e-gw-exchange-creds"
			bot1Name   = "e2e-gw-bot-1"
			bot2Name   = "e2e-gw-bot-2"
			gwHost     = "gw-e2e.example.com"
		)
		ctx := context.Background()
		key := types.NamespacedName{Name: frequiName, Namespace: tradingNamespace}
		bot1RouteKey := types.NamespacedName{Name: frequiName + "-" + bot1Name, Namespace: tradingNamespace}
		bot2RouteKey := types.NamespacedName{Name: frequiName + "-" + bot2Name, Namespace: tradingNamespace}

		BeforeAll(func() {
			// Reuses tradingNamespace if another top-level Ordered Context already created it -
			// top-level Ordered containers are not ordered relative to each other (same pattern as
			// backtest_test.go's own BeforeAll).
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

			By("creating the exchange Secret, Strategy, TradeBotConfig, and two TradeBots")
			Expect(k8sClient.Create(ctx, newExchangeSecret(exchangeSA))).To(Succeed())
			Expect(k8sClient.Create(ctx, newStrategy(strategy))).To(Succeed())
			Expect(k8sClient.Create(ctx, newDryRunTradeBotConfig(config, exchangeSA))).To(Succeed())
			Expect(k8sClient.Create(ctx, newTradeBot(bot1Name, config, strategy))).To(Succeed())
			Expect(k8sClient.Create(ctx, newTradeBot(bot2Name, config, strategy))).To(Succeed())

			// Only TradeBotConfig's own validation needs to complete here - reaching Valid needs
			// no live network call, unlike the bots' own pods actually starting (which this spec
			// never waits on: HTTPRoute reconciliation only depends on TradeBotConfig's Phase and
			// spec.apiServer.enabled, not on the bot pod being Ready).
			By("waiting for the TradeBotConfig to become Valid")
			Eventually(func(g Gomega) {
				var cfg freqtradev1alpha1.TradeBotConfig
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: config, Namespace: tradingNamespace}, &cfg)).To(Succeed())
				g.Expect(cfg.Status.Phase).To(Equal("Valid"))
			}, "2m").Should(Succeed())

			By("creating a Gateway-mode FreqUI referencing both bots")
			frequi := &freqtradev1beta1.FreqUI{
				ObjectMeta: metav1.ObjectMeta{Name: frequiName, Namespace: tradingNamespace},
				Spec: freqtradev1beta1.FreqUISpec{
					Host:     gwHost,
					Exposure: freqtradev1beta1.FUExposureGateway,
					Gateway: &freqtradev1beta1.FUGatewaySpec{
						ParentRefs: []gatewayv1.ParentReference{{Name: "e2e-nonexistent-gateway"}},
					},
					TradeBotRefs: []corev1.LocalObjectReference{{Name: bot1Name}, {Name: bot2Name}},
				},
			}
			Expect(k8sClient.Create(ctx, frequi)).To(Succeed())
		})

		AfterAll(func() {
			// tradingNamespace itself is cleaned up centrally by e2e_test.go's own AfterAll.
			frequi := &freqtradev1beta1.FreqUI{ObjectMeta: metav1.ObjectMeta{Name: frequiName, Namespace: tradingNamespace}}
			_ = k8sClient.Delete(context.Background(), frequi)
		})

		It("produces one HTTPRoute for the UI and one per eligible TradeBot, with expected hostnames/backends", func() {
			Eventually(func(g Gomega) {
				var ui gatewayv1.HTTPRoute
				g.Expect(k8sClient.Get(ctx, key, &ui)).To(Succeed())
				g.Expect(ui.Spec.Hostnames).To(ConsistOf(gatewayv1.Hostname(gwHost)))
				g.Expect(ui.Spec.Rules).To(HaveLen(1))
				g.Expect(string(ui.Spec.Rules[0].BackendRefs[0].Name)).To(Equal(frequiName))

				var bot1Route gatewayv1.HTTPRoute
				g.Expect(k8sClient.Get(ctx, bot1RouteKey, &bot1Route)).To(Succeed())
				g.Expect(bot1Route.Spec.Hostnames).To(ConsistOf(gatewayv1.Hostname(bot1Name + "." + gwHost)))
				g.Expect(string(bot1Route.Spec.Rules[0].BackendRefs[0].Name)).To(Equal(bot1Name))

				var bot2Route gatewayv1.HTTPRoute
				g.Expect(k8sClient.Get(ctx, bot2RouteKey, &bot2Route)).To(Succeed())
				g.Expect(bot2Route.Spec.Hostnames).To(ConsistOf(gatewayv1.Hostname(bot2Name + "." + gwHost)))
			}, "2m").Should(Succeed())
		})

		It("prunes a dropped bot's HTTPRoute while the UI route and the other bot's survive", func() {
			Eventually(func() error {
				var frequi freqtradev1beta1.FreqUI
				if err := k8sClient.Get(ctx, key, &frequi); err != nil {
					return err
				}
				frequi.Spec.TradeBotRefs = []corev1.LocalObjectReference{{Name: bot1Name}}
				return k8sClient.Update(ctx, &frequi)
			}, "1m").Should(Succeed())

			Eventually(func() bool {
				var route gatewayv1.HTTPRoute
				return errors.IsNotFound(k8sClient.Get(ctx, bot2RouteKey, &route))
			}, "2m").Should(BeTrue(), "expected bot2's HTTPRoute to be pruned")

			var ui gatewayv1.HTTPRoute
			Expect(k8sClient.Get(ctx, key, &ui)).To(Succeed())
			var bot1Route gatewayv1.HTTPRoute
			Expect(k8sClient.Get(ctx, bot1RouteKey, &bot1Route)).To(Succeed())
		})

		It("switches to Ingress: deletes the HTTPRoutes, creates the Ingress", func() {
			Eventually(func() error {
				var frequi freqtradev1beta1.FreqUI
				if err := k8sClient.Get(ctx, key, &frequi); err != nil {
					return err
				}
				frequi.Spec.Exposure = freqtradev1beta1.FUExposureIngress
				frequi.Spec.Gateway = nil
				return k8sClient.Update(ctx, &frequi)
			}, "1m").Should(Succeed())

			Eventually(func() error {
				var ingress networkingv1.Ingress
				return k8sClient.Get(ctx, key, &ingress)
			}, "2m").Should(Succeed())

			Eventually(func() bool {
				var route gatewayv1.HTTPRoute
				return errors.IsNotFound(k8sClient.Get(ctx, key, &route))
			}, "2m").Should(BeTrue(), "expected the UI HTTPRoute to be pruned once exposure flips to Ingress")
			Eventually(func() bool {
				var route gatewayv1.HTTPRoute
				return errors.IsNotFound(k8sClient.Get(ctx, bot1RouteKey, &route))
			}, "2m").Should(BeTrue(), "expected bot1's HTTPRoute to be pruned once exposure flips to Ingress")
		})
	})
}
