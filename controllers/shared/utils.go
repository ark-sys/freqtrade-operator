package shared

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
)

// FieldOwner identifies this operator's writes for server-side apply. All
// resources it manages are applied under one owner, since nothing else is
// meant to co-manage them - client.ForceOwnership resolves any conflict in
// this operator's own favor rather than erroring.
const FieldOwner = "freqtrade-operator"

// ConditionedObject is any CRD whose status carries the standard
// Conditions/ObservedGeneration pair - all four CRDs in this operator
// (TradeBot, FreqUI, Strategy, TradeBotConfig each implement
// SetObservedGeneration alongside the generated accessors on ObjectMeta).
type ConditionedObject interface {
	client.Object
	SetObservedGeneration(generation int64)
}

// PatchStatus re-fetches obj (in place - obj must be a pointer, and mutate
// must operate on that same pointer, typically via closure capture of the
// caller's already-typed variable), lets mutate compute the desired status
// - normally one or more meta.SetStatusCondition calls plus a Phase/Message
// derived from the resulting condition set - sets ObservedGeneration to the
// object's current generation, and writes the result only if something
// actually changed (so reconciling an unchanged object costs zero API
// writes, not even a no-op status Update). The whole sequence is retried on
// write conflicts instead of a fixed sleep: mutate must be idempotent (safe
// to call again against a newer copy of obj), which holds naturally since
// it recomputes conditions from the same inputs each time rather than
// accumulating changes. meta.SetStatusCondition itself is idempotent too -
// it only bumps LastTransitionTime when Status actually flips - so calling
// it with the same Type/Status/Reason/Message on an unchanged object is a
// true no-op that the equality check below correctly detects.
//
// This replaces the old StatusUpdater, which type-switched over concrete
// CRD types and returned "unsupported object type" for anything it didn't
// special-case (notably TradeBot and FreqUI). One helper now serves all
// four CRDs without knowing which one it's holding.
func PatchStatus(ctx context.Context, c client.Client, obj ConditionedObject, mutate func()) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if err := c.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
			return err
		}
		before := obj.DeepCopyObject()
		mutate()
		obj.SetObservedGeneration(obj.GetGeneration())
		if equality.Semantic.DeepEqual(before, obj) {
			return nil
		}
		return c.Status().Update(ctx, obj)
	})
}

// Apply patches obj into the cluster via server-side apply, after setting
// owner as its controller reference. This is the one shared replacement for
// what used to be six-going-on-nine near-identical ApplyX functions, each
// doing get -> reflect.DeepEqual a hand-picked field subset -> Update: those
// comparisons were subtly wrong in every case, since the API server
// defaults fields (terminationMessagePath, dnsPolicy, ...) that never
// appear in the freshly-built desired object, so DeepEqual was always
// false and every reconcile issued a pointless Update - write
// amplification, and for a StatefulSet, a potential rolling restart of a
// live trading bot. Server-side apply only ever considers fields this
// field manager's own applied configuration mentions, so a defaulted field
// it never set is simply not part of the comparison.
//
// controllerutil.SetControllerReference (rather than a hand-built
// OwnerReference) gets the scheme check and the
// already-owned-by-another-controller error for free. obj's GVK is looked
// up and set explicitly: server-side apply's patch body needs it, and a
// typed Go struct literal never carries it (TypeMeta is normally left for
// the API server to infer from the request path).
func Apply(ctx context.Context, c client.Client, owner, obj client.Object) error {
	if err := controllerutil.SetControllerReference(owner, obj, c.Scheme()); err != nil {
		return fmt.Errorf("failed to set controller reference: %w", err)
	}
	return applyServerSide(ctx, c, obj)
}

// ApplyUnowned is Apply without a controller reference, for a resource
// that must outlive any single object's lifecycle - e.g. the Backtest
// controller's shared, namespace-wide sidecar ServiceAccount/Role/
// RoleBinding (P6-2): owning it by whichever Backtest happened to create
// it first would cascade-delete it out from under every other Backtest in
// the namespace the moment that one is deleted.
func ApplyUnowned(ctx context.Context, c client.Client, obj client.Object) error {
	return applyServerSide(ctx, c, obj)
}

func applyServerSide(ctx context.Context, c client.Client, obj client.Object) error {
	gvk, err := apiutil.GVKForObject(obj, c.Scheme())
	if err != nil {
		return fmt.Errorf("failed to look up GroupVersionKind: %w", err)
	}
	obj.GetObjectKind().SetGroupVersionKind(gvk)
	return c.Patch(ctx, obj, client.Apply, client.FieldOwner(FieldOwner), client.ForceOwnership)
}

// EnqueueTradeBotsByConfigRef creates a handler function that enqueues the
// TradeBots referencing obj (a TradeBotConfig or Strategy) via a spec field
// registered as a field index under refField (e.g. TradeBot's setup.go).
// The index is what keeps this an indexed lookup rather than listing every
// TradeBot in the namespace and checking each one's spec in memory.
func EnqueueTradeBotsByConfigRef(c client.Client, refField string) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		logger := log.FromContext(ctx)

		var tradeBots freqtradev1alpha1.TradeBotList
		if err := c.List(ctx, &tradeBots,
			client.InNamespace(obj.GetNamespace()),
			client.MatchingFields{refField: obj.GetName()},
		); err != nil {
			logger.V(2).Error(err, "Failed to list TradeBots", "refField", refField)
			return nil
		}

		requests := make([]reconcile.Request, 0, len(tradeBots.Items))
		for _, bot := range tradeBots.Items {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: bot.Name, Namespace: bot.Namespace},
			})
		}

		if len(requests) > 0 {
			logger.V(1).Info("Enqueuing TradeBots for config change",
				"refField", refField, "configName", obj.GetName(), "tradeBotCount", len(requests))
		}

		return requests
	}
}

// EnqueueTradeBotsByFreqUIRef creates a handler function that enqueues TradeBots referenced by a FreqUI
func EnqueueTradeBotsByFreqUIRef(c client.Client) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		logger := log.FromContext(ctx)

		frequi, ok := obj.(*freqtradev1alpha1.FreqUI)
		if !ok {
			logger.Error(fmt.Errorf("unexpected object type"), "Expected FreqUI", "actualType", fmt.Sprintf("%T", obj))
			return nil
		}

		var requests []reconcile.Request
		for _, tradeBotRef := range frequi.Spec.TradeBotRefs {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      tradeBotRef,
					Namespace: frequi.Namespace,
				},
			})
		}

		if len(requests) > 0 {
			logger.V(1).Info("Enqueuing TradeBots for FreqUI change",
				"frequiName", frequi.Name, "tradeBotRefs", frequi.Spec.TradeBotRefs, "tradeBotCount", len(requests))
		}

		return requests
	}
}
