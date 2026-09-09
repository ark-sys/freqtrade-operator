# Freqtrade Operator Helm Chart

This Helm chart deploys the Freqtrade Operator and its Custom Resource Definitions (CRDs) to a Kubernetes cluster.

## Prerequisites

- Kubernetes 1.29+ (native sidecar containers, required for Backtest result extraction)
- Helm 3.8+
- (Optional) ArgoCD for GitOps deployments

## Installation

### Using Helm Repository

```bash
# Add the Freqtrade Operator Helm repository
helm repo add freqtrade-operator https://ark-sys.github.io/freqtrade-operator/
helm repo update

# Install the chart
helm install my-freqtrade-operator freqtrade-operator/freqtrade-operator \
  --namespace freqtrade-operator-system \
  --create-namespace
```

### Using OCI Registry

```bash
# Install directly from OCI registry
helm install my-freqtrade-operator oci://ghcr.io/ark-sys/freqtrade-operator \
  --version 0.1.0 \
  --namespace freqtrade-operator-system \
  --create-namespace
```

### Using Local Chart

```bash
# Clone the repository
git clone https://github.com/ark-sys/freqtrade-operator.git
cd freqtrade-operator

# Install the chart
helm install my-freqtrade-operator ./helm/ \
  --namespace freqtrade-operator-system \
  --create-namespace
```

## Configuration

### Basic Configuration

The following table lists the configurable parameters of the chart and their default values.

| Parameter | Description | Default |
|-----------|-------------|---------|
| `controllerManager.replicas` | Number of controller manager replicas | `1` |
| `controllerManager.image.repository` | Controller manager image repository | `arksys/freqtrade-operator` |
| `controllerManager.image.tag` | Controller manager image tag | `""` (uses appVersion) |
| `controllerManager.image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `controllerManager.logLevel` | Log level (debug, info, warn, error) | `info` |
| `controllerManager.resources.limits.cpu` | CPU limit | `500m` |
| `controllerManager.resources.limits.memory` | Memory limit | `512Mi` |
| `controllerManager.resources.requests.cpu` | CPU request | `100m` |
| `controllerManager.resources.requests.memory` | Memory request | `64Mi` |
| `controllerManager.metrics.enabled` | Enable metrics endpoint | `true` |
| `serviceAccount.create` | Create service account | `true` |
| `serviceAccount.name` | Service account name | `""` (generated) |
| `rbac.create` | Create RBAC resources | `true` |
| `nodeSelector` | Node selector for pod assignment | `{}` |
| `tolerations` | Tolerations for pod assignment | `[]` |
| `affinity` | Affinity for pod assignment | `{}` |

### Example Values

#### Development Configuration

```yaml
# values-dev.yaml
controllerManager:
  logLevel: debug
  resources:
    limits:
      cpu: 200m
      memory: 256Mi
    requests:
      cpu: 50m
      memory: 32Mi
  metrics:
    enabled: true
```

#### Production Configuration

```yaml
# values-prod.yaml
controllerManager:
  replicas: 2
  logLevel: info
  resources:
    limits:
      cpu: 1000m
      memory: 1Gi
    requests:
      cpu: 200m
      memory: 128Mi
  metrics:
    enabled: true
  extraArgs:
    - --leader-elect-lease-duration=15s
    - --leader-elect-renew-deadline=10s

podSecurityContext:
  runAsNonRoot: true
  runAsUser: 65534
  runAsGroup: 65534
  fsGroup: 65534

securityContext:
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: true
  runAsNonRoot: true
  capabilities:
    drop:
    - "ALL"

affinity:
  nodeAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
      nodeSelectorTerms:
        - matchExpressions:
          - key: kubernetes.io/arch
            operator: In
            values:
              - amd64
              - arm64
          - key: kubernetes.io/os
            operator: In
            values:
              - linux
```

## ArgoCD Deployment

The chart is designed to work seamlessly with ArgoCD for GitOps deployments.

### Basic ArgoCD Application

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: freqtrade-operator
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://ark-sys.github.io/freqtrade-operator/
    chart: freqtrade-operator
    targetRevision: "0.1.0"
    helm:
      values: |
        controllerManager:
          image:
            tag: "latest"
          logLevel: info
  destination:
    server: https://kubernetes.default.svc
    namespace: freqtrade-operator-system
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
      - CreateNamespace=true
      - ServerSideApply=true
      - Replace=true
```

**Important for ArgoCD Users:** 
- The `ServerSideApply=true` and `Replace=true` sync options are required for proper CRD installation
- CRDs will be installed in sync wave -1, RBAC resources in wave 0, and the operator deployment in wave 1
- This ensures proper installation order and prevents CRD-related sync failures

For more ArgoCD examples, see the [examples directory](./examples/).

For detailed ArgoCD configuration guidance, see [ARGOCD-CONFIGURATION.md](./ARGOCD-CONFIGURATION.md).

## Custom Resource Definitions

This chart automatically installs the following CRDs:

- **TradeBots** (`tradebots.freqtrade.io`) - Main trading bot configurations
- **TradeBotConfigs** (`tradebotconfigs.freqtrade.io`) - Configurations for individual trade bots
- **Strategies** (`strategies.freqtrade.io`) - Trading strategy configurations
- **FreqUIs** (`frequis.freqtrade.io`) - FreqUI dashboard configurations

## Monitoring

When metrics are enabled (`controllerManager.metrics.enabled: true`), the controller manager exposes Prometheus metrics on port 8080. You can scrape these metrics using Prometheus or access them via port-forwarding:

```bash
kubectl port-forward -n freqtrade-operator-system deployment/my-freqtrade-operator-controller-manager 8080:8080
curl http://localhost:8080/metrics
```

## Troubleshooting

### Check Controller Manager Status

```bash
kubectl get deployment -n freqtrade-operator-system
kubectl describe deployment my-freqtrade-operator-controller-manager -n freqtrade-operator-system
```

### View Logs

```bash
kubectl logs -f deployment/my-freqtrade-operator-controller-manager -n freqtrade-operator-system
```

### Check CRDs

```bash
kubectl get crd | grep freqtrade.io
```

### Verify RBAC

```bash
kubectl get clusterrole | grep freqtrade-operator
kubectl get clusterrolebinding | grep freqtrade-operator
```

### ArgoCD-Specific Issues

If you're experiencing sync failures with ArgoCD:

1. **CRD Installation Issues**: Ensure your Application uses `ServerSideApply=true` and `Replace=true` sync options
2. **Resource Too Large**: Some CRDs are >600KB; server-side apply handles this better than client-side apply
3. **Sync Wave Order**: Check that resources are installing in the correct order (-1 for CRDs, 0 for RBAC, 1 for apps)
4. **Permission Errors**: Verify your ArgoCD service account has sufficient cluster permissions

```bash
# Check ArgoCD application status
argocd app get freqtrade-operator

# Force refresh if using cached manifests
argocd app hard-refresh freqtrade-operator

# Check sync waves
kubectl get crds -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.metadata.annotations.argocd\.argoproj\.io/sync-wave}{"\n"}{end}' | grep freqtrade
```

## Upgrading

### Using Helm

```bash
helm repo update
helm upgrade my-freqtrade-operator freqtrade-operator/freqtrade-operator \
  --namespace freqtrade-operator-system
```

### Using ArgoCD

ArgoCD will automatically detect and apply updates based on your sync policy configuration.

## Uninstallation

```bash
helm uninstall my-freqtrade-operator --namespace freqtrade-operator-system
```

**Note:** CRDs are not automatically removed by Helm. To remove them:

```bash
kubectl delete crd -l app.kubernetes.io/name=freqtrade-operator
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes to the Helm chart
4. Test your changes using the provided test workflows
5. Submit a pull request

## License

This chart is licensed under the Apache License 2.0. See the [LICENSE](../LICENSE) file for details.

## Support

- GitHub Issues: [ark-sys/freqtrade-operator](https://github.com/ark-sys/freqtrade-operator/issues)
- Documentation: [Freqtrade Operator Docs](https://github.com/ark-sys/freqtrade-operator)
- Community: [Freqtrade Discord](https://discord.gg/freqtrade) 