package tradebot

import (
	"context"
	"testing"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// TestPruneStaleWorkloads covers the P0-4 money-losing bug: switching
// freqtrade_command between "trade" and a one-shot command must delete the
// previous mode's workload, or the old one (a live-trading StatefulSet, in
// the worst case) keeps running under stale config.
func TestPruneStaleWorkloads(t *testing.T) {
	scheme := newWorkloadStatusScheme(t)

	t.Run("switching to trade deletes a stale Job", func(t *testing.T) {
		job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "my-bot", Namespace: "trading"}}
		c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(job).Build()
		r := &Reconciler{Client: c}
		tradeBot := &freqtradev1alpha1.TradeBot{ObjectMeta: metav1.ObjectMeta{Name: "my-bot", Namespace: "trading"}}

		if err := r.pruneStaleWorkloads(context.Background(), tradeBot, "trade"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var got batchv1.Job
		err := c.Get(context.Background(), types.NamespacedName{Name: "my-bot", Namespace: "trading"}, &got)
		if !errors.IsNotFound(err) {
			t.Errorf("expected the stale Job to be deleted, got err=%v", err)
		}
	})

	t.Run("switching to a one-shot command deletes a stale StatefulSet and Service", func(t *testing.T) {
		sts := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "my-bot", Namespace: "trading"}}
		svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "my-bot", Namespace: "trading"}}
		c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sts, svc).Build()
		r := &Reconciler{Client: c}
		tradeBot := &freqtradev1alpha1.TradeBot{ObjectMeta: metav1.ObjectMeta{Name: "my-bot", Namespace: "trading"}}

		if err := r.pruneStaleWorkloads(context.Background(), tradeBot, "backtesting"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		key := types.NamespacedName{Name: "my-bot", Namespace: "trading"}
		var gotSts appsv1.StatefulSet
		if err := c.Get(context.Background(), key, &gotSts); !errors.IsNotFound(err) {
			t.Errorf("expected the stale StatefulSet to be deleted, got err=%v", err)
		}
		var gotSvc corev1.Service
		if err := c.Get(context.Background(), key, &gotSvc); !errors.IsNotFound(err) {
			t.Errorf("expected the stale Service to be deleted, got err=%v", err)
		}
	})

	t.Run("nothing stale is a no-op", func(t *testing.T) {
		c := fake.NewClientBuilder().WithScheme(scheme).Build()
		r := &Reconciler{Client: c}
		tradeBot := &freqtradev1alpha1.TradeBot{ObjectMeta: metav1.ObjectMeta{Name: "my-bot", Namespace: "trading"}}

		if err := r.pruneStaleWorkloads(context.Background(), tradeBot, "trade"); err != nil {
			t.Errorf("unexpected error with nothing to prune (trade): %v", err)
		}
		if err := r.pruneStaleWorkloads(context.Background(), tradeBot, "backtesting"); err != nil {
			t.Errorf("unexpected error with nothing to prune (backtesting): %v", err)
		}
	})
}

// TestWorkloadImmutableCondition covers the condition workloadImmutableCondition
// builds for a Job-mode TradeBot whose spec has (or hasn't) drifted from the
// Job actually running. It's a pure function - no fake client needed - since
// P2-1 folded this into Reconcile's single status PatchStatus call instead
// of giving it an independent one (see resources.go).
func TestWorkloadImmutableCondition(t *testing.T) {
	t.Run("spec changed sets the condition true", func(t *testing.T) {
		cond := workloadImmutableCondition("my-bot", 3, true)
		if cond.Status != metav1.ConditionTrue {
			t.Errorf("expected status True, got %v", cond.Status)
		}
		if cond.Reason != freqtradev1alpha1.ReasonSpecChangeIgnored {
			t.Errorf("expected reason SpecChangeIgnored, got %v", cond.Reason)
		}
		if cond.Message == "" {
			t.Error("expected a non-empty message explaining the remedy")
		}
		if cond.ObservedGeneration != 3 {
			t.Errorf("expected observedGeneration 3, got %d", cond.ObservedGeneration)
		}
	})

	t.Run("spec unchanged reports the condition false", func(t *testing.T) {
		cond := workloadImmutableCondition("my-bot", 5, false)
		if cond.Status != metav1.ConditionFalse {
			t.Errorf("expected status False, got %v", cond.Status)
		}
		if cond.Reason != freqtradev1alpha1.ReasonSpecMatchesWorkload {
			t.Errorf("expected reason SpecMatchesWorkload, got %v", cond.Reason)
		}
		if cond.Message != "" {
			t.Errorf("expected an empty message when nothing is wrong, got %q", cond.Message)
		}
	})
}
