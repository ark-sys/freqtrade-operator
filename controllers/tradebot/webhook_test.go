package tradebot

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
)

// This file covers the P1-4 admission webhooks: rejecting bad input
// synchronously at apply time instead of accepting it and failing later,
// asynchronously, in a Condition. Every other spec in this package creates
// only objects that pass these webhooks - if that ever stops being true,
// those specs fail loudly at object-creation time, which is a deliberate
// side effect: it's the regression check that the webhook isn't being
// bypassed for the fixtures this package already relies on.
var _ = Describe("TradeBot admission webhook (P1-4)", func() {
	It("rejects a TradeBot referencing a nonexistent TradeBotConfig", func() {
		strategy := newTestStrategy("wh-tb-strategy-1")
		Expect(k8sClient.Create(context.Background(), strategy)).To(Succeed())

		tradeBot := newTestTradeBot("wh-tb-missing-config", strategy.Name, "nonexistent-config", "trade")
		err := k8sClient.Create(context.Background(), tradeBot)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("TradeBotConfig"))
		Expect(err.Error()).To(ContainSubstring("not found"))
	})

	It("rejects a TradeBot referencing a nonexistent Strategy", func() {
		cfg := newTestTradeBotConfig("wh-tb-config-1")
		Expect(k8sClient.Create(context.Background(), cfg)).To(Succeed())

		tradeBot := newTestTradeBot("wh-tb-missing-strategy", "nonexistent-strategy", cfg.Name, "trade")
		err := k8sClient.Create(context.Background(), tradeBot)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("Strategy"))
		Expect(err.Error()).To(ContainSubstring("not found"))
	})

	It("rejects freqtrade_arguments that repeat an operator-owned flag", func() {
		strategy := newTestStrategy("wh-tb-strategy-2")
		Expect(k8sClient.Create(context.Background(), strategy)).To(Succeed())
		cfg := newTestTradeBotConfig("wh-tb-config-2")
		Expect(k8sClient.Create(context.Background(), cfg)).To(Succeed())

		tradeBot := newTestTradeBot("wh-tb-owned-flag", strategy.Name, cfg.Name, "trade")
		tradeBot.Spec.FreqtradeArguments = []string{"--strategy", "EvilStrategy"}
		err := k8sClient.Create(context.Background(), tradeBot)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("set by the operator"))
	})

	It("rejects freqtrade_arguments containing a shell metacharacter", func() {
		strategy := newTestStrategy("wh-tb-strategy-3")
		Expect(k8sClient.Create(context.Background(), strategy)).To(Succeed())
		cfg := newTestTradeBotConfig("wh-tb-config-3")
		Expect(k8sClient.Create(context.Background(), cfg)).To(Succeed())

		tradeBot := newTestTradeBot("wh-tb-shell-meta", strategy.Name, cfg.Name, "trade")
		tradeBot.Spec.FreqtradeArguments = []string{"--timerange=20240101-;rm -rf /"}
		err := k8sClient.Create(context.Background(), tradeBot)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("shell metacharacter"))
	})

	// P4-3: freqtrade's REST API isn't built for tight polling loops, and
	// this operator is not a market-data source - a typo'd "6s" (meant
	// "60s") must be rejected before it becomes a de facto load test.
	It("rejects spec.introspection.interval under 10s", func() {
		strategy := newTestStrategy("wh-tb-strategy-interval")
		Expect(k8sClient.Create(context.Background(), strategy)).To(Succeed())
		cfg := newTestTradeBotConfig("wh-tb-config-interval")
		Expect(k8sClient.Create(context.Background(), cfg)).To(Succeed())

		tradeBot := newTestTradeBot("wh-tb-interval-too-short", strategy.Name, cfg.Name, "trade")
		tradeBot.Spec.Introspection = &freqtradev1alpha1.IntrospectionSpec{
			Interval: metav1.Duration{Duration: 5 * time.Second},
		}
		err := k8sClient.Create(context.Background(), tradeBot)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("spec.introspection.interval"))
	})

	It("accepts spec.introspection.interval at or above 10s", func() {
		strategy := newTestStrategy("wh-tb-strategy-interval-ok")
		Expect(k8sClient.Create(context.Background(), strategy)).To(Succeed())
		cfg := newTestTradeBotConfig("wh-tb-config-interval-ok")
		Expect(k8sClient.Create(context.Background(), cfg)).To(Succeed())

		tradeBot := newTestTradeBot("wh-tb-interval-ok", strategy.Name, cfg.Name, "trade")
		tradeBot.Spec.Introspection = &freqtradev1alpha1.IntrospectionSpec{
			Interval: metav1.Duration{Duration: 10 * time.Second},
		}
		Expect(k8sClient.Create(context.Background(), tradeBot)).To(Succeed())
	})
})

var _ = Describe("TradeBotConfig admission webhook (P1-4)", func() {
	It("rejects a TradeBotConfig with no exchange section", func() {
		cfg := &freqtradev1alpha1.TradeBotConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "wh-tbc-no-exchange", Namespace: testNamespace},
			Spec:       freqtradev1alpha1.TradeBotConfigSpec{Bot: &freqtradev1alpha1.BotConfig{DryRun: ptr.To(true)}},
		}
		err := k8sClient.Create(context.Background(), cfg)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("spec.exchange is required"))
	})

	It("rejects a live (non-dry-run) config with no credentials source", func() {
		cfg := &freqtradev1alpha1.TradeBotConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "wh-tbc-live-no-secret", Namespace: testNamespace},
			Spec: freqtradev1alpha1.TradeBotConfigSpec{
				Bot:      &freqtradev1alpha1.BotConfig{DryRun: ptr.To(false)},
				Exchange: &freqtradev1alpha1.ExchangeSpec{Name: "binance"},
			},
		}
		err := k8sClient.Create(context.Background(), cfg)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("secretRef is required"))
	})

	It("rejects a config whose dry_run is unset, not just one explicitly false", func() {
		cfg := &freqtradev1alpha1.TradeBotConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "wh-tbc-unset-dry-run", Namespace: testNamespace},
			Spec: freqtradev1alpha1.TradeBotConfigSpec{
				Bot:      &freqtradev1alpha1.BotConfig{},
				Exchange: &freqtradev1alpha1.ExchangeSpec{Name: "binance"},
			},
		}
		err := k8sClient.Create(context.Background(), cfg)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("secretRef is required"))
	})

	It("rejects a secretRef pointing at a Secret that doesn't exist", func() {
		cfg := &freqtradev1alpha1.TradeBotConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "wh-tbc-missing-secret", Namespace: testNamespace},
			Spec: freqtradev1alpha1.TradeBotConfigSpec{
				Bot:      &freqtradev1alpha1.BotConfig{DryRun: ptr.To(false)},
				Exchange: &freqtradev1alpha1.ExchangeSpec{Name: "binance", SecretRef: "nonexistent-secret"},
			},
		}
		err := k8sClient.Create(context.Background(), cfg)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(`Secret "nonexistent-secret" not found`))
	})

	It("rejects a secretRef Secret with none of the expected credential keys", func() {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "wh-tbc-empty-secret", Namespace: testNamespace},
			Data:       map[string][]byte{"unrelated-key": []byte("value")},
		}
		Expect(k8sClient.Create(context.Background(), secret)).To(Succeed())

		cfg := &freqtradev1alpha1.TradeBotConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "wh-tbc-bad-secret", Namespace: testNamespace},
			Spec: freqtradev1alpha1.TradeBotConfigSpec{
				Bot:      &freqtradev1alpha1.BotConfig{DryRun: ptr.To(false)},
				Exchange: &freqtradev1alpha1.ExchangeSpec{Name: "binance", SecretRef: secret.Name},
			},
		}
		err := k8sClient.Create(context.Background(), cfg)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("none of the expected credential keys"))
	})

	It("accepts a dry-run config with no secretRef", func() {
		cfg := &freqtradev1alpha1.TradeBotConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "wh-tbc-paper-trading", Namespace: testNamespace},
			Spec: freqtradev1alpha1.TradeBotConfigSpec{
				Bot:      &freqtradev1alpha1.BotConfig{DryRun: ptr.To(true)},
				Exchange: &freqtradev1alpha1.ExchangeSpec{Name: "binance"},
			},
		}
		Expect(k8sClient.Create(context.Background(), cfg)).To(Succeed())
	})

	It("accepts a live config with a secretRef Secret carrying a recognized credential key", func() {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "wh-tbc-good-secret", Namespace: testNamespace},
			Data:       map[string][]byte{"api-key": []byte("value")},
		}
		Expect(k8sClient.Create(context.Background(), secret)).To(Succeed())

		cfg := &freqtradev1alpha1.TradeBotConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "wh-tbc-live-good", Namespace: testNamespace},
			Spec: freqtradev1alpha1.TradeBotConfigSpec{
				Bot:      &freqtradev1alpha1.BotConfig{DryRun: ptr.To(false)},
				Exchange: &freqtradev1alpha1.ExchangeSpec{Name: "binance", SecretRef: secret.Name},
			},
		}
		Expect(k8sClient.Create(context.Background(), cfg)).To(Succeed())
	})

	It("rejects a deprecated plaintext exchange.key without the allow-plaintext-credentials annotation", func() {
		cfg := &freqtradev1alpha1.TradeBotConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "wh-tbc-plaintext-key", Namespace: testNamespace},
			Spec: freqtradev1alpha1.TradeBotConfigSpec{
				Bot:      &freqtradev1alpha1.BotConfig{DryRun: ptr.To(true)},
				Exchange: &freqtradev1alpha1.ExchangeSpec{Name: "binance", Key: "plaintext-api-key"},
			},
		}
		err := k8sClient.Create(context.Background(), cfg)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("spec.exchange.key"))
		Expect(err.Error()).To(ContainSubstring("freqtrade.io/allow-plaintext-credentials"))
	})

	It("rejects a deprecated plaintext apiServer.password and names it alongside other plaintext fields", func() {
		cfg := &freqtradev1alpha1.TradeBotConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "wh-tbc-plaintext-multi", Namespace: testNamespace},
			Spec: freqtradev1alpha1.TradeBotConfigSpec{
				Bot:       &freqtradev1alpha1.BotConfig{DryRun: ptr.To(true)},
				Exchange:  &freqtradev1alpha1.ExchangeSpec{Name: "binance"},
				APIServer: &freqtradev1alpha1.APIServerConfig{Password: "plaintext-password"},
			},
		}
		err := k8sClient.Create(context.Background(), cfg)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("spec.apiServer.password"))
	})

	It("accepts a deprecated plaintext credential field when allow-plaintext-credentials is set", func() {
		cfg := &freqtradev1alpha1.TradeBotConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "wh-tbc-plaintext-allowed",
				Namespace:   testNamespace,
				Annotations: map[string]string{"freqtrade.io/allow-plaintext-credentials": "true"},
			},
			Spec: freqtradev1alpha1.TradeBotConfigSpec{
				Bot:      &freqtradev1alpha1.BotConfig{DryRun: ptr.To(true)},
				Exchange: &freqtradev1alpha1.ExchangeSpec{Name: "binance", Key: "plaintext-api-key"},
			},
		}
		Expect(k8sClient.Create(context.Background(), cfg)).To(Succeed())
	})

	It("does not reject account_id, which isn't a credential", func() {
		cfg := &freqtradev1alpha1.TradeBotConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "wh-tbc-account-id", Namespace: testNamespace},
			Spec: freqtradev1alpha1.TradeBotConfigSpec{
				Bot:      &freqtradev1alpha1.BotConfig{DryRun: ptr.To(true)},
				Exchange: &freqtradev1alpha1.ExchangeSpec{Name: "binance", AccountID: "12345"},
			},
		}
		Expect(k8sClient.Create(context.Background(), cfg)).To(Succeed())
	})
})

var _ = Describe("Strategy admission webhook (P1-4)", func() {
	It("rejects a Strategy with an invalid Python class name", func() {
		strategy := &freqtradev1alpha1.Strategy{
			ObjectMeta: metav1.ObjectMeta{Name: "wh-strategy-bad-name", Namespace: testNamespace},
			Spec:       freqtradev1alpha1.StrategySpec{Name: "2024Strategy", Script: validTestStrategyScript},
		}
		err := k8sClient.Create(context.Background(), strategy)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("not a valid Python class name"))
	})

	It("rejects a Strategy script missing a required method", func() {
		strategy := &freqtradev1alpha1.Strategy{
			ObjectMeta: metav1.ObjectMeta{Name: "wh-strategy-bad-script", Namespace: testNamespace},
			Spec: freqtradev1alpha1.StrategySpec{
				Name:   "SampleStrategy",
				Script: "import freqtrade\nclass SampleStrategy:\n    def populate_indicators(self): pass",
			},
		}
		err := k8sClient.Create(context.Background(), strategy)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("populate_entry_trend"))
	})
})
