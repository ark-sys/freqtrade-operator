# Upgrading to v1beta1

This only matters if you have an existing install from before the `v1beta1` split. If this is a
fresh install, nothing here applies - just follow the main [README](../README.md).

**Trade-mode TradeBots need no action.** `v1alpha1` is still served, and every existing trade-mode
TradeBot (`spec.freqtrade_command` unset or `trade`) converts to and from `v1beta1` transparently.
Upgrade the operator and they keep reconciling exactly as before.

**Job-mode TradeBots (`backtesting`/`hyperopt`/`plot`) need migrating first.** Find them before you
upgrade:

```bash
kubectl get tradebots -A -o json | \
  jq -r '.items[] | select(.spec.freqtrade_command != null and .spec.freqtrade_command != "trade") | "\(.metadata.namespace)/\(.metadata.name): \(.spec.freqtrade_command)"'
```

For each one, recreate it as a [Backtest](../README.md#backtest-runs-v1beta1) (the same run,
expressed as a dedicated one-shot resource instead of a mode on a live bot), then delete the old
Job-mode TradeBot. Do this *before* upgrading, not after: once the operator's CRDs are updated,
`v1beta1` becomes the storage version, and `v1beta1` has no representation for Job mode at all
(`api/v1alpha1/tradebot_conversion.go`'s `ConvertTo` rejects it outright). A Job-mode object left in
place still reads back fine immediately after the upgrade - the controller's own `Get` requests
`v1alpha1` and needs no actual conversion for an object still stored as `v1alpha1` bytes - which is
exactly why the reconciler carries a second, explicit rejection for it (see the `3.5` step in
[controllers/tradebot/main.go](../controllers/tradebot/main.go)). But any write that has to
round-trip that object through `v1beta1` storage, including the status patch the reconciler itself
issues to report that rejection, hits the same conversion error - so an un-migrated Job-mode
TradeBot doesn't fail cleanly with a friendly `ReasonJobModeRemoved` message, it gets stuck retrying
a conversion error every few seconds instead. Migrating first avoids this path entirely.

**`TradeBotConfig`, `Strategy`, and `FreqUI` also have a `v1beta1`.** Same deal as `TradeBot`
above - `v1alpha1` is still served and converts transparently, so nothing you already have needs to
change to keep working. Two things are worth knowing regardless:

- **Plaintext exchange/notification credentials cannot reach `v1beta1` storage at all.** If any
  `TradeBotConfig` still sets `spec.exchange.{key,secret,password,uid,wallet_address,private_key}`,
  `spec.apiServer.{password,jwtSecretKey}`, or `spec.notification.telegram.token` directly instead
  of via `secretRef`, move the value into a Secret and point `secretRef` at it *before* upgrading:

  ```bash
  kubectl create secret generic <name> -n <namespace> --from-literal=api-key=... --from-literal=secret=...
  ```

  Then set `spec.exchange.secretRef: <name>` (and the equivalent for `apiServer`/`notification.telegram`)
  and remove the plaintext field. This isn't optional, and no annotation changes it:
  `freqtrade.io/allow-plaintext-credentials` only ever bypasses `v1alpha1`'s own admission check, and
  `v1beta1` - the storage version once you upgrade - has no field a plaintext credential could
  occupy. A `TradeBotConfig` left with a plaintext field set fails on the very next write (including
  the reconciler's own status patch) with a conversion error, not a clean one-time rejection at
  `kubectl apply`.
- **`FreqUI.spec.tradeBotRefs` is typed in `v1beta1`.** `v1alpha1` still accepts the plain string
  list you already have (`tradeBotRefs: [my-bot]`); nothing to change unless you write `v1beta1`
  objects directly, in which case it's `tradeBotRefs: [{name: my-bot}]` instead.

`Strategy`'s `v1beta1` is a straight mirror of `v1alpha1` - no field changes, no action needed
either way.

## Upgrading the CRDs themselves, via Helm

`helm upgrade` never touches the contents of a chart's `crds/` directory - that's a deliberate Helm
limitation (CRDs are treated as install-once, cluster-scoped, too risky to prune automatically), not
specific to this chart. Apply the new CRDs yourself before or as part of every upgrade:

```bash
kubectl apply -f https://github.com/ark-sys/freqtrade-operator/releases/latest/download/freqtrade-operator-crds.tar.gz
helm upgrade freqtrade-operator oci://ghcr.io/ark-sys/freqtrade-operator --version <chart-version>
```

(fetch and extract the tarball first - `kubectl apply -f <url>` doesn't unpack `.tar.gz` on its
own). The `install.yaml` path (`kubectl apply -f .../install.yaml`) already includes the CRDs on
every apply, so a plain re-apply is sufficient there.

## Rewriting existing objects into `v1beta1` storage

Every object above keeps reading back fine as `v1alpha1` for as long as this operator serves it.
But an object created before this upgrade is still stored as `v1alpha1`'s bytes until something
writes to it again - which will matter if a future release ever drops `v1alpha1` entirely (not
planned yet; marking it deprecated now is what makes that possible later). Force the rewrite
yourself, once you've migrated any plaintext credentials above:

```bash
kubectl get tradebots -A -o yaml | kubectl replace -f -
kubectl get tradebotconfigs -A -o yaml | kubectl replace -f -
kubectl get strategies -A -o yaml | kubectl replace -f -
kubectl get frequis -A -o yaml | kubectl replace -f -
```

This operator does not ship an automated storage-version migrator - four kinds and no existing
installs to migrate makes a `kubectl replace` loop the right amount of tooling. If that stops being
true, [`storage-version-migrator`](https://github.com/kubernetes-sigs/kube-storage-version-migrator)
is the upstream tool built for exactly this.
