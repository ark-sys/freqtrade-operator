package backtest

import (
	"context"
	"testing"

	freqtradev1beta1 "github.com/ark-sys/freqtrade-operator/api/v1beta1"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newWorkloadStatusScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := batchv1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add batchv1 to scheme: %v", err)
	}
	if err := freqtradev1beta1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add v1beta1 to scheme: %v", err)
	}
	return scheme
}

func TestComputeWorkloadStatus(t *testing.T) {
	tests := []struct {
		name          string
		jobStatus     batchv1.JobStatus
		wantSucceeded bool
		wantFailed    bool
		wantRunning   bool
		wantRequeue   bool
	}{
		{name: "succeeded", jobStatus: batchv1.JobStatus{Succeeded: 1}, wantSucceeded: true, wantRequeue: false},
		{name: "failed", jobStatus: batchv1.JobStatus{Failed: 1}, wantFailed: true, wantRequeue: false},
		{name: "active", jobStatus: batchv1.JobStatus{Active: 1}, wantRunning: true, wantRequeue: true},
		{name: "not started", jobStatus: batchv1.JobStatus{}, wantRequeue: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := newWorkloadStatusScheme(t)
			job := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{Name: "my-run", Namespace: "trading"},
				Status:     tt.jobStatus,
			}
			c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(job).WithStatusSubresource(job).Build()
			r := &Reconciler{Client: c}

			backtest := &freqtradev1beta1.Backtest{ObjectMeta: metav1.ObjectMeta{Name: "my-run", Namespace: "trading"}}

			got, err := r.computeWorkloadStatus(context.Background(), backtest)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.succeeded != tt.wantSucceeded {
				t.Errorf("succeeded = %v, want %v (full: %+v)", got.succeeded, tt.wantSucceeded, got)
			}
			if got.failed != tt.wantFailed {
				t.Errorf("failed = %v, want %v (full: %+v)", got.failed, tt.wantFailed, got)
			}
			if got.running != tt.wantRunning {
				t.Errorf("running = %v, want %v (full: %+v)", got.running, tt.wantRunning, got)
			}
			if (got.requeueAfter > 0) != tt.wantRequeue {
				t.Errorf("expected requeue=%v, got requeueAfter=%v", tt.wantRequeue, got.requeueAfter)
			}
		})
	}
}

func TestDeriveBacktestPhase(t *testing.T) {
	tests := []struct {
		name     string
		workload workloadStatus
		want     string
	}{
		{name: "succeeded", workload: workloadStatus{succeeded: true}, want: "Succeeded"},
		{name: "failed", workload: workloadStatus{failed: true}, want: "Failed"},
		{name: "running", workload: workloadStatus{running: true}, want: "Running"},
		{name: "not started yet", workload: workloadStatus{}, want: "Pending"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := deriveBacktestPhase(tt.workload); got != tt.want {
				t.Errorf("deriveBacktestPhase() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWorkloadStatus_ConditionStatusAndReason(t *testing.T) {
	tests := []struct {
		name       string
		workload   workloadStatus
		wantStatus metav1.ConditionStatus
		wantReason string
	}{
		{
			name: "succeeded is Ready=True", workload: workloadStatus{succeeded: true},
			wantStatus: metav1.ConditionTrue, wantReason: freqtradev1beta1.ReasonWorkloadSucceeded,
		},
		{
			name: "failed is Ready=False", workload: workloadStatus{failed: true},
			wantStatus: metav1.ConditionFalse, wantReason: freqtradev1beta1.ReasonWorkloadFailed,
		},
		{
			name: "still progressing is Ready=False", workload: workloadStatus{running: true},
			wantStatus: metav1.ConditionFalse, wantReason: freqtradev1beta1.ReasonWorkloadProgressing,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, reason := tt.workload.conditionStatusAndReason()
			if status != tt.wantStatus || reason != tt.wantReason {
				t.Errorf("conditionStatusAndReason() = (%v, %v), want (%v, %v)",
					status, reason, tt.wantStatus, tt.wantReason)
			}
		})
	}
}
