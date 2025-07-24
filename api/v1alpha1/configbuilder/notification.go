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
		} else {
			if notification.Spec.Telegram.Token != "" {
				telegram["token"] = notification.Spec.Telegram.Token
			}
			if notification.Spec.Telegram.ChatID != "" {
				telegram["chat_id"] = notification.Spec.Telegram.ChatID
			}
		}

		if notification.Spec.Telegram.BalanceDustLevel > 0 {
			telegram["balance_dust_level"] = notification.Spec.Telegram.BalanceDustLevel
		}

		if notification.Spec.Telegram.Reload {
			telegram["reload"] = notification.Spec.Telegram.Reload
		}

		if notification.Spec.Telegram.AllowCustomMessages {
			telegram["allow_custom_messages"] = notification.Spec.Telegram.AllowCustomMessages
		}

		if notification.Spec.Telegram.TopicID != "" {
			telegram["topic_id"] = notification.Spec.Telegram.TopicID
		}

		if len(notification.Spec.Telegram.AuthorizedUsers) > 0 {
			telegram["authorized_users"] = notification.Spec.Telegram.AuthorizedUsers
		}

		if notification.Spec.Telegram.Settings != nil {
			settings := map[string]interface{}{}
			if notification.Spec.Telegram.Settings.Status != "" {
				settings["status"] = notification.Spec.Telegram.Settings.Status
			}
			if notification.Spec.Telegram.Settings.Warning != "" {
				settings["warning"] = notification.Spec.Telegram.Settings.Warning
			}
			if notification.Spec.Telegram.Settings.Startup != "" {
				settings["startup"] = notification.Spec.Telegram.Settings.Startup
			}
			if notification.Spec.Telegram.Settings.Entry != "" {
				settings["entry"] = notification.Spec.Telegram.Settings.Entry
			}
			if notification.Spec.Telegram.Settings.EntryFill != "" {
				settings["entry_fill"] = notification.Spec.Telegram.Settings.EntryFill
			}
			if notification.Spec.Telegram.Settings.EntryCancel != "" {
				settings["entry_cancel"] = notification.Spec.Telegram.Settings.EntryCancel
			}
			if notification.Spec.Telegram.Settings.Exit != "" {
				settings["exit"] = notification.Spec.Telegram.Settings.Exit
			}
			if notification.Spec.Telegram.Settings.ExitFill != "" {
				settings["exit_fill"] = notification.Spec.Telegram.Settings.ExitFill
			}
			if notification.Spec.Telegram.Settings.ExitCancel != "" {
				settings["exit_cancel"] = notification.Spec.Telegram.Settings.ExitCancel
			}
			if notification.Spec.Telegram.Settings.ProtectionTrigger != "" {
				settings["protection_trigger"] = notification.Spec.Telegram.Settings.ProtectionTrigger
			}
			if notification.Spec.Telegram.Settings.ProtectionTriggerGlobal != "" {
				settings["protection_trigger_global"] = notification.Spec.Telegram.Settings.ProtectionTriggerGlobal
			}
			telegram["settings"] = settings
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
			webhook["webhookexit"] = notification.Spec.Webhook.Exit
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

	// Discord configuration
	if notification.Spec.Discord != nil && notification.Spec.Discord.Enabled {
		discord := map[string]interface{}{
			"enabled": true,
		}

		if notification.Spec.Discord.WebhookURL != "" {
			discord["webhook_url"] = notification.Spec.Discord.WebhookURL
		}

		if len(notification.Spec.Discord.EntryFill) > 0 {
			discord["entry_fill"] = notification.Spec.Discord.EntryFill
		}

		if len(notification.Spec.Discord.ExitFill) > 0 {
			discord["exit_fill"] = notification.Spec.Discord.ExitFill
		}

		cfg["discord"] = discord
	}

	return cfg, nil
}
