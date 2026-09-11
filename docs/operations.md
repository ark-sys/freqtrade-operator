# Operations

For installation, see [installation.md](installation.md). For every field on every CRD, see
[api-reference.md](api-reference.md).

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
- The operator's own pod, which polls each bot's own API for live status - see [Bot introspection](monitoring.md#bot-introspection)
  and [Bot state control](usage.md#bot-state-control).

Nothing else - other bots, arbitrary pods in the namespace, etc. - can reach port `8080`. If something else legitimately
needs to (your own monitoring, a custom integration), it isn't currently configurable per-bot; open an issue or add your
own additional `NetworkPolicy` alongside the operator's, since they compose (Kubernetes ORs every applicable policy's
allow rules together).

The allow-from-operator rule depends on the operator's pod knowing its own namespace via the `POD_NAMESPACE` downward-API
env var (wired into both the Helm chart and the kustomize manifests already). A custom Deployment that omits it just
drops that one peer - FreqUI access still works, but nothing else can reach the API on `8080` either, and no error is
raised.

Egress is untouched - a bot can still reach its exchange, Telegram, DNS, etc. without restriction.

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
- **Monitoring.** At minimum, alert on `freqtrade_bot_up == 0` (see [Bot introspection](monitoring.md#bot-introspection)) - it's
  the one signal that means "this bot has gone quiet and the operator can no longer confirm what it's doing."
  `freqtrade_operator_reconcile_errors_total` and the operator's own liveness/readiness are worth a second alert
  tier: a struggling operator can't roll out config changes or report drift even while bots keep trading unaffected.
- **Config-drift semantics.** Decide `spec.updateStrategy` per bot deliberately (see
  [Config changes and restarts](usage.md#config-changes-and-restarts)) rather than leaving the default everywhere - `Manual`
  (the default) is the right choice for anything holding open positions, since it never restarts a bot out from
  under itself, but that also means a `TradeBotConfig` edit silently does nothing to the running bot until someone
  acts on the `ConfigDrift` condition. Alert on `freqtrade_operator_config_drift == 1` if a stale-but-unnoticed
  config is a risk for your bots.
