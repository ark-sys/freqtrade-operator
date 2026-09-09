package v1alpha1

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
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

// allowPlaintextCredentialsAnnotation opts a TradeBotConfig out of P3-1's
// plaintext-credential rejection. account_id and Telegram's ChatID/TopicID
// aren't credentials (they're identifiers, not secrets) and so aren't
// covered by this at all - only the fields listed in plaintextCredentialFields.
const allowPlaintextCredentialsAnnotation = "freqtrade.io/allow-plaintext-credentials"

// TradeBotConfigCustomValidator validates TradeBotConfig create/update requests.
//
// +kubebuilder:object:generate=false
type TradeBotConfigCustomValidator struct {
	Client client.Client
	// Recorder emits a warning Event when allowPlaintextCredentialsAnnotation
	// is used to admit a plaintext credential field. Nil is fine -
	// warnPlaintextCredentialsUsed skips emitting rather than dereferencing
	// a nil interface.
	Recorder record.EventRecorder
}

// SetupWebhookWithManager registers the TradeBotConfig validating webhook.
func (c *TradeBotConfig) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(c).
		WithValidator(&TradeBotConfigCustomValidator{
			Client:   mgr.GetClient(),
			Recorder: mgr.GetEventRecorderFor("tradebotconfig-webhook"),
		}).
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
	if fields := plaintextCredentialFields(cfg); len(fields) > 0 {
		if cfg.Annotations[allowPlaintextCredentialsAnnotation] != "true" {
			return fmt.Errorf(
				"spec sets deprecated plaintext credential field(s) [%s]; use a secretRef instead, or set the "+
					"%q annotation to \"true\" to override (not recommended: these fields are stored unencrypted "+
					"in etcd and readable by anyone who can get this TradeBotConfig)",
				strings.Join(fields, ", "), allowPlaintextCredentialsAnnotation,
			)
		}
		v.warnPlaintextCredentialsUsed(cfg, fields)
	}

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

// plaintextCredentialFields returns the dotted spec path of every deprecated
// plaintext credential field (P3-1) set to a non-empty value. Exactly the
// fields configbuilder falls back to when a secretRef doesn't supply a
// given value (see exchange.go, bot.go's APIServerConfig block, and
// notification.go's Telegram branch) - not account_id, ChatID, or TopicID,
// which are identifiers rather than secrets and so were never deprecated.
func plaintextCredentialFields(cfg *TradeBotConfig) []string {
	var fields []string
	if e := cfg.Spec.Exchange; e != nil {
		for _, f := range []struct {
			set  bool
			path string
		}{
			{e.Key != "", "spec.exchange.key"},
			{e.Secret != "", "spec.exchange.secret"},
			{e.Password != "", "spec.exchange.password"},
			{e.UID != "", "spec.exchange.uid"},
			{e.WalletAddress != "", "spec.exchange.wallet_address"},
			{e.PrivateKey != "", "spec.exchange.private_key"},
		} {
			if f.set {
				fields = append(fields, f.path)
			}
		}
	}
	if a := cfg.Spec.APIServer; a != nil {
		if a.Password != "" {
			fields = append(fields, "spec.apiServer.password")
		}
		if a.JWTSecretKey != "" {
			fields = append(fields, "spec.apiServer.jwtSecretKey")
		}
	}
	if n := cfg.Spec.Notification; n != nil && n.Telegram != nil && n.Telegram.Token != "" {
		fields = append(fields, "spec.notification.telegram.token")
	}
	return fields
}

// warnPlaintextCredentialsUsed emits the P3-1 warning Event for a
// TradeBotConfig admitted only because allowPlaintextCredentialsAnnotation
// was set.
func (v *TradeBotConfigCustomValidator) warnPlaintextCredentialsUsed(cfg *TradeBotConfig, fields []string) {
	if v.Recorder == nil {
		return
	}
	v.Recorder.Event(cfg, corev1.EventTypeWarning, "PlaintextCredentialsUsed",
		fmt.Sprintf("Admitted via %s: deprecated plaintext credential field(s) [%s] - migrate to secretRef",
			allowPlaintextCredentialsAnnotation, strings.Join(fields, ", ")))
}
