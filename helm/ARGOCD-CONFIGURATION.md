# ArgoCD Configuration Guide

This document provides specific guidance for deploying the Freqtrade Operator using ArgoCD.

## Required Sync Options

When deploying with ArgoCD, you **must** include the following sync options:

```yaml
syncOptions:
  - CreateNamespace=true
  - ServerSideApply=true  # Required for large CRDs
  - Replace=true          # Required for CRD updates
```

## Why These Options Are Required

### ServerSideApply=true
- **Problem**: Some Freqtrade CRDs are very large (>600KB), exceeding kubectl's annotation size limits
- **Solution**: Server-side apply handles large resources without annotation size constraints
- **Symptoms without this**: `entity too large` or `annotation too long` errors

### Replace=true
- **Problem**: CRD updates can fail with strategic merge patch conflicts
- **Solution**: Replace strategy ensures clean CRD updates
- **Symptoms without this**: `failed to apply patch` or `resource mapping` errors

## Sync Wave Configuration

The chart includes sync wave annotations to ensure proper installation order:

1. **Wave -1**: Custom Resource Definitions (CRDs)
2. **Wave 0**: RBAC resources (ClusterRole, ServiceAccount, etc.)
3. **Wave 1**: Application resources (Deployment, Service)

This prevents the common issue where the operator starts before CRDs are fully installed.

## Complete Example

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
    retry:
      limit: 5
      backoff:
        duration: 5s
        factor: 2
        maxDuration: 3m
```

## Troubleshooting

### Sync Failures
If you experience sync failures:

1. Check the ArgoCD application events:
   ```bash
   argocd app get freqtrade-operator
   ```

2. Force a hard refresh to bypass cache:
   ```bash
   argocd app hard-refresh freqtrade-operator
   ```

3. Verify sync wave order:
   ```bash
   kubectl get crds -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.metadata.annotations.argocd\.argoproj\.io/sync-wave}{"\n"}{end}' | grep freqtrade
   ```

### Common Error Messages

- **"entity too large"**: Add `ServerSideApply=true`
- **"annotation too long"**: Add `ServerSideApply=true`
- **"resource mapping not found"**: CRDs not installed yet; sync waves should fix this
- **"failed to apply patch"**: Add `Replace=true`

### Manual Sync

If automated sync fails, you can manually sync with options:

```bash
argocd app sync freqtrade-operator --server-side-apply --replace
```

## Best Practices

1. **Always use both sync options** (`ServerSideApply=true` and `Replace=true`)
2. **Enable automated sync** for continuous deployment
3. **Set retry policies** to handle transient failures
4. **Monitor sync waves** to ensure proper ordering
5. **Test in non-production** environments first

## Upgrading

When upgrading the chart version:

1. ArgoCD will automatically detect the change
2. Sync waves ensure CRDs are updated first
3. The operator deployment will be updated last
4. No manual intervention should be required

## Support

If you encounter issues specific to ArgoCD deployment, please include:

1. ArgoCD version
2. Kubernetes version
3. Complete Application manifest
4. ArgoCD application status output
5. Any error messages from the ArgoCD UI or CLI