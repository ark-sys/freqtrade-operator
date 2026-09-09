package v1alpha1

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/validate-freqtrade-io-v1alpha1-strategy,mutating=false,failurePolicy=fail,sideEffects=None,groups=freqtrade.io,resources=strategies,verbs=create;update,versions=v1alpha1,name=vstrategy.kb.io,admissionReviewVersions=v1

// StrategyCustomValidator validates Strategy create/update requests.
//
// +kubebuilder:object:generate=false
type StrategyCustomValidator struct{}

// SetupWebhookWithManager registers the Strategy validating webhook.
func (s *Strategy) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(s).
		WithValidator(&StrategyCustomValidator{}).
		Complete()
}

// ValidateCreate implements admission.CustomValidator.
func (v *StrategyCustomValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	strategy, ok := obj.(*Strategy)
	if !ok {
		return nil, fmt.Errorf("expected a Strategy but got %T", obj)
	}
	return nil, validateStrategySpec(strategy)
}

// ValidateUpdate implements admission.CustomValidator.
func (v *StrategyCustomValidator) ValidateUpdate(
	_ context.Context, _, newObj runtime.Object,
) (admission.Warnings, error) {
	strategy, ok := newObj.(*Strategy)
	if !ok {
		return nil, fmt.Errorf("expected a Strategy but got %T", newObj)
	}
	return nil, validateStrategySpec(strategy)
}

// ValidateDelete implements admission.CustomValidator. Deletion is never
// rejected.
func (v *StrategyCustomValidator) ValidateDelete(context.Context, runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// validateStrategySpec is the Python-shape check the controller (to set its
// Ready condition) and this webhook (to reject synchronously) both need -
// consolidated here so there's exactly one copy of the orchestration; the
// leaf checks below are exported so controllers/strategy can call them too.
func validateStrategySpec(strategy *Strategy) error {
	if strategy.Spec.Name == "" {
		return fmt.Errorf("spec.name is required")
	}
	if strategy.Spec.Script == "" {
		return fmt.Errorf("spec.script is required")
	}
	if !IsValidPythonClassName(strategy.Spec.Name) {
		return fmt.Errorf("spec.name %q is not a valid Python class name", strategy.Spec.Name)
	}
	return ValidateStrategyScript(strategy.Spec.Script, strategy.Spec.Name)
}

// ValidateStrategyScript performs a basic shape check of a strategy script
// against what Freqtrade expects from an IStrategy subclass.
func ValidateStrategyScript(script, strategyName string) error {
	if strings.TrimSpace(script) == "" {
		return fmt.Errorf("strategy script cannot be empty")
	}

	expectedClassDef := fmt.Sprintf("class %s", strategyName)
	if !strings.Contains(script, expectedClassDef) {
		return fmt.Errorf("strategy script must contain class definition: %s", expectedClassDef)
	}

	requiredMethods := []string{"populate_indicators", "populate_entry_trend", "populate_exit_trend"}
	for _, method := range requiredMethods {
		if !strings.Contains(script, fmt.Sprintf("def %s", method)) {
			return fmt.Errorf("strategy script must contain method: %s", method)
		}
	}

	requiredImports := []string{"import freqtrade", "from freqtrade.strategy"}
	hasRequiredImport := false
	for _, importStmt := range requiredImports {
		if strings.Contains(script, importStmt) {
			hasRequiredImport = true
			break
		}
	}
	if !hasRequiredImport {
		return fmt.Errorf("strategy script must contain freqtrade imports")
	}

	return nil
}

// IsValidPythonClassName reports whether name is a syntactically valid
// Python class identifier: a letter or underscore, followed by any number of
// letters, digits, or underscores.
func IsValidPythonClassName(name string) bool {
	if name == "" {
		return false
	}

	first := name[0]
	validStart := (first >= 'A' && first <= 'Z') || (first >= 'a' && first <= 'z') || first == '_'
	if !validStart {
		return false
	}

	for _, char := range name[1:] {
		validRest := (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') ||
			(char >= '0' && char <= '9') || char == '_'
		if !validRest {
			return false
		}
	}

	return true
}
