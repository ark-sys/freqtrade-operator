package frequi

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	freqtradev1beta1 "github.com/ark-sys/freqtrade-operator/api/v1beta1"
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

	// GatewayAPIAvailable reports whether this cluster serves
	// gateway.networking.k8s.io/v1 HTTPRoute (G3-1, shared.GatewayAPIAvailable),
	// checked once at operator startup - installing Gateway API afterwards
	// needs an operator restart to be noticed. Reconciling a Gateway-mode
	// FreqUI while this is false is G2-2's job (not yet implemented);
	// Ingress-mode FreqUIs are unaffected either way.
	GatewayAPIAvailable bool
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
	var frequi freqtradev1beta1.FreqUI
	if err := r.Get(ctx, req.NamespacedName, &frequi); err != nil {
		if kerrors.IsNotFound(err) {
			logger.V(1).Info("FreqUI resource not found, ignoring since it must be deleted")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// 2. Reconcile all resources
	unresolvedRefs, exposureCond, err := r.reconcileAllResources(ctx, &frequi)
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
		if exposureCond != nil {
			meta.SetStatusCondition(&frequi.Status.Conditions, *exposureCond)
		}
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
	ctx context.Context, frequi *freqtradev1beta1.FreqUI, message string, requeueAfter time.Duration, err error,
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
// itself fails to deploy. The returned *metav1.Condition, when non-nil, is
// folded into ExposureReady (G2-2/G4-1) - nil means this call had nothing
// exposure-specific worth reporting yet, not that exposure is healthy.
func (r *Reconciler) reconcileAllResources(
	ctx context.Context, frequi *freqtradev1beta1.FreqUI,
) ([]string, *metav1.Condition, error) {
	logger := log.FromContext(ctx)

	// 1. Reconcile Deployment
	deployment := resources.BuildFreqUIDeployment(*frequi)
	if err := shared.Apply(ctx, r.Client, frequi, &deployment); err != nil {
		return nil, nil, fmt.Errorf("failed to apply deployment: %w", err)
	}
	logger.V(2).Info("Deployment reconciled", "name", deployment.Name)

	// 2. Reconcile Service
	service := resources.BuildFreqUIService(*frequi)
	if err := shared.Apply(ctx, r.Client, frequi, &service); err != nil {
		return nil, nil, fmt.Errorf("failed to apply service: %w", err)
	}
	logger.V(2).Info("Service reconciled", "name", service.Name)

	// 3. Resolve TradeBotRefs into API routes - shared by every exposure mode.
	var apiRoutes []resources.TradeBotAPIRoute
	var unresolvedRefs []string
	for _, ref := range frequi.Spec.TradeBotRefs {
		tradeBotRef := ref.Name
		var tradeBot freqtradev1alpha1.TradeBot
		err := r.Get(ctx, types.NamespacedName{Name: tradeBotRef, Namespace: frequi.Namespace}, &tradeBot)
		switch {
		case kerrors.IsNotFound(err):
			logger.V(1).Info("TradeBotRef does not resolve to any TradeBot in this namespace", "tradeBot", tradeBotRef)
			unresolvedRefs = append(unresolvedRefs, tradeBotRef)
			continue
		case err != nil:
			logger.Error(err, "Failed to get TradeBot for API route", "tradeBot", tradeBotRef)
			return nil, nil, fmt.Errorf("failed to get TradeBot %s: %w", tradeBotRef, err)
		}

		// Retrieve TradeBotConfig for the TradeBot
		var tradeBotConfig freqtradev1alpha1.TradeBotConfig
		configKey := types.NamespacedName{Name: tradeBot.Spec.Config, Namespace: frequi.Namespace}
		if err := r.Get(ctx, configKey, &tradeBotConfig); err != nil {
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

	// 4. Reconcile exposure - Ingress, HTTPRoutes, or neither, per spec.exposure (D2/G2-2).
	exposureCond, err := r.reconcileExposure(ctx, frequi, apiRoutes)
	if err != nil {
		return nil, nil, err
	}

	return unresolvedRefs, exposureCond, nil
}

// reconcileExposure implements D2's three spec.exposure modes. Ingress and
// None both prune any leftover HTTPRoutes from a previous Gateway-mode
// setting; Gateway prunes the Ingress instead. Only the two failure cases
// G2-2 calls out (Gateway API absent, a name collision with another
// FreqUI's HTTPRoute/Ingress) produce a non-nil condition - deriving
// ExposureReady from actual route/Ingress status is G4-1's job.
func (r *Reconciler) reconcileExposure(
	ctx context.Context, frequi *freqtradev1beta1.FreqUI, apiRoutes []resources.TradeBotAPIRoute,
) (*metav1.Condition, error) {
	logger := log.FromContext(ctx)

	switch frequi.Spec.Exposure {
	case freqtradev1beta1.FUExposureGateway:
		if !r.GatewayAPIAvailable {
			logger.V(1).Info("spec.exposure is Gateway but this cluster does not serve Gateway API - skipping route creation")
			return &metav1.Condition{
				Type: freqtradev1alpha1.ConditionExposureReady, Status: metav1.ConditionFalse,
				Reason: freqtradev1alpha1.ReasonGatewayAPINotInstalled,
				Message: "spec.exposure is Gateway, but this cluster does not serve gateway.networking.k8s.io/v1 " +
					"HTTPRoute. Install Gateway API and restart the operator - availability is only checked at startup.",
			}, nil
		}

		routes, skippedRoutes := resources.BuildFreqUIHTTPRoutes(*frequi, apiRoutes)
		desired := make(map[string]bool, len(routes))
		var conflictNames []string
		for i := range routes {
			route := routes[i]
			if err := shared.Apply(ctx, r.Client, frequi, &route); err != nil {
				var alreadyOwned *controllerutil.AlreadyOwnedError
				if errors.As(err, &alreadyOwned) {
					logger.Error(err, "HTTPRoute name collides with an object owned by another FreqUI", "name", route.Name)
					conflictNames = append(conflictNames, route.Name)
					continue
				}
				return nil, fmt.Errorf("failed to apply HTTPRoute %s: %w", route.Name, err)
			}
			desired[route.Name] = true
			logger.V(2).Info("HTTPRoute reconciled", "name", route.Name)
		}

		if err := r.pruneHTTPRoutes(ctx, frequi, desired); err != nil {
			return nil, err
		}
		if err := r.deleteOwnedIngress(ctx, frequi); err != nil {
			return nil, err
		}

		if len(conflictNames) > 0 || len(skippedRoutes) > 0 {
			return &metav1.Condition{
				Type: freqtradev1alpha1.ConditionExposureReady, Status: metav1.ConditionFalse,
				Reason: freqtradev1alpha1.ReasonRouteNameConflict,
				Message: fmt.Sprintf("could not reconcile every HTTPRoute: name conflicts=%v, over-long names=%v",
					conflictNames, skippedRoutes),
			}, nil
		}
		return nil, nil

	case freqtradev1beta1.FUExposureNone:
		if err := r.pruneHTTPRoutes(ctx, frequi, nil); err != nil {
			return nil, err
		}
		if err := r.deleteOwnedIngress(ctx, frequi); err != nil {
			return nil, err
		}
		return nil, nil

	default: // "" and FUExposureIngress - today's behaviour, unchanged.
		ingress := resources.BuildFreqUIIngress(*frequi, apiRoutes)
		if err := shared.Apply(ctx, r.Client, frequi, &ingress); err != nil {
			var alreadyOwned *controllerutil.AlreadyOwnedError
			if errors.As(err, &alreadyOwned) {
				logger.Error(err, "Ingress name collides with an object owned by another FreqUI", "name", ingress.Name)
				return &metav1.Condition{
					Type: freqtradev1alpha1.ConditionExposureReady, Status: metav1.ConditionFalse,
					Reason:  freqtradev1alpha1.ReasonRouteNameConflict,
					Message: fmt.Sprintf("could not apply Ingress %s: %v", ingress.Name, err),
				}, nil
			}
			return nil, fmt.Errorf("failed to apply ingress: %w", err)
		}
		logger.V(2).Info("Ingress reconciled", "name", ingress.Name)

		if err := r.pruneHTTPRoutes(ctx, frequi, nil); err != nil {
			return nil, err
		}
		return nil, nil
	}
}

// pruneHTTPRoutes deletes every HTTPRoute carrying this FreqUI's ownership
// label that both (a) is not in desired and (b) is actually controller-owned
// by this FreqUI's UID - the label is user-writable (spec.gateway.labels
// can't override it, but a hand-crafted object could still carry it), the
// controller ref is the real ownership claim. desired may be nil (Ingress
// and None modes prune everything). A no-op when Gateway API isn't
// installed: List would otherwise fail with "no matches for kind" for a CRD
// that was never in the cluster to begin with.
func (r *Reconciler) pruneHTTPRoutes(
	ctx context.Context, frequi *freqtradev1beta1.FreqUI, desired map[string]bool,
) error {
	if !r.GatewayAPIAvailable {
		return nil
	}
	logger := log.FromContext(ctx)

	var routes gatewayv1.HTTPRouteList
	if err := r.List(ctx, &routes,
		client.InNamespace(frequi.Namespace), client.MatchingLabels{resources.FrequiRouteLabelKey: frequi.Name},
	); err != nil {
		return fmt.Errorf("failed to list HTTPRoutes for pruning: %w", err)
	}

	for i := range routes.Items {
		route := &routes.Items[i]
		if desired[route.Name] {
			continue
		}
		owner := metav1.GetControllerOf(route)
		if owner == nil || owner.UID != frequi.UID {
			continue
		}
		if err := r.Delete(ctx, route); err != nil && !kerrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete stale HTTPRoute %s: %w", route.Name, err)
		}
		logger.V(2).Info("Pruned stale HTTPRoute", "name", route.Name)
	}
	return nil
}

// deleteOwnedIngress deletes the Ingress named after this FreqUI, but only
// when this FreqUI is actually its controller owner - never an Ingress it
// doesn't own, even if the name happens to match.
func (r *Reconciler) deleteOwnedIngress(ctx context.Context, frequi *freqtradev1beta1.FreqUI) error {
	var ingress networkingv1.Ingress
	err := r.Get(ctx, types.NamespacedName{Name: frequi.Name, Namespace: frequi.Namespace}, &ingress)
	if kerrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get Ingress for deletion: %w", err)
	}
	owner := metav1.GetControllerOf(&ingress)
	if owner == nil || owner.UID != frequi.UID {
		return nil
	}
	if err := r.Delete(ctx, &ingress); err != nil && !kerrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete Ingress: %w", err)
	}
	log.FromContext(ctx).V(2).Info("Deleted owned Ingress", "name", ingress.Name)
	return nil
}

// isDeploymentReady checks if a deployment is ready
func isDeploymentReady(deployment *appsv1.Deployment) bool {
	return deployment.Status.AvailableReplicas == deployment.Status.Replicas &&
		deployment.Status.Replicas > 0
}
