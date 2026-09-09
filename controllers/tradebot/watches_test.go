package tradebot

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
)

// This file covers P2-3's acceptance test and the correctness bug it names:
// spec.config/spec.strategy field indexes were registered but never used,
// and nothing watched TradeBotConfig or Strategy at all, so editing either
// never re-rendered a referencing TradeBot's config Secret until some
// unrelated event happened to trigger a reconcile.
var _ = Describe("watching referenced TradeBotConfig/Strategy (P2-3)", func() {
	const eventuallyTimeout = "15s"
	const eventuallyPoll = "250ms"

	It("re-renders the config Secret when the referenced TradeBotConfig changes, without touching the TradeBot", func() {
		ctx := context.Background()
		strategy := newTestStrategy("watch-config-strategy")
		Expect(k8sClient.Create(ctx, strategy)).To(Succeed())

		config := &freqtradev1alpha1.TradeBotConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "watch-config-config", Namespace: testNamespace},
			Spec: freqtradev1alpha1.TradeBotConfigSpec{
				Bot:       &freqtradev1alpha1.BotConfig{DryRun: ptr.To(true), StakeAmount: "100"},
				Exchange:  &freqtradev1alpha1.ExchangeSpec{Name: "binance"},
				APIServer: &freqtradev1alpha1.APIServerConfig{},
			},
		}
		Expect(k8sClient.Create(ctx, config)).To(Succeed())

		tradeBot := newTestTradeBot("watch-config-bot", strategy.Name, config.Name, "trade")
		Expect(k8sClient.Create(ctx, tradeBot)).To(Succeed())
		secretKey := types.NamespacedName{Name: tradeBot.Name + "-config", Namespace: testNamespace}

		Eventually(func(g Gomega) {
			var secret corev1.Secret
			g.Expect(k8sClient.Get(ctx, secretKey, &secret)).To(Succeed())
			g.Expect(string(secret.Data["config.json"])).To(ContainSubstring(`"stake_amount": 100`))
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

		// Only the TradeBotConfig is touched from here on - if the Secret
		// still updates, it can only be because the new Watches() on
		// TradeBotConfig (setup.go) picked this up on its own.
		var latestConfig freqtradev1alpha1.TradeBotConfig
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: config.Name, Namespace: testNamespace}, &latestConfig)).To(Succeed())
		latestConfig.Spec.Bot.StakeAmount = "150"
		Expect(k8sClient.Update(ctx, &latestConfig)).To(Succeed())

		Eventually(func(g Gomega) {
			var secret corev1.Secret
			g.Expect(k8sClient.Get(ctx, secretKey, &secret)).To(Succeed())
			g.Expect(string(secret.Data["config.json"])).To(ContainSubstring(`"stake_amount": 150`))
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
	})

	It("re-renders the strategy ConfigMap when the referenced Strategy's script changes", func() {
		ctx := context.Background()
		strategy := &freqtradev1alpha1.Strategy{
			ObjectMeta: metav1.ObjectMeta{Name: "watch-strategy-strategy", Namespace: testNamespace},
			Spec:       freqtradev1alpha1.StrategySpec{Name: "SampleStrategy", Script: validTestStrategyScript},
		}
		Expect(k8sClient.Create(ctx, strategy)).To(Succeed())
		config := newTestTradeBotConfig("watch-strategy-config")
		Expect(k8sClient.Create(ctx, config)).To(Succeed())

		tradeBot := newTestTradeBot("watch-strategy-bot", strategy.Name, config.Name, "trade")
		Expect(k8sClient.Create(ctx, tradeBot)).To(Succeed())
		cmKey := types.NamespacedName{Name: tradeBot.Name + "-strategy", Namespace: testNamespace}

		Eventually(func(g Gomega) {
			var cm corev1.ConfigMap
			g.Expect(k8sClient.Get(ctx, cmKey, &cm)).To(Succeed())
			g.Expect(cm.Data["SampleStrategy.py"]).To(Equal(validTestStrategyScript))
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

		updatedScript := validTestStrategyScript + "\n    # a harmless comment marking the update\n"
		var latestStrategy freqtradev1alpha1.Strategy
		strategyKey := types.NamespacedName{Name: strategy.Name, Namespace: testNamespace}
		Expect(k8sClient.Get(ctx, strategyKey, &latestStrategy)).To(Succeed())
		latestStrategy.Spec.Script = updatedScript
		Expect(k8sClient.Update(ctx, &latestStrategy)).To(Succeed())

		Eventually(func(g Gomega) {
			var cm corev1.ConfigMap
			g.Expect(k8sClient.Get(ctx, cmKey, &cm)).To(Succeed())
			g.Expect(cm.Data["SampleStrategy.py"]).To(Equal(updatedScript))
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
	})
})
