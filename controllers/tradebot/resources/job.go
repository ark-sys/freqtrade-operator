// controllers/tradebot/resources/job.go
package resources

import (
	"context"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Job mode is leaving TradeBot entirely once Backtest/Hyperopt land (v1beta1);
// these are deliberately hardcoded rather than CRD fields until then.
const (
	// jobBackoffLimit allows one retry instead of failing permanently after a
	// single pod failure (the previous BackoffLimit: 0 did exactly that).
	jobBackoffLimit = int32(1)
	// jobTTLSecondsAfterFinished reaps a finished Job (and its pod) after 24h
	// so completed one-shot runs don't accumulate forever.
	jobTTLSecondsAfterFinished = int32(86400)
)

// BuildJob creates a Job for one-shot commands (e.g., backtesting, hyperopt)
// using the reusable PodSpec from BuildPod. strategyName is the resolved
// Strategy.Spec.Name; the caller is responsible for having already fetched
// and validated the referenced Strategy exists.
func BuildJob(
	tradeBot freqtradev1alpha1.TradeBot,
	strategyName, configSecretName, strategyConfigMapName, pvcName string,
) batchv1.Job {
	freqCommand := tradeBot.Spec.FreqtradeCommand
	if freqCommand == "" {
		freqCommand = "trade"
	}

	// Build the PodSpec via the shared builder
	podSpec := BuildPod(
		tradeBot,
		strategyName,
		configSecretName,
		strategyConfigMapName,
		pvcName,
		freqCommand,
		tradeBot.Spec.FreqtradeArguments,
	)

	// Jobs require RestartPolicy Never or OnFailure
	if podSpec.RestartPolicy == "" {
		podSpec.RestartPolicy = corev1.RestartPolicyOnFailure
	}

	backoff := jobBackoffLimit
	ttl := jobTTLSecondsAfterFinished

	return batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tradeBot.Name,
			Namespace: tradeBot.Namespace,
			Labels:    map[string]string{"name": tradeBot.Name, "app": "freqtrade"},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(&tradeBot, freqtradev1alpha1.GroupVersion.WithKind("TradeBot")),
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoff,
			TTLSecondsAfterFinished: &ttl,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"name": tradeBot.Name, "app": "freqtrade"},
				},
				Spec: podSpec,
			},
		},
	}
}

// ApplyJob creates the Job if it doesn't already exist. It deliberately never
// updates: batchv1.Job.Spec.Template is immutable after creation, so an
// Update here would be rejected by the API server on every single reconcile
// once the TradeBot's spec drifts from the Job that was first created for it
// (see reconcileResources, which surfaces that drift as a condition instead).
// If the Job already exists, job is overwritten with the existing object so
// the caller can compare what was desired against what's actually running.
func ApplyJob(ctx context.Context, c client.Client, job *batchv1.Job) error {
	var existing batchv1.Job
	err := c.Get(ctx, types.NamespacedName{Name: job.Name, Namespace: job.Namespace}, &existing)
	switch {
	case err == nil:
		*job = existing
		return nil
	case errors.IsNotFound(err):
		return c.Create(ctx, job)
	default:
		return err
	}
}
