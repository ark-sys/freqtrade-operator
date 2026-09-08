# freqtrade-operator — Production Readiness Plan

**Audience:** an AI coding agent (Sonnet 5) implementing this work task by task.
**Source of truth:** this repo at branch `main`. Verified against the code on 2026-09-08.

---

## 0. Read this first (agent briefing)

### What the project is

A Kubebuilder/operator-sdk operator (`domain: freqtrade.io`, module `github.com/ark-sys/freqtrade-operator`) that runs [Freqtrade](https://www.freqtrade.io) trading bots as Kubernetes workloads. Four CRDs, four controllers:

| CRD | Controller | Owns / produces |
|---|---|---|
| `TradeBot` | `controllers/tradebot` | Secret (`config.json`), ConfigMap (strategy .py), PVC, StatefulSet + Service (`trade` mode), or Job (`backtesting`/`hyperopt`) |
| `TradeBotConfig` | `controllers/tradebotconfig` | validation + status only; `configbuilder/` renders freqtrade `config.json` |
| `Strategy` | `controllers/strategy` | validation + status only |
| `FreqUI` | `controllers/frequi` | Deployment, Service, Ingress; drives CORS hosts back into TradeBot config |

**Where it's going:** `TradeBot` becomes trade-only. One-shot runs move to dedicated `Backtest` and `Hyperopt` CRDs in `v1beta1`. See **Phase 6**, and read **§0.3 Locked decisions** before touching anything in Phase 0 — several Phase 0 tasks are deliberately throwaway.

### Current state, honestly

~8.6k lines of Go (2.2k of it generated deepcopy). It compiles (`go build ./...` clean, `go vet ./...` clean). It has **zero unit tests and zero envtest coverage** — the only `_test.go` files are the scaffolded e2e suite in `test/e2e/`, which just checks that the manager pod runs and serves metrics. No reconcile loop has ever been exercised in CI.

The architecture is sound. The gap to production is: correctness bugs that panic or wedge the controller, an API surface with no validation, status that lies, no tests, and secrets handling that is not defensible for a system holding live exchange API keys.

### 0.1 Ground rules for every task

1. **Do not do a global rewrite.** Work task by task, in phase order. Each task lists acceptance criteria — satisfy them before moving on.
2. **After every task, run:** `make fmt vet` then `make test`. Both must be clean. Tests a task adds must pass.
3. **Never hand-edit generated files.** `api/v1alpha1/zz_generated.deepcopy.go` and `config/crd/bases/*.yaml` come from `make generate` / `make manifests`. Change the Go type + markers and regenerate.
4. **Match the existing idiom.** Builders live in `controllers/<crd>/resources/*.go` as `BuildX()` + `ApplyX()` pairs. Keep that shape unless a task says otherwise.
5. **Do not add dependencies** beyond what a task names. The tree is currently controller-runtime + ginkgo/gomega only. Phase 4 adds nothing (use `net/http`). Keep it that way.
6. **Logging:** the codebase is full of `logger.V(2).Info("=== SOMETHING ===")` banners. Task **P0-7** removes them. Until then, don't add more.
7. **Financial-risk framing:** this operator deploys bots that trade real money. Any change that could silently alter a running bot's `config.json` (stake amount, dry-run flag, stoploss, exchange keys) is high-severity. Prefer failing loudly and leaving the bot untouched over "best effort" reconciliation.
8. **Commit granularity:** one commit per task, message `<TASK-ID>: <what changed>`. Do not squash phases.

### 0.2 Verification commands

```bash
make fmt vet              # formatting + go vet
make test                 # unit + envtest (currently vacuous — Phase 5 fixes that)
make lint                 # golangci-lint v2, config in .golangci.yml
make manifests generate   # regenerate CRDs, RBAC, deepcopy — must produce no unexpected diff
make test-e2e             # kind cluster; slow
```

### 0.3 Locked decisions

These were decided by the project owner. **Do not re-litigate them; implement them.**

| # | Decision | Affects |
|---|---|---|
| **D1** | One-shot runs get **their own CRDs** (`Backtest`, `Hyperopt`), each owning a **per-run results PVC**. `TradeBot` becomes trade-only. | P0-4 (scoped down), Phase 6 |
| **D2** | Config changes default to **`Manual`** (report drift, don't restart), **configurable to `Auto`**. | P2-4 |
| **D3** | **Same-namespace references only.** Not a limitation to engineer around — a documented constraint. | P2-5, P6-2, P7-2 |
| **D4** | **Implement bot introspection.** The operator polls each bot's freqtrade REST API for live state. | Phase 4 |
| **D5** | **Typed fields per command** replace free-form `freqtradeArguments`. | Phase 6 (largely falls out of D1) |
| **D6** | Backtest results are extracted by a **sidecar writing a ConfigMap**. | P6-2, P1-2 |
| **D7** | **`Backtest` ships alone in `v1beta1`.** `Hyperopt` follows in a later minor. | P6-3 |
| **D8** | Keep an **annotation-gated `extraArgs`** escape hatch on `Backtest`/`Hyperopt`, with a denylist. | P6-4 |
| **D9** | Introspection stays **read-only**. The client is *built* to accommodate writes; `spec.state` is a scoped follow-on in the reconciler, never in the poller. | P4-3, P4-4 |
| **D10** | **Results PVCs accumulate for now.** Per-object TTL + `RetentionPolicy` only — no cluster-wide pruning. Document the growth characteristic. | P6-1, §Deferred |

**Sequencing consequence you must respect:** D1 means the Job-mode code path inside `TradeBot` is *going to be deleted* in Phase 6. Task **P0-4** therefore does the minimum needed to stop the bleeding and nothing more. Do not invest in elegant Job-mode-inside-TradeBot code — it has a demolition date.

---

## Phase 0 — Stop the bleeding

Bugs that crash the manager, wedge reconciliation permanently, or report false status. Nothing else matters until these are fixed. **Do this phase first, in order.**

---

### P0-1 — Nil-pointer panic on optional config sections

**Severity:** crashes the whole manager process (a panic in a reconciler takes down the pod).

**Files:** [controllers/tradebotconfig/configbuilder/build_config.go:26](controllers/tradebotconfig/configbuilder/build_config.go:26), [:44](controllers/tradebotconfig/configbuilder/build_config.go:44)

**Problem:** `BuildConfig` dereferences `tradeBotConfig.Spec.APIServer.SecretRef` and `tradeBotConfig.Spec.Exchange.SecretRef`. Both `APIServer` and `Exchange` are `*T` with `json:",omitempty"` in [api/v1alpha1/tradebotconfig_types.go](api/v1alpha1/tradebotconfig_types.go) — they are optional. A `TradeBotConfig` omitting either panics the manager the moment a referencing `TradeBot` reconciles.

**Do:**
- Guard both dereferences. `GetSecretData` already returns `(nil, nil)` for an empty name, so compute the secret name into a local `string` that stays `""` when the parent is nil.
- Audit the rest of `configbuilder/` for the same pattern (`bot.go`, `notification.go`, `order.go`, `pricing.go`, `riskmanagement.go`, `pairlistmethods.go`) — every `Spec.X.Y` where `X` is a pointer.
- `Exchange` is *effectively* required. Do **not** silently default it — return a typed `ErrMissingExchange` the caller surfaces as a `ConfigInvalid` condition (P1-3). CRD-level enforcement lands in P1-1.

**Acceptance:**
- Unit test in `configbuilder/build_config_test.go`: `BuildConfig` with `Spec.APIServer == nil` and `Spec.Notification == nil` returns a config without panicking.
- Unit test: `Spec.Exchange == nil` returns a non-nil error, not a panic.

---

### P0-2 — `panic(err)` inside resource builders

**Severity:** crashes the manager on any transient API-server error.

**Files:** [controllers/tradebot/resources/statefulset.go:30](controllers/tradebot/resources/statefulset.go:30), [controllers/tradebot/resources/job.go:37](controllers/tradebot/resources/job.go:37)

**Problem:** `BuildStatefulSet` and `BuildJob` call `c.Get()` to resolve the `Strategy` and `panic(err)` on anything that isn't `IsNotFound`. A 5-second API-server blip kills the operator. On `IsNotFound` they return a **zero-valued** `appsv1.StatefulSet{}` / `batchv1.Job{}`, which the caller in [controllers/tradebot/resources.go](controllers/tradebot/resources.go) passes to `ApplyX` — creating a nameless, namespace-less object the API server rejects with a confusing error.

**Do:**
- Change both signatures to return `(T, error)`. Remove all `panic`.
- The `Strategy` is already fetched by `fetchReferencedResources` in [controllers/tradebot/resources.go](controllers/tradebot/resources.go). **Stop re-fetching it inside builders.** Pass the resolved `strategyName string` down. Builders become pure functions — no `client.Client`, no `context.Context`. This also makes them trivially unit-testable (P5-1).
- Same for the redundant second `Strategy` fetch in `reconcileResources` step 2 — thread through the already-fetched object.

**Acceptance:**
- `grep -rn "panic(" controllers/` returns nothing.
- `BuildStatefulSet` and `BuildJob` take no `client.Client` / `context.Context`.
- Unit tests for both, constructed from plain structs, no fake client.

---

### P0-3 — TradeBot never reports success, and stays stuck in Error

**Severity:** status is actively misleading; `kubectl get tradebot` is useless operationally.

**Files:** [controllers/tradebot/main.go:185,233,245](controllers/tradebot/main.go:185)

**Problem:** the reconciler only assigns `Status.Phase` on *error* paths (`"Error"`, `"ConfigError"`, `"ResourceError"`). There is no success assignment anywhere. So: a healthy bot has `phase: ""` forever; and once a bot errors, the phase is never cleared — after the user fixes the config the happy path runs, `statusChanged` stays `false`, and the stale `"ConfigError"` persists indefinitely.

**Do:** superseded by conditions in **P1-3**, but the immediate fix:
- On the success path set `Phase` from the underlying workload (`Pending` while StatefulSet `ReadyReplicas < 1`, `Running` when ready) and clear `Message`.
- Compute `statusChanged` by comparing against a `DeepCopy()` taken at the top of `Reconcile` — the pattern already used correctly in [controllers/frequi/main.go:44](controllers/frequi/main.go:44). Do not hand-track a boolean across 200 lines.

**Acceptance:** envtest (Phase 5): bad config → `phase: ConfigError`; fix it → phase transitions away without an operator restart.

---

### P0-4 — Contain Job mode (deliberately minimal)

> **Scope note:** per **D1**, Job mode is leaving `TradeBot` entirely in Phase 6. This task stops the damage and nothing more. Do not refactor, do not add features, do not make it elegant. Roughly 40 lines of change.

**Files:** [controllers/tradebot/resources/job.go](controllers/tradebot/resources/job.go), [controllers/tradebot/resources.go:107](controllers/tradebot/resources.go:107)

**Problems:**

1. **`batchv1.Job.Spec.Template` is immutable.** `ApplyJob` compares templates and calls `Update` when they differ — rejected by the API server on every reconcile, forever. Any `freqtradeArguments` change after first apply produces an infinite error loop.
2. **Name collision.** StatefulSet ([statefulset.go:61](controllers/tradebot/resources/statefulset.go:61)) and Job ([job.go:67](controllers/tradebot/resources/job.go:67)) are both named exactly `tradeBot.Name`. Switching `freqtrade_command` from `trade` to `backtesting` leaves an **orphaned StatefulSet still running and trading**.
3. `BackoffLimit: 0` + `RestartPolicy: OnFailure` — one pod failure and the Job is permanently `Failed`.
4. No `TTLSecondsAfterFinished` — completed Jobs and pods accumulate forever.
5. Results are destroyed: [resources.go:107](controllers/tradebot/resources.go:107) passes `pvcName: ""`, so `BuildPod` mounts an `emptyDir` at `/freqtrade/user_data`.

**Do — only this:**
- `ApplyJob` becomes **create-if-absent, never update**. If the Job exists, read its status and return. Delete the `reflect.DeepEqual` comparison block entirely.
- Add `pruneStaleWorkloads(ctx, tradeBot, mode)` to `reconcileResources`: in `trade` mode delete any Job named `tradeBot.Name`; in one-shot mode delete the StatefulSet and Service. **This is the money-losing bug — do it carefully and test it.**
- Set `TTLSecondsAfterFinished: 86400` and `BackoffLimit: 1` as hardcoded constants. Do not add CRD fields for them; `Backtest` will carry them properly in Phase 6.
- Surface a condition when a Job-mode TradeBot's spec changes after the Job exists: `WorkloadImmutable=True, Reason=SpecChangeIgnored`, message telling the user to delete and recreate. Honest beats silently wrong.
- **Leave the `emptyDir` results problem alone.** It is fixed properly by per-run PVCs in Phase 6 (D1). Note it in the condition message.

**Acceptance:**
- envtest: create a Job-mode TradeBot, change `freqtradeArguments`, assert no `Update` error appears and `WorkloadImmutable` is set.
- envtest: flip `freqtrade_command` from `trade` to `backtesting`; assert the StatefulSet **and** Service are deleted.
- envtest: flip back; assert the Job is deleted.

---

### P0-5 — Blocking sleep inside the finalizer

**Severity:** stalls the single reconcile worker up to 2 minutes; other deletions queue behind it.

**File:** [controllers/tradebot/finalizers.go](controllers/tradebot/finalizers.go) (`waitForStatefulSetScaleDown`)

**Problem:** the finalizer scales the StatefulSet to 0 then busy-waits in a 5-second ticker for up to 2 minutes *inside* `Reconcile`. `MaxConcurrentReconciles` is 1 ([setup.go](controllers/tradebot/setup.go)), so one deleting bot blocks every TradeBot in the cluster.

**Do:** convert to the requeue pattern. Scale to 0, return `ctrl.Result{RequeueAfter: 5*time.Second}`, re-check next pass. Track elapsed time from `DeletionTimestamp`; past a deadline (default 2 min, a manager flag) remove the finalizer anyway and record a warning Event.

**Acceptance:**
- No `time.Sleep` or `Ticker` in any reconcile path.
- envtest with two TradeBots: deleting one does not delay reconciliation of the other.

---

### P0-6 — CI does not run on the branch you develop on

**Files:** [.github/workflows/test.yml](.github/workflows/test.yml), [lint.yml](.github/workflows/lint.yml), [test-e2e.yml](.github/workflows/test-e2e.yml)

**Problem:** all three trigger on `staging`. The default branch is `main`. No test, lint, or e2e run has ever gated a change on `main`.

**Do:**
- Retarget to `main` (keep `staging` if still used).
- Add `make manifests generate && git diff --exit-code` as a CI step — generated-artifact drift is a recurring failure mode here (P1-2).
- Pin action versions; add least-privilege `permissions: contents: read` to each workflow.
- Add a `govulncheck` job.

**Acceptance:** a draft PR against `main` runs all workflows.

---

### P0-7 — Logging noise, spec dumps, and a stray `fmt.Println`

**Severity:** low individually, but it makes the operator unoperable — and it leaks credentials.

**Files:** [controllers/tradebot/main.go](controllers/tradebot/main.go), [controllers/tradebot/setup.go](controllers/tradebot/setup.go), [controllers/tradebotconfig/configbuilder/build_config.go:49](controllers/tradebotconfig/configbuilder/build_config.go:49)

**Problem:** ~40 lines of `logger.V(2).Info("=== STATUS UPDATE CHECK ===", ...)`. Several log the **entire spec** (`"spec", tradeBot.Spec`, `fmt.Sprintf("%#v", newObj.Spec)`) — which for a `TradeBotConfig` includes plaintext exchange credentials if `spec.exchange.key` is set (P3-1). That is a credential leak into cluster logs. Plus a bare `fmt.Println` at build_config.go:49.

**Do:**
- Delete every `=== BANNER ===` line. At most one `V(1)` line per reconcile phase.
- **Never log a full spec, config map, or secret.** Names, generations, counts only.
- Delete the `fmt.Println`.
- [cmd/main.go](cmd/main.go) hardcodes `zap.Options{Development: true}` — unstructured dev output in production. Make it flag-driven, defaulting to production encoding.

**Acceptance:** `grep -rn '"===' controllers/`, `grep -rn "fmt.Print" controllers/`, and `grep -rn '"spec", ' controllers/` all return nothing.

---

## Phase 1 — API contract (v1alpha1 hardening)

The CRDs accept almost anything. `config/crd/bases/*.yaml` contains **zero** `kubebuilder:validation` markers from this project — the only `required:` entries come from embedded upstream `k8s.io/api` types.

> These are non-breaking hardening changes to `v1alpha1`. The breaking restructure is Phase 6.

---

### P1-1 — Validation, defaults, and printer columns

**Files:** `api/v1alpha1/*_types.go`

**Do:** annotate every type. Mandatory, non-exhaustive:

`TradeBotSpec`:
- `FreqtradeCommand`: `+kubebuilder:validation:Enum=trade;backtesting;hyperopt;download-data;lookahead-analysis`, `+kubebuilder:default=trade`. The code branches on `== "trade"` in three places with ad-hoc `strings.TrimSpace` defaulting — push it into the API.
- `Config`, `Strategy`: `Required`, `MinLength=1`, DNS-1123 pattern (they are object names).
- `FreqtradeArguments`: `MaxItems=64`. This is arbitrary argv into the trading container; the webhook (P1-4) adds a denylist. Fully replaced in Phase 6 per **D5**.

`DataCacheSpec.DownloadPolicy`: `Enum=always;ifMissing;never`, `default=always`. **Note:** [pod.go](controllers/tradebot/resources/pod.go) treats `ifMissing` identically to `always`. Either implement it or drop the value — do not ship a lie.

`PVCSpec.StorageSize`: change `string` → `resource.Quantity`. Today an invalid value reaches `resource.MustParse` in [persistentvolumeclaim.go](controllers/tradebot/resources/persistentvolumeclaim.go) and **panics the manager**. This is Phase-0-severity living in the API; fixing the type fixes it at admission time.

`BotConfig`: `TradingMode` `Enum=spot;margin;futures`; `MaxOpenTrades` `Minimum=-1`; `DryRunWallet` `Minimum=0`.
`ExchangeSpec.Name`: `Required`.
`APIServerConfig.ListenPort`: `Minimum=1 Maximum=65535`.

Printer columns on all four CRDs — `kubectl get tradebot` should show something:
```go
// +kubebuilder:printcolumn:name="Command",type=string,JSONPath=`.spec.freqtrade_command`
// +kubebuilder:printcolumn:name="Strategy",type=string,JSONPath=`.spec.strategy`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:resource:shortName=tb;tbot
```

CEL (`+kubebuilder:validation:XValidation`) for cross-field invariants: `trailing_stop_positive_offset` > `trailing_stop_positive`; `spec.data` only meaningful when command != `trade`.

**Acceptance:**
- `make manifests` produces `enum:`, `default:`, `required:`, `additionalPrinterColumns`.
- `kubectl apply` of `freqtrade_command: nonsense` is rejected by the API server.
- `resource.MustParse` no longer appears on any user-supplied path.

---

### P1-2 — Make RBAC generated, not hand-written

**Files:** [config/rbac/role.yaml](config/rbac/role.yaml), [helm/templates/clusterrole.yaml](helm/templates/clusterrole.yaml), all controllers

**Problem:** `grep -rn "kubebuilder:rbac" --include=*.go .` returns **nothing**. `config/rbac/role.yaml` is hand-maintained while `make manifests` runs `controller-gen rbac:roleName=manager-role` — guaranteed drift. The Helm ClusterRole is a third hand-maintained copy that has **already diverged**. The role also grants `events: [list, watch]` but **not `create`**, so the operator cannot emit Events once P4-1 adds them.

**Do:**
- Add `+kubebuilder:rbac:groups=...` markers above each `Reconcile`. TradeBot needs core (secrets, configmaps, services, pvcs), apps/statefulsets, batch/jobs, and `events: create;patch`.
- Regenerate; confirm the result matches the hand-written role minus anything unused. **Drop both `pods` and `pods/log`** — neither is used, and per **D6** the results collector is a sidecar, so the log-reading fallback that might have justified `pods/log` is off the table.
- Phase 6 adds a *second*, separate Role for backtest Job pods (`create`/`update` on ConfigMaps only) — do not merge it into the manager role.
- Generate `helm/templates/clusterrole.yaml` from `config/rbac/role.yaml`. Add `make helm-sync` (copies `config/crd/bases/*.yaml` → `helm/crds/`, renders the ClusterRole) plus a CI check. `helm/crds/freqtrade.io_tradebots.yaml` is **already stale**.

**Acceptance:** `make manifests && git diff --exit-code config/rbac/` and `make helm-sync && git diff --exit-code helm/` are both clean, enforced in CI.

---

### P1-3 — Replace `phase`/`message` with standard conditions

**Files:** all `*_types.go`, all controllers, [controllers/shared/utils.go](controllers/shared/utils.go)

**Problem:** every status is `{Phase string, Message string}`. No `observedGeneration` anywhere, so a client cannot tell whether reported status reflects the spec it just applied. `shared.StatusUpdater.UpdateConfigStatus` type-switches over concrete CRD types and returns `"unsupported object type"` for anything else — it doesn't even handle `TradeBot` or `FreqUI`.

**Do:**
- Add to every status struct:
  ```go
  // +optional
  // +patchStrategy=merge
  // +listType=map
  // +listMapKey=type
  Conditions []metav1.Condition `json:"conditions,omitempty"`
  ObservedGeneration int64 `json:"observedGeneration,omitempty"`
  ```
- Keep `Phase` as a derived, human-facing summary for the printer column only. Never the source of truth.
- Define the vocabulary once in `api/v1alpha1/conditions.go`:
  - `Ready`, `ConfigResolved`, `WorkloadReady`
  - Reasons: `ReferenceNotFound`, `ConfigInvalid`, `SecretMissing`, `WorkloadProgressing`, `WorkloadFailed`, `WorkloadImmutable`, `ReconcileError`
  - Reserve `ConfigDrift` (P2-4) and `BotReachable` (P4-3).
- Replace `shared.StatusUpdater` with `meta.SetStatusCondition` + one generic status-patch helper (P2-2).
- Set `ObservedGeneration = obj.Generation` on every successful status write.

**Acceptance:** `kubectl get tradebot -o yaml` shows `status.conditions` with `lastTransitionTime`/`observedGeneration`/`reason`/`message`. No type switch over CRD types remains in `shared/`.

---

### P1-4 — Admission webhooks

**Files:** new `api/v1alpha1/*_webhook.go`, [cmd/main.go](cmd/main.go), `config/webhook/`

**Problem:** [cmd/main.go](cmd/main.go) builds a full `webhook.NewServer(...)` with cert watchers and registers **zero** webhooks. Validation that CEL can't express runs inside the reconcile loop, so bad input is accepted and fails later, asynchronously, in a log line.

**Do:** validating webhooks for:
- `TradeBot`: referenced `TradeBotConfig` and `Strategy` exist **in the same namespace** (per **D3**, reject cross-namespace refs explicitly with a message pointing at the docs); `freqtradeArguments` contains no shell metacharacters and none of the flags the operator owns (`--config`, `--strategy`, `--strategy-path`, `--db-url`, `--logfile`, `--userdir`, `--datadir`).
- `TradeBotConfig`: exchange section present; **if `dry_run` is false then `exchange.secretRef` must be set** — refuse to let a live-trading bot start with no credentials source; `secretRef` targets an existing Secret with the expected keys.
- `Strategy`: move the Python-shape checks from [controllers/strategy/main.go](controllers/strategy/main.go) (`validateStrategyScript`) into the webhook so a bad strategy is rejected at `kubectl apply` time.
- Defaulting webhook for anything CRD defaults can't express.

Wire cert-manager into `config/default/kustomization.yaml` (the `[WEBHOOK]`/`[CERTMANAGER]` patches are scaffolded, commented out) and add cert plumbing to Helm ([helm/values.yaml](helm/values.yaml) already has a commented `webhook-certs` placeholder).

**Acceptance:** applying a TradeBot referencing a nonexistent Strategy is rejected synchronously with a clear message; e2e covers a rejection case.

---

## Phase 2 — Reconciliation correctness

---

### P2-1 — Replace hand-rolled `ApplyX` with server-side apply

**Files:** every `controllers/*/resources/*.go`

**Problem:** six near-identical `ApplyX` functions: get → `reflect.DeepEqual` a hand-picked field subset → set `ResourceVersion` → `Update`, wrapped in `wait.PollUntilContextTimeout`. The comparisons are subtly wrong in every case: `ApplyStatefulSet` compares `existing.Spec.Template.Spec` against a desired spec the API server has since defaulted (`terminationMessagePath`, `dnsPolicy`, `schedulerName`…), so `DeepEqual` is **always false** and the operator issues a pointless `Update` every reconcile. That's write amplification and, for a StatefulSet, potential rolling restarts of a live trading bot.

**Do:**
- Replace all `ApplyX` with **server-side apply**: `r.Patch(ctx, obj, client.Apply, client.FieldOwner("freqtrade-operator"), client.ForceOwnership)`. SSA removes the diffing problem entirely and gives correct multi-owner semantics. (`controllerutil.CreateOrUpdate` is the fallback if SSA causes trouble with a specific type.)
- One shared helper, not six copies.
- Use `controllerutil.SetControllerReference` instead of hand-built `OwnerReferences` — the current code calls `metav1.NewControllerRef` directly in six places, skipping the scheme check and the already-owned-by-another error.

**Acceptance:**
- No `reflect.DeepEqual` on Kubernetes spec objects remains in `controllers/`.
- envtest: reconcile twice with no spec change, assert the StatefulSet's `resourceVersion` is unchanged after the second pass. **Write this test first, watch it fail, then fix.**

---

### P2-2 — Fix status update conflicts

**Files:** [controllers/tradebot/main.go](controllers/tradebot/main.go) (`finishReconciliation`), [controllers/frequi/main.go](controllers/frequi/main.go), [controllers/shared/utils.go](controllers/shared/utils.go)

**Problem:** status writes are get-latest → overwrite whole `.Status` → `Update`, with a `time.Sleep(100ms)` retry loop in `RetryUpdateConfigStatus`. Whole-status overwrite clobbers concurrent writers; the sleep blocks the worker.

**Do:** `retry.RetryOnConflict(retry.DefaultBackoff, ...)` from `k8s.io/client-go/util/retry`, or a status **patch** (`client.MergeFrom`). One helper, used by all controllers.

**Acceptance:** `grep -rn "time.Sleep" controllers/` returns nothing.

---

### P2-3 — Simplify predicates, and add the watches that are missing

**File:** [controllers/tradebot/setup.go](controllers/tradebot/setup.go)

**Problem:** ~180 lines of hand-written `predicate.Funcs` doing type switches and `reflect.DeepEqual` on owned specs, each branch logging. `mainResourcePredicate` reimplements `GenerationChangedPredicate` badly (comparing specs directly misses annotation changes like `freqtrade.io/preserve-data`).

**The real bug hiding in here:** the field indexes `spec.config` and `spec.strategy` are registered in `SetupWithManager` but **never used**. There is no `Watches()` on `TradeBotConfig` or `Strategy`, and `shared.EnqueueTradeBotsByConfigRef` is defined but **never called**. Editing a `TradeBotConfig` does not re-render the bot's config until something else happens to trigger a reconcile. This is a correctness bug, not cleanup.

**Do:**
- `For(&TradeBot{}, builder.WithPredicates(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.AnnotationChangedPredicate{})))` — the annotation half keeps `preserve-data` working.
- Drop `isOwnedByTradeBot`: `Owns()` already filters by ownerRef via controller-runtime's handler.
- **Add `Watches()` on `TradeBotConfig` and `Strategy`** using the existing indexes and `shared.EnqueueTradeBotsByConfigRef`.
- Keep the FreqUI cross-watch; it's correct.
- Raise `MaxConcurrentReconciles` from 1 to a configurable default of 4 (safe once P0-5 removes the blocking wait).

**Acceptance:**
- envtest: change a `TradeBotConfig`'s `stake_amount`; assert the referencing TradeBot's config Secret updates without touching the TradeBot.
- `setup.go` is under 80 lines.

---

### P2-4 — Config drift: report by default, restart on request *(implements D2)*

**Files:** [controllers/tradebot/resources/statefulset.go](controllers/tradebot/resources/statefulset.go), [controllers/tradebot/resources/pod.go](controllers/tradebot/resources/pod.go), `api/v1alpha1/tradebot_types.go`

**Problem:** `config.json` is mounted from a Secret. Updating the Secret does **not** restart the pod, and freqtrade reads its config once at startup. A user edits `TradeBotConfig`, the operator rewrites the Secret and reports success — and the bot keeps trading on the old stake amount and stoploss. This is the most dangerous silent failure in the codebase.

**Do:**
- Compute `configHash = sha256(config.json bytes + strategy script bytes)`, truncated to 16 hex chars. Store the applied hash in `status.appliedConfigHash`.
- Add to `TradeBotSpec`:
  ```go
  // +kubebuilder:validation:Enum=Manual;Auto
  // +kubebuilder:default=Manual
  UpdateStrategy string `json:"updateStrategy,omitempty"`
  ```
  **Default `Manual` per D2.** A bot may be holding open positions; an unrequested restart is the operator's decision to make only when asked.
- **`Auto`**: write `freqtrade.io/config-hash: <hash>` onto the StatefulSet **pod template**, triggering a controlled rolling restart. Emit an Event naming what changed (section names only — never values, per P0-7).
- **`Manual`**: leave the pod template untouched. Set `ConfigDrift=True, Reason=PendingRestart` with a message giving the exact remedy:
  `kubectl rollout restart statefulset/<name> -n <ns>`.
  Clear the condition once `status.appliedConfigHash` matches the running pod's annotation.
- Surface drift as a metric (`freqtrade_operator_config_drift{namespace,tradebot}`) so it's alertable — a bot silently running stale config for a week is exactly what monitoring is for.
- Document both modes prominently in the README with the open-positions rationale.

**Acceptance:**
- envtest, `Manual`: update the config → Secret updated, pod template annotation **unchanged**, `ConfigDrift=True`.
- envtest, `Auto`: update the config → pod template annotation changes, no `ConfigDrift`.
- envtest: after a restart under `Manual`, `ConfigDrift` clears.

---

### P2-5 — Ownership and namespace hygiene *(implements D3)*

- References resolve in `req.Namespace` implicitly. Make it explicit in code and rejected-with-a-message in the webhook (P1-4). **Same-namespace-only is a deliberate, documented constraint (D3), not a TODO** — say so in the API doc comments so future readers don't "fix" it.
- The PVC preservation path in [finalizers.go](controllers/tradebot/finalizers.go) strips owner refs on delete. Verify a re-created TradeBot of the same name **adopts** the preserved PVC rather than failing "already exists". Add a test.
- `FreqUISpec.TradeBotRefs` is a list of bare names with no existence check — a typo silently yields no CORS entry. Surface unresolvable refs as a condition on the FreqUI object.

---

## Phase 3 — Security

This operator holds live exchange API keys. Non-optional.

---

### P3-1 — Remove plaintext credential fields from the API

**File:** [api/v1alpha1/tradebotconfig_types.go](api/v1alpha1/tradebotconfig_types.go)

**Problem:** `ExchangeSpec` has plaintext `Key`, `Secret`, `Password`, `UID`, `PrivateKey`, `WalletAddress`. `APIServerConfig` has plaintext `Password` and `JWTSecretKey`. `NotificationTelegram` has plaintext `Token`. These live in a CRD: stored unencrypted in etcd unless the cluster opts in, readable by anyone with `get tradebotconfig`, and — per P0-7 — **currently logged in full**. Worse, [configbuilder/exchange.go](controllers/tradebotconfig/configbuilder/exchange.go) explicitly prefers these plaintext fields *over* the Secret values.

**Do:**
- Mark all deprecated in `v1alpha1`; **remove in `v1beta1`** (Phase 6). `secretRef` becomes the only path.
- Until removal: reject them in the validating webhook unless an explicit `freqtrade.io/allow-plaintext-credentials: "true"` annotation is present, and emit a warning Event when used.
- Document the expected Secret keys (`api-key`, `secret`, `password`, `uid`, `account_id`, `wallet_address`, `private_key`) — currently discoverable only by reading the source.

**Acceptance:** a `TradeBotConfig` with `spec.exchange.key` set is rejected by the webhook.

---

### P3-2 — Harden the rendered config Secret

**File:** [controllers/tradebot/resources/secret.go](controllers/tradebot/resources/secret.go)

The rendered `config.json` necessarily contains live API keys.

**Do:**
- Add a `freqtrade.io/contains-credentials: "true"` label; document it for backup-tool exclusion.
- `ApplySecret` logs key names and byte lengths on every diff — **remove that logging entirely**.
- Consider `immutable: true` + name-by-hash, pairing naturally with the config-hash from P2-4.
- Evaluate External Secrets Operator / CSI Secret Store so keys never land in etcd. Document as the recommended production topology even if not implemented.

---

### P3-3 — Pod security for the workloads

**File:** [controllers/tradebot/resources/pod.go](controllers/tradebot/resources/pod.go)

**Problems:**
- The `init-user-data` init container runs as **root** (`RunAsUser: 0`) purely to `chmod`/`chown` the PVC. `fsGroup: 1000` is already set — the chown is redundant on most CSI drivers. Drop it; gate behind an opt-in field if some driver needs it.
- No container-level `SecurityContext`: missing `allowPrivilegeEscalation: false`, `readOnlyRootFilesystem`, `capabilities.drop: [ALL]`, `runAsNonRoot: true`, `seccompProfile: RuntimeDefault`. Bot workloads cannot run under a `restricted` PSA namespace as written — note the e2e suite labels the *operator's* namespace `restricted` but never tests a bot namespace.
- `ImagePullPolicy: PullAlways` with the floating tag `freqtradeorg/freqtrade:stable` hardcoded. A restart can silently pick up a new freqtrade version **mid-trading**. Pin a digest, make the default image a manager flag, surface the resolved image in status.
- No default `resources` requests/limits → bots land in `BestEffort` QoS, first to be OOM-killed.

**Acceptance:** an e2e case deploys a bot into a `pod-security.kubernetes.io/enforce=restricted` namespace and it starts.

---

### P3-4 — Network policy and API exposure

- Freqtrade's REST API is on port 8080 with no `NetworkPolicy`. Generate default-deny + allow-from-FreqUI per bot. **Phase 4 (D4) also requires allow-from-operator on 8080** — build both rules together.
- No validation that `jwtSecretKey` exists and is of adequate length. Generate one if absent rather than letting freqtrade start with a weak default.
- `collectCORSHostsForTradeBot` in [main.go](controllers/tradebot/main.go) constructs a `<botname>.<host>` subdomain entry unconditionally. Confirm that's intended — permissive CORS on a trading API deserves a second look.

---

### P3-5 — Supply chain

- Dockerfile builds on `alpine:3.19` with a manually created user. Switch to `gcr.io/distroless/static:nonroot` (kubebuilder default; removes the shell).
- Add `-trimpath`, `-ldflags="-s -w"`, version stamping.
- SBOM (syft) + image signing (cosign) in the release workflow.
- Add `dependabot.yml` for Go modules, Actions, and Docker.
- [release.yml](.github/workflows/release.yml) pushes `arksys/freqtrade-operator:latest` on **every push to main**, including untagged commits. Restrict `latest` to tags.

---

## Phase 4 — Observability and bot introspection

### P4-1 — Kubernetes Events

No controller has an `EventRecorder` (`grep -rn EventRecorder` → nothing). Users get no `kubectl describe` narrative.

**Do:** add `record.EventRecorder` to each Reconciler from `mgr.GetEventRecorderFor("<controller>")`. Emit on: config rendered, workload created/updated, validation failed, secret missing, bot became ready, config drift detected, restart triggered, finalizer timeout. Requires the `events: create;patch` RBAC from P1-2.

### P4-2 — Operator metrics

`metrics-bind-address` defaults to `"0"` (disabled) in [cmd/main.go:69](cmd/main.go:69) while [helm/values.yaml](helm/values.yaml) says `metrics.enabled: true` — the chart and the binary disagree.

**Do:**
- Fix the mismatch.
- Register via `sigs.k8s.io/controller-runtime/pkg/metrics`: `freqtrade_operator_tradebots{phase}`, `freqtrade_operator_reconcile_errors_total{controller,reason}`, `freqtrade_operator_config_render_duration_seconds`, `freqtrade_operator_config_drift{namespace,tradebot}` (P2-4), `freqtrade_operator_backtests_total{result}` (Phase 6).
- Ship the `ServiceMonitor` (scaffold exists at [config/prometheus/monitor.yaml](config/prometheus/monitor.yaml), absent from Helm) and a Grafana dashboard JSON.

---

### P4-3 — Bot introspection *(implements D4)*

The operator polls each bot's freqtrade REST API and surfaces live trading state in status and as metrics. This is the highest-value operational feature in the plan and the biggest architectural commitment — it makes the operator a **client** of the workloads it manages.

**Design constraints — get these right or it will hurt:**

1. **Not in the reconcile loop.** Polling N bots inside `Reconcile` ties reconcile latency to bot responsiveness, and one hung bot blocks the worker. Implement a separate poller registered with `mgr.Add(&BotPoller{...})` implementing both `manager.Runnable` and `manager.LeaderElectionRunnable` (only the leader polls). Internally: a work queue plus a small worker pool, one in-flight request per bot.
2. **Read-only, but build the client for more** *(D9)*. Poll only — no start/stop/force-exit in this task. However, structure the client around a general `do(ctx, method, path, body) (*Response, error)` primitive rather than a `get(path)` helper, so P4-4 widens it without a rewrite. This costs nothing now.
3. **No new dependencies.** `net/http` with an explicit `http.Client{Timeout: 5s}`, redirects disabled, proxy disabled, and a `io.LimitReader` on the response body.
4. **Never log response bodies.** They contain balances and open positions. Log status codes and durations only.

**Do:**

- API surface:
  ```go
  type IntrospectionSpec struct {
      // +kubebuilder:default=true
      Enabled *bool `json:"enabled,omitempty"`
      // +kubebuilder:default="60s"
      Interval metav1.Duration `json:"interval,omitempty"`  // webhook enforces >= 10s
  }
  ```
  on `TradeBotSpec`, plus a `status.bot` subobject:
  ```go
  type BotStatus struct {
      State            string       `json:"state,omitempty"`        // running | stopped | unknown
      Version          string       `json:"version,omitempty"`
      DryRun           *bool        `json:"dryRun,omitempty"`
      OpenTrades       *int         `json:"openTrades,omitempty"`
      MaxOpenTrades    *int         `json:"maxOpenTrades,omitempty"`
      TotalProfitAbs   string       `json:"totalProfitAbs,omitempty"`   // string, not float — no floats in API types
      TotalProfitPct   string       `json:"totalProfitPct,omitempty"`
      LastPollTime     *metav1.Time `json:"lastPollTime,omitempty"`
      LastPollError    string       `json:"lastPollError,omitempty"`
  }
  ```
- Endpoints: `/api/v1/ping` (unauthenticated — use for liveness), then `/api/v1/status`, `/api/v1/profit`, `/api/v1/count`, `/api/v1/show_config`, `/api/v1/version`. Auth is HTTP Basic from the `apiServer.secretRef` Secret — the operator already renders those credentials, so it can read them back.
- Condition `BotReachable` (`True` / `False` with reason `ConnectionRefused`, `AuthFailed`, `Timeout`).
- **Circuit breaking:** per-bot exponential backoff after consecutive failures, capped at ~10× the interval. A bot that is down must not generate error spam or hammer the API.
- Prometheus gauges — arguably more valuable than the status fields, since this is what you alert on:
  `freqtrade_bot_up`, `freqtrade_bot_state`, `freqtrade_bot_open_trades`, `freqtrade_bot_max_open_trades`, `freqtrade_bot_profit_abs`, `freqtrade_bot_profit_ratio`, `freqtrade_bot_balance`, `freqtrade_bot_last_poll_timestamp_seconds` — labelled `{namespace, tradebot, strategy, exchange, dry_run}`. Delete series when a TradeBot is deleted (avoid unbounded cardinality growth).
- Requires the operator→bot:8080 NetworkPolicy rule from P3-4.
- Extend the Grafana dashboard with a per-bot P&L row.

**Acceptance:**
- Unit tests for the client against an `httptest.Server` covering: happy path, 401, timeout, malformed JSON, oversized body.
- envtest/e2e: a dry-run bot's `status.bot.state` reaches `running` and `freqtrade_bot_up` is 1.
- A bot scaled to 0 flips `BotReachable=False` and backs off — assert the request rate does not grow.
- `grep` confirms no response body is passed to a logger.

---

### P4-4 — Bot state control *(follow-on to P4-3; implements D9)*

> Do this **after** P4-3 is running in anger. It depends on `BotReachable` existing — you cannot reconcile desired state against a bot you cannot reach.

**Why it's a separate task, not part of P4-3:** observation and desired state are different loops. Introspection *watches* (a poller, leader-elected, on its own interval). `spec.state` is *desired state* and must be **continuously reconciled** — a bot that crashes and restarts comes back `running`, and the operator has to notice and re-stop it. Putting a write into the poller would be a category error.

**What gap this actually fills:** `AdvancedConfig.InitialState` already sets a bot's state at startup via `config.json`. The value here is changing state on a **running** bot without a restart — which matters precisely because a restart is the thing you're trying to avoid when a bot holds open positions (cf. **D2**).

**Do:**
- Add to `TradeBotSpec`:
  ```go
  // +kubebuilder:validation:Enum=Running;Stopped
  // +kubebuilder:default=Running
  State string `json:"state,omitempty"`
  ```
- Reconcile it in the **TradeBot reconciler**: compare `spec.state` against `status.bot.state` (populated by P4-3) and issue `POST /api/v1/start` or `/api/v1/stop` on mismatch. Requeue to re-verify; do not fire-and-forget.
- Skip the call entirely when `BotReachable=False` — set `StateReconciled=False, Reason=BotUnreachable` and back off. Never retry-storm an unreachable bot.
- Emit an Event on every state transition. This is a trading-affecting action and must be auditable.
- **Hard scope boundary:** start and stop only. Do **not** implement `/api/v1/forceexit`, `/forcebuy`, `/forceenter`, or anything that opens or closes a position. Those are trading decisions and stay with the human. Write this boundary into the package doc comment so it isn't eroded later.
- Update the threat model (P7-2): the operator can now halt trading. That is mostly a safety property — a kill switch — but it is a new capability that belongs in the doc.

**Acceptance:**
- envtest/e2e: set `spec.state: Stopped` → bot stops; restart the pod → operator re-stops it without user action.
- Unreachable bot → `StateReconciled=False, Reason=BotUnreachable`, request rate flat.
- `grep -rn "forceexit\|forcebuy\|forceenter" controllers/` returns nothing.

---

## Phase 5 — Testing

**Highest-leverage phase in the document.** There are currently zero unit tests; everything above is unverifiable without it. If you reorder anything, do Phase 0 → Phase 5 → the rest.

### P5-1 — Unit tests for pure logic

Target ≥80% on packages that need no cluster:

| Package | What to test |
|---|---|
| `controllers/tradebotconfig/configbuilder` | every `BuildXConfig`: nil input → nil output; secret-vs-plaintext precedence; JSON shape vs [schema.json](schema.json). Golden-file the rendered `config.json` for a full example. |
| `controllers/tradebot/resources` | `BuildPod` (trade vs job; cache vs none; override merging), `BuildStatefulSet`, `BuildJob`, `BuildSecret`, `BuildUserDataPVC`, `mergePodSpecOverrides`, `mergePVCSpecOverrides` |
| `controllers/tradebot` | `collectCORSHostsForTradeBot` (dedup, scheme inference, localhost case), `validateCORSHosts`, config-hash stability |
| `controllers/strategy` | `isValidPythonClassName` (a hand-rolled ASCII loop carrying a `// TODO: A new hope` — test it, then consider a compiled regexp), `validateStrategyScript` |
| bot API client (P4-3) | against `httptest.Server` |

Table-driven. Golden files under `testdata/`.

### P5-2 — envtest integration tests

Add `controllers/suite_test.go` using `envtest` (the Makefile already wires `setup-envtest` and `ENVTEST_K8S_VERSION` — it's simply unused). Cover:

- TradeBot create → Secret + ConfigMap + PVC + StatefulSet + Service, correct owner refs.
- Job-mode create → Job, no StatefulSet.
- **Mode switch trade → backtesting prunes the StatefulSet and Service** (P0-4 — the money-losing case).
- Missing `Strategy` / `TradeBotConfig` ref → `ConfigResolved=False, Reason=ReferenceNotFound`, no workload, no panic.
- `TradeBotConfig` edited → config Secret re-rendered (P2-3).
- **Idempotency:** reconcile twice, no `resourceVersion` churn on owned objects (P2-1).
- Config drift `Manual` vs `Auto` (P2-4).
- Deletion: finalizer runs, StatefulSet scaled to 0, resources GC'd.
- Deletion with `freqtrade.io/preserve-data: "true"` → PVC survives, owner ref stripped, annotations set; re-created TradeBot adopts it (P2-5).
- `observedGeneration` tracks `metadata.generation`.
- FreqUI ↔ TradeBot CORS propagation.
- Phase 6: `Backtest` create → Job + results PVC; completion → parsed metrics in status; deletion → PVC GC'd (or retained).

### P5-3 — e2e tests worth their runtime

The current suite ([test/e2e/e2e_test.go](test/e2e/e2e_test.go)) only asserts the manager runs and serves metrics. Extend to:

- Deploy a TradeBot in **dry-run mode** against a stub exchange. **Never a real exchange, never real credentials, in CI.**
- Pod reaches Ready; `/api/v1/ping` responds; introspection populates `status.bot` (P4-3).
- FreqUI reaches the bot.
- Run a `Backtest` to completion; assert results land on the PVC **and** metrics appear in `status.results`.
- Webhook rejection cases (P1-4).
- Bot workload starts in a `restricted` PSA namespace (P3-3).
- Upgrade test: install N, apply resources, upgrade to N+1, assert bots keep running and CRDs convert.

### P5-4 — Test infrastructure

- Fixture builders so tests don't hand-construct 60-line CRs.
- `make test-unit` / `test-integration` / `test-e2e` split so the fast loop stays fast.
- Coverage gate in CI — start where Phase 5 lands, ratchet up, never let it drop.
- Verify or delete [hack/lazy-test.sh](hack/lazy-test.sh) and [hack/kind-test-cluster.sh](hack/kind-test-cluster.sh).

---

## Phase 6 — `v1beta1`: the API split *(implements D1, D3, D5)*

One coherent breaking change carrying every API restructure at once, protected by the tests from Phase 5.

### Why split

`TradeBot.spec.freqtrade_command` overloads two opposite lifecycles. A **bot** is a mutable singleton you edit in place. A **backtest** is an immutable fact about a `(strategy, config, timerange, data)` tuple that you want *many* of, with history. Forcing both through one CRD is the root cause of the name collision, the orphaned StatefulSet, the illegal `Job.Spec.Template` update, and a status vocabulary that cannot express both "Ready" and "Succeeded".

Splitting makes the hard parts vanish rather than get fixed. An immutable CR maps cleanly onto an immutable Job, so the spec-hash-in-the-Job-name workaround disappears — **the CR is the run identity**. And `kubectl get backtests` gains real printer columns, which is the actual point of backtesting and is impossible today.

The code already grew the seam: `TradeBot.Spec.Data` is gated on `!isTrade` at [pod.go:35](controllers/tradebot/resources/pod.go:35) — a backtest-only field sitting on the wrong CRD.

### P6-1 — `Backtest` CRD

> **Shape note (because of D7).** `Hyperopt` ships in a later minor, but factor the fields both will share into an embedded `RunSpec` with `json:",inline"` **now**. Inline embedding is invisible in the serialized JSON, so `Hyperopt` can reuse it later with **zero API churn on `Backtest`**. Retrofitting the struct after `v1beta1` is stored would be a breaking change for no benefit.
>
> `RunSpec` holds: `ConfigRef`, `StrategyRef`, `Timerange`, `Timeframe`, `Pairs`, `Data`, `Results`, `Pod`, `TTLSecondsAfterFinished`, `ExtraArgs`. Backtest-specific fields (`Breakdown`, `Cache`, `EnableProtections`, …) stay on `BacktestSpec` directly.

```go
type BacktestSpec struct {
    RunSpec `json:",inline"`   // shared with Hyperopt later (D7)

    // All fields immutable after creation — enforced by CEL
    // (self == oldSelf) on the whole spec. To change a parameter, create a new Backtest.

    ConfigRef   corev1.LocalObjectReference `json:"configRef"`
    StrategyRef corev1.LocalObjectReference `json:"strategyRef"`

    // Typed parameters — replaces freqtradeArguments (D5)
    Timerange         string             `json:"timerange,omitempty"`   // pattern: ^\d{8}-(\d{8})?$
    Timeframe         string             `json:"timeframe,omitempty"`   // pattern: ^\d+[mhdw]$
    TimeframeDetail   string             `json:"timeframeDetail,omitempty"`
    Pairs             []string           `json:"pairs,omitempty"`
    MaxOpenTrades     *int               `json:"maxOpenTrades,omitempty"`
    StakeAmount       string             `json:"stakeAmount,omitempty"` // "unlimited" or a quantity
    DryRunWallet      *resource.Quantity `json:"dryRunWallet,omitempty"`
    Fee               *string            `json:"fee,omitempty"`
    EnableProtections *bool              `json:"enableProtections,omitempty"`
    Breakdown         []string           `json:"breakdown,omitempty"`   // enum: day;week;month
    Cache             string             `json:"cache,omitempty"`       // enum: none;day;week;month

    Data    *DataSourceSpec `json:"data,omitempty"`     // moved off TradeBot
    Results *ResultsSpec    `json:"results,omitempty"`  // per-run PVC (D1)
    Pod     *PodSpec        `json:"pod,omitempty"`      // resources, scheduling

    // +kubebuilder:default=86400
    TTLSecondsAfterFinished *int32 `json:"ttlSecondsAfterFinished,omitempty"`
}

type ResultsSpec struct {
    // +kubebuilder:default="1Gi"
    Size             resource.Quantity `json:"size,omitempty"`
    StorageClassName *string           `json:"storageClassName,omitempty"`
    // +kubebuilder:validation:Enum=Delete;Retain
    // +kubebuilder:default=Delete
    RetentionPolicy string `json:"retentionPolicy,omitempty"`
}

type BacktestStatus struct {
    Conditions         []metav1.Condition `json:"conditions,omitempty"`
    ObservedGeneration int64              `json:"observedGeneration,omitempty"`
    Phase              string             `json:"phase,omitempty"` // Pending|Running|Succeeded|Failed
    JobName            string             `json:"jobName,omitempty"`
    ResultsPVCName     string             `json:"resultsPVCName,omitempty"`
    StartTime          *metav1.Time       `json:"startTime,omitempty"`
    CompletionTime     *metav1.Time       `json:"completionTime,omitempty"`
    Results            *BacktestResults   `json:"results,omitempty"`
}

// All numeric results are strings — never use float64 in an API type
// (no stable round-trip, and controller-gen needs allowDangerousTypes).
type BacktestResults struct {
    TotalTrades    int    `json:"totalTrades,omitempty"`
    ProfitAbs      string `json:"profitAbs,omitempty"`
    ProfitPct      string `json:"profitPct,omitempty"`
    WinRatePct     string `json:"winRatePct,omitempty"`
    MaxDrawdownPct string `json:"maxDrawdownPct,omitempty"`
    SharpeRatio    string `json:"sharpeRatio,omitempty"`
    SortinoRatio   string `json:"sortinoRatio,omitempty"`
    CAGRPct        string `json:"cagrPct,omitempty"`
    BestPair       string `json:"bestPair,omitempty"`
    WorstPair      string `json:"worstPair,omitempty"`
    ResultFile     string `json:"resultFile,omitempty"` // path on the results PVC
}
```

Printer columns — this is the payoff:
```go
// +kubebuilder:printcolumn:name="Strategy",type=string,JSONPath=`.spec.strategyRef.name`
// +kubebuilder:printcolumn:name="Timerange",type=string,JSONPath=`.spec.timerange`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Trades",type=integer,JSONPath=`.status.results.totalTrades`
// +kubebuilder:printcolumn:name="Profit%",type=string,JSONPath=`.status.results.profitPct`
// +kubebuilder:printcolumn:name="Drawdown%",type=string,JSONPath=`.status.results.maxDrawdownPct`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:resource:shortName=bt
```

**Controller behaviour:**
- On create: render `config.json` (reuse `configbuilder` — generalize `BuildConfig` to take `name, namespace string` instead of `*TradeBot`; it only uses those two fields today), create the results PVC, create the Job. Job name = `<backtest-name>` — no hashing needed, the CR *is* the identity.
- Spec is immutable via CEL, so the Job never needs updating. `ApplyJob`'s whole problem class is gone.
- Watch the Job; mirror its status into `Backtest.status`.
- On success, extract results (P6-2).
- Results PVC is owned by the `Backtest`; `RetentionPolicy: Retain` strips the owner ref on delete (reuse the existing `preserve-data` pattern in [finalizers.go](controllers/tradebot/finalizers.go)).
- **Per D10, results accumulate.** `TTLSecondsAfterFinished` reaps the *Job*; it does not touch the results PVC — that is deliberate, since the results are the artifact you kept the run for. There is no pruning controller. See **§Deferred by decision** for the growth math and the options when it becomes a problem.

### P6-2 — Results extraction *(implements D6)*

Freqtrade writes `user_data/backtest_results/backtest-result-<ts>.json` plus a `.last_result.json` pointer. The operator cannot mount the RWO results PVC itself (single deployment, node-bound volume).

**Mechanism — decided, build this:** a lightweight **sidecar** in the same Job pod, sharing the results volume. It waits for the main container to exit, reads the result JSON, and writes a summary to a ConfigMap `<backtest-name>-results`. The operator watches that ConfigMap and parses it into `status.results`.

**Implementation detail:**
- Use a **native sidecar** (`initContainers` entry with `restartPolicy: Always`, GA since Kubernetes 1.29) rather than a plain second container. A plain sidecar never exits, so the Job never completes. If you must support < 1.29, use a shared-`emptyDir` completion-file handshake instead and document the floor.
- The sidecar is the operator's own image with a `--mode=collect-results` flag — do **not** introduce a second image to build, scan, and release.
- Dedicated ServiceAccount per namespace with a narrow Role: `create`/`update` on ConfigMaps, scoped with `resourceNames: [<backtest-name>-results]` where the API allows. Keep it separate from the manager role (P1-2).
- ConfigMap is owned by the `Backtest` so it GCs with the run.
- Parse defensively: a malformed or absent result file sets `Succeeded` with `ResultsUnavailable=True` rather than failing the whole `Backtest`. The run happened; only the summary is missing, and the raw file is still on the PVC.

The ConfigMap is independently useful — users can read raw results without mounting anything.

*Alternatives rejected: operator-run collector Job (extra pod per run, more moving parts); reading via `pods/log` (fragile, size-limited — and P1-2 now drops that grant); object-storage upload (best long-term, adds a dependency — leave as a documented future exporter).*

### P6-3 — `Hyperopt` CRD — **deferred to a later minor** *(D7)*

**Not part of `v1beta1`.** `Backtest` ships alone; `Hyperopt` lands in a subsequent minor once `Backtest`'s shape has settled in real use. Adding a CRD is additive and non-breaking, so nothing is lost by waiting — and quite a lot is gained by not designing two CRDs against zero production feedback.

**The only thing to do now:** the `RunSpec` inline embedding in P6-1. That is the sole decision that would be expensive to retrofit.

Sketch, for when it comes (do not build it yet): sibling CRD embedding the same `RunSpec`, plus `epochs`, `spaces` (`buy;sell;roi;stoploss;trailing;protection;trades`), `lossFunction` (enum), `jobWorkers`, `randomState`, `minTrades`. Status carries best loss and best params, and — the valuable part — **emits the best parameters as a ConfigMap** the user can feed straight into a `TradeBotConfig`, closing the optimize→deploy loop inside the cluster. The P6-2 sidecar generalizes to collect it.

### P6-4 — `TradeBot` becomes trade-only

- Delete `FreqtradeCommand`, `FreqtradeArguments`, and `Data` from `TradeBotSpec` in `v1beta1`.
- Delete the Job branch from `reconcileResources`, `BuildJob`, `ApplyJob`, and the `jobWatchPredicate`. The P0-4 containment code goes with it.
- Add the small typed set a live bot actually needs (per **D5**): `spec.updateStrategy` (P2-4), `spec.introspection` (P4-3), `spec.state` (P4-4).
- **Escape hatch — decided, build it** *(D8)*: a fully typed surface means new freqtrade flags would otherwise require an operator release. Keep `extraArgs` on `Backtest`/`Hyperopt` **only** (never on `TradeBot` — a live bot's argv is not a place for improvisation), gated behind a `freqtrade.io/allow-extra-args: "true"` annotation and validated against a denylist of operator-owned flags: `--config`, `--strategy`, `--strategy-path`, `--db-url`, `--logfile`, `--userdir`, `--datadir`. Reject shell metacharacters. Typed fields remain the documented surface; this is a pressure valve, not an alternative — say so in the field's doc comment, and emit a warning Event when it is used so its usage is visible rather than quietly load-bearing.

### P6-5 — Reference types, credential removal, conversion

- Every bare-string reference becomes `corev1.LocalObjectReference`: `TradeBotSpec.Config`/`.Strategy`, `FreqUISpec.TradeBotRefs`, every `SecretRef`. **Same-namespace only per D3** — document the constraint in the type's doc comment so nobody "fixes" it later.
- **Delete** every plaintext credential field deprecated in P3-1.
- Conversion webhooks `v1alpha1` ↔ `v1beta1`, with round-trip fuzz tests. A `v1alpha1` TradeBot with `freqtrade_command: backtesting` has no `v1beta1` equivalent — converting it must fail loudly with a message pointing at `Backtest`, and the upgrade guide must cover the manual migration.
- `v1beta1` becomes the storage version; `v1alpha1` served but deprecated.

---

## Phase 7 — Packaging, docs, release

### P7-1 — Helm chart

- CRDs in `helm/crds/` are stale vs `config/crd/bases/` (`freqtrade.io_tradebots.yaml` differs). Automate via `make helm-sync` (P1-2). Helm never upgrades `crds/` — document the manual CRD upgrade step, or move CRDs to templates behind a `crd.install` flag.
- Add `values.schema.json`.
- Missing: `PodDisruptionBudget`, `priorityClassName`, `topologySpreadConstraints`, `ServiceMonitor`, webhook cert wiring (P1-4), workload-side `seccompProfile`.
- Make `helm test` meaningful ([helm/templates/tests/test-deployment.yaml](helm/templates/tests/test-deployment.yaml) exists).
- [helm/values.yaml](helm/values.yaml) defaults `image.repository` to a private registry (`registry.horizonscloud.ovh/...`) while releases publish to `arksys/freqtrade-operator`. Pick one; make the default installable.

### P7-2 — Docs

The README describes install paths that don't exist: it points at `releases/latest/download/crds.yaml` and `operator.yaml` (the workflow publishes `freqtrade-operator-crds.tar.gz`) and a Helm repo at `ark-sys.github.io` (verify it's live). Fix or mark as planned.

Add:
- **Threat model** — what the operator can access, where credentials live, what an attacker with `get tradebotconfig` gets, and what P4-3 introspection means (the operator now holds and uses trading credentials to make network calls).
- **Production checklist** — resource limits, trades-DB PVC backups, monitoring, and the P2-4 config-drift semantics.
- **Namespace model (D3)** — same-namespace references only, stated as a design decision with rationale, plus the recommended pattern (one namespace per trading environment).
- **Backtest/Hyperopt guide** — the run-many-compare-results workflow that Phase 6 unlocks.
- Generated API reference (`crd-ref-docs`), upgrade guide, real `CHANGELOG.md`.
- `CONTRIBUTING.md`, `SECURITY.md`, and a `LICENSE` file — code headers say Apache 2.0 but **there is no LICENSE at repo root**.
- A **disclaimer**: this software deploys automated trading systems; users are responsible for their own funds.

### P7-3 — Release engineering

- SemVer + generated changelog.
- Multi-arch images (`make docker-buildx` exists; the release workflow builds single-arch).
- Publish `install.yaml` via `make build-installer` so the README's instructions become true.
- OLM bundle (`make bundle` is scaffolded) if OperatorHub is a goal — otherwise **delete** the scorecard/bundle scaffolding to cut surface area.

### P7-4 — Repo hygiene *(do this before Phase 0)*

- `.idea/` is committed, including `workspace.xml`. Remove from git, add to `.gitignore`.
- `helm/ARGOCD-CONFIGURATION.md` is untracked; `roadmap.md` is deleted-but-staged.
- **There is substantial uncommitted work across `pod.go`, `job.go`, `statefulset.go`, `resources.go`, and `setup.go`.** Review and commit or discard it before starting — several Phase 0 tasks touch exactly these files.

---

## Suggested order

```
P7-4                      — clean the working tree first
Phase 0  (P0-1 … P0-7)    — stop the bleeding; 1–2 days
Phase 5  (P5-1, P5-2)     — pull forward; nothing else is verifiable without it
Phase 1  (P1-1 … P1-4)    — v1alpha1 hardening
Phase 2  (P2-1 … P2-5)    — reconciliation correctness
Phase 3  (P3-1 … P3-5)    — security
Phase 4  (P4-1 … P4-3)    — observability + introspection (independent of Phase 6)
Phase 6  (P6-1, P6-2,     — v1beta1: Backtest split, now protected by tests
          P6-4, P6-5)        (P6-3 Hyperopt is deferred — D7)
Phase 5  (P5-3, P5-4)     — e2e, extended to cover Backtest
Phase 4  (P4-4)           — bot state control, once introspection is proven
Phase 7  (P7-1 … P7-3)    — packaging and docs
```

Phase 4 (P4-1…P4-3) and Phase 6 are independent; run them in either order, or in parallel if two agents are working. **P4-4 is not**: it depends on `BotReachable` from P4-3 having real operating experience behind it, so it lands after e2e.

---

## Definition of done

- [ ] `make test` runs real tests; ≥70% overall, ≥80% on `configbuilder` and `resources`
- [ ] `make lint` clean with no `//nolint` added
- [ ] `make manifests generate && git diff --exit-code` clean, enforced in CI
- [ ] `grep -rn "panic(\|time.Sleep\|fmt.Print" controllers/` returns nothing
- [ ] No credential field can be set in plaintext on any CRD
- [ ] Every CRD has validation, defaults, printer columns, conditions, and `observedGeneration`
- [ ] Reconciling an unchanged TradeBot produces **zero** writes to the API server
- [ ] A config change either restarts the bot (`Auto`) or reports `ConfigDrift` (`Manual`) — never silently diverges
- [ ] Mode-switch and deletion paths never leave an orphaned trading workload running
- [ ] Bot workloads run under a `restricted` PodSecurity namespace
- [ ] `kubectl get backtests` shows strategy, timerange, trades, profit, and drawdown
- [ ] Backtest results survive the Job pod and are addressable after it exits
- [ ] `status.bot` reflects live bot state; `freqtrade_bot_*` metrics are scrapeable
- [ ] `spec.state: Stopped` survives a pod restart — the operator re-stops the bot unprompted
- [ ] No position-affecting endpoint is reachable from operator code (`forceexit`/`forcebuy`/`forceenter`)
- [ ] e2e deploys a working dry-run bot **and** runs a Backtest to completion in CI
- [ ] Operator survives: manager restart, API-server unavailability, unreachable bots, missing referenced CRs, malformed CRs
- [ ] `v1beta1` served and stored, `v1alpha1` converted, migration documented
- [ ] README install instructions work verbatim on a fresh cluster

---

## Deferred by decision

**No open design questions remain.** Everything in §0.3 is settled. These are things deliberately *not* being built, recorded so nobody re-opens them mid-implementation and so whoever picks them up later inherits the analysis rather than redoing it.

### Results PVC accumulation *(D10)*

Every `Backtest` owns a results PVC that outlives its Job. Nothing prunes them.

**Growth:** `total = runs × spec.results.size` (default `1Gi`). A team running ~10 backtests a day reaches ~300Gi in a month, plus one PV per run against the cluster's PV limit and the storage class's quota. The PV count usually bites before the capacity does.

**Why defer:** premature. The right policy depends on how the team actually uses backtesting — a handful of curated runs versus a sweep harness generating hundreds — and that is unknown until Phase 6 ships. Building a pruning controller now would encode a guess.

**What exists today as mitigation:** `RetentionPolicy: Delete` (the default) GCs the PVC when the `Backtest` CR is deleted, so `kubectl delete backtest --field-selector status.phase=Succeeded` is a working manual reaper.

**Options when it becomes a problem, roughly in order of cost:**
1. Document the manual reaper above and leave it. Often sufficient.
2. A `maxHistory` field on a future `BacktestSchedule`/sweep CRD — prune at the level that creates the runs, not globally.
3. A namespace-level `keep last N per strategy` reaper in the controller. Needs a tiebreak policy (newest? best Sharpe?) and an opt-out for runs a human marked as keepers.
4. Object-storage export + immediate PVC release. Best long-term, adds a dependency and credentials to manage.

**Trigger to revisit:** first report of PV-quota exhaustion, or a sweep workflow landing.

### `Hyperopt` CRD *(D7)*

Deferred to a minor after `v1beta1`. See P6-3 — the only thing that must be done now is the `RunSpec` inline embedding in P6-1.

### Bot write operations beyond start/stop *(D9)*

`spec.state` is in scope as **P4-4**. Position-affecting endpoints (`forceexit`, `forcebuy`, `forceenter`) are permanently out of scope for the operator, not merely deferred. If that is ever revisited it needs its own threat model, not a follow-on task.
