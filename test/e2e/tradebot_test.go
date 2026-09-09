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

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	"github.com/ark-sys/freqtrade-operator/test/utils"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// tradingNamespace holds every trading-resource spec's objects (TradeBot, TradeBotConfig,
// Strategy, Backtest, FreqUI, the exchange Secret) - deliberately separate from the operator's
// own namespace, matching the README's own recommended one-namespace-per-trading-environment
// pattern (D3), and labeled restricted PSA (P3-3, P5-3) so every workload built in it - the bot's
// own StatefulSet pod included, not just the manager - is proven to actually start under it.
const tradingNamespace = "freqtrade-e2e-trading"

// tradeBotDryRunContext covers P5-3's live-bot acceptance criteria: pod reaches Ready,
// /api/v1/ping-equivalent connectivity (via the operator's own introspection succeeding - a
// stronger check than a redundant manual ping, since it exercises the real production poll path),
// and FreqUI reaching the bot through the NetworkPolicy.
func tradeBotDryRunContext() {
	Context("TradeBot dry-run against a sandboxed exchange", Ordered, func() {
		const (
			botName    = "e2e-dryrun-bot"
			strategy   = "e2e-dryrun-strategy"
			config     = "e2e-dryrun-config"
			exchangeSA = "e2e-exchange-creds"
			frequiName = "e2e-frequi"
		)
		ctx := context.Background()

		BeforeAll(func() {
			By("creating the trading namespace, labeled restricted PSA")
			cmd := exec.Command("kubectl", "create", "ns", tradingNamespace)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create trading namespace")
			cmd = exec.Command("kubectl", "label", "--overwrite", "ns", tradingNamespace,
				"pod-security.kubernetes.io/enforce=restricted")
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to label trading namespace")

			By("creating the exchange Secret, Strategy, TradeBotConfig, TradeBot, and FreqUI")
			Expect(k8sClient.Create(ctx, newExchangeSecret(exchangeSA))).To(Succeed())
			Expect(k8sClient.Create(ctx, newStrategy(strategy))).To(Succeed())
			Expect(k8sClient.Create(ctx, newDryRunTradeBotConfig(config, exchangeSA))).To(Succeed())
			Expect(k8sClient.Create(ctx, newTradeBot(botName, tradingNamespace, config, strategy))).To(Succeed())
			Expect(k8sClient.Create(ctx, newFreqUI(frequiName, tradingNamespace, []string{botName}))).To(Succeed())
		})

		// tradingNamespace is cleaned up once, centrally, in the outer Describe's own AfterAll
		// (e2e_test.go) - see its comment for why that's better than doing it here.

		It("brings the bot's pod up under a restricted Pod Security Standard", func() {
			By("waiting for the StatefulSet pod to reach Ready")
			verifyPodReady := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "pod", botName+"-0",
					"-n", tradingNamespace, "-o", "jsonpath={.status.containerStatuses[0].ready}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("true"), "bot pod not Ready yet")
			}
			Eventually(verifyPodReady, "3m").Should(Succeed())
		})

		It("introspects the bot successfully (proves live REST API connectivity)", func() {
			By("waiting for TradeBot.status.bot to populate and BotReachable=True")
			var tb freqtradev1alpha1.TradeBot
			verifyIntrospected := func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: botName, Namespace: tradingNamespace}, &tb)).To(Succeed())
				g.Expect(tb.Status.Bot).NotTo(BeNil(), "status.bot not populated yet")
				reachable := meta.FindStatusCondition(tb.Status.Conditions, "BotReachable")
				g.Expect(reachable).NotTo(BeNil())
				g.Expect(reachable.Status).To(
					Equal(metav1.ConditionTrue), "BotReachable=%v: %s", reachable.Status, reachable.Message)
			}
			Eventually(verifyIntrospected, "3m").Should(Succeed())
			Expect(tb.Status.Bot.DryRun).NotTo(BeNil())
			Expect(*tb.Status.Bot.DryRun).To(BeTrue())
		})

		It("lets a referencing FreqUI reach the bot through the NetworkPolicy", func() {
			By("waiting for the FreqUI Deployment to be available")
			cmd := exec.Command("kubectl", "wait", "--for=condition=available", "--timeout=2m",
				"deployment/"+frequiName, "-n", tradingNamespace)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "FreqUI deployment did not become available")

			By("probing the bot's API from a pod carrying FreqUI's own NetworkPolicy identity (app label)")
			// A real exec into the frequi pod would depend on that image shipping a shell/curl,
			// which isn't guaranteed - this pod is given the exact label
			// (controllers/tradebot/resources/networkpolicy.go's allow-rule: podSelector
			// app=<frequi-name>) the NetworkPolicy actually keys off, so it is treated
			// identically by the policy without depending on frequi's image contents.
			// tradingNamespace enforces restricted PSA (same as the operator's own namespace).
			// curlimages/curl already defaults to a non-root user (curl_user, uid 100 - verified
			// directly: docker run curlimages/curl:latest id), but runAsUser still has to be given
			// explicitly and numerically: the kubelet won't infer non-root-ness from a *named*
			// image user, even a genuinely non-root one - verified directly, a first pass with only
			// runAsNonRoot:true (no runAsUser) failed every container with "image has non-numeric
			// user (curl_user), cannot verify user is non-root", a CreateContainerConfigError, not
			// a PodSecurity admission rejection (that part alone was already satisfied).
			probePod := "e2e-frequi-identity-probe"
			cmd = exec.Command("kubectl", "run", probePod, "--restart=Never",
				"-n", tradingNamespace,
				"--labels=app="+frequiName,
				"--image=curlimages/curl:latest",
				"--overrides", fmt.Sprintf(`{
					"spec": {
						"containers": [{
							"name": "%s",
							"image": "curlimages/curl:latest",
							"command": ["curl", "-sf", "-m", "5", "-o", "/dev/null",
								"http://%s.%s.svc.cluster.local:8080/api/v1/ping"],
							"securityContext": {
								"allowPrivilegeEscalation": false,
								"capabilities": {"drop": ["ALL"]},
								"runAsNonRoot": true,
								"runAsUser": 100,
								"seccompProfile": {"type": "RuntimeDefault"}
							}
						}]
					}
				}`, probePod, botName, tradingNamespace),
			)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create the FreqUI-identity probe pod")

			verifyProbeSucceeded := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "pod", probePod, "-n", tradingNamespace,
					"-o", "jsonpath={.status.phase}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Succeeded"), "curl (as FreqUI) could not reach the bot's API")
			}
			Eventually(verifyProbeSucceeded, "1m").Should(Succeed())

			cmd = exec.Command("kubectl", "delete", "pod", probePod, "-n", tradingNamespace, "--ignore-not-found")
			_, _ = utils.Run(cmd)
		})
	})
}
