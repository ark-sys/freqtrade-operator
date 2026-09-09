package frequi

import (
	"context"
	"fmt"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	"github.com/ark-sys/freqtrade-operator/controllers/frequi/resources"
	"github.com/ark-sys/freqtrade-operator/controllers/shared"
)

// Reconciler FreqUIReconciler reconciles a FreqUI object
type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// Recorder emits the P4-1 Events below. Nil is fine - not every test
	// constructs one.
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=freqtrade.io,resources=frequis,verbs=get;list;watch
// +kubebuilder:rbac:groups=freqtrade.io,resources=frequis/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=freqtrade.io,resources=tradebots,verbs=get;list;watch
// +kubebuilder:rbac:groups=freqtrade.io,resources=tradebotconfigs,verbs=get;list;watch
// patch (not update) is what server-side apply issues (shared.Apply, P2-1).
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;patch
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;patch
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;patch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile handles the reconciliation loop for FreqUI resources
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.V(1).Info("Reconciling FreqUI", "namespacedName", req.NamespacedName)

	// 1. Fetch FreqUI resource
	var frequi freqtradev1alpha1.FreqUI
	if err := r.Get(ctx, req.NamespacedName, &frequi); err != nil {
		if errors.IsNotFound(err) {
			logger.V(1).Info("FreqUI resource not found, ignoring since it must be deleted")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// 2. Reconcile all resources
	unresolvedRefs, err := r.reconcileAllResources(ctx, &frequi)
	if err != nil {
		logger.Error(err, "Failed to reconcile resources")
		return r.failReconcile(ctx, &frequi, fmt.Sprintf("Failed to reconcile resources: %v", err), 30*time.Second, err)
	}

	// 3. Check if deployment is ready
	var deployment appsv1.Deployment
	if err := r.Get(ctx, types.NamespacedName{Name: frequi.Name, Namespace: frequi.Namespace}, &deployment); err != nil {
		logger.Error(err, "Failed to get deployment status")
		return r.failReconcile(ctx, &frequi, fmt.Sprintf("Failed to get deployment status: %v", err), 30*time.Second, err)
	}

	// 4. Reflect deployment readiness into status.
	ready := isDeploymentReady(&deployment)
	requeueAfter := 15 * time.Second
	message := fmt.Sprintf("Waiting for deployment to be ready (%d/%d replicas)",
		deployment.Status.AvailableReplicas, deployment.Status.Replicas)
	url := ""
	if ready {
		message = "FreqUI deployed successfully"
		url = fmt.Sprintf("http://%s.%s.svc.cluster.local", frequi.Name, frequi.Namespace)
		requeueAfter = 5 * time.Minute
	}

	wasReady := meta.IsStatusConditionTrue(frequi.Status.Conditions, freqtradev1alpha1.ConditionWorkloadReady)

	if err := shared.PatchStatus(ctx, r.Client, &frequi, func() {
		condition := metav1.Condition{Type: freqtradev1alpha1.ConditionWorkloadReady, Message: message}
		if ready {
			condition.Status, condition.Reason = metav1.ConditionTrue, freqtradev1alpha1.ReasonWorkloadHealthy
		} else {
			condition.Status, condition.Reason = metav1.ConditionFalse, freqtradev1alpha1.ReasonWorkloadProgressing
		}
		meta.SetStatusCondition(&frequi.Status.Conditions, condition)
		meta.SetStatusCondition(&frequi.Status.Conditions, metav1.Condition{
			Type: freqtradev1alpha1.ConditionReady, Status: condition.Status, Reason: condition.Reason, Message: message,
		})
		meta.SetStatusCondition(&frequi.Status.Conditions, tradeBotRefsResolvedCondition(unresolvedRefs))
		frequi.Status.Phase = deriveFreqUIPhase(frequi.Status.Conditions)
		frequi.Status.Message = message
		frequi.Status.URL = url
	}); err != nil {
		logger.Error(err, "Failed to update FreqUI status")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, err
	}

	if r.Recorder != nil && !wasReady && ready {
		r.Recorder.Event(&frequi, corev1.EventTypeNormal, "WorkloadReady", "FreqUI deployment is ready")
	}

	logger.V(1).Info("FreqUI reconciliation completed successfully", "name", frequi.Name, "phase", frequi.Status.Phase)
	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

// deriveFreqUIPhase computes the human-facing Phase from Conditions - it is
// never itself the source of truth.
func deriveFreqUIPhase(conditions []metav1.Condition) string {
	c := meta.FindStatusCondition(conditions, freqtradev1alpha1.ConditionWorkloadReady)
	if c == nil {
		return "Initializing"
	}
	switch c.Reason {
	case freqtradev1alpha1.ReasonWorkloadHealthy:
		return "Running"
	case freqtradev1alpha1.ReasonReconcileError:
		return "ResourceError"
	default:
		return "Pending"
	}
}

// tradeBotRefsResolvedCondition builds the TradeBotRefsResolved condition
// (P2-5): non-blocking, since a typo in one TradeBotRef shouldn't fail
// FreqUI's own deployment - it should just be visible instead of silently
// yielding no CORS/API route for that name.
func tradeBotRefsResolvedCondition(unresolvedRefs []string) metav1.Condition {
	if len(unresolvedRefs) == 0 {
		return metav1.Condition{
			Type: freqtradev1alpha1.ConditionTradeBotRefsResolved, Status: metav1.ConditionTrue,
			Reason: freqtradev1alpha1.ReasonAsExpected,
		}
	}
	return metav1.Condition{
		Type: freqtradev1alpha1.ConditionTradeBotRefsResolved, Status: metav1.ConditionFalse,
		Reason: freqtradev1alpha1.ReasonUnresolvableTradeBotRefs,
		Message: fmt.Sprintf("spec.tradeBotRefs entries with no matching TradeBot in this namespace: %s",
			strings.Join(unresolvedRefs, ", ")),
	}
}

// failReconcile sets WorkloadReady and Ready to False with the given
// message, derives Phase from the result, and returns a ctrl.Result:
// requeueAfter is honored when err is nil, otherwise controller-runtime's
// own exponential backoff takes over (its Result is ignored whenever a
// non-nil error is also returned).
func (r *Reconciler) failReconcile(
	ctx context.Context, frequi *freqtradev1alpha1.FreqUI, message string, requeueAfter time.Duration, err error,
) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	if patchErr := shared.PatchStatus(ctx, r.Client, frequi, func() {
		condition := metav1.Condition{
			Type: freqtradev1alpha1.ConditionWorkloadReady, Status: metav1.ConditionFalse,
			Reason: freqtradev1alpha1.ReasonReconcileError, Message: message,
		}
		meta.SetStatusCondition(&frequi.Status.Conditions, condition)
		meta.SetStatusCondition(&frequi.Status.Conditions, metav1.Condition{
			Type: freqtradev1alpha1.ConditionReady, Status: metav1.ConditionFalse,
			Reason: freqtradev1alpha1.ReasonReconcileError, Message: message,
		})
		frequi.Status.Phase = deriveFreqUIPhase(frequi.Status.Conditions)
		frequi.Status.Message = message
	}); patchErr != nil {
		logger.Error(patchErr, "Failed to update FreqUI status")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, patchErr
	}
	if r.Recorder != nil {
		r.Recorder.Event(frequi, corev1.EventTypeWarning, freqtradev1alpha1.ReasonReconcileError, message)
	}
	shared.ReconcileErrorsTotal.WithLabelValues("frequi", freqtradev1alpha1.ReasonReconcileError).Inc()
	return ctrl.Result{RequeueAfter: requeueAfter}, err
}

// reconcileAllResources creates or updates all resources needed by FreqUI.
// The returned slice names any FreqUI.Spec.TradeBotRefs entry that doesn't
// resolve to an actual TradeBot (P2-5) - Reconcile folds it into the
// TradeBotRefsResolved condition. It's nil, not an error: a typo in
// TradeBotRefs means that one bot gets no CORS/API route, not that FreqUI
// itself fails to deploy.
func (r *Reconciler) reconcileAllResources(ctx context.Context, frequi *freqtradev1alpha1.FreqUI) ([]string, error) {
	logger := log.FromContext(ctx)

	// 1. Reconcile Deployment
	deployment := resources.BuildFreqUIDeployment(*frequi)
	if err := shared.Apply(ctx, r.Client, frequi, &deployment); err != nil {
		return nil, fmt.Errorf("failed to apply deployment: %w", err)
	}
	logger.V(2).Info("Deployment reconciled", "name", deployment.Name)

	// 2. Reconcile Service
	service := resources.BuildFreqUIService(*frequi)
	if err := shared.Apply(ctx, r.Client, frequi, &service); err != nil {
		return nil, fmt.Errorf("failed to apply service: %w", err)
	}
	logger.V(2).Info("Service reconciled", "name", service.Name)

	// 3. Reconcile Ingress if configured
	var apiRoutes []resources.TradeBotAPIRoute
	var unresolvedRefs []string
	for _, tradeBotRef := range frequi.Spec.TradeBotRefs {
		var tradeBot freqtradev1alpha1.TradeBot
		err := r.Get(ctx, types.NamespacedName{Name: tradeBotRef, Namespace: frequi.Namespace}, &tradeBot)
		switch {
		case errors.IsNotFound(err):
			logger.V(1).Info("TradeBotRef does not resolve to any TradeBot in this namespace", "tradeBot", tradeBotRef)
			unresolvedRefs = append(unresolvedRefs, tradeBotRef)
			continue
		case err != nil:
			logger.Error(err, "Failed to get TradeBot for API route", "tradeBot", tradeBotRef)
			return nil, fmt.Errorf("failed to get TradeBot %s: %w", tradeBotRef, err)
		}

		// Retrieve TradeBotConfig for the TradeBot
		var tradeBotConfig freqtradev1alpha1.TradeBotConfig
		if err := r.Get(ctx, types.NamespacedName{Name: tradeBot.Spec.Config, Namespace: frequi.Namespace}, &tradeBotConfig); err != nil {
			logger.Error(err, "Failed to get TradeBotConfig for TradeBot", "tradeBot", tradeBotRef)
			continue
		}

		// Only add API route if TradeBotConfig is valid and API server is enabled
		if tradeBotConfig.Status.Phase == "Valid" && tradeBotConfig.Spec.APIServer != nil &&
			tradeBotConfig.Spec.APIServer.Enabled != nil && *tradeBotConfig.Spec.APIServer.Enabled {
			apiRoutes = append(apiRoutes, resources.TradeBotAPIRoute{
				Name:        tradeBotRef,
				ServiceName: tradeBotRef,
				PathPrefix:  tradeBotRef,
			})
		} else {
			logger.V(2).Info("Skipping TradeBot API route", "tradeBot", tradeBotRef,
				"reason", "TradeBotConfig is not valid or API server is disabled")
		}
	}

	ingress := resources.BuildFreqUIIngress(*frequi, apiRoutes)
	if err := shared.Apply(ctx, r.Client, frequi, &ingress); err != nil {
		return nil, fmt.Errorf("failed to apply ingress: %w", err)
	}
	logger.V(2).Info("Ingress reconciled", "name", ingress.Name)

	return unresolvedRefs, nil
}

// isDeploymentReady checks if a deployment is ready
func isDeploymentReady(deployment *appsv1.Deployment) bool {
	return deployment.Status.AvailableReplicas == deployment.Status.Replicas &&
		deployment.Status.Replicas > 0
}
