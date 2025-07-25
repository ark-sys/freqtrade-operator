package frequi

import (
	"context"
	"fmt"
	"reflect"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	"github.com/ark-sys/freqtrade-operator/controllers/frequi/resources"
)

// Reconciler FreqUIReconciler reconciles a FreqUI object
type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// Reconcile handles the reconciliation loop for FreqUI resources
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.V(1).Info("Starting FreqUI reconciliation", "namespacedName", req.NamespacedName)

	// 1. Fetch FreqUI resource
	var frequi freqtradev1alpha1.FreqUI
	if err := r.Get(ctx, req.NamespacedName, &frequi); err != nil {
		if errors.IsNotFound(err) {
			// Object not found, likely deleted, nothing to do
			logger.Info("FreqUI resource not found, ignoring since it must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get FreqUI resource")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Store original status for comparison later
	originalStatus := frequi.Status.DeepCopy()
	statusChanged := false

	// Initialize status if empty
	if frequi.Status.Phase == "" {
		frequi.Status.Phase = "Initializing"
		statusChanged = true
	}

	// 2. Reconcile all resources
	if err := r.reconcileAllResources(ctx, &frequi); err != nil {
		logger.Error(err, "Failed to reconcile resources")
		frequi.Status.Phase = "ResourceError"
		frequi.Status.Message = fmt.Sprintf("Failed to reconcile resources: %v", err)
		statusChanged = true
		return r.finishReconciliation(ctx, &frequi, originalStatus, statusChanged, 30*time.Second, err)
	}

	// 3. Check if deployment is ready
	deployment := &appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{Name: frequi.Name, Namespace: frequi.Namespace}, deployment); err != nil {
		logger.Error(err, "Failed to get deployment status")
		frequi.Status.Phase = "DeploymentError"
		frequi.Status.Message = fmt.Sprintf("Failed to get deployment status: %v", err)
		statusChanged = true
		return r.finishReconciliation(ctx, &frequi, originalStatus, statusChanged, 30*time.Second, err)
	}

	// 4. Update status based on deployment readiness
	if isDeploymentReady(deployment) {
		frequi.Status.Phase = "Running"
		frequi.Status.Message = "FreqUI deployed successfully"

		// Set URL based on whether host is specified
		if frequi.Spec.Host != "" {
			protocol := "http"
			if len(frequi.Spec.TLS) > 0 {
				protocol = "https"
			}
			frequi.Status.URL = fmt.Sprintf("%s://%s", protocol, frequi.Spec.Host)
		} else {
			frequi.Status.URL = fmt.Sprintf("http://%s.%s.svc.cluster.local", frequi.Name, frequi.Namespace)
		}

		statusChanged = !reflect.DeepEqual(originalStatus, &frequi.Status)

		logger.Info("FreqUI reconciliation completed successfully", "name", frequi.Name)

		// If nothing changed and we're already in Running state, don't requeue
		if !statusChanged && originalStatus.Phase == "Running" {
			logger.V(1).Info("FreqUI is running and stable, no requeue needed")
			return ctrl.Result{}, nil
		}

		// Otherwise, requeue after longer period
		return r.finishReconciliation(ctx, &frequi, originalStatus, statusChanged, 5*time.Minute, nil)
	}

	// Deployment is not ready yet, update status and requeue sooner
	frequi.Status.Phase = "Pending"
	frequi.Status.Message = "Waiting for deployment to be ready"
	statusChanged = !reflect.DeepEqual(originalStatus, &frequi.Status)

	logger.Info("Deployment not ready yet, requeuing",
		"name", frequi.Name,
		"availableReplicas", deployment.Status.AvailableReplicas,
		"replicas", deployment.Status.Replicas)

	return r.finishReconciliation(ctx, &frequi, originalStatus, statusChanged, 10*time.Second, nil)
}

// finishReconciliation handles status updates and returns the appropriate result
func (r *Reconciler) finishReconciliation(
	ctx context.Context,
	frequi *freqtradev1alpha1.FreqUI,
	originalStatus *freqtradev1alpha1.FreqUIStatus,
	statusChanged bool,
	requeueAfter time.Duration,
	err error) (ctrl.Result, error) {

	logger := log.FromContext(ctx)

	// Only update status if it has changed
	if statusChanged {
		updateErr := r.Status().Update(ctx, frequi)
		if updateErr != nil {
			logger.Error(updateErr, "Failed to update FreqUI status")
			return ctrl.Result{RequeueAfter: 5 * time.Second}, updateErr
		}
		logger.Info("Updated FreqUI status", "phase", frequi.Status.Phase)
	}

	// Return appropriate result based on error
	if err != nil {
		return ctrl.Result{RequeueAfter: requeueAfter}, err
	}

	// If status is "Running" and no changes were made, don't requeue
	if frequi.Status.Phase == "Running" && !statusChanged {
		// Only log at a higher verbosity level to reduce noise
		logger.V(1).Info("FreqUI is running and stable, no requeue needed")
		return ctrl.Result{}, nil
	}

	// Otherwise, requeue after the specified duration
	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

// reconcileAllResources creates or updates all resources needed by FreqUI
func (r *Reconciler) reconcileAllResources(ctx context.Context, frequi *freqtradev1alpha1.FreqUI) error {
	logger := log.FromContext(ctx)

	// 1. Reconcile Configuration ConfigMap
	if err := r.reconcileConfigMap(ctx, frequi); err != nil {
		return fmt.Errorf("failed to reconcile configmap: %w", err)
	}

	// 2. Reconcile Deployment
	desired := r.buildFreqUIDeployment(frequi)
	if err := controllerutil.SetControllerReference(frequi, &desired, r.Scheme); err != nil {
		return fmt.Errorf("failed to set controller reference on deployment: %w", err)
	}

	existing := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing)

	if err != nil && errors.IsNotFound(err) {
		// Create new deployment
		logger.Info("Creating new Deployment", "name", desired.Name)
		if err := r.Create(ctx, &desired); err != nil {
			return fmt.Errorf("failed to create deployment: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to get deployment: %w", err)
	} else {
		// Check if update is needed by comparing only the fields we control
		needsUpdate := false

		// Compare replicas
		if existing.Spec.Replicas == nil || desired.Spec.Replicas == nil || *existing.Spec.Replicas != *desired.Spec.Replicas {
			needsUpdate = true
		}

		// Compare selector (this should rarely change)
		if !reflect.DeepEqual(existing.Spec.Selector, desired.Spec.Selector) {
			needsUpdate = true
		}

		// Compare template labels
		if !reflect.DeepEqual(existing.Spec.Template.ObjectMeta.Labels, desired.Spec.Template.ObjectMeta.Labels) {
			needsUpdate = true
		}

		// Compare containers count
		if len(existing.Spec.Template.Spec.Containers) != len(desired.Spec.Template.Spec.Containers) {
			needsUpdate = true
		} else {
			// Compare container specs - only the fields we care about
			for i, existingContainer := range existing.Spec.Template.Spec.Containers {
				desiredContainer := desired.Spec.Template.Spec.Containers[i]

				// Compare basic container properties
				if existingContainer.Name != desiredContainer.Name ||
					existingContainer.Image != desiredContainer.Image ||
					existingContainer.ImagePullPolicy != desiredContainer.ImagePullPolicy {
					needsUpdate = true
					break
				}

				// Compare ports
				if !reflect.DeepEqual(existingContainer.Ports, desiredContainer.Ports) {
					needsUpdate = true
					break
				}

				// Compare probes - but be more careful about the comparison
				if !probesEqual(existingContainer.LivenessProbe, desiredContainer.LivenessProbe) {
					needsUpdate = true
					break
				}

				if !probesEqual(existingContainer.ReadinessProbe, desiredContainer.ReadinessProbe) {
					needsUpdate = true
					break
				}
			}
		}

		if needsUpdate {
			logger.Info("Updating existing Deployment", "name", existing.Name)
		} else {
			logger.V(1).Info("Deployment is up to date, no update needed", "name", existing.Name)
			// Preserve the existing metadata but update the spec
			existing.Spec.Template.ObjectMeta.Labels = desired.Spec.Template.ObjectMeta.Labels
			existing.Spec.Template.Spec = desired.Spec.Template.Spec
			existing.Spec.Selector = desired.Spec.Selector
			existing.Spec.Replicas = desired.Spec.Replicas
			if err := r.Update(ctx, existing); err != nil {
				return fmt.Errorf("failed to update deployment: %w", err)
			}
		}
	}

	// 2. Reconcile Service
	desiredService := r.buildFreqUIService(frequi)
	if err := controllerutil.SetControllerReference(frequi, &desiredService, r.Scheme); err != nil {
		return fmt.Errorf("failed to set controller reference on service: %w", err)
	}

	existingService := &corev1.Service{}
	err = r.Get(ctx, types.NamespacedName{Name: desiredService.Name, Namespace: desiredService.Namespace}, existingService)

	if err != nil && errors.IsNotFound(err) {
		// Create new service
		logger.Info("Creating new Service", "name", desiredService.Name)
		if err := r.Create(ctx, &desiredService); err != nil {
			return fmt.Errorf("failed to create service: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to get service: %w", err)
	} else {
		// Check if update is needed - only compare fields we control
		needsUpdate := false

		// Compare service type (though we typically don't set this)
		if existingService.Spec.Type != desiredService.Spec.Type && desiredService.Spec.Type != "" {
			needsUpdate = true
		}

		// Compare ports
		if !reflect.DeepEqual(existingService.Spec.Ports, desiredService.Spec.Ports) {
			needsUpdate = true
		}

		// Compare selector
		if !reflect.DeepEqual(existingService.Spec.Selector, desiredService.Spec.Selector) {
			needsUpdate = true
		}

		if needsUpdate {
			logger.Info("Updating existing Service", "name", existingService.Name)
		} else {
			logger.V(1).Info("Service is up to date, no update needed", "name", existingService.Name)
			// Preserve system-managed fields
			clusterIP := existingService.Spec.ClusterIP
			clusterIPs := existingService.Spec.ClusterIPs

			existingService.Spec.Ports = desiredService.Spec.Ports
			existingService.Spec.Selector = desiredService.Spec.Selector
			if desiredService.Spec.Type != "" {
				existingService.Spec.Type = desiredService.Spec.Type
			}

			// Preserve system-managed fields
			existingService.Spec.ClusterIP = clusterIP
			existingService.Spec.ClusterIPs = clusterIPs

			if err := r.Update(ctx, existingService); err != nil {
				return fmt.Errorf("failed to update service: %w", err)
			}
		}
	}

	// 3. Reconcile Ingress if host is specified
	if frequi.Spec.Host != "" {
		desiredIngress := r.buildFreqUIIngress(ctx, frequi)
		if err := controllerutil.SetControllerReference(frequi, &desiredIngress, r.Scheme); err != nil {
			return fmt.Errorf("failed to set controller reference on ingress: %w", err)
		}

		existingIngress := &networkingv1.Ingress{}
		err = r.Get(ctx, types.NamespacedName{Name: desiredIngress.Name, Namespace: desiredIngress.Namespace}, existingIngress)

		if err != nil && errors.IsNotFound(err) {
			// Create new ingress
			logger.Info("Creating new Ingress", "name", desiredIngress.Name)
			if err := r.Create(ctx, &desiredIngress); err != nil {
				return fmt.Errorf("failed to create ingress: %w", err)
			}
		} else if err != nil {
			return fmt.Errorf("failed to get ingress: %w", err)
		} else {
			// Check if update is needed - only compare fields we control
			needsUpdate := false

			// Compare ingress rules
			if !reflect.DeepEqual(existingIngress.Spec.Rules, desiredIngress.Spec.Rules) {
				needsUpdate = true
			}

			// Compare TLS configuration
			if !reflect.DeepEqual(existingIngress.Spec.TLS, desiredIngress.Spec.TLS) {
				needsUpdate = true
			}

			// Compare ingress class name
			if !reflect.DeepEqual(existingIngress.Spec.IngressClassName, desiredIngress.Spec.IngressClassName) {
				needsUpdate = true
			}

			// Compare only the annotations we set (be more selective)
			if !annotationsEqual(existingIngress.ObjectMeta.Annotations, desiredIngress.ObjectMeta.Annotations) {
				needsUpdate = true
			}

			if needsUpdate {
				logger.Info("Updating existing Ingress", "name", existingIngress.Name)
			} else {
				logger.V(1).Info("Ingress is up to date, no update needed", "name", existingIngress.Name)
				existingIngress.Spec.Rules = desiredIngress.Spec.Rules
				existingIngress.Spec.TLS = desiredIngress.Spec.TLS
				existingIngress.Spec.IngressClassName = desiredIngress.Spec.IngressClassName

				// Merge annotations - preserve existing ones and add/update ours
				if existingIngress.ObjectMeta.Annotations == nil {
					existingIngress.ObjectMeta.Annotations = make(map[string]string)
				}
				for k, v := range desiredIngress.ObjectMeta.Annotations {
					existingIngress.ObjectMeta.Annotations[k] = v
				}

				if err := r.Update(ctx, existingIngress); err != nil {
					return fmt.Errorf("failed to update ingress: %w", err)
				}
			}
		}
	}

	return nil
}

// isDeploymentReady checks if a deployment is ready
func isDeploymentReady(deployment *appsv1.Deployment) bool {
	return deployment.Status.AvailableReplicas == deployment.Status.Replicas &&
		deployment.Status.Replicas > 0
}

// probesEqual compares two probes, handling nil cases properly
func probesEqual(existing, desired *corev1.Probe) bool {
	if existing == nil && desired == nil {
		return true
	}
	if existing == nil || desired == nil {
		return false
	}

	// Compare the fields we care about
	return existing.InitialDelaySeconds == desired.InitialDelaySeconds &&
		existing.PeriodSeconds == desired.PeriodSeconds &&
		existing.TimeoutSeconds == desired.TimeoutSeconds &&
		existing.FailureThreshold == desired.FailureThreshold &&
		existing.SuccessThreshold == desired.SuccessThreshold &&
		reflect.DeepEqual(existing.ProbeHandler, desired.ProbeHandler)
}

// annotationsEqual compares annotations, only checking the ones we care about
func annotationsEqual(existing, desired map[string]string) bool {
	// If desired is nil or empty, we don't need to check anything
	if len(desired) == 0 {
		return true
	}

	// If existing is nil but desired has values, they're not equal
	if existing == nil {
		return false
	}

	// Check if all desired annotations exist and have the same value
	for k, v := range desired {
		if existingV, exists := existing[k]; !exists || existingV != v {
			return false
		}
	}

	return true
}

// buildFreqUIDeployment creates a Deployment for FreqUI
func (r *Reconciler) buildFreqUIDeployment(frequi *freqtradev1alpha1.FreqUI) appsv1.Deployment {
	// Set default image if not specified
	image := "freqtradeorg/frequi:latest"
	if frequi.Spec.Image != "" {
		image = frequi.Spec.Image
	}

	return resources.BuildFreqUIDeployment(resources.FreqUIOptions{
		Name:      frequi.Name,
		Namespace: frequi.Namespace,
		Image:     image,
	})
}

// buildFreqUIService creates a Service for FreqUI
func (r *Reconciler) buildFreqUIService(frequi *freqtradev1alpha1.FreqUI) corev1.Service {
	return resources.BuildFreqUIService(resources.FreqUIOptions{
		Name:      frequi.Name,
		Namespace: frequi.Namespace,
	})
}

// reconcileConfigMap creates or updates a ConfigMap with FreqUI configuration
func (r *Reconciler) reconcileConfigMap(ctx context.Context, frequi *freqtradev1alpha1.FreqUI) error {
	logger := log.FromContext(ctx)

	// Build API endpoints configuration
	apiEndpoints := make(map[string]string)
	protocol := "http"
	if len(frequi.Spec.TLS) > 0 {
		protocol = "https"
	}

	baseURL := fmt.Sprintf("%s://%s", protocol, frequi.Spec.Host)
	if frequi.Spec.Host == "" {
		baseURL = fmt.Sprintf("http://%s.%s.svc.cluster.local", frequi.Name, frequi.Namespace)
	}

	for _, tradeBotRef := range frequi.Spec.TradeBotRefs {
		apiEndpoints[tradeBotRef] = fmt.Sprintf("%s/api/%s", baseURL, tradeBotRef)
	}

	// Create configuration as JSON
	configData := map[string]string{
		"config.js": fmt.Sprintf(`window.ftConfig = { apiEndpoints: %v };`, apiEndpoints),
	}

	desired := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      frequi.Name + "-config",
			Namespace: frequi.Namespace,
		},
		Data: configData,
	}

	if err := controllerutil.SetControllerReference(frequi, desired, r.Scheme); err != nil {
		return fmt.Errorf("failed to set controller reference on configmap: %w", err)
	}

	existing := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing)

	if err != nil && errors.IsNotFound(err) {
		logger.Info("Creating new ConfigMap", "name", desired.Name)
		return r.Create(ctx, desired)
	} else if err != nil {
		return fmt.Errorf("failed to get configmap: %w", err)
	}

	// Update if data has changed
	if !reflect.DeepEqual(existing.Data, desired.Data) {
		logger.Info("Updating existing ConfigMap", "name", existing.Name)
		existing.Data = desired.Data
		return r.Update(ctx, existing)
	}

	return nil
}

// buildFreqUIIngress creates an Ingress for FreqUI
func (r *Reconciler) buildFreqUIIngress(ctx context.Context, frequi *freqtradev1alpha1.FreqUI) networkingv1.Ingress {
	// Build API routes for referenced TradeBots
	var apiRoutes []resources.TradeBotAPIRoute

	for _, tradeBotRef := range frequi.Spec.TradeBotRefs {
		// Fetch the TradeBot to verify it exists and has API enabled
		var tradeBot freqtradev1alpha1.TradeBot
		err := r.Get(ctx, types.NamespacedName{Name: tradeBotRef, Namespace: frequi.Namespace}, &tradeBot)
		if err == nil && tradeBot.Spec.APIServer != nil && tradeBot.Spec.APIServer.Enabled {
			apiRoutes = append(apiRoutes, resources.TradeBotAPIRoute{
				Name:        tradeBotRef,
				ServiceName: tradeBotRef,
				PathPrefix:  tradeBotRef,
			})
		}
	}

	return resources.BuildFreqUIIngress(resources.FreqUIOptions{
		Name:               frequi.Name,
		Namespace:          frequi.Namespace,
		Host:               frequi.Spec.Host,
		IngressAnnotations: frequi.Spec.IngressAnnotations,
		TLS:                frequi.Spec.TLS,
		TradeBotAPIRoutes:  apiRoutes,
	})
}
