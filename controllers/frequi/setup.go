package frequi

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
)

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	specChanged := predicate.GenerationChangedPredicate{}
	// Deployment/Service/Ingress have no generation guaranteed to bump on
	// every spec change; reconciling on any change is fine now that doing
	// so is a no-op when nothing actually changed (server-side apply +
	// PatchStatus). Matches controllers/tradebot/setup.go's ownedResourceChanged.
	ownedResourceChanged := predicate.ResourceVersionChangedPredicate{}

	return ctrl.NewControllerManagedBy(mgr).
		For(&freqtradev1alpha1.FreqUI{}, builder.WithPredicates(specChanged)).
		Owns(&appsv1.Deployment{}, builder.WithPredicates(ownedResourceChanged)).
		Owns(&corev1.Service{}, builder.WithPredicates(ownedResourceChanged)).
		Owns(&networkingv1.Ingress{}, builder.WithPredicates(ownedResourceChanged)).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
		}).
		Complete(r)
}
