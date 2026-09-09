package backtest

import (
	"context"
	"fmt"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	freqtradev1beta1 "github.com/ark-sys/freqtrade-operator/api/v1beta1"
	"github.com/ark-sys/freqtrade-operator/controllers/backtest/resources"
	"github.com/ark-sys/freqtrade-operator/controllers/shared"
	"github.com/ark-sys/freqtrade-operator/controllers/tradebotconfig/configbuilder"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// fetchReferencedObjects resolves spec.strategyRef/spec.configRef, same
// namespace only (D3) - both still v1alpha1 types; only Backtest itself is
// new in v1beta1 (P6-1), Strategy and TradeBotConfig haven't moved yet.
func (r *Reconciler) fetchReferencedObjects(
	ctx context.Context, backtest *freqtradev1beta1.Backtest,
) (*freqtradev1alpha1.Strategy, *freqtradev1alpha1.TradeBotConfig, error) {
	var strategy freqtradev1alpha1.Strategy
	strategyKey := types.NamespacedName{Name: backtest.Spec.StrategyRef.Name, Namespace: backtest.Namespace}
	if err := r.Get(ctx, strategyKey, &strategy); err != nil {
		return nil, nil, fmt.Errorf("failed to get Strategy %s: %w", backtest.Spec.StrategyRef.Name, err)
	}

	var tradeBotConfig freqtradev1alpha1.TradeBotConfig
	configKey := types.NamespacedName{Name: backtest.Spec.ConfigRef.Name, Namespace: backtest.Namespace}
	if err := r.Get(ctx, configKey, &tradeBotConfig); err != nil {
		return nil, nil, fmt.Errorf("failed to get TradeBotConfig %s: %w", backtest.Spec.ConfigRef.Name, err)
	}

	return &strategy, &tradeBotConfig, nil
}

func (r *Reconciler) defaultImage() string {
	if r.DefaultImage != "" {
		return r.DefaultImage
	}
	return shared.DefaultFreqtradeImage
}

// reconcileResources creates every resource a Backtest run needs, exactly
// once. Unlike TradeBot's continuous apply-every-reconcile pattern, a
// Backtest's spec never changes after creation (CEL, see
// api/v1beta1/backtest_types.go) - so once its Job exists, there's nothing
// left to reconcile here ever again, and this returns immediately without
// re-rendering config or re-fetching referenced objects. That matters
// beyond just avoiding wasted work: a credentials Secret rotated or deleted
// long after a run finished must never turn a long-done Backtest's routine
// reconcile (triggered by, say, a manager restart's resync) into a
// ReconcileError - the run already happened, and there is nothing further
// to apply.
func (r *Reconciler) reconcileResources(
	ctx context.Context, backtest *freqtradev1beta1.Backtest,
) (jobName string, err error) {
	logger := log.FromContext(ctx)
	jobName = backtest.Name

	var existing batchv1.Job
	getErr := r.Get(ctx, types.NamespacedName{Name: backtest.Name, Namespace: backtest.Namespace}, &existing)
	switch {
	case getErr == nil:
		return jobName, nil
	case !errors.IsNotFound(getErr):
		return "", fmt.Errorf("failed to check for an existing Job: %w", getErr)
	}

	strategy, tradeBotConfig, err := r.fetchReferencedObjects(ctx, backtest)
	if err != nil {
		return "", err
	}

	configData, err := configbuilder.BuildConfig(ctx, r.Client, backtest.Name, backtest.Namespace, tradeBotConfig, nil)
	if err != nil {
		return "", fmt.Errorf("failed to build config: %w", err)
	}

	configSecret := resources.BuildConfigSecret(*backtest, configData)
	if err := shared.Apply(ctx, r.Client, backtest, &configSecret); err != nil {
		return "", fmt.Errorf("failed to apply config Secret: %w", err)
	}

	strategyConfigMap := resources.BuildStrategyConfigMap(*backtest, strategy.Spec.Name, strategy.Spec.Script)
	if err := shared.Apply(ctx, r.Client, backtest, &strategyConfigMap); err != nil {
		return "", fmt.Errorf("failed to apply strategy ConfigMap: %w", err)
	}

	resultsPVC := resources.BuildResultsPVC(*backtest)
	if err := shared.Apply(ctx, r.Client, backtest, &resultsPVC); err != nil {
		return "", fmt.Errorf("failed to apply results PVC: %w", err)
	}
	logger.V(1).Info("Results PVC applied", "name", resultsPVC.Name)

	if err := r.ensureSidecarRBAC(ctx, backtest.Namespace); err != nil {
		return "", fmt.Errorf("failed to provision the results-collection sidecar's RBAC: %w", err)
	}

	job := resources.BuildJob(
		*backtest, r.defaultImage(), r.OperatorImage, strategy.Spec.Name, configSecret.Name, strategyConfigMap.Name,
	)
	if err := shared.Apply(ctx, r.Client, backtest, &job); err != nil {
		return "", fmt.Errorf("failed to apply Job: %w", err)
	}
	logger.V(1).Info("Job applied", "name", job.Name)

	return jobName, nil
}

// ensureSidecarRBAC applies the P6-2 results-collection sidecar's
// namespace-wide ServiceAccount/Role/RoleBinding - shared by every
// Backtest's Job in namespace, so applied unowned (shared.ApplyUnowned,
// not shared.Apply) rather than owned by whichever Backtest happens to
// trigger the first reconcile after the namespace gets a new one: owning
// it by a single Backtest would cascade-delete it out from under every
// other Backtest still using it the moment that one is deleted. Safe to
// call on every Backtest's first reconcile in the namespace - applying an
// already-current object is a no-op (server-side apply, P2-1).
func (r *Reconciler) ensureSidecarRBAC(ctx context.Context, namespace string) error {
	sa := resources.BuildSidecarServiceAccount(namespace)
	if err := shared.ApplyUnowned(ctx, r.Client, &sa); err != nil {
		return fmt.Errorf("failed to apply sidecar ServiceAccount: %w", err)
	}
	role := resources.BuildSidecarRole(namespace)
	if err := shared.ApplyUnowned(ctx, r.Client, &role); err != nil {
		return fmt.Errorf("failed to apply sidecar Role: %w", err)
	}
	roleBinding := resources.BuildSidecarRoleBinding(namespace)
	if err := shared.ApplyUnowned(ctx, r.Client, &roleBinding); err != nil {
		return fmt.Errorf("failed to apply sidecar RoleBinding: %w", err)
	}
	return nil
}
