package e2e

import (
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/ark-sys/freqtrade-operator/test/utils"
)

var _ = Describe("Pairlists", func() {
	var testNamespace = "freqtrade-test"

	BeforeEach(func() {
		By("Creating test namespace")
		cmd := exec.Command("kubectl", "create", "ns", testNamespace)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create test namespace")
	})

	AfterEach(func() {
		By("Cleaning up test resources")
		cmd := exec.Command("kubectl", "delete", "ns", testNamespace, "--wait=false")
		_, _ = utils.Run(cmd)
	})

	It("should successfully create and use pairlists configuration", func() {
		By("Creating exchange secret")
		cmd := exec.Command("kubectl", "apply", "-f", "../../examples/exchange-secret.yaml", "-n", testNamespace)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create exchange secret")

		By("Creating exchange resource")
		cmd = exec.Command("kubectl", "apply", "-f", "../../examples/exchange.yaml", "-n", testNamespace)
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create exchange resource")

		By("Creating pairlists resource")
		cmd = exec.Command("kubectl", "apply", "-f", "../../examples/pairlists.yaml", "-n", testNamespace)
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create pairlists resource")

		By("Creating strategy resource")
		cmd = exec.Command("kubectl", "apply", "-f", "../../examples/strategy.yaml", "-n", testNamespace)
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create strategy resource")

		By("Creating tradebot resource")
		// Create a temporary tradebot.yaml file with the correct namespace
		cmd = exec.Command("sed", "s/namespace: freqtrade/namespace: "+testNamespace+"/g", "../../examples/tradebot.yaml")
		tradebotYaml, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to generate tradebot yaml")

		cmd = exec.Command("kubectl", "apply", "-f", "-", "-n", testNamespace)
		cmd.Stdin = strings.NewReader(tradebotYaml)
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create tradebot resource")

		By("Verifying the tradebot configmap is created with pairlists configuration")
		verifyConfigMap := func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "configmap", "btc-trader-config", "-n", testNamespace, "-o", "jsonpath={.data['config\\.json']}")
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred(), "Failed to get configmap")

			// Verify the configmap contains pairlists configuration
			g.Expect(output).To(ContainSubstring("\"pairlists\""), "Configmap should contain pairlists configuration")
			g.Expect(output).To(ContainSubstring("\"StaticPairList\""), "Configmap should contain StaticPairList")
			g.Expect(output).To(ContainSubstring("\"VolumePairList\""), "Configmap should contain VolumePairList")
			g.Expect(output).To(ContainSubstring("\"AgeFilter\""), "Configmap should contain AgeFilter")
		}
		Eventually(verifyConfigMap, 30*time.Second, time.Second).Should(Succeed())

		By("Verifying the tradebot pod is running")
		verifyTradebotRunning := func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "pod", "-l", "app=btc-trader", "-n", testNamespace, "-o", "jsonpath={.items[0].status.phase}")
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred(), "Failed to get tradebot pod")
			g.Expect(output).To(Equal("Running"), "Tradebot pod should be running")
		}
		Eventually(verifyTradebotRunning, 2*time.Minute, 5*time.Second).Should(Succeed())
	})
})
