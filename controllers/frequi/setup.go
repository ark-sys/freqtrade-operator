package frequi

import (
	"fmt"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
)

// isOwnedByFreqUI checks if a resource is owned by a FreqUI CRD
func isOwnedByFreqUI(obj client.Object) bool {
	for _, ownerRef := range obj.GetOwnerReferences() {
		if ownerRef.Kind == "FreqUI" && ownerRef.APIVersion == "freqtrade.io/v1alpha1" {
			return true
		}
	}
	return false
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	setupLog := ctrl.Log.WithName("frequi-predicate")

	// Create predicates for the main resource (FreqUI)
	mainResourcePredicate := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			// Skip reconciliation if only status or metadata changed
			oldObj, oldOk := e.ObjectOld.(*freqtradev1alpha1.FreqUI)
			newObj, newOk := e.ObjectNew.(*freqtradev1alpha1.FreqUI)

			if !oldOk || !newOk {
				setupLog.Info("FreqUI predicate: type assertion failed, processing", "eventType", "Update")
				return true
			}

			// Only reconcile if spec changed
			if reflect.DeepEqual(oldObj.Spec, newObj.Spec) {
				setupLog.V(1).Info("FreqUI predicate: spec unchanged, skipping reconciliation", "eventType", "Update", "name", oldObj.GetName())
				return false
			}

			setupLog.Info("FreqUI predicate: spec changed, triggering reconciliation", "eventType", "Update", "name", oldObj.GetName())
			return true
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			setupLog.Info("FreqUI predicate: delete event, processing", "eventType", "Delete", "name", e.Object.GetName())
			return true
		},
		CreateFunc: func(e event.CreateEvent) bool {
			setupLog.Info("FreqUI predicate: create event, processing", "eventType", "Create", "name", e.Object.GetName())
			return true
		},
		GenericFunc: func(e event.GenericEvent) bool {
			setupLog.V(1).Info("FreqUI predicate: generic event, skipping", "eventType", "Generic", "name", e.Object.GetName())
			return false
		},
	}

	// Create predicates for owned resources to filter out status-only changes
	// Use a more conservative approach - only reconcile on creation and deletion of owned resources
	ownedResourcePredicate := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			// First check if this resource is actually owned by a FreqUI
			if !isOwnedByFreqUI(e.ObjectOld) {
				return false // Skip resources not owned by FreqUI
			}
			// For owned resources, be very conservative about updates
			// Only reconcile if the generation changed (indicating a spec change)
			generationChanged := e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration()
			if generationChanged {
				setupLog.Info("FreqUI owned resource: generation changed, processing", "eventType", "Update", "type", fmt.Sprintf("%T", e.ObjectOld), "name", e.ObjectOld.GetName())
			} else {
				setupLog.V(1).Info("FreqUI owned resource: generation unchanged, skipping", "eventType", "Update", "type", fmt.Sprintf("%T", e.ObjectOld), "name", e.ObjectOld.GetName())
			}
			return generationChanged
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			// Check if this resource is actually owned by a FreqUI
			if !isOwnedByFreqUI(e.Object) {
				return false // Skip resources not owned by FreqUI
			}
			setupLog.Info("FreqUI owned resource: delete event, processing", "eventType", "Delete", "type", fmt.Sprintf("%T", e.Object), "name", e.Object.GetName())
			return true
		},
		CreateFunc: func(e event.CreateEvent) bool {
			// Check if this resource is actually owned by a FreqUI
			if !isOwnedByFreqUI(e.Object) {
				return false // Skip resources not owned by FreqUI
			}
			setupLog.Info("FreqUI owned resource: create event, processing", "eventType", "Create", "type", fmt.Sprintf("%T", e.Object), "name", e.Object.GetName())
			return true
		},
		GenericFunc: func(e event.GenericEvent) bool {
			setupLog.V(1).Info("FreqUI owned resource: generic event, skipping", "eventType", "Generic", "type", fmt.Sprintf("%T", e.Object), "name", e.Object.GetName())
			return false
		},
	}

	// Create controller with options
	return ctrl.NewControllerManagedBy(mgr).
		For(&freqtradev1alpha1.FreqUI{}, builder.WithPredicates(mainResourcePredicate)).
		Owns(&appsv1.Deployment{}, builder.WithPredicates(ownedResourcePredicate)).
		Owns(&corev1.Service{}, builder.WithPredicates(ownedResourcePredicate)).
		Owns(&networkingv1.Ingress{}, builder.WithPredicates(ownedResourcePredicate)).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
		}).
		Complete(r)
}
