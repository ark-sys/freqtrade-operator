package frequi

import (
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
)

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Create predicates for the main resource (FreqUI)
	mainResourcePredicate := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			// Skip reconciliation if only status or metadata changed
			oldObj, oldOk := e.ObjectOld.(*freqtradev1alpha1.FreqUI)
			newObj, newOk := e.ObjectNew.(*freqtradev1alpha1.FreqUI)

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
	// Use a more conservative approach - only reconcile on creation and deletion of owned resources
	ownedResourcePredicate := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			// For owned resources, be very conservative about updates
			// Only reconcile if the generation changed (indicating a spec change)
			return e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration()
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
