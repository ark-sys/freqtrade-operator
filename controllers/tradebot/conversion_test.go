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
	freqtradev1beta1 "github.com/ark-sys/freqtrade-operator/api/v1beta1"
)

// This suite exercises the P6-5 conversion webhook against a real
// apiserver, not just the Go-level fuzz round-trip tests in
// api/v1alpha1/tradebot_conversion_test.go: envtest's own CRDInstallOptions
// auto-detects conversion.Convertible types already registered in the
// scheme (the same mechanism controller-runtime's webhook builder uses to
// auto-register /convert - see api/v1alpha1/tradebot_webhook.go's
// SetupWebhookWithManager, unchanged by P6-5) and rewrites the installed
// CRD's spec.conversion to point at this suite's own local webhook
// server. Nothing beyond what P6-1 through P6-4 already wired needed to
// change for this to work.
var _ = Describe("TradeBot v1alpha1<->v1beta1 conversion (P6-5)", func() {
	It("serves a trade-mode TradeBot created as v1alpha1 correctly as v1beta1", func() {
		ctx := context.Background()
		strategy := newTestStrategy("strategy-conv")
		config := newTestTradeBotConfig("config-conv")
		Expect(k8sClient.Create(ctx, strategy)).To(Succeed())
		Expect(k8sClient.Create(ctx, config)).To(Succeed())

		alpha := newTestTradeBot("bot-conv", strategy.Name, config.Name, "trade")
		alpha.Spec.UpdateStrategy = updateStrategyAuto
		alpha.Spec.Introspection = &freqtradev1alpha1.IntrospectionSpec{Enabled: ptr.To(true)}
		Expect(k8sClient.Create(ctx, alpha)).To(Succeed())

		key := types.NamespacedName{Name: alpha.Name, Namespace: testNamespace}
		var beta freqtradev1beta1.TradeBot
		Expect(k8sClient.Get(ctx, key, &beta)).To(Succeed())

		Expect(beta.Spec.ConfigRef.Name).To(Equal(config.Name))
		Expect(beta.Spec.StrategyRef.Name).To(Equal(strategy.Name))
		Expect(beta.Spec.UpdateStrategy).To(Equal(updateStrategyAuto))
		Expect(beta.Spec.Introspection).NotTo(BeNil())
		Expect(beta.Spec.Introspection.Enabled).NotTo(BeNil())
		Expect(*beta.Spec.Introspection.Enabled).To(BeTrue())
	})

	It("rejects a Job-mode TradeBot outright rather than silently dropping its command", func() {
		ctx := context.Background()
		strategy := newTestStrategy("strategy-conv-job")
		config := newTestTradeBotConfig("config-conv-job")
		Expect(k8sClient.Create(ctx, strategy)).To(Succeed())
		Expect(k8sClient.Create(ctx, config)).To(Succeed())

		// v1beta1 is the storage version (+kubebuilder:storageversion), so
		// the apiserver must convert this v1alpha1 object to v1beta1 to
		// store it at all - meaning ConvertTo's rejection surfaces right
		// here, at Create, not on some later Get.
		alpha := newTestTradeBot("bot-conv-job", strategy.Name, config.Name, "backtesting")
		err := k8sClient.Create(ctx, alpha)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("Backtest"))
	})

	It("creates a trade-only TradeBot directly as v1beta1 and serves it back correctly as v1alpha1", func() {
		ctx := context.Background()
		strategy := newTestStrategy("strategy-conv-beta")
		config := newTestTradeBotConfig("config-conv-beta")
		Expect(k8sClient.Create(ctx, strategy)).To(Succeed())
		Expect(k8sClient.Create(ctx, config)).To(Succeed())

		beta := &freqtradev1beta1.TradeBot{
			ObjectMeta: metav1.ObjectMeta{Name: "bot-conv-beta", Namespace: testNamespace},
			Spec: freqtradev1beta1.TradeBotSpec{
				ConfigRef:   corev1.LocalObjectReference{Name: config.Name},
				StrategyRef: corev1.LocalObjectReference{Name: strategy.Name},
			},
		}
		Expect(k8sClient.Create(ctx, beta)).To(Succeed())

		key := types.NamespacedName{Name: beta.Name, Namespace: testNamespace}
		var alpha freqtradev1alpha1.TradeBot
		Expect(k8sClient.Get(ctx, key, &alpha)).To(Succeed())
		Expect(alpha.Spec.Config).To(Equal(config.Name))
		Expect(alpha.Spec.Strategy).To(Equal(strategy.Name))
		// A blank FreqtradeCommand is exactly what "trade" already means -
		// see ConvertFrom's own doc comment.
		Expect(alpha.Spec.FreqtradeCommand).To(BeEmpty())
	})
})
