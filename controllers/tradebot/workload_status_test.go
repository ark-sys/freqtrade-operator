package tradebot

import (
	"context"
	"testing"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// newWorkloadStatusScheme builds the scheme shared by this package's
// fake-client-backed tests (workload status, stale-workload pruning).
func newWorkloadStatusScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add corev1 to scheme: %v", err)
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add appsv1 to scheme: %v", err)
	}
	if err := batchv1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add batchv1 to scheme: %v", err)
	}
	if err := networkingv1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add networkingv1 to scheme: %v", err)
	}
	if err := freqtradev1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add v1alpha1 to scheme: %v", err)
	}
	return scheme
}

// TestComputeWorkloadStatus_TradeMode covers the P0-3 acceptance criterion:
// workload health is derived from the StatefulSet, so a requeue is
// scheduled while it's not yet ready.
func TestComputeWorkloadStatus_TradeMode(t *testing.T) {
	tests := []struct {
		name          string
		readyReplicas int32
		wantReady     bool
		wantRequeue   bool
	}{
		{name: "not ready yet", readyReplicas: 0, wantReady: false, wantRequeue: true},
		{name: "ready", readyReplicas: 1, wantReady: true, wantRequeue: false},
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
			}

			got, err := r.computeWorkloadStatus(context.Background(), tradeBot)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.ready != tt.wantReady {
				t.Errorf("expected ready=%v, got %+v", tt.wantReady, got)
			}
			if (got.requeueAfter > 0) != tt.wantRequeue {
				t.Errorf("expected requeue=%v, got requeueAfter=%v", tt.wantRequeue, got.requeueAfter)
			}
		})
	}
}

func TestComputeWorkloadStatus_JobMode(t *testing.T) {
	tests := []struct {
		name          string
		jobStatus     batchv1.JobStatus
		wantReady     bool
		wantSucceeded bool
		wantFailed    bool
		wantRequeue   bool
	}{
		{name: "succeeded", jobStatus: batchv1.JobStatus{Succeeded: 1}, wantSucceeded: true, wantRequeue: false},
		{name: "failed", jobStatus: batchv1.JobStatus{Failed: 1}, wantFailed: true, wantRequeue: false},
		{name: "active", jobStatus: batchv1.JobStatus{Active: 1}, wantRequeue: true},
		{name: "not started", jobStatus: batchv1.JobStatus{}, wantRequeue: true},
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

			got, err := r.computeWorkloadStatus(context.Background(), tradeBot)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.ready != tt.wantReady || got.succeeded != tt.wantSucceeded || got.failed != tt.wantFailed {
				t.Errorf("expected ready=%v succeeded=%v failed=%v, got %+v",
					tt.wantReady, tt.wantSucceeded, tt.wantFailed, got)
			}
			if (got.requeueAfter > 0) != tt.wantRequeue {
				t.Errorf("expected requeue=%v, got requeueAfter=%v", tt.wantRequeue, got.requeueAfter)
			}
		})
	}
}

// cond builds a metav1.Condition with just the fields deriveTradeBotPhase
// looks at, to keep TestDeriveTradeBotPhase's table readable.
func cond(condType string, status metav1.ConditionStatus, reason string) metav1.Condition {
	return metav1.Condition{Type: condType, Status: status, Reason: reason}
}

// TestDeriveTradeBotPhase covers the human-facing Phase computed from
// Conditions - Phase is never itself the source of truth (P1-3).
func TestDeriveTradeBotPhase(t *testing.T) {
	const (
		configResolved = freqtradev1alpha1.ConditionConfigResolved
		workloadReady  = freqtradev1alpha1.ConditionWorkloadReady
		asExpected     = freqtradev1alpha1.ReasonAsExpected
	)

	tests := []struct {
		name       string
		conditions []metav1.Condition
		want       string
	}{
		{name: "no conditions yet", conditions: nil, want: ""},
		{
			name: "config not resolved, reference not found",
			conditions: []metav1.Condition{
				cond(configResolved, metav1.ConditionFalse, freqtradev1alpha1.ReasonReferenceNotFound),
			},
			want: "Error",
		},
		{
			name:       "config not resolved, other reason",
			conditions: []metav1.Condition{cond(configResolved, metav1.ConditionFalse, freqtradev1alpha1.ReasonConfigInvalid)},
			want:       "ConfigError",
		},
		{
			name: "workload healthy",
			conditions: []metav1.Condition{
				cond(configResolved, metav1.ConditionTrue, asExpected),
				cond(workloadReady, metav1.ConditionTrue, freqtradev1alpha1.ReasonWorkloadHealthy),
			},
			want: "Running",
		},
		{
			name: "workload succeeded",
			conditions: []metav1.Condition{
				cond(configResolved, metav1.ConditionTrue, asExpected),
				cond(workloadReady, metav1.ConditionTrue, freqtradev1alpha1.ReasonWorkloadSucceeded),
			},
			want: "Succeeded",
		},
		{
			name: "workload failed",
			conditions: []metav1.Condition{
				cond(configResolved, metav1.ConditionTrue, asExpected),
				cond(workloadReady, metav1.ConditionFalse, freqtradev1alpha1.ReasonWorkloadFailed),
			},
			want: "Failed",
		},
		{
			name: "workload progressing",
			conditions: []metav1.Condition{
				cond(configResolved, metav1.ConditionTrue, asExpected),
				cond(workloadReady, metav1.ConditionFalse, freqtradev1alpha1.ReasonWorkloadProgressing),
			},
			want: "Pending",
		},
		{
			name: "resource error",
			conditions: []metav1.Condition{
				cond(configResolved, metav1.ConditionTrue, asExpected),
				cond(workloadReady, metav1.ConditionFalse, freqtradev1alpha1.ReasonReconcileError),
			},
			want: "ResourceError",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := deriveTradeBotPhase(tt.conditions); got != tt.want {
				t.Errorf("deriveTradeBotPhase() = %q, want %q", got, tt.want)
			}
		})
	}
}
