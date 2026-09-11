# Usage

For installation, see [installation.md](installation.md). For every field on every CRD, see
[api-reference.md](api-reference.md).

## Credentials

[`examples/live-trading/`](../examples/live-trading/) is a complete, self-contained kustomization - its own namespace,
placeholder credential Secrets, a `Strategy`, a `TradeBotConfig`, and a dry-run `TradeBot` (all `v1beta1`). Fill in
real credentials first (or leave them as placeholders and the bot will simply fail to authenticate, rather than
trade on garbage ones), then:

```bash
kubectl apply -k examples/live-trading/
```

This creates everything in a `freqtrade-example` namespace, including the namespace itself - no separate `kubectl
create namespace` step needed. `FreqUI` is deliberately left out of that kustomization (it needs a real Ingress
controller, DNS, and a cert-manager `ClusterIssuer` to actually be reachable); apply
[`examples/live-trading/frequi.yaml`](../examples/live-trading/frequi.yaml) separately once you've adjusted its
`host`/`tls`/`ingressAnnotations` for your own cluster, then:

```bash
kubectl get frequi -n freqtrade-example
```

Credentials are referenced via `TradeBotConfig.spec.exchange.secretRef` (and, the same way,
`spec.apiServer.secretRef` / `spec.notification.telegram.secretRef`) - in `v1beta1` (what the example above uses
and what's shown below), each of these is a typed reference, `secretRef: {name: <secret-name>}`, rather than the
bare-string `secretRef: <secret-name>` that `v1alpha1` still accepts. The operator reads whichever of these keys
the Secret's `data`/`stringData` provides - set only the ones your exchange needs:

| Secret | Expected keys |
|---|---|
| `spec.exchange.secretRef` | `api-key`, `secret`, `password`, `uid`, `account_id`, `wallet_address`, `private_key` |
| `spec.apiServer.secretRef` | `user`, `password`, `jwt_secret_key` |
| `spec.notification.telegram.secretRef` | `token`, `chat-id` |

`spec.exchange.key`/`secret`/`password`/`uid`/`wallet_address`/`private_key`, `spec.apiServer.password`/`jwtSecretKey`, and
`spec.notification.telegram.token` are plaintext equivalents of the fields above. They're deprecated - stored unencrypted
in etcd and readable by anyone who can `get` the `TradeBotConfig` - and the admission webhook rejects setting any of them
unless the `TradeBotConfig` carries the annotation `freqtrade.io/allow-plaintext-credentials: "true"`. Use `secretRef`
instead; `TradeBotConfig`'s `v1beta1` has no plaintext-credential fields at all - see [upgrading.md](upgrading.md).

If `spec.apiServer.enabled: true` and neither source above supplies `jwt_secret_key`, the operator generates a random one
itself and keeps reusing that same value on every later reconcile - freqtrade needs *some* signing key to start its REST
API, and forwarding a short, guessable, or absent one is worse than picking a good one for you. A `jwt_secret_key` you *do*
supply is still rejected if it's under 32 characters, from either source.

See also [`examples/`](../examples/), which has two independent, self-contained sets - `live-trading/` (above) and
[`backtest/`](../examples/backtest/), a self-downloading `Backtest` that needs nothing beyond `kubectl apply -k
examples/backtest/` to actually run. See [`examples/README.md`](../examples/README.md) for what's in each and what to
edit before applying either for real.

## Deploying a TradeBot

`TradeBot` is trade-only (`v1beta1`): it always materializes as a StatefulSet running a live bot bound to its
referenced `TradeBotConfig` and `Strategy`. One-shot runs - what used to be `TradeBot`'s own `backtesting`/`hyperopt`/
`plot` commands under `v1alpha1` - are [Backtest runs](backtesting.md#backtest-runs-v1beta1) now, a dedicated CRD instead of an
overloaded field on a live bot's own spec (see [architecture.md](architecture.md) for the reasoning). A
`v1alpha1` TradeBot with `freqtrade_command` set to anything but `trade` has no `v1beta1` equivalent at all and can
no longer be created (the conversion webhook rejects it, since `v1beta1` is the storage version) - recreate it as a
`Backtest` instead.

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

## Bot state control

`TradeBot.spec.state` (`v1beta1` only) starts or stops a live bot's trading loop without restarting it - the
operator continuously reconciles it, so a bot that crashes and comes back up is re-stopped without you having to
ask again:

```yaml
spec:
  state: Stopped   # default: Running
```

This calls freqtrade's own `/api/v1/start`/`/api/v1/stop` - stopping means no new entries; any trade already open is
left to exit on its own configured signals, exactly like stopping a bot by hand through the freqtrade UI or API.
It's not the same thing as [Bot introspection](monitoring.md#bot-introspection) (which only observes), and not the same as
`TradeBotConfig`'s `spec.advanced.initial_state`, which only sets a bot's state at startup, in `config.json` -
`spec.state` is for changing a **running** bot's state without the restart that avoiding open positions is the
whole point of.

The operator only ever calls start/stop - never anything that opens or closes a position (force-exiting,
force-buying, or force-entering a trade); those stay with you. It also never acts on a bot it can't currently
reach: if `BotReachable` is `False`, `StateReconciled` reports `False`/`BotUnreachable` instead of retrying
blindly, and resumes once the next poll confirms the bot is back.

`spec.state` exists only on `v1beta1` - see [upgrading.md](upgrading.md) if you're still creating
TradeBots as `v1alpha1`: once you set it, that object can only be edited via `v1beta1` from then on.
