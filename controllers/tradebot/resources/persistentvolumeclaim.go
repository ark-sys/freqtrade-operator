package resources

import (
	"context"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// BuildUserDataPVC creates a PVC for the bot's user_data directory
func BuildUserDataPVC(tradeBot freqtradev1alpha1.TradeBot) corev1.PersistentVolumeClaim {
	storageSize := resource.MustParse("1Gi") // Default size
	if tradeBot.Spec.Deployment != nil && tradeBot.Spec.Deployment.StorageSize != "" {
		storageSize = resource.MustParse(tradeBot.Spec.Deployment.StorageSize)
	}

	// Default storage class
	storageClassName := "standard"
	if tradeBot.Spec.Deployment != nil && tradeBot.Spec.Deployment.StorageClassName != "" {
		storageClassName = tradeBot.Spec.Deployment.StorageClassName
	}

	return corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tradeBot.Name + "-user-data",
			Namespace: tradeBot.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(&tradeBot, freqtradev1alpha1.GroupVersion.WithKind("TradeBot")),
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: storageSize,
				},
			},
			StorageClassName: &storageClassName,
		},
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
	return nil
}
