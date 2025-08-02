package tradebot

import (
	"context"
	"fmt"
	"reflect"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	"github.com/ark-sys/freqtrade-operator/controllers/shared"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// isOwnedByTradeBot checks if a resource is owned by a TradeBot CRD
func isOwnedByTradeBot(obj client.Object) bool {
	for _, ownerRef := range obj.GetOwnerReferences() {
		if ownerRef.Kind == "TradeBot" && ownerRef.APIVersion == "freqtrade.io/v1alpha1" {
			return true
		}
	}
	return false
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &freqtradev1alpha1.TradeBot{}, "spec.config", func(obj client.Object) []string {
		tradeBot := obj.(*freqtradev1alpha1.TradeBot)
		if tradeBot.Spec.Config != "" {
			return []string{tradeBot.Spec.Config}
		}
		return nil
	}); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &freqtradev1alpha1.TradeBot{}, "spec.strategy", func(obj client.Object) []string {
		tradeBot := obj.(*freqtradev1alpha1.TradeBot)
		if tradeBot.Spec.Strategy != "" {
			return []string{tradeBot.Spec.Strategy}
		}
		return nil
	}); err != nil {
		return err
	}

	setupLog := ctrl.Log.WithName("predicate")

	mainResourcePredicate := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			setupLog.Info("=== MAIN RESOURCE UPDATE EVENT ===", "eventType", "Update", "objectType", fmt.Sprintf("%T", e.ObjectOld), "name", e.ObjectOld.GetName(), "namespace", e.ObjectOld.GetNamespace())

			oldObj, oldOk := e.ObjectOld.(*freqtradev1alpha1.TradeBot)
			newObj, newOk := e.ObjectNew.(*freqtradev1alpha1.TradeBot)

			if !oldOk || !newOk {
				setupLog.Info("Predicate triggered: type assertion failed", "eventType", "Update")
				return true
			}

			// Check if only status changed (should not trigger reconciliation)
			if reflect.DeepEqual(oldObj.Spec, newObj.Spec) {
				if !reflect.DeepEqual(oldObj.Status, newObj.Status) {
					setupLog.Info("Predicate: ONLY status changed, skipping reconciliation", "eventType", "Update", "name", oldObj.GetName(), "oldStatus", oldObj.Status, "newStatus", newObj.Status)
				} else {
					setupLog.Info("Predicate: no changes detected, skipping", "eventType", "Update", "name", oldObj.GetName())
				}
				return false
			}

			setupLog.Info("Predicate triggered: spec changed", "eventType", "Update", "name", oldObj.GetName(), "oldSpec", fmt.Sprintf("%#v", oldObj.Spec), "newSpec", fmt.Sprintf("%#v", newObj.Spec))
			return true
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			setupLog.Info("Predicate triggered: delete", "eventType", "Delete", "name", e.Object.GetName())
			return true
		},
		CreateFunc: func(e event.CreateEvent) bool {
			setupLog.Info("Predicate triggered: create", "eventType", "Create", "name", e.Object.GetName())
			return true
		},
		GenericFunc: func(e event.GenericEvent) bool {
			setupLog.Info("Predicate triggered: generic (skipped)", "eventType", "Generic", "name", e.Object.GetName())
			return false
		},
	}

	ownedResourcePredicate := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			// First check if this resource is actually owned by a TradeBot
			if !isOwnedByTradeBot(e.ObjectOld) {
				return false // Skip resources not owned by TradeBot
			}
			setupLog.Info("=== OWNED RESOURCE UPDATE EVENT ===", "eventType", "Update", "objectType", fmt.Sprintf("%T", e.ObjectOld), "name", e.ObjectOld.GetName(), "namespace", e.ObjectOld.GetNamespace())
			switch old := e.ObjectOld.(type) {
			case *appsv1.StatefulSet:
				new := e.ObjectNew.(*appsv1.StatefulSet)
				if reflect.DeepEqual(old.Spec, new.Spec) {
					setupLog.Info("Owned predicate: StatefulSet spec unchanged, skipping", "eventType", "Update", "name", old.GetName())
					return false
				}
				setupLog.Info("Owned predicate: StatefulSet spec changed", "eventType", "Update", "name", old.GetName())
				return true
			case *corev1.ConfigMap:
				new := e.ObjectNew.(*corev1.ConfigMap)
				if reflect.DeepEqual(old.Data, new.Data) {
					setupLog.Info("Owned predicate: ConfigMap data unchanged, skipping", "eventType", "Update", "name", old.GetName())
					return false
				}
				setupLog.Info("Owned predicate: ConfigMap data changed", "eventType", "Update", "name", old.GetName())
				return true
			case *corev1.Service:
				new := e.ObjectNew.(*corev1.Service)
				if reflect.DeepEqual(old.Spec.Ports, new.Spec.Ports) && reflect.DeepEqual(old.Spec.Selector, new.Spec.Selector) {
					setupLog.Info("Owned predicate: Service ports/selectors unchanged, skipping", "eventType", "Update", "name", old.GetName())
					return false
				}
				setupLog.Info("Owned predicate: Service ports/selectors changed", "eventType", "Update", "name", old.GetName())
				return true
			case *corev1.Secret:
				new := e.ObjectNew.(*corev1.Secret)
				if reflect.DeepEqual(old.Data, new.Data) {
					setupLog.Info("Owned predicate: Secret data unchanged, skipping", "eventType", "Update", "name", old.GetName())
					return false
				}
				setupLog.Info("Owned predicate: Secret data changed", "eventType", "Update", "name", old.GetName())
				return true
			case *corev1.PersistentVolumeClaim:
				new := e.ObjectNew.(*corev1.PersistentVolumeClaim)
				// For PVCs, we typically only care about spec changes, not status changes
				if reflect.DeepEqual(old.Spec, new.Spec) {
					setupLog.Info("Owned predicate: PVC spec unchanged, skipping", "eventType", "Update", "name", old.GetName())
					return false
				}
				setupLog.Info("Owned predicate: PVC spec changed", "eventType", "Update", "name", old.GetName())
				return true
			default:
				setupLog.Info("Owned predicate: unknown type, processing", "eventType", "Update", "type", fmt.Sprintf("%T", old), "name", old.GetName())
				return true
			}
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			// Check if this resource is actually owned by a TradeBot
			if !isOwnedByTradeBot(e.Object) {
				return false // Skip resources not owned by TradeBot
			}
			setupLog.Info("Owned predicate: delete", "eventType", "Delete", "name", e.Object.GetName())
			return true
		},
		CreateFunc: func(e event.CreateEvent) bool {
			// Check if this resource is actually owned by a TradeBot
			if !isOwnedByTradeBot(e.Object) {
				return false // Skip resources not owned by TradeBot
			}
			setupLog.Info("Owned predicate: create", "eventType", "Create", "name", e.Object.GetName())
			return true
		},
		GenericFunc: func(e event.GenericEvent) bool {
			setupLog.Info("Owned predicate: generic (skipped)", "eventType", "Generic", "name", e.Object.GetName())
			return false
		},
	}

	// Create predicate to only trigger TradeBot reconciliation on FreqUI spec changes, not status changes
	frequiWatchPredicate := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldObj, oldOk := e.ObjectOld.(*freqtradev1alpha1.FreqUI)
			newObj, newOk := e.ObjectNew.(*freqtradev1alpha1.FreqUI)

			if !oldOk || !newOk {
				return true
			}

			// Only trigger TradeBot reconciliation if FreqUI spec changed
			specChanged := !reflect.DeepEqual(oldObj.Spec, newObj.Spec)
			if specChanged {
				setupLog.Info("FreqUI spec changed, triggering TradeBot reconciliation", "frequi", oldObj.Name, "tradeBotRefs", newObj.Spec.TradeBotRefs)
			}
			return specChanged
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			frequi := e.Object.(*freqtradev1alpha1.FreqUI)
			setupLog.Info("FreqUI deleted, triggering TradeBot reconciliation", "frequi", frequi.Name, "tradeBotRefs", frequi.Spec.TradeBotRefs)
			return true
		},
		CreateFunc: func(e event.CreateEvent) bool {
			frequi := e.Object.(*freqtradev1alpha1.FreqUI)
			setupLog.Info("FreqUI created, triggering TradeBot reconciliation", "frequi", frequi.Name, "tradeBotRefs", frequi.Spec.TradeBotRefs)
			return true
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&freqtradev1alpha1.TradeBot{}, builder.WithPredicates(mainResourcePredicate)).
		// Watch FreqUI resources and trigger TradeBot reconciliation when they reference this TradeBot
		Watches(
			&freqtradev1alpha1.FreqUI{},
			handler.EnqueueRequestsFromMapFunc(shared.EnqueueTradeBotsByFreqUIRef(mgr.GetClient())),
			builder.WithPredicates(frequiWatchPredicate),
		).
		Owns(&corev1.Secret{}, builder.WithPredicates(ownedResourcePredicate)).
		Owns(&corev1.ConfigMap{}, builder.WithPredicates(ownedResourcePredicate)).
		Owns(&corev1.Service{}, builder.WithPredicates(ownedResourcePredicate)).
		Owns(&corev1.PersistentVolumeClaim{}, builder.WithPredicates(ownedResourcePredicate)).
		Owns(&appsv1.StatefulSet{}, builder.WithPredicates(ownedResourcePredicate)).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
		}).
		Complete(r)
}
