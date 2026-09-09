package tradebot

import (
	"context"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	"github.com/ark-sys/freqtrade-operator/controllers/shared"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	// configRefField/strategyRefField double as field-index names and the
	// refField EnqueueTradeBotsByConfigRef filters its List by.
	configRefField   = "spec.config"
	strategyRefField = "spec.strategy"
	// defaultMaxConcurrentReconciles applies when MaxConcurrentReconciles is
	// unset - safe above 1 since P0-5 stopped finalizeTradeBot from blocking.
	defaultMaxConcurrentReconciles = 4
)

// indexTradeBotRef indexes one TradeBot spec reference field, so Watches()
// can look up referencing TradeBots directly instead of listing every one.
func indexTradeBotRef(mgr ctrl.Manager, field string, get func(*freqtradev1alpha1.TradeBot) string) error {
	return mgr.GetFieldIndexer().IndexField(context.Background(), &freqtradev1alpha1.TradeBot{}, field,
		func(obj client.Object) []string {
			if v := get(obj.(*freqtradev1alpha1.TradeBot)); v != "" {
				return []string{v}
			}
			return nil
		})
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := indexTradeBotRef(mgr, configRefField,
		func(t *freqtradev1alpha1.TradeBot) string { return t.Spec.Config }); err != nil {
		return err
	}
	if err := indexTradeBotRef(mgr, strategyRefField,
		func(t *freqtradev1alpha1.TradeBot) string { return t.Spec.Strategy }); err != nil {
		return err
	}

	// Generation only bumps on a spec change for types with a status
	// subresource (all four CRDs here). AnnotationChangedPredicate is ORed
	// in for TradeBot alone: freqtrade.io/preserve-data is an annotation.
	tradeBotChanged := predicate.Or(predicate.GenerationChangedPredicate{}, predicate.AnnotationChangedPredicate{})
	specChanged := predicate.GenerationChangedPredicate{}
	// Owned built-ins have no generation of their own; reconciling on any
	// change is fine now that doing so is a no-op when nothing actually
	// changed (server-side apply + PatchStatus, P2-1).
	ownedResourceChanged := predicate.ResourceVersionChangedPredicate{}

	maxReconciles := defaultMaxConcurrentReconciles
	if r.MaxConcurrentReconciles > 0 {
		maxReconciles = r.MaxConcurrentReconciles
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&freqtradev1alpha1.TradeBot{}, builder.WithPredicates(tradeBotChanged)).
		Watches(&freqtradev1alpha1.FreqUI{},
			handler.EnqueueRequestsFromMapFunc(shared.EnqueueTradeBotsByFreqUIRef(mgr.GetClient())),
			builder.WithPredicates(specChanged)).
		// Without these two, editing a TradeBotConfig/Strategy never
		// re-renders the referencing config Secret - the bug the old
		// hand-written predicates hid.
		Watches(&freqtradev1alpha1.TradeBotConfig{},
			handler.EnqueueRequestsFromMapFunc(shared.EnqueueTradeBotsByConfigRef(mgr.GetClient(), configRefField)),
			builder.WithPredicates(specChanged)).
		Watches(&freqtradev1alpha1.Strategy{},
			handler.EnqueueRequestsFromMapFunc(shared.EnqueueTradeBotsByConfigRef(mgr.GetClient(), strategyRefField)),
			builder.WithPredicates(specChanged)).
		Owns(&corev1.Secret{}, builder.WithPredicates(ownedResourceChanged)).
		Owns(&corev1.ConfigMap{}, builder.WithPredicates(ownedResourceChanged)).
		Owns(&corev1.Service{}, builder.WithPredicates(ownedResourceChanged)).
		Owns(&corev1.PersistentVolumeClaim{}, builder.WithPredicates(ownedResourceChanged)).
		Owns(&appsv1.StatefulSet{}, builder.WithPredicates(ownedResourceChanged)).
		WithOptions(controller.Options{MaxConcurrentReconciles: maxReconciles}).
		Complete(r)
}
