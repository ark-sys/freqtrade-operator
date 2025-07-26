package resources

import (
	"context"
	"reflect"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	"github.com/ark-sys/freqtrade-operator/controllers/shared"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// BuildUserDataPVC creates a PVC for the bot's user_data directory
func BuildUserDataPVC(tradeBot freqtradev1alpha1.TradeBot) corev1.PersistentVolumeClaim {
	// Default PVC spec
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

	// Merge with user-provided PVC configuration
	finalSpec := basePVCSpec
	if tradeBot.Spec.App != nil && !reflect.DeepEqual(tradeBot.Spec.App.PVCSpec, corev1.PersistentVolumeClaimSpec{}) {
		finalSpec = shared.MergeSpecsWithStrategicPatch(basePVCSpec, tradeBot.Spec.App.PVCSpec, &corev1.PersistentVolumeClaimSpec{})
	}

	return corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tradeBot.Name + "-user-data",
			Namespace: tradeBot.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(&tradeBot, freqtradev1alpha1.GroupVersion.WithKind("TradeBot")),
			},
		},
		Spec: finalSpec,
	}
}

// ApplyPVC creates or updates the PVC
func ApplyPVC(ctx context.Context, c client.Client, pvc *corev1.PersistentVolumeClaim) error {
	var existing corev1.PersistentVolumeClaim
	err := c.Get(ctx, types.NamespacedName{Name: pvc.Name, Namespace: pvc.Namespace}, &existing)
	if errors.IsNotFound(err) {
		return c.Create(ctx, pvc)
	} else if err != nil {
		return err
	}
	// PVC is immutable except for spec.resources.requests.storage (which can only be increased)
	// and metadata.labels/annotations, so we don't update it if it exists
	if !reflect.DeepEqual(existing.Spec.Resources.Requests, pvc.Spec.Resources.Requests) {
		// Verify that new storage request is greater than existing
		quantity := existing.Spec.Resources.Requests[corev1.ResourceStorage]
		if quantity.Cmp(pvc.Spec.Resources.Requests[corev1.ResourceStorage]) >= 0 {
			return nil // No update needed, existing storage is sufficient
		} else {
			// Update the storage request
			quantity = pvc.Spec.Resources.Requests[corev1.ResourceStorage]
			if err := c.Update(ctx, &existing); err != nil {
				return err
			}
		}

	}

	// Update labels and annotations if they differ
	if !reflect.DeepEqual(existing.ObjectMeta.Labels, pvc.ObjectMeta.Labels) ||
		!reflect.DeepEqual(existing.ObjectMeta.Annotations, pvc.ObjectMeta.Annotations) {
		existing.ObjectMeta.Labels = pvc.ObjectMeta.Labels
		existing.ObjectMeta.Annotations = pvc.ObjectMeta.Annotations
		if err := c.Update(ctx, &existing); err != nil {
			return err
		}
	}

	return nil
}
