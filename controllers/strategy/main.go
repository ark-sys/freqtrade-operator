package strategy

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	"github.com/ark-sys/freqtrade-operator/controllers/shared"
)

// Reconciler reconciles a Strategy object
type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// Recorder emits the P4-1 ValidationFailed Event. Nil is fine (see
	// recordValidationFailed) - not every test constructs one.
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=freqtrade.io,resources=strategies,verbs=get;list;watch
// +kubebuilder:rbac:groups=freqtrade.io,resources=strategies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=freqtrade.io,resources=tradebots,verbs=list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile handles the reconciliation loop for Strategy resources
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.V(1).Info("Reconciling Strategy", "namespacedName", req.NamespacedName)

	// Fetch Strategy resource
	var strategy freqtradev1alpha1.Strategy
	if err := r.Get(ctx, req.NamespacedName, &strategy); err != nil {
		if errors.IsNotFound(err) {
			logger.V(1).Info("Strategy resource not found, ignoring since it must be deleted")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Strategy has no external references and no owned workload, so
	// validation is a pure, synchronous function of its own spec: Ready is
	// the only condition it ever sets.
	validateErr := r.validateStrategy(ctx, &strategy)

	if err := shared.PatchStatus(ctx, r.Client, &strategy, func() {
		condition := metav1.Condition{
			Type:    freqtradev1alpha1.ConditionReady,
			Status:  metav1.ConditionTrue,
			Reason:  freqtradev1alpha1.ReasonAsExpected,
			Message: "Strategy configuration is valid",
		}
		if validateErr != nil {
			condition.Status = metav1.ConditionFalse
			condition.Reason = freqtradev1alpha1.ReasonConfigInvalid
			condition.Message = validateErr.Error()
		}
		meta.SetStatusCondition(&strategy.Status.Conditions, condition)
		strategy.Status.Phase = deriveStrategyPhase(strategy.Status.Conditions)
	}); err != nil {
		logger.Error(err, "Failed to update Strategy status")
		return ctrl.Result{RequeueAfter: 10 * time.Second}, err
	}

	if validateErr != nil {
		logger.V(1).Error(validateErr, "Strategy validation failed")
		if r.Recorder != nil {
			r.Recorder.Event(&strategy, corev1.EventTypeWarning, "ValidationFailed", validateErr.Error())
		}
		// Nothing to retry: re-validating an unchanged spec always
		// produces the same result. The next reconcile the user's own edit
		// triggers is what can change the outcome.
		return ctrl.Result{}, nil
	}

	logger.V(1).Info("Strategy reconciliation completed successfully", "name", strategy.Name)
	return ctrl.Result{}, nil
}

// deriveStrategyPhase computes the human-facing Phase from Conditions - it
// is never itself the source of truth.
func deriveStrategyPhase(conditions []metav1.Condition) string {
	ready := meta.FindStatusCondition(conditions, freqtradev1alpha1.ConditionReady)
	if ready == nil {
		return "Validating"
	}
	if ready.Status == metav1.ConditionTrue {
		return "Valid"
	}
	return "Invalid"
}

// validateStrategy performs validation of Strategy configuration
func (r *Reconciler) validateStrategy(ctx context.Context, strategy *freqtradev1alpha1.Strategy) error {
	logger := log.FromContext(ctx)

	// Validate required fields
	if strategy.Spec.Name == "" {
		return fmt.Errorf("strategy name is required")
	}

	if strategy.Spec.Script == "" {
		return fmt.Errorf("strategy script is required")
	}

	// Validate strategy name format (should be valid Python class name)
	if !freqtradev1alpha1.IsValidPythonClassName(strategy.Spec.Name) {
		return fmt.Errorf("strategy name '%s' is not a valid Python class name", strategy.Spec.Name)
	}

	// Basic script validation - same shape check the admission webhook runs
	// synchronously (P1-4); this copy is what backs the Ready condition, and
	// is kept independent so an object that predates the webhook, or a
	// reconcile that runs while it's unavailable, still gets validated.
	if err := freqtradev1alpha1.ValidateStrategyScript(strategy.Spec.Script, strategy.Spec.Name); err != nil {
		return fmt.Errorf("strategy script validation failed: %w", err)
	}

	logger.V(2).Info("Strategy validation passed", "name", strategy.Name)
	return nil
}
