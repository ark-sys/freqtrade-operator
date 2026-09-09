package tradebot

import (
	"context"
	"testing"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// TestReconcile_MissingReference covers the P0-1/P0-2 acceptance criterion:
// a TradeBot whose Strategy or TradeBotConfig reference doesn't resolve must
// be a reported Error, never a panic that takes the manager down with it.
//
// This exercises Reconcile against a fake client rather than through
// envtest/Ginkgo: since P1-4, the admission webhook rejects such a
// reference synchronously at apply time, so this state is no longer
// reachable via k8sClient.Create in the envtest suite. It still matters -
// the Strategy/TradeBotConfig a TradeBot references can be deleted after
// the TradeBot itself is created, since the webhook only validates the
// TradeBot's own create/update, not deletion of what it points to - and the
// fake client, having no admission chain, is the right place to construct
// that "reference existed, then vanished" state directly.
func TestReconcile_MissingReference(t *testing.T) {
	tests := []struct {
		name     string
		strategy string
		config   string
	}{
		{name: "missing Strategy", strategy: "does-not-exist", config: "some-config"},
		{name: "missing TradeBotConfig", strategy: "some-strategy", config: "does-not-exist"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := newWorkloadStatusScheme(t)
			// The finalizer is pre-seeded so Reconcile's first pass goes
			// straight to fetching referenced resources, rather than
			// spending it only on adding the finalizer (see main.go step 3).
			tradeBot := &freqtradev1alpha1.TradeBot{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-bot", Namespace: "trading", Finalizers: []string{BotFinalizer},
				},
				Spec: freqtradev1alpha1.TradeBotSpec{Strategy: tt.strategy, Config: tt.config},
			}
			c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tradeBot).WithStatusSubresource(tradeBot).Build()
			r := &Reconciler{Client: c, Scheme: scheme}

			req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "my-bot", Namespace: "trading"}}
			// Reconcile returns the underlying NotFound error alongside the
			// status update, so controller-runtime's own backoff takes
			// over - that's expected, not a failure of this test.
			_, _ = r.Reconcile(context.Background(), req)

			var got freqtradev1alpha1.TradeBot
			if err := c.Get(context.Background(), req.NamespacedName, &got); err != nil {
				t.Fatalf("failed to get TradeBot: %v", err)
			}
			if got.Status.Phase != "Error" {
				t.Errorf("expected Phase=Error, got %q (conditions: %+v)", got.Status.Phase, got.Status.Conditions)
			}

			err := c.Get(context.Background(), req.NamespacedName, &appsv1.StatefulSet{})
			if !apierrors.IsNotFound(err) {
				t.Errorf("expected no StatefulSet to be created, got err=%v", err)
			}
		})
	}
}
