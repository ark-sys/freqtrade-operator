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
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// BuildSecret creates a Secret for the bot's config.json
func BuildSecret(tradeBot freqtradev1alpha1.TradeBot, secretData map[string]string) corev1.Secret {
	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tradeBot.Name + "-config",
			Namespace: tradeBot.Namespace,
			Labels: map[string]string{
				"app":  "freqtrade",
				"name": tradeBot.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(&tradeBot, freqtradev1alpha1.GroupVersion.WithKind("TradeBot")),
			},
		},
		StringData: secretData,
		Type:       corev1.SecretTypeOpaque,
	}
}

// ApplySecret creates or updates the Secret
func ApplySecret(ctx context.Context, c client.Client, secret *corev1.Secret) error {
	var existing corev1.Secret
	err := c.Get(ctx, types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace}, &existing)
	if errors.IsNotFound(err) {
		return c.Create(ctx, secret)
	} else if err != nil {
		return err
	}

	// Check if update is needed by comparing data
	needsUpdate := false

	// Convert StringData to Data for comparison since Kubernetes stores secrets as base64 encoded Data
	expectedData := make(map[string][]byte)
	for key, value := range secret.StringData {
		expectedData[key] = []byte(value)
	}

	// Compare the actual data content
	needsUpdate = !reflect.DeepEqual(existing.Data, expectedData)

	// Debug logging to help troubleshoot
	logger := log.FromContext(ctx)
	if needsUpdate {
		logger.V(2).Info("Secret data differs, update needed",
			"secretName", secret.Name,
			"existingKeys", len(existing.Data),
			"expectedKeys", len(expectedData))
		// Log first few characters of each key for debugging
		for key := range existing.Data {
			existingLen := len(existing.Data[key])
			expectedLen := len(expectedData[key])
			logger.V(2).Info("Key comparison", "key", key, "existingLen", existingLen, "expectedLen", expectedLen)
		}
		secret.ResourceVersion = existing.ResourceVersion
		return c.Update(ctx, secret)
	}

	return nil
}
