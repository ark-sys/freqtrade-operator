package backtest

import (
	"context"
	"encoding/json"
	"fmt"

	freqtradev1beta1 "github.com/ark-sys/freqtrade-operator/api/v1beta1"
	"github.com/ark-sys/freqtrade-operator/controllers/backtest/resources"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// resultsDataKey/errorDataKey must match collectresults' own writer -
// controllers/backtest/collectresults/collectresults.go.
const (
	resultsDataKey = "results.json"
	errorDataKey   = "error"
)

// adoptAndParseResults reads back the P6-2 sidecar's own results
// ConfigMap once the Job has succeeded, adopts it (setting the Backtest
// as its controller owner, so it GCs with the run - the sidecar itself
// has no RBAC to do this: reading the Backtest object to learn its UID
// isn't among the narrow permissions resources.BuildSidecarRole grants
// it, deliberately), and parses it into status.results.
//
// The three return values are: the extracted results (nil unless fully
// successful), the ResultsAvailable condition to set (Type is left zero
// when there's nothing new to say - only the "ConfigMap not found yet"
// case, which the caller requeues for instead), and whether the caller
// should requeue shortly to check again.
func (r *Reconciler) adoptAndParseResults(
	ctx context.Context, backtest *freqtradev1beta1.Backtest,
) (*freqtradev1beta1.BacktestResults, metav1.Condition, bool, error) {
	var cm corev1.ConfigMap
	key := types.NamespacedName{Name: resources.ResultsConfigMapName(backtest.Name), Namespace: backtest.Namespace}
	switch err := r.Get(ctx, key, &cm); {
	case apierrors.IsNotFound(err):
		return nil, metav1.Condition{}, true, nil
	case err != nil:
		return nil, metav1.Condition{}, false, fmt.Errorf("failed to get results ConfigMap: %w", err)
	}

	if err := r.ensureOwned(ctx, backtest, &cm); err != nil {
		return nil, metav1.Condition{}, false, fmt.Errorf("failed to adopt results ConfigMap: %w", err)
	}

	if errMsg, ok := cm.Data[errorDataKey]; ok {
		return nil, unavailableCondition(errMsg, backtest.Generation), false, nil
	}

	raw, ok := cm.Data[resultsDataKey]
	if !ok {
		return nil, unavailableCondition(
			fmt.Sprintf("results ConfigMap %s has neither %q nor %q", cm.Name, resultsDataKey, errorDataKey),
			backtest.Generation,
		), false, nil
	}

	var results freqtradev1beta1.BacktestResults
	if err := json.Unmarshal([]byte(raw), &results); err != nil {
		return nil, unavailableCondition(
			fmt.Sprintf("results ConfigMap %s's %s is malformed: %v", cm.Name, resultsDataKey, err), backtest.Generation,
		), false, nil
	}

	return &results, metav1.Condition{
		Type: freqtradev1beta1.ConditionResultsAvailable, Status: metav1.ConditionTrue,
		Reason: freqtradev1beta1.ReasonAsExpected, ObservedGeneration: backtest.Generation,
	}, false, nil
}

func unavailableCondition(message string, generation int64) metav1.Condition {
	return metav1.Condition{
		Type: freqtradev1beta1.ConditionResultsAvailable, Status: metav1.ConditionFalse,
		Reason: freqtradev1beta1.ReasonResultsUnavailable, Message: message, ObservedGeneration: generation,
	}
}

// ensureOwned sets owner as cm's controller owner if it isn't already -
// a plain client.Update, not shared.PatchStatus (this is a spec-level
// field, OwnerReferences, not a status subresource) nor shared.Apply
// (server-side apply here would fight the sidecar's own field ownership
// of cm.Data for no reason; this only ever needs to touch OwnerReferences).
func (r *Reconciler) ensureOwned(ctx context.Context, owner *freqtradev1beta1.Backtest, cm *corev1.ConfigMap) error {
	for _, ref := range cm.OwnerReferences {
		if ref.UID == owner.UID {
			return nil
		}
	}
	if err := controllerutil.SetControllerReference(owner, cm, r.Scheme); err != nil {
		return err
	}
	return r.Update(ctx, cm)
}
