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
	freqtradev1beta1 "github.com/ark-sys/freqtrade-operator/api/v1beta1"
	"github.com/ark-sys/freqtrade-operator/test/utils"
	"k8s.io/apimachinery/pkg/types"
)

// upgradeContext covers P5-3's upgrade acceptance criterion as closely as this repo's actual
// state allows: there is no prior tagged release to install as "N" and upgrade from, so this
// tests what an upgrade actually depends on - that a TradeBot created against one running
// manager keeps its workload running, and stays readable through conversion as both API
// versions, across the manager Deployment being redeployed out from under it - rather than
// swapping between two genuinely different operator versions.
func upgradeContext() {
	Context("A TradeBot survives an operator redeploy, still converting correctly", Ordered, func() {
		const (
			botName    = "e2e-upgrade-bot"
			strategy   = "e2e-upgrade-strategy"
			config     = "e2e-upgrade-config"
			exchangeSA = "e2e-upgrade-exchange-creds"
		)
		ctx := context.Background()
		var podUID string

		BeforeAll(func() {
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

			By("creating a v1alpha1 TradeBot before the redeploy")
			Expect(k8sClient.Create(ctx, newExchangeSecret(exchangeSA))).To(Succeed())
			Expect(k8sClient.Create(ctx, newStrategy(strategy))).To(Succeed())
			Expect(k8sClient.Create(ctx, newDryRunTradeBotConfig(config, exchangeSA))).To(Succeed())
			Expect(k8sClient.Create(ctx, newTradeBot(botName, tradingNamespace, config, strategy))).To(Succeed())

			By("waiting for its pod to reach Ready before redeploying the operator")
			verifyPodReady := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "pod", botName+"-0",
					"-n", tradingNamespace, "-o", "jsonpath={.status.containerStatuses[0].ready}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("true"), "bot pod not Ready yet")
			}
			Eventually(verifyPodReady, "3m").Should(Succeed())

			cmd = exec.Command("kubectl", "get", "pod", botName+"-0", "-n", tradingNamespace, "-o", "jsonpath={.metadata.uid}")
			uid, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			podUID = uid
		})

		It("keeps the bot's pod running, untouched, across a manager redeploy", func() {
			By("redeploying the manager (closest available proxy for an operator upgrade - see doc comment above)")
			cmd := exec.Command("make", "deploy", fmt.Sprintf("IMG=%s", projectImage))
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to redeploy the controller-manager")
			cmd = exec.Command("kubectl", "rollout", "restart", "deployment/freqtrade-operator-controller-manager",
				"-n", namespace)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to restart the controller-manager deployment")
			cmd = exec.Command("kubectl", "rollout", "status", "deployment/freqtrade-operator-controller-manager",
				"-n", namespace, "--timeout=2m")
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Manager deployment did not roll out")

			By("checking the bot's pod was never recreated")
			cmd = exec.Command("kubectl", "get", "pod", botName+"-0", "-n", tradingNamespace, "-o", "jsonpath={.metadata.uid}")
			uidAfter, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(uidAfter).To(Equal(podUID), "bot pod was recreated across the manager redeploy, not left running")
		})

		It("still reads back as both v1alpha1 and v1beta1 after the redeploy", func() {
			// v1beta1 is the storage version (P6-5), so reading this back as v1alpha1 needs the
			// apiserver to call the conversion webhook - a genuinely different readiness signal
			// than the previous It's plain `kubectl get pod` UID check, which never touches any
			// webhook at all. kubectl rollout status only waits for the new pod's own /readyz
			// (the operator's own health endpoint) to pass, not for its separate webhook HTTPS
			// listener on :9443 to actually be accepting connections - the same class of gap
			// InstallCertManager's pollForCABundle already works around for cert-manager's own
			// webhook (test/utils/utils.go). Verified directly: a first pass with a bare (not
			// Eventually-wrapped) Get here failed with "dial tcp ... connect: connection
			// refused" against the webhook Service right after a real rollout completed.
			verifyReadableAsBothVersions := func(g Gomega) {
				var alpha freqtradev1alpha1.TradeBot
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: botName, Namespace: tradingNamespace}, &alpha)).To(Succeed())
				g.Expect(alpha.Spec.Config).To(Equal(config))

				var beta freqtradev1beta1.TradeBot
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: botName, Namespace: tradingNamespace}, &beta)).To(Succeed())
				g.Expect(beta.Spec.ConfigRef.Name).To(Equal(config))
			}
			Eventually(verifyReadableAsBothVersions, "1m").Should(Succeed())
		})
	})
}
