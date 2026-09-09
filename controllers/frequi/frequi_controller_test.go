package frequi

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
)

const testNamespace = "default"
const eventuallyTimeout = "15s"
const eventuallyPoll = "250ms"

// This covers P2-5's acceptance case: FreqUISpec.TradeBotRefs is a list of
// bare names with no existence check, so a typo used to silently yield no
// CORS entry for that bot with nothing visible anywhere. It's now surfaced
// as the TradeBotRefsResolved condition instead.
var _ = Describe("FreqUI TradeBotRefsResolved condition (P2-5)", func() {
	It("is True when TradeBotRefs is empty", func() {
		ctx := context.Background()
		frequi := &freqtradev1alpha1.FreqUI{
			ObjectMeta: metav1.ObjectMeta{Name: "no-refs", Namespace: testNamespace},
		}
		Expect(k8sClient.Create(ctx, frequi)).To(Succeed())
		key := types.NamespacedName{Name: frequi.Name, Namespace: testNamespace}

		Eventually(func(g Gomega) {
			var latest freqtradev1alpha1.FreqUI
			g.Expect(k8sClient.Get(ctx, key, &latest)).To(Succeed())
			cond := meta.FindStatusCondition(latest.Status.Conditions, freqtradev1alpha1.ConditionTradeBotRefsResolved)
			g.Expect(cond).NotTo(BeNil())
			g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
	})

	It("is False and names the ref when a TradeBotRef doesn't match any TradeBot", func() {
		ctx := context.Background()
		frequi := &freqtradev1alpha1.FreqUI{
			ObjectMeta: metav1.ObjectMeta{Name: "typo-ref", Namespace: testNamespace},
			Spec:       freqtradev1alpha1.FreqUISpec{TradeBotRefs: []string{"does-not-exist"}},
		}
		Expect(k8sClient.Create(ctx, frequi)).To(Succeed())
		key := types.NamespacedName{Name: frequi.Name, Namespace: testNamespace}

		Eventually(func(g Gomega) {
			var latest freqtradev1alpha1.FreqUI
			g.Expect(k8sClient.Get(ctx, key, &latest)).To(Succeed())
			cond := meta.FindStatusCondition(latest.Status.Conditions, freqtradev1alpha1.ConditionTradeBotRefsResolved)
			g.Expect(cond).NotTo(BeNil())
			g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(cond.Reason).To(Equal(freqtradev1alpha1.ReasonUnresolvableTradeBotRefs))
			g.Expect(cond.Message).To(ContainSubstring("does-not-exist"))
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

		// A typo must not stop FreqUI itself from deploying.
		var latest freqtradev1alpha1.FreqUI
		Expect(k8sClient.Get(ctx, key, &latest)).To(Succeed())
		readyCond := meta.FindStatusCondition(latest.Status.Conditions, freqtradev1alpha1.ConditionReady)
		Expect(readyCond).NotTo(BeNil())
		Expect(readyCond.Reason).NotTo(Equal(freqtradev1alpha1.ReasonReconcileError))
	})

	It("is True once a previously-unresolvable ref is created", func() {
		ctx := context.Background()
		frequi := &freqtradev1alpha1.FreqUI{
			ObjectMeta: metav1.ObjectMeta{Name: "ref-arrives-late", Namespace: testNamespace},
			Spec:       freqtradev1alpha1.FreqUISpec{TradeBotRefs: []string{"bot-arrives-late"}},
		}
		Expect(k8sClient.Create(ctx, frequi)).To(Succeed())
		key := types.NamespacedName{Name: frequi.Name, Namespace: testNamespace}

		Eventually(func(g Gomega) {
			var latest freqtradev1alpha1.FreqUI
			g.Expect(k8sClient.Get(ctx, key, &latest)).To(Succeed())
			cond := meta.FindStatusCondition(latest.Status.Conditions, freqtradev1alpha1.ConditionTradeBotRefsResolved)
			g.Expect(cond).NotTo(BeNil())
			g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

		tradeBot := &freqtradev1alpha1.TradeBot{
			ObjectMeta: metav1.ObjectMeta{Name: "bot-arrives-late", Namespace: testNamespace},
			Spec:       freqtradev1alpha1.TradeBotSpec{Config: "irrelevant", Strategy: "irrelevant"},
		}
		Expect(k8sClient.Create(ctx, tradeBot)).To(Succeed())

		Eventually(func(g Gomega) {
			var latest freqtradev1alpha1.FreqUI
			g.Expect(k8sClient.Get(ctx, key, &latest)).To(Succeed())
			cond := meta.FindStatusCondition(latest.Status.Conditions, freqtradev1alpha1.ConditionTradeBotRefsResolved)
			g.Expect(cond).NotTo(BeNil())
			g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
	})
})

// P4-1: WorkloadReady is a status Deployment change, which
// ownedResourcePredicate (setup.go) deliberately filters out of the Owns()
// watch - the controller relies on RequeueAfter polling to notice it in a
// real cluster, not a watch event. A direct Reconcile call here is what
// makes "became ready" observable in a test without an envtest kubelet
// (which never runs one) or a real wall-clock wait for that poll.
var _ = Describe("FreqUI lifecycle Events (P4-1)", func() {
	It("records a WorkloadReady event when the Deployment transitions to ready", func() {
		ctx := context.Background()
		frequi := &freqtradev1alpha1.FreqUI{
			ObjectMeta: metav1.ObjectMeta{Name: "events-ready", Namespace: testNamespace},
		}
		Expect(k8sClient.Create(ctx, frequi)).To(Succeed())
		key := types.NamespacedName{Name: frequi.Name, Namespace: testNamespace}

		var deployment appsv1.Deployment
		Eventually(func() error {
			return k8sClient.Get(ctx, key, &deployment)
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

		deployment.Status.Replicas = 1
		deployment.Status.ReadyReplicas = 1
		deployment.Status.AvailableReplicas = 1
		Expect(k8sClient.Status().Update(ctx, &deployment)).To(Succeed())

		_, err := testReconciler.Reconcile(ctx, ctrl.Request{NamespacedName: key})
		Expect(err).NotTo(HaveOccurred())

		Eventually(func(g Gomega) {
			var events corev1.EventList
			g.Expect(k8sClient.List(ctx, &events, client.InNamespace(testNamespace))).To(Succeed())
			found := false
			for _, e := range events.Items {
				if e.InvolvedObject.Name == frequi.Name && e.Reason == "WorkloadReady" {
					found = true
				}
			}
			g.Expect(found).To(BeTrue(), "expected a WorkloadReady event for %s", frequi.Name)
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
	})
})
