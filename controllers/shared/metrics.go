package shared

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
)

// ReconcileErrorsTotal counts every reconcile failure across every
// controller (P4-2), labeled by which one and why - each controller's own
// central failure path (failReconcile, or the equivalent inline branch in
// strategy/tradebotconfig) increments this the same place it already emits
// a P4-1 Warning Event, using the same reason string both times.
var ReconcileErrorsTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "freqtrade_operator_reconcile_errors_total",
		Help: "Total number of reconcile errors, by controller and reason.",
	},
	[]string{"controller", "reason"},
)

// ConfigRenderDuration times configbuilder.BuildConfig, the one call in the
// TradeBot reconcile loop that does real work (fetching secrets, merging
// every config section) rather than a handful of cheap object reads.
var ConfigRenderDuration = prometheus.NewHistogram(
	prometheus.HistogramOpts{
		Name:    "freqtrade_operator_config_render_duration_seconds",
		Help:    "How long BuildConfig took to render a TradeBot's config.json.",
		Buckets: prometheus.DefBuckets,
	},
)

func init() {
	metrics.Registry.MustRegister(ReconcileErrorsTotal, ConfigRenderDuration)
}

// tradeBotCollector computes freqtrade_operator_tradebots and
// freqtrade_operator_config_drift fresh from a List at every scrape,
// instead of maintaining them incrementally off reconcile events. The
// incremental approach has two correctness problems a scrape-time
// computation sidesteps entirely: a deleted TradeBot's own reconciler never
// runs again to decrement/remove its series (the exact unbounded-cardinality
// growth P4-3 later warns about for its own per-bot gauges), and a gauge
// nobody re-derives from current state can drift from reality after a
// restart with no corrective moment. One List call serves both metrics.
type tradeBotCollector struct {
	reader          client.Reader
	tradeBotsDesc   *prometheus.Desc
	configDriftDesc *prometheus.Desc
}

// NewTradeBotCollector returns a prometheus.Collector for
// freqtrade_operator_tradebots{phase} and
// freqtrade_operator_config_drift{namespace,tradebot} (P2-4). Register it
// once, in cmd/main.go, with metrics.Registry.MustRegister - reader is
// normally mgr.GetClient(), the same cached client every controller uses.
func NewTradeBotCollector(reader client.Reader) prometheus.Collector {
	return &tradeBotCollector{
		reader: reader,
		tradeBotsDesc: prometheus.NewDesc(
			"freqtrade_operator_tradebots",
			"Number of TradeBots currently in each phase.",
			[]string{"phase"}, nil,
		),
		configDriftDesc: prometheus.NewDesc(
			"freqtrade_operator_config_drift",
			"1 if the rendered config hasn't rolled out to the workload yet, else 0.",
			[]string{"namespace", "tradebot"}, nil,
		),
	}
}

func (c *tradeBotCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.tradeBotsDesc
	ch <- c.configDriftDesc
}

// Collect is best-effort: a List failure just yields no samples for this
// scrape (Prometheus already treats a gap as "unknown," not "zero") rather
// than taking the whole /metrics endpoint down over one transient read error.
func (c *tradeBotCollector) Collect(ch chan<- prometheus.Metric) {
	var list freqtradev1alpha1.TradeBotList
	if err := c.reader.List(context.Background(), &list); err != nil {
		return
	}

	byPhase := make(map[string]int, 4)
	for i := range list.Items {
		tradeBot := &list.Items[i]
		byPhase[tradeBot.Status.Phase]++

		drift := 0.0
		if meta.IsStatusConditionTrue(tradeBot.Status.Conditions, freqtradev1alpha1.ConditionConfigDrift) {
			drift = 1
		}
		ch <- prometheus.MustNewConstMetric(
			c.configDriftDesc, prometheus.GaugeValue, drift, tradeBot.Namespace, tradeBot.Name,
		)
	}
	for phase, count := range byPhase {
		ch <- prometheus.MustNewConstMetric(c.tradeBotsDesc, prometheus.GaugeValue, float64(count), phase)
	}
}
