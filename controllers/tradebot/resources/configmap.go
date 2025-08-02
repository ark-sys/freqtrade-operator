package resources

import (
	"context"
	"reflect"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// BuildStrategyConfigMap creates a ConfigMap for the bot's strategy script
func BuildStrategyConfigMap(tradeBot freqtradev1alpha1.TradeBot, strategy freqtradev1alpha1.Strategy) corev1.ConfigMap {
	return corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tradeBot.Name + "-strategy",
			Namespace: tradeBot.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(&tradeBot, freqtradev1alpha1.GroupVersion.WithKind("TradeBot")),
			},
		},
		Data: map[string]string{
			strategy.Spec.Name + ".py": strategy.Spec.Script,
		},
	}
}

// ApplyConfigMap creates or updates the ConfigMap
func ApplyConfigMap(ctx context.Context, c client.Client, cm *corev1.ConfigMap) error {
	var existing corev1.ConfigMap
	err := c.Get(ctx, types.NamespacedName{Name: cm.Name, Namespace: cm.Namespace}, &existing)
	if errors.IsNotFound(err) {
		return c.Create(ctx, cm)
	} else if err != nil {
		return err
	}

	// Check if update is needed by comparing data
	needsUpdate := false

	// Compare data fields
	if !reflect.DeepEqual(existing.Data, cm.Data) {
		needsUpdate = true
	}

	// Only update if there are actual changes
	if needsUpdate {
		cm.ResourceVersion = existing.ResourceVersion
		return c.Update(ctx, cm)
	}

	return nil
}
