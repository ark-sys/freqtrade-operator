# Installation

For what this operator is and why you'd run it, see the main [README](../README.md). For every
field on every CRD, see [api-reference.md](api-reference.md).

## Prerequisites

- Kubernetes cluster 1.29+ (native sidecar containers, GA since 1.29, are required for
  [Backtest result extraction](backtesting.md#backtest-runs-v1beta1))
- kubectl 1.29+
- Helm 3+ (optional, for Helm chart installation)
- Go 1.24+ (for building from source)

## Using Helm

The Helm chart is published to GHCR as an OCI artifact on every release (`.github/workflows/helm-release.yml`) -
this is the preferred installation method:

```bash
helm install freqtrade-operator oci://ghcr.io/ark-sys/freqtrade-operator --version <chart-version>
```

## Using pre-built manifests

```bash
kubectl apply --server-side -f https://github.com/ark-sys/freqtrade-operator/releases/latest/download/install.yaml
```

`--server-side` is required, not optional: a plain `kubectl apply` embeds the whole applied object into a
`kubectl.kubernetes.io/last-applied-configuration` annotation, capped at 256KiB - the `TradeBot` CRD alone (two
full served API versions since `v1beta1`) already exceeds that, and `kubectl apply` without `--server-side` fails
outright on it with "metadata.annotations: Too long". Server-side apply tracks field ownership instead of
embedding the whole object, so it isn't affected by this at all - the same fix
[helm/ARGOCD-CONFIGURATION.md](../helm/ARGOCD-CONFIGURATION.md) already documents for the Helm chart's ArgoCD path.

`install.yaml` is a one-file install - CRDs, RBAC, webhooks, and the operator Deployment together, built by
`make build-installer` against the exact image the same release job builds, signs, and pushes. The CRDs alone are
also published separately as `freqtrade-operator-crds.tar.gz`, for anyone managing RBAC/Deployment themselves (e.g.
via the Helm chart above) but still wanting the plain CRD YAMLs - the same `--server-side` requirement applies to
applying those directly too.

Every image the release workflow pushes is signed with [cosign](https://docs.sigstore.dev/) (keyless, via GitHub Actions'
own OIDC identity - no key to fetch or trust out of band) and ships an SPDX SBOM as a release asset. Verify an image with:

```bash
cosign verify ghcr.io/ark-sys/freqtrade-operator:<tag> \
  --certificate-identity-regexp 'https://github.com/ark-sys/freqtrade-operator/.github/workflows/release.yml@.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

## Using OLM

Every tagged release also publishes an [OLM](https://olm.operatorframework.io/) bundle image
(`ghcr.io/ark-sys/freqtrade-operator-bundle`), installable via `operator-sdk run bundle` against a cluster that already has
OLM installed, or addable to your own catalog via `make catalog-build` (see `make help`).

## Building from source

1. Clone the repository:
```bash
git clone https://github.com/ark-sys/freqtrade-operator.git
cd freqtrade-operator
```

2. Build and deploy the operator:
```bash
# Build the operator image
make docker-build IMG=your-registry/freqtrade-operator:latest

# Push the image to your registry
make docker-push IMG=your-registry/freqtrade-operator:latest

# Deploy the operator to your cluster
make deploy IMG=your-registry/freqtrade-operator:latest
```
