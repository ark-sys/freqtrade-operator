# Troubleshooting

Concrete symptoms, what they actually mean, and what to do about them. If something here doesn't
match what you're seeing, `kubectl describe` the object in question first - most of this
operator's own failure states land on a `status.conditions` entry or a Kubernetes `Event`, not
just a raw error buried in a pod log.

## Backtest

### `No data found. Terminating.` in the `freqtrade` container's logs

The backtest ran against a data directory with nothing in it for the requested
`(pair, timeframe, timerange)`. Two different causes produce this identical message:

1. **`spec.data` is unset entirely.** The operator never auto-provisions historical market data -
   omit `spec.data.pvcName` and no cache volume is mounted at all, full stop. See the README's
   [Backtest runs](backtesting.md#backtest-runs-v1beta1) section and
   [`examples/backtest/`](../examples/backtest/) for the required shape (a pre-existing PVC you
   create yourself, referenced by name).
2. **The requested `timerange` predates what the exchange actually has.** This bites hardest
   against `testnet.binance.vision` specifically: its historical candle data does not behave like
   a simple rolling window relative to "now" - it can reset to a fixed, much more recent start
   point (observed directly: BTC/USDT 5m history beginning at a fixed timestamp earlier the same
   day, with genuinely nothing before it). A closed `timerange` computed relative to "N days ago"
   can land entirely before wherever the exchange's own history actually starts on any given day.
   Against a real (non-sandboxed) exchange this is far less likely - real Binance, for instance,
   has years of history - but any exchange can still reject a `timerange` older than what it
   retains for a given pair/timeframe combination.

Check the `init-download-data` container's own logs first
(`kubectl logs <pod> -c init-download-data`) - it logs exactly what candle-data window the
exchange reports being available, which tells you immediately whether this is cause 1 or 2.

### `status.phase: Succeeded` but `status.results` never appears (`ResultsAvailable: False`)

The run genuinely completed - `ResultsAvailable=False`/`ResultsUnavailable` means the
`collect-results` sidecar couldn't parse a summary out of the result file, not that the `Backtest`
failed. Check the sidecar's own logs (`kubectl logs <pod> -c collect-results`) for the specific
parse error, and the result file itself (copied onto `status.resultsPVCName` by the same sidecar,
so it's addressable independent of whether the summary extraction worked) if you need the raw
data.

### The `collect-results` sidecar's logs are completely empty, and it exited non-zero

This is the signature of the sidecar losing a race with kubelet's own SIGTERM delivery, not a
crash in its own logic - already fixed in this codebase (the sidecar now treats SIGTERM as "check
right now" rather than the Go default of "die immediately, nothing flushed"), but worth knowing
the shape of if you're running a build predating that fix, or if you ever see it again: kubelet
sends every remaining native sidecar SIGTERM the instant a pod's last regular container exits,
and that arrives faster than any fixed polling interval can reliably notice on its own.

### `Failed to provision the results-collection sidecar's RBAC` / sidecar logs `cannot get resource "pods"`

The namespace's `freqtrade-backtest-sidecar` `Role` is missing `pods:get` - the sidecar needs it
to poll its own Pod's `containerStatuses` and learn when the main `freqtrade` container has
exited (see [architecture.md](architecture.md#backtest-the-one-shot-path)). This `Role` is
provisioned once per namespace, the first time any `Backtest` runs there, and is **not**
retroactively updated for `Backtest`s created before an operator upgrade that changed what it
grants - if you're seeing this after upgrading the operator itself, deleting and recreating one
`Backtest` in the affected namespace re-triggers provisioning.

## TradeBot / TradeBotConfig

### Admission rejected with a message mentioning `allow-plaintext-credentials`

You set a deprecated plaintext credential field directly (`exchange.key`/`.secret`,
`apiServer.password`, `notification.telegram.token`, ...) instead of `secretRef`. This is
deliberate, not a bug: those fields are stored unencrypted in etcd, readable by anyone who can
read the `TradeBotConfig` object itself. Switch to the corresponding `secretRef` - see
[`examples/live-trading/config.yaml`](../examples/live-trading/config.yaml) for the pattern, and
double-check the *exact* key names the Secret needs (they don't always match the field name you'd
guess - the API server's `secretRef`, for instance, reads a key literally named `user`, not
`username`; see [architecture.md](architecture.md#config-rendering)). If you genuinely need the
plaintext path (e.g. a throwaway dev cluster), the annotation the error message names
(`freqtrade.io/allow-plaintext-credentials: "true"`) opts back in.

### A `secretRef` is set, but the bot behaves as if the credential were never provided

Almost always a Secret key-name mismatch, not a reference-resolution problem - a `secretRef`
pointing at a real Secret that's simply missing the specific key a given field reads silently
renders that one field empty rather than erroring (deliberate: a Secret can supply a subset of
what a config section reads, e.g. exchange credentials without a `password` most exchanges don't
need). Cross-check the exact key your `secretRef` needs to supply against
`controllers/tradebotconfig/configbuilder/` - `exchange.go`, `bot.go` (API server), and
`notification.go` (Telegram) between them cover every `secretRef` this API has.

### Pod stuck in `CreateContainerConfigError`: `image has non-numeric user ... cannot verify user is non-root`

You've overridden `spec.app.pod.image` (or `spec.pod.image` on a `Backtest`) with an image whose
own Dockerfile declares a *named* non-root user (e.g. `USER app`) rather than a numeric UID. The
kubelet can only verify non-root-ness for a numeric `runAsUser` under a `restricted` Pod Security
Standard namespace - it can't resolve a named user against `/etc/passwd` at admission time. Add an
explicit, numeric `runAsUser` via `spec.app.pod.securityContext` (or `spec.pod.securityContext` on
`Backtest`) matching whatever UID that image's own named user actually maps to (check the image's
own Dockerfile, or `docker run <image> id`).

## Installation / cluster setup

### `kubectl apply` fails with `metadata.annotations: Too long: may not be more than 262144 bytes`

Plain (client-side) `kubectl apply` embeds the entire applied object into a
`kubectl.kubernetes.io/last-applied-configuration` annotation, capped at 256KiB - the `TradeBot`
CRD alone (it carries two full served API versions) already exceeds that on its own. Use
`kubectl apply --server-side` instead (tracks field ownership server-side rather than embedding
the whole object) - this is what `make install`/`make deploy` and the README's own install
instructions already do; only relevant if you're applying `dist/install.yaml` (or any of
`config/crd`) by hand.

### Right after installing: webhook calls fail with `connection refused` or an x509 error naming cert-manager

cert-manager's own webhook `Deployment` can report `Available` before the CA bundle it's supposed
to inject into your `ValidatingWebhookConfiguration`/`CustomResourceDefinition` conversion webhook
has actually propagated - a real, observed race, not a hypothetical one. It resolves itself within
a couple of minutes; if you're scripting an install and need a harder guarantee than "wait and
retry," poll the specific webhook configuration's own `clientConfig.caBundle` directly rather than
trusting `Deployment` readiness as a proxy for it:

```bash
kubectl get validatingwebhookconfigurations freqtrade-operator-validating-webhook-configuration \
  -o jsonpath='{.webhooks[0].clientConfig.caBundle}'
```

Empty output means it hasn't propagated yet.

### `make deploy` rewrote `config/manager/kustomization.yaml`

Expected, not a bug: `make deploy IMG=...` runs `kustomize edit set image` against that file as
part of pointing the Deployment at whatever image you passed - it's a local, source-tree-only
side effect of the Makefile target, not something the running operator itself ever touches. If
you were just testing locally with a throwaway `IMG` value, `git restore
config/manager/kustomization.yaml` puts it back.
