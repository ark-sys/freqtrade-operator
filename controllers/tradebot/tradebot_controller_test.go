package tradebot

import (
	"context"
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
)

const testNamespace = "default"

func newTestStrategy(name string) *freqtradev1alpha1.Strategy {
	return &freqtradev1alpha1.Strategy{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNamespace},
		Spec: freqtradev1alpha1.StrategySpec{
			Name:   "SampleStrategy",
			Script: "class SampleStrategy:\n    pass",
		},
	}
}

func newTestTradeBotConfig(name string) *freqtradev1alpha1.TradeBotConfig {
	return &freqtradev1alpha1.TradeBotConfig{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNamespace},
		Spec: freqtradev1alpha1.TradeBotConfigSpec{
			// Bot has no omitempty on its json tag, so controller-gen already
			// marks spec.bot required in the generated CRD schema even
			// without an explicit kubebuilder marker (P1-1 hasn't landed
			// yet). Exchange is effectively required for a different reason
			// (P0-1). Everything else can be left nil for these tests, which
			// only care that config.json renders successfully, not its
			// content (that's P5-1's job).
			Bot:      &freqtradev1alpha1.BotConfig{},
			Exchange: &freqtradev1alpha1.ExchangeSpec{Name: "binance"},
			// APIServer must be non-nil for the api_server section (and so
			// CORS_origins) to render at all - see BuildTradeBotConfig.
			APIServer: &freqtradev1alpha1.APIServerConfig{},
		},
	}
}

func newTestTradeBot(name, strategyName, configName, command string) *freqtradev1alpha1.TradeBot {
	return &freqtradev1alpha1.TradeBot{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNamespace},
		Spec: freqtradev1alpha1.TradeBotSpec{
			FreqtradeCommand: command,
			Config:           configName,
			Strategy:         strategyName,
		},
	}
}

var _ = Describe("TradeBot controller", func() {
	// Every fresh reconcile spends its first pass only adding the
	// finalizer (main.go step 3) and requeues after 5s before building any
	// resources, so every scenario here needs headroom past that baseline.
	const eventuallyTimeout = "15s"
	const eventuallyPoll = "250ms"

	AfterEach(func() {
		// Best-effort cleanup so one spec's objects can't leak into another's
		// List() calls (e.g. the FreqUI CORS spec lists all FreqUIs in the
		// namespace). Failures here are not asserted on: some objects may
		// already be gone by design (finalizer scenarios).
		_ = k8sClient.DeleteAllOf(context.Background(), &freqtradev1alpha1.TradeBot{}, client.InNamespace(testNamespace))
		_ = k8sClient.DeleteAllOf(context.Background(), &freqtradev1alpha1.FreqUI{}, client.InNamespace(testNamespace))
	})

	Describe("creating a trade-mode TradeBot", func() {
		It("creates Secret, ConfigMap, PVC, StatefulSet and Service with correct owner references", func() {
			ctx := context.Background()
			strategy := newTestStrategy("strategy-trade")
			config := newTestTradeBotConfig("config-trade")
			Expect(k8sClient.Create(ctx, strategy)).To(Succeed())
			Expect(k8sClient.Create(ctx, config)).To(Succeed())

			tradeBot := newTestTradeBot("bot-trade", strategy.Name, config.Name, "trade")
			Expect(k8sClient.Create(ctx, tradeBot)).To(Succeed())

			key := types.NamespacedName{Name: tradeBot.Name, Namespace: testNamespace}

			Eventually(func(g Gomega) {
				var sts appsv1.StatefulSet
				g.Expect(k8sClient.Get(ctx, key, &sts)).To(Succeed())
				g.Expect(sts.OwnerReferences).To(HaveLen(1))
				g.Expect(sts.OwnerReferences[0].Name).To(Equal(tradeBot.Name))

				var svc corev1.Service
				g.Expect(k8sClient.Get(ctx, key, &svc)).To(Succeed())

				pvcKey := types.NamespacedName{Name: tradeBot.Name + "-user-data", Namespace: testNamespace}
				var pvc corev1.PersistentVolumeClaim
				g.Expect(k8sClient.Get(ctx, pvcKey, &pvc)).To(Succeed())

				secretKey := types.NamespacedName{Name: tradeBot.Name + "-config", Namespace: testNamespace}
				var secret corev1.Secret
				g.Expect(k8sClient.Get(ctx, secretKey, &secret)).To(Succeed())
				g.Expect(secret.Data).To(HaveKey("config.json"))

				cmKey := types.NamespacedName{Name: tradeBot.Name + "-strategy", Namespace: testNamespace}
				var cm corev1.ConfigMap
				g.Expect(k8sClient.Get(ctx, cmKey, &cm)).To(Succeed())
			}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
		})
	})

	Describe("creating a job-mode (one-shot) TradeBot", func() {
		It("creates a Job and never a StatefulSet or Service", func() {
			ctx := context.Background()
			strategy := newTestStrategy("strategy-job")
			config := newTestTradeBotConfig("config-job")
			Expect(k8sClient.Create(ctx, strategy)).To(Succeed())
			Expect(k8sClient.Create(ctx, config)).To(Succeed())

			tradeBot := newTestTradeBot("bot-job", strategy.Name, config.Name, "backtesting")
			Expect(k8sClient.Create(ctx, tradeBot)).To(Succeed())

			key := types.NamespacedName{Name: tradeBot.Name, Namespace: testNamespace}

			Eventually(func(g Gomega) {
				var job batchv1.Job
				g.Expect(k8sClient.Get(ctx, key, &job)).To(Succeed())
			}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

			Consistently(func() bool {
				var sts appsv1.StatefulSet
				return errors.IsNotFound(k8sClient.Get(ctx, key, &sts))
			}, "3s", eventuallyPoll).Should(BeTrue(), "a job-mode TradeBot must never get a StatefulSet")
		})
	})

	// The money-losing case (P0-4): switching modes must not leave the
	// previous mode's workload running under stale config.
	Describe("switching freqtrade_command after creation", func() {
		It("prunes the StatefulSet and Service when switching from trade to a one-shot command", func() {
			ctx := context.Background()
			strategy := newTestStrategy("strategy-switch-to-job")
			config := newTestTradeBotConfig("config-switch-to-job")
			Expect(k8sClient.Create(ctx, strategy)).To(Succeed())
			Expect(k8sClient.Create(ctx, config)).To(Succeed())

			tradeBot := newTestTradeBot("bot-switch-to-job", strategy.Name, config.Name, "trade")
			Expect(k8sClient.Create(ctx, tradeBot)).To(Succeed())
			key := types.NamespacedName{Name: tradeBot.Name, Namespace: testNamespace}

			Eventually(func() error {
				return k8sClient.Get(ctx, key, &appsv1.StatefulSet{})
			}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

			Eventually(func() error {
				var latest freqtradev1alpha1.TradeBot
				if err := k8sClient.Get(ctx, key, &latest); err != nil {
					return err
				}
				latest.Spec.FreqtradeCommand = "backtesting"
				return k8sClient.Update(ctx, &latest)
			}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(errors.IsNotFound(k8sClient.Get(ctx, key, &appsv1.StatefulSet{}))).To(BeTrue())
				g.Expect(errors.IsNotFound(k8sClient.Get(ctx, key, &corev1.Service{}))).To(BeTrue())
				g.Expect(k8sClient.Get(ctx, key, &batchv1.Job{})).To(Succeed())
			}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
		})

		It("prunes the Job when switching back from a one-shot command to trade", func() {
			ctx := context.Background()
			strategy := newTestStrategy("strategy-switch-to-trade")
			config := newTestTradeBotConfig("config-switch-to-trade")
			Expect(k8sClient.Create(ctx, strategy)).To(Succeed())
			Expect(k8sClient.Create(ctx, config)).To(Succeed())

			tradeBot := newTestTradeBot("bot-switch-to-trade", strategy.Name, config.Name, "backtesting")
			Expect(k8sClient.Create(ctx, tradeBot)).To(Succeed())
			key := types.NamespacedName{Name: tradeBot.Name, Namespace: testNamespace}

			Eventually(func() error {
				return k8sClient.Get(ctx, key, &batchv1.Job{})
			}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

			Eventually(func() error {
				var latest freqtradev1alpha1.TradeBot
				if err := k8sClient.Get(ctx, key, &latest); err != nil {
					return err
				}
				latest.Spec.FreqtradeCommand = "trade"
				return k8sClient.Update(ctx, &latest)
			}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(errors.IsNotFound(k8sClient.Get(ctx, key, &batchv1.Job{}))).To(BeTrue())
				g.Expect(k8sClient.Get(ctx, key, &appsv1.StatefulSet{})).To(Succeed())
			}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
		})
	})

	// Full ConfigResolved=False/Reason=ReferenceNotFound conditions are
	// P1-3's vocabulary; today the observable signal is Status.Phase. The
	// point of these two specs is P0-1/P0-2: a missing reference must be a
	// reported error, never a panic that takes the manager down with it.
	Describe("a TradeBot referencing a resource that doesn't exist", func() {
		It("reports an Error phase and creates no workload when the Strategy is missing", func() {
			ctx := context.Background()
			config := newTestTradeBotConfig("config-missing-strategy")
			Expect(k8sClient.Create(ctx, config)).To(Succeed())

			tradeBot := newTestTradeBot("bot-missing-strategy", "does-not-exist", config.Name, "trade")
			Expect(k8sClient.Create(ctx, tradeBot)).To(Succeed())
			key := types.NamespacedName{Name: tradeBot.Name, Namespace: testNamespace}

			Eventually(func(g Gomega) {
				var latest freqtradev1alpha1.TradeBot
				g.Expect(k8sClient.Get(ctx, key, &latest)).To(Succeed())
				g.Expect(latest.Status.Phase).To(Equal("Error"))
			}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

			Expect(errors.IsNotFound(k8sClient.Get(ctx, key, &appsv1.StatefulSet{}))).To(BeTrue())
		})

		It("reports an Error phase and creates no workload when the TradeBotConfig is missing", func() {
			ctx := context.Background()
			strategy := newTestStrategy("strategy-missing-config")
			Expect(k8sClient.Create(ctx, strategy)).To(Succeed())

			tradeBot := newTestTradeBot("bot-missing-config", strategy.Name, "does-not-exist", "trade")
			Expect(k8sClient.Create(ctx, tradeBot)).To(Succeed())
			key := types.NamespacedName{Name: tradeBot.Name, Namespace: testNamespace}

			Eventually(func(g Gomega) {
				var latest freqtradev1alpha1.TradeBot
				g.Expect(k8sClient.Get(ctx, key, &latest)).To(Succeed())
				g.Expect(latest.Status.Phase).To(Equal("Error"))
			}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

			Expect(errors.IsNotFound(k8sClient.Get(ctx, key, &appsv1.StatefulSet{}))).To(BeTrue())
		})
	})

	Describe("deleting a TradeBot", func() {
		It("scales the StatefulSet to zero and removes the finalizer", func() {
			ctx := context.Background()
			strategy := newTestStrategy("strategy-delete")
			config := newTestTradeBotConfig("config-delete")
			Expect(k8sClient.Create(ctx, strategy)).To(Succeed())
			Expect(k8sClient.Create(ctx, config)).To(Succeed())

			tradeBot := newTestTradeBot("bot-delete", strategy.Name, config.Name, "trade")
			Expect(k8sClient.Create(ctx, tradeBot)).To(Succeed())
			key := types.NamespacedName{Name: tradeBot.Name, Namespace: testNamespace}

			Eventually(func() error {
				return k8sClient.Get(ctx, key, &appsv1.StatefulSet{})
			}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

			var toDelete freqtradev1alpha1.TradeBot
			Expect(k8sClient.Get(ctx, key, &toDelete)).To(Succeed())
			Expect(k8sClient.Delete(ctx, &toDelete)).To(Succeed())

			// envtest runs no StatefulSet controller, so Status never
			// reports readiness on its own; ensureStatefulSetScaledDown
			// only waits for Spec.Replicas to reach 0, which is entirely
			// driven by this reconciler.
			Eventually(func(g Gomega) {
				var sts appsv1.StatefulSet
				err := k8sClient.Get(ctx, key, &sts)
				if errors.IsNotFound(err) {
					return // already gone, which is fine too
				}
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(sts.Spec.Replicas).NotTo(BeNil())
				g.Expect(*sts.Spec.Replicas).To(Equal(int32(0)))
			}, "20s", eventuallyPoll).Should(Succeed())

			Eventually(func() bool {
				return errors.IsNotFound(k8sClient.Get(ctx, key, &freqtradev1alpha1.TradeBot{}))
			}, "20s", eventuallyPoll).Should(BeTrue(), "the TradeBot itself should be gone once the finalizer completes")
		})

		It("preserves the PVC with its owner reference stripped when annotated to do so", func() {
			ctx := context.Background()
			strategy := newTestStrategy("strategy-preserve")
			config := newTestTradeBotConfig("config-preserve")
			Expect(k8sClient.Create(ctx, strategy)).To(Succeed())
			Expect(k8sClient.Create(ctx, config)).To(Succeed())

			tradeBot := newTestTradeBot("bot-preserve", strategy.Name, config.Name, "trade")
			tradeBot.Annotations = map[string]string{"freqtrade.io/preserve-data": "true"}
			Expect(k8sClient.Create(ctx, tradeBot)).To(Succeed())
			key := types.NamespacedName{Name: tradeBot.Name, Namespace: testNamespace}
			pvcKey := types.NamespacedName{Name: tradeBot.Name + "-user-data", Namespace: testNamespace}

			Eventually(func() error {
				return k8sClient.Get(ctx, pvcKey, &corev1.PersistentVolumeClaim{})
			}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

			var toDelete freqtradev1alpha1.TradeBot
			Expect(k8sClient.Get(ctx, key, &toDelete)).To(Succeed())
			Expect(k8sClient.Delete(ctx, &toDelete)).To(Succeed())

			Eventually(func() bool {
				return errors.IsNotFound(k8sClient.Get(ctx, key, &freqtradev1alpha1.TradeBot{}))
			}, "20s", eventuallyPoll).Should(BeTrue())

			var pvc corev1.PersistentVolumeClaim
			Expect(k8sClient.Get(ctx, pvcKey, &pvc)).To(Succeed(), "the preserved PVC must survive TradeBot deletion")
			Expect(pvc.OwnerReferences).To(BeEmpty())
			Expect(pvc.Annotations).To(HaveKeyWithValue("freqtrade.io/preserved-from", tradeBot.Name))
			Expect(pvc.Annotations).To(HaveKey("freqtrade.io/preserved-at"))
		})
	})

	Describe("FreqUI to TradeBot CORS propagation", func() {
		It("renders a FreqUI's host into the TradeBot's config Secret", func() {
			ctx := context.Background()
			strategy := newTestStrategy("strategy-cors")
			config := newTestTradeBotConfig("config-cors")
			Expect(k8sClient.Create(ctx, strategy)).To(Succeed())
			Expect(k8sClient.Create(ctx, config)).To(Succeed())

			tradeBot := newTestTradeBot("bot-cors", strategy.Name, config.Name, "trade")
			Expect(k8sClient.Create(ctx, tradeBot)).To(Succeed())
			secretKey := types.NamespacedName{Name: tradeBot.Name + "-config", Namespace: testNamespace}

			Eventually(func() error {
				return k8sClient.Get(ctx, secretKey, &corev1.Secret{})
			}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

			frequi := &freqtradev1alpha1.FreqUI{
				ObjectMeta: metav1.ObjectMeta{Name: "ui-cors", Namespace: testNamespace},
				Spec:       freqtradev1alpha1.FreqUISpec{Host: "frequi.example.com", TradeBotRefs: []string{tradeBot.Name}},
			}
			Expect(k8sClient.Create(ctx, frequi)).To(Succeed())

			Eventually(func(g Gomega) {
				var secret corev1.Secret
				g.Expect(k8sClient.Get(ctx, secretKey, &secret)).To(Succeed())

				var rendered map[string]interface{}
				g.Expect(json.Unmarshal(secret.Data["config.json"], &rendered)).To(Succeed())

				apiServer, _ := rendered["api_server"].(map[string]interface{})
				g.Expect(apiServer).NotTo(BeNil())
				corsOrigins, _ := apiServer["CORS_origins"].([]interface{})
				g.Expect(corsOrigins).To(ContainElement("https://frequi.example.com"))
			}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
		})
	})
})
