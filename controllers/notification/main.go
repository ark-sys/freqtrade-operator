package notification

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	"github.com/ark-sys/freqtrade-operator/controllers/shared"
)

// Reconciler reconciles a Notification object
type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme
	shared.StatusUpdater
}

// Reconcile handles the reconciliation loop for Notification resources
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.V(1).Info("Starting Notification reconciliation", "namespacedName", req.NamespacedName)

	// Fetch Notification resource
	var notification freqtradev1alpha1.Notification
	if err := r.Get(ctx, req.NamespacedName, &notification); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Notification resource not found, ignoring since it must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get Notification resource")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Initialize status if empty
	if notification.Status.Phase == "" {
		if err := r.UpdateConfigStatus(ctx, &notification, "Validating", "Starting validation"); err != nil {
			logger.Error(err, "Failed to initialize Notification status")
			return ctrl.Result{RequeueAfter: 10 * time.Second}, err
		}
	}

	// Validate Notification configuration
	if err := r.validateNotification(ctx, &notification); err != nil {
		logger.Error(err, "Notification validation failed")
		if updateErr := r.UpdateConfigStatus(ctx, &notification, "Invalid", err.Error()); updateErr != nil {
			logger.Error(updateErr, "Failed to update Notification status after validation failure")
		}
		result := shared.FinishReconciliation("Invalid", err, 30*time.Second)
		return result.Result, result.Error
	}

	// Update status to valid
	if err := r.UpdateConfigStatus(ctx, &notification, "Valid", "Notification configuration is valid"); err != nil {
		logger.Error(err, "Failed to update Notification status to valid")
		result := shared.FinishReconciliation("Valid", err, 30*time.Second)
		return result.Result, result.Error
	}

	logger.Info("Notification reconciliation completed successfully", "name", notification.Name)
	result := shared.FinishReconciliation("Valid", nil, 5*time.Minute)
	return result.Result, result.Error
}

// validateNotification performs validation of Notification configuration
func (r *Reconciler) validateNotification(ctx context.Context, notification *freqtradev1alpha1.Notification) error {
	logger := log.FromContext(ctx)

	// At least one notification method must be configured
	hasConfig := false

	// Validate Telegram configuration
	if notification.Spec.Telegram != nil {
		hasConfig = true
		if err := r.validateTelegram(notification.Spec.Telegram); err != nil {
			return fmt.Errorf("telegram validation failed: %w", err)
		}
	}

	// Validate Webhook configuration
	if notification.Spec.Webhook != nil {
		hasConfig = true
		if err := r.validateWebhook(notification.Spec.Webhook); err != nil {
			return fmt.Errorf("webhook validation failed: %w", err)
		}
	}

	// Validate Discord configuration
	if notification.Spec.Discord != nil {
		hasConfig = true
		if err := r.validateDiscord(notification.Spec.Discord); err != nil {
			return fmt.Errorf("discord validation failed: %w", err)
		}
	}

	if !hasConfig {
		return fmt.Errorf("at least one notification method (telegram, webhook, or discord) must be configured")
	}

	logger.Info("Notification validation passed", "name", notification.Name)
	return nil
}

// validateTelegram validates Telegram notification configuration
func (r *Reconciler) validateTelegram(telegram *freqtradev1alpha1.NotificationTelegram) error {
	if !telegram.Enabled {
		return nil // Skip validation if disabled
	}

	// Either token or secretRef must be provided
	if telegram.Token == "" && telegram.SecretRef == "" {
		return fmt.Errorf("either token or secretRef must be provided for telegram")
	}

	if telegram.ChatID == "" {
		return fmt.Errorf("chat_id is required for telegram notifications")
	}

	// Validate balance dust level
	if telegram.BalanceDustLevel != nil && *telegram.BalanceDustLevel < 0 {
		return fmt.Errorf("balance_dust_level must be non-negative (current: %f)", *telegram.BalanceDustLevel)
	}

	return nil
}

// validateWebhook validates Webhook notification configuration
func (r *Reconciler) validateWebhook(webhook *freqtradev1alpha1.NotificationWebhook) error {
	if !webhook.Enabled {
		return nil // Skip validation if disabled
	}

	if webhook.URL == "" {
		return fmt.Errorf("url is required for webhook notifications")
	}

	// Basic URL validation could be added here
	// For now, just check it's not empty

	return nil
}

// validateDiscord validates Discord notification configuration
func (r *Reconciler) validateDiscord(discord *freqtradev1alpha1.NotificationDiscord) error {
	if !discord.Enabled {
		return nil // Skip validation if disabled
	}

	if discord.WebhookURL == "" {
		return fmt.Errorf("webhook_url is required for discord notifications")
	}

	return nil
}
