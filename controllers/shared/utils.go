package shared

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
)

// StatusUpdater provides common status update functionality for config controllers
type StatusUpdater struct {
	client.Client
}

func (s *StatusUpdater) RetryUpdateConfigStatus(ctx context.Context, obj client.Object, phase, message string, maxRetries int) error {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		lastErr = s.UpdateConfigStatus(ctx, obj, phase, message)
		if lastErr == nil {
			return nil
		}
		if !errors.IsConflict(lastErr) {
			return lastErr
		}
		// Brief backoff before retrying
		time.Sleep(100 * time.Millisecond)
	}
	return lastErr
}

// UpdateConfigStatus updates the status of a config CRD with phase and message
func (s *StatusUpdater) UpdateConfigStatus(ctx context.Context, obj client.Object, phase, message string) error {
	logger := log.FromContext(ctx)

	// Get the latest version to avoid conflicts
	latest := obj.DeepCopyObject().(client.Object)
	if err := s.Get(ctx, client.ObjectKeyFromObject(obj), latest); err != nil {
		return fmt.Errorf("failed to get latest version: %w", err)
	}

	// Update status based on object type
	switch o := latest.(type) {
	case *freqtradev1alpha1.Strategy:
		if o.Status.Phase != phase || o.Status.Message != message {
			o.Status.Phase = phase
			o.Status.Message = message
			if err := s.Status().Update(ctx, o); err != nil {
				return fmt.Errorf("failed to update Strategy status: %w", err)
			}
			logger.V(2).Info("Updated Strategy status", "name", o.Name, "phase", phase)
		}

	case *freqtradev1alpha1.TradeBotConfig:
		if o.Status.Phase != phase || o.Status.Message != message {
			o.Status.Phase = phase
			o.Status.Message = message
			if err := s.Status().Update(ctx, o); err != nil {
				return fmt.Errorf("failed to update TradeBotConfig status: %w", err)
			}
			logger.V(2).Info("Updated TradeBotConfig status", "name", o.Name, "phase",
				phase)

		}

	default:
		return fmt.Errorf("unsupported object type: %T", latest)
	}

	return nil
}

// EnqueueTradeBotsByConfigRef creates a handler function that enqueues TradeBots referencing a config CRD
func EnqueueTradeBotsByConfigRef(c client.Client, refField string) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		logger := log.FromContext(ctx)

		var tradeBots freqtradev1alpha1.TradeBotList
		if err := c.List(ctx, &tradeBots, client.InNamespace(obj.GetNamespace())); err != nil {
			logger.V(2).Error(err, "Failed to list TradeBots", "refField", refField)
			return nil
		}

		var requests []reconcile.Request
		for _, bot := range tradeBots.Items {
			if shouldEnqueueTradeBot(&bot, obj.GetName(), refField) {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      bot.Name,
						Namespace: bot.Namespace,
					},
				})
			}
		}

		if len(requests) > 0 {
			logger.V(1).Info("Enqueuing TradeBots for config change",
				"configType", refField, "configName", obj.GetName(), "tradeBotCount", len(requests))
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

// shouldEnqueueTradeBot checks if a TradeBot should be enqueued based on the reference field
func shouldEnqueueTradeBot(bot *freqtradev1alpha1.TradeBot, configName, refField string) bool {
	strategyRef := bot.Spec.Strategy
	tradebotconfigRef := bot.Spec.Config
	switch refField {
	case "strategy":
		return strategyRef == configName
	case "config":
		return tradebotconfigRef == configName
	}
	return false
}

// ReconcileResult represents the result of a reconciliation operation
type ReconcileResult struct {
	Result ctrl.Result
	Error  error
}

// FinishReconciliation handles common reconciliation completion logic
func FinishReconciliation(phase string, err error, requeueAfter time.Duration) ReconcileResult {
	if err != nil {
		return ReconcileResult{
			Result: ctrl.Result{RequeueAfter: requeueAfter},
			Error:  err,
		}
	}

	if phase == "Valid" {
		// Config is valid and stable, don't requeue frequently
		return ReconcileResult{
			Result: ctrl.Result{},
			Error:  nil,
		}
	}

	// For other phases, requeue more frequently
	return ReconcileResult{
		Result: ctrl.Result{RequeueAfter: requeueAfter},
		Error:  nil,
	}
}
