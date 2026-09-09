# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versioning follows
[Semantic Versioning](https://semver.org/) once the first release is tagged.

For this project specifically: a **major** bump means an existing CRD's
served version drops support for something it previously accepted (e.g. the
`v1beta1` `TradeBot` no longer accepting Job-mode specs would be major if
`v1alpha1` weren't still served unchanged alongside it), or a CRD/served
version is removed outright. A **minor** bump means a new CRD, a new served
API version, or a new backward-compatible field or capability. A **patch**
bump is a bug or security fix with no API surface change.

No version has been tagged yet - everything below is still `[Unreleased]`.
`make changelog-draft` lists commits since the last tag (or, before any tag
exists, the full history) grouped by a best-effort read of each commit's
leading verb, as a starting point to edit into shape here - it's a draft
aid, not a substitute for actually reading what changed.

## [Unreleased]

### Added
- `Backtest` (`v1beta1`): a dedicated CRD for one-shot backtesting runs, immutable after creation, with typed
  parameters, an `extraArgs` escape hatch gated behind an annotation and a denylist, and results extracted by a
  native sidecar into `status.results` (D1, D6, D8).
- Bot introspection: a leader-only poller reads each trade-mode bot's own freqtrade REST API on a schedule and
  surfaces `status.bot` plus per-bot Prometheus metrics (`freqtrade_bot_up`, `_open_trades`, `_profit_abs`, ...) -
  read-only, nothing here can act on a bot (D4, D9).
  Kubernetes Events on every controller, and operator-level Prometheus metrics (`freqtrade_operator_tradebots`,
  `_config_drift`, `_reconcile_errors_total`, `_config_render_duration_seconds`).
- Admission webhooks (validation and defaults) for `TradeBot`, `TradeBotConfig`, `Strategy`, and `Backtest`;
  conversion webhooks converting `TradeBot` transparently between `v1alpha1` and `v1beta1`.
- LICENSE (Apache 2.0), SECURITY.md (vulnerability reporting + threat model), CONTRIBUTING.md, and a generated API
  reference (`docs/api-reference.md`, regenerated from the Go types via `make api-docs`).
- Helm chart: `PodDisruptionBudget`, `priorityClassName`, `topologySpreadConstraints`, `values.schema.json`, and a
  `helm test` hook that actually exercises the CRDs and the authenticated metrics endpoint.
- Multi-arch (`linux/amd64`+`linux/arm64`) release images, and an `install.yaml` release asset that actually
  matches what the README's install instructions promise.

### Changed
- **`TradeBot` is trade-only as of `v1beta1`** (D1): Job-mode (`backtesting`/`hyperopt`/`plot`) has no `v1beta1`
  representation at all - a `Backtest` is its replacement. `v1alpha1` is still served, unchanged, and converts
  transparently for any existing trade-mode bot; a `v1alpha1` Job-mode `TradeBot` can no longer be created once
  `v1beta1` is the storage version, and must be migrated to a `Backtest` before upgrading (see the README's
  Upgrading section).
- RBAC is now generated from controller-gen markers instead of hand-maintained; status reporting moved from an
  ad-hoc phase/message pair to standard Kubernetes Conditions with `observedGeneration`.
- Reconciliation now uses server-side apply throughout instead of hand-rolled apply functions; predicates were
  simplified and `TradeBotConfig`/`Strategy` changes now trigger the referencing `TradeBot`'s reconciliation via an
  explicit watch instead of periodic re-checks.
- A `TradeBotConfig` edit now only restarts a running bot if `spec.updateStrategy: Auto` is set; the default
  (`Manual`) reports the drift via a `ConfigDrift` condition instead of restarting a bot that may be holding open
  positions out from under itself (D-config-drift).

### Fixed
- Several Phase-0 correctness bugs: nil-pointer dereferences in `configbuilder`, `panic(err)` in resource builders
  (replaced with returned errors), a `TradeBot` that could get stuck reporting a stale error phase forever, a
  finalizer that could block the reconcile worker, and CI running against a staging branch that didn't exist.
  Job mode's immutable-update loop and an orphaned-workload bug that could leave a Job running (and an exchange
  position open) with nothing left in the cluster pointing at it, ahead of Job mode's later removal.
- The manager's metrics endpoint being silently unreachable through the Helm chart due to a port/flag mismatch
  between the chart and `cmd/main.go`.
- Release/install tooling pointing at things that didn't exist or were wrong: README install instructions
  referencing release assets (`crds.yaml`/`operator.yaml`) the release workflow never published; a Helm chart
  image default (`registry.horizonscloud.ovh/...`) that wasn't the image releases actually publish to; a
  hardcoded wrong GitHub org (`freqtrade.github.io` / `ghcr.io/freqtrade`) in the Helm release workflow and every
  Helm chart example/doc file.

### Security
- Plaintext credential fields (`TradeBotConfig` exchange/API-server/Telegram) are rejected by the admission webhook
  unless explicitly opted into via an annotation; `secretRef` is the default, encouraged path.
- The rendered `config.json` Secret is labeled for backup-tool exclusion and hardened; TradeBot pods run under a
  restricted Pod Security Standard (no root, no privilege escalation, read-only root filesystem, `RuntimeDefault`
  seccomp) end to end, including the Backtest results-collection sidecar.
- Every trade-mode bot gets a default-deny `NetworkPolicy` on its REST API port; JWT signing keys are generated
  server-side when not supplied and validated for minimum length; CORS origins are derived from referencing
  `FreqUI` resources instead of left wide open.
- Release images are signed with cosign (keyless, via GitHub Actions OIDC) and ship an SPDX SBOM; the manager runs
  as a distroless, non-root image.

### Removed
- Job-mode `TradeBot` support (see Changed above) - the StatefulSet-only code path, its Job resource builder, and
  every reconciler branch that handled it were deleted outright once `Backtest` existed as its replacement, not
  left in place as unreachable code.
- Debug-oriented banner logging, full spec dumps, and stray `fmt.Println` calls from the reconcilers.
