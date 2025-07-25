package exchange

import (
	"reflect"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	"github.com/ark-sys/freqtrade-operator/controllers/shared"
)

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Initialize the StatusUpdater
	r.StatusUpdater = shared.StatusUpdater{Client: mgr.GetClient()}

	// Create predicates for the main resource (Exchange)
	mainResourcePredicate := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			// Only reconcile if spec changed
			oldObj, oldOk := e.ObjectOld.(*freqtradev1alpha1.Exchange)
			newObj, newOk := e.ObjectNew.(*freqtradev1alpha1.Exchange)

			if !oldOk || !newOk {
				return true
			}

			// Reconcile if spec changed
			return !reflect.DeepEqual(oldObj.Spec, newObj.Spec)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			// Process all delete events
			return true
		},
		CreateFunc: func(e event.CreateEvent) bool {
			// Process all create events
			return true
		},
		GenericFunc: func(e event.GenericEvent) bool {
			// Skip generic events
			return false
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&freqtradev1alpha1.Exchange{}, builder.WithPredicates(mainResourcePredicate)).
		// Watch PairList changes and trigger Exchange reconciliation if referenced
		Watches(
			&freqtradev1alpha1.PairList{},
			handler.EnqueueRequestsFromMapFunc(shared.EnqueueTradeBotsByPairListRef(mgr.GetClient(), "whitelist")),
		).
		Watches(
			&freqtradev1alpha1.PairList{},
			handler.EnqueueRequestsFromMapFunc(shared.EnqueueTradeBotsByPairListRef(mgr.GetClient(), "blacklist")),
		).
		// Also trigger TradeBot reconciliation when Exchange changes
		Watches(
			&freqtradev1alpha1.Exchange{},
			handler.EnqueueRequestsFromMapFunc(shared.EnqueueTradeBotsByConfigRef(mgr.GetClient(), "exchangeRef")),
		).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 2,
		}).
		Complete(r)
}
