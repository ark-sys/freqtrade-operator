package resources

import (
	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		},
		Spec: finalSpec,
	}
}
