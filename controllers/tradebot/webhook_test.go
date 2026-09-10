package tradebot

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

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

	// Regression test for a real deadlock found via e2e testing (P5-3): deleting a whole
	// namespace tears down objects in no guaranteed order, so the Strategy this TradeBot
	// references can already be gone by the time the reconciler tries to strip its finalizer -
	// which is itself an Update, and ValidateUpdate used to re-validate spec.strategy's existence
	// on every update, including that one. Without the DeletionTimestamp skip
	// (api/v1alpha1/tradebot_webhook.go), that Update is permanently rejected: the finalizer can
	// never be removed, so the TradeBot - and its whole namespace - never finishes terminating.
	// Reproduced directly against a real kind cluster before this fix existed (a `kubectl delete
	// ns` wedged for 10+ minutes); this proves the same sequence resolves against envtest's real
	// admission chain, not just that the reconciler's own Go code path is reachable.
	It("still lets the reconciler strip the finalizer after its Strategy is deleted first", func() {
		strategy := newTestStrategy("wh-tb-strategy-deadlock")
		Expect(k8sClient.Create(context.Background(), strategy)).To(Succeed())
		cfg := newTestTradeBotConfig("wh-tb-config-deadlock")
		Expect(k8sClient.Create(context.Background(), cfg)).To(Succeed())
		tradeBot := newTestTradeBot("wh-tb-deadlock", strategy.Name, cfg.Name, "trade")
		Expect(k8sClient.Create(context.Background(), tradeBot)).To(Succeed())

		By("waiting for the reconciler to add its finalizer")
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(tradeBot), tradeBot)).To(Succeed())
			g.Expect(tradeBot.Finalizers).To(ContainElement(BotFinalizer))
		}).Should(Succeed())

		By("deleting the referenced Strategy first")
		Expect(k8sClient.Delete(context.Background(), strategy)).To(Succeed())

		By("deleting the TradeBot - it must still fully terminate, not wedge")
		Expect(k8sClient.Delete(context.Background(), tradeBot)).To(Succeed())
		Eventually(func(g Gomega) {
			err := k8sClient.Get(context.Background(), client.ObjectKeyFromObject(tradeBot), tradeBot)
			g.Expect(errors.IsNotFound(err)).To(BeTrue(), "TradeBot should be fully deleted, not stuck with a finalizer")
		}).Should(Succeed())
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

	// B2 made TradeBotConfig multi-version with v1beta1 as storage, and
	// v1beta1 has no field for any plaintext credential at all - every
	// create/update of an object with one set gets converted to v1beta1
	// before it can be persisted (regardless of which version submitted
	// it, or which admission webhook runs), and api/v1alpha1/
	// tradebotconfig_conversion.go's ConvertTo rejects that unconditionally.
	// A validating webhook registered for v1beta1 uses the default
	// Equivalent matchPolicy, so it applies to this v1alpha1 write too -
	// converting this object to check it is what actually fails here, with
	// ConvertTo's own message, before v1alpha1's own admission-level
	// plaintext-credential check (below) even gets a chance to produce its
	// annotation-aware one.
	It("rejects a deprecated plaintext exchange.key (via conversion, since v1beta1 has no field for it)", func() {
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
		Expect(err.Error()).To(ContainSubstring("secretRef"))
	})

	// v1alpha1's own admission-level gate (api/v1alpha1/tradebotconfig_webhook.go)
	// still runs and still rejects a plaintext credential with no
	// allow-plaintext-credentials annotation - this covers that path
	// specifically, in isolation from the conversion-level rejection the
	// test above actually hits first for a plain k8sClient.Create. A direct
	// ValidateCreate call is the only way to observe v1alpha1's own message
	// now, since any real write also has to survive conversion.
	It("v1alpha1's own admission webhook independently rejects the same plaintext field", func() {
		cfg := &freqtradev1alpha1.TradeBotConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "wh-tbc-plaintext-key-direct", Namespace: testNamespace},
			Spec: freqtradev1alpha1.TradeBotConfigSpec{
				Bot:      &freqtradev1alpha1.BotConfig{DryRun: ptr.To(true)},
				Exchange: &freqtradev1alpha1.ExchangeSpec{Name: "binance", Key: "plaintext-api-key"},
			},
		}
		validator := &freqtradev1alpha1.TradeBotConfigCustomValidator{}
		_, err := validator.ValidateCreate(context.Background(), cfg)
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

	// Before B2, allow-plaintext-credentials let this create fully succeed
	// (v1alpha1 was storage - nothing else needed to represent the value).
	// Now it only gets the object past v1alpha1's own admission check -
	// see the "rejects a deprecated plaintext exchange.key" test above's
	// doc comment for why this still can't ever be persisted. The
	// annotation is not a dead letter, though: it changes *which* rejection
	// the caller sees (conversion's, either way - v1alpha1's own check no
	// longer fires at all once it's satisfied) and it still triggers
	// warnPlaintextCredentialsUsed's audit Event before conversion fails.
	It("still fails to create with allow-plaintext-credentials set - v1beta1 storage can't represent it either way",
		func() {
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
			err := k8sClient.Create(context.Background(), cfg)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("spec.exchange.key"))
		})

	// The annotation's continued, real effect: v1alpha1's own admission
	// webhook (in isolation - see the test above for why a real Create
	// can't observe this directly anymore) no longer rejects when it's set.
	It("lets v1alpha1's own admission webhook admit a plaintext field when allow-plaintext-credentials is set", func() {
		cfg := &freqtradev1alpha1.TradeBotConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "wh-tbc-plaintext-allowed-direct",
				Namespace:   testNamespace,
				Annotations: map[string]string{"freqtrade.io/allow-plaintext-credentials": "true"},
			},
			Spec: freqtradev1alpha1.TradeBotConfigSpec{
				Bot:      &freqtradev1alpha1.BotConfig{DryRun: ptr.To(true)},
				Exchange: &freqtradev1alpha1.ExchangeSpec{Name: "binance", Key: "plaintext-api-key"},
			},
		}
		validator := &freqtradev1alpha1.TradeBotConfigCustomValidator{}
		_, err := validator.ValidateCreate(context.Background(), cfg)
		Expect(err).NotTo(HaveOccurred())
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
