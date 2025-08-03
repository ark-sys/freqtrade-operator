package strategy

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

	// Create predicates for the main resource (Strategy)
	mainResourcePredicate := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			// Only reconcile if spec changed
			oldObj, oldOk := e.ObjectOld.(*freqtradev1alpha1.Strategy)
			newObj, newOk := e.ObjectNew.(*freqtradev1alpha1.Strategy)

			if !oldOk || !newOk {
				ctrl.Log.WithName("strategy-predicate").V(2).Info("Type assertion failed, processing", "eventType", "Update")
				return true
			}

			// Reconcile if spec changed
			return !reflect.DeepEqual(oldObj.Spec, newObj.Spec)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			ctrl.Log.WithName("strategy-predicate").V(2).Info("Delete event, processing", "eventType", "Delete", "name", e.Object.GetName())
			return true
		},
		CreateFunc: func(e event.CreateEvent) bool {
			ctrl.Log.WithName("strategy-predicate").V(2).Info("Create event, processing", "eventType", "Create", "name", e.Object.GetName())
			return true
		},
		GenericFunc: func(e event.GenericEvent) bool {
			ctrl.Log.WithName("strategy-predicate").V(3).Info("Generic event, skipping", "eventType", "Generic", "name", e.Object.GetName())
			return false
		},
	}

	// Create predicate to only trigger TradeBot reconciliation on spec changes, not status changes
	strategyWatchPredicate := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldObj, oldOk := e.ObjectOld.(*freqtradev1alpha1.Strategy)
			newObj, newOk := e.ObjectNew.(*freqtradev1alpha1.Strategy)

			if !oldOk || !newOk {
				ctrl.Log.WithName("strategy-watch").V(2).Info("Type assertion failed, processing", "eventType", "Update")
				return true
			}

			// Only trigger TradeBot reconciliation if Strategy spec changed
			specChanged := !reflect.DeepEqual(oldObj.Spec, newObj.Spec)
			if specChanged {
				ctrl.Log.WithName("strategy-watch").V(2).Info("Strategy spec changed, triggering TradeBot reconciliation", "strategy", oldObj.Name)
			}
			return specChanged
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			ctrl.Log.WithName("strategy-watch").V(2).Info("Strategy deleted, triggering TradeBot reconciliation", "strategy", e.Object.GetName())
			return true
		},
		CreateFunc: func(e event.CreateEvent) bool {
			ctrl.Log.WithName("strategy-watch").V(2).Info("Strategy created, triggering TradeBot reconciliation", "strategy", e.Object.GetName())
			return true
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&freqtradev1alpha1.Strategy{}, builder.WithPredicates(mainResourcePredicate)).
		// Trigger TradeBot reconciliation when Strategy changes (with predicate to avoid status-only updates)
		Watches(
			&freqtradev1alpha1.Strategy{},
			handler.EnqueueRequestsFromMapFunc(shared.EnqueueTradeBotsByConfigRef(mgr.GetClient(), "strategy")),
			builder.WithPredicates(strategyWatchPredicate),
		).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 2,
		}).
		Complete(r)
}
