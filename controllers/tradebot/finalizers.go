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

// defaultFinalizerGracePeriod is how long finalizeTradeBot waits for the
// StatefulSet to scale down before giving up and removing the finalizer
// anyway, when Reconciler.FinalizerGracePeriod is unset.
const defaultFinalizerGracePeriod = 2 * time.Minute

// finalizeTradeBot drives StatefulSet scale-down and PVC preservation
// forward by one step and reports whether cleanup is complete. It never
// blocks: MaxConcurrentReconciles is 1, so a reconcile that waits inline
// for the StatefulSet to actually scale down would stall every other
// TradeBot's reconciliation for as long as it waits. The caller is expected
// to requeue when this returns (false, nil) and re-check on the next pass.
func (r *Reconciler) finalizeTradeBot(ctx context.Context, tradeBot *freqtradev1alpha1.TradeBot) (bool, error) {
	logger := log.FromContext(ctx)
	logger.V(1).Info("Finalizing TradeBot", "name", tradeBot.Name)

	var sts appsv1.StatefulSet
	stsName := types.NamespacedName{Name: tradeBot.Name, Namespace: tradeBot.Namespace}
	err := r.Get(ctx, stsName, &sts)

	switch {
	case errors.IsNotFound(err):
		// Nothing to scale down.
	case err != nil:
		logger.Error(err, "Failed to get StatefulSet during finalization")
		return false, err
	default:
		scaledDown, err := r.ensureStatefulSetScaledDown(ctx, &sts)
		if err != nil {
			return false, err
		}
		if !scaledDown {
			if r.finalizerDeadlinePassed(tradeBot) {
				// TODO(P4-1): also record a warning Event once every
				// controller has an EventRecorder wired up.
				logger.Info("StatefulSet did not scale down within the grace period, removing finalizer anyway",
					"name", sts.Name, "gracePeriod", r.finalizerGracePeriod())
			} else {
				return false, nil
			}
		}
	}

	if err := r.handlePVCPreservation(ctx, tradeBot); err != nil {
		logger.Error(err, "Failed to handle PVC preservation, but continuing cleanup")
		// Don't block deletion on PVC issues.
	}

	logger.Info("TradeBot finalization completed successfully", "name", tradeBot.Name)
	return true, nil
}

// ensureStatefulSetScaledDown requests replicas=0 if that hasn't been
// requested yet, and reports whether the StatefulSet has actually finished
// scaling down. A scale-down request just issued has, by definition, not
// taken effect yet, so it always reports false - the next reconcile (driven
// by the caller's requeue) observes the result via a fresh Get.
func (r *Reconciler) ensureStatefulSetScaledDown(ctx context.Context, sts *appsv1.StatefulSet) (bool, error) {
	logger := log.FromContext(ctx)

	if sts.Spec.Replicas == nil || *sts.Spec.Replicas != 0 {
		logger.V(1).Info("Scaling down StatefulSet before deletion", "name", sts.Name)
		replicas := int32(0)
		sts.Spec.Replicas = &replicas
		if err := r.Update(ctx, sts); err != nil && !errors.IsConflict(err) {
			logger.Error(err, "Failed to scale down StatefulSet")
			return false, err
		}
		// A conflict just means someone else updated it concurrently; the
		// next reconcile's Get will see the current state either way.
		return false, nil
	}

	return sts.Status.Replicas == 0 && sts.Status.ReadyReplicas == 0, nil
}

func (r *Reconciler) finalizerGracePeriod() time.Duration {
	if r.FinalizerGracePeriod > 0 {
		return r.FinalizerGracePeriod
	}
	return defaultFinalizerGracePeriod
}

func (r *Reconciler) finalizerDeadlinePassed(tradeBot *freqtradev1alpha1.TradeBot) bool {
	dt := tradeBot.GetDeletionTimestamp()
	if dt == nil {
		return false
	}
	return time.Since(dt.Time) > r.finalizerGracePeriod()
}

// handlePVCPreservation handles the PVC preservation logic
func (r *Reconciler) handlePVCPreservation(ctx context.Context, tradeBot *freqtradev1alpha1.TradeBot) error {
	logger := log.FromContext(ctx)

	// Check if we need to preserve the PVC
	if preserveData, exists := tradeBot.Annotations["freqtrade.io/preserve-data"]; exists && preserveData == "true" {
		logger.V(1).Info("Preserving PVC as requested by annotation", "name", tradeBot.Name)

		// Find the PVC
		pvcName := tradeBot.Name + "-user-data"
		var pvc corev1.PersistentVolumeClaim
		err := r.Get(ctx, types.NamespacedName{Name: pvcName, Namespace: tradeBot.Namespace}, &pvc)
		if err != nil {
			if errors.IsNotFound(err) {
				logger.V(3).Info("PVC not found, nothing to preserve", "name", pvcName)
				return nil
			}
			logger.V(1).Error(err, "Failed to get PVC during finalization")
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
			logger.V(1).Error(err, "Failed to update PVC to preserve it")
			return err
		}
		logger.V(1).Info("Successfully preserved PVC", "name", pvcName)
	} else {
		logger.V(2).Info("PVC will be garbage collected as no preserve annotation was found")
	}

	return nil
}
