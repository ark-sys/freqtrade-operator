package tradebot

import (
	"context"
	"testing"
	"time"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// TestFinalizeTradeBot_NeverBlocks covers the P0-5 fix: finalizeTradeBot must
// return quickly and report "not done yet" via its bool result instead of
// blocking this reconcile worker until the StatefulSet actually scales down
// (MaxConcurrentReconciles is 1, so blocking here stalls every other
// TradeBot too).
func TestFinalizeTradeBot_NeverBlocks(t *testing.T) {
	scheme := newWorkloadStatusScheme(t)

	t.Run("no StatefulSet completes immediately", func(t *testing.T) {
		c := fake.NewClientBuilder().WithScheme(scheme).Build()
		r := &Reconciler{Client: c}
		tradeBot := &freqtradev1alpha1.TradeBot{
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-bot", Namespace: "trading",
				DeletionTimestamp: &metav1.Time{Time: time.Now()},
				Finalizers:        []string{BotFinalizer},
			},
		}

		done, err := r.finalizeTradeBot(context.Background(), tradeBot)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !done {
			t.Error("expected finalization to complete when there's no StatefulSet to scale down")
		}
	})

	t.Run("StatefulSet not yet requested to scale down triggers one update and reports not done", func(t *testing.T) {
		replicas := int32(1)
		sts := &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{Name: "my-bot", Namespace: "trading"},
			Spec:       appsv1.StatefulSetSpec{Replicas: &replicas},
			Status:     appsv1.StatefulSetStatus{Replicas: 1, ReadyReplicas: 1},
		}
		c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sts).WithStatusSubresource(sts).Build()
		r := &Reconciler{Client: c}
		tradeBot := &freqtradev1alpha1.TradeBot{
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-bot", Namespace: "trading",
				DeletionTimestamp: &metav1.Time{Time: time.Now()},
			},
		}

		done, err := r.finalizeTradeBot(context.Background(), tradeBot)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if done {
			t.Error("expected finalization to report not-done immediately after requesting scale-down")
		}

		var got appsv1.StatefulSet
		if err := c.Get(context.Background(), types.NamespacedName{Name: "my-bot", Namespace: "trading"}, &got); err != nil {
			t.Fatalf("failed to get StatefulSet: %v", err)
		}
		if got.Spec.Replicas == nil || *got.Spec.Replicas != 0 {
			t.Errorf("expected Spec.Replicas to be requested as 0, got %v", got.Spec.Replicas)
		}
	})

	t.Run("scale-down requested but status not yet caught up reports not done", func(t *testing.T) {
		replicas := int32(0)
		sts := &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{Name: "my-bot", Namespace: "trading"},
			Spec:       appsv1.StatefulSetSpec{Replicas: &replicas},
			Status:     appsv1.StatefulSetStatus{Replicas: 1, ReadyReplicas: 1}, // pod hasn't terminated yet
		}
		c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sts).WithStatusSubresource(sts).Build()
		r := &Reconciler{Client: c}
		tradeBot := &freqtradev1alpha1.TradeBot{
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-bot", Namespace: "trading",
				DeletionTimestamp: &metav1.Time{Time: time.Now()}, // well within the grace period
			},
		}

		done, err := r.finalizeTradeBot(context.Background(), tradeBot)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if done {
			t.Error("expected finalization to wait while the StatefulSet is still scaling down")
		}
	})

	t.Run("grace period exceeded proceeds anyway", func(t *testing.T) {
		replicas := int32(0)
		sts := &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{Name: "my-bot", Namespace: "trading"},
			Spec:       appsv1.StatefulSetSpec{Replicas: &replicas},
			Status:     appsv1.StatefulSetStatus{Replicas: 1, ReadyReplicas: 1}, // still stuck
		}
		c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sts).WithStatusSubresource(sts).Build()
		r := &Reconciler{Client: c, FinalizerGracePeriod: time.Minute}
		tradeBot := &freqtradev1alpha1.TradeBot{
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-bot", Namespace: "trading",
				DeletionTimestamp: &metav1.Time{Time: time.Now().Add(-2 * time.Minute)}, // past the 1-minute grace period
			},
		}

		done, err := r.finalizeTradeBot(context.Background(), tradeBot)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !done {
			t.Error("expected finalization to proceed once the grace period is exceeded, even with a stuck StatefulSet")
		}
	})

	t.Run("StatefulSet already fully scaled down completes", func(t *testing.T) {
		replicas := int32(0)
		sts := &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{Name: "my-bot", Namespace: "trading"},
			Spec:       appsv1.StatefulSetSpec{Replicas: &replicas},
			Status:     appsv1.StatefulSetStatus{Replicas: 0, ReadyReplicas: 0},
		}
		c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sts).WithStatusSubresource(sts).Build()
		r := &Reconciler{Client: c}
		tradeBot := &freqtradev1alpha1.TradeBot{
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-bot", Namespace: "trading",
				DeletionTimestamp: &metav1.Time{Time: time.Now()},
			},
		}

		done, err := r.finalizeTradeBot(context.Background(), tradeBot)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !done {
			t.Error("expected finalization to complete once the StatefulSet is fully scaled down")
		}
	})
}

func TestFinalizerDeadlinePassed(t *testing.T) {
	r := &Reconciler{FinalizerGracePeriod: time.Minute}

	noDeletionTimestamp := &freqtradev1alpha1.TradeBot{}
	if r.finalizerDeadlinePassed(noDeletionTimestamp) {
		t.Error("expected false when DeletionTimestamp is nil")
	}

	withinGracePeriod := &freqtradev1alpha1.TradeBot{
		ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: &metav1.Time{Time: time.Now()}},
	}
	if r.finalizerDeadlinePassed(withinGracePeriod) {
		t.Error("expected false within the grace period")
	}

	pastGracePeriod := &freqtradev1alpha1.TradeBot{
		ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: &metav1.Time{Time: time.Now().Add(-2 * time.Minute)}},
	}
	if !r.finalizerDeadlinePassed(pastGracePeriod) {
		t.Error("expected true once the grace period has elapsed")
	}
}
