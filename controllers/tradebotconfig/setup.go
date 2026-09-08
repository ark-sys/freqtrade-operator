package tradebotconfig

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
	// Create predicates for the main resource (Tradebotconfig)
	mainResourcePredicate := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			// Only reconcile if spec changed
			oldObj, oldOk := e.ObjectOld.(*freqtradev1alpha1.TradeBotConfig)
			newObj, newOk := e.ObjectNew.(*freqtradev1alpha1.TradeBotConfig)

			if !oldOk || !newOk {
				ctrl.Log.WithName("tradebotconfig-predicate").V(2).Info("Type assertion failed, processing", "eventType", "Update")
				return true
			}

			// Reconcile if spec changed
			return !reflect.DeepEqual(oldObj.Spec, newObj.Spec)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			ctrl.Log.WithName("tradebotconfig-predicate").V(2).Info("Delete event, processing", "eventType", "Delete", "name", e.Object.GetName())
			return true
		},
		CreateFunc: func(e event.CreateEvent) bool {
			ctrl.Log.WithName("tradebotconfig-predicate").V(2).Info("Create event, processing", "eventType", "Create", "name", e.Object.GetName())
			return true
		},
		GenericFunc: func(e event.GenericEvent) bool {
			ctrl.Log.WithName("tradebotconfig-predicate").V(3).Info("Generic event, skipping", "eventType", "Generic", "name", e.Object.GetName())
			return false
		},
	}

	// Create predicate to only trigger TradeBot reconciliation on spec changes, not status changes
	tradeBotConfigWatchPredicate := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldObj, oldOk := e.ObjectOld.(*freqtradev1alpha1.TradeBotConfig)
			newObj, newOk := e.ObjectNew.(*freqtradev1alpha1.TradeBotConfig)

			if !oldOk || !newOk {
				ctrl.Log.WithName("tradebotconfig-watch").V(2).Info("Type assertion failed, processing", "eventType", "Update")
				return true
			}

			// Only trigger TradeBot reconciliation if TradeBotConfig spec changed
			specChanged := !reflect.DeepEqual(oldObj.Spec, newObj.Spec)
			if specChanged {
				ctrl.Log.WithName("tradebotconfig-watch").V(2).Info("TradeBotConfig spec changed, triggering TradeBot reconciliation", "config", oldObj.Name)
			}
			return specChanged
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			ctrl.Log.WithName("tradebotconfig-watch").V(2).Info("TradeBotConfig deleted, triggering TradeBot reconciliation", "config", e.Object.GetName())
			return true
		},
		CreateFunc: func(e event.CreateEvent) bool {
			ctrl.Log.WithName("tradebotconfig-watch").V(2).Info("TradeBotConfig created, triggering TradeBot reconciliation", "config", e.Object.GetName())
			return true
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&freqtradev1alpha1.TradeBotConfig{}, builder.WithPredicates(mainResourcePredicate)).
		// Trigger TradeBot reconciliation when Tradebotconfig changes (with predicate to avoid status-only updates)
		Watches(
			&freqtradev1alpha1.TradeBotConfig{},
			handler.EnqueueRequestsFromMapFunc(shared.EnqueueTradeBotsByConfigRef(mgr.GetClient(), "config")),
			builder.WithPredicates(tradeBotConfigWatchPredicate),
		).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 2,
		}).
		Complete(r)
}
