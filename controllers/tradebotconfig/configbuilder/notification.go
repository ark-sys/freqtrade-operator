package configbuilder

import (
	"github.com/ark-sys/freqtrade-operator/api/v1alpha1"
)

// BuildNotificationConfig builds the notification configuration for Freqtrade config.json
func BuildNotificationConfig(
	notification *v1alpha1.NotificationSpec,
	secretData map[string][]byte,
) (map[string]interface{}, error) {
	if notification == nil {
		return nil, nil
	}

	cfg := map[string]interface{}{}

	// Telegram configuration
	if notification.Telegram != nil {
		telegram := map[string]interface{}{}

		if notification.Telegram.Enabled != nil {
			telegram["enabled"] = *notification.Telegram.Enabled
		}
		// Get Telegram credentials from Secret
		if notification.Telegram.SecretRef != "" {
			if token, ok := secretData["token"]; ok {
				telegram["token"] = string(token)
			}

			if chatID, ok := secretData["chat-id"]; ok {
				telegram["chat_id"] = string(chatID)
			}
		} else {
			if notification.Telegram.Token != "" {
				telegram["token"] = notification.Telegram.Token
			}
			if notification.Telegram.ChatID != "" {
				telegram["chat_id"] = notification.Telegram.ChatID
			}
		}

		if notification.Telegram.BalanceDustLevel != nil {
			telegram["balance_dust_level"] = *notification.Telegram.BalanceDustLevel
		}

		if notification.Telegram.Reload != nil {
			telegram["reload"] = *notification.Telegram.Reload
		}

		if notification.Telegram.AllowCustomMessages != nil {
			telegram["allow_custom_messages"] = *notification.Telegram.AllowCustomMessages
		}

		if notification.Telegram.TopicID != "" {
			telegram["topic_id"] = notification.Telegram.TopicID
		}

		if len(notification.Telegram.AuthorizedUsers) > 0 {
			telegram["authorized_users"] = notification.Telegram.AuthorizedUsers
		}

		if notification.Telegram.Settings != nil {
			settings := map[string]interface{}{}
			if notification.Telegram.Settings.Status != "" {
				settings["status"] = notification.Telegram.Settings.Status
			}
			if notification.Telegram.Settings.Warning != "" {
				settings["warning"] = notification.Telegram.Settings.Warning
			}
			if notification.Telegram.Settings.Startup != "" {
				settings["startup"] = notification.Telegram.Settings.Startup
			}
			if notification.Telegram.Settings.Entry != "" {
				settings["entry"] = notification.Telegram.Settings.Entry
			}
			if notification.Telegram.Settings.EntryFill != "" {
				settings["entry_fill"] = notification.Telegram.Settings.EntryFill
			}
			if notification.Telegram.Settings.EntryCancel != "" {
				settings["entry_cancel"] = notification.Telegram.Settings.EntryCancel
			}
			if notification.Telegram.Settings.Exit != "" {
				settings["exit"] = notification.Telegram.Settings.Exit
			}
			if notification.Telegram.Settings.ExitFill != "" {
				settings["exit_fill"] = notification.Telegram.Settings.ExitFill
			}
			if notification.Telegram.Settings.ExitCancel != "" {
				settings["exit_cancel"] = notification.Telegram.Settings.ExitCancel
			}
			if notification.Telegram.Settings.ProtectionTrigger != "" {
				settings["protection_trigger"] = notification.Telegram.Settings.ProtectionTrigger
			}
			if notification.Telegram.Settings.ProtectionTriggerGlobal != "" {
				settings["protection_trigger_global"] = notification.Telegram.Settings.ProtectionTriggerGlobal
			}
			telegram["settings"] = settings
		}

		if notification.Telegram.TopicID != "" {
			telegram["topic_id"] = notification.Telegram.TopicID
		}

		if len(notification.Telegram.AuthorizedUsers) > 0 {
			telegram["authorized_users"] = notification.Telegram.AuthorizedUsers
		}

		if notification.Telegram.Settings != nil {
			settings := map[string]interface{}{}
			if notification.Telegram.Settings.Status != "" {
				settings["status"] = notification.Telegram.Settings.Status
			}
			if notification.Telegram.Settings.Warning != "" {
				settings["warning"] = notification.Telegram.Settings.Warning
			}
			if notification.Telegram.Settings.Startup != "" {
				settings["startup"] = notification.Telegram.Settings.Startup
			}
			if notification.Telegram.Settings.Entry != "" {
				settings["entry"] = notification.Telegram.Settings.Entry
			}
			if notification.Telegram.Settings.EntryFill != "" {
				settings["entry_fill"] = notification.Telegram.Settings.EntryFill
			}
			if notification.Telegram.Settings.EntryCancel != "" {
				settings["entry_cancel"] = notification.Telegram.Settings.EntryCancel
			}
			if notification.Telegram.Settings.Exit != "" {
				settings["exit"] = notification.Telegram.Settings.Exit
			}
			if notification.Telegram.Settings.ExitFill != "" {
				settings["exit_fill"] = notification.Telegram.Settings.ExitFill
			}
			if notification.Telegram.Settings.ExitCancel != "" {
				settings["exit_cancel"] = notification.Telegram.Settings.ExitCancel
			}
			if notification.Telegram.Settings.ProtectionTrigger != "" {
				settings["protection_trigger"] = notification.Telegram.Settings.ProtectionTrigger
			}
			if notification.Telegram.Settings.ProtectionTriggerGlobal != "" {
				settings["protection_trigger_global"] = notification.Telegram.Settings.ProtectionTriggerGlobal
			}
			telegram["settings"] = settings
		}

		cfg["telegram"] = telegram
	}

	// Webhook configuration
	if notification.Webhook != nil && notification.Webhook.Enabled != nil && *notification.Webhook.Enabled {
		webhook := map[string]interface{}{
			"enabled": true,
			"url":     notification.Webhook.URL,
		}

		if notification.Webhook.Entry != "" {
			webhook["webhookentry"] = notification.Webhook.Entry
		}

		if notification.Webhook.EntryCancel != "" {
			webhook["webhookentrycancel"] = notification.Webhook.EntryCancel
		}

		if notification.Webhook.EntryFill != "" {
			webhook["webhookentryfill"] = notification.Webhook.EntryFill
		}

		if notification.Webhook.Exit != "" {
			webhook["webhookexit"] = notification.Webhook.Exit
		}

		if notification.Webhook.ExitCancel != "" {
			webhook["webhookexitcancel"] = notification.Webhook.ExitCancel
		}

		if notification.Webhook.ExitFill != "" {
			webhook["webhookexitfill"] = notification.Webhook.ExitFill
		}

		if notification.Webhook.Status != "" {
			webhook["webhookstatus"] = notification.Webhook.Status
		}

		if notification.Webhook.AllowCustomMessages != nil {
			webhook["allow_custom_messages"] = *notification.Webhook.AllowCustomMessages
		}

		cfg["webhook"] = webhook
	}

	// Discord configuration
	if notification.Discord != nil {
		discord := map[string]interface{}{}

		if notification.Discord.Enabled != nil {
			discord["enabled"] = *notification.Discord.Enabled
		}
		if notification.Discord.WebhookURL != "" {
			discord["webhook_url"] = notification.Discord.WebhookURL
		}

		if len(notification.Discord.EntryFill) > 0 {
			discord["entry_fill"] = notification.Discord.EntryFill
		}

		if len(notification.Discord.ExitFill) > 0 {
			discord["exit_fill"] = notification.Discord.ExitFill
		}

		cfg["discord"] = discord
	}

	return cfg, nil
}
