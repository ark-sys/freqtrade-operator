package tradebot

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

// DefaultFreqtradeImage is used when Reconciler.DefaultImage is unset (also
// cmd/main.go's --default-freqtrade-image flag default, so both stay in
// sync from this one definition). Digest-pinned (P3-3) rather than a
// floating tag like the old freqtradeorg/freqtrade:stable, so a pod
// restart can never silently change what version of freqtrade a bot is
// running. Update it deliberately, as its own reviewable change, when
// freqtrade ships a version worth moving to; verified this exact digest
// starts cleanly under the P3-3 restricted SecurityContext (both `trade`
// and `download-data`) before pinning it.
const DefaultFreqtradeImage = "freqtradeorg/freqtrade@sha256:" +
	"7031bca43ed7668ebf421725dd5016acade6ef88b0771db3e08c96e6d19a42db"

func (r *Reconciler) defaultImage() string {
	if r.DefaultImage != "" {
		return r.DefaultImage
	}
	return DefaultFreqtradeImage
}

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

// reconcileOutcome carries what Reconcile's single final PatchStatus call
// needs from reconcileResources, computed here rather than via a second,
// independent status write - see workloadImmutableCondition's comment for
// why a second PatchStatus call earlier in the same reconcile is a race,
// not just redundant.
type reconcileOutcome struct {
	// jobSpecChanged is whether a Job-mode TradeBot's spec no longer
	// matches the Job actually running (see the Job branch below).
	jobSpecChanged bool
	// appliedConfigHash is sha256(config.json + strategy script)[:16] for
	// the config just rendered into the Secret, regardless of mode -
	// TradeBotStatus.AppliedConfigHash always reflects it.
	appliedConfigHash string
	// resolvedImage is the exact image reference the built workload's main
	// container ended up with, read back post-merge so an
	// spec.app.pod.image override is reflected too - TradeBotStatus.ResolvedImage
	// always reflects it (P3-3).
	resolvedImage string
	// configDrift is whether a trade-mode TradeBot's rendered config has
	// moved on from what the StatefulSet pod template - and so the running
	// pods - still reflect. Always false in Job mode, where
	// WorkloadImmutable already covers "the spec changed after the
	// workload was created" (P0-4); always false under
	// spec.updateStrategy: Auto, which keeps the template current itself.
	configDrift bool
}

// reconcileResources creates or updates all resources needed by the TradeBot.
// strategy must be the already-resolved Strategy referenced by tradeBot.Spec.Strategy
// (fetchReferencedResources fetches it once per reconcile; this stops a second,
// redundant fetch here and lets Build{StatefulSet,Job} stay pure functions).
func (r *Reconciler) reconcileResources(
	ctx context.Context,
	tradeBot *freqtradev1alpha1.TradeBot,
	strategy *freqtradev1alpha1.Strategy,
	configData map[string]string) (reconcileOutcome, error) {

	logger := log.FromContext(ctx)
	logger.V(2).Info("Reconciling resources for TradeBot", "name", tradeBot.Name)

	outcome := reconcileOutcome{appliedConfigHash: computeConfigHash(configData, strategy.Spec.Script)}

	// 1. Create a config.json Secret with the TradeBotConfig data
	configSecret := resources.BuildSecret(*tradeBot, configData)
	if err := shared.Apply(ctx, r.Client, tradeBot, &configSecret); err != nil {
		logger.Error(err, "Failed to apply Secret")
		return outcome, fmt.Errorf("failed to apply Secret: %w", err)
	}
	logger.V(2).Info("Secret applied successfully", "name", configSecret.Name)

	// 2. Create a ConfigMap for the Strategy script
	strategyConfigMap := resources.BuildStrategyConfigMap(*tradeBot, *strategy)
	if err := shared.Apply(ctx, r.Client, tradeBot, &strategyConfigMap); err != nil {
		logger.Error(err, "Failed to apply Strategy ConfigMap")
		return outcome, fmt.Errorf("failed to apply Strategy ConfigMap: %w", err)
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
		return outcome, fmt.Errorf("failed to prune stale workloads: %w", err)
	}

	if effectiveCmd == "trade" {
		// PVC storage may only grow, never shrink - the API server already
		// enforces that (a request to reduce Resources.Requests.Storage is
		// rejected), so an attempted shrink surfaces as a normal apply error
		// here rather than the previous silent no-op.
		pvc := resources.BuildUserDataPVC(*tradeBot)
		if err := shared.Apply(ctx, r.Client, tradeBot, &pvc); err != nil {
			logger.Error(err, "Failed to apply PVC")
			return outcome, fmt.Errorf("failed to apply PVC: %w", err)
		}
		logger.V(2).Info("PVC applied successfully", "name", pvc.Name)

		templateHash, err := r.statefulSetTemplateHash(ctx, tradeBot, outcome.appliedConfigHash)
		if err != nil {
			logger.Error(err, "Failed to determine StatefulSet config-hash annotation")
			return outcome, fmt.Errorf("failed to determine StatefulSet config-hash annotation: %w", err)
		}
		outcome.configDrift = templateHash != outcome.appliedConfigHash

		// Stateful, long-running bot
		sts := resources.BuildStatefulSet(
			*tradeBot, r.defaultImage(), strategy.Spec.Name, configSecret.Name, strategyConfigMap.Name, pvc.Name, templateHash,
		)
		outcome.resolvedImage = sts.Spec.Template.Spec.Containers[0].Image
		if err := shared.Apply(ctx, r.Client, tradeBot, &sts); err != nil {
			logger.Error(err, "Failed to apply StatefulSet")
			return outcome, fmt.Errorf("failed to apply StatefulSet: %w", err)
		}
		logger.V(2).Info("StatefulSet applied successfully", "name", sts.Name)
		r.recordConfigRestartEvent(tradeBot, templateHash)

		svc := resources.BuildService(*tradeBot)
		if err := shared.Apply(ctx, r.Client, tradeBot, &svc); err != nil {
			logger.Error(err, "Failed to apply Service")
			return outcome, fmt.Errorf("failed to apply Service: %w", err)
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
		desiredJob := resources.BuildJob(
			*tradeBot, r.defaultImage(), strategy.Spec.Name, configSecret.Name, strategyConfigMap.Name, "",
		)
		outcome.resolvedImage = desiredJob.Spec.Template.Spec.Containers[0].Image
		switch err := shared.Apply(ctx, r.Client, tradeBot, &desiredJob); {
		case err == nil:
			logger.V(2).Info("Job applied successfully", "name", desiredJob.Name)
		case resources.IsJobTemplateImmutableError(err):
			outcome.jobSpecChanged = true
			logger.V(1).Info("Job spec changed after creation; template is immutable, surfacing as WorkloadImmutable",
				"name", desiredJob.Name)
		default:
			logger.Error(err, "Failed to apply Job")
			return outcome, fmt.Errorf("failed to apply Job: %w", err)
		}
	}

	return outcome, nil
}

// configDriftCondition builds the ConfigDrift condition (P2-4): true when a
// trade-mode TradeBot's rendered config has moved on from what the running
// StatefulSet pods still reflect, because spec.updateStrategy is Manual (the
// default, D2) and nothing has restarted them yet. Kept pure for the same
// reason workloadImmutableCondition is - see its comment.
func configDriftCondition(tradeBotName, namespace string, observedGeneration int64, drift bool) metav1.Condition {
	condition := metav1.Condition{
		Type:               freqtradev1alpha1.ConditionConfigDrift,
		Status:             metav1.ConditionFalse,
		Reason:             freqtradev1alpha1.ReasonAsExpected,
		Message:            "",
		ObservedGeneration: observedGeneration,
	}
	if drift {
		condition.Status = metav1.ConditionTrue
		condition.Reason = freqtradev1alpha1.ReasonPendingRestart
		condition.Message = fmt.Sprintf(
			"Rendered config has changed but spec.updateStrategy is Manual, so the StatefulSet pod template was "+
				"deliberately left untouched (a bot may be holding open positions - an unrequested restart is not "+
				"this operator's call to make). Roll it out with: kubectl rollout restart statefulset/%s -n %s "+
				"- or set spec.updateStrategy: Auto, which this operator applies itself and clears this condition for.",
			tradeBotName, namespace,
		)
	}
	return condition
}

// computeConfigHash returns sha256(config.json bytes + strategy script
// bytes), truncated to 16 hex characters - see TradeBotStatus.AppliedConfigHash.
func computeConfigHash(configData map[string]string, strategyScript string) string {
	h := sha256.New()
	h.Write([]byte(configData["config.json"]))
	h.Write([]byte(strategyScript))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// statefulSetTemplateHash decides which config-hash value BuildStatefulSet
// should write onto the pod template this reconcile: under Auto, always the
// freshly-rendered one, so a real change is a template diff Kubernetes
// rolls out on its own. Under Manual (the default, D2), the template's own
// currently-persisted value, so this reconcile can never itself be what
// triggers a restart - unless nothing has been applied yet (a brand new
// StatefulSet has no prior pods to disturb), in which case it seeds the
// template with the fresh hash instead of leaving the annotation empty.
func (r *Reconciler) statefulSetTemplateHash(
	ctx context.Context, tradeBot *freqtradev1alpha1.TradeBot, freshHash string,
) (string, error) {
	if tradeBot.Spec.UpdateStrategy == "Auto" {
		return freshHash, nil
	}

	var existing appsv1.StatefulSet
	key := types.NamespacedName{Name: tradeBot.Name, Namespace: tradeBot.Namespace}
	switch err := r.Get(ctx, key, &existing); {
	case err == nil:
		if h := existing.Spec.Template.Annotations[resources.ConfigHashAnnotation]; h != "" {
			return h, nil
		}
		return freshHash, nil
	case errors.IsNotFound(err):
		return freshHash, nil
	default:
		return "", err
	}
}

// recordConfigRestartEvent emits a value-free Event when an Auto-mode
// reconcile just changed the StatefulSet's config-hash annotation, i.e. is
// about to cause a real rolling restart - never when the value it wrote
// matches what was already there (a fresh create, or a no-op reconcile).
// r.Recorder is nil in tests that have no need of it; skip rather than
// panic on a nil interface call.
func (r *Reconciler) recordConfigRestartEvent(tradeBot *freqtradev1alpha1.TradeBot, newHash string) {
	if r.Recorder == nil || tradeBot.Spec.UpdateStrategy != "Auto" {
		return
	}
	if tradeBot.Status.AppliedConfigHash == "" || tradeBot.Status.AppliedConfigHash == newHash {
		return
	}
	r.Recorder.Event(tradeBot, corev1.EventTypeNormal, "ConfigRestart",
		"Config changed and spec.updateStrategy is Auto: triggering a StatefulSet rolling restart")
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
