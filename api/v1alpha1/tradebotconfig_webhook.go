package v1alpha1

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/validate-freqtrade-io-v1alpha1-tradebotconfig,mutating=false,failurePolicy=fail,sideEffects=None,groups=freqtrade.io,resources=tradebotconfigs,verbs=create;update,versions=v1alpha1,name=vtradebotconfig.kb.io,admissionReviewVersions=v1

// knownCredentialSecretKeys are the Secret data keys BuildExchangeConfig
// (controllers/tradebotconfig/configbuilder/exchange.go) knows how to read.
// Documented here per P3-1's problem statement: today this is discoverable
// only by reading the source.
var knownCredentialSecretKeys = []string{
	"api-key", "secret", "password", "uid", "account_id", "wallet_address", "private_key",
}

// TradeBotConfigCustomValidator validates TradeBotConfig create/update requests.
//
// +kubebuilder:object:generate=false
type TradeBotConfigCustomValidator struct {
	Client client.Client
}

// SetupWebhookWithManager registers the TradeBotConfig validating webhook.
func (c *TradeBotConfig) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(c).
		WithValidator(&TradeBotConfigCustomValidator{Client: mgr.GetClient()}).
		Complete()
}

// ValidateCreate implements admission.CustomValidator.
func (v *TradeBotConfigCustomValidator) ValidateCreate(
	ctx context.Context, obj runtime.Object,
) (admission.Warnings, error) {
	cfg, ok := obj.(*TradeBotConfig)
	if !ok {
		return nil, fmt.Errorf("expected a TradeBotConfig but got %T", obj)
	}
	return nil, v.validate(ctx, cfg)
}

// ValidateUpdate implements admission.CustomValidator.
func (v *TradeBotConfigCustomValidator) ValidateUpdate(
	ctx context.Context, _, newObj runtime.Object,
) (admission.Warnings, error) {
	cfg, ok := newObj.(*TradeBotConfig)
	if !ok {
		return nil, fmt.Errorf("expected a TradeBotConfig but got %T", newObj)
	}
	return nil, v.validate(ctx, cfg)
}

// ValidateDelete implements admission.CustomValidator. Deletion is never
// rejected.
func (v *TradeBotConfigCustomValidator) ValidateDelete(context.Context, runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// validate rejects a TradeBotConfig with no exchange section, a live
// (non-dry-run) bot with no credentials source, or a secretRef that doesn't
// resolve to a Secret carrying at least one recognized credential key.
//
// dry_run defaults to nil, not false: an unset dry_run is treated as "not
// explicitly true" and held to the same bar as dry_run=false, erring toward
// safety since this field gates whether real money moves.
//
// This is a synchronous, webhook-level complement to configbuilder's
// ErrMissingExchange (P0-1) check, not a replacement for it - CRD schema
// doesn't make spec.exchange required, so that runtime check still matters
// for any object that predates this webhook or reaches the reconciler by
// some other path.
func (v *TradeBotConfigCustomValidator) validate(ctx context.Context, cfg *TradeBotConfig) error {
	if cfg.Spec.Exchange == nil {
		return fmt.Errorf("spec.exchange is required")
	}

	dryRun := cfg.Spec.Bot.DryRun != nil && *cfg.Spec.Bot.DryRun
	if !dryRun && cfg.Spec.Exchange.SecretRef == "" {
		return fmt.Errorf("spec.exchange.secretRef is required when spec.bot.dry_run is not explicitly true")
	}

	if cfg.Spec.Exchange.SecretRef == "" {
		return nil
	}

	var secret corev1.Secret
	secretKey := types.NamespacedName{Name: cfg.Spec.Exchange.SecretRef, Namespace: cfg.Namespace}
	if err := v.Client.Get(ctx, secretKey, &secret); err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("spec.exchange.secretRef: Secret %q not found in namespace %q",
				cfg.Spec.Exchange.SecretRef, cfg.Namespace)
		}
		return fmt.Errorf("spec.exchange.secretRef: failed to look up Secret %q: %w", cfg.Spec.Exchange.SecretRef, err)
	}

	for _, key := range knownCredentialSecretKeys {
		if _, ok := secret.Data[key]; ok {
			return nil
		}
	}
	return fmt.Errorf("spec.exchange.secretRef: Secret %q has none of the expected credential keys (%s)",
		cfg.Spec.Exchange.SecretRef, strings.Join(knownCredentialSecretKeys, ", "))
}
