package backtest

import (
	"context"
	"fmt"
	"time"

	freqtradev1beta1 "github.com/ark-sys/freqtrade-operator/api/v1beta1"
	"github.com/ark-sys/freqtrade-operator/controllers/shared"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// BacktestFinalizer is the finalizer name used for Backtest resources -
// same literal as TradeBot's own (different GVK, so no collision), kept
// consistent for anyone grepping the cluster for freqtrade.io finalizers.
const BacktestFinalizer = "freqtrade.io/finalizer"

// Reconciler reconciles a Backtest object.
type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// Recorder emits Warning Events on reconcile failure. Nil is fine in
	// tests that have no need of it.
	Recorder record.EventRecorder

	// DefaultImage is the freqtrade image used when spec.pod.image doesn't
	// override it. Empty means shared.DefaultFreqtradeImage.
	DefaultImage string

	// OperatorImage is this manager's own image, used to run the P6-2
	// results-collection sidecar - see cmd/main.go's resolveOwnImage. Never
	// defaulted to anything: an empty value here means every Backtest's Job
	// gets a sidecar container with an empty image, failing loudly at
	// creation rather than silently running some guessed image.
	OperatorImage string
}

// +kubebuilder:rbac:groups=freqtrade.io,resources=backtests,verbs=get;list;watch;update
// +kubebuilder:rbac:groups=freqtrade.io,resources=backtests/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=freqtrade.io,resources=backtests/finalizers,verbs=update
// +kubebuilder:rbac:groups=freqtrade.io,resources=strategies,verbs=get;list;watch
// +kubebuilder:rbac:groups=freqtrade.io,resources=tradebotconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;patch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;patch;update
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;patch;update
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// P6-2: provisioning the results-collection sidecar's own dedicated,
// narrow-RBAC identity (never the manager's own ServiceAccount, P1-2) -
// and reading this manager's own Pod once at startup to learn its image
// (cmd/main.go's resolveOwnImage), both new in this phase.
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;patch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings,verbs=get;list;watch;create;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get

// Reconcile handles the reconciliation loop for Backtest resources.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var backtest freqtradev1beta1.Backtest
	if err := r.Get(ctx, req.NamespacedName, &backtest); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if backtest.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(&backtest, BacktestFinalizer) {
			if err := r.finalizeBacktest(ctx, &backtest); err != nil {
				logger.Error(err, "failed to finalize Backtest")
				return ctrl.Result{RequeueAfter: 10 * time.Second}, err
			}
			controllerutil.RemoveFinalizer(&backtest, BacktestFinalizer)
			if err := r.Update(ctx, &backtest); err != nil {
				logger.Error(err, "failed to remove finalizer from Backtest")
				return ctrl.Result{RequeueAfter: 5 * time.Second}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(&backtest, BacktestFinalizer) {
		controllerutil.AddFinalizer(&backtest, BacktestFinalizer)
		if err := r.Update(ctx, &backtest); err != nil {
			logger.Error(err, "failed to add finalizer to Backtest")
			return ctrl.Result{RequeueAfter: 5 * time.Second}, err
		}
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	jobName, err := r.reconcileResources(ctx, &backtest)
	if err != nil {
		reason := freqtradev1beta1.ReasonReconcileError
		if errors.IsNotFound(err) {
			reason = freqtradev1beta1.ReasonReferenceNotFound
		}
		return r.failReconcile(ctx, &backtest, freqtradev1beta1.ConditionConfigResolved, reason,
			fmt.Sprintf("failed to reconcile resources: %v", err), err)
	}

	workload, err := r.computeWorkloadStatus(ctx, &backtest)
	if err != nil {
		return r.failReconcile(ctx, &backtest, freqtradev1beta1.ConditionWorkloadReady, freqtradev1beta1.ReasonReconcileError,
			fmt.Sprintf("failed to read workload status: %v", err), err)
	}

	// Only once the Job has actually succeeded, and only until it resolves
	// once (a result never changes after that - the Job is immutable, and
	// re-fetching an already-adopted ConfigMap every reconcile forever would
	// be pure waste). A "not found yet" outcome requeues shortly below
	// rather than being treated as a final answer - see
	// resultsRequeueAfter's own doc comment for why.
	var results *freqtradev1beta1.BacktestResults
	var resultsCondition metav1.Condition
	resultsRequeue := false
	if workload.succeeded && backtest.Status.Results == nil {
		results, resultsCondition, resultsRequeue, err = r.adoptAndParseResults(ctx, &backtest)
		if err != nil {
			return r.failReconcile(ctx, &backtest, freqtradev1beta1.ConditionResultsAvailable,
				freqtradev1beta1.ReasonReconcileError, fmt.Sprintf("failed to adopt results ConfigMap: %v", err), err)
		}
	}

	if err := shared.PatchStatus(ctx, r.Client, &backtest, func() {
		meta.SetStatusCondition(&backtest.Status.Conditions, metav1.Condition{
			Type: freqtradev1beta1.ConditionConfigResolved, Status: metav1.ConditionTrue,
			Reason: freqtradev1beta1.ReasonAsExpected,
		})
		status, reason := workload.conditionStatusAndReason()
		meta.SetStatusCondition(&backtest.Status.Conditions, metav1.Condition{
			Type: freqtradev1beta1.ConditionWorkloadReady, Status: status, Reason: reason, Message: workload.message,
		})
		meta.SetStatusCondition(&backtest.Status.Conditions, metav1.Condition{
			Type: freqtradev1beta1.ConditionReady, Status: status, Reason: reason, Message: workload.message,
		})
		if resultsCondition.Type != "" {
			meta.SetStatusCondition(&backtest.Status.Conditions, resultsCondition)
		}
		if results != nil {
			backtest.Status.Results = results
		}
		backtest.Status.JobName = jobName
		backtest.Status.ResultsPVCName = backtest.Name + "-results"
		if workload.startTime != nil {
			backtest.Status.StartTime = workload.startTime
		}
		if workload.completionTime != nil {
			backtest.Status.CompletionTime = workload.completionTime
		}
		backtest.Status.Phase = deriveBacktestPhase(workload)
		backtest.Status.Message = workload.message
	}); err != nil {
		logger.Error(err, "failed to update Backtest status")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, err
	}

	requeueAfter := workload.requeueAfter
	if resultsRequeue && requeueAfter == 0 {
		requeueAfter = resultsRequeueAfter
	}
	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

// resultsRequeueAfter is how soon a Succeeded Backtest whose results
// ConfigMap wasn't found yet gets re-checked - short, since by the time
// the Job itself reports Succeeded, the native sidecar semantics
// (RestartPolicy: Always) mean the results sidecar should already have
// finished too; "not found" here is expected to be a brief, rare race, not
// the normal path.
const resultsRequeueAfter = 5 * time.Second

// workloadStatus is the raw outcome of inspecting the Job a Backtest owns,
// before it's translated into conditions/Phase.
type workloadStatus struct {
	running, succeeded, failed bool
	message                    string
	requeueAfter               time.Duration
	startTime, completionTime  *metav1.Time
}

func (w workloadStatus) conditionStatusAndReason() (metav1.ConditionStatus, string) {
	switch {
	case w.failed:
		return metav1.ConditionFalse, freqtradev1beta1.ReasonWorkloadFailed
	case w.succeeded:
		return metav1.ConditionTrue, freqtradev1beta1.ReasonWorkloadSucceeded
	default:
		return metav1.ConditionFalse, freqtradev1beta1.ReasonWorkloadProgressing
	}
}

// computeWorkloadStatus inspects the Job this Backtest owns.
func (r *Reconciler) computeWorkloadStatus(
	ctx context.Context, backtest *freqtradev1beta1.Backtest,
) (workloadStatus, error) {
	var job batchv1.Job
	if err := r.Get(ctx, client.ObjectKeyFromObject(backtest), &job); err != nil {
		return workloadStatus{}, fmt.Errorf("failed to get Job: %w", err)
	}
	ws := workloadStatus{startTime: job.Status.StartTime, completionTime: job.Status.CompletionTime}
	switch {
	case job.Status.Succeeded > 0:
		ws.succeeded = true
	case job.Status.Failed > 0:
		ws.failed = true
		ws.message = "Job failed; check pod logs"
	case job.Status.Active > 0:
		ws.running = true
		ws.message = "Job is running"
		ws.requeueAfter = 15 * time.Second
	default:
		ws.message = "Waiting for Job to start"
		ws.requeueAfter = 15 * time.Second
	}
	return ws, nil
}

// deriveBacktestPhase computes the human-facing Phase from workload - it is
// never itself the source of truth (Conditions are), only the printer-column summary.
func deriveBacktestPhase(workload workloadStatus) string {
	switch {
	case workload.succeeded:
		return "Succeeded"
	case workload.failed:
		return "Failed"
	case workload.running:
		return "Running"
	default:
		return "Pending"
	}
}

// failReconcileRequeueAfter is how soon a failed reconcile re-checks, when
// the failure itself isn't returned as an error (see failReconcile).
const failReconcileRequeueAfter = 30 * time.Second

// failReconcile sets conditionType and Ready to False with the given
// reason/message, derives Phase, and returns a ctrl.Result:
// failReconcileRequeueAfter is honored when err is nil, otherwise
// controller-runtime's own exponential backoff takes over.
func (r *Reconciler) failReconcile(
	ctx context.Context, backtest *freqtradev1beta1.Backtest,
	conditionType, reason, message string, err error,
) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	if patchErr := shared.PatchStatus(ctx, r.Client, backtest, func() {
		meta.SetStatusCondition(&backtest.Status.Conditions, metav1.Condition{
			Type: conditionType, Status: metav1.ConditionFalse, Reason: reason, Message: message,
		})
		meta.SetStatusCondition(&backtest.Status.Conditions, metav1.Condition{
			Type: freqtradev1beta1.ConditionReady, Status: metav1.ConditionFalse, Reason: reason, Message: message,
		})
		backtest.Status.Phase = "Error"
		backtest.Status.Message = message
	}); patchErr != nil {
		logger.Error(patchErr, "failed to update Backtest status")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, patchErr
	}
	if r.Recorder != nil {
		r.Recorder.Event(backtest, corev1.EventTypeWarning, reason, message)
	}
	shared.ReconcileErrorsTotal.WithLabelValues("backtest", reason).Inc()
	return ctrl.Result{RequeueAfter: failReconcileRequeueAfter}, err
}
