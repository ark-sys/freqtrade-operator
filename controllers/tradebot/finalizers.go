package tradebot

import (
	"context"
	"fmt"
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
func (r *Reconciler) waitForStatefulSetScaleDown(ctx context.Context, sts *appsv1.StatefulSet) error {
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
