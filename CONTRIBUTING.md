# Contributing

## Building and running

```bash
make manifests generate  # regenerate CRDs/RBAC/webhooks and deepcopy code after any api/ change
make api-docs             # regenerate docs/api-reference.md after any api/ change too
make fmt vet
make run                 # run the manager against whatever cluster your kubeconfig points at
```

`make run` talks to a real cluster (CRDs must already be installed there via `make install`). There's no
requirement to run against a full cluster for most changes, though - see Testing below.

For manually poking at a running instance (not automated, not what CI runs) rather than reading test output,
[hack/kind-test-cluster.sh](hack/kind-test-cluster.sh) sets up a kind cluster wired to a local image registry
(`localhost:5001`, kind's own documented local-registry pattern) and [hack/lazy-test.sh](hack/lazy-test.sh) drives
it end to end - cluster, build, push, deploy, wait for the operator, apply `examples/`.

## Testing

```bash
make test-unit         # fast, no envtest - go test -short, every envtest suite skips itself
make test-integration  # just the envtest-backed controller suites (downloads a pinned kube-apiserver/etcd on first run)
make test              # both of the above - what CI runs
make test-e2e          # spins up a real kind cluster; only needed for changes to installation/e2e-level behavior
make coverage-gate     # after `make test`: checks coverage against hack/coverage-gate.sh's ratcheted floors
```

Two test styles coexist, by design, matching what each package needs:

- **Pure logic** (config rendering, status derivation, condition helpers) uses plain `go test` with table-driven
  tests - see [controllers/tradebot/poller_test.go](controllers/tradebot/poller_test.go) for the shape.
- **Anything that needs a real API server** (a controller's actual `Reconcile`, admission/conversion webhooks) uses
  [Ginkgo/Gomega](https://onsi.github.io/ginkgo/) against `envtest` - see `*_suite_test.go` and
  `*_controller_test.go` in each `controllers/<kind>/` package. These three packages
  (`controllers/tradebot`, `controllers/backtest`, `controllers/frequi`) are what `test-unit`'s `-short` skips and
  `test-integration` runs alone - if you add a fourth envtest-backed package, add it to both
  `test-integration`'s package list in the Makefile and the `testing.Short()` skip in its own `TestControllers`.

New reconciler or webhook behavior needs an envtest spec, not just a unit test against the pure-logic helpers it
calls - the failure modes worth catching here are usually in how those helpers get wired into a real `Reconcile`
loop (wrong owner refs, a missed `Update`, a status patch that doesn't actually apply), which a fake client or a
unit test around the helper alone won't exercise.

If you touch API conversion (`api/v1alpha1/*_conversion.go`), add or extend the round-trip fuzz test alongside it
(see [api/v1alpha1/tradebot_conversion_test.go](api/v1alpha1/tradebot_conversion_test.go)) rather than only
hand-writing example cases - the fuzz test is what actually catches a field silently dropped in one direction.

`hack/coverage-gate.sh`'s floors are a ratchet, not a target: they should only ever move up, tracking wherever
coverage actually is (with a little headroom), never held at a fixed aspirational number. Raise them in the same
PR that raises coverage - don't let them drift stale in the other direction either.

## Linting

```bash
make lint       # golangci-lint, config in .golangci.yml
make lint-fix   # same, with autofix
```

Run `make lint` twice before trusting the diff - some of the enabled linters (`unused`, `staticcheck`) can report a
different finding set between the first and second run against generated code, so a single run isn't a reliable
baseline by itself.

Don't add a `//nolint` to silence a finding; fix the underlying issue, or, if the finding is a genuine false
positive, adjust the rule in `.golangci.yml` with a comment explaining why - the project's own definition of done
requires `make lint` clean with no `//nolint` added.

## Before opening a PR

```bash
make fmt vet
make test
make coverage-gate
make lint            # run twice
make manifests generate && git diff --exit-code   # generated output must already be committed
make helm-sync       # if you touched anything under config/crd or config/rbac
make api-docs && git diff --exit-code docs/       # if you touched anything under api/
```

CI runs the same checks; running them locally first avoids a slow feedback loop through GitHub Actions for
something `make lint` would have told you in seconds.

A few conventions this codebase already follows, worth keeping:

- Comments explain *why*, not *what* - a comment restating what the next line obviously does gets removed in
  review. Save them for a non-obvious constraint, a workaround, or a decision that would otherwise look arbitrary.
- Prefer extending an existing package's patterns (e.g. `controllers/shared/` for anything more than one
  controller needs) over introducing a new abstraction for a single call site.
- Commit messages explain the *why* behind a change, not a restatement of the diff - see recent history
  (`git log --oneline`) for the expected tone and level of detail.

## Releasing

Tags matching `v*` (e.g. `v0.2.0`) trigger `release.yml`/`helm-release.yml` - see
[CHANGELOG.md](CHANGELOG.md)'s header for how this project reads SemVer for a CRD-shaped project. Before tagging:

1. Update [CHANGELOG.md](CHANGELOG.md): move `[Unreleased]` into a new `## [x.y.z] - YYYY-MM-DD` section.
   `make changelog-draft` prints a rough starting point grouped from commits since the last tag - it has no
   conventional-commit prefixes to key off, so treat its output as a first pass to edit, not a final result.
2. Optionally, `make bundle IMG=arksys/freqtrade-operator:vX.Y.Z VERSION=X.Y.Z` and commit the refreshed
   `bundle/`/`config/manifests/` - keeps the committed OLM bundle reasonably current for anyone browsing the repo
   or preparing an OperatorHub submission. Not required to actually release: `release.yml` regenerates and
   pushes a correctly-versioned bundle image itself on every tag push (see below), from source, independent of
   whatever's already committed.
3. Commit the changelog update (and bundle refresh, if you did one), then tag and push:
   `git tag vX.Y.Z && git push origin vX.Y.Z`.

Pushing the tag triggers `release.yml` (multi-arch image, signed and SBOM'd; `install.yaml` and a CRD tarball as
release assets; an OLM bundle image pushed to `arksys/freqtrade-operator-bundle`) and `helm-release.yml` (chart
packaged, pushed to `ghcr.io/ark-sys` as an OCI artifact, and published to GitHub Pages if enabled).

## Design decisions

Before proposing a change to how the CRDs or controllers are shaped, skim
[PRODUCTION-PLAN.md](PRODUCTION-PLAN.md)'s "Decisions" table - several things that might look like oversights
(no Job-mode `TradeBot`, no `spec.state` write path from the poller, same-namespace-only references) are
deliberate, already-settled design decisions with a stated rationale, not gaps waiting to be filled.
