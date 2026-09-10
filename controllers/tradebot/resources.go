package tradebot

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	"github.com/ark-sys/freqtrade-operator/controllers/shared"
	"github.com/ark-sys/freqtrade-operator/controllers/tradebot/resources"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *Reconciler) defaultImage() string {
	if r.DefaultImage != "" {
		return r.DefaultImage
	}
	return shared.DefaultFreqtradeImage
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
	configKey := types.NamespacedName{Name: tradeBot.Spec.Config, Namespace: namespace}
	if err := r.Get(ctx, configKey, tradeBotConfig); err != nil {
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
// needs from reconcileResources.
type reconcileOutcome struct {
	// appliedConfigHash is sha256(config.json + strategy script)[:16] for
	// the config just rendered into the Secret - TradeBotStatus.AppliedConfigHash
	// always reflects it.
	appliedConfigHash string
	// resolvedImage is the exact image reference the built StatefulSet's main
	// container ended up with, read back post-merge so a
	// spec.app.pod.image override is reflected too - TradeBotStatus.ResolvedImage
	// always reflects it (P3-3).
	resolvedImage string
	// configDrift is whether the rendered config has moved on from what the
	// StatefulSet pod template - and so the running pods - still reflect.
	// Always false under spec.updateStrategy: Auto, which keeps the
	// template current itself.
	configDrift bool
}

// reconcileResources creates or updates all resources needed by the TradeBot.
// Trade-only (P6-4): by the time this runs, Reconcile has already rejected
// anything but a live bot (see its own step 3.5), so there's no command
// branch here anymore - every TradeBot gets a StatefulSet.
// strategy must be the already-resolved Strategy referenced by tradeBot.Spec.Strategy
// (fetchReferencedResources fetches it once per reconcile; this stops a second,
// redundant fetch here and lets BuildStatefulSet stay a pure function).
func (r *Reconciler) reconcileResources(
	ctx context.Context,
	tradeBot *freqtradev1alpha1.TradeBot,
	strategy *freqtradev1alpha1.Strategy,
	configData map[string]string,
	freqUINames []string,
) (reconcileOutcome, error) {

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

	// 3. PVC storage may only grow, never shrink - the API server already
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

	netpol := resources.BuildNetworkPolicy(*tradeBot, freqUINames, r.OperatorNamespace)
	if err := shared.Apply(ctx, r.Client, tradeBot, &netpol); err != nil {
		logger.Error(err, "Failed to apply NetworkPolicy")
		return outcome, fmt.Errorf("failed to apply NetworkPolicy: %w", err)
	}
	logger.V(2).Info("NetworkPolicy applied successfully", "name", netpol.Name)

	return outcome, nil
}

// configDriftCondition builds the ConfigDrift condition (P2-4): true when a
// trade-mode TradeBot's rendered config has moved on from what the running
// StatefulSet pods still reflect, because spec.updateStrategy is Manual (the
// default, D2) and nothing has restarted them yet. Kept pure - no I/O, no
// PatchStatus call of its own - so Reconcile's single final PatchStatus
// call can set this condition alongside every other one it computes, in
// one round-trip.
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
