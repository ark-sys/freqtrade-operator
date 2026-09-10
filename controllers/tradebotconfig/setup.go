package tradebotconfig

import (
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
)

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&freqtradev1alpha1.TradeBotConfig{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 2,
		}).
		Complete(r)
}
