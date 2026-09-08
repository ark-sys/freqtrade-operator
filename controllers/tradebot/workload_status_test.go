package tradebot

import (
	"context"
	"testing"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newWorkloadStatusScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add appsv1 to scheme: %v", err)
	}
	if err := batchv1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add batchv1 to scheme: %v", err)
	}
	if err := freqtradev1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add v1alpha1 to scheme: %v", err)
	}
	return scheme
}

// TestUpdateWorkloadStatus_TradeMode covers the acceptance criterion for
// P0-3: phase is derived from the StatefulSet on the success path, so a
// previously-set error phase clears once the workload comes up, and a
// requeue is scheduled while it's not yet ready.
func TestUpdateWorkloadStatus_TradeMode(t *testing.T) {
	tests := []struct {
		name          string
		readyReplicas int32
		wantPhase     string
		wantRequeue   bool
	}{
		{name: "not ready yet", readyReplicas: 0, wantPhase: "Pending", wantRequeue: true},
		{name: "ready", readyReplicas: 1, wantPhase: "Running", wantRequeue: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := newWorkloadStatusScheme(t)
			sts := &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{Name: "my-bot", Namespace: "trading"},
				Status:     appsv1.StatefulSetStatus{ReadyReplicas: tt.readyReplicas},
			}
			c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sts).WithStatusSubresource(sts).Build()
			r := &Reconciler{Client: c}

			tradeBot := &freqtradev1alpha1.TradeBot{
				ObjectMeta: metav1.ObjectMeta{Name: "my-bot", Namespace: "trading"},
				Status:     freqtradev1alpha1.TradeBotStatus{Phase: "ConfigError", Message: "stale error from a previous reconcile"},
			}

			requeueAfter, err := r.updateWorkloadStatus(context.Background(), tradeBot)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tradeBot.Status.Phase != tt.wantPhase {
				t.Errorf("expected phase %q, got %q", tt.wantPhase, tradeBot.Status.Phase)
			}
			if tt.wantPhase == "Running" && tradeBot.Status.Message != "" {
				t.Errorf("expected Message cleared once Running, got %q", tradeBot.Status.Message)
			}
			if (requeueAfter > 0) != tt.wantRequeue {
				t.Errorf("expected requeue=%v, got requeueAfter=%v", tt.wantRequeue, requeueAfter)
			}
		})
	}
}

func TestUpdateWorkloadStatus_JobMode(t *testing.T) {
	tests := []struct {
		name        string
		jobStatus   batchv1.JobStatus
		wantPhase   string
		wantRequeue bool
	}{
		{name: "succeeded", jobStatus: batchv1.JobStatus{Succeeded: 1}, wantPhase: "Succeeded", wantRequeue: false},
		{name: "failed", jobStatus: batchv1.JobStatus{Failed: 1}, wantPhase: "Failed", wantRequeue: false},
		{name: "active", jobStatus: batchv1.JobStatus{Active: 1}, wantPhase: "Running", wantRequeue: true},
		{name: "not started", jobStatus: batchv1.JobStatus{}, wantPhase: "Pending", wantRequeue: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := newWorkloadStatusScheme(t)
			job := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{Name: "my-bot", Namespace: "trading"},
				Status:     tt.jobStatus,
			}
			c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(job).WithStatusSubresource(job).Build()
			r := &Reconciler{Client: c}

			tradeBot := &freqtradev1alpha1.TradeBot{
				ObjectMeta: metav1.ObjectMeta{Name: "my-bot", Namespace: "trading"},
				Spec:       freqtradev1alpha1.TradeBotSpec{FreqtradeCommand: "backtesting"},
			}

			requeueAfter, err := r.updateWorkloadStatus(context.Background(), tradeBot)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tradeBot.Status.Phase != tt.wantPhase {
				t.Errorf("expected phase %q, got %q", tt.wantPhase, tradeBot.Status.Phase)
			}
			if (requeueAfter > 0) != tt.wantRequeue {
				t.Errorf("expected requeue=%v, got requeueAfter=%v", tt.wantRequeue, requeueAfter)
			}
		})
	}
}
