package backtest

import (
	freqtradev1beta1 "github.com/ark-sys/freqtrade-operator/api/v1beta1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// defaultMaxConcurrentReconciles applies when MaxConcurrentReconciles is unset.
const defaultMaxConcurrentReconciles = 4

// SetupWithManager sets up the controller with the Manager. Unlike
// controllers/tradebot, this watches nothing beyond its own owned resources -
// a Backtest's spec is immutable (CEL) and never re-renders config after
// its first successful reconcile (see reconcileResources's own doc
// comment), so an edit to the Strategy or TradeBotConfig it once referenced
// has nothing left here to re-trigger.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Generation only bumps on a spec change for types with a status
	// subresource; Backtest's spec can't change post-creation anyway (CEL),
	// so this predicate mainly just filters out pure status/metadata churn
	// on this controller's own resyncs.
	backtestChanged := predicate.GenerationChangedPredicate{}
	// Owned built-ins have no generation of their own; reconciling on any
	// change is fine since server-side apply + PatchStatus (P2-1) make an
	// unchanged reconcile a zero-write no-op.
	ownedResourceChanged := predicate.ResourceVersionChangedPredicate{}

	return ctrl.NewControllerManagedBy(mgr).
		For(&freqtradev1beta1.Backtest{}, builder.WithPredicates(backtestChanged)).
		Owns(&corev1.Secret{}, builder.WithPredicates(ownedResourceChanged)).
		Owns(&corev1.ConfigMap{}, builder.WithPredicates(ownedResourceChanged)).
		Owns(&corev1.PersistentVolumeClaim{}, builder.WithPredicates(ownedResourceChanged)).
		Owns(&batchv1.Job{}, builder.WithPredicates(ownedResourceChanged)).
		WithOptions(controller.Options{MaxConcurrentReconciles: defaultMaxConcurrentReconciles}).
		Complete(r)
}
