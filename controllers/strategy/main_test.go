package strategy

import (
	"context"
	"strings"
	"testing"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// validTestStrategyScript satisfies the P1-4 admission webhook's shape
// check (class definition, the three populate_* methods, a freqtrade
// import) - mirrors controllers/tradebot's own fixture of the same name.
const validTestStrategyScript = `import freqtrade
from freqtrade.strategy import IStrategy

class SampleStrategy(IStrategy):
    def populate_indicators(self, dataframe, metadata):
        return dataframe

    def populate_entry_trend(self, dataframe, metadata):
        return dataframe

    def populate_exit_trend(self, dataframe, metadata):
        return dataframe
`

func newStrategyTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := freqtradev1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add v1alpha1 to scheme: %v", err)
	}
	return scheme
}

// TestReconcile_ValidationFailedEvent covers P4-1: a Warning Event must
// accompany the Ready=False condition an invalid Strategy already got
// (P1-3), not just the condition on its own - kubectl describe is where
// most users actually look first.
func TestReconcile_ValidationFailedEvent(t *testing.T) {
	scheme := newStrategyTestScheme(t)
	badStrategy := &freqtradev1alpha1.Strategy{
		ObjectMeta: metav1.ObjectMeta{Name: "bad-strategy", Namespace: "default"},
		Spec:       freqtradev1alpha1.StrategySpec{Name: "", Script: ""},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(&freqtradev1alpha1.Strategy{}).
		WithObjects(badStrategy).
		Build()
	recorder := record.NewFakeRecorder(10)
	r := &Reconciler{Client: c, Recorder: recorder}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(badStrategy),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case event := <-recorder.Events:
		if !strings.Contains(event, "Warning") || !strings.Contains(event, "ValidationFailed") {
			t.Errorf("expected a Warning ValidationFailed event, got %q", event)
		}
	default:
		t.Error("expected a ValidationFailed event to be recorded, got none")
	}
}

func TestReconcile_NoEventOnValidStrategy(t *testing.T) {
	scheme := newStrategyTestScheme(t)
	goodStrategy := &freqtradev1alpha1.Strategy{
		ObjectMeta: metav1.ObjectMeta{Name: "good-strategy", Namespace: "default"},
		Spec: freqtradev1alpha1.StrategySpec{
			Name:   "SampleStrategy",
			Script: validTestStrategyScript,
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(&freqtradev1alpha1.Strategy{}).
		WithObjects(goodStrategy).
		Build()
	recorder := record.NewFakeRecorder(10)
	r := &Reconciler{Client: c, Recorder: recorder}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(goodStrategy),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case event := <-recorder.Events:
		t.Errorf("expected no event for a valid Strategy, got %q", event)
	default:
	}
}
