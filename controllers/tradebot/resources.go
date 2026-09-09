package tradebot

import (
	"context"
	"fmt"
	"strings"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	"github.com/ark-sys/freqtrade-operator/controllers/shared"
	"github.com/ark-sys/freqtrade-operator/controllers/tradebot/resources"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// referencedResources holds all the resources referenced by a TradeBot
type referencedResources struct {
	strategy       *freqtradev1alpha1.Strategy
	tradebotconfig *freqtradev1alpha1.TradeBotConfig
}

// fetchReferencedResources retrieves all resources referenced by the TradeBot
func (r *Reconciler) fetchReferencedResources(
	ctx context.Context,
	tradeBot *freqtradev1alpha1.TradeBot,
	namespace string) (*referencedResources, error) {

	logger := log.FromContext(ctx)
	result := &referencedResources{}

	// Fetch TradeBotConfig
	tradeBotConfig := &freqtradev1alpha1.TradeBotConfig{}
	if err := r.Get(ctx, types.NamespacedName{Name: tradeBot.Spec.Config, Namespace: namespace}, tradeBotConfig); err != nil {
		logger.Error(err, "Failed to fetch TradeBotConfig")
		return nil, err
	}
	result.tradebotconfig = tradeBotConfig

	// Fetch Strategy
	strategy := &freqtradev1alpha1.Strategy{}
	if err := r.Get(ctx, types.NamespacedName{Name: tradeBot.Spec.Strategy, Namespace: namespace}, strategy); err != nil {
		logger.Error(err, "Failed to fetch Strategy")
		return nil, err
	}
	result.strategy = strategy

	return result, nil
}

// reconcileResources creates or updates all resources needed by the TradeBot.
// strategy must be the already-resolved Strategy referenced by tradeBot.Spec.Strategy
// (fetchReferencedResources fetches it once per reconcile; this stops a second,
// redundant fetch here and lets Build{StatefulSet,Job} stay pure functions).
//
// The returned bool is jobSpecChanged - whether a Job-mode TradeBot's spec
// no longer matches the Job actually running (see the Job branch below). It
// is only meaningful when err is nil: Reconcile folds it into the same
// PatchStatus call that sets every other condition, rather than this
// function writing WorkloadImmutable itself - a second, independent
// PatchStatus call earlier in the same reconcile previously raced the
// final one (its own re-fetch could read a cache that hadn't yet observed
// the earlier call's write, silently reverting it).
func (r *Reconciler) reconcileResources(
	ctx context.Context,
	tradeBot *freqtradev1alpha1.TradeBot,
	strategy *freqtradev1alpha1.Strategy,
	configData map[string]string) (bool, error) {

	logger := log.FromContext(ctx)
	logger.V(2).Info("Reconciling resources for TradeBot", "name", tradeBot.Name)

	// 1. Create a config.json Secret with the TradeBotConfig data
	configSecret := resources.BuildSecret(*tradeBot, configData)
	if err := shared.Apply(ctx, r.Client, tradeBot, &configSecret); err != nil {
		logger.Error(err, "Failed to apply Secret")
		return false, fmt.Errorf("failed to apply Secret: %w", err)
	}
	logger.V(2).Info("Secret applied successfully", "name", configSecret.Name)

	// 2. Create a ConfigMap for the Strategy script
	strategyConfigMap := resources.BuildStrategyConfigMap(*tradeBot, *strategy)
	if err := shared.Apply(ctx, r.Client, tradeBot, &strategyConfigMap); err != nil {
		logger.Error(err, "Failed to apply Strategy ConfigMap")
		return false, fmt.Errorf("failed to apply Strategy ConfigMap: %w", err)
	}
	logger.V(2).Info("Strategy ConfigMap applied successfully", "name", strategyConfigMap.Name)

	// 3. Branch: trade -> StatefulSet + Service + PVC, else -> Job
	effectiveCmd := strings.TrimSpace(tradeBot.Spec.FreqtradeCommand)
	if effectiveCmd == "" {
		effectiveCmd = "trade"
	}

	// Switching freqtrade_command must not leave the previous mode's workload
	// running: a StatefulSet orphaned by a switch to backtesting is a bot
	// still trading on stale config.
	if err := r.pruneStaleWorkloads(ctx, tradeBot, effectiveCmd); err != nil {
		logger.Error(err, "Failed to prune stale workloads")
		return false, fmt.Errorf("failed to prune stale workloads: %w", err)
	}

	// jobSpecChanged is only ever set true inside the Job branch below; this
	// makes sure the WorkloadImmutable condition is cleared whenever the
	// TradeBot isn't in Job mode (e.g. it just switched back to "trade").
	jobSpecChanged := false

	if effectiveCmd == "trade" {
		// PVC storage may only grow, never shrink - the API server already
		// enforces that (a request to reduce Resources.Requests.Storage is
		// rejected), so an attempted shrink surfaces as a normal apply error
		// here rather than the previous silent no-op.
		pvc := resources.BuildUserDataPVC(*tradeBot)
		if err := shared.Apply(ctx, r.Client, tradeBot, &pvc); err != nil {
			logger.Error(err, "Failed to apply PVC")
			return false, fmt.Errorf("failed to apply PVC: %w", err)
		}
		logger.V(2).Info("PVC applied successfully", "name", pvc.Name)
		// Stateful, long-running bot
		sts := resources.BuildStatefulSet(*tradeBot, strategy.Spec.Name, configSecret.Name, strategyConfigMap.Name, pvc.Name)
		if err := shared.Apply(ctx, r.Client, tradeBot, &sts); err != nil {
			logger.Error(err, "Failed to apply StatefulSet")
			return false, fmt.Errorf("failed to apply StatefulSet: %w", err)
		}
		logger.V(2).Info("StatefulSet applied successfully", "name", sts.Name)

		svc := resources.BuildService(*tradeBot)
		if err := shared.Apply(ctx, r.Client, tradeBot, &svc); err != nil {
			logger.Error(err, "Failed to apply Service")
			return false, fmt.Errorf("failed to apply Service: %w", err)
		}
		logger.V(2).Info("Service applied successfully", "name", svc.Name)
	} else {
		// One-shot commands -> Job. batchv1.Job.spec.template is immutable
		// once created - verified empirically, server-side apply enforces
		// this exactly like a typed Update would - so applying an unchanged
		// desired Job is a clean no-op, and the apply failing with
		// IsJobTemplateImmutableError *is* the drift signal, not something to
		// detect separately by comparing specs (which would have to compare
		// an undefaulted desired spec against a defaulted existing one).
		desiredJob := resources.BuildJob(*tradeBot, strategy.Spec.Name, configSecret.Name, strategyConfigMap.Name, "")
		switch err := shared.Apply(ctx, r.Client, tradeBot, &desiredJob); {
		case err == nil:
			logger.V(2).Info("Job applied successfully", "name", desiredJob.Name)
		case resources.IsJobTemplateImmutableError(err):
			jobSpecChanged = true
			logger.V(1).Info("Job spec changed after creation; template is immutable, surfacing as WorkloadImmutable",
				"name", desiredJob.Name)
		default:
			logger.Error(err, "Failed to apply Job")
			return false, fmt.Errorf("failed to apply Job: %w", err)
		}
	}

	return jobSpecChanged, nil
}

// pruneStaleWorkloads deletes the workload type the TradeBot is NOT currently
// using, named identically to it. Job mode is leaving TradeBot entirely once
// Backtest/Hyperopt land (v1beta1); until then, switching freqtrade_command
// between "trade" and a one-shot command must never leave the previous
// mode's workload running - an orphaned StatefulSet is a bot still trading
// on stale config, which is why this runs before the current mode's
// resources are applied, not after.
func (r *Reconciler) pruneStaleWorkloads(
	ctx context.Context, tradeBot *freqtradev1alpha1.TradeBot, effectiveCmd string,
) error {
	logger := log.FromContext(ctx)
	key := types.NamespacedName{Name: tradeBot.Name, Namespace: tradeBot.Namespace}

	// batchv1.Job specifically does not cascade-delete by default (the API
	// server orphans its pods unless told otherwise), which also means a
	// cluster with no garbage-collector controller running never actually
	// removes the Job object itself. Background propagation makes deletion
	// unambiguous instead of depending on a per-resource, per-version default.
	deleteOpts := []client.DeleteOption{client.PropagationPolicy(metav1.DeletePropagationBackground)}

	if effectiveCmd == "trade" {
		var job batchv1.Job
		err := r.Get(ctx, key, &job)
		if err == nil {
			logger.Info("Deleting stale Job left over from a previous one-shot command", "name", job.Name)
			if err := r.Delete(ctx, &job, deleteOpts...); err != nil && !errors.IsNotFound(err) {
				return fmt.Errorf("failed to delete stale Job %s: %w", job.Name, err)
			}
		} else if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to check for stale Job: %w", err)
		}
		return nil
	}

	var sts appsv1.StatefulSet
	err := r.Get(ctx, key, &sts)
	if err == nil {
		logger.Info("Deleting stale StatefulSet left over from trade mode", "name", sts.Name)
		if err := r.Delete(ctx, &sts, deleteOpts...); err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("failed to delete stale StatefulSet %s: %w", sts.Name, err)
		}
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check for stale StatefulSet: %w", err)
	}

	var svc corev1.Service
	err = r.Get(ctx, key, &svc)
	if err == nil {
		logger.Info("Deleting stale Service left over from trade mode", "name", svc.Name)
		if err := r.Delete(ctx, &svc, deleteOpts...); err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("failed to delete stale Service %s: %w", svc.Name, err)
		}
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check for stale Service: %w", err)
	}

	return nil
}

// workloadImmutableCondition builds the WorkloadImmutable condition: whether
// a Job-mode TradeBot's spec no longer matches the Job actually running.
// Job.Spec.Template is immutable after creation (see
// resources.IsJobTemplateImmutableError), so the changed spec was silently
// not applied unless this says so.
//
// Kept pure - no I/O, no PatchStatus call of its own - so Reconcile's single
// final PatchStatus call can set this condition alongside every other one it
// computes, in one round-trip. An earlier version gave this its own
// independent PatchStatus call instead, which raced the final one: that
// call's own re-fetch could read a cache that hadn't yet observed this
// one's write, silently reverting it.
func workloadImmutableCondition(tradeBotName string, observedGeneration int64, specChanged bool) metav1.Condition {
	condition := metav1.Condition{
		Type:               freqtradev1alpha1.ConditionWorkloadImmutable,
		Status:             metav1.ConditionFalse,
		Reason:             freqtradev1alpha1.ReasonSpecMatchesWorkload,
		Message:            "",
		ObservedGeneration: observedGeneration,
	}
	if specChanged {
		condition.Status = metav1.ConditionTrue
		condition.Reason = freqtradev1alpha1.ReasonSpecChangeIgnored
		condition.Message = fmt.Sprintf(
			"TradeBot spec changed after Job %q was created; batchv1.Job.spec.template is immutable, "+
				"so the change was not applied. Delete and recreate this TradeBot to run with the new spec. "+
				"Results from the current run are on an emptyDir and will be lost when its pod is gone "+
				"(per-run result PVCs are planned for a future Backtest/Hyperopt CRD).",
			tradeBotName,
		)
	}
	return condition
}
