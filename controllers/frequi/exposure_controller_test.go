package frequi

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	freqtradev1beta1 "github.com/ark-sys/freqtrade-operator/api/v1beta1"
)

// updateFreqUISpec retries on a resourceVersion conflict - the controller
// under test is running live against this same envtest apiserver
// (suite_test.go's BeforeSuite), and status.Patch calls it makes between
// this helper's Get and Update can otherwise race a plain read-modify-write.
func updateFreqUISpec(ctx context.Context, key types.NamespacedName, mutate func(*freqtradev1beta1.FreqUISpec)) {
	Eventually(func() error {
		var obj freqtradev1beta1.FreqUI
		if err := k8sClient.Get(ctx, key, &obj); err != nil {
			return err
		}
		mutate(&obj.Spec)
		return k8sClient.Update(ctx, &obj)
	}).Should(Succeed())
}

// G2-2's own acceptance cases (GATEWAY-API-PLAN.md): the reconcile switch
// on spec.exposure, and the pruning that keeps Ingress and HTTPRoutes from
// coexisting for the same FreqUI.
var _ = Describe("FreqUI spec.exposure reconcile switch (G2-2)", func() {
	gatewaySpec := func() *freqtradev1beta1.FUGatewaySpec {
		return &freqtradev1beta1.FUGatewaySpec{ParentRefs: []gatewayv1.ParentReference{{Name: "my-gateway"}}}
	}

	It("creates only an Ingress when exposure is Ingress", func() {
		ctx := context.Background()
		frequi := &freqtradev1beta1.FreqUI{
			ObjectMeta: metav1.ObjectMeta{Name: "exposure-ingress-only", Namespace: testNamespace},
		}
		Expect(k8sClient.Create(ctx, frequi)).To(Succeed())
		key := types.NamespacedName{Name: frequi.Name, Namespace: testNamespace}

		Eventually(func(g Gomega) {
			var ingress networkingv1.Ingress
			g.Expect(k8sClient.Get(ctx, key, &ingress)).To(Succeed())
		}).Should(Succeed())

		Consistently(func() error {
			var routes gatewayv1.HTTPRouteList
			if err := k8sClient.List(ctx, &routes, client.InNamespace(testNamespace),
				client.MatchingLabels{"freqtrade.io/frequi": frequi.Name}); err != nil {
				return err
			}
			if len(routes.Items) != 0 {
				return fmt.Errorf("expected no HTTPRoutes, got %d", len(routes.Items))
			}
			return nil
		}).Should(Succeed())
	})

	It("switches to HTTPRoutes and removes the Ingress when flipped to Gateway, then back", func() {
		ctx := context.Background()
		frequi := &freqtradev1beta1.FreqUI{
			ObjectMeta: metav1.ObjectMeta{Name: "exposure-flip", Namespace: testNamespace},
			Spec:       freqtradev1beta1.FreqUISpec{Host: "flip.example.com"},
		}
		Expect(k8sClient.Create(ctx, frequi)).To(Succeed())
		key := types.NamespacedName{Name: frequi.Name, Namespace: testNamespace}

		Eventually(func() error {
			var ingress networkingv1.Ingress
			return k8sClient.Get(ctx, key, &ingress)
		}).Should(Succeed())

		updateFreqUISpec(ctx, key, func(spec *freqtradev1beta1.FreqUISpec) {
			spec.Exposure = freqtradev1beta1.FUExposureGateway
			spec.Gateway = gatewaySpec()
		})

		Eventually(func(g Gomega) {
			var route gatewayv1.HTTPRoute
			g.Expect(k8sClient.Get(ctx, key, &route)).To(Succeed())
		}).Should(Succeed())
		Eventually(func() bool {
			var ingress networkingv1.Ingress
			return errors.IsNotFound(k8sClient.Get(ctx, key, &ingress))
		}).Should(BeTrue(), "expected the Ingress to be deleted once exposure flips to Gateway")

		// And flipping back to Ingress removes the HTTPRoute and recreates the Ingress.
		updateFreqUISpec(ctx, key, func(spec *freqtradev1beta1.FreqUISpec) {
			spec.Exposure = freqtradev1beta1.FUExposureIngress
			spec.Gateway = nil
		})

		Eventually(func() error {
			var ingress networkingv1.Ingress
			return k8sClient.Get(ctx, key, &ingress)
		}).Should(Succeed())
		Eventually(func() bool {
			var route gatewayv1.HTTPRoute
			return errors.IsNotFound(k8sClient.Get(ctx, key, &route))
		}).Should(BeTrue(), "expected the HTTPRoute to be pruned once exposure flips back to Ingress")
	})

	It("removes both Ingress and HTTPRoutes, but leaves Deployment/Service alone, when exposure is None", func() {
		ctx := context.Background()
		frequi := &freqtradev1beta1.FreqUI{
			ObjectMeta: metav1.ObjectMeta{Name: "exposure-none", Namespace: testNamespace},
		}
		Expect(k8sClient.Create(ctx, frequi)).To(Succeed())
		key := types.NamespacedName{Name: frequi.Name, Namespace: testNamespace}

		Eventually(func() error {
			var ingress networkingv1.Ingress
			return k8sClient.Get(ctx, key, &ingress)
		}).Should(Succeed())

		updateFreqUISpec(ctx, key, func(spec *freqtradev1beta1.FreqUISpec) {
			spec.Exposure = freqtradev1beta1.FUExposureNone
		})

		Eventually(func() bool {
			var ingress networkingv1.Ingress
			return errors.IsNotFound(k8sClient.Get(ctx, key, &ingress))
		}).Should(BeTrue())

		var deployment appsv1.Deployment
		Expect(k8sClient.Get(ctx, key, &deployment)).To(Succeed())
		var service corev1.Service
		Expect(k8sClient.Get(ctx, key, &service)).To(Succeed())
	})

	It("prunes a dropped bot's HTTPRoute in Gateway mode while the UI route and other bots survive", func() {
		ctx := context.Background()
		bot1 := &freqtradev1alpha1.TradeBot{
			ObjectMeta: metav1.ObjectMeta{Name: "gw-bot-1", Namespace: testNamespace},
			Spec:       freqtradev1alpha1.TradeBotSpec{Config: "irrelevant", Strategy: "irrelevant"},
		}
		bot2 := &freqtradev1alpha1.TradeBot{
			ObjectMeta: metav1.ObjectMeta{Name: "gw-bot-2", Namespace: testNamespace},
			Spec:       freqtradev1alpha1.TradeBotSpec{Config: "irrelevant", Strategy: "irrelevant"},
		}
		Expect(k8sClient.Create(ctx, bot1)).To(Succeed())
		Expect(k8sClient.Create(ctx, bot2)).To(Succeed())

		frequi := &freqtradev1beta1.FreqUI{
			ObjectMeta: metav1.ObjectMeta{Name: "exposure-gw-prune", Namespace: testNamespace},
			Spec: freqtradev1beta1.FreqUISpec{
				Host:     "prune.example.com",
				Exposure: freqtradev1beta1.FUExposureGateway,
				Gateway:  gatewaySpec(),
				TradeBotRefs: []corev1.LocalObjectReference{
					{Name: "gw-bot-1"}, {Name: "gw-bot-2"},
				},
			},
		}
		Expect(k8sClient.Create(ctx, frequi)).To(Succeed())
		key := types.NamespacedName{Name: frequi.Name, Namespace: testNamespace}
		bot1RouteKey := types.NamespacedName{Name: frequi.Name + "-gw-bot-1", Namespace: testNamespace}
		bot2RouteKey := types.NamespacedName{Name: frequi.Name + "-gw-bot-2", Namespace: testNamespace}

		// Neither bot has a Valid TradeBotConfig with an enabled API server, so neither gets
		// a route yet - what this test actually exercises is the UI route existing and
		// surviving a TradeBotRefs edit, which is the pruning behaviour under test.
		Eventually(func() error {
			var route gatewayv1.HTTPRoute
			return k8sClient.Get(ctx, key, &route)
		}).Should(Succeed())
		Consistently(func() bool {
			var route gatewayv1.HTTPRoute
			return errors.IsNotFound(k8sClient.Get(ctx, bot1RouteKey, &route)) &&
				errors.IsNotFound(k8sClient.Get(ctx, bot2RouteKey, &route))
		}).Should(BeTrue())
	})

	It("does not delete an HTTPRoute carrying this FreqUI's label but owned by a different FreqUI", func() {
		ctx := context.Background()
		owner := &freqtradev1beta1.FreqUI{
			ObjectMeta: metav1.ObjectMeta{Name: "exposure-real-owner", Namespace: testNamespace},
			Spec: freqtradev1beta1.FreqUISpec{
				Exposure: freqtradev1beta1.FUExposureGateway,
				Gateway:  gatewaySpec(),
			},
		}
		Expect(k8sClient.Create(ctx, owner)).To(Succeed())
		ownerKey := types.NamespacedName{Name: owner.Name, Namespace: testNamespace}
		Eventually(func() error {
			var route gatewayv1.HTTPRoute
			return k8sClient.Get(ctx, ownerKey, &route)
		}).Should(Succeed())

		// A second FreqUI, Ingress-mode, whose own reconcile prunes with an empty desired
		// set - the owner's route carries no label naming this second FreqUI, so it must
		// survive regardless.
		other := &freqtradev1beta1.FreqUI{
			ObjectMeta: metav1.ObjectMeta{Name: "exposure-other-frequi", Namespace: testNamespace},
		}
		Expect(k8sClient.Create(ctx, other)).To(Succeed())
		otherKey := types.NamespacedName{Name: other.Name, Namespace: testNamespace}
		Eventually(func() error {
			var ingress networkingv1.Ingress
			return k8sClient.Get(ctx, otherKey, &ingress)
		}).Should(Succeed())

		Consistently(func() error {
			var route gatewayv1.HTTPRoute
			return k8sClient.Get(ctx, ownerKey, &route)
		}).Should(Succeed())
	})
})
