package controllers

import (
	"context"
	"fmt"
	"time"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	"github.com/ark-sys/freqtrade-operator/api/v1alpha1/configbuilder"
	"github.com/ark-sys/freqtrade-operator/controllers/resources"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	// TradeBotFinalizer is the finalizer name used for TradeBot resources
	TradeBotFinalizer = "freqtrade.io/finalizer"
)

// TradeBotReconciler reconciles a TradeBot object
type TradeBotReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// Reconcile handles the reconciliation loop for TradeBot resources
func (r *TradeBotReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling TradeBot", "request", req.NamespacedName)

	// 1. Fetch TradeBot
	var tradeBot freqtradev1alpha1.TradeBot
	if err := r.Get(ctx, req.NamespacedName, &tradeBot); err != nil {
		if errors.IsNotFound(err) {
			// Object not found, likely deleted, return
			logger.Info("TradeBot resource not found, ignoring since it must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get TradeBot")
		return ctrl.Result{}, err
	}

	// Check if the TradeBot instance is marked for deletion
	if tradeBot.GetDeletionTimestamp() != nil {
		logger.Info("TradeBot is being deleted", "name", tradeBot.Name)
		if controllerutil.ContainsFinalizer(&tradeBot, TradeBotFinalizer) {
			// Run finalization logic
			if err := r.finalizeTradeBot(ctx, &tradeBot); err != nil {
				logger.Error(err, "Failed to finalize TradeBot")
				return ctrl.Result{}, err
			}

			// Remove finalizer from the TradeBot object
			controllerutil.RemoveFinalizer(&tradeBot, TradeBotFinalizer)
			if err := r.Update(ctx, &tradeBot); err != nil {
				logger.Error(err, "Failed to remove finalizer from TradeBot")
				return ctrl.Result{}, err
			}
			logger.Info("Finalizer removed from TradeBot", "name", tradeBot.Name)
		}
		return ctrl.Result{}, nil
	}

	// Add finalizer if it doesn't exist
	if !controllerutil.ContainsFinalizer(&tradeBot, TradeBotFinalizer) {
		controllerutil.AddFinalizer(&tradeBot, TradeBotFinalizer)
		if err := r.Update(ctx, &tradeBot); err != nil {
			logger.Error(err, "Failed to add finalizer to TradeBot")
			return ctrl.Result{}, err
		}
		logger.Info("Finalizer added to TradeBot", "name", tradeBot.Name)
		// Return here to avoid processing further until the update is processed
		return ctrl.Result{}, nil
	}

	// 2. Fetch referenced CRDs
	var exchange freqtradev1alpha1.Exchange
	if err := r.Get(ctx, types.NamespacedName{Name: tradeBot.Spec.ExchangeRef, Namespace: req.Namespace}, &exchange); err != nil {
		logger.Error(err, "failed to fetch Exchange")
		return ctrl.Result{}, err
	}

	// Fetch PairLists referenced by Exchange
	var pairWhitelist, pairBlacklist *freqtradev1alpha1.PairList
	if exchange.Spec.WhitelistRef != "" {
		var wl freqtradev1alpha1.PairList
		if err := r.Get(ctx, types.NamespacedName{Name: exchange.Spec.WhitelistRef, Namespace: req.Namespace}, &wl); err != nil {
			logger.Error(err, "failed to fetch PairWhitelist")
			return ctrl.Result{}, err
		}
		pairWhitelist = &wl
	}
	if exchange.Spec.BlacklistRef != "" {
		var bl freqtradev1alpha1.PairList
		if err := r.Get(ctx, types.NamespacedName{Name: exchange.Spec.BlacklistRef, Namespace: req.Namespace}, &bl); err != nil {
			logger.Error(err, "failed to fetch PairBlacklist")
			return ctrl.Result{}, err
		}
		pairBlacklist = &bl
	}

	var entryPricing *freqtradev1alpha1.EntryPricing
	if tradeBot.Spec.EntryPricingRef != "" {
		ep := &freqtradev1alpha1.EntryPricing{}
		if err := r.Get(ctx, types.NamespacedName{Name: tradeBot.Spec.EntryPricingRef, Namespace: req.Namespace}, ep); err != nil {
			logger.Error(err, "failed to fetch EntryPricing")
			return ctrl.Result{}, err
		}
		entryPricing = ep
	}

	var exitPricing *freqtradev1alpha1.ExitPricing
	if tradeBot.Spec.ExitPricingRef != "" {
		ep := &freqtradev1alpha1.ExitPricing{}
		if err := r.Get(ctx, types.NamespacedName{Name: tradeBot.Spec.ExitPricingRef, Namespace: req.Namespace}, ep); err != nil {
			logger.Error(err, "failed to fetch ExitPricing")
			return ctrl.Result{}, err
		}
		exitPricing = ep
	}

	var orderTypes *freqtradev1alpha1.OrderTypes
	if tradeBot.Spec.OrderTypesRef != "" {
		ot := &freqtradev1alpha1.OrderTypes{}
		if err := r.Get(ctx, types.NamespacedName{Name: tradeBot.Spec.OrderTypesRef, Namespace: req.Namespace}, ot); err != nil {
			logger.Error(err, "failed to fetch OrderTypes")
			return ctrl.Result{}, err
		}
		orderTypes = ot
	}

	var riskManagement *freqtradev1alpha1.RiskManagement
	if tradeBot.Spec.RiskManagementRef != "" {
		rm := &freqtradev1alpha1.RiskManagement{}
		if err := r.Get(ctx, types.NamespacedName{Name: tradeBot.Spec.RiskManagementRef, Namespace: req.Namespace}, rm); err != nil {
			logger.Error(err, "failed to fetch RiskManagement")
			return ctrl.Result{}, err
		}
		riskManagement = rm
	}

	var notification *freqtradev1alpha1.Notification
	if tradeBot.Spec.NotificationRef != "" {
		n := &freqtradev1alpha1.Notification{}
		if err := r.Get(ctx, types.NamespacedName{Name: tradeBot.Spec.NotificationRef, Namespace: req.Namespace}, n); err != nil {
			logger.Error(err, "failed to fetch Notification")
			return ctrl.Result{}, err
		}
		notification = n
	}

	var strategy freqtradev1alpha1.Strategy
	if err := r.Get(ctx, types.NamespacedName{Name: tradeBot.Spec.StrategyRef, Namespace: req.Namespace}, &strategy); err != nil {
		logger.Error(err, "failed to fetch Strategy")
		return ctrl.Result{}, err
	}

	// Fetch Pairlists if specified
	var pairlistMethods *freqtradev1alpha1.PairlistMethods
	if tradeBot.Spec.PairlistMethodsRef != "" {
		pm := &freqtradev1alpha1.PairlistMethods{}
		if err := r.Get(ctx, types.NamespacedName{Name: tradeBot.Spec.PairlistMethodsRef, Namespace: req.Namespace}, pm); err != nil {
			logger.Error(err, "failed to fetch PairlistMethods")
			return ctrl.Result{}, err
		}
		pairlistMethods = pm
	}

	// 3. Assemble config.json from TradeBot and referenced CRDs using configbuilder
	configData, jwtSecretKey, err := configbuilder.AssembleConfig(
		ctx, r.Client,
		&tradeBot, &exchange, pairWhitelist, pairBlacklist,
		entryPricing, exitPricing, orderTypes, riskManagement, notification, &strategy,
		pairlistMethods,
	)
	if err != nil {
		logger.Error(err, "failed to assemble config")
		return ctrl.Result{}, err
	}

	// 4. Create or update ConfigMap for config.json
	configMap := resources.BuildConfigMap(tradeBot, configData)
	if err := resources.ApplyConfigMap(ctx, r.Client, &configMap); err != nil {
		logger.Error(err, "failed to apply ConfigMap")
		return ctrl.Result{}, err
	}

	// 5. Create or update ConfigMap for strategy
	strategyConfigMap := resources.BuildStrategyConfigMap(tradeBot, strategy)
	if err := resources.ApplyConfigMap(ctx, r.Client, &strategyConfigMap); err != nil {
		logger.Error(err, "failed to apply Strategy ConfigMap")
		return ctrl.Result{}, err
	}

	// 6. Create or update PVC for user_data
	pvc := resources.BuildUserDataPVC(tradeBot)
	if err := resources.ApplyPVC(ctx, r.Client, &pvc); err != nil {
		logger.Error(err, "failed to apply PVC")
		return ctrl.Result{}, err
	}

	// 7. Create or update StatefulSet
	sts := resources.BuildStatefulSet(tradeBot, configMap.Name, strategyConfigMap.Name, pvc.Name)
	if err := resources.ApplyStatefulSet(ctx, r.Client, &sts); err != nil {
		logger.Error(err, "failed to apply StatefulSet")
		return ctrl.Result{}, err
	}

	// 8. Create or update Service
	svc := resources.BuildService(tradeBot)
	if err := resources.ApplyService(ctx, r.Client, &svc); err != nil {
		logger.Error(err, "failed to apply Service")
		return ctrl.Result{}, err
	}

	// 9. Update status with JWT secret key
	tradeBot.Status.Phase = "Running"
	tradeBot.Status.Message = "Bot deployed successfully"
	tradeBot.Status.JWTSecretKey = jwtSecretKey
	if err := r.Status().Update(ctx, &tradeBot); err != nil {
		logger.Error(err, "failed to update status")
		return ctrl.Result{}, err
	}

	logger.Info("Reconciliation completed successfully")
	return ctrl.Result{}, nil
}

// finalizeTradeBot performs cleanup operations when a TradeBot is being deleted
func (r *TradeBotReconciler) finalizeTradeBot(ctx context.Context, tradeBot *freqtradev1alpha1.TradeBot) error {
	logger := log.FromContext(ctx)
	logger.Info("Finalizing TradeBot", "name", tradeBot.Name)

	// 1. Get the StatefulSet to check if it exists
	var sts appsv1.StatefulSet
	stsName := types.NamespacedName{Name: tradeBot.Name, Namespace: tradeBot.Namespace}
	stsErr := r.Get(ctx, stsName, &sts)

	// 2. If the StatefulSet exists, scale it down to 0 to ensure graceful termination
	if stsErr == nil {
		logger.Info("Scaling down StatefulSet before deletion", "name", sts.Name)
		replicas := int32(0)
		sts.Spec.Replicas = &replicas
		if err := r.Update(ctx, &sts); err != nil {
			logger.Error(err, "Failed to scale down StatefulSet")
			return err
		}

		// Wait for the StatefulSet to scale down
		if err := r.waitForStatefulSetScaleDown(ctx, &sts); err != nil {
			logger.Error(err, "Failed to wait for StatefulSet scale down")
			return err
		}
		logger.Info("StatefulSet scaled down successfully", "name", sts.Name)
	} else if !errors.IsNotFound(stsErr) {
		logger.Error(stsErr, "Failed to get StatefulSet during finalization")
		return stsErr
	}

	// 3. Check if we need to preserve the PVC
	// Look for the preserve-data annotation
	if preserveData, exists := tradeBot.Annotations["freqtrade.io/preserve-data"]; exists && preserveData == "true" {
		logger.Info("Preserving PVC as requested by annotation", "name", tradeBot.Name)

		// Find the PVC
		pvcName := tradeBot.Name + "-user-data"
		var pvc corev1.PersistentVolumeClaim
		err := r.Get(ctx, types.NamespacedName{Name: pvcName, Namespace: tradeBot.Namespace}, &pvc)
		if err != nil {
			if !errors.IsNotFound(err) {
				logger.Error(err, "Failed to get PVC during finalization")
				return err
			}
			// PVC not found, nothing to preserve
			logger.Info("PVC not found, nothing to preserve", "name", pvcName)
			return nil
		}

		// Remove owner reference to prevent garbage collection
		var newOwnerRefs []metav1.OwnerReference
		for _, ownerRef := range pvc.OwnerReferences {
			if ownerRef.UID != tradeBot.UID {
				newOwnerRefs = append(newOwnerRefs, ownerRef)
			}
		}
		pvc.OwnerReferences = newOwnerRefs

		// Add annotation to indicate this PVC was preserved
		if pvc.Annotations == nil {
			pvc.Annotations = make(map[string]string)
		}
		pvc.Annotations["freqtrade.io/preserved-from"] = tradeBot.Name
		pvc.Annotations["freqtrade.io/preserved-at"] = time.Now().Format(time.RFC3339)

		// Update the PVC
		if err := r.Update(ctx, &pvc); err != nil {
			logger.Error(err, "Failed to update PVC to preserve it")
			return err
		}
		logger.Info("Successfully preserved PVC", "name", pvcName)
	} else {
		logger.Info("PVC will be garbage collected as no preserve annotation was found")
	}

	logger.Info("TradeBot finalization completed successfully", "name", tradeBot.Name)
	return nil
}

// waitForStatefulSetScaleDown waits for the StatefulSet to scale down to 0 replicas
func (r *TradeBotReconciler) waitForStatefulSetScaleDown(ctx context.Context, sts *appsv1.StatefulSet) error {
	logger := log.FromContext(ctx)
	namespacedName := types.NamespacedName{
		Name:      sts.Name,
		Namespace: sts.Namespace,
	}

	// Wait for up to 2 minutes for the StatefulSet to scale down
	// In a production environment, you might want to make this configurable
	maxAttempts := 24 // 24 attempts * 5 seconds = 2 minutes
	for i := 0; i < maxAttempts; i++ {
		// Check if context is done
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled while waiting for StatefulSet to scale down")
		default:
			// Continue
		}

		// Get the latest StatefulSet
		if err := r.Get(ctx, namespacedName, sts); err != nil {
			if errors.IsNotFound(err) {
				// StatefulSet is gone, which is fine
				return nil
			}
			return err
		}

		// Check if the StatefulSet is scaled down
		if sts.Status.Replicas == 0 && sts.Status.ReadyReplicas == 0 {
			logger.Info("StatefulSet is fully scaled down", "name", sts.Name)
			return nil
		}

		// Log progress
		logger.Info("Waiting for StatefulSet to scale down",
			"name", sts.Name,
			"current", sts.Status.Replicas,
			"ready", sts.Status.ReadyReplicas,
			"attempt", i+1,
			"maxAttempts", maxAttempts)

		// Wait before checking again
		time.Sleep(5 * time.Second)
	}

	return fmt.Errorf("timed out waiting for StatefulSet %s to scale down", sts.Name)
}

// SetupWithManager sets up the controller with the Manager.
func (r *TradeBotReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Add indexes for faster lookups
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &freqtradev1alpha1.TradeBot{}, "spec.exchangeRef", func(obj client.Object) []string {
		tradeBot := obj.(*freqtradev1alpha1.TradeBot)
		return []string{tradeBot.Spec.ExchangeRef}
	}); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &freqtradev1alpha1.TradeBot{}, "spec.strategyRef", func(obj client.Object) []string {
		tradeBot := obj.(*freqtradev1alpha1.TradeBot)
		return []string{tradeBot.Spec.StrategyRef}
	}); err != nil {
		return err
	}

	// Setup watches for all referenced resources
	return ctrl.NewControllerManagedBy(mgr).
		For(&freqtradev1alpha1.TradeBot{}).
		// Watch owned resources
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&appsv1.StatefulSet{}).
		// Watch Exchange changes and enqueue TradeBots referencing them
		Watches(
			&freqtradev1alpha1.Exchange{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
				var reqs []reconcile.Request
				var bots freqtradev1alpha1.TradeBotList
				if err := mgr.GetClient().List(ctx, &bots, client.MatchingFields{
					"spec.exchangeRef": obj.GetName(),
				}); err != nil {
					mgr.GetLogger().Error(err, "Failed to list TradeBots for Exchange", "exchange", obj.GetName())
					return nil
				}
				for _, bot := range bots.Items {
					if bot.Namespace == obj.GetNamespace() {
						reqs = append(reqs, reconcile.Request{
							NamespacedName: types.NamespacedName{
								Name:      bot.Name,
								Namespace: bot.Namespace,
							},
						})
					}
				}
				return reqs
			}),
		).
		// Watch Strategy changes and enqueue TradeBots referencing them
		Watches(
			&freqtradev1alpha1.Strategy{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
				var reqs []reconcile.Request
				var bots freqtradev1alpha1.TradeBotList
				if err := mgr.GetClient().List(ctx, &bots, client.MatchingFields{
					"spec.strategyRef": obj.GetName(),
				}); err != nil {
					mgr.GetLogger().Error(err, "Failed to list TradeBots for Strategy", "strategy", obj.GetName())
					return nil
				}
				for _, bot := range bots.Items {
					if bot.Namespace == obj.GetNamespace() {
						reqs = append(reqs, reconcile.Request{
							NamespacedName: types.NamespacedName{
								Name:      bot.Name,
								Namespace: bot.Namespace,
							},
						})
					}
				}
				return reqs
			}),
		).
		// Watch Pairlists changes and enqueue TradeBots referencing them
		Watches(
			&freqtradev1alpha1.PairlistMethods{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
				var reqs []reconcile.Request
				var bots freqtradev1alpha1.TradeBotList
				if err := mgr.GetClient().List(ctx, &bots); err != nil {
					mgr.GetLogger().Error(err, "Failed to list TradeBots for PairlistMethods", "pairlistMethods", obj.GetName())
					return nil
				}
				for _, bot := range bots.Items {
					if bot.Spec.PairlistMethodsRef == obj.GetName() && bot.Namespace == obj.GetNamespace() {
						reqs = append(reqs, reconcile.Request{
							NamespacedName: types.NamespacedName{
								Name:      bot.Name,
								Namespace: bot.Namespace,
							},
						})
					}
				}
				return reqs
			}),
		).
		// Watch EntryPricing changes
		Watches(
			&freqtradev1alpha1.EntryPricing{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
				var reqs []reconcile.Request
				var bots freqtradev1alpha1.TradeBotList
				if err := mgr.GetClient().List(ctx, &bots); err != nil {
					mgr.GetLogger().Error(err, "Failed to list TradeBots for EntryPricing", "entrypricing", obj.GetName())
					return nil
				}
				for _, bot := range bots.Items {
					if bot.Spec.EntryPricingRef == obj.GetName() && bot.Namespace == obj.GetNamespace() {
						reqs = append(reqs, reconcile.Request{
							NamespacedName: types.NamespacedName{
								Name:      bot.Name,
								Namespace: bot.Namespace,
							},
						})
					}
				}
				return reqs
			}),
		).
		// Watch ExitPricing changes
		Watches(
			&freqtradev1alpha1.ExitPricing{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
				var reqs []reconcile.Request
				var bots freqtradev1alpha1.TradeBotList
				if err := mgr.GetClient().List(ctx, &bots); err != nil {
					mgr.GetLogger().Error(err, "Failed to list TradeBots for ExitPricing", "exitpricing", obj.GetName())
					return nil
				}
				for _, bot := range bots.Items {
					if bot.Spec.ExitPricingRef == obj.GetName() && bot.Namespace == obj.GetNamespace() {
						reqs = append(reqs, reconcile.Request{
							NamespacedName: types.NamespacedName{
								Name:      bot.Name,
								Namespace: bot.Namespace,
							},
						})
					}
				}
				return reqs
			}),
		).
		// Watch OrderTypes changes
		Watches(
			&freqtradev1alpha1.OrderTypes{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
				var reqs []reconcile.Request
				var bots freqtradev1alpha1.TradeBotList
				if err := mgr.GetClient().List(ctx, &bots); err != nil {
					mgr.GetLogger().Error(err, "Failed to list TradeBots for OrderTypes", "ordertypes", obj.GetName())
					return nil
				}
				for _, bot := range bots.Items {
					if bot.Spec.OrderTypesRef == obj.GetName() && bot.Namespace == obj.GetNamespace() {
						reqs = append(reqs, reconcile.Request{
							NamespacedName: types.NamespacedName{
								Name:      bot.Name,
								Namespace: bot.Namespace,
							},
						})
					}
				}
				return reqs
			}),
		).
		// Watch RiskManagement changes
		Watches(
			&freqtradev1alpha1.RiskManagement{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
				var reqs []reconcile.Request
				var bots freqtradev1alpha1.TradeBotList
				if err := mgr.GetClient().List(ctx, &bots); err != nil {
					mgr.GetLogger().Error(err, "Failed to list TradeBots for RiskManagement", "riskmanagement", obj.GetName())
					return nil
				}
				for _, bot := range bots.Items {
					if bot.Spec.RiskManagementRef == obj.GetName() && bot.Namespace == obj.GetNamespace() {
						reqs = append(reqs, reconcile.Request{
							NamespacedName: types.NamespacedName{
								Name:      bot.Name,
								Namespace: bot.Namespace,
							},
						})
					}
				}
				return reqs
			}),
		).
		// Watch Notification changes
		Watches(
			&freqtradev1alpha1.Notification{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
				var reqs []reconcile.Request
				var bots freqtradev1alpha1.TradeBotList
				if err := mgr.GetClient().List(ctx, &bots); err != nil {
					mgr.GetLogger().Error(err, "Failed to list TradeBots for Notification", "notification", obj.GetName())
					return nil
				}
				for _, bot := range bots.Items {
					if bot.Spec.NotificationRef == obj.GetName() && bot.Namespace == obj.GetNamespace() {
						reqs = append(reqs, reconcile.Request{
							NamespacedName: types.NamespacedName{
								Name:      bot.Name,
								Namespace: bot.Namespace,
							},
						})
					}
				}
				return reqs
			}),
		).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 4, // Allow multiple reconciles in parallel
		}).
		Complete(r)
}
