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

	if notification.Telegram != nil {
		cfg["telegram"] = buildTelegramConfig(notification.Telegram, secretData)
	}

	if notification.Webhook != nil && notification.Webhook.Enabled != nil && *notification.Webhook.Enabled {
		cfg["webhook"] = buildWebhookConfig(notification.Webhook)
	}

	if notification.Discord != nil {
		cfg["discord"] = buildDiscordConfig(notification.Discord)
	}

	return cfg, nil
}

// buildTelegramConfig builds the "telegram" section from
// NotificationSpec.Telegram.
func buildTelegramConfig(telegram *v1alpha1.NotificationTelegram, secretData map[string][]byte) map[string]interface{} {
	cfg := map[string]interface{}{}

	if telegram.Enabled != nil {
		cfg["enabled"] = *telegram.Enabled
	}
	// Get Telegram credentials from Secret
	if telegram.SecretRef != "" {
		if token, ok := secretData["token"]; ok {
			cfg["token"] = string(token)
		}

		if chatID, ok := secretData["chat-id"]; ok {
			cfg["chat_id"] = string(chatID)
		}
	} else {
		if telegram.Token != "" {
			cfg["token"] = telegram.Token
		}
		if telegram.ChatID != "" {
			cfg["chat_id"] = telegram.ChatID
		}
	}

	if telegram.BalanceDustLevel != nil {
		cfg["balance_dust_level"] = *telegram.BalanceDustLevel
	}

	if telegram.Reload != nil {
		cfg["reload"] = *telegram.Reload
	}

	if telegram.AllowCustomMessages != nil {
		cfg["allow_custom_messages"] = *telegram.AllowCustomMessages
	}

	if telegram.TopicID != "" {
		cfg["topic_id"] = telegram.TopicID
	}

	if len(telegram.AuthorizedUsers) > 0 {
		cfg["authorized_users"] = telegram.AuthorizedUsers
	}

	if telegram.Settings != nil {
		cfg["notification_settings"] = buildTelegramSettingsConfig(telegram.Settings)
	}

	return cfg
}

// buildTelegramSettingsConfig builds telegram.settings from
// NotificationTelegram.Settings.
func buildTelegramSettingsConfig(s *v1alpha1.NotificationTelegramSettings) map[string]interface{} {
	settings := map[string]interface{}{}
	if s.Status != "" {
		settings["status"] = s.Status
	}
	if s.Warning != "" {
		settings["warning"] = s.Warning
	}
	if s.Startup != "" {
		settings["startup"] = s.Startup
	}
	if s.Entry != "" {
		settings["entry"] = s.Entry
	}
	if s.EntryFill != "" {
		settings["entry_fill"] = s.EntryFill
	}
	if s.EntryCancel != "" {
		settings["entry_cancel"] = s.EntryCancel
	}
	if s.Exit != "" {
		settings["exit"] = s.Exit
	}
	if s.ExitFill != "" {
		settings["exit_fill"] = s.ExitFill
	}
	if s.ExitCancel != "" {
		settings["exit_cancel"] = s.ExitCancel
	}
	if s.ProtectionTrigger != "" {
		settings["protection_trigger"] = s.ProtectionTrigger
	}
	if s.ProtectionTriggerGlobal != "" {
		settings["protection_trigger_global"] = s.ProtectionTriggerGlobal
	}
	return settings
}

// buildWebhookConfig builds the "webhook" section from
// NotificationSpec.Webhook. The caller only invokes this once Webhook is
// known non-nil and enabled.
func buildWebhookConfig(webhook *v1alpha1.NotificationWebhook) map[string]interface{} {
	cfg := map[string]interface{}{
		"enabled": true,
		"url":     webhook.URL,
	}

	if webhook.Entry != "" {
		cfg["webhookentry"] = webhook.Entry
	}

	if webhook.EntryCancel != "" {
		cfg["webhookentrycancel"] = webhook.EntryCancel
	}

	if webhook.EntryFill != "" {
		cfg["webhookentryfill"] = webhook.EntryFill
	}

	if webhook.Exit != "" {
		cfg["webhookexit"] = webhook.Exit
	}

	if webhook.ExitCancel != "" {
		cfg["webhookexitcancel"] = webhook.ExitCancel
	}

	if webhook.ExitFill != "" {
		cfg["webhookexitfill"] = webhook.ExitFill
	}

	if webhook.Status != "" {
		cfg["webhookstatus"] = webhook.Status
	}

	if webhook.AllowCustomMessages != nil {
		cfg["allow_custom_messages"] = *webhook.AllowCustomMessages
	}

	return cfg
}

// buildDiscordConfig builds the "discord" section from
// NotificationSpec.Discord.
func buildDiscordConfig(discord *v1alpha1.NotificationDiscord) map[string]interface{} {
	cfg := map[string]interface{}{}

	if discord.Enabled != nil {
		cfg["enabled"] = *discord.Enabled
	}
	if discord.WebhookURL != "" {
		cfg["webhook_url"] = discord.WebhookURL
	}

	if len(discord.EntryFill) > 0 {
		cfg["entry_fill"] = discord.EntryFill
	}

	if len(discord.ExitFill) > 0 {
		cfg["exit_fill"] = discord.ExitFill
	}

	return cfg
}
