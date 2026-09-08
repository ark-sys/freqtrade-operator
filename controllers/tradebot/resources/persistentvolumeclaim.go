package resources

import (
	"context"
	"reflect"
	"time"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// mergePVCSpecOverrides merges user overrides from App.PVCSpec into the default PVC spec.
func mergePVCSpecOverrides(defaultSpec corev1.PersistentVolumeClaimSpec, override *freqtradev1alpha1.PVCSpec) corev1.PersistentVolumeClaimSpec {
	if override == nil {
		return defaultSpec
	}

	if len(override.AccessModes) > 0 {
		defaultSpec.AccessModes = override.AccessModes
	}
	if override.StorageSize != nil {
		defaultSpec.Resources.Requests[corev1.ResourceStorage] = *override.StorageSize
	}
	if override.StorageClassName != "" {
		defaultSpec.StorageClassName = &override.StorageClassName
	}
	if override.VolumeName != "" {
		defaultSpec.VolumeName = override.VolumeName
	}
	return defaultSpec
}

// BuildUserDataPVC creates a PVC for the bot's user_data directory
func BuildUserDataPVC(tradeBot freqtradev1alpha1.TradeBot) corev1.PersistentVolumeClaim {
	defaultStorageSize := resource.MustParse("1Gi")
	defaultStorageClassName := "standard"

	basePVCSpec := corev1.PersistentVolumeClaimSpec{
		AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
		Resources: corev1.VolumeResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: defaultStorageSize,
			},
		},
		StorageClassName: &defaultStorageClassName,
	}

	finalSpec := basePVCSpec
	var annotations map[string]string
	var labels map[string]string

	if tradeBot.Spec.App != nil && tradeBot.Spec.App.PVCSpec != nil {
		finalSpec = mergePVCSpecOverrides(basePVCSpec, tradeBot.Spec.App.PVCSpec)
		if len(tradeBot.Spec.App.PVCSpec.Annotations) > 0 {
			annotations = tradeBot.Spec.App.PVCSpec.Annotations
		}
		if len(tradeBot.Spec.App.PVCSpec.Labels) > 0 {
			labels = tradeBot.Spec.App.PVCSpec.Labels
		}
	}

	return corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        tradeBot.Name + "-user-data",
			Namespace:   tradeBot.Namespace,
			Annotations: annotations,
			Labels:      labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(&tradeBot, freqtradev1alpha1.GroupVersion.WithKind("TradeBot")),
			},
		},
		Spec: finalSpec,
	}
}

// ApplyPVC creates or updates the PVC with retry logic for resource version conflicts
func ApplyPVC(ctx context.Context, c client.Client, pvc *corev1.PersistentVolumeClaim) error {
	logger := log.FromContext(ctx)

	// Retry logic for resource version conflicts
	return wait.PollUntilContextTimeout(ctx, 100*time.Millisecond, 2*time.Second, true, func(ctx context.Context) (bool, error) {
		var existing corev1.PersistentVolumeClaim
		err := c.Get(ctx, types.NamespacedName{Name: pvc.Name, Namespace: pvc.Namespace}, &existing)
		if errors.IsNotFound(err) {
			err = c.Create(ctx, pvc)
			if errors.IsAlreadyExists(err) {
				logger.V(2).Info("PVC already exists, retrying", "name", pvc.Name)
				return false, nil
			}
			return true, err
		} else if err != nil {
			return true, err
		}

		needsUpdate := false

		// Only allow increasing storage
		existingQty := existing.Spec.Resources.Requests[corev1.ResourceStorage]
		newQty := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
		if newQty.Cmp(existingQty) > 0 {
			existing.Spec.Resources.Requests[corev1.ResourceStorage] = newQty
			needsUpdate = true
		}

		// Update labels and annotations if they differ
		if !reflect.DeepEqual(existing.Labels, pvc.Labels) {
			existing.Labels = pvc.Labels
			needsUpdate = true
		}
		if !reflect.DeepEqual(existing.Annotations, pvc.Annotations) {
			existing.Annotations = pvc.Annotations
			needsUpdate = true
		}

		if needsUpdate {
			err = c.Update(ctx, &existing)
			if errors.IsConflict(err) {
				logger.V(2).Info("PVC resource version conflict, retrying", "name", pvc.Name)
				return false, nil
			}
			return true, err
		}

		return true, nil
	})
}
