package backtest

import (
	"context"
	"testing"

	freqtradev1beta1 "github.com/ark-sys/freqtrade-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newFinalizerTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add corev1 to scheme: %v", err)
	}
	if err := freqtradev1beta1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add v1beta1 to scheme: %v", err)
	}
	return scheme
}

func TestFinalizeBacktest_DeletePolicyIsANoOp(t *testing.T) {
	scheme := newFinalizerTestScheme(t)
	backtest := &freqtradev1beta1.Backtest{
		ObjectMeta: metav1.ObjectMeta{Name: "my-run", Namespace: "trading", UID: "abc"},
		// RetentionPolicy unset - defaults to Delete.
	}
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-run-results", Namespace: "trading",
			OwnerReferences: []metav1.OwnerReference{{UID: "abc", Name: "my-run"}},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")},
			},
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(backtest, pvc).Build()
	r := &Reconciler{Client: c}

	if err := r.finalizeBacktest(context.Background(), backtest); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got corev1.PersistentVolumeClaim
	key := types.NamespacedName{Name: "my-run-results", Namespace: "trading"}
	if err := c.Get(context.Background(), key, &got); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(got.OwnerReferences) != 1 {
		t.Errorf("expected the owner reference to be left alone under RetentionPolicy: Delete, got %+v", got.OwnerReferences)
	}
}

func TestFinalizeBacktest_RetainStripsOwnerReferenceAndAnnotates(t *testing.T) {
	scheme := newFinalizerTestScheme(t)
	backtest := &freqtradev1beta1.Backtest{
		ObjectMeta: metav1.ObjectMeta{Name: "my-run", Namespace: "trading", UID: "abc"},
		Spec: freqtradev1beta1.BacktestSpec{
			RunSpec: freqtradev1beta1.RunSpec{Results: &freqtradev1beta1.ResultsSpec{RetentionPolicy: "Retain"}},
		},
	}
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-run-results", Namespace: "trading",
			OwnerReferences: []metav1.OwnerReference{{UID: "abc", Name: "my-run"}, {UID: "other", Name: "someone-else"}},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")},
			},
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(backtest, pvc).Build()
	r := &Reconciler{Client: c}

	if err := r.finalizeBacktest(context.Background(), backtest); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got corev1.PersistentVolumeClaim
	key := types.NamespacedName{Name: "my-run-results", Namespace: "trading"}
	if err := c.Get(context.Background(), key, &got); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(got.OwnerReferences) != 1 || got.OwnerReferences[0].UID != "other" {
		t.Errorf("expected only the Backtest's own owner reference removed, got %+v", got.OwnerReferences)
	}
	if got.Annotations["freqtrade.io/preserved-from"] != "my-run" {
		t.Errorf("expected preserved-from annotation, got %v", got.Annotations)
	}
	if _, ok := got.Annotations["freqtrade.io/preserved-at"]; !ok {
		t.Error("expected preserved-at annotation to be set")
	}
}

func TestFinalizeBacktest_RetainWithMissingPVCIsANoOp(t *testing.T) {
	scheme := newFinalizerTestScheme(t)
	backtest := &freqtradev1beta1.Backtest{
		ObjectMeta: metav1.ObjectMeta{Name: "my-run", Namespace: "trading", UID: "abc"},
		Spec: freqtradev1beta1.BacktestSpec{
			RunSpec: freqtradev1beta1.RunSpec{Results: &freqtradev1beta1.ResultsSpec{RetentionPolicy: "Retain"}},
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(backtest).Build()
	r := &Reconciler{Client: c}

	if err := r.finalizeBacktest(context.Background(), backtest); err != nil {
		t.Fatalf("expected a missing PVC to be a no-op, got error: %v", err)
	}
}
