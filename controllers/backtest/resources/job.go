// controllers/backtest/resources/job.go
package resources

import (
	freqtradev1beta1 "github.com/ark-sys/freqtrade-operator/api/v1beta1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// defaultTTLSecondsAfterFinished applies when spec.ttlSecondsAfterFinished
// is unset - matches the +kubebuilder:default on the field itself; used
// only by tests and callers that build a Backtest without going through
// admission defaulting.
const defaultTTLSecondsAfterFinished = int32(86400)

// jobBackoffLimit allows one retry instead of failing permanently after a
// single pod failure - a transient node/scheduling issue shouldn't burn an
// entire backtest run.
const jobBackoffLimit = int32(1)

// BuildJob creates the Job for a Backtest run. Name is the Backtest's own
// name - no spec-hash suffix needed (unlike v1alpha1 TradeBot's old Job
// mode), because BacktestSpec is immutable after creation (CEL, see
// api/v1beta1/backtest_types.go): the CR itself is the run's identity, per
// D1.
func BuildJob(
	backtest freqtradev1beta1.Backtest, image, operatorImage, strategyName, configSecretName, strategyConfigMapName string,
) batchv1.Job {
	podSpec := BuildPod(backtest, image, operatorImage, strategyName, configSecretName, strategyConfigMapName)

	backoff := jobBackoffLimit
	ttl := defaultTTLSecondsAfterFinished
	if backtest.Spec.TTLSecondsAfterFinished != nil {
		ttl = *backtest.Spec.TTLSecondsAfterFinished
	}

	labels := map[string]string{"name": backtest.Name, "app": "freqtrade-backtest"}
	return batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: backtest.Name, Namespace: backtest.Namespace, Labels: labels},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoff,
			TTLSecondsAfterFinished: &ttl,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec:       podSpec,
			},
		},
	}
}
