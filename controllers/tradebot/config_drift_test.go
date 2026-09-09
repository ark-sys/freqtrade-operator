package tradebot

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	"github.com/ark-sys/freqtrade-operator/controllers/tradebot/resources"
)

// updateStrategyAuto matches TradeBotSpec.UpdateStrategy's "Auto" enum value.
const updateStrategyAuto = "Auto"

// updateStakeAmount re-renders configName's Secret by changing the one
// field these specs care about, without ever touching the TradeBot itself -
// that's the whole point: config drift is about the Secret changing
// underneath an unrestarted workload.
func updateStakeAmount(ctx context.Context, configName, amount string) {
	var latestConfig freqtradev1alpha1.TradeBotConfig
	key := types.NamespacedName{Name: configName, Namespace: testNamespace}
	Expect(k8sClient.Get(ctx, key, &latestConfig)).To(Succeed())
	latestConfig.Spec.Bot.StakeAmount = amount
	Expect(k8sClient.Update(ctx, &latestConfig)).To(Succeed())
}

// This file covers P2-4: config.json is mounted from a Secret and read once
// at freqtrade startup, so rewriting the Secret alone doesn't change what a
// running bot is doing. spec.updateStrategy: Manual (the default, D2) never
// lets a config change trigger a restart on its own - a bot may be holding
// open positions - and reports the pending restart via ConfigDrift instead;
// Auto applies it by writing the new hash onto the StatefulSet pod
// template, letting Kubernetes' own rolling update carry it out.
var _ = Describe("config drift (P2-4)", func() {
	const eventuallyTimeout = "15s"
	const eventuallyPoll = "250ms"

	newDriftTestConfig := func(name, stakeAmount string) *freqtradev1alpha1.TradeBotConfig {
		return &freqtradev1alpha1.TradeBotConfig{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNamespace},
			Spec: freqtradev1alpha1.TradeBotConfigSpec{
				Bot:       &freqtradev1alpha1.BotConfig{DryRun: ptr.To(true), StakeAmount: stakeAmount},
				Exchange:  &freqtradev1alpha1.ExchangeSpec{Name: "binance"},
				APIServer: &freqtradev1alpha1.APIServerConfig{},
			},
		}
	}

	It("Manual: reports ConfigDrift and leaves the pod template untouched when the config changes", func() {
		ctx := context.Background()
		strategy := newTestStrategy("drift-manual-strategy")
		Expect(k8sClient.Create(ctx, strategy)).To(Succeed())
		config := newDriftTestConfig("drift-manual-config", "100")
		Expect(k8sClient.Create(ctx, config)).To(Succeed())

		tradeBot := newTestTradeBot("drift-manual-bot", strategy.Name, config.Name, "trade")
		Expect(k8sClient.Create(ctx, tradeBot)).To(Succeed())
		key := types.NamespacedName{Name: tradeBot.Name, Namespace: testNamespace}

		var firstHash string
		Eventually(func(g Gomega) {
			var sts appsv1.StatefulSet
			g.Expect(k8sClient.Get(ctx, key, &sts)).To(Succeed())
			firstHash = sts.Spec.Template.Annotations[resources.ConfigHashAnnotation]
			g.Expect(firstHash).NotTo(BeEmpty())
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

		updateStakeAmount(ctx, config.Name, "200")

		Eventually(func(g Gomega) {
			var latest freqtradev1alpha1.TradeBot
			g.Expect(k8sClient.Get(ctx, key, &latest)).To(Succeed())
			cond := findStatusCondition(latest.Status.Conditions, freqtradev1alpha1.ConditionConfigDrift)
			g.Expect(cond).NotTo(BeNil())
			g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(cond.Reason).To(Equal(freqtradev1alpha1.ReasonPendingRestart))
			g.Expect(latest.Status.AppliedConfigHash).NotTo(Equal(firstHash),
				"the applied hash should track the fresh config even though the workload doesn't yet")
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

		var secret corev1.Secret
		secretKey := types.NamespacedName{Name: tradeBot.Name + "-config", Namespace: testNamespace}
		Expect(k8sClient.Get(ctx, secretKey, &secret)).To(Succeed())
		Expect(string(secret.Data["config.json"])).To(ContainSubstring(`"stake_amount": 200`),
			"the Secret must be rewritten with the new config even though the workload isn't restarted yet")

		var sts appsv1.StatefulSet
		Expect(k8sClient.Get(ctx, key, &sts)).To(Succeed())
		Expect(sts.Spec.Template.Annotations[resources.ConfigHashAnnotation]).To(Equal(firstHash),
			"Manual must never let a config change touch the pod template")
	})

	It("Auto: rolls the pod template forward on its own, with no ConfigDrift", func() {
		ctx := context.Background()
		strategy := newTestStrategy("drift-auto-strategy")
		Expect(k8sClient.Create(ctx, strategy)).To(Succeed())
		config := newDriftTestConfig("drift-auto-config", "100")
		Expect(k8sClient.Create(ctx, config)).To(Succeed())

		tradeBot := newTestTradeBot("drift-auto-bot", strategy.Name, config.Name, "trade")
		tradeBot.Spec.UpdateStrategy = updateStrategyAuto
		Expect(k8sClient.Create(ctx, tradeBot)).To(Succeed())
		key := types.NamespacedName{Name: tradeBot.Name, Namespace: testNamespace}

		var firstHash string
		Eventually(func(g Gomega) {
			var sts appsv1.StatefulSet
			g.Expect(k8sClient.Get(ctx, key, &sts)).To(Succeed())
			firstHash = sts.Spec.Template.Annotations[resources.ConfigHashAnnotation]
			g.Expect(firstHash).NotTo(BeEmpty())
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

		updateStakeAmount(ctx, config.Name, "200")

		Eventually(func(g Gomega) {
			var sts appsv1.StatefulSet
			g.Expect(k8sClient.Get(ctx, key, &sts)).To(Succeed())
			g.Expect(sts.Spec.Template.Annotations[resources.ConfigHashAnnotation]).NotTo(Equal(firstHash),
				"Auto must apply a real config change to the pod template itself")
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

		var latest freqtradev1alpha1.TradeBot
		Expect(k8sClient.Get(ctx, key, &latest)).To(Succeed())
		cond := findStatusCondition(latest.Status.Conditions, freqtradev1alpha1.ConditionConfigDrift)
		Expect(cond).NotTo(BeNil())
		Expect(cond.Status).To(Equal(metav1.ConditionFalse))
	})

	It("clears ConfigDrift once switched from Manual to Auto", func() {
		ctx := context.Background()
		strategy := newTestStrategy("drift-clear-strategy")
		Expect(k8sClient.Create(ctx, strategy)).To(Succeed())
		config := newDriftTestConfig("drift-clear-config", "100")
		Expect(k8sClient.Create(ctx, config)).To(Succeed())

		tradeBot := newTestTradeBot("drift-clear-bot", strategy.Name, config.Name, "trade")
		Expect(k8sClient.Create(ctx, tradeBot)).To(Succeed())
		key := types.NamespacedName{Name: tradeBot.Name, Namespace: testNamespace}

		Eventually(func() error {
			return k8sClient.Get(ctx, key, &appsv1.StatefulSet{})
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

		updateStakeAmount(ctx, config.Name, "200")

		Eventually(func(g Gomega) {
			var latest freqtradev1alpha1.TradeBot
			g.Expect(k8sClient.Get(ctx, key, &latest)).To(Succeed())
			cond := findStatusCondition(latest.Status.Conditions, freqtradev1alpha1.ConditionConfigDrift)
			g.Expect(cond).NotTo(BeNil())
			g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

		// This is the remedy the ConfigDrift message itself names: switching
		// to Auto applies the pending config immediately.
		var latest freqtradev1alpha1.TradeBot
		Expect(k8sClient.Get(ctx, key, &latest)).To(Succeed())
		latest.Spec.UpdateStrategy = updateStrategyAuto
		Expect(k8sClient.Update(ctx, &latest)).To(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, key, &latest)).To(Succeed())
			cond := findStatusCondition(latest.Status.Conditions, freqtradev1alpha1.ConditionConfigDrift)
			g.Expect(cond).NotTo(BeNil())
			g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
	})
})
