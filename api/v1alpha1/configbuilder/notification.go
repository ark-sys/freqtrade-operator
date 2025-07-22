package configbuilder

import (
	"context"
	"fmt"

	"github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// BuildNotificationConfig builds the notification configuration for Freqtrade config.json
func BuildNotificationConfig(
	ctx context.Context,
	k8sClient client.Client,
	notification *v1alpha1.Notification,
) (map[string]interface{}, error) {
	if notification == nil {
		return nil, nil
	}

	cfg := map[string]interface{}{}

	// Telegram configuration
	if notification.Spec.Telegram != nil && notification.Spec.Telegram.Enabled {
		telegram := map[string]interface{}{
			"enabled": true,
		}

		// Get Telegram credentials from Secret
		if notification.Spec.Telegram.SecretRef != "" {
			secretData, err := GetSecretData(ctx, k8sClient, notification.Namespace, notification.Spec.Telegram.SecretRef)
			if err != nil {
				return nil, fmt.Errorf("failed to get telegram secret: %w", err)
			}

			if token, ok := secretData["token"]; ok {
				telegram["token"] = string(token)
			}

			if chatID, ok := secretData["chat-id"]; ok {
				telegram["chat_id"] = string(chatID)
			}
		}

		if notification.Spec.Telegram.BalanceDustLevel > 0 {
			telegram["balance_dust_level"] = notification.Spec.Telegram.BalanceDustLevel
		}

		if notification.Spec.Telegram.Reload != nil {
			telegram["reload"] = *notification.Spec.Telegram.Reload
		}

		if notification.Spec.Telegram.AllowCustomMessages != nil {
			telegram["allow_custom_messages"] = *notification.Spec.Telegram.AllowCustomMessages
		}

		cfg["telegram"] = telegram
	}

	// Webhook configuration
	if notification.Spec.Webhook != nil && notification.Spec.Webhook.Enabled {
		webhook := map[string]interface{}{
			"enabled": true,
			"url":     notification.Spec.Webhook.URL,
		}

		if notification.Spec.Webhook.Entry != "" {
			webhook["webhookentry"] = notification.Spec.Webhook.Entry
		}

		if notification.Spec.Webhook.EntryCancel != "" {
			webhook["webhookentrycancel"] = notification.Spec.Webhook.EntryCancel
		}

		if notification.Spec.Webhook.EntryFill != "" {
			webhook["webhookentryfill"] = notification.Spec.Webhook.EntryFill
		}

		if notification.Spec.Webhook.Exit != "" {
			webhook["webhookentryfill"] = notification.Spec.Webhook.Exit
		}

		if notification.Spec.Webhook.ExitCancel != "" {
			webhook["webhookexitcancel"] = notification.Spec.Webhook.ExitCancel
		}

		if notification.Spec.Webhook.ExitFill != "" {
			webhook["webhookexitfill"] = notification.Spec.Webhook.ExitFill
		}

		if notification.Spec.Webhook.Status != "" {
			webhook["webhookstatus"] = notification.Spec.Webhook.Status
		}

		if notification.Spec.Webhook.AllowCustomMessages != nil {
			webhook["allow_custom_messages"] = *notification.Spec.Webhook.AllowCustomMessages
		}

		cfg["webhook"] = webhook
	}

	// Note: API server configuration is now handled by TradeBot configuration
	// If notification has API server config, we can return it separately for merging
	// but the primary API server config should come from TradeBot

	return cfg, nil
}

// BuildAPIServerConfigFromNotification builds API server configuration from notification spec
// This is used to merge additional API server settings from notification into the main config
func BuildAPIServerConfigFromNotification(
	ctx context.Context,
	k8sClient client.Client,
	notification *v1alpha1.Notification,
) (map[string]interface{}, error) {
	if notification == nil || notification.Spec.APIServer == nil || !notification.Spec.APIServer.Enabled {
		return nil, nil
	}

	apiServer := map[string]interface{}{}

	// Override listen IP if specified
	if notification.Spec.APIServer.ListenIP != "" {
		apiServer["listen_ip_address"] = notification.Spec.APIServer.ListenIP
	}

	// Override listen port if specified
	if notification.Spec.APIServer.ListenPort > 0 {
		apiServer["listen_port"] = notification.Spec.APIServer.ListenPort
	}

	// Override verbosity if specified
	if notification.Spec.APIServer.Verbosity != "" {
		apiServer["verbosity"] = notification.Spec.APIServer.Verbosity
	}

	// Override enable_openapi if specified
	if notification.Spec.APIServer.EnableOpenAPI != nil {
		apiServer["enable_openapi"] = *notification.Spec.APIServer.EnableOpenAPI
	}

	// Get API server credentials from Secret
	if notification.Spec.APIServer.SecretRef != "" {
		secretData, err := GetSecretData(ctx, k8sClient, notification.Namespace, notification.Spec.APIServer.SecretRef)
		if err != nil {
			return nil, fmt.Errorf("failed to get api server secret: %w", err)
		}

		if username, ok := secretData["username"]; ok {
			apiServer["username"] = string(username)
		}

		if password, ok := secretData["password"]; ok {
			apiServer["password"] = string(password)
		}

		if wsToken, ok := secretData["ws-token"]; ok {
			apiServer["ws_token"] = string(wsToken)
		}
	}

	return apiServer, nil
}
