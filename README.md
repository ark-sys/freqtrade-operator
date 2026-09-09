# FreqTrade Operator

A Kubernetes operator for managing FreqTrade cryptocurrency trading bots.

## Overview

The FreqTrade Operator provides a Kubernetes-native way to deploy and manage FreqTrade bots. It introduces custom resources for all FreqTrade components and handles the deployment and configuration of the bots.

Key features:
- Deploy FreqTrade bots with custom strategies
- Manage exchange configurations securely using Kubernetes Secrets
- Configure notifications, risk management, and order types
- Deploy a central FreqUI instance for monitoring all bots
- Automatically configure CORS and JWT for secure communication

## Architecture

The operator consists of the following components:

1. **Custom Resource Definitions (CRDs)**:
   - `TradeBot`: Main resource for deploying a FreqTrade bot
   - `TradeBotConfig`: Configuration file for a TradeBot
   - `Strategy`: Python script of the Strategy run by the bot
   - `FreqUI`: Web interface for monitoring and managing the bot

2. **Controllers**:
   - `TradeBotController`: Manages the lifecycle of TradeBot resources
   - `TradeBotConfigController`: Manages the lifecycle of TradeBotConfig resources.
   - `StrategyController`: Manages the lifecycle of Strategy resources.
   - `FreqUIController`: Manages the lifecycle of FreqUI resources and updates CORS settings on referenced TradeBots.

## Prerequisites

- Kubernetes cluster 1.19+
- kubectl 1.19+
- Helm 3+ (optional, for Helm chart installation)
- Go 1.19+ (for building from source)

## Installation

### Using pre-built images

```bash
# Apply CRDs
kubectl apply -f https://github.com/ark-sys/freqtrade-operator/releases/latest/download/crds.yaml

# Deploy the operator
kubectl apply -f https://github.com/ark-sys/freqtrade-operator/releases/latest/download/operator.yaml
```

Every image the release workflow pushes is signed with [cosign](https://docs.sigstore.dev/) (keyless, via GitHub Actions'
own OIDC identity - no key to fetch or trust out of band) and ships an SPDX SBOM as a release asset. Verify an image with:

```bash
cosign verify arksys/freqtrade-operator:<tag> \
  --certificate-identity-regexp 'https://github.com/ark-sys/freqtrade-operator/.github/workflows/release.yml@.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

### Building from source

1. Clone the repository:
```bash
git clone https://github.com/ark-sys/freqtrade-operator.git
cd freqtrade-operator
```

2. Build and deploy the operator:
```bash
# Build the operator image
make docker-build IMG=your-registry/freqtrade-operator:latest

# Push the image to your registry
make docker-push IMG=your-registry/freqtrade-operator:latest

# Deploy the operator to your cluster
make deploy IMG=your-registry/freqtrade-operator:latest
```

### Using Helm

```bash
helm repo add ark-sys https://ark-sys.github.io/freqtrade-operator/
helm repo update
helm install freqtrade-operator ark-sys/freqtrade-operator
```

## Usage

### 1. Create a namespace for your trading bots

```bash
kubectl create namespace freqtrade
```

### 2. Create secrets for exchange API credentials

```bash
kubectl apply -f examples/binance-credentials.yaml
```

Reference the Secret from `TradeBotConfig.spec.exchange.secretRef` (and, the same way, `spec.apiServer.secretRef` /
`spec.notification.telegram.secretRef` for those credentials). The operator reads whichever of these keys the Secret's
`data`/`stringData` provides - set only the ones your exchange needs:

| Secret | Expected keys |
|---|---|
| `spec.exchange.secretRef` | `api-key`, `secret`, `password`, `uid`, `account_id`, `wallet_address`, `private_key` |
| `spec.apiServer.secretRef` | `user`, `password`, `jwt_secret_key` |
| `spec.notification.telegram.secretRef` | `token`, `chat-id` |

`spec.exchange.key`/`secret`/`password`/`uid`/`wallet_address`/`private_key`, `spec.apiServer.password`/`jwtSecretKey`, and
`spec.notification.telegram.token` are plaintext equivalents of the fields above. They're deprecated - stored unencrypted
in etcd and readable by anyone who can `get` the `TradeBotConfig` - and the admission webhook rejects setting any of them
unless the `TradeBotConfig` carries the annotation `freqtrade.io/allow-plaintext-credentials: "true"`. Use `secretRef`
instead; the plaintext fields will be removed in `v1beta1`.

If `spec.apiServer.enabled: true` and neither source above supplies `jwt_secret_key`, the operator generates a random one
itself and keeps reusing that same value on every later reconcile - freqtrade needs *some* signing key to start its REST
API, and forwarding a short, guessable, or absent one is worse than picking a good one for you. A `jwt_secret_key` you *do*
supply is still rejected if it's under 32 characters, from either source.

### 3. Create FreqUI deployment

```bash
kubectl apply -f examples/frequi.yaml
```

### 4. Deploy a TradeBot

```bash
# Apply supporting resources
kubectl apply -f examples/strategy.yaml
kubectl apply -f examples/config.yaml

# Deploy the bot
kubectl apply -f examples/tradebot.yaml
```

### 5. Access FreqUI

Get the FreqUI URL:
```bash
kubectl get frequi -n trading
```

## Examples

The `examples/` directory contains example resources for deploying a complete FreqTrade setup:

- `namespace.yaml`: Namespace for trading resources
- `binance-credentials.yaml`: Secret for exchange API credentials
- `config.yaml`: Configuration file for a TradeBot
- `strategy.yaml`: Python script of the Strategy run by the bot
- `tradebot.yaml`: TradeBot deployment
- `frequi.yaml`: FreqUI deployment


## Different ways to deploy a TradeBot 

The default command executed by an instance of a TradeBot is `trade`. This command materializes the TradeBot resource as a StatefulSet that binds configuration and strategy from referenced resources.
The `trade` command can be overridden by setting the `command` field in the TradeBot resource. This allows for different ways to deploy a TradeBot.

### Using the `trade` command

The `trade` command is used to run a live trading bot. It can be used to trade a strategy in a live environment.
This command materializes the TradeBot resource as a StatefulSet that binds configuration and strategy from referenced resources.
The `trade` command is used when no command is specified.

### Using the `backtesting` command

The `backtesting` command is used to backtest a strategy. It can be used to test a strategy before deploying it to a live environment.
This command materializes the TradeBot resource as a Job that runs the backtesting command. 

### Using the `hyperopt` command

The `hyperopt` command is used to run a hyperopt. It can be used to find the best parameters for a strategy.
This command materializes the TradeBot resource as a Job that runs the hyperopt command.

### Using the `plot` command

The `plot` command is used to plot the results of a backtest. It can be used to visualize the results of a backtest.
This command materializes the TradeBot resource as a Job that runs the plot command.

### Deploying an AI bot

To enable this mode, the `trade` command must be provided with the `--freqaimodel` flag.

This command materializes the TradeBot resource as a StatefulSet that binds configuration and strategy from referenced resources.
Also, this resource will look for annotation to determine if a GPU is to be used. If a GPU is available, the TradeBot container will be setup with GPU support.

## Config changes and restarts

`config.json` is mounted into the bot's pod from a Secret, and freqtrade reads it once at startup. Editing a `TradeBotConfig`
or `Strategy` always re-renders that Secret immediately - but rewriting the Secret alone does **not** change what an
already-running bot is doing, since nothing tells freqtrade to reload it. Making that take effect means restarting the pod,
and restarting a bot that's holding open positions is a decision this operator will not make for you without being asked.

`TradeBot.spec.updateStrategy` controls which side of that trade-off applies:

- **`Manual` (the default).** A config change is rendered into the Secret and reported, but the running StatefulSet is left
  exactly as it is. The `ConfigDrift` condition turns `True` with a message giving the exact remedy:
  ```
  kubectl rollout restart statefulset/<name> -n <namespace>
  ```
  (or set `spec.updateStrategy: Auto`, which the operator applies itself, clearing `ConfigDrift` in the process).
- **`Auto`.** The operator writes the new config's hash onto the StatefulSet's pod template annotations itself, and
  Kubernetes' own rolling update carries out the restart - no `ConfigDrift` ever appears, since the operator keeps the
  template current on its own.

Use `Auto` for bots where a config change should always take effect immediately (e.g. paper-trading/dry-run bots with no
open positions to protect). Use `Manual` - or leave the field unset - for anything live, and restart it on your own schedule
once you've confirmed the change is safe to apply.

`TradeBot.status.appliedConfigHash` always reflects the hash of the config currently rendered into the Secret, whether or
not it has been rolled out to the running pods yet.

## Config Secret backups

The rendered `config.json` Secret (`<tradebot-name>-config`) necessarily contains live exchange API keys - every Secret
this operator creates for a TradeBot carries the label `freqtrade.io/contains-credentials: "true"` so a backup tool can
be configured to exclude it, e.g. with [Velero](https://velero.io/)'s label selectors:

```bash
velero backup create my-backup --exclude-namespaces freqtrade --selector 'freqtrade.io/contains-credentials!=true'
```

or the equivalent label-exclusion option in Kasten, Stash, or whatever your cluster uses.

**Recommended production topology:** keys still land in etcd unencrypted unless the cluster has
[encryption at rest](https://kubernetes.io/docs/tasks/administer-cluster/encrypt-data/) enabled. For clusters that need
credentials to never touch etcd at all, point `secretRef` at a Secret synced from your vault by the
[External Secrets Operator](https://external-secrets.io/) or mounted via a
[CSI Secret Store driver](https://secrets-store-csi-driver.sigs.k8s.io/) instead of a plain `kubectl apply`'d one - this
operator only ever reads the Secret by name, so either integrates transparently. Neither is wired up by this chart; both
are worth evaluating for anything beyond local/dev use.

The rendered Secret is deliberately *not* `immutable: true` + named by `status.appliedConfigHash`, even though the hash
is already computed and available (see above). Doing that would mean a config change creates a new Secret object rather
than updating the existing one, which would need the StatefulSet's volume reference - not just the config-hash annotation
- to move in step with `spec.updateStrategy`, coupling the Secret's identity to the same Manual/Auto gating its *content*
already goes through independently. That's a real feature, not a rejected idea, but it's a bigger one than "harden the
Secret" implies - revisit it if immutable audit trails for the config Secret specifically become a real requirement.

## Network policy

Every trade-mode TradeBot gets a `NetworkPolicy` (named after the bot) that default-denies ingress to freqtrade's REST API
(port `8080`) and allows only two kinds of traffic in:

- Any FreqUI whose `spec.tradeBotRefs` includes the bot - re-evaluated on every reconcile, so referencing (or
  un-referencing) a bot from a FreqUI updates its `NetworkPolicy` automatically.
- The operator's own pod, for its own future use polling each bot's API for live status - not implemented yet, but the
  network access is opened now so that later work doesn't need a second security-relevant change to land it.

Nothing else - other bots, arbitrary pods in the namespace, etc. - can reach port `8080`. If something else legitimately
needs to (your own monitoring, a custom integration), it isn't currently configurable per-bot; open an issue or add your
own additional `NetworkPolicy` alongside the operator's, since they compose (Kubernetes ORs every applicable policy's
allow rules together).

The allow-from-operator rule depends on the operator's pod knowing its own namespace via the `POD_NAMESPACE` downward-API
env var (wired into both the Helm chart and the kustomize manifests already). A custom Deployment that omits it just
drops that one peer - FreqUI access still works, but nothing else can reach the API on `8080` either, and no error is
raised.

Egress is untouched - a bot can still reach its exchange, Telegram, DNS, etc. without restriction.

## Metrics

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
per-bot below) is at [config/prometheus/grafana-dashboard.json](config/prometheus/grafana-dashboard.json) - import it
directly, or adapt it.

## Bot introspection

The operator polls each trade-mode bot's own freqtrade REST API on a schedule and surfaces what it learns in
`status.bot` and as Prometheus metrics - read-only: nothing here can start, stop, or otherwise act on a bot.

```yaml
spec:
  introspection:
    enabled: true    # default; set false for a bot that deliberately has no api_server (e.g. it never runs one)
    interval: 60s     # default; the admission webhook rejects anything under 10s
```

Polling needs `spec.apiServer` configured on the bot's `TradeBotConfig` with Basic Auth credentials the operator can
read back (`secretRef` or, deprecated, the plaintext `username`/`password` fields) - the same credentials
[Create secrets for exchange API credentials](#2-create-secrets-for-exchange-api-credentials) already covers. A bot
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

## Development

### Prerequisites

- Go 1.19+
- Docker
- Kubernetes cluster (or minikube/kind)
- Operator SDK

### Setup Development Environment

1. Install the required tools:
```bash
# Install controller-gen
make controller-gen

# Install kustomize
make kustomize
```

2. Generate manifests and code:
```bash
make manifests
make generate
```

3. Run the operator locally:
```bash
make run
```

### Running Tests

```bash
# Run unit tests
make test

# Run e2e tests
make test-e2e
```

## License

This project is licensed under the Apache 2.0 License - see the LICENSE file for details.
