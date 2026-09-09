package collectresults

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func discardLogger(t *testing.T) logr.Logger {
	t.Helper()
	return logr.Discard()
}

func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add corev1 to scheme: %v", err)
	}
	return scheme
}

func TestWaitForMainContainerExit_ReturnsOnceTerminated(t *testing.T) {
	scheme := newTestScheme(t)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "my-run-pod", Namespace: "trading"},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{
				{Name: "freqtrade", State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0}}},
			},
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pod).WithStatusSubresource(pod).Build()

	opts := Options{PodName: "my-run-pod", Namespace: "trading", MainContainerName: "freqtrade"}
	done := make(chan error, 1)
	go func() { done <- waitForMainContainerExit(context.Background(), c, opts) }()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("waitForMainContainerExit did not return once the container was already terminated")
	}
}

func TestWaitForMainContainerExit_ContextCancelled(t *testing.T) {
	scheme := newTestScheme(t)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "my-run-pod", Namespace: "trading"},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{
				{Name: "freqtrade", State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}},
			},
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pod).WithStatusSubresource(pod).Build()

	ctx, cancel := context.WithCancel(context.Background())
	opts := Options{PodName: "my-run-pod", Namespace: "trading", MainContainerName: "freqtrade"}
	done := make(chan error, 1)
	go func() { done <- waitForMainContainerExit(ctx, c, opts) }()

	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Error("expected a context-cancellation error, got nil")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("waitForMainContainerExit did not return promptly after context cancellation")
	}
}

func TestExtract_WritesErrorKeyWhenResultFileMissing(t *testing.T) {
	opts := Options{ResultsDir: t.TempDir(), StrategyName: "SampleStrategy"}
	data := extract(opts, discardLogger(t))

	if _, hasResults := data[resultsConfigMapDataKey]; hasResults {
		t.Errorf("expected no %s key when the result file is missing, got %v", resultsConfigMapDataKey, data)
	}
	if _, hasError := data[errorConfigMapDataKey]; !hasError {
		t.Errorf("expected an %s key when the result file is missing, got %v", errorConfigMapDataKey, data)
	}
}
