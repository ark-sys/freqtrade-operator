package v1beta1

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/validate-freqtrade-io-v1beta1-tradebotconfig,mutating=false,failurePolicy=fail,sideEffects=None,groups=freqtrade.io,resources=tradebotconfigs,verbs=create;update,versions=v1beta1,name=vtradebotconfigv1beta1.kb.io,admissionReviewVersions=v1

// knownCredentialSecretKeys are the Secret data keys BuildExchangeConfig
// (controllers/tradebotconfig/configbuilder/exchange.go) knows how to read.
// Documented here per P3-1's problem statement: today this is discoverable
// only by reading the source.
var knownCredentialSecretKeys = []string{
	"api-key", "secret", "password", "uid", "account_id", "wallet_address", "private_key",
}

// TradeBotConfigCustomValidator validates TradeBotConfig create/update
// requests. This is B2's (REMAINING-WORK.md) move of the structural
// validation api/v1alpha1/tradebotconfig_webhook.go used to own -
// v1beta1 has no plaintext credential fields at all, so there's no
// annotation gate to enforce here (see that file's own reduced scope);
// only the "does this config make sense" checks live here now.
//
// +kubebuilder:object:generate=false
type TradeBotConfigCustomValidator struct {
	// Client is read-only and, in production, uncached (mgr.GetAPIReader()) - see
	// api/v1alpha1.TradeBotCustomValidator.Client: the same stale-cache "not found" applies to a
	// referenced Secret created in the same apply as the TradeBotConfig that names it.
	Client client.Reader
}

// SetupWebhookWithManager registers the v1beta1 TradeBotConfig validating webhook.
func (c *TradeBotConfig) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, c).
		WithValidator(&TradeBotConfigCustomValidator{
			Client: mgr.GetAPIReader(),
		}).
		Complete()
}

// ValidateCreate implements admission.Validator.
func (v *TradeBotConfigCustomValidator) ValidateCreate(
	ctx context.Context, cfg *TradeBotConfig,
) (admission.Warnings, error) {
	return nil, v.validate(ctx, cfg)
}

// ValidateUpdate implements admission.Validator. Skips validation once DeletionTimestamp
// is set - same deadlock risk, and same fix, as TradeBotCustomValidator.ValidateUpdate (see its
// doc comment): the referenced exchange Secret can already be gone by the time something needs
// to update this object (e.g. to strip its finalizer, if it has one) during a namespace-wide
// delete, and re-validating the Secret's existence at that point can only block cleanup, never
// help it.
func (v *TradeBotConfigCustomValidator) ValidateUpdate(
	ctx context.Context, _, cfg *TradeBotConfig,
) (admission.Warnings, error) {
	if cfg.DeletionTimestamp != nil {
		return nil, nil
	}
	return nil, v.validate(ctx, cfg)
}

// ValidateDelete implements admission.Validator. Deletion is never
// rejected.
func (v *TradeBotConfigCustomValidator) ValidateDelete(context.Context, *TradeBotConfig) (admission.Warnings, error) {
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
	if !dryRun && cfg.Spec.Exchange.SecretRef.Name == "" {
		return fmt.Errorf("spec.exchange.secretRef is required when spec.bot.dry_run is not explicitly true")
	}

	if cfg.Spec.Exchange.SecretRef.Name == "" {
		return nil
	}

	var secret corev1.Secret
	secretKey := types.NamespacedName{Name: cfg.Spec.Exchange.SecretRef.Name, Namespace: cfg.Namespace}
	if err := v.Client.Get(ctx, secretKey, &secret); err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("spec.exchange.secretRef: Secret %q not found in namespace %q",
				cfg.Spec.Exchange.SecretRef.Name, cfg.Namespace)
		}
		return fmt.Errorf("spec.exchange.secretRef: failed to look up Secret %q: %w", cfg.Spec.Exchange.SecretRef.Name, err)
	}

	for _, key := range knownCredentialSecretKeys {
		if _, ok := secret.Data[key]; ok {
			return nil
		}
	}
	return fmt.Errorf("spec.exchange.secretRef: Secret %q has none of the expected credential keys (%s)",
		cfg.Spec.Exchange.SecretRef.Name, strings.Join(knownCredentialSecretKeys, ", "))
}
