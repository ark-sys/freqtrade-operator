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
	go func() { done <- waitForMainContainerExit(context.Background(), context.Background(), c, opts) }()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("waitForMainContainerExit did not return once the container was already terminated")
	}
}

// Regression test for the actual production bug (found on a real cluster, not envtest): kubelet
// sends this sidecar SIGTERM the instant the main container exits, since that's the whole native
// sidecar contract - but kubelet's own exit detection is far faster than this loop's own
// pollInterval-paced polling of the Kubernetes API for the same fact. Left unhandled, SIGTERM's
// default Go disposition (immediate death) reliably won that race, so the extraction step never
// ran at all. sigCtx cancellation must trigger an immediate recheck rather than waiting out the
// rest of pollInterval - proven here by setting pollInterval to an hour, so a fast return can
// only be explained by the signal path, never by an ordinary tick.
func TestWaitForMainContainerExit_SignalTriggersImmediateRecheck(t *testing.T) {
	orig := pollInterval
	pollInterval = time.Hour
	defer func() { pollInterval = orig }()

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

	sigCtx, cancel := context.WithCancel(context.Background())
	opts := Options{PodName: "my-run-pod", Namespace: "trading", MainContainerName: "freqtrade"}
	done := make(chan error, 1)
	go func() { done <- waitForMainContainerExit(context.Background(), sigCtx, c, opts) }()

	// Confirm it's genuinely still waiting (container not yet terminated) before simulating the
	// signal - otherwise a fast return below wouldn't actually prove anything about the signal
	// path specifically.
	select {
	case err := <-done:
		t.Fatalf("expected waitForMainContainerExit to keep waiting while the container is still running, got: %v", err)
	case <-time.After(200 * time.Millisecond):
	}

	pod.Status.ContainerStatuses[0].State = corev1.ContainerState{
		Terminated: &corev1.ContainerStateTerminated{ExitCode: 0},
	}
	if err := c.Status().Update(context.Background(), pod); err != nil {
		t.Fatalf("failed to update pod status: %v", err)
	}
	cancel() // simulates SIGTERM arriving

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("waitForMainContainerExit did not react promptly to the signal - " +
			"looks like it fell back to waiting out pollInterval (set to an hour in this test)")
	}
}

// A signal that arrives before the main container has actually exited (in principle possible,
// even if kubelet's own SIGTERM to this sidecar is in practice always trustworthy) must not be
// treated as fatal - waitForMainContainerExit should just fall back to normal polling and
// eventually succeed once the container does terminate.
func TestWaitForMainContainerExit_SpuriousSignalFallsBackToPolling(t *testing.T) {
	orig := pollInterval
	pollInterval = 20 * time.Millisecond
	defer func() { pollInterval = orig }()

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

	sigCtx, cancel := context.WithCancel(context.Background())
	opts := Options{PodName: "my-run-pod", Namespace: "trading", MainContainerName: "freqtrade"}
	done := make(chan error, 1)
	go func() { done <- waitForMainContainerExit(context.Background(), sigCtx, c, opts) }()

	cancel() // a signal, but the container has not exited yet
	time.Sleep(100 * time.Millisecond)

	pod.Status.ContainerStatuses[0].State = corev1.ContainerState{
		Terminated: &corev1.ContainerStateTerminated{ExitCode: 0},
	}
	if err := c.Status().Update(context.Background(), pod); err != nil {
		t.Fatalf("failed to update pod status: %v", err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected normal polling to pick up the eventual termination after a spurious signal")
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
