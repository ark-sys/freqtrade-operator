package tradebot

import (
	"context"
	"reflect"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Add indexes for faster lookups
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &freqtradev1alpha1.TradeBot{}, "spec.references.exchangeRef", func(obj client.Object) []string {
		tradeBot := obj.(*freqtradev1alpha1.TradeBot)
		if tradeBot.Spec.References != nil {
			return []string{tradeBot.Spec.References.ExchangeRef}
		}
		return nil
	}); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &freqtradev1alpha1.TradeBot{}, "spec.references.strategyRef", func(obj client.Object) []string {
		tradeBot := obj.(*freqtradev1alpha1.TradeBot)
		if tradeBot.Spec.References != nil {
			return []string{tradeBot.Spec.References.StrategyRef}
		}
		return nil
	}); err != nil {
		return err
	}

	// Create predicates for the main resource (TradeBot)
	mainResourcePredicate := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			// Skip reconciliation if only status or metadata changed
			oldObj, oldOk := e.ObjectOld.(*freqtradev1alpha1.TradeBot)
			newObj, newOk := e.ObjectNew.(*freqtradev1alpha1.TradeBot)

			if !oldOk || !newOk {
				return true
			}

			// Only reconcile if spec changed
			if reflect.DeepEqual(oldObj.Spec, newObj.Spec) {
				return false
			}

			return true
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

	// Create predicates for owned resources to filter out status-only changes
	ownedResourcePredicate := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			// For StatefulSet, ignore status changes
			if _, ok := e.ObjectOld.(*appsv1.StatefulSet); ok {
				oldSts := e.ObjectOld.(*appsv1.StatefulSet)
				newSts := e.ObjectNew.(*appsv1.StatefulSet)

				// Only trigger reconciliation if the spec changed
				return !reflect.DeepEqual(oldSts.Spec, newSts.Spec)
			}

			// For ConfigMap, only care about data changes
			if _, ok := e.ObjectOld.(*corev1.ConfigMap); ok {
				oldCm := e.ObjectOld.(*corev1.ConfigMap)
				newCm := e.ObjectNew.(*corev1.ConfigMap)

				return !reflect.DeepEqual(oldCm.Data, newCm.Data)
			}

			// For Service, ignore changes to things like clusterIP
			if _, ok := e.ObjectOld.(*corev1.Service); ok {
				oldSvc := e.ObjectOld.(*corev1.Service)
				newSvc := e.ObjectNew.(*corev1.Service)

				return !reflect.DeepEqual(oldSvc.Spec.Ports, newSvc.Spec.Ports) ||
					!reflect.DeepEqual(oldSvc.Spec.Selector, newSvc.Spec.Selector)
			}

			return true
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			// Always process delete events for owned resources
			return true
		},
		CreateFunc: func(e event.CreateEvent) bool {
			// Always process create events for owned resources
			return true
		},
		GenericFunc: func(e event.GenericEvent) bool {
			// Skip generic events
			return false
		},
	}

	// Setup watches for all referenced resources
	return ctrl.NewControllerManagedBy(mgr).
		For(&freqtradev1alpha1.TradeBot{}, builder.WithPredicates(mainResourcePredicate)).
		// Watch owned resources with predicates
		Owns(&corev1.ConfigMap{}, builder.WithPredicates(ownedResourcePredicate)).
		Owns(&corev1.Service{}, builder.WithPredicates(ownedResourcePredicate)).
		Owns(&corev1.PersistentVolumeClaim{}, builder.WithPredicates(ownedResourcePredicate)).
		Owns(&appsv1.StatefulSet{}, builder.WithPredicates(ownedResourcePredicate)).
		// Note: Config CRD watching is now handled by individual config controllers
		// which will trigger TradeBot reconciliation when their configs change
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 2,
		}).
		Complete(r)
}
