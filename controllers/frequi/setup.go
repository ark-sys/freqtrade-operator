package frequi

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	freqtradev1beta1 "github.com/ark-sys/freqtrade-operator/api/v1beta1"
)

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	specChanged := predicate.GenerationChangedPredicate{}
	// Deployment/Service/Ingress have no generation guaranteed to bump on
	// every spec change; reconciling on any change is fine now that doing
	// so is a no-op when nothing actually changed (server-side apply +
	// PatchStatus). Matches controllers/tradebot/setup.go's ownedResourceChanged.
	ownedResourceChanged := predicate.ResourceVersionChangedPredicate{}

	bldr := ctrl.NewControllerManagedBy(mgr).
		For(&freqtradev1beta1.FreqUI{}, builder.WithPredicates(specChanged)).
		Owns(&appsv1.Deployment{}, builder.WithPredicates(ownedResourceChanged)).
		Owns(&corev1.Service{}, builder.WithPredicates(ownedResourceChanged)).
		Owns(&networkingv1.Ingress{}, builder.WithPredicates(ownedResourceChanged))

	// An unconditional Owns on a CRD that isn't installed fails the cache at manager start
	// and takes down every controller in the binary (G2-2, GATEWAY-API-PLAN.md) - most
	// clusters don't have Gateway API installed (D9), so this watch is opt-in on
	// r.GatewayAPIAvailable (set once at startup, cmd/main.go, G3-1).
	if r.GatewayAPIAvailable {
		bldr = bldr.Owns(&gatewayv1.HTTPRoute{}, builder.WithPredicates(ownedResourceChanged))
	}

	return bldr.
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
		}).
		Complete(r)
}
