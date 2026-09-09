package resources

import (
	"testing"

	freqtradev1beta1 "github.com/ark-sys/freqtrade-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildResultsPVC_DefaultSizeWhenResultsUnset(t *testing.T) {
	backtest := freqtradev1beta1.Backtest{ObjectMeta: metav1.ObjectMeta{Name: "my-run", Namespace: testNamespace}}
	pvc := BuildResultsPVC(backtest)

	if pvc.Name != "my-run-results" {
		t.Errorf("expected name my-run-results, got %s", pvc.Name)
	}
	got := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
	want := resource.MustParse("1Gi")
	if got.Cmp(want) != 0 {
		t.Errorf("expected default size 1Gi, got %s", got.String())
	}
}

func TestBuildResultsPVC_ExplicitSizeAndStorageClass(t *testing.T) {
	sc := "fast-ssd"
	backtest := freqtradev1beta1.Backtest{
		ObjectMeta: metav1.ObjectMeta{Name: "my-run"},
		Spec: freqtradev1beta1.BacktestSpec{
			RunSpec: freqtradev1beta1.RunSpec{
				Results: &freqtradev1beta1.ResultsSpec{Size: resource.MustParse("5Gi"), StorageClassName: &sc},
			},
		},
	}
	pvc := BuildResultsPVC(backtest)

	got := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
	want := resource.MustParse("5Gi")
	if got.Cmp(want) != 0 {
		t.Errorf("expected explicit size 5Gi, got %s", got.String())
	}
	if pvc.Spec.StorageClassName == nil || *pvc.Spec.StorageClassName != "fast-ssd" {
		t.Errorf("expected storage class fast-ssd, got %v", pvc.Spec.StorageClassName)
	}
}

func TestResultsRetentionPolicy_DefaultsToDelete(t *testing.T) {
	backtest := &freqtradev1beta1.Backtest{}
	if got := ResultsRetentionPolicy(backtest); got != "Delete" {
		t.Errorf("expected default RetentionPolicy Delete, got %q", got)
	}
}

func TestResultsRetentionPolicy_ExplicitRetain(t *testing.T) {
	backtest := &freqtradev1beta1.Backtest{
		Spec: freqtradev1beta1.BacktestSpec{
			RunSpec: freqtradev1beta1.RunSpec{Results: &freqtradev1beta1.ResultsSpec{RetentionPolicy: "Retain"}},
		},
	}
	if got := ResultsRetentionPolicy(backtest); got != "Retain" {
		t.Errorf("expected explicit RetentionPolicy Retain, got %q", got)
	}
}
