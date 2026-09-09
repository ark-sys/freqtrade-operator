package tradebotconfig

import (
	"context"
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

// Reconciler reconciles a Tradebotconfig object
type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// Recorder emits the P4-1 ValidationFailed Event. Nil is fine (see
	// Reconcile) - not every test constructs one.
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=freqtrade.io,resources=tradebotconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=freqtrade.io,resources=tradebotconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=freqtrade.io,resources=tradebots,verbs=list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile handles the reconciliation loop for Tradebotconfig resources
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.V(1).Info("Reconciling TradeBotConfig", "namespacedName", req.NamespacedName)

	// Fetch Tradebotconfig resource
	var tradebotconfig freqtradev1alpha1.TradeBotConfig
	if err := r.Get(ctx, req.NamespacedName, &tradebotconfig); err != nil {
		if errors.IsNotFound(err) {
			logger.V(1).Info("Tradebotconfig resource not found, ignoring since it must be deleted")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// TradeBotConfig has no external references and no owned workload, so
	// validation is a pure, synchronous function of its own spec: Ready is
	// the only condition it ever sets.
	validateErr := r.validateTradeBotConfig(ctx, &tradebotconfig)

	if err := shared.PatchStatus(ctx, r.Client, &tradebotconfig, func() {
		condition := metav1.Condition{
			Type:    freqtradev1alpha1.ConditionReady,
			Status:  metav1.ConditionTrue,
			Reason:  freqtradev1alpha1.ReasonAsExpected,
			Message: "TradeBotConfig configuration is valid",
		}
		if validateErr != nil {
			condition.Status = metav1.ConditionFalse
			condition.Reason = freqtradev1alpha1.ReasonConfigInvalid
			condition.Message = validateErr.Error()
		}
		meta.SetStatusCondition(&tradebotconfig.Status.Conditions, condition)
		tradebotconfig.Status.Phase = deriveTradeBotConfigPhase(tradebotconfig.Status.Conditions)
	}); err != nil {
		logger.Error(err, "Failed to update TradeBotConfig status")
		return ctrl.Result{RequeueAfter: 10 * time.Second}, err
	}

	if validateErr != nil {
		logger.V(1).Error(validateErr, "TradeBotConfig validation failed")
		if r.Recorder != nil {
			r.Recorder.Event(&tradebotconfig, corev1.EventTypeWarning, "ValidationFailed", validateErr.Error())
		}
		shared.ReconcileErrorsTotal.WithLabelValues("tradebotconfig", "ValidationFailed").Inc()
		// Nothing to retry: re-validating an unchanged spec always
		// produces the same result. The next reconcile the user's own edit
		// triggers is what can change the outcome.
		return ctrl.Result{}, nil
	}

	logger.V(1).Info("Tradebotconfig reconciliation completed successfully", "name", tradebotconfig.Name)
	return ctrl.Result{}, nil
}

// deriveTradeBotConfigPhase computes the human-facing Phase from Conditions
// - it is never itself the source of truth.
func deriveTradeBotConfigPhase(conditions []metav1.Condition) string {
	ready := meta.FindStatusCondition(conditions, freqtradev1alpha1.ConditionReady)
	if ready == nil {
		return "Validating"
	}
	if ready.Status == metav1.ConditionTrue {
		return "Valid"
	}
	return "Invalid"
}

// validateTradebotconfig performs validation of Tradebotconfig configuration
func (r *Reconciler) validateTradeBotConfig(ctx context.Context, tradebotconfig *freqtradev1alpha1.TradeBotConfig) error {
	logger := log.FromContext(ctx)

	//// Validate required fields
	//if tradebotconfig.Spec.Bot.BotName == "" {
	//	return fmt.Errorf("tradebotconfig name is required")
	//}

	// TODO Check other required fields

	logger.V(2).Info("Tradebotconfig validation passed", "name", tradebotconfig.Name)
	return nil
}
