# FreqTrade Operator

A Kubernetes operator for managing FreqTrade cryptocurrency trading bots.

> **Disclaimer.** This software deploys and manages automated cryptocurrency trading systems. Running any of these
> resources with `dry_run: false` and real exchange credentials places real funds at risk of loss through strategy
> behavior, exchange conditions, software defects, or misconfiguration. The maintainers and contributors provide this
> software "as is", without warranty of any kind (see [LICENSE](LICENSE)), and are not responsible for financial
> losses incurred through its use. You are solely responsible for the strategies you run, the credentials you grant
> this operator, and the funds under their control. See [SECURITY.md](SECURITY.md) for the threat model this design
> assumes.

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
   - `TradeBot` (`v1beta1`, trade-only): the live bot - a StatefulSet bound to its referenced `TradeBotConfig` and
     `Strategy`. `v1alpha1` is still served for compatibility, converted transparently; see
     [Deploying a TradeBot](#deploying-a-tradebot).
   - `TradeBotConfig`: the rendered `config.json` a `TradeBot` or `Backtest` runs with
   - `Strategy`: the Python strategy script a bot or run executes
   - `FreqUI`: web interface for monitoring and managing bots
   - `Backtest` (`v1beta1`): a one-shot backtesting run, with its own results PVC and extracted summary; see
     [Backtest runs](#backtest-runs-v1beta1)

2. **Controllers**: one per CRD above, each owning that resource's lifecycle - `TradeBot`'s also renders CORS/JWT
   config from referencing `FreqUI` resources, and runs a leader-only background poller for
   [bot introspection](#bot-introspection).

For every field on every CRD - types, defaults, validation constraints, and doc comments straight from the Go
source - see [docs/api-reference.md](docs/api-reference.md), regenerated from `api/*/*_types.go` via
`make api-docs` whenever those types change.

## Namespace model

Every cross-resource reference this operator follows - `TradeBot.spec.strategyRef`/`.configRef`,
`Backtest.spec.strategyRef`/`.configRef`, `FreqUI.spec.tradeBotRefs`, every `secretRef` - is **same-namespace-only**.
A `TradeBot` cannot reference a `Strategy`, `TradeBotConfig`, or credential `Secret` in a different namespace than
its own. This is a deliberate design decision (D3), not a current limitation waiting to be lifted: it keeps RBAC
boundaries meaningful (a `Role` scoped to one namespace is a real security boundary, not one an object's own spec
can silently reach past) and keeps every reference resolvable with a plain namespaced `Get` - no cluster-scoped
lookup, and no admission-time check that has to reason about a second namespace's RBAC to decide whether a
reference is even allowed.

**Recommended pattern:** one namespace per trading environment, not per bot - e.g. `trading-prod`,
`trading-staging`, `trading-backtest`. Bots, their configs, strategies, and credential Secrets that belong together
live together; environments that shouldn't be able to affect each other (most importantly, staging and prod
sharing no namespace at all) are isolated by the same boundary Kubernetes RBAC already uses, with no extra
mechanism this operator has to enforce on your behalf.

## Prerequisites

- Kubernetes cluster 1.29+ (native sidecar containers, GA since 1.29, are required for
  [Backtest result extraction](#backtest-runs-v1beta1))
- kubectl 1.29+
- Helm 3+ (optional, for Helm chart installation)
- Go 1.24+ (for building from source)

## Installation

### Using pre-built images

```bash
kubectl apply -f https://github.com/ark-sys/freqtrade-operator/releases/latest/download/install.yaml
```

`install.yaml` is a one-file install - CRDs, RBAC, webhooks, and the operator Deployment together, built by
`make build-installer` against the exact image the same release job builds, signs, and pushes. The CRDs alone are
also published separately as `freqtrade-operator-crds.tar.gz`, for anyone managing RBAC/Deployment themselves (e.g.
via the Helm chart below) but still wanting the plain CRD YAMLs.

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

The Helm chart is published to GHCR as an OCI artifact on every release (`.github/workflows/helm-release.yml`):

```bash
helm install freqtrade-operator oci://ghcr.io/ark-sys/freqtrade-operator --version <chart-version>
```

The same workflow also publishes a traditional chart repository via GitHub Pages, but that requires Pages to be
enabled for this repository first (not yet done as of this writing - `https://ark-sys.github.io/freqtrade-operator/`
currently 404s). Once it is:

```bash
helm repo add ark-sys https://ark-sys.github.io/freqtrade-operator/
helm repo update
helm install freqtrade-operator ark-sys/freqtrade-operator
```

### Using OLM

Every tagged release also publishes an [OLM](https://olm.operatorframework.io/) bundle image
(`arksys/freqtrade-operator-bundle`), installable via `operator-sdk run bundle` against a cluster that already has
OLM installed, or addable to your own catalog via `make catalog-build` (see `make help`). This project is not yet
listed on [OperatorHub](https://operatorhub.io/) - that needs a one-time submission to the
[community-operators](https://github.com/k8s-operatorhub/community-operators) repository, a manual step not yet
taken.

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


## Deploying a TradeBot

`TradeBot` is trade-only (`v1beta1`, P6-4): it always materializes as a StatefulSet running a live bot bound to its
referenced `TradeBotConfig` and `Strategy`. One-shot runs - what used to be `TradeBot`'s own `backtesting`/`hyperopt`/
`plot` commands under `v1alpha1` - are [Backtest runs](#backtest-runs-v1beta1) now, a dedicated CRD instead of an
overloaded field on a live bot's own spec; see D1 in the production plan for why. A `v1alpha1` TradeBot with
`freqtrade_command` set to anything but `trade` has no `v1beta1` equivalent at all and can no longer be created
(the conversion webhook rejects it, since `v1beta1` is the storage version) - recreate it as a `Backtest` instead.

### Deploying an AI bot

To enable this mode, the `trade` command must be provided with the `--freqaimodel` flag.

This command materializes the TradeBot resource as a StatefulSet that binds configuration and strategy from referenced resources.
Also, this resource will look for annotation to determine if a GPU is to be used. If a GPU is available, the TradeBot container will be setup with GPU support.

## Upgrading to v1beta1

This only matters if you have an existing install from before the `v1beta1` split (P6-4/P6-5) - if this is a fresh
install, skip to [Installation](#installation).

**Trade-mode TradeBots need no action.** `v1alpha1` is still served, and every existing trade-mode TradeBot
(`spec.freqtrade_command` unset or `trade`) converts to and from `v1beta1` transparently. Upgrade the operator and
they keep reconciling exactly as before.

**Job-mode TradeBots (`backtesting`/`hyperopt`/`plot`) need migrating first.** Find them before you upgrade:

```bash
kubectl get tradebots -A -o json | \
  jq -r '.items[] | select(.spec.freqtrade_command != null and .spec.freqtrade_command != "trade") | "\(.metadata.namespace)/\(.metadata.name): \(.spec.freqtrade_command)"'
```

For each one, recreate it as a [Backtest](#backtest-runs-v1beta1) (the same run, expressed as a dedicated one-shot
resource instead of a mode on a live bot - see [Deploying a TradeBot](#deploying-a-tradebot) for why), then delete
the old Job-mode TradeBot. Do this *before* upgrading, not after: once the operator's CRDs are updated,
`v1beta1` becomes the storage version, and `v1beta1` has no representation for Job mode at all
(`api/v1alpha1/tradebot_conversion.go`'s `ConvertTo` rejects it outright). A Job-mode object left in place still
reads back fine immediately after the upgrade - the controller's own `Get` requests `v1alpha1` and needs no actual
conversion for an object still stored as `v1alpha1` bytes - which is exactly why the reconciler carries a second,
explicit rejection for it (see the `3.5` step in [controllers/tradebot/main.go](controllers/tradebot/main.go)).
But any write that has to round-trip that object through `v1beta1` storage, including the status patch the
reconciler itself issues to report that rejection, hits the same conversion error - so an un-migrated Job-mode
TradeBot doesn't fail cleanly with a friendly `ReasonJobModeRemoved` message, it gets stuck retrying a conversion
error every few seconds instead. Migrating first avoids this path entirely.

**Upgrading the CRDs themselves, via Helm:** `helm upgrade` never touches the contents of a chart's `crds/`
directory - that's a deliberate Helm limitation (CRDs are treated as install-once, cluster-scoped, too risky to
prune automatically), not specific to this chart. Apply the new CRDs yourself before or as part of every upgrade:

```bash
kubectl apply -f https://github.com/ark-sys/freqtrade-operator/releases/latest/download/freqtrade-operator-crds.tar.gz
helm upgrade freqtrade-operator oci://ghcr.io/ark-sys/freqtrade-operator --version <chart-version>
```

(fetch and extract the tarball first - `kubectl apply -f <url>` doesn't unpack `.tar.gz` on its own). The
`install.yaml` path (`kubectl apply -f .../install.yaml`) already includes the CRDs on every apply, so a plain
re-apply is sufficient there.

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

## Backtest runs (v1beta1)

`Backtest` is a dedicated CRD for one-shot backtesting runs, introduced alongside `TradeBot` rather than overloading
it: a live bot is a mutable singleton you edit in place, a backtest is an immutable fact about a
`(strategy, config, timerange, data)` tuple you want many of, with history - forcing both through one CRD is why the
old `TradeBot` Job mode needed a spec-hash suffixed onto the Job name just to avoid collisions. A `Backtest`'s name
*is* its run identity, and `kubectl get backtests` gets real printer columns for phase, trades, and profit.

```yaml
apiVersion: freqtrade.io/v1beta1
kind: Backtest
metadata:
  name: sample-strategy-jan-2023
spec:
  configRef:
    name: my-tradebotconfig
  strategyRef:
    name: my-strategy
  timerange: "20230101-20230201"
  timeframe: 5m
  stakeAmount: unlimited
  results:
    size: 2Gi
    retentionPolicy: Delete   # default; Retain keeps the results PVC (owner ref stripped) after this Backtest is deleted
```

**Spec is immutable after creation** (enforced by the API server itself, not just convention) - every field is fixed
the moment the `Backtest` is admitted, so its Job's pod template never needs to change and can never drift from what
actually ran. To change a parameter, create a new `Backtest`; nothing here is a place to iterate in-place.

Typed fields (`timerange`, `timeframe`, `pairs`, `maxOpenTrades`, `stakeAmount`, `dryRunWallet`, `fee`,
`enableProtections`, `breakdown`, `cache`, ...) cover the common cases. For a freqtrade flag the typed surface
doesn't have yet, `spec.extraArgs` is an escape hatch - but it's off by default: set it and the admission webhook
rejects the `Backtest` unless the `freqtrade.io/allow-extra-args: "true"` annotation is also present, and even then
every value is checked against a denylist of flags this operator manages itself (`--config`, `--strategy`,
`--strategy-path`, `--db-url`, `--logfile`, `--userdir`, `--datadir`) and against shell metacharacters. Prefer a
typed field whenever one exists.

`status.phase` (`Pending`/`Running`/`Succeeded`/`Failed`) and the `WorkloadReady` condition reflect the underlying
Job. Once it succeeds, a results-collection sidecar in the same pod (a native sidecar - `restartPolicy: Always` on
an init container entry, GA since Kubernetes **1.29** - running this operator's own image, not a second one to
build and release) reads the run's own result file and writes a summary ConfigMap (`<name>-results`) the operator
reads back into `status.results` and the `ResultsAvailable` condition. A summary that can't be extracted (a
malformed or missing result file) reports `ResultsAvailable=False`/`ResultsUnavailable` rather than failing the
`Backtest` - the run happened either way, and the raw file stays on the results PVC regardless, readable by
mounting it from another pod.

> Field-name mapping for the extracted summary (`totalTrades`, `profitAbs`, `winRatePct`, `sharpeRatio`, ...) is a
> best-effort reading of freqtrade's own result JSON, not verified against a real run's output file - if extraction
> is silently landing on `ResultsUnavailable` for successful runs, this is the first place to check
> ([controllers/backtest/collectresults/parse.go](controllers/backtest/collectresults/parse.go)).

### Running many backtests and comparing results

Because a `Backtest`'s spec is immutable and its name is its identity, sweeping a parameter is just applying one
`Backtest` per value you want to try, then reading their `status.results` back - there's no in-place "edit and
rerun" step to serialize on:

```bash
for tf in 5m 15m 1h; do
  cat <<EOF | kubectl apply -f -
apiVersion: freqtrade.io/v1beta1
kind: Backtest
metadata:
  name: sample-strategy-tf-${tf}
  labels:
    app: sample-strategy
spec:
  configRef: {name: my-tradebotconfig}
  strategyRef: {name: my-strategy}
  timerange: "20230101-20230201"
  timeframe: ${tf}
  stakeAmount: unlimited
EOF
done

# Jobs run in parallel (subject to whatever your cluster/namespace resource
# quota allows) - poll until every run has left Pending/Running:
kubectl get backtests -l app=sample-strategy -w

# status.results is printed as columns directly - no need to dig into YAML
# for the common case of eyeballing which run did best:
kubectl get backtests -l app=sample-strategy \
  -o custom-columns='NAME:.metadata.name,TRADES:.status.results.totalTrades,PROFIT:.status.results.profitAbs,WINRATE:.status.results.winRatePct'
```

Give related runs a shared label (`app=sample-strategy` above, or whatever grouping makes sense for your sweep) when
you create them - `Backtest` doesn't group runs into a "study" or "experiment" object of its own, so a label
selector is the mechanism for finding everything that belongs to one comparison later.

Each `Backtest` gets its own results PVC (`spec.results.size`, default namespace storage class unless overridden),
which by default is deleted along with the `Backtest` (`spec.results.retentionPolicy: Delete`). Runs accumulate -
there is currently no cluster-wide pruning of old `Backtest` objects or their PVCs (D10) - so clean up runs you no
longer need with `kubectl delete backtest -l ...` once you've captured what you wanted from `status.results`, rather
than leaving a sweep's full history to grow unbounded.

`Hyperopt` (parameter optimization, rather than a single fixed-parameter run) is not implemented yet - `Backtest`
ships alone in the first `v1beta1` release by design (D7), with `Hyperopt` following as its own CRD in a later
minor once `Backtest` has real operating experience behind it.

## Production checklist

A few things worth deciding deliberately before pointing any of this at a real exchange account, gathered from
sections above:

- **Resource limits.** Every TradeBot pod gets a default `resources` block (100m/1 CPU request/limit, 256Mi/1Gi
  memory) via `spec.app.pod.resources` if left unset - enough to keep the container out of `BestEffort` QoS, not
  necessarily enough for every strategy. A FreqAI bot in particular should set its own, larger `spec.app.pod.resources`
  explicitly; the default is sized for a plain trade-mode bot.
- **Trades-DB PVC backups.** Each bot's `tradesv3.sqlite` lives on its own PVC (`<tradebot-name>-user-data`, 1Gi on
  the `standard` storage class by default, overridable via `spec.app.pvc`) - it is not covered by anything in
  [Config Secret backups](#config-secret-backups), which only concerns the *credential* Secret. Losing this PVC
  loses trade history and open-position bookkeeping, not funds directly, but back it up like any other stateful
  data your incident response would want after a node or cluster failure.
- **Monitoring.** At minimum, alert on `freqtrade_bot_up == 0` (see [Bot introspection](#bot-introspection)) - it's
  the one signal that means "this bot has gone quiet and the operator can no longer confirm what it's doing."
  `freqtrade_operator_reconcile_errors_total` and the operator's own liveness/readiness are worth a second alert
  tier: a struggling operator can't roll out config changes or report drift even while bots keep trading unaffected.
- **Config-drift semantics.** Decide `spec.updateStrategy` per bot deliberately (see
  [Config changes and restarts](#config-changes-and-restarts)) rather than leaving the default everywhere - `Manual`
  (the default) is the right choice for anything holding open positions, since it never restarts a bot out from
  under itself, but that also means a `TradeBotConfig` edit silently does nothing to the running bot until someone
  acts on the `ConfigDrift` condition. Alert on `freqtrade_operator_config_drift == 1` if a stale-but-unnoticed
  config is a risk for your bots.

## Development

### Prerequisites

- Go 1.24+
- Docker
- Kubernetes cluster (or minikube/kind), 1.29+ - see [Prerequisites](#prerequisites) above
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

## Changelog

See [CHANGELOG.md](CHANGELOG.md) for notable changes to this project.

## License

This project is licensed under the Apache 2.0 License - see the LICENSE file for details.
