package resources

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	"reflect"
	"time"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// BuildStatefulSet creates a StatefulSet for the bot, merging App.PodSpec overrides.
func BuildStatefulSet(ctx context.Context, c client.Client, tradeBot freqtradev1alpha1.TradeBot, configSecretName, strategyConfigMapName, pvcName string) appsv1.StatefulSet {
	replicas := int32(1)

	// Fetch the referenced Strategy resource to resolve the Python strategy name.
	var strategy freqtradev1alpha1.Strategy
	err := c.Get(ctx, types.NamespacedName{Namespace: tradeBot.Namespace, Name: tradeBot.Spec.Strategy}, &strategy)
	if err != nil {
		if errors.IsNotFound(err) {
			return appsv1.StatefulSet{}
		}
		panic(err)
	}
	strategyName := strategy.Spec.Name

	// Build the reusable PodSpec for trade mode.
	podSpec := BuildPod(
		tradeBot,
		strategyName,
		configSecretName,
		strategyConfigMapName,
		pvcName,
		"trade", // force long-running mode here
		tradeBot.Spec.FreqtradeArguments,
	)

	baseStatefulSetSpec := appsv1.StatefulSetSpec{
		Replicas: &replicas,
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"name": tradeBot.Name, "app": "freqtrade"},
		},
		ServiceName: tradeBot.Name,
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"name": tradeBot.Name, "app": "freqtrade"},
			},
			Spec: podSpec,
		},
	}

	return appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tradeBot.Name,
			Namespace: tradeBot.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(&tradeBot, freqtradev1alpha1.GroupVersion.WithKind("TradeBot")),
			},
		},
		Spec: baseStatefulSetSpec,
	}
}

// ApplyStatefulSet creates or updates the StatefulSet with retry logic for resource version conflicts.
func ApplyStatefulSet(ctx context.Context, c client.Client, sts *appsv1.StatefulSet) error {
	logger := log.FromContext(ctx)

	// Retry logic for resource version conflicts
	return wait.PollUntilContextTimeout(ctx, 100*time.Millisecond, 2*time.Second, true, func(ctx context.Context) (bool, error) {
		var existing appsv1.StatefulSet
		err := c.Get(ctx, types.NamespacedName{Name: sts.Name, Namespace: sts.Namespace}, &existing)
		if errors.IsNotFound(err) {
			err = c.Create(ctx, sts)
			if errors.IsAlreadyExists(err) {
				// Resource was created by another reconciliation, retry
				logger.V(2).Info("StatefulSet already exists, retrying", "name", sts.Name)
				return false, nil
			}
			return true, err
		} else if err != nil {
			return true, err
		}

		needsUpdate := false

		if !reflect.DeepEqual(existing.Spec.Template.Spec, sts.Spec.Template.Spec) {
			needsUpdate = true
		}
		if !reflect.DeepEqual(existing.Spec.Selector, sts.Spec.Selector) {
			needsUpdate = true
		}
		if existing.Spec.Replicas == nil || sts.Spec.Replicas == nil || *existing.Spec.Replicas != *sts.Spec.Replicas {
			needsUpdate = true
		}

		if needsUpdate {
			sts.ResourceVersion = existing.ResourceVersion
			err = c.Update(ctx, sts)
			if errors.IsConflict(err) {
				// Resource version conflict, retry
				logger.V(2).Info("StatefulSet resource version conflict, retrying", "name", sts.Name)
				return false, nil
			}
			return true, err
		}

		return true, nil
	})
}
