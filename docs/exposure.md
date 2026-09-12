# Exposing FreqUI

For installation, see [installation.md](installation.md). For every field on every CRD, see
[api-reference.md](api-reference.md).

`FreqUI.spec.exposure` selects how a `FreqUI` is reachable from outside the cluster. It has three values:

| `spec.exposure` | What gets created | Needs |
|---|---|---|
| `Ingress` (default) | One `networking.k8s.io/v1` `Ingress`, with a rule per hostname (see below) | An Ingress controller, DNS, and (for TLS) a cert-manager `ClusterIssuer` |
| `Gateway` | One `gateway.networking.k8s.io/v1` `HTTPRoute` per hostname (see below) | The Gateway API CRDs installed, and a `Gateway` your cluster owner already created |
| `None` | Neither - `Deployment` + `Service` only | Nothing; reach it with `kubectl port-forward` |

`Ingress` is the default because it's what every release before this one did unconditionally -
existing `FreqUI` objects with no `spec.exposure` set behave exactly as before. `None` is the
zero-infrastructure way to try FreqUI on any cluster:

```bash
kubectl port-forward -n <namespace> svc/<frequi-name> 8080:80
```

## Why Gateway mode creates more than one object

An `Ingress` can carry many hostnames in one object because each rule has its own `host`.
`HTTPRoute` can't: `HTTPRouteSpec.Hostnames` is a single list that applies to *every* rule in the
route - there's no per-rule host field. So where Ingress mode is one object with N rules (one for
the UI, one per `TradeBotRef` with an enabled API server, each on its own `<bot>.<host>`
subdomain), Gateway mode is N+1 separate `HTTPRoute` objects, one per hostname - the UI's own
route, plus one per bot subdomain. Both modes end up with the same hostname topology; only the
object shape differs.

## What this operator does and doesn't create

This operator creates `HTTPRoute`s only. It **never creates a `Gateway` or `GatewayClass`** - a
`Gateway` is infrastructure your cluster owner provides, the same way an `IngressClass` is. You
need one already running, with a listener that accepts routes from your `FreqUI`'s namespace,
before setting `spec.exposure: Gateway`.

A minimal `Gateway` a cluster owner might set up, matching
[`examples/live-trading/frequi.yaml`](../examples/live-trading/frequi.yaml)'s commented-out block:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: my-gateway
  namespace: gateway-infra
spec:
  gatewayClassName: <your Gateway controller's GatewayClass, e.g. envoy-gateway or istio>
  listeners:
    - name: https
      protocol: HTTPS
      port: 443
      hostname: "*.example.com"   # must cover the UI host AND every <bot>.<host> subdomain
      tls:
        certificateRefs:
          - name: example-com-tls
      allowedRoutes:
        namespaces:
          from: Selector
          selector:
            matchLabels:
              kubernetes.io/metadata.name: freqtrade-example   # or: from: All
```

Point `FreqUI.spec.gateway.parentRefs` at it:

```yaml
spec:
  exposure: Gateway
  gateway:
    parentRefs:
      - name: my-gateway
        namespace: gateway-infra
```

`spec.tls` and `spec.ingressAnnotations` do nothing in Gateway mode - TLS terminates at the
`Gateway`'s own listener, which this operator doesn't manage; setting `spec.tls` alongside
`exposure: Gateway` is rejected outright by a CEL validation rule at apply time, so this fails
loudly rather than silently doing nothing.

## Detection happens once, at operator startup

Whether the cluster serves Gateway API is checked once when the operator starts, not on a timer.
**Installing Gateway API after the operator is already running needs an operator restart** for
it to be noticed. Until then, a `Gateway`-mode `FreqUI` reports why in its own condition (next
section) rather than silently doing nothing.

## Reading `status.conditions[ExposureReady]`

```bash
kubectl get frequi <name> -o jsonpath='{.status.conditions[?(@.type=="ExposureReady")]}'
```

| Status / Reason | Meaning |
|---|---|
| `True` / `ExposureNone` | `spec.exposure: None` - nothing was meant to be created. |
| `True` / `AsExpected` | The `Ingress` applied cleanly, or every generated `HTTPRoute` was accepted by its `Gateway`. |
| `False` / `GatewayAPINotInstalled` | `spec.exposure: Gateway`, but this cluster doesn't serve `gateway.networking.k8s.io/v1` `HTTPRoute`. Install it, then **restart the operator** - see above. |
| `False` / `RoutePending` | Every `HTTPRoute` applied, but at least one has no `status.parents` yet - no Gateway controller has claimed it. Not an error; check that your `Gateway`'s `allowedRoutes` actually covers this namespace. |
| `False` / `RouteNotAccepted` | At least one `HTTPRoute`'s `status.parents` explicitly rejected it (`Accepted=False` or `ResolvedRefs=False`) - the message names the route and the first failing reason. |
| `False` / `RouteNameConflict` | A generated `HTTPRoute` or `Ingress` name collided with an object owned by a different `FreqUI`. Rename one of the two `FreqUI`s. |

`status.phase` reflects this too: a `FreqUI` whose `Deployment` is healthy but whose
`ExposureReady` is `False` reports `Degraded`, not `Running` - it's up, but nothing is routing
traffic to it.

## The NetworkPolicy caveat

Each trade-mode `TradeBot`'s `NetworkPolicy` allows exactly two kinds of peer on its API port:
the operator's own namespace, and same-namespace `FreqUI` pods. **Nothing allows an
ingress-controller or Gateway data-plane pod**, which typically run in their own namespace
(`envoy-gateway-system`, `istio-system`, `ingress-nginx`, ...). This means the per-bot API
subdomains this operator generates - `Ingress` rules today, `HTTPRoute`s in Gateway mode - are
**not actually reachable from outside the cluster** on any cluster with a `NetworkPolicy`-enforcing
CNI. This is a pre-existing limitation, not something Gateway mode introduces, and it isn't fixed
yet: widening a live trading API's network reachability is a decision for a cluster operator to
make explicitly, not something this operator does on your behalf by default.

If you need the per-bot API subdomains reachable, add your own `NetworkPolicy` allowing your
ingress controller's or Gateway's data-plane namespace/pods to reach the affected `TradeBot`'s API
port. The UI itself (the non-bot-subdomain hostname) is unaffected - it only ever needs to reach
the `FreqUI` Deployment's own port 80, which has no such restriction.
