# Backtest runs (v1beta1)

For installation, see [installation.md](installation.md). For every field on every CRD, see
[api-reference.md](api-reference.md).

`Backtest` is a dedicated CRD for one-shot backtesting runs, introduced alongside `TradeBot` rather than overloading
it: a live bot is a mutable singleton you edit in place, a backtest is an immutable fact about a
`(strategy, config, timerange, data)` tuple you want many of, with history - forcing both through one CRD is why the
old `TradeBot` Job mode needed a spec-hash suffixed onto the Job name just to avoid collisions. A `Backtest`'s name
*is* its run identity, and `kubectl get backtests` gets real printer columns for phase, trades, and profit.

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: sample-strategy-cache
spec:
  accessModes: ["ReadWriteOnce"]
  resources:
    requests:
      storage: 1Gi
---
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
  data:
    pvcName: sample-strategy-cache   # must already exist - see note below
    downloadPolicy: always           # default; downloads every run, see the other policies below
  results:
    size: 2Gi
    retentionPolicy: Delete   # default; Retain keeps the results PVC (owner ref stripped) after this Backtest is deleted
```

**Spec is immutable after creation** (enforced by the API server itself, not just convention) - every field is fixed
the moment the `Backtest` is admitted, so its Job's pod template never needs to change and can never drift from what
actually ran. To change a parameter, create a new `Backtest`; nothing here is a place to iterate in-place.

**`spec.data.pvcName` is how a `Backtest` gets historical OHLCV data to run against, and the operator never creates
this PVC for you** - omit `spec.data` entirely and no cache volume is mounted at all, so freqtrade's own market-data
load finds nothing on disk and the run fails with `No data found. Terminating.` regardless of how the rest of the
spec is configured. Create the PVC yourself first (as in the example above), then reference it by name. Once
referenced, an init container runs `freqtrade download-data` into it before the backtest itself starts, honoring
the same `timerange`/`timeframe`/`pairs` the run itself uses - `spec.data.downloadPolicy` controls when this
actually happens: `always` (default, re-downloads every run - safest, but redownloads data every time even if nothing
changed), `ifMissing` (skip downloading if the PVC already has *anything* on it, regardless of whether it actually
covers this run's timerange), or `never` (use whatever's already there, no download attempt at all - useful once
you've primed a shared cache and want every subsequent run to be fast). `spec.data.downloadArgs` appends extra raw
`download-data` flags (e.g. `["--days", "30"]`) for anything the typed fields above don't cover.

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
build and release) copies the run's own result file (freqtrade writes this as a `backtest-result-<ts>.zip`
containing the actual JSON alongside a config echo, the strategy source, and a couple of `.feather` files) onto
`spec.results`' PVC - durable beyond the Job pod's own lifetime, unlike the emptyDir it's read from - and writes a
summary ConfigMap (`<name>-results`) the operator reads back into `status.results` and the `ResultsAvailable`
condition. A summary that can't be extracted (a malformed or missing result file) reports
`ResultsAvailable=False`/`ResultsUnavailable` rather than failing the `Backtest` - the run happened either way,
and the raw file itself still gets copied onto the results PVC regardless, addressable by mounting `status.resultsPVCName`
from another pod (or `kubectl cp` while the run's own pod still exists).

## Running many backtests and comparing results

Because a `Backtest`'s spec is immutable and its name is its identity, sweeping a parameter is just applying one
`Backtest` per value you want to try, then reading their `status.results` back - there's no in-place "edit and
rerun" step to serialize on:

A sweep's runs benefit from sharing one cache PVC rather than each downloading its own copy - but since their Jobs
run in parallel, potentially on different nodes, that shared PVC needs a `ReadWriteMany`-capable storage class (e.g.
NFS, CephFS, EFS/Azure Files - most default cloud block-storage classes are `ReadWriteOnce` only and won't work
here). If you don't have one available, give each `Backtest` its own `ReadWriteOnce` cache PVC instead (as in the
single-run example above) and accept the redundant downloads.

`downloadPolicy` is left at its `always` default deliberately below, not set to `ifMissing`: each run here wants a
*different* `timeframe`, but `ifMissing` only checks whether the cache directory is empty at all, not whether it
already has any one run's specific timeframe - with three parallel runs racing for the same cache, `ifMissing` would
let whichever pod starts first "win" and leave the other two timeframes never downloaded at all.

```bash
kubectl apply -f - <<EOF
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: sample-strategy-sweep-cache
spec:
  accessModes: ["ReadWriteMany"]
  storageClassName: efs-sc   # any ReadWriteMany-capable class
  resources:
    requests:
      storage: 1Gi
EOF

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
  data:
    pvcName: sample-strategy-sweep-cache
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
there is currently no cluster-wide pruning of old `Backtest` objects or their PVCs - so clean up runs you no
longer need with `kubectl delete backtest -l ...` once you've captured what you wanted from `status.results`, rather
than leaving a sweep's full history to grow unbounded.

`Hyperopt` (parameter optimization, rather than a single fixed-parameter run) is not implemented yet - `Backtest`
ships alone in the first `v1beta1` release by design, with `Hyperopt` following as its own CRD in a later minor
once `Backtest` has real operating experience behind it.
