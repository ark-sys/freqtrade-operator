package shared

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"time"

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
	case *freqtradev1alpha1.Exchange:
		if o.Status.Phase != phase || o.Status.Message != message {
			o.Status.Phase = phase
			o.Status.Message = message
			if err := s.Status().Update(ctx, o); err != nil {
				return fmt.Errorf("failed to update Exchange status: %w", err)
			}
			logger.Info("Updated Exchange status", "name", o.Name, "phase", phase)
		}
	case *freqtradev1alpha1.Strategy:
		if o.Status.Phase != phase || o.Status.Message != message {
			o.Status.Phase = phase
			o.Status.Message = message
			if err := s.Status().Update(ctx, o); err != nil {
				return fmt.Errorf("failed to update Strategy status: %w", err)
			}
			logger.Info("Updated Strategy status", "name", o.Name, "phase", phase)
		}
	case *freqtradev1alpha1.RiskManagement:
		if o.Status.Phase != phase || o.Status.Message != message {
			o.Status.Phase = phase
			o.Status.Message = message
			if err := s.Status().Update(ctx, o); err != nil {
				return fmt.Errorf("failed to update RiskManagement status: %w", err)
			}
			logger.Info("Updated RiskManagement status", "name", o.Name, "phase", phase)
		}
	case *freqtradev1alpha1.Notification:
		if o.Status.Phase != phase || o.Status.Message != message {
			o.Status.Phase = phase
			o.Status.Message = message
			if err := s.Status().Update(ctx, o); err != nil {
				return fmt.Errorf("failed to update Notification status: %w", err)
			}
			logger.Info("Updated Notification status", "name", o.Name, "phase", phase)
		}
	case *freqtradev1alpha1.Pricing:
		if o.Status.Phase != phase || o.Status.Message != message {
			o.Status.Phase = phase
			o.Status.Message = message
			if err := s.Status().Update(ctx, o); err != nil {
				return fmt.Errorf("failed to update Pricing status: %w", err)
			}
			logger.Info("Updated Pricing status", "name", o.Name, "phase", phase)
		}
	case *freqtradev1alpha1.Order:
		if o.Status.Phase != phase || o.Status.Message != message {
			o.Status.Phase = phase
			o.Status.Message = message
			if err := s.Status().Update(ctx, o); err != nil {
				return fmt.Errorf("failed to update Order status: %w", err)
			}
			logger.Info("Updated Order status", "name", o.Name, "phase", phase)
		}
	case *freqtradev1alpha1.PairList:
		if o.Status.Phase != phase || o.Status.Message != message {
			o.Status.Phase = phase
			o.Status.Message = message
			if err := s.Status().Update(ctx, o); err != nil {
				return fmt.Errorf("failed to update PairList status: %w", err)
			}
			logger.Info("Updated PairList status", "name", o.Name, "phase", phase)
		}
	case *freqtradev1alpha1.PairlistMethods:
		if o.Status.Phase != phase || o.Status.Message != message {
			o.Status.Phase = phase
			o.Status.Message = message
			if err := s.Status().Update(ctx, o); err != nil {
				return fmt.Errorf("failed to update PairlistMethods status: %w", err)
			}
			logger.Info("Updated PairlistMethods status", "name", o.Name, "phase", phase)
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
			logger.Error(err, "Failed to list TradeBots", "refField", refField)
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
			logger.Info("Enqueuing TradeBots for config change",
				"configType", refField, "configName", obj.GetName(), "tradeBotCount", len(requests))
		}

		return requests
	}
}

// shouldEnqueueTradeBot checks if a TradeBot should be enqueued based on the reference field
func shouldEnqueueTradeBot(bot *freqtradev1alpha1.TradeBot, configName, refField string) bool {
	refs := bot.Spec.References
	if refs == nil {
		return false
	}

	switch refField {
	case "exchangeRef":
		return refs.ExchangeRef == configName
	case "strategyRef":
		return refs.StrategyRef == configName
	case "riskManagementRef":
		return refs.RiskManagementRef == configName
	case "notificationRef":
		return refs.NotificationRef == configName
	case "entryPricingRef":
		return refs.EntryPricingRef == configName
	case "exitPricingRef":
		return refs.ExitPricingRef == configName
	case "orderTypesRef":
		return refs.OrderTypesRef == configName
	case "pairlistMethodsRef":
		return refs.PairlistMethodsRef == configName
	default:
		return false
	}
}

// EnqueueTradeBotsByPairListRef handles PairList references which are nested in Exchange
func EnqueueTradeBotsByPairListRef(c client.Client, pairListType string) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		logger := log.FromContext(ctx)

		// Find Exchanges that reference this PairList
		var exchanges freqtradev1alpha1.ExchangeList
		if err := c.List(ctx, &exchanges, client.InNamespace(obj.GetNamespace())); err != nil {
			logger.Error(err, "Failed to list Exchanges for PairList reference")
			return nil
		}

		var affectedExchanges []string
		for _, exchange := range exchanges.Items {
			if (pairListType == "whitelist" && exchange.Spec.WhitelistRef == obj.GetName()) ||
				(pairListType == "blacklist" && exchange.Spec.BlacklistRef == obj.GetName()) {
				affectedExchanges = append(affectedExchanges, exchange.Name)
			}
		}

		if len(affectedExchanges) == 0 {
			return nil
		}

		// Find TradeBots that reference these Exchanges
		var tradeBots freqtradev1alpha1.TradeBotList
		if err := c.List(ctx, &tradeBots, client.InNamespace(obj.GetNamespace())); err != nil {
			logger.Error(err, "Failed to list TradeBots for PairList reference")
			return nil
		}

		var requests []reconcile.Request
		for _, bot := range tradeBots.Items {
			if bot.Spec.References != nil {
				for _, exchangeName := range affectedExchanges {
					if bot.Spec.References.ExchangeRef == exchangeName {
						requests = append(requests, reconcile.Request{
							NamespacedName: types.NamespacedName{
								Name:      bot.Name,
								Namespace: bot.Namespace,
							},
						})
						break
					}
				}
			}
		}

		if len(requests) > 0 {
			logger.Info("Enqueuing TradeBots for PairList change",
				"pairListType", pairListType, "pairListName", obj.GetName(), "tradeBotCount", len(requests))
		}

		return requests
	}
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

func MergeSpecsWithStrategicPatch[T any](defaultSpec, userSpec T, patchMeta any) T {
	defaultJSON, err := json.Marshal(defaultSpec)
	if err != nil {
		// Log error if you have logging setup
		return defaultSpec
	}

	userJSON, err := json.Marshal(userSpec)
	if err != nil {
		return defaultSpec
	}

	merged, err := strategicpatch.StrategicMergePatch(defaultJSON, userJSON, patchMeta)
	if err != nil {
		return defaultSpec
	}

	var result T
	if err := json.Unmarshal(merged, &result); err != nil {
		return defaultSpec
	}

	return result
}
