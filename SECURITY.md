# Security Policy

## Reporting a vulnerability

Please report suspected security vulnerabilities privately, using
[GitHub's private vulnerability reporting](https://github.com/ark-sys/freqtrade-operator/security/advisories/new)
for this repository, rather than filing a public issue. Include the affected version, a description of the issue,
and, if possible, reproduction steps. We aim to acknowledge reports within a few days.

## Threat model

This operator runs automated cryptocurrency trading bots and holds the credentials that let them trade. Understand
this before deploying it against a real exchange account.

### What the operator can access

The manager runs under one ClusterRole scoped to the specific resource types it manages: this operator's own CRDs
(`tradebots`, `tradebotconfigs`, `strategies`, `frequis`, `backtests`), the workloads it builds
(`statefulsets`, `jobs`, `services`, `networkpolicies`), `secrets` and `configmaps` it reads and renders, `events`
it emits, and (for the [Backtest results sidecar](README.md#backtest-runs-v1beta1)) a narrowly-scoped
`serviceaccounts`/`roles`/`rolebindings` grant limited to `configmaps` in the sidecar's own namespace. There is no
`get`/`list` on arbitrary cluster resources, and no access to `pods/exec` or `pods/log` (deliberately - see
[Config Secret backups](README.md#config-secret-backups) for why `pods/log` in particular was ruled out for result
extraction).

The ClusterRole itself is cluster-scoped (it has to be, to watch every namespace), but every cross-resource
reference this operator follows - `TradeBot.spec.strategyRef`, `.configRef`, `FreqUI.spec.tradeBotRefs`, every
`secretRef` - is same-namespace-only by design (D3), not merely by convention. A `TradeBot` in namespace `a` cannot
be pointed at a `TradeBotConfig` or credential `Secret` in namespace `b`. Use one namespace per trading environment
(e.g. `trading-prod`, `trading-staging`) as the isolation boundary, not per-bot RBAC you'd have to maintain by hand.

### Where credentials live

Exchange API keys, Telegram bot tokens, and the freqtrade REST API's own Basic Auth password all follow the same
pattern: a `secretRef` pointing at a Kubernetes `Secret` the operator reads and renders into the bot's `config.json`,
itself stored in a Secret (labeled `freqtrade.io/contains-credentials: "true"` - see
[Config Secret backups](README.md#config-secret-backups) for what that means for your backup tooling). Plaintext
credential fields on `TradeBotConfig` still exist on the served (but deprecated) `v1alpha1` and are rejected by its
admission webhook unless the object carries `freqtrade.io/allow-plaintext-credentials: "true"` - but that
annotation only bypasses `v1alpha1`'s own admission check. `v1beta1`, the storage version, has no field a plaintext
credential could occupy at all: any write that round-trips through storage - including one admitted past
`v1alpha1`'s webhook via the annotation - is rejected at conversion instead. There is no way to persist a plaintext
credential, opt-out annotation or not (see the README's [Upgrading to v1beta1](README.md#upgrading-to-v1beta1) for
migrating any that predate this).

Anyone who can `get` a credential `Secret` in a trading namespace has the same access the bot itself has: the
exchange account, at whatever permissions that API key was scoped to on the exchange side (**scope it to trading
only, never withdrawal**, regardless of what this operator does or doesn't enforce - that's an exchange-side
control this software cannot substitute for). Anyone who can only `get tradebotconfig` (without `get secret`) sees
the exchange name, risk/pairlist parameters, and whether `dry_run` is set - never the credentials themselves, since
(per above) a `TradeBotConfig` cannot hold a plaintext one at all.

### What P4-3 introspection changes

The operator's own manager process polls each bot's freqtrade REST API directly (leader replica only), using the
same Basic Auth credentials rendered into that bot's config. This means the operator process itself now holds and
actively uses those credentials to make outbound HTTP calls, not just Kubernetes to store them - a pod compromise
of the manager already implied Secret read access across every namespace it watches, so this isn't a new grant, but
it is a new *behavior*: the manager now originates authenticated network traffic to every bot it introspects.
Introspection is strictly read-only (D9) - `ping`, `version`, `show_config`, `count`, `profit`, `balance` - never
anything that can start, stop, or otherwise act on a bot. Position-affecting endpoints (`forceexit`, `forcebuy`,
`forceenter`) are permanently out of scope for this operator, not merely deferred; revisiting that would need its
own, separate threat model.

### Supply chain

Every released image is signed with [cosign](https://docs.sigstore.dev/) (keyless, via this repository's own GitHub
Actions OIDC identity) and ships an SPDX SBOM as a release asset - see the [Installation](README.md#installation)
section for the verification command. Dependencies are kept current via Dependabot (Go modules, GitHub Actions,
and the base image).
