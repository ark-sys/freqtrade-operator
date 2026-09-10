package v1alpha1

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/validate-freqtrade-io-v1alpha1-tradebotconfig,mutating=false,failurePolicy=fail,sideEffects=None,groups=freqtrade.io,resources=tradebotconfigs,verbs=create;update,versions=v1alpha1,name=vtradebotconfig.kb.io,admissionReviewVersions=v1

// allowPlaintextCredentialsAnnotation opts a TradeBotConfig out of P3-1's
// plaintext-credential rejection. account_id and Telegram's ChatID/TopicID
// aren't credentials (they're identifiers, not secrets) and so aren't
// covered by this at all - only the fields listed in plaintextCredentialFields.
const allowPlaintextCredentialsAnnotation = "freqtrade.io/allow-plaintext-credentials"

// TradeBotConfigCustomValidator validates TradeBotConfig create/update
// requests. B2 (REMAINING-WORK.md) moved everything this webhook used to
// validate structurally (exchange required, secretRef must resolve to a
// Secret with a recognized key, dry_run requirement) to
// api/v1beta1/tradebotconfig_webhook.go - v1beta1 has no plaintext
// credential fields to gate at all, so that validator doesn't need this
// one's logic, and this one doesn't need Client access anymore. What
// remains here - the P3-1 plaintext-credential annotation gate - only
// applies to v1alpha1 objects and stays as long as v1alpha1 is served.
//
// +kubebuilder:object:generate=false
type TradeBotConfigCustomValidator struct {
	// Recorder emits a warning Event when allowPlaintextCredentialsAnnotation
	// is used to admit a plaintext credential field. Nil is fine -
	// warnPlaintextCredentialsUsed skips emitting rather than dereferencing
	// a nil interface.
	Recorder record.EventRecorder
}

// SetupWebhookWithManager registers the v1alpha1 TradeBotConfig validating webhook.
func (c *TradeBotConfig) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(c).
		WithValidator(&TradeBotConfigCustomValidator{
			Recorder: mgr.GetEventRecorderFor("tradebotconfig-webhook"),
		}).
		Complete()
}

// ValidateCreate implements admission.CustomValidator.
func (v *TradeBotConfigCustomValidator) ValidateCreate(
	_ context.Context, obj runtime.Object,
) (admission.Warnings, error) {
	cfg, ok := obj.(*TradeBotConfig)
	if !ok {
		return nil, fmt.Errorf("expected a TradeBotConfig but got %T", obj)
	}
	return nil, v.validate(cfg)
}

// ValidateUpdate implements admission.CustomValidator. Skips validation once DeletionTimestamp
// is set - same deadlock risk, and same fix, as TradeBotCustomValidator.ValidateUpdate (see its
// doc comment).
func (v *TradeBotConfigCustomValidator) ValidateUpdate(
	_ context.Context, _, newObj runtime.Object,
) (admission.Warnings, error) {
	cfg, ok := newObj.(*TradeBotConfig)
	if !ok {
		return nil, fmt.Errorf("expected a TradeBotConfig but got %T", newObj)
	}
	if cfg.DeletionTimestamp != nil {
		return nil, nil
	}
	return nil, v.validate(cfg)
}

// ValidateDelete implements admission.CustomValidator. Deletion is never
// rejected.
func (v *TradeBotConfigCustomValidator) ValidateDelete(context.Context, runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// validate rejects a TradeBotConfig that sets a deprecated plaintext
// credential field without allowPlaintextCredentialsAnnotation.
func (v *TradeBotConfigCustomValidator) validate(cfg *TradeBotConfig) error {
	fields := plaintextCredentialFields(cfg)
	if len(fields) == 0 {
		return nil
	}
	if cfg.Annotations[allowPlaintextCredentialsAnnotation] != "true" {
		return fmt.Errorf(
			"spec sets deprecated plaintext credential field(s) [%s]; use a secretRef instead. Setting the %q "+
				"annotation only bypasses this specific check - v1beta1 (the storage version, B2) has no field "+
				"for these at all, so the write is still rejected at conversion regardless of this annotation; "+
				"there is no way to persist a plaintext credential",
			strings.Join(fields, ", "), allowPlaintextCredentialsAnnotation,
		)
	}
	v.warnPlaintextCredentialsUsed(cfg, fields)
	return nil
}

// plaintextCredentialFields returns the dotted spec path of every deprecated
// plaintext credential field (P3-1) set to a non-empty value. Exactly the
// fields configbuilder falls back to when a secretRef doesn't supply a
// given value (see exchange.go, bot.go's APIServerConfig block, and
// notification.go's Telegram branch) - not account_id, ChatID, or TopicID,
// which are identifiers rather than secrets and so were never deprecated.
// Also used by tradebotconfig_conversion.go's ConvertTo, which rejects
// converting any of these to v1beta1 rather than silently dropping them.
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
