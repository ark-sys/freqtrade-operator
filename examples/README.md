# Examples

Two independent, self-contained example sets. Each has its own `kustomization.yaml` and its own
namespace, so you can apply either without the other.

```bash
kubectl apply -k examples/live-trading/
# or
kubectl apply -k examples/backtest/
```

Both assume the operator itself is already installed and running - see
[docs/installation.md](../docs/installation.md) if it isn't yet.

## [`live-trading/`](live-trading/)

A dry-run `TradeBot` against Binance, with a `Strategy`, `TradeBotConfig`, and a commented-out
`FreqUI` (needs a real Ingress controller/DNS to actually be reachable - see the comment in its
`kustomization.yaml`) - all four `v1beta1`. Every credential is a placeholder `Secret` - fill in
real values, or leave them as-is and the bot will fail to authenticate rather than trade on
garbage credentials.

Before applying for real:
- Edit `binance-credentials.yaml`, `api-server-credentials.yaml`, and `telegram-credentials.yaml`
  with real values (or drop the `telegram` block from `config.yaml` entirely if you don't want
  notifications).
- `config.yaml` has `dry_run: true` - this is a paper-trading example, not a live-money one.
  Read the [Production checklist](../docs/operations.md#production-checklist) before ever
  flipping that off.
- `tradebot.yaml`'s `app.pvc.storageClassName` is left unset (uses your cluster's default) -
  override it if you need a specific one.

## [`backtest/`](backtest/)

A self-downloading `Backtest`, with its `Strategy` and `TradeBotConfig` (all `v1beta1`): apply it as-is and it fetches its own historical data
(`cache-pvc.yaml` + `spec.data.pvcName`) before running, no live bot involved. `stakeAmount:
"unlimited"` and `dry_run: true` in `config.yaml` mean nothing here places a real order -
backtesting never does, regardless of either setting.

Before applying:
- `backtest.yaml`'s `spec.timerange` is a placeholder - change it to a real, recent range
  (freqtrade needs actual historical candles to exist for whatever you ask for).
- `exchange-credentials.yaml` can often stay blank - most exchanges, Binance included, serve
  historical market data over public (unauthenticated) endpoints - but real credentials get you
  a much higher rate limit if `download-data` is slow or fails partway through.

Once it reaches `status.phase: Succeeded`:

```bash
kubectl get backtest -n freqtrade-backtest-example
kubectl get backtest btc-backtest-jan-2024 -n freqtrade-backtest-example -o jsonpath='{.status.results}' | jq
```

See [Backtest runs](../docs/backtesting.md#backtest-runs-v1beta1) for
`spec.data.downloadPolicy`'s other options, running a parameter sweep across many `Backtest`s at
once, and what `status.results` actually contains.
