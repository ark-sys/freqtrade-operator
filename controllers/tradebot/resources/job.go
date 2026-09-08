// controllers/tradebot/resources/job.go
package resources

import (
	"context"
	"reflect"
	"time"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
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

	// Optional defaults for Job behavior
	backoff := int32(0)

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
			BackoffLimit: &backoff,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"name": tradeBot.Name, "app": "freqtrade"},
				},
				Spec: podSpec,
			},
		},
	}
}

// ApplyJob creates or updates the Job with simple conflict-retry semantics.
// Note: Some Job fields are immutable after creation; if updates fail, you may
// choose to delete and recreate in a future iteration.
func ApplyJob(ctx context.Context, c client.Client, job *batchv1.Job) error {
	logger := log.FromContext(ctx)

	return wait.PollUntilContextTimeout(ctx, 100*time.Millisecond, 2*time.Second, true, func(ctx context.Context) (bool, error) {
		var existing batchv1.Job
		err := c.Get(ctx, types.NamespacedName{Name: job.Name, Namespace: job.Namespace}, &existing)
		if errors.IsNotFound(err) {
			err = c.Create(ctx, job)
			if errors.IsAlreadyExists(err) {
				logger.V(2).Info("Job already exists, retrying", "name", job.Name)
				return false, nil
			}
			return true, err
		} else if err != nil {
			return true, err
		}

		// Compare fields that are commonly mutable; Jobs can be restrictive.
		needsUpdate := false
		if !reflect.DeepEqual(existing.Spec.Template.Spec, job.Spec.Template.Spec) {
			needsUpdate = true
		}
		if !reflect.DeepEqual(existing.Spec.Template.ObjectMeta.Labels, job.Spec.Template.ObjectMeta.Labels) {
			needsUpdate = true
		}
		if existing.Spec.BackoffLimit == nil || job.Spec.BackoffLimit == nil ||
			*existing.Spec.BackoffLimit != *job.Spec.BackoffLimit {
			needsUpdate = true
		}

		if needsUpdate {
			job.ResourceVersion = existing.ResourceVersion
			err = c.Update(ctx, job)
			if errors.IsConflict(err) {
				logger.V(2).Info("Job resource version conflict, retrying", "name", job.Name)
				return false, nil
			}
			return true, err
		}

		return true, nil
	})
}
