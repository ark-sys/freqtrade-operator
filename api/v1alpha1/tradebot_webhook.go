package v1alpha1

import (
	"context"
	"fmt"
	"regexp"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/validate-freqtrade-io-v1alpha1-tradebot,mutating=false,failurePolicy=fail,sideEffects=None,groups=freqtrade.io,resources=tradebots,verbs=create;update,versions=v1alpha1,name=vtradebot.kb.io,admissionReviewVersions=v1

// minIntrospectionInterval is the floor the webhook enforces on
// spec.introspection.interval (P4-3).
const minIntrospectionInterval = 10 * time.Second

// ownedFreqtradeFlags are argv flags BuildPod (controllers/tradebot/resources/pod.go)
// already sets from the TradeBot/Strategy spec. freqtrade's argparse takes the
// last occurrence of a repeated flag, and freqtradeArguments is appended after
// these, so letting one through would let it silently override - e.g. a
// smuggled "--strategy" would run a different strategy than spec.strategy names.
var ownedFreqtradeFlags = map[string]struct{}{
	"--config":        {},
	"--strategy":      {},
	"--strategy-path": {},
	"--db-url":        {},
	"--logfile":       {},
	"--userdir":       {},
	"--datadir":       {},
}

// shellMetacharacters is a conservative denylist for freqtradeArguments
// elements. BuildPod passes these as literal argv, never through a shell, so
// this isn't shell injection on the current code path - it's defense in
// depth against a future path that does, and against other tooling that
// might render these strings unsafely.
var shellMetacharacters = regexp.MustCompile("[;&|`$(){}<>\\\\\n]")

// TradeBotCustomValidator validates TradeBot create/update requests.
//
// +kubebuilder:object:generate=false
type TradeBotCustomValidator struct {
	Client client.Client
}

// SetupWebhookWithManager registers the TradeBot validating webhook.
func (t *TradeBot) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(t).
		WithValidator(&TradeBotCustomValidator{Client: mgr.GetClient()}).
		Complete()
}

// ValidateCreate implements admission.CustomValidator.
func (v *TradeBotCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	tradeBot, ok := obj.(*TradeBot)
	if !ok {
		return nil, fmt.Errorf("expected a TradeBot but got %T", obj)
	}
	return nil, v.validate(ctx, tradeBot)
}

// ValidateUpdate implements admission.CustomValidator.
func (v *TradeBotCustomValidator) ValidateUpdate(
	ctx context.Context, _, newObj runtime.Object,
) (admission.Warnings, error) {
	tradeBot, ok := newObj.(*TradeBot)
	if !ok {
		return nil, fmt.Errorf("expected a TradeBot but got %T", newObj)
	}
	return nil, v.validate(ctx, tradeBot)
}

// ValidateDelete implements admission.CustomValidator. Deletion is never
// rejected: there is nothing about deleting a TradeBot that this operator
// needs to guard against.
func (v *TradeBotCustomValidator) ValidateDelete(context.Context, runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// validate rejects a TradeBot whose Strategy/TradeBotConfig references don't
// resolve, and whose freqtrade_arguments try to smuggle a shell
// metacharacter or repeat a flag BuildPod already owns. References are
// same-namespace only (D3): Config/Strategy are bare names resolved in
// tradeBot.Namespace, with no cross-namespace syntax at all, so "does an
// object with this name exist here" already is the whole check.
func (v *TradeBotCustomValidator) validate(ctx context.Context, tradeBot *TradeBot) error {
	var strategy Strategy
	strategyKey := types.NamespacedName{Name: tradeBot.Spec.Strategy, Namespace: tradeBot.Namespace}
	if err := v.Client.Get(ctx, strategyKey, &strategy); err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("spec.strategy: Strategy %q not found in namespace %q",
				tradeBot.Spec.Strategy, tradeBot.Namespace)
		}
		return fmt.Errorf("spec.strategy: failed to look up Strategy %q: %w", tradeBot.Spec.Strategy, err)
	}

	var tradeBotConfig TradeBotConfig
	configKey := types.NamespacedName{Name: tradeBot.Spec.Config, Namespace: tradeBot.Namespace}
	if err := v.Client.Get(ctx, configKey, &tradeBotConfig); err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("spec.config: TradeBotConfig %q not found in namespace %q",
				tradeBot.Spec.Config, tradeBot.Namespace)
		}
		return fmt.Errorf("spec.config: failed to look up TradeBotConfig %q: %w", tradeBot.Spec.Config, err)
	}

	for _, arg := range tradeBot.Spec.FreqtradeArguments {
		if shellMetacharacters.MatchString(arg) {
			return fmt.Errorf("spec.freqtrade_arguments: %q contains a shell metacharacter, which is not allowed", arg)
		}
		if _, owned := ownedFreqtradeFlags[arg]; owned {
			return fmt.Errorf("spec.freqtrade_arguments: %q is set by the operator and cannot be overridden", arg)
		}
	}

	// freqtrade's REST API isn't built for tight polling loops, and this
	// operator is not a market-data source (P4-3) - reject before a typo'd
	// "6s" (meant "60s") turns into a de facto load test.
	if tradeBot.Spec.Introspection != nil && tradeBot.Spec.Introspection.Interval.Duration != 0 &&
		tradeBot.Spec.Introspection.Interval.Duration < minIntrospectionInterval {
		return fmt.Errorf("spec.introspection.interval: must be at least %s, got %s",
			minIntrospectionInterval, tradeBot.Spec.Introspection.Interval.Duration)
	}

	return nil
}
