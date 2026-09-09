package backtest

import (
	"context"
	"fmt"
	"time"

	freqtradev1beta1 "github.com/ark-sys/freqtrade-operator/api/v1beta1"
	"github.com/ark-sys/freqtrade-operator/controllers/backtest/resources"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// finalizeBacktest handles spec.results.retentionPolicy: Retain by
// stripping the results PVC's owner reference before the Backtest itself
// is removed, so the API server's normal owner-ref garbage collection
// leaves it behind - the same pattern controllers/tradebot/finalizers.go
// uses for freqtrade.io/preserve-data, generalized here into the CRD's own
// typed field rather than an annotation, since RetentionPolicy is a
// first-class, immutable-at-creation spec choice for a Backtest (D1),
// unlike TradeBot's opt-in-per-deletion annotation. Unlike TradeBot's
// finalizer, there is no live workload to scale down first - a Backtest's
// Job either already finished or is safe to let cascade-delete along with
// everything else - so this always completes in one pass.
func (r *Reconciler) finalizeBacktest(ctx context.Context, backtest *freqtradev1beta1.Backtest) error {
	logger := log.FromContext(ctx)

	if resources.ResultsRetentionPolicy(backtest) != "Retain" {
		return nil
	}

	pvcName := backtest.Name + "-results"
	var pvc corev1.PersistentVolumeClaim
	key := types.NamespacedName{Name: pvcName, Namespace: backtest.Namespace}
	if err := r.Get(ctx, key, &pvc); err != nil {
		if errors.IsNotFound(err) {
			logger.V(1).Info("Results PVC not found, nothing to preserve", "name", pvcName)
			return nil
		}
		return fmt.Errorf("failed to get results PVC during finalization: %w", err)
	}

	var newOwnerRefs []metav1.OwnerReference
	for _, ownerRef := range pvc.OwnerReferences {
		if ownerRef.UID != backtest.UID {
			newOwnerRefs = append(newOwnerRefs, ownerRef)
		}
	}
	pvc.OwnerReferences = newOwnerRefs

	if pvc.Annotations == nil {
		pvc.Annotations = make(map[string]string)
	}
	pvc.Annotations["freqtrade.io/preserved-from"] = backtest.Name
	pvc.Annotations["freqtrade.io/preserved-at"] = time.Now().Format(time.RFC3339)

	if err := r.Update(ctx, &pvc); err != nil {
		return fmt.Errorf("failed to update results PVC to preserve it: %w", err)
	}
	logger.Info("Preserved results PVC per spec.results.retentionPolicy: Retain", "name", pvcName)
	return nil
}
