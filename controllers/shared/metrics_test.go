package shared

import (
	"strings"
	"testing"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	"github.com/prometheus/client_golang/prometheus/testutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newMetricsTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := freqtradev1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add v1alpha1 to scheme: %v", err)
	}
	return scheme
}

func tradeBotWithPhaseAndDrift(name, phase string, drift bool) *freqtradev1alpha1.TradeBot {
	tb := &freqtradev1alpha1.TradeBot{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "trading"},
		Status:     freqtradev1alpha1.TradeBotStatus{Phase: phase},
	}
	if drift {
		tb.Status.Conditions = append(tb.Status.Conditions, metav1.Condition{
			Type: freqtradev1alpha1.ConditionConfigDrift, Status: metav1.ConditionTrue,
			Reason: freqtradev1alpha1.ReasonPendingRestart, LastTransitionTime: metav1.Now(),
		})
	}
	return tb
}

// TestTradeBotCollector_ByPhaseAndDrift covers the P4-2 point of computing
// both metrics from a fresh List every scrape, not incrementally off
// reconcile events: a deleted TradeBot's own reconciler never runs again to
// clean up a stale series, so nothing here is allowed to remember state
// between Collect calls.
func TestTradeBotCollector_ByPhaseAndDrift(t *testing.T) {
	scheme := newMetricsTestScheme(t)
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		tradeBotWithPhaseAndDrift("running-no-drift", "Running", false),
		tradeBotWithPhaseAndDrift("running-with-drift", "Running", true),
		tradeBotWithPhaseAndDrift("pending-bot", "Pending", false),
	).Build()

	collector := NewTradeBotCollector(c)

	want := `
		# HELP freqtrade_operator_config_drift 1 if the rendered config hasn't rolled out to the workload yet, else 0.
		# TYPE freqtrade_operator_config_drift gauge
		freqtrade_operator_config_drift{namespace="trading",tradebot="pending-bot"} 0
		freqtrade_operator_config_drift{namespace="trading",tradebot="running-no-drift"} 0
		freqtrade_operator_config_drift{namespace="trading",tradebot="running-with-drift"} 1
		# HELP freqtrade_operator_tradebots Number of TradeBots currently in each phase.
		# TYPE freqtrade_operator_tradebots gauge
		freqtrade_operator_tradebots{phase="Pending"} 1
		freqtrade_operator_tradebots{phase="Running"} 2
	`
	if err := testutil.CollectAndCompare(collector, strings.NewReader(want),
		"freqtrade_operator_tradebots", "freqtrade_operator_config_drift"); err != nil {
		t.Error(err)
	}
}

func TestTradeBotCollector_NoTradeBotsEmitsNoSamples(t *testing.T) {
	scheme := newMetricsTestScheme(t)
	c := fake.NewClientBuilder().WithScheme(scheme).Build()

	collector := NewTradeBotCollector(c)

	if err := testutil.CollectAndCompare(collector, strings.NewReader(""),
		"freqtrade_operator_tradebots", "freqtrade_operator_config_drift"); err != nil {
		t.Error(err)
	}
}
