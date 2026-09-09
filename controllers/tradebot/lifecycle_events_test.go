package tradebot

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
)

// hasEvent reports whether events contains one for objName with the given
// reason - the shared assertion every spec in this file needs, since
// EventRecorder has no query API of its own: envtest persists each emitted
// Event as a real object, listable like anything else (P4-1).
func hasEvent(events *corev1.EventList, objName, reason string) bool {
	for _, e := range events.Items {
		if e.InvolvedObject.Name == objName && e.Reason == reason {
			return true
		}
	}
	return false
}

// This file covers P4-1: the plan's own list of notable TradeBot lifecycle
// moments an EventRecorder should surface - "config rendered, workload
// created/updated, ... secret missing, bot became ready, config drift
// detected, ...". ("restart triggered" is recordConfigRestartEvent, P2-4,
// already covered; "finalizer timeout" is finalizers_test.go's fake-client
// spec, faster than waiting out a real grace period here.)
var _ = Describe("TradeBot lifecycle Events (P4-1)", func() {
	const eventuallyTimeout = "15s"
	const eventuallyPoll = "250ms"

	It("records WorkloadCreated and ConfigRendered on the first successful reconcile", func() {
		ctx := context.Background()
		strategy := newTestStrategy("events-created-strategy")
		Expect(k8sClient.Create(ctx, strategy)).To(Succeed())
		config := newTestTradeBotConfig("events-created-config")
		Expect(k8sClient.Create(ctx, config)).To(Succeed())

		tradeBot := newTestTradeBot("events-created-bot", strategy.Name, config.Name, "trade")
		Expect(k8sClient.Create(ctx, tradeBot)).To(Succeed())

		Eventually(func(g Gomega) {
			var events corev1.EventList
			g.Expect(k8sClient.List(ctx, &events, client.InNamespace(testNamespace))).To(Succeed())
			g.Expect(hasEvent(&events, tradeBot.Name, "WorkloadCreated")).To(BeTrue(), "expected a WorkloadCreated event")
			g.Expect(hasEvent(&events, tradeBot.Name, "ConfigRendered")).To(BeTrue(), "expected a ConfigRendered event")
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
	})

	// StatefulSet readiness never happens organically under envtest (no
	// kubelet runs there), so this patches Status directly - the same
	// simulate-what-a-real-controller-would-eventually-do pattern
	// frequi_controller_test.go uses for its own WorkloadReady spec - and
	// calls Reconcile directly rather than waiting on the 15s poll interval
	// computeWorkloadStatus falls back to.
	It("records a BotReady event once the StatefulSet becomes ready", func() {
		ctx := context.Background()
		strategy := newTestStrategy("events-ready-strategy")
		Expect(k8sClient.Create(ctx, strategy)).To(Succeed())
		config := newTestTradeBotConfig("events-ready-config")
		Expect(k8sClient.Create(ctx, config)).To(Succeed())

		tradeBot := newTestTradeBot("events-ready-bot", strategy.Name, config.Name, "trade")
		Expect(k8sClient.Create(ctx, tradeBot)).To(Succeed())
		key := types.NamespacedName{Name: tradeBot.Name, Namespace: testNamespace}

		var sts appsv1.StatefulSet
		Eventually(func() error {
			return k8sClient.Get(ctx, key, &sts)
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

		sts.Status.Replicas = 1
		sts.Status.ReadyReplicas = 1
		Expect(k8sClient.Status().Update(ctx, &sts)).To(Succeed())

		_, err := testReconciler.Reconcile(ctx, ctrl.Request{NamespacedName: key})
		Expect(err).NotTo(HaveOccurred())

		Eventually(func(g Gomega) {
			var events corev1.EventList
			g.Expect(k8sClient.List(ctx, &events, client.InNamespace(testNamespace))).To(Succeed())
			g.Expect(hasEvent(&events, tradeBot.Name, "BotReady")).To(BeTrue(), "expected a BotReady event")
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
	})

	It("records a ConfigDrift warning event for a Manual-mode bot whose rendered config changed", func() {
		ctx := context.Background()
		strategy := newTestStrategy("events-drift-strategy")
		Expect(k8sClient.Create(ctx, strategy)).To(Succeed())
		config := newTestTradeBotConfig("events-drift-config")
		Expect(k8sClient.Create(ctx, config)).To(Succeed())

		tradeBot := newTestTradeBot("events-drift-bot", strategy.Name, config.Name, "trade")
		Expect(k8sClient.Create(ctx, tradeBot)).To(Succeed())
		key := types.NamespacedName{Name: tradeBot.Name, Namespace: testNamespace}

		Eventually(func(g Gomega) {
			var sts appsv1.StatefulSet
			g.Expect(k8sClient.Get(ctx, key, &sts)).To(Succeed())
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

		updateStakeAmount(ctx, config.Name, "300")

		Eventually(func(g Gomega) {
			var events corev1.EventList
			g.Expect(k8sClient.List(ctx, &events, client.InNamespace(testNamespace))).To(Succeed())
			g.Expect(hasEvent(&events, tradeBot.Name, "ConfigDrift")).To(BeTrue(), "expected a ConfigDrift warning event")
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
	})

	// A TradeBot can never be created referencing a nonexistent Strategy -
	// the P1-4 admission webhook rejects that at apply time (see
	// tradebot_controller_test.go's "deleting a TradeBot" section for the
	// full explanation) - but the Strategy it references can still be
	// deleted afterward, which is what this reproduces.
	It("records a Warning event via failReconcile when a referenced Strategy is deleted", func() {
		ctx := context.Background()
		strategy := newTestStrategy("events-missing-ref-strategy")
		Expect(k8sClient.Create(ctx, strategy)).To(Succeed())
		config := newTestTradeBotConfig("events-missing-ref-config")
		Expect(k8sClient.Create(ctx, config)).To(Succeed())

		tradeBot := newTestTradeBot("events-missing-ref-bot", strategy.Name, config.Name, "trade")
		Expect(k8sClient.Create(ctx, tradeBot)).To(Succeed())
		key := types.NamespacedName{Name: tradeBot.Name, Namespace: testNamespace}

		Eventually(func(g Gomega) {
			var latest freqtradev1alpha1.TradeBot
			g.Expect(k8sClient.Get(ctx, key, &latest)).To(Succeed())
			g.Expect(latest.Status.ResolvedImage).NotTo(BeEmpty())
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

		Expect(k8sClient.Delete(ctx, strategy)).To(Succeed())

		Eventually(func(g Gomega) {
			var events corev1.EventList
			g.Expect(k8sClient.List(ctx, &events, client.InNamespace(testNamespace))).To(Succeed())
			g.Expect(hasEvent(&events, tradeBot.Name, freqtradev1alpha1.ReasonReferenceNotFound)).To(BeTrue(),
				"expected a ReferenceNotFound warning event")
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
	})
})
