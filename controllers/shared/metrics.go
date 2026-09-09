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

// botMetricLabels is the label set every freqtrade_bot_* gauge below
// shares (P4-3) - kept as one slice so BotMetricLabelValues and
// DeleteBotMetrics can't drift out of sync with the Vecs' own declarations.
var botMetricLabels = []string{"namespace", "tradebot", "strategy", "exchange", "dry_run"}

// BotUp, BotState, BotOpenTrades, BotMaxOpenTrades, BotProfitAbs,
// BotProfitRatio, BotBalance, and BotLastPollTimestampSeconds are pushed by
// BotPoller after every poll attempt (P4-3) - unlike ReconcileErrorsTotal
// and ConfigRenderDuration above, or P4-2's own tradeBotCollector, these
// can't be recomputed at scrape time from a List: what they report (bot
// state, balances) only exists in the bot's own REST API response, which
// only the poller ever fetches. That means, unlike a live collector,
// nothing here self-heals when a TradeBot is deleted - BotPoller's own
// scheduler loop must call DeleteBotMetrics for a bot it notices has
// disappeared, or its last-known series would linger in /metrics forever
// (the exact unbounded-cardinality growth the plan's own P4-3 text warns
// about by name).
var (
	BotUp = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "freqtrade_bot_up",
		Help: "1 if the operator's most recent poll of this bot's REST API succeeded, else 0.",
	}, botMetricLabels)

	BotState = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "freqtrade_bot_state",
		Help: "1 if the bot's own reported state is \"running\", 0 for \"stopped\" or unknown.",
	}, botMetricLabels)

	BotOpenTrades = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "freqtrade_bot_open_trades",
		Help: "Number of currently open trades, as last reported by the bot.",
	}, botMetricLabels)

	BotMaxOpenTrades = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "freqtrade_bot_max_open_trades",
		Help: "The bot's own configured max_open_trades, as last reported.",
	}, botMetricLabels)

	BotProfitAbs = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "freqtrade_bot_profit_abs",
		Help: "All-time profit in stake currency, as last reported by the bot.",
	}, botMetricLabels)

	BotProfitRatio = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "freqtrade_bot_profit_ratio",
		Help: "All-time profit as a ratio (0.05 = 5%), as last reported by the bot.",
	}, botMetricLabels)

	BotBalance = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "freqtrade_bot_balance",
		Help: "Total portfolio value in stake currency, as last reported by the bot.",
	}, botMetricLabels)

	BotLastPollTimestampSeconds = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "freqtrade_bot_last_poll_timestamp_seconds",
		Help: "Unix timestamp of the operator's most recent poll attempt for this bot, successful or not.",
	}, botMetricLabels)
)

// DeleteBotMetrics removes every freqtrade_bot_* series for one bot -
// call this, not just letting a Set call lapse, whenever BotPoller notices
// a TradeBot it was tracking is gone.
func DeleteBotMetrics(namespace, tradeBot, strategy, exchange, dryRun string) {
	labels := prometheus.Labels{
		"namespace": namespace, "tradebot": tradeBot,
		"strategy": strategy, "exchange": exchange, "dry_run": dryRun,
	}
	BotUp.Delete(labels)
	BotState.Delete(labels)
	BotOpenTrades.Delete(labels)
	BotMaxOpenTrades.Delete(labels)
	BotProfitAbs.Delete(labels)
	BotProfitRatio.Delete(labels)
	BotBalance.Delete(labels)
	BotLastPollTimestampSeconds.Delete(labels)
}

func init() {
	metrics.Registry.MustRegister(
		ReconcileErrorsTotal, ConfigRenderDuration,
		BotUp, BotState, BotOpenTrades, BotMaxOpenTrades,
		BotProfitAbs, BotProfitRatio, BotBalance, BotLastPollTimestampSeconds,
	)
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
