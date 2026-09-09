// controllers/backtest/resources/pvc.go
package resources

import (
	freqtradev1beta1 "github.com/ark-sys/freqtrade-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// defaultResultsSize applies when spec.results is unset entirely - matches
// the +kubebuilder:default on ResultsSpec.Size itself; used only by tests
// and callers that build a Backtest without going through admission
// defaulting.
var defaultResultsSize = resource.MustParse("1Gi")

// BuildResultsPVC creates the per-run results PVC (D1) - RWO is enough
// (only this run's own Job pod, and later its sidecar, ever mount it; the
// operator itself reads results via the P6-2 ConfigMap, never by mounting
// the volume directly).
func BuildResultsPVC(backtest freqtradev1beta1.Backtest) corev1.PersistentVolumeClaim {
	size := defaultResultsSize
	var storageClassName *string
	if r := backtest.Spec.Results; r != nil {
		if !r.Size.IsZero() {
			size = r.Size
		}
		storageClassName = r.StorageClassName
	}

	return corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: backtest.Name + "-results", Namespace: backtest.Namespace},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			StorageClassName: storageClassName,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: size},
			},
		},
	}
}

// ResultsRetentionPolicy returns spec.results.retentionPolicy, or the
// +kubebuilder default ("Delete") for a Backtest with no explicit value -
// the same "admission defaults it, but don't assume every caller went
// through admission" stance used elsewhere in this operator (e.g.
// controllers/tradebot/poller.go's shouldPoll).
func ResultsRetentionPolicy(backtest *freqtradev1beta1.Backtest) string {
	if backtest.Spec.Results != nil && backtest.Spec.Results.RetentionPolicy != "" {
		return backtest.Spec.Results.RetentionPolicy
	}
	return "Delete"
}
