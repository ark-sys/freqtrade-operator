package v1beta1

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// allowExtraArgsAnnotation gates RunSpec.ExtraArgs (D8): a live bot's argv
// is not a place for improvisation (TradeBot never gets this field at all),
// but a one-shot run's typed surface can legitimately lag freqtrade's own
// flag set - this annotation is the deliberate, visible opt-in for that gap,
// not a default-on escape hatch.
const allowExtraArgsAnnotation = "freqtrade.io/allow-extra-args"

// denylistedExtraArgs are flags this operator itself controls (the config
// path, strategy, database, logging, and directory layout every run is
// built around) - extraArgs may never override them, annotation or not.
var denylistedExtraArgs = []string{
	"--config", "--strategy", "--strategy-path", "--db-url", "--logfile", "--userdir", "--datadir",
}

// shellMetacharacters are rejected from any extraArgs value even though
// today's Job pod exec's freqtrade directly (no shell wrapper, see
// controllers/backtest/resources/pod.go) - defense in depth per D8, so a
// future change introducing a shell wrapper can't turn this into an
// injection vector by surprise.
const shellMetacharacters = ";&|$`<>(){}\n"

// +kubebuilder:webhook:path=/validate-freqtrade-io-v1beta1-backtest,mutating=false,failurePolicy=fail,sideEffects=None,groups=freqtrade.io,resources=backtests,verbs=create;update,versions=v1beta1,name=vbacktest.kb.io,admissionReviewVersions=v1

// BacktestCustomValidator validates Backtest create/update requests.
//
// +kubebuilder:object:generate=false
type BacktestCustomValidator struct{}

// SetupWebhookWithManager registers the Backtest validating webhook.
func (b *Backtest) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(b).
		WithValidator(&BacktestCustomValidator{}).
		Complete()
}

// ValidateCreate implements admission.CustomValidator.
func (v *BacktestCustomValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	backtest, ok := obj.(*Backtest)
	if !ok {
		return nil, fmt.Errorf("expected a Backtest but got %T", obj)
	}
	return nil, validateBacktestSpec(backtest)
}

// ValidateUpdate implements admission.CustomValidator. Spec is immutable
// (CEL self == oldSelf on BacktestSpec already rejects any spec change), so
// this only ever re-validates a spec that was already accepted at create -
// kept for defense in depth rather than assuming CEL alone is enough.
func (v *BacktestCustomValidator) ValidateUpdate(
	_ context.Context, _, newObj runtime.Object,
) (admission.Warnings, error) {
	backtest, ok := newObj.(*Backtest)
	if !ok {
		return nil, fmt.Errorf("expected a Backtest but got %T", newObj)
	}
	return nil, validateBacktestSpec(backtest)
}

// ValidateDelete implements admission.CustomValidator. Deletion is never
// rejected.
func (v *BacktestCustomValidator) ValidateDelete(context.Context, runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func validateBacktestSpec(backtest *Backtest) error {
	if strings.TrimSpace(backtest.Spec.ConfigRef.Name) == "" {
		return fmt.Errorf("spec.configRef.name is required")
	}
	if strings.TrimSpace(backtest.Spec.StrategyRef.Name) == "" {
		return fmt.Errorf("spec.strategyRef.name is required")
	}
	return validateExtraArgs(backtest)
}

// validateExtraArgs enforces D8: extraArgs requires an explicit opt-in
// annotation, and can never override a flag this operator itself sets.
func validateExtraArgs(backtest *Backtest) error {
	if len(backtest.Spec.ExtraArgs) == 0 {
		return nil
	}
	if backtest.Annotations[allowExtraArgsAnnotation] != "true" {
		return fmt.Errorf(
			"spec.extraArgs is set but the %q annotation is not \"true\" - "+
				"extraArgs is a deliberate escape hatch for flags the typed spec doesn't cover yet, "+
				"not a default-on surface", allowExtraArgsAnnotation,
		)
	}
	for _, arg := range backtest.Spec.ExtraArgs {
		if strings.ContainsAny(arg, shellMetacharacters) {
			return fmt.Errorf("spec.extraArgs value %q contains a disallowed shell metacharacter", arg)
		}
		for _, denied := range denylistedExtraArgs {
			if arg == denied || strings.HasPrefix(arg, denied+"=") {
				return fmt.Errorf("spec.extraArgs may not set %q - this operator manages it directly", denied)
			}
		}
	}
	return nil
}
