package controllers

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	"github.com/ark-sys/freqtrade-operator/controllers/resources"
)

// FreqUIReconciler reconciles a FreqUI object
type FreqUIReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// Reconcile handles the reconciliation loop for FreqUI resources
func (r *FreqUIReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling FreqUI", "request", req.NamespacedName)

	// 1. Fetch FreqUI
	var frequi freqtradev1alpha1.FreqUI
	if err := r.Get(ctx, req.NamespacedName, &frequi); err != nil {
		if errors.IsNotFound(err) {
			// Object not found, likely deleted, return
			logger.Info("FreqUI resource not found, ignoring since it must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get FreqUI")
		return ctrl.Result{}, err
	}

	// 2. Create or update FreqUI Deployment
	deployment := r.buildFreqUIDeployment(&frequi)
	if err := controllerutil.SetControllerReference(&frequi, &deployment, r.Scheme); err != nil {
		logger.Error(err, "Failed to set controller reference for Deployment")
		return ctrl.Result{}, err
	}

	if err := r.applyDeployment(ctx, &deployment); err != nil {
		logger.Error(err, "Failed to apply Deployment")
		return ctrl.Result{}, err
	}

	// 3. Create or update FreqUI Service
	service := r.buildFreqUIService(&frequi)
	if err := controllerutil.SetControllerReference(&frequi, &service, r.Scheme); err != nil {
		logger.Error(err, "Failed to set controller reference for Service")
		return ctrl.Result{}, err
	}

	if err := r.applyService(ctx, &service); err != nil {
		logger.Error(err, "Failed to apply Service")
		return ctrl.Result{}, err
	}

	// 4. Create or update FreqUI Ingress if host is specified
	if frequi.Spec.Host != "" {
		ingress := r.buildFreqUIIngress(&frequi)
		if err := controllerutil.SetControllerReference(&frequi, &ingress, r.Scheme); err != nil {
			logger.Error(err, "Failed to set controller reference for Ingress")
			return ctrl.Result{}, err
		}

		if err := r.applyIngress(ctx, &ingress); err != nil {
			logger.Error(err, "Failed to apply Ingress")
			return ctrl.Result{}, err
		}
	}

	// 5. Update status
	frequi.Status.Phase = "Running"
	frequi.Status.Message = "FreqUI deployed successfully"
	if frequi.Spec.Host != "" {
		protocol := "http"
		if len(frequi.Spec.TLS) > 0 {
			protocol = "https"
		}
		frequi.Status.URL = fmt.Sprintf("%s://%s", protocol, frequi.Spec.Host)
	} else {
		frequi.Status.URL = fmt.Sprintf("http://%s.%s.svc.cluster.local", frequi.Name, frequi.Namespace)
	}

	if err := r.Status().Update(ctx, &frequi); err != nil {
		logger.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	logger.Info("Reconciliation completed successfully")
	return ctrl.Result{}, nil
}

// buildFreqUIDeployment creates a Deployment for FreqUI
func (r *FreqUIReconciler) buildFreqUIDeployment(frequi *freqtradev1alpha1.FreqUI) appsv1.Deployment {
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
func (r *FreqUIReconciler) buildFreqUIService(frequi *freqtradev1alpha1.FreqUI) corev1.Service {
	return resources.BuildFreqUIService(resources.FreqUIOptions{
		Name:      frequi.Name,
		Namespace: frequi.Namespace,
	})
}

// buildFreqUIIngress creates an Ingress for FreqUI
func (r *FreqUIReconciler) buildFreqUIIngress(frequi *freqtradev1alpha1.FreqUI) networkingv1.Ingress {
	return resources.BuildFreqUIIngress(resources.FreqUIOptions{
		Name:               frequi.Name,
		Namespace:          frequi.Namespace,
		Host:               frequi.Spec.Host,
		IngressAnnotations: frequi.Spec.IngressAnnotations,
		TLS:                frequi.Spec.TLS,
	})
}

// applyDeployment creates or updates the Deployment
func (r *FreqUIReconciler) applyDeployment(ctx context.Context, deploy *appsv1.Deployment) error {
	var existing appsv1.Deployment
	err := r.Get(ctx, types.NamespacedName{Name: deploy.Name, Namespace: deploy.Namespace}, &existing)
	if errors.IsNotFound(err) {
		return r.Create(ctx, deploy)
	} else if err != nil {
		return err
	}
	deploy.ResourceVersion = existing.ResourceVersion
	return r.Update(ctx, deploy)
}

// applyService creates or updates the Service
func (r *FreqUIReconciler) applyService(ctx context.Context, svc *corev1.Service) error {
	var existing corev1.Service
	err := r.Get(ctx, types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}, &existing)
	if errors.IsNotFound(err) {
		return r.Create(ctx, svc)
	} else if err != nil {
		return err
	}
	// Preserve the cluster IP
	svc.Spec.ClusterIP = existing.Spec.ClusterIP
	svc.ResourceVersion = existing.ResourceVersion
	return r.Update(ctx, svc)
}

// applyIngress creates or updates the Ingress
func (r *FreqUIReconciler) applyIngress(ctx context.Context, ing *networkingv1.Ingress) error {
	var existing networkingv1.Ingress
	err := r.Get(ctx, types.NamespacedName{Name: ing.Name, Namespace: ing.Namespace}, &existing)
	if errors.IsNotFound(err) {
		return r.Create(ctx, ing)
	} else if err != nil {
		return err
	}
	ing.ResourceVersion = existing.ResourceVersion
	return r.Update(ctx, ing)
}

// SetupWithManager sets up the controller with the Manager.
func (r *FreqUIReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&freqtradev1alpha1.FreqUI{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&networkingv1.Ingress{}).
		Complete(r)
}
