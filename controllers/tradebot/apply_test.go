package tradebot

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
)

// This file covers P2-1: replacing the hand-rolled ApplyX functions (get ->
// reflect.DeepEqual a hand-picked field subset -> Update) with server-side
// apply. The bug that motivated the change was that those comparisons were
// always false once the API server had defaulted fields the freshly-built
// desired object never mentions, so every reconcile issued a pointless
// Update - for a StatefulSet, a potential rolling restart of a live trading
// bot. These specs are the plan's own suggested acceptance test: reconcile
// twice with no spec change, and prove nothing was written the second time.
var _ = Describe("server-side apply (P2-1)", func() {
	const eventuallyTimeout = "15s"
	const eventuallyPoll = "250ms"

	It("does not touch the StatefulSet's resourceVersion on a second reconcile with no spec change", func() {
		ctx := context.Background()
		strategy := newTestStrategy("ssa-sts-strategy")
		Expect(k8sClient.Create(ctx, strategy)).To(Succeed())
		config := newTestTradeBotConfig("ssa-sts-config")
		Expect(k8sClient.Create(ctx, config)).To(Succeed())

		tradeBot := newTestTradeBot("ssa-sts-bot", strategy.Name, config.Name, "trade")
		Expect(k8sClient.Create(ctx, tradeBot)).To(Succeed())
		key := types.NamespacedName{Name: tradeBot.Name, Namespace: testNamespace}

		var firstResourceVersion string
		Eventually(func(g Gomega) {
			var sts appsv1.StatefulSet
			g.Expect(k8sClient.Get(ctx, key, &sts)).To(Succeed())
			firstResourceVersion = sts.ResourceVersion
			g.Expect(firstResourceVersion).NotTo(BeEmpty())
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

		// A direct second Reconcile call - rather than waiting on the
		// background controller's own 15s not-ready requeue - is what makes
		// "reconcile again with nothing changed in between" deterministic
		// and fast; the outcome being tested (no write happens) doesn't
		// depend on which one triggered it.
		_, err := testReconciler.Reconcile(ctx, ctrl.Request{NamespacedName: key})
		Expect(err).NotTo(HaveOccurred())

		var sts appsv1.StatefulSet
		Expect(k8sClient.Get(ctx, key, &sts)).To(Succeed())
		Expect(sts.ResourceVersion).To(Equal(firstResourceVersion),
			"expected server-side apply to be a no-op on an unchanged StatefulSet, not issue a spurious Update")
	})

	It("sets an owner reference on the applied StatefulSet even though BuildStatefulSet no longer does", func() {
		ctx := context.Background()
		strategy := newTestStrategy("ssa-owner-strategy")
		Expect(k8sClient.Create(ctx, strategy)).To(Succeed())
		config := newTestTradeBotConfig("ssa-owner-config")
		Expect(k8sClient.Create(ctx, config)).To(Succeed())

		tradeBot := newTestTradeBot("ssa-owner-bot", strategy.Name, config.Name, "trade")
		Expect(k8sClient.Create(ctx, tradeBot)).To(Succeed())
		key := types.NamespacedName{Name: tradeBot.Name, Namespace: testNamespace}

		Eventually(func(g Gomega) {
			var sts appsv1.StatefulSet
			g.Expect(k8sClient.Get(ctx, key, &sts)).To(Succeed())
			g.Expect(sts.OwnerReferences).To(HaveLen(1))
			g.Expect(sts.OwnerReferences[0].Name).To(Equal(tradeBot.Name))
			g.Expect(sts.OwnerReferences[0].Controller).NotTo(BeNil())
			g.Expect(*sts.OwnerReferences[0].Controller).To(BeTrue())
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
	})

	It("surfaces a real spec.template drift on a Job-mode TradeBot as WorkloadImmutable via a genuine reconcile", func() {
		ctx := context.Background()
		strategy := newTestStrategy("ssa-job-drift-strategy")
		Expect(k8sClient.Create(ctx, strategy)).To(Succeed())
		config := newTestTradeBotConfig("ssa-job-drift-config")
		Expect(k8sClient.Create(ctx, config)).To(Succeed())

		tradeBot := newTestTradeBot("ssa-job-drift-bot", strategy.Name, config.Name, "backtesting")
		Expect(k8sClient.Create(ctx, tradeBot)).To(Succeed())
		key := types.NamespacedName{Name: tradeBot.Name, Namespace: testNamespace}

		Eventually(func(g Gomega) {
			var latest freqtradev1alpha1.TradeBot
			g.Expect(k8sClient.Get(ctx, key, &latest)).To(Succeed())
			cond := findStatusCondition(latest.Status.Conditions, freqtradev1alpha1.ConditionWorkloadImmutable)
			g.Expect(cond).NotTo(BeNil())
			g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

		// Change something that reaches the Job's immutable spec.template -
		// freqtrade_arguments flow straight into the container args BuildJob
		// builds. This update itself matches mainResourcePredicate (it's a
		// spec change), so the background controller reconciles it on its
		// own; a direct extra Reconcile call here would race that one
		// (nothing serializes the two, since MaxConcurrentReconciles only
		// governs the manager's own work queue), so this waits for it via
		// Eventually instead, like every other spec in this suite does.
		var latest freqtradev1alpha1.TradeBot
		Expect(k8sClient.Get(ctx, key, &latest)).To(Succeed())
		latest.Spec.FreqtradeArguments = []string{"--timerange", "20240101-20240201"}
		Expect(k8sClient.Update(ctx, &latest)).To(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, key, &latest)).To(Succeed())
			cond := findStatusCondition(latest.Status.Conditions, freqtradev1alpha1.ConditionWorkloadImmutable)
			g.Expect(cond).NotTo(BeNil())
			g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(cond.Reason).To(Equal(freqtradev1alpha1.ReasonSpecChangeIgnored))
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
	})
})
