# Metrics

For installation, see [installation.md](installation.md). For every field on every CRD, see
[api-reference.md](api-reference.md).

`controllerManager.metrics.enabled` (default `true` in `helm/values.yaml`) makes the manager serve Prometheus metrics over
HTTPS on port `8443` - both the flag that turns the metrics server on and the container port it's exposed on now come from
this one value, so they can no longer drift apart the way they previously could.

Beyond controller-runtime's own built-ins (`controller_runtime_reconcile_total`, work queue depth, etc.), this operator
registers:

| Metric | Type | Labels | What it means |
|---|---|---|---|
| `freqtrade_operator_tradebots` | Gauge | `phase` | How many TradeBots are currently in each `status.phase`. |
| `freqtrade_operator_config_drift` | Gauge | `namespace`, `tradebot` | `1` if a `Manual`-mode bot's rendered config hasn't been rolled out to its workload yet. |
| `freqtrade_operator_reconcile_errors_total` | Counter | `controller`, `reason` | Every reconcile failure, by controller and the same `Reason` string its status condition and Event both use. |
| `freqtrade_operator_config_render_duration_seconds` | Histogram | - | How long rendering a TradeBot's `config.json` took. |

The `tradebots`/`config_drift` gauges are recomputed from a fresh List on every scrape rather than tracked incrementally
off reconcile events, specifically so a deleted TradeBot's series disappears on the next scrape instead of being stuck
at a stale value forever (nothing reconciles a deleted object to clean it up otherwise).

Set `controllerManager.metrics.serviceMonitor.enabled: true` to also install a `ServiceMonitor` (requires the Prometheus
Operator's CRDs already in the cluster - off by default for the same reason the kustomize `config/prometheus` overlay
isn't included by default either). A starting Grafana dashboard covering every metric on this page (operator-level above,
per-bot below) is at [config/prometheus/grafana-dashboard.json](../config/prometheus/grafana-dashboard.json) - import it
directly, or adapt it.

## Bot introspection

The operator polls each trade-mode bot's own freqtrade REST API on a schedule and surfaces what it learns in
`status.bot` and as Prometheus metrics - read-only, in the sense that nothing here **acts** on a bot: starting or
stopping one is a separate, deliberate capability, [Bot state control](usage.md#bot-state-control).

```yaml
spec:
  introspection:
    enabled: true    # default; set false for a bot that deliberately has no api_server (e.g. it never runs one)
    interval: 60s     # default; the admission webhook rejects anything under 10s
```

Polling needs `spec.apiServer` configured on the bot's `TradeBotConfig` with Basic Auth credentials the operator can
read back (`secretRef` or, deprecated, the plaintext `username`/`password` fields) - the same credentials
[Credentials](usage.md#credentials) already covers. A bot
with no `apiServer` section at all just fails every poll attempt and reports `BotReachable=False` - set
`spec.introspection.enabled: false` on it to silence that instead.

`status.bot` (`state`, `version`, `dryRun`, `openTrades`, `maxOpenTrades`, `totalProfitAbs`, `totalProfitPct`,
`lastPollTime`, `lastPollError`) reflects the most recent **successful** poll - a bot that goes temporarily
unreachable keeps its last known-good numbers rather than losing them, with `lastPollError` and the `BotReachable`
condition (`False`, reason `ConnectionRefused`/`AuthFailed`/`Timeout`) saying they're now stale. A struggling bot backs
off exponentially, capped at 10x its own `interval`, so a genuinely down bot doesn't get hammered with retries.

Per-bot metrics, labelled `{namespace, tradebot, strategy, exchange, dry_run}`:

| Metric | Type | What it means |
|---|---|---|
| `freqtrade_bot_up` | Gauge | `1` if the most recent poll succeeded, else `0` - what you'd alert on first. |
| `freqtrade_bot_state` | Gauge | `1` if the bot reports itself `running`, else `0`. |
| `freqtrade_bot_open_trades` / `freqtrade_bot_max_open_trades` | Gauge | Current vs. configured max open trades. |
| `freqtrade_bot_profit_abs` / `freqtrade_bot_profit_ratio` | Gauge | All-time profit, in stake currency and as a ratio (`0.05` = 5%). |
| `freqtrade_bot_balance` | Gauge | Total portfolio value in stake currency. |
| `freqtrade_bot_last_poll_timestamp_seconds` | Gauge | Unix timestamp of the most recent poll attempt, successful or not. |

Unlike the operator-level gauges above, these are pushed by the poller on its own schedule rather than recomputed at
scrape time - nothing else in the operator ever learns a bot's balance or trade count except by asking the bot itself.
The poller deletes a bot's series itself the moment it notices the TradeBot is gone, so cardinality doesn't grow
unbounded across a namespace's bot lifecycle.

Only the leader replica polls; polling never runs inside the reconcile loop, so a slow or hung bot can't stall
reconciliation of any TradeBot, including itself.
