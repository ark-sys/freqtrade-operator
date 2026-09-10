package tradebot

import (
	"context"
	"testing"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	freqtradev1beta1 "github.com/ark-sys/freqtrade-operator/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// newWorkloadStatusScheme builds the scheme shared by this package's
// fake-client-backed tests.
func newWorkloadStatusScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add corev1 to scheme: %v", err)
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add appsv1 to scheme: %v", err)
	}
	if err := networkingv1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add networkingv1 to scheme: %v", err)
	}
	if err := freqtradev1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add v1alpha1 to scheme: %v", err)
	}
	// patchTradeBotStatus (P4-4) always writes TradeBot status through
	// v1beta1, regardless of which version's type the caller itself uses.
	if err := freqtradev1beta1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add v1beta1 to scheme: %v", err)
	}
	return scheme
}

// TestComputeWorkloadStatus covers the P0-3 acceptance criterion: workload
// health is derived from the StatefulSet, so a requeue is scheduled while
// it's not yet ready. Trade-only (P6-4) - there is no Job-mode case
// anymore, since Reconcile itself now rejects anything but a live bot
// before computeWorkloadStatus is ever reached.
func TestComputeWorkloadStatus(t *testing.T) {
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
