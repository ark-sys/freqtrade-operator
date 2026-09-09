package backtest

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	freqtradev1beta1 "github.com/ark-sys/freqtrade-operator/api/v1beta1"
)

const testNamespace = "default"

// validTestStrategyScript satisfies the P1-4 admission webhook's shape
// check on Strategy - a fixture creating one that fails this is rejected
// before this package's own tests ever run.
const validTestStrategyScript = `import freqtrade
from freqtrade.strategy import IStrategy

class SampleStrategy(IStrategy):
    def populate_indicators(self, dataframe, metadata):
        return dataframe

    def populate_entry_trend(self, dataframe, metadata):
        return dataframe

    def populate_exit_trend(self, dataframe, metadata):
        return dataframe
`

func newTestStrategy(name string) *freqtradev1alpha1.Strategy {
	return &freqtradev1alpha1.Strategy{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNamespace},
		Spec:       freqtradev1alpha1.StrategySpec{Name: "SampleStrategy", Script: validTestStrategyScript},
	}
}

func newTestTradeBotConfig(name string) *freqtradev1alpha1.TradeBotConfig {
	return &freqtradev1alpha1.TradeBotConfig{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNamespace},
		Spec: freqtradev1alpha1.TradeBotConfigSpec{
			Bot:      &freqtradev1alpha1.BotConfig{DryRun: ptr.To(true)},
			Exchange: &freqtradev1alpha1.ExchangeSpec{Name: "binance"},
		},
	}
}

func newTestBacktest(name, strategyName, configName string) *freqtradev1beta1.Backtest {
	return &freqtradev1beta1.Backtest{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNamespace},
		Spec: freqtradev1beta1.BacktestSpec{
			RunSpec: freqtradev1beta1.RunSpec{
				ConfigRef:   corev1.LocalObjectReference{Name: configName},
				StrategyRef: corev1.LocalObjectReference{Name: strategyName},
				Timerange:   "20230101-20230201",
			},
		},
	}
}

var _ = Describe("Backtest controller", func() {
	const eventuallyTimeout = "15s"
	const eventuallyPoll = "250ms"

	AfterEach(func() {
		_ = k8sClient.DeleteAllOf(context.Background(), &freqtradev1beta1.Backtest{}, client.InNamespace(testNamespace))
	})

	Describe("creating a Backtest", func() {
		It("creates Secret, ConfigMap, results PVC and Job with correct owner references", func() {
			ctx := context.Background()
			strategy := newTestStrategy("strategy-bt")
			config := newTestTradeBotConfig("config-bt")
			Expect(k8sClient.Create(ctx, strategy)).To(Succeed())
			Expect(k8sClient.Create(ctx, config)).To(Succeed())

			backtest := newTestBacktest("run-bt", strategy.Name, config.Name)
			Expect(k8sClient.Create(ctx, backtest)).To(Succeed())

			key := types.NamespacedName{Name: backtest.Name, Namespace: testNamespace}

			Eventually(func(g Gomega) {
				var job batchv1.Job
				g.Expect(k8sClient.Get(ctx, key, &job)).To(Succeed())
				g.Expect(job.OwnerReferences).To(HaveLen(1))
				g.Expect(job.OwnerReferences[0].Name).To(Equal(backtest.Name))

				secretKey := types.NamespacedName{Name: backtest.Name + "-config", Namespace: testNamespace}
				var secret corev1.Secret
				g.Expect(k8sClient.Get(ctx, secretKey, &secret)).To(Succeed())
				g.Expect(secret.Data).To(HaveKey("config.json"))

				cmKey := types.NamespacedName{Name: backtest.Name + "-strategy", Namespace: testNamespace}
				g.Expect(k8sClient.Get(ctx, cmKey, &corev1.ConfigMap{})).To(Succeed())

				pvcKey := types.NamespacedName{Name: backtest.Name + "-results", Namespace: testNamespace}
				g.Expect(k8sClient.Get(ctx, pvcKey, &corev1.PersistentVolumeClaim{})).To(Succeed())

				var got freqtradev1beta1.Backtest
				g.Expect(k8sClient.Get(ctx, key, &got)).To(Succeed())
				g.Expect(got.Status.JobName).To(Equal(backtest.Name))
			}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
		})

		It("reflects the Job reaching Succeeded into status.phase and WorkloadReady", func() {
			ctx := context.Background()
			strategy := newTestStrategy("strategy-bt-succeed")
			config := newTestTradeBotConfig("config-bt-succeed")
			Expect(k8sClient.Create(ctx, strategy)).To(Succeed())
			Expect(k8sClient.Create(ctx, config)).To(Succeed())

			backtest := newTestBacktest("run-bt-succeed", strategy.Name, config.Name)
			Expect(k8sClient.Create(ctx, backtest)).To(Succeed())
			key := types.NamespacedName{Name: backtest.Name, Namespace: testNamespace}

			var job batchv1.Job
			Eventually(func() error {
				return k8sClient.Get(ctx, key, &job)
			}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

			// envtest runs no Job controller, so nothing ever marks a Job
			// Succeeded on its own - simulate what a real cluster's job
			// controller would eventually write.
			job.Status.Succeeded = 1
			Expect(k8sClient.Status().Update(ctx, &job)).To(Succeed())

			Eventually(func(g Gomega) {
				var got freqtradev1beta1.Backtest
				g.Expect(k8sClient.Get(ctx, key, &got)).To(Succeed())
				g.Expect(got.Status.Phase).To(Equal("Succeeded"))
				cond := findStatusCondition(got.Status.Conditions, freqtradev1beta1.ConditionWorkloadReady)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
				g.Expect(cond.Reason).To(Equal(freqtradev1beta1.ReasonWorkloadSucceeded))
			}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
		})

		// envtest runs no kubelet, so nothing ever runs the P6-2 sidecar for
		// real - this stands in for it exactly the way the previous spec
		// stands in for the Job controller, writing the same ConfigMap
		// shape controllers/backtest/collectresults itself would.
		It("adopts and parses the sidecar's results ConfigMap once the Job succeeds (P6-2)", func() {
			ctx := context.Background()
			strategy := newTestStrategy("strategy-bt-results")
			config := newTestTradeBotConfig("config-bt-results")
			Expect(k8sClient.Create(ctx, strategy)).To(Succeed())
			Expect(k8sClient.Create(ctx, config)).To(Succeed())

			backtest := newTestBacktest("run-bt-results", strategy.Name, config.Name)
			Expect(k8sClient.Create(ctx, backtest)).To(Succeed())
			key := types.NamespacedName{Name: backtest.Name, Namespace: testNamespace}

			var job batchv1.Job
			Eventually(func() error {
				return k8sClient.Get(ctx, key, &job)
			}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

			resultsCM := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: backtest.Name + "-results", Namespace: testNamespace},
				Data:       map[string]string{"results.json": `{"totalTrades":3,"profitAbs":"42.5"}`},
			}
			Expect(k8sClient.Create(ctx, resultsCM)).To(Succeed())

			job.Status.Succeeded = 1
			Expect(k8sClient.Status().Update(ctx, &job)).To(Succeed())

			Eventually(func(g Gomega) {
				var got freqtradev1beta1.Backtest
				g.Expect(k8sClient.Get(ctx, key, &got)).To(Succeed())
				g.Expect(got.Status.Results).NotTo(BeNil())
				g.Expect(got.Status.Results.TotalTrades).To(Equal(3))
				g.Expect(got.Status.Results.ProfitAbs).To(Equal("42.5"))
				cond := findStatusCondition(got.Status.Conditions, freqtradev1beta1.ConditionResultsAvailable)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

			var adoptedCM corev1.ConfigMap
			cmKey := types.NamespacedName{Name: backtest.Name + "-results", Namespace: testNamespace}
			Expect(k8sClient.Get(ctx, cmKey, &adoptedCM)).To(Succeed())
			Expect(adoptedCM.OwnerReferences).To(HaveLen(1))
			Expect(adoptedCM.OwnerReferences[0].Name).To(Equal(backtest.Name))
		})

		It("provisions the sidecar's ServiceAccount/Role/RoleBinding and wires the Job's sidecar container", func() {
			ctx := context.Background()
			strategy := newTestStrategy("strategy-bt-sidecar")
			config := newTestTradeBotConfig("config-bt-sidecar")
			Expect(k8sClient.Create(ctx, strategy)).To(Succeed())
			Expect(k8sClient.Create(ctx, config)).To(Succeed())

			backtest := newTestBacktest("run-bt-sidecar", strategy.Name, config.Name)
			Expect(k8sClient.Create(ctx, backtest)).To(Succeed())

			rbacKey := types.NamespacedName{Name: "freqtrade-backtest-sidecar", Namespace: testNamespace}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, rbacKey, &corev1.ServiceAccount{})).To(Succeed())
				g.Expect(k8sClient.Get(ctx, rbacKey, &rbacv1.Role{})).To(Succeed())
				g.Expect(k8sClient.Get(ctx, rbacKey, &rbacv1.RoleBinding{})).To(Succeed())

				var job batchv1.Job
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name: backtest.Name, Namespace: testNamespace,
				}, &job)).To(Succeed())
				g.Expect(job.Spec.Template.Spec.ServiceAccountName).To(Equal("freqtrade-backtest-sidecar"))

				var sidecar *corev1.Container
				for i := range job.Spec.Template.Spec.InitContainers {
					if job.Spec.Template.Spec.InitContainers[i].Name == "collect-results" {
						sidecar = &job.Spec.Template.Spec.InitContainers[i]
					}
				}
				g.Expect(sidecar).NotTo(BeNil())
				g.Expect(sidecar.RestartPolicy).NotTo(BeNil())
				g.Expect(*sidecar.RestartPolicy).To(Equal(corev1.ContainerRestartPolicyAlways))
				g.Expect(sidecar.Image).To(Equal("test-operator-image:latest"))
			}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
		})
	})

	Describe("a Backtest referencing a Strategy that doesn't exist", func() {
		It("reports ConfigResolved=False/ReferenceNotFound and creates no Job", func() {
			ctx := context.Background()
			config := newTestTradeBotConfig("config-bt-missing-strategy")
			Expect(k8sClient.Create(ctx, config)).To(Succeed())

			backtest := newTestBacktest("run-bt-missing-strategy", "does-not-exist", config.Name)
			Expect(k8sClient.Create(ctx, backtest)).To(Succeed())
			key := types.NamespacedName{Name: backtest.Name, Namespace: testNamespace}

			Eventually(func(g Gomega) {
				var got freqtradev1beta1.Backtest
				g.Expect(k8sClient.Get(ctx, key, &got)).To(Succeed())
				cond := findStatusCondition(got.Status.Conditions, freqtradev1beta1.ConditionConfigResolved)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal(freqtradev1beta1.ReasonReferenceNotFound))
			}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

			Expect(errors.IsNotFound(k8sClient.Get(ctx, key, &batchv1.Job{}))).To(BeTrue())
		})
	})

	Describe("CRD immutability (CEL, P6-1)", func() {
		It("rejects a spec change after creation", func() {
			ctx := context.Background()
			strategy := newTestStrategy("strategy-bt-immutable")
			config := newTestTradeBotConfig("config-bt-immutable")
			Expect(k8sClient.Create(ctx, strategy)).To(Succeed())
			Expect(k8sClient.Create(ctx, config)).To(Succeed())

			backtest := newTestBacktest("run-bt-immutable", strategy.Name, config.Name)
			Expect(k8sClient.Create(ctx, backtest)).To(Succeed())

			backtest.Spec.Timerange = "20240101-20240201"
			Expect(k8sClient.Update(ctx, backtest)).NotTo(Succeed())
		})
	})

	Describe("CRD schema validation (P6-1)", func() {
		It("rejects a malformed timerange", func() {
			backtest := newTestBacktest("run-bt-bad-timerange", "irrelevant", "irrelevant")
			backtest.Spec.Timerange = "not-a-timerange"
			Expect(k8sClient.Create(context.Background(), backtest)).NotTo(Succeed())
		})

		It("rejects extraArgs without the allow-extra-args annotation", func() {
			backtest := newTestBacktest("run-bt-bad-extraargs", "irrelevant", "irrelevant")
			backtest.Spec.ExtraArgs = []string{"--enable-position-stacking"}
			Expect(k8sClient.Create(context.Background(), backtest)).NotTo(Succeed())
		})
	})
})

func findStatusCondition(conditions []metav1.Condition, condType string) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == condType {
			return &conditions[i]
		}
	}
	return nil
}
