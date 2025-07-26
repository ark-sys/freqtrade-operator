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
		return c.Update(ctx, &existing)
	}

	return nil
}
