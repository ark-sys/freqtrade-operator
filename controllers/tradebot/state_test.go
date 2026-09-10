package tradebot

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	freqtradev1beta1 "github.com/ark-sys/freqtrade-operator/api/v1beta1"
	"github.com/ark-sys/freqtrade-operator/controllers/tradebot/botclient"
)

// This file covers P4-4: spec.state reconciliation. reconcileStateChange
// (the part that actually calls botclient.Start/Stop) is unit-tested here
// directly against an httptest.Server, the same way poll/pollWithClient
// already are (see reconcileState's own doc comment on why) - a real
// bot's base URL is in-cluster DNS, unreachable from a unit test. The
// "never call an unreachable bot" skip path is instead covered against
// the real envtest suite below, since it only depends on the
// BotReachable condition already being observable there (BotPoller
// itself never runs in this suite - see suite_test.go - so BotReachable
// stays unset for every fixture here, which is exactly the unreachable
// case).

func newStateTestTradeBot(name string) *freqtradev1alpha1.TradeBot {
	return &freqtradev1alpha1.TradeBot{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "trading"},
	}
}

// betaMirrorOf returns an empty v1beta1.TradeBot sharing alpha's name and
// namespace. patchTradeBotStatus (P4-4) always operates through v1beta1,
// which a real apiserver's CRD conversion transparently backs with this
// same stored object; the fake client has no such conversion and tracks
// each version as an entirely separate object, so any fake-client test
// exercising a patchTradeBotStatus call site must seed this mirror
// alongside the v1alpha1 fixture itself - otherwise the internal Get 404s,
// the whole patch is silently skipped, and the fixture's status never
// changes.
func betaMirrorOf(alpha *freqtradev1alpha1.TradeBot) *freqtradev1beta1.TradeBot {
	return &freqtradev1beta1.TradeBot{
		ObjectMeta: metav1.ObjectMeta{Name: alpha.Name, Namespace: alpha.Namespace},
	}
}

func TestReconcileStateChange_IssuesStopSuccessfully(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/stop" || r.Method != http.MethodPost {
			t.Errorf("expected POST /api/v1/stop, got %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"status":"stopping trader ..."}`))
	}))
	defer srv.Close()

	scheme := newWorkloadStatusScheme(t)
	tradeBot := newStateTestTradeBot("my-bot")
	tradeBotBeta := betaMirrorOf(tradeBot)
	c := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(tradeBot, tradeBotBeta).
		WithStatusSubresource(tradeBot, tradeBotBeta).
		Build()
	recorder := record.NewFakeRecorder(10)
	r := &Reconciler{Client: c, Recorder: recorder}

	botClient := botclient.New(srv.URL, "", "")
	requeue, err := r.reconcileStateChange(
		context.Background(), tradeBot, botClient, freqtradev1beta1.TradeBotStateStopped,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if requeue <= 0 {
		t.Error("expected a non-zero requeue duration after issuing a state change")
	}

	// patchTradeBotStatus mutates tradeBot.Status in place before persisting
	// it through v1beta1 (see betaMirrorOf's doc comment), so the caller's
	// own pointer already reflects the write - no need to (and, since a
	// fresh v1alpha1 Get wouldn't see a v1beta1-routed write against a fake
	// client anyway, no ability to) re-fetch it here.
	cond := findStatusCondition(tradeBot.Status.Conditions, freqtradev1alpha1.ConditionStateReconciled)
	if cond == nil {
		t.Fatal("expected a StateReconciled condition")
	}
	if cond.Status != metav1.ConditionFalse || cond.Reason != freqtradev1alpha1.ReasonStateChangePending {
		t.Errorf("expected StateReconciled=False/StateChangePending, got %s/%s", cond.Status, cond.Reason)
	}

	select {
	case event := <-recorder.Events:
		if !strings.Contains(event, "Normal") || !strings.Contains(event, "BotStopped") {
			t.Errorf("expected a Normal BotStopped event, got %q", event)
		}
	default:
		t.Error("expected a BotStopped event to be recorded, got none")
	}
}

func TestReconcileStateChange_IssuesStartSuccessfully(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/start" || r.Method != http.MethodPost {
			t.Errorf("expected POST /api/v1/start, got %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"status":"starting trader ..."}`))
	}))
	defer srv.Close()

	scheme := newWorkloadStatusScheme(t)
	tradeBot := newStateTestTradeBot("my-bot")
	tradeBotBeta := betaMirrorOf(tradeBot)
	c := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(tradeBot, tradeBotBeta).
		WithStatusSubresource(tradeBot, tradeBotBeta).
		Build()
	recorder := record.NewFakeRecorder(10)
	r := &Reconciler{Client: c, Recorder: recorder}

	botClient := botclient.New(srv.URL, "", "")
	requeue, err := r.reconcileStateChange(
		context.Background(), tradeBot, botClient, freqtradev1beta1.TradeBotStateRunning,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if requeue <= 0 {
		t.Error("expected a non-zero requeue duration after issuing a state change")
	}

	select {
	case event := <-recorder.Events:
		if !strings.Contains(event, "Normal") || !strings.Contains(event, "BotStarted") {
			t.Errorf("expected a Normal BotStarted event, got %q", event)
		}
	default:
		t.Error("expected a BotStarted event to be recorded, got none")
	}
}

func TestReconcileStateChange_RecordsFailureWhenCallErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	scheme := newWorkloadStatusScheme(t)
	tradeBot := newStateTestTradeBot("my-bot")
	tradeBotBeta := betaMirrorOf(tradeBot)
	c := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(tradeBot, tradeBotBeta).
		WithStatusSubresource(tradeBot, tradeBotBeta).
		Build()
	recorder := record.NewFakeRecorder(10)
	r := &Reconciler{Client: c, Recorder: recorder}

	botClient := botclient.New(srv.URL, "", "")
	requeue, err := r.reconcileStateChange(
		context.Background(), tradeBot, botClient, freqtradev1beta1.TradeBotStateStopped,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if requeue != 0 {
		t.Errorf("expected no requeue override when the call itself fails, got %v", requeue)
	}

	// See the mirror-image assertion in TestReconcileStateChange_IssuesStopSuccessfully:
	// tradeBot.Status is already the post-patch value in memory.
	cond := findStatusCondition(tradeBot.Status.Conditions, freqtradev1alpha1.ConditionStateReconciled)
	if cond == nil {
		t.Fatal("expected a StateReconciled condition")
	}
	if cond.Status != metav1.ConditionFalse || cond.Reason != freqtradev1alpha1.ReasonStateChangeFailed {
		t.Errorf("expected StateReconciled=False/StateChangeFailed, got %s/%s", cond.Status, cond.Reason)
	}

	select {
	case event := <-recorder.Events:
		if !strings.Contains(event, "Warning") || !strings.Contains(event, "BotStateChangeFailed") {
			t.Errorf("expected a Warning BotStateChangeFailed event, got %q", event)
		}
	default:
		t.Error("expected a BotStateChangeFailed event to be recorded, got none")
	}
}

// The remainder of this file covers the "never call an unreachable bot"
// skip path (P4-4) against the real envtest suite - see this file's own
// top comment for why that's the right layer for this specific case.
var _ = Describe("TradeBot state reconciliation (P4-4)", func() {
	It("sets StateReconciled=False/BotUnreachable when the bot has never been reachable", func() {
		ctx := context.Background()
		strategy := newTestStrategy("state-unreachable-strategy")
		Expect(k8sClient.Create(ctx, strategy)).To(Succeed())
		cfg := newTestTradeBotConfig("state-unreachable-config")
		Expect(k8sClient.Create(ctx, cfg)).To(Succeed())
		tradeBot := newTestTradeBot("state-unreachable-bot", strategy.Name, cfg.Name, "trade")
		Expect(k8sClient.Create(ctx, tradeBot)).To(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: tradeBot.Name, Namespace: testNamespace}, tradeBot)).
				To(Succeed())
			cond := findStatusCondition(tradeBot.Status.Conditions, freqtradev1alpha1.ConditionStateReconciled)
			g.Expect(cond).NotTo(BeNil(), "expected a StateReconciled condition")
			g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(cond.Reason).To(Equal(freqtradev1alpha1.ReasonBotUnreachable))
		}, "15s", "250ms").Should(Succeed())
	})

	// The mirror image: spec.state=Stopped doesn't change the outcome -
	// the bot being unreachable means the call is never even attempted,
	// regardless of what's desired.
	It("still reports BotUnreachable, never attempting a call, when spec.state=Stopped on an unreachable bot", func() {
		ctx := context.Background()
		strategy := newTestStrategy("state-unreachable-stopped-strategy")
		Expect(k8sClient.Create(ctx, strategy)).To(Succeed())
		cfg := newTestTradeBotConfig("state-unreachable-stopped-config")
		Expect(k8sClient.Create(ctx, cfg)).To(Succeed())
		tradeBot := newTestTradeBot("state-unreachable-stopped-bot", strategy.Name, cfg.Name, "trade")
		Expect(k8sClient.Create(ctx, tradeBot)).To(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: tradeBot.Name, Namespace: testNamespace}, tradeBot)).
				To(Succeed())
			g.Expect(tradeBot.Finalizers).To(ContainElement(BotFinalizer))
		}).Should(Succeed())

		var beta freqtradev1beta1.TradeBot
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: tradeBot.Name, Namespace: testNamespace}, &beta)).To(Succeed())
		beta.Spec.State = freqtradev1beta1.TradeBotStateStopped
		Expect(k8sClient.Update(ctx, &beta)).To(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: tradeBot.Name, Namespace: testNamespace}, tradeBot)).
				To(Succeed())
			cond := findStatusCondition(tradeBot.Status.Conditions, freqtradev1alpha1.ConditionStateReconciled)
			g.Expect(cond).NotTo(BeNil(), "expected a StateReconciled condition")
			g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(cond.Reason).To(Equal(freqtradev1alpha1.ReasonBotUnreachable))
		}, "15s", "250ms").Should(Succeed())
	})

	// Covers the Definition of Done's own acceptance criterion: "spec.state:
	// Stopped survives a pod restart - the operator re-stops the bot
	// unprompted." A pod restart is simulated the way it actually manifests
	// in status.bot.state: the poller (P4-3, which doesn't run in this
	// suite - see this file's own top comment) would observe the
	// freshly-restarted bot as "running" again, exactly like it was never
	// stopped. What matters here is that reconcileState *detects* that
	// mismatch and re-attempts the call on its own - there is no real bot
	// listening in envtest for the call to actually reach, so it fails
	// (StateChangeFailed, not StateChangePending), but that failure is
	// proof the reconciler tried: staying at BotUnreachable (the previous
	// It's outcome) would mean it never did.
	//
	// Status is set to reachable/running *before* spec.state changes,
	// deliberately: TradeBot's own watch predicate
	// (controllers/tradebot/setup.go) only re-triggers Reconcile on a
	// generation or annotation change, never a status-only one (standard -
	// metadata.generation itself only ever increments on a spec change),
	// so a status-only write here would otherwise sit unobserved until
	// whatever the *previous* pass's own RequeueAfter happens to fire, up
	// to 15s later. Committing status first means the spec.state write's
	// own immediate, watch-triggered reconcile already sees both at once.
	It("re-attempts the call when a reachable bot's observed state no longer matches spec.state", func() {
		ctx := context.Background()
		strategy := newTestStrategy("state-mismatch-strategy")
		Expect(k8sClient.Create(ctx, strategy)).To(Succeed())
		cfg := newTestTradeBotConfig("state-mismatch-config")
		Expect(k8sClient.Create(ctx, cfg)).To(Succeed())
		tradeBot := newTestTradeBot("state-mismatch-bot", strategy.Name, cfg.Name, "trade")
		Expect(k8sClient.Create(ctx, tradeBot)).To(Succeed())
		key := types.NamespacedName{Name: tradeBot.Name, Namespace: testNamespace}

		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, key, tradeBot)).To(Succeed())
			g.Expect(tradeBot.Finalizers).To(ContainElement(BotFinalizer))
		}).Should(Succeed())

		// Simulate the poller having just observed a reachable, running bot
		// (as it would immediately after a restart) - retried as a whole
		// get-mutate-update sequence since the real reconciler is
		// concurrently patching this same object's status.
		Eventually(func(g Gomega) {
			var fresh freqtradev1alpha1.TradeBot
			g.Expect(k8sClient.Get(ctx, key, &fresh)).To(Succeed())
			meta.SetStatusCondition(&fresh.Status.Conditions, metav1.Condition{
				Type: freqtradev1alpha1.ConditionBotReachable, Status: metav1.ConditionTrue,
				Reason: freqtradev1alpha1.ReasonAsExpected,
			})
			fresh.Status.Bot = &freqtradev1alpha1.BotStatus{State: "running"}
			g.Expect(k8sClient.Status().Update(ctx, &fresh)).To(Succeed())
		}).Should(Succeed())

		var beta freqtradev1beta1.TradeBot
		Expect(k8sClient.Get(ctx, key, &beta)).To(Succeed())
		beta.Spec.State = freqtradev1beta1.TradeBotStateStopped
		Expect(k8sClient.Update(ctx, &beta)).To(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, key, tradeBot)).To(Succeed())
			cond := findStatusCondition(tradeBot.Status.Conditions, freqtradev1alpha1.ConditionStateReconciled)
			g.Expect(cond).NotTo(BeNil(), "expected a StateReconciled condition")
			g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(cond.Reason).To(Equal(freqtradev1alpha1.ReasonStateChangeFailed),
				"expected the reconciler to have attempted (and, with no real bot listening, failed) a call, "+
					"not stayed at BotUnreachable")
		}, "15s", "1s").Should(Succeed())
	})
})
