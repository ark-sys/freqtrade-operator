package resources

import (
	"testing"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildUserDataPVC_Defaults(t *testing.T) {
	tradeBot := freqtradev1alpha1.TradeBot{
		ObjectMeta: metav1.ObjectMeta{Name: "my-bot", Namespace: "trading"},
	}

	pvc := BuildUserDataPVC(tradeBot)

	if pvc.Name != "my-bot-user-data" {
		t.Errorf("expected Name %q, got %q", "my-bot-user-data", pvc.Name)
	}
	if len(pvc.OwnerReferences) != 1 || pvc.OwnerReferences[0].Name != "my-bot" {
		t.Errorf("expected a single owner reference to my-bot, got %+v", pvc.OwnerReferences)
	}
	gotSize := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
	if gotSize.String() != "1Gi" {
		t.Errorf("expected default storage size 1Gi, got %s", gotSize.String())
	}
	if pvc.Spec.StorageClassName == nil || *pvc.Spec.StorageClassName != "standard" {
		t.Errorf("expected default storage class %q, got %v", "standard", pvc.Spec.StorageClassName)
	}
}

func TestBuildUserDataPVC_Overrides(t *testing.T) {
	tradeBot := freqtradev1alpha1.TradeBot{
		ObjectMeta: metav1.ObjectMeta{Name: "my-bot", Namespace: "trading"},
		Spec: freqtradev1alpha1.TradeBotSpec{
			App: &freqtradev1alpha1.TBAppConfig{
				PVCSpec: &freqtradev1alpha1.PVCSpec{
					AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
					StorageSize:      quantityPtr("10Gi"),
					StorageClassName: "fast-ssd",
					VolumeName:       "pv-precreated",
					Annotations:      map[string]string{"backup.example.com/exclude": "true"},
					Labels:           map[string]string{"tier": "trading"},
				},
			},
		},
	}

	pvc := BuildUserDataPVC(tradeBot)

	if pvc.Spec.AccessModes[0] != corev1.ReadWriteMany {
		t.Errorf("expected overridden access mode ReadWriteMany, got %v", pvc.Spec.AccessModes)
	}
	gotSize := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
	if gotSize.String() != "10Gi" {
		t.Errorf("expected overridden storage size 10Gi, got %s", gotSize.String())
	}
	if pvc.Spec.StorageClassName == nil || *pvc.Spec.StorageClassName != "fast-ssd" {
		t.Errorf("expected overridden storage class %q, got %v", "fast-ssd", pvc.Spec.StorageClassName)
	}
	if pvc.Spec.VolumeName != "pv-precreated" {
		t.Errorf("expected overridden volume name, got %q", pvc.Spec.VolumeName)
	}
	if pvc.Annotations["backup.example.com/exclude"] != "true" {
		t.Errorf("expected the override annotation to be set, got %v", pvc.Annotations)
	}
	if pvc.Labels["tier"] != "trading" {
		t.Errorf("expected the override label to be set, got %v", pvc.Labels)
	}
}

func TestMergePVCSpecOverrides_NilOverrideIsNoOp(t *testing.T) {
	defaultSpec := corev1.PersistentVolumeClaimSpec{
		AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
	}

	got := mergePVCSpecOverrides(defaultSpec, nil)

	if len(got.AccessModes) != 1 || got.AccessModes[0] != corev1.ReadWriteOnce {
		t.Errorf("expected the default spec unchanged, got %+v", got)
	}
}

func TestMergePVCSpecOverrides_PartialOverrideOnlyTouchesSetFields(t *testing.T) {
	defaultSpec := corev1.PersistentVolumeClaimSpec{
		AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
		Resources: corev1.VolumeResourceRequirements{
			Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")},
		},
	}

	// Only StorageClassName is overridden; AccessModes and StorageSize must be left alone.
	got := mergePVCSpecOverrides(defaultSpec, &freqtradev1alpha1.PVCSpec{StorageClassName: "fast-ssd"})

	if got.AccessModes[0] != corev1.ReadWriteOnce {
		t.Errorf("expected AccessModes untouched, got %v", got.AccessModes)
	}
	gotSize := got.Resources.Requests[corev1.ResourceStorage]
	if gotSize.String() != "1Gi" {
		t.Errorf("expected storage size untouched, got %s", gotSize.String())
	}
	if got.StorageClassName == nil || *got.StorageClassName != "fast-ssd" {
		t.Errorf("expected storage class overridden, got %v", got.StorageClassName)
	}
}

func quantityPtr(s string) *resource.Quantity {
	q := resource.MustParse(s)
	return &q
}
