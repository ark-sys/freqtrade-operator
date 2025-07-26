package tradebot

import (
	"context"
	"time"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// finalizeTradeBot performs cleanup operations when a TradeBot is being deleted
func (r *Reconciler) finalizeTradeBot(ctx context.Context, tradeBot *freqtradev1alpha1.TradeBot) error {
	logger := log.FromContext(ctx)
	logger.Info("Finalizing TradeBot", "name", tradeBot.Name)

	// 1. Get the StatefulSet to check if it exists
	var sts appsv1.StatefulSet
	stsName := types.NamespacedName{Name: tradeBot.Name, Namespace: tradeBot.Namespace}
	stsErr := r.Get(ctx, stsName, &sts)

	// 2. If the StatefulSet exists, scale it down to 0 to ensure graceful termination
	if stsErr == nil {
		logger.Info("Scaling down StatefulSet before deletion", "name", sts.Name)

		// Only scale down if not already at 0
		if sts.Spec.Replicas == nil || *sts.Spec.Replicas > 0 {
			replicas := int32(0)
			sts.Spec.Replicas = &replicas
			if err := r.Update(ctx, &sts); err != nil {
				// If update fails due to conflict, try to get latest version
				if errors.IsConflict(err) {
					logger.Info("Conflict updating StatefulSet, retrying with latest version")
					var latestSts appsv1.StatefulSet
					if getErr := r.Get(ctx, stsName, &latestSts); getErr != nil {
						if errors.IsNotFound(getErr) {
							// StatefulSet is gone, continue
							logger.Info("StatefulSet no longer exists, continuing finalization")
						} else {
							logger.Error(getErr, "Failed to get latest StatefulSet")
							return getErr
						}
					} else {
						// Try scaling down the latest version
						replicas := int32(0)
						latestSts.Spec.Replicas = &replicas
						if updateErr := r.Update(ctx, &latestSts); updateErr != nil {
							logger.Error(updateErr, "Failed to scale down StatefulSet on retry")
							return updateErr
						}
					}
				} else {
					logger.Error(err, "Failed to scale down StatefulSet")
					return err
				}
			}

			// Wait for the StatefulSet to scale down, but with timeout
			if err := r.waitForStatefulSetScaleDown(ctx, stsName); err != nil {
				logger.Error(err, "Failed to wait for StatefulSet scale down, but continuing cleanup")
				// Don't return error here - we want to continue with cleanup even if scaling fails
			}
		}
		logger.Info("StatefulSet scaled down successfully", "name", sts.Name)
	} else if !errors.IsNotFound(stsErr) {
		logger.Error(stsErr, "Failed to get StatefulSet during finalization")
		return stsErr
	}

	// 3. Handle PVC preservation logic
	if err := r.handlePVCPreservation(ctx, tradeBot); err != nil {
		logger.Error(err, "Failed to handle PVC preservation, but continuing cleanup")
		// Don't return error here - we don't want PVC issues to block deletion
	}

	logger.Info("TradeBot finalization completed successfully", "name", tradeBot.Name)
	return nil
}

// handlePVCPreservation handles the PVC preservation logic
func (r *Reconciler) handlePVCPreservation(ctx context.Context, tradeBot *freqtradev1alpha1.TradeBot) error {
	logger := log.FromContext(ctx)

	// Check if we need to preserve the PVC
	if preserveData, exists := tradeBot.Annotations["freqtrade.io/preserve-data"]; exists && preserveData == "true" {
		logger.Info("Preserving PVC as requested by annotation", "name", tradeBot.Name)

		// Find the PVC
		pvcName := tradeBot.Name + "-user-data"
		var pvc corev1.PersistentVolumeClaim
		err := r.Get(ctx, types.NamespacedName{Name: pvcName, Namespace: tradeBot.Namespace}, &pvc)
		if err != nil {
			if errors.IsNotFound(err) {
				logger.Info("PVC not found, nothing to preserve", "name", pvcName)
				return nil
			}
			logger.Error(err, "Failed to get PVC during finalization")
			return err
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

	return nil
}

// waitForStatefulSetScaleDown waits for the StatefulSet to scale down to 0 replicas
func (r *Reconciler) waitForStatefulSetScaleDown(ctx context.Context, namespacedName types.NamespacedName) error {
	logger := log.FromContext(ctx)

	// Create a timeout context - don't wait forever
	timeoutCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeoutCtx.Done():
			logger.Info("Timeout waiting for StatefulSet to scale down, continuing with cleanup")
			return nil // Don't block deletion on timeout
		case <-ticker.C:
			var sts appsv1.StatefulSet
			if err := r.Get(ctx, namespacedName, &sts); err != nil {
				if errors.IsNotFound(err) {
					// StatefulSet is gone, which is fine
					logger.Info("StatefulSet no longer exists")
					return nil
				}
				logger.Error(err, "Failed to get StatefulSet while waiting for scale down")
				return nil // Don't block on transient errors
			}

			// Check if the StatefulSet is scaled down
			if sts.Status.Replicas == 0 && sts.Status.ReadyReplicas == 0 {
				logger.Info("StatefulSet is fully scaled down", "name", sts.Name)
				return nil
			}

			// Log progress
			logger.V(1).Info("Still waiting for StatefulSet to scale down",
				"name", sts.Name,
				"current", sts.Status.Replicas,
				"ready", sts.Status.ReadyReplicas)
		}
	}
}
