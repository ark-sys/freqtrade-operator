package tradebotconfig

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
)

const testNamespace = "default"

const eventuallyTimeout = "15s"
const eventuallyPoll = "250ms"

func ptrBool(b bool) *bool { return &b }

// A5 (REMAINING-WORK.md): SetupWithManager's For() predicate used to be a
// hand-written predicate.Funcs doing reflect.DeepEqual on Spec, replaced
// here with predicate.GenerationChangedPredicate{}. This proves a spec
// change still reaches Reconcile - shared.PatchStatus stamps
// Status.ObservedGeneration to the object's current generation on every
// reconcile that completes, so ObservedGeneration catching up to the new
// generation after a spec edit is direct evidence the predicate let the
// Update event through, not just that the initial Create did.
var _ = Describe("TradeBotConfig controller", func() {
	It("still reconciles a spec change (A5 predicate modernization)", func() {
		ctx := context.Background()
		tradeBotConfig := &freqtradev1alpha1.TradeBotConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "predicate-check", Namespace: testNamespace},
			Spec: freqtradev1alpha1.TradeBotConfigSpec{
				Bot:      &freqtradev1alpha1.BotConfig{BotName: "predicate-check", DryRun: ptrBool(true)},
				Exchange: &freqtradev1alpha1.ExchangeSpec{Name: "binance"},
			},
		}
		Expect(k8sClient.Create(ctx, tradeBotConfig)).To(Succeed())
		key := types.NamespacedName{Name: tradeBotConfig.Name, Namespace: tradeBotConfig.Namespace}

		Eventually(func(g Gomega) {
			var latest freqtradev1alpha1.TradeBotConfig
			g.Expect(k8sClient.Get(ctx, key, &latest)).To(Succeed())
			g.Expect(latest.Status.ObservedGeneration).To(Equal(latest.Generation))
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

		var toUpdate freqtradev1alpha1.TradeBotConfig
		Expect(k8sClient.Get(ctx, key, &toUpdate)).To(Succeed())
		toUpdate.Spec.Bot.BotName = "predicate-check-renamed"
		Expect(k8sClient.Update(ctx, &toUpdate)).To(Succeed())
		Expect(toUpdate.Generation).To(BeNumerically(">", 1), "expected the spec edit to bump generation")

		Eventually(func(g Gomega) {
			var latest freqtradev1alpha1.TradeBotConfig
			g.Expect(k8sClient.Get(ctx, key, &latest)).To(Succeed())
			g.Expect(latest.Status.ObservedGeneration).To(Equal(latest.Generation),
				"expected the reconciler to observe the new generation after the spec change")
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
	})
})
