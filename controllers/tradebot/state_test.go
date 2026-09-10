package tradebot

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

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
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tradeBot).WithStatusSubresource(tradeBot).Build()
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

	var got freqtradev1alpha1.TradeBot
	key := types.NamespacedName{Name: tradeBot.Name, Namespace: tradeBot.Namespace}
	if err := c.Get(context.Background(), key, &got); err != nil {
		t.Fatalf("failed to get TradeBot: %v", err)
	}
	cond := findStatusCondition(got.Status.Conditions, freqtradev1alpha1.ConditionStateReconciled)
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
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tradeBot).WithStatusSubresource(tradeBot).Build()
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
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tradeBot).WithStatusSubresource(tradeBot).Build()
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

	var got freqtradev1alpha1.TradeBot
	key := types.NamespacedName{Name: tradeBot.Name, Namespace: tradeBot.Namespace}
	if err := c.Get(context.Background(), key, &got); err != nil {
		t.Fatalf("failed to get TradeBot: %v", err)
	}
	cond := findStatusCondition(got.Status.Conditions, freqtradev1alpha1.ConditionStateReconciled)
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
})
