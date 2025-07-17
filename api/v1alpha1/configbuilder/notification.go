package configbuilder

import (
	"github.com/ark-sys/freqtrade-operator/api/v1alpha1"
)

// BuildNotificationConfig builds the "notification" section for Freqtrade config.json
func BuildNotificationConfig(notification *v1alpha1.Notification) map[string]interface{} {
	if notification == nil {
		return nil
	}

	cfg := map[string]interface{}{}

	// Configure Telegram
	if notification.Spec.Telegram != nil && notification.Spec.Telegram.Enabled {
		telegram := map[string]interface{}{
			"enabled": true,
			"token":   notification.Spec.Telegram.Token,
			"chat_id": notification.Spec.Telegram.ChatID,
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

	// Configure Webhook
	if notification.Spec.Webhook != nil && notification.Spec.Webhook.Enabled {
		webhook := map[string]interface{}{
			"enabled": true,
			"url":     notification.Spec.Webhook.URL,
		}

		// Add webhook endpoints if specified
		if notification.Spec.Webhook.Entry != "" {
			webhook["entry"] = notification.Spec.Webhook.Entry
		}
		if notification.Spec.Webhook.EntryCancel != "" {
			webhook["entry_cancel"] = notification.Spec.Webhook.EntryCancel
		}
		if notification.Spec.Webhook.EntryFill != "" {
			webhook["entry_fill"] = notification.Spec.Webhook.EntryFill
		}
		if notification.Spec.Webhook.Exit != "" {
			webhook["exit"] = notification.Spec.Webhook.Exit
		}
		if notification.Spec.Webhook.ExitCancel != "" {
			webhook["exit_cancel"] = notification.Spec.Webhook.ExitCancel
		}
		if notification.Spec.Webhook.ExitFill != "" {
			webhook["exit_fill"] = notification.Spec.Webhook.ExitFill
		}
		if notification.Spec.Webhook.Status != "" {
			webhook["status"] = notification.Spec.Webhook.Status
		}

		if notification.Spec.Webhook.AllowCustomMessages != nil {
			webhook["allow_custom_messages"] = *notification.Spec.Webhook.AllowCustomMessages
		}

		cfg["webhook"] = webhook
	}

	// Configure API Server
	if notification.Spec.APIServer != nil && notification.Spec.APIServer.Enabled {
		apiServer := map[string]interface{}{
			"enabled":           true,
			"listen_ip_address": notification.Spec.APIServer.ListenIP,
			"listen_port":       notification.Spec.APIServer.ListenPort,
			"verbosity":         notification.Spec.APIServer.Verbosity,
			"username":          notification.Spec.APIServer.Username,
			"password":          notification.Spec.APIServer.Password,
		}

		if notification.Spec.APIServer.WSToken != "" {
			apiServer["ws_token"] = notification.Spec.APIServer.WSToken
		}

		cfg["api_server"] = apiServer
	}

	return cfg
}
