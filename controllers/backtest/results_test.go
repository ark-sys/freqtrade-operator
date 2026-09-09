package backtest

import (
	"context"
	"testing"

	freqtradev1beta1 "github.com/ark-sys/freqtrade-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newResultsTestScheme(t *testing.T) *runtime.Scheme {
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

func TestAdoptAndParseResults_ConfigMapNotFoundRequestsRequeue(t *testing.T) {
	scheme := newResultsTestScheme(t)
	backtest := &freqtradev1beta1.Backtest{ObjectMeta: metav1.ObjectMeta{Name: "my-run", Namespace: "trading"}}
	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	r := &Reconciler{Client: c, Scheme: scheme}

	results, cond, requeue, err := r.adoptAndParseResults(context.Background(), backtest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results != nil {
		t.Errorf("expected nil results, got %+v", results)
	}
	if cond.Type != "" {
		t.Errorf("expected no condition to be set yet (caller requeues instead), got %+v", cond)
	}
	if !requeue {
		t.Error("expected requeue=true when the results ConfigMap doesn't exist yet")
	}
}

func TestAdoptAndParseResults_HappyPathParsesAndAdopts(t *testing.T) {
	scheme := newResultsTestScheme(t)
	backtest := &freqtradev1beta1.Backtest{
		ObjectMeta: metav1.ObjectMeta{Name: "my-run", Namespace: "trading", UID: "abc-123"},
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "my-run-results", Namespace: "trading"},
		Data:       map[string]string{"results.json": `{"totalTrades": 5, "profitAbs": "10.5"}`},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(backtest, cm).Build()
	r := &Reconciler{Client: c, Scheme: scheme}

	results, cond, requeue, err := r.adoptAndParseResults(context.Background(), backtest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if requeue {
		t.Error("expected requeue=false once results were parsed")
	}
	if results == nil || results.TotalTrades != 5 || results.ProfitAbs != "10.5" {
		t.Errorf("expected parsed results {TotalTrades:5, ProfitAbs:10.5}, got %+v", results)
	}
	if cond.Status != metav1.ConditionTrue || cond.Reason != freqtradev1beta1.ReasonAsExpected {
		t.Errorf("expected ResultsAvailable=True/AsExpected, got %+v", cond)
	}

	var got corev1.ConfigMap
	key := types.NamespacedName{Name: "my-run-results", Namespace: "trading"}
	if err := c.Get(context.Background(), key, &got); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(got.OwnerReferences) != 1 || got.OwnerReferences[0].UID != "abc-123" {
		t.Errorf("expected the ConfigMap to be adopted by the Backtest, got owners %+v", got.OwnerReferences)
	}
}

func TestAdoptAndParseResults_ErrorKeySetsResultsUnavailable(t *testing.T) {
	scheme := newResultsTestScheme(t)
	backtest := &freqtradev1beta1.Backtest{ObjectMeta: metav1.ObjectMeta{Name: "my-run", Namespace: "trading", UID: "abc"}}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "my-run-results", Namespace: "trading"},
		Data:       map[string]string{"error": "result file not found"},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(backtest, cm).Build()
	r := &Reconciler{Client: c, Scheme: scheme}

	results, cond, requeue, err := r.adoptAndParseResults(context.Background(), backtest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results != nil {
		t.Errorf("expected nil results when the sidecar reported an error, got %+v", results)
	}
	if requeue {
		t.Error("expected requeue=false - an error key is a final answer, not a race to retry")
	}
	if cond.Status != metav1.ConditionFalse || cond.Reason != freqtradev1beta1.ReasonResultsUnavailable {
		t.Errorf("expected ResultsAvailable=False/ResultsUnavailable, got %+v", cond)
	}
	if cond.Message != "result file not found" {
		t.Errorf("expected the sidecar's own error message surfaced, got %q", cond.Message)
	}
}

func TestAdoptAndParseResults_MalformedResultsJSONSetsResultsUnavailable(t *testing.T) {
	scheme := newResultsTestScheme(t)
	backtest := &freqtradev1beta1.Backtest{ObjectMeta: metav1.ObjectMeta{Name: "my-run", Namespace: "trading", UID: "abc"}}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "my-run-results", Namespace: "trading"},
		Data:       map[string]string{"results.json": `not valid json`},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(backtest, cm).Build()
	r := &Reconciler{Client: c, Scheme: scheme}

	results, cond, _, err := r.adoptAndParseResults(context.Background(), backtest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results != nil {
		t.Errorf("expected nil results for malformed JSON, got %+v", results)
	}
	if cond.Status != metav1.ConditionFalse || cond.Reason != freqtradev1beta1.ReasonResultsUnavailable {
		t.Errorf("expected ResultsAvailable=False/ResultsUnavailable, got %+v", cond)
	}
}

func TestEnsureOwned_AlreadyOwnedIsANoOp(t *testing.T) {
	scheme := newResultsTestScheme(t)
	backtest := &freqtradev1beta1.Backtest{ObjectMeta: metav1.ObjectMeta{Name: "my-run", Namespace: "trading", UID: "abc"}}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-run-results", Namespace: "trading",
			OwnerReferences: []metav1.OwnerReference{{UID: "abc", Name: "my-run"}},
			// A ResourceVersion an Update against the fake client would
			// reject as stale, so this test fails loudly if ensureOwned
			// tries to write despite already being owned.
			ResourceVersion: "1",
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(backtest, cm).Build()
	r := &Reconciler{Client: c, Scheme: scheme}

	if err := r.ensureOwned(context.Background(), backtest, cm); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
