// controllers/tradebot/resources/job.go
package resources

import (
	"strings"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// IsJobTemplateImmutableError reports whether err is the API server
// rejecting an attempt to change batchv1.Job.spec.template on an existing
// Job. Verified empirically against envtest: server-side apply enforces
// this the same way a typed Update does, failing with an Invalid error
// whose message says the field is immutable. reconcileResources uses this
// to tell "the Job's spec drifted from what's running" (expected, surfaced
// as the WorkloadImmutable condition) apart from a genuine apply failure.
func IsJobTemplateImmutableError(err error) bool {
	return errors.IsInvalid(err) && strings.Contains(err.Error(), "field is immutable")
}
