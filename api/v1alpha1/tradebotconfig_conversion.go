package v1alpha1

import (
	"fmt"
	"strings"

	"github.com/ark-sys/freqtrade-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo converts this v1alpha1 TradeBotConfig to the v1beta1 Hub (B2).
// Fails loudly, per the plan's own instruction, rather than silently
// dropping a credential a v1beta1 TradeBotConfig has no field for at all:
// a bot that started with no exchange API key against a live exchange is
// worse than a failed conversion. Mirrors how
// api/v1alpha1/tradebot_conversion.go rejects Job-mode TradeBots for the
// same "no v1beta1 equivalent" reason.
func (c *TradeBotConfig) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta1.TradeBotConfig)

	if fields := plaintextCredentialFields(c); len(fields) > 0 {
		return fmt.Errorf(
			"TradeBotConfig %s/%s sets deprecated plaintext credential field(s) [%s] - v1beta1 has no field for "+
				"these at all; move the value(s) into a Secret and set the corresponding secretRef before converting",
			c.Namespace, c.Name, strings.Join(fields, ", "),
		)
	}

	dst.ObjectMeta = c.ObjectMeta

	if err := convertSimpleSpecFieldsToBeta(&c.Spec, &dst.Spec); err != nil {
		return err
	}

	if c.Spec.Exchange != nil {
		v, err := convertExchangeSpecToBeta(c.Spec.Exchange)
		if err != nil {
			return fmt.Errorf("converting spec.exchange: %w", err)
		}
		dst.Spec.Exchange = v
	}
	if c.Spec.APIServer != nil {
		dst.Spec.APIServer = &v1beta1.APIServerConfig{
			Enabled:       c.Spec.APIServer.Enabled,
			ListenIP:      c.Spec.APIServer.ListenIP,
			ListenPort:    c.Spec.APIServer.ListenPort,
			Verbosity:     c.Spec.APIServer.Verbosity,
			EnableOpenAPI: c.Spec.APIServer.EnableOpenAPI,
			Username:      c.Spec.APIServer.Username,
			SecretRef:     corev1.LocalObjectReference{Name: c.Spec.APIServer.SecretRef},
			CORSOrigins:   c.Spec.APIServer.CORSOrigins,
		}
	}
	if c.Spec.Notification != nil {
		v, err := convertNotificationSpecToBeta(c.Spec.Notification)
		if err != nil {
			return fmt.Errorf("converting spec.notification: %w", err)
		}
		dst.Spec.Notification = v
	}

	dst.Status.Phase = c.Status.Phase
	dst.Status.Message = c.Status.Message
	dst.Status.Conditions = c.Status.Conditions
	dst.Status.ObservedGeneration = c.Status.ObservedGeneration

	return nil
}

// ConvertFrom converts the v1beta1 Hub to this v1alpha1 TradeBotConfig -
// unlike ConvertTo, always succeeds: everything a v1beta1 TradeBotConfig
// can express, a v1alpha1 one already could (v1beta1's shape is a strict
// subset - fewer credential fields, a typed secretRef instead of a bare
// string - not a superset), so there's no lossy or rejected direction
// converting down.
func (c *TradeBotConfig) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta1.TradeBotConfig)

	c.ObjectMeta = src.ObjectMeta

	if err := convertSimpleSpecFieldsFromBeta(&src.Spec, &c.Spec); err != nil {
		return err
	}

	if src.Spec.Exchange != nil {
		v, err := convertExchangeSpecFromBeta(src.Spec.Exchange)
		if err != nil {
			return fmt.Errorf("converting spec.exchange: %w", err)
		}
		c.Spec.Exchange = v
	}
	if src.Spec.APIServer != nil {
		c.Spec.APIServer = &APIServerConfig{
			Enabled:       src.Spec.APIServer.Enabled,
			ListenIP:      src.Spec.APIServer.ListenIP,
			ListenPort:    src.Spec.APIServer.ListenPort,
			Verbosity:     src.Spec.APIServer.Verbosity,
			EnableOpenAPI: src.Spec.APIServer.EnableOpenAPI,
			Username:      src.Spec.APIServer.Username,
			SecretRef:     src.Spec.APIServer.SecretRef.Name,
			CORSOrigins:   src.Spec.APIServer.CORSOrigins,
		}
	}
	if src.Spec.Notification != nil {
		v, err := convertNotificationSpecFromBeta(src.Spec.Notification)
		if err != nil {
			return fmt.Errorf("converting spec.notification: %w", err)
		}
		c.Spec.Notification = v
	}

	c.Status.Phase = src.Status.Phase
	c.Status.Message = src.Status.Message
	c.Status.Conditions = src.Status.Conditions
	c.Status.ObservedGeneration = src.Status.ObservedGeneration

	return nil
}

// convertSimpleSpecFieldsToBeta handles every TradeBotConfigSpec field
// that's structurally identical between versions (no credentials, no
// reference fields to retype) via convertJSON, keeping ConvertTo itself
// focused on the fields that actually need field-specific handling
// (Exchange/APIServer/Notification).
func convertSimpleSpecFieldsToBeta(spec *TradeBotConfigSpec, dst *v1beta1.TradeBotConfigSpec) error {
	if spec.Bot != nil {
		var v v1beta1.BotConfig
		if err := convertJSON(spec.Bot, &v); err != nil {
			return fmt.Errorf("converting spec.bot: %w", err)
		}
		dst.Bot = &v
	}
	if spec.AI != nil {
		var v v1beta1.AIConfig
		if err := convertJSON(spec.AI, &v); err != nil {
			return fmt.Errorf("converting spec.ai: %w", err)
		}
		dst.AI = &v
	}
	if spec.Data != nil {
		var v v1beta1.DataConfig
		if err := convertJSON(spec.Data, &v); err != nil {
			return fmt.Errorf("converting spec.data: %w", err)
		}
		dst.Data = &v
	}
	if spec.Advanced != nil {
		var v v1beta1.AdvancedConfig
		if err := convertJSON(spec.Advanced, &v); err != nil {
			return fmt.Errorf("converting spec.advanced: %w", err)
		}
		dst.Advanced = &v
	}
	if spec.Timeout != nil {
		var v v1beta1.UnfilledTimeoutConfig
		if err := convertJSON(spec.Timeout, &v); err != nil {
			return fmt.Errorf("converting spec.timeout: %w", err)
		}
		dst.Timeout = &v
	}
	if spec.Internals != nil {
		var v v1beta1.InternalsConfig
		if err := convertJSON(spec.Internals, &v); err != nil {
			return fmt.Errorf("converting spec.internals: %w", err)
		}
		dst.Internals = &v
	}
	if spec.Pairlist != nil {
		var v v1beta1.PairListSpec
		if err := convertJSON(spec.Pairlist, &v); err != nil {
			return fmt.Errorf("converting spec.pairlist: %w", err)
		}
		dst.Pairlist = &v
	}
	if spec.PairlistMethod != nil {
		var v v1beta1.PairlistMethodsSpec
		if err := convertJSON(spec.PairlistMethod, &v); err != nil {
			return fmt.Errorf("converting spec.pairlist_method: %w", err)
		}
		dst.PairlistMethod = &v
	}
	if spec.EntryPricing != nil {
		var v v1beta1.PricingSpec
		if err := convertJSON(spec.EntryPricing, &v); err != nil {
			return fmt.Errorf("converting spec.entry_pricing: %w", err)
		}
		dst.EntryPricing = &v
	}
	if spec.ExitPricing != nil {
		var v v1beta1.PricingSpec
		if err := convertJSON(spec.ExitPricing, &v); err != nil {
			return fmt.Errorf("converting spec.exit_pricing: %w", err)
		}
		dst.ExitPricing = &v
	}
	if spec.Order != nil {
		var v v1beta1.OrderSpec
		if err := convertJSON(spec.Order, &v); err != nil {
			return fmt.Errorf("converting spec.order: %w", err)
		}
		dst.Order = &v
	}
	if spec.RiskManagement != nil {
		var v v1beta1.RiskManagementSpec
		if err := convertJSON(spec.RiskManagement, &v); err != nil {
			return fmt.Errorf("converting spec.risk_management: %w", err)
		}
		dst.RiskManagement = &v
	}
	if spec.Experimental != nil {
		var v v1beta1.ExperimentalConfig
		if err := convertJSON(spec.Experimental, &v); err != nil {
			return fmt.Errorf("converting spec.experimental: %w", err)
		}
		dst.Experimental = &v
	}
	if spec.Logging != nil {
		var v v1beta1.LoggingConfig
		if err := convertJSON(spec.Logging, &v); err != nil {
			return fmt.Errorf("converting spec.logging: %w", err)
		}
		dst.Logging = &v
	}
	return nil
}

// convertSimpleSpecFieldsFromBeta is convertSimpleSpecFieldsToBeta's inverse.
func convertSimpleSpecFieldsFromBeta(spec *v1beta1.TradeBotConfigSpec, dst *TradeBotConfigSpec) error {
	if spec.Bot != nil {
		var v BotConfig
		if err := convertJSON(spec.Bot, &v); err != nil {
			return fmt.Errorf("converting spec.bot: %w", err)
		}
		dst.Bot = &v
	}
	if spec.AI != nil {
		var v AIConfig
		if err := convertJSON(spec.AI, &v); err != nil {
			return fmt.Errorf("converting spec.ai: %w", err)
		}
		dst.AI = &v
	}
	if spec.Data != nil {
		var v DataConfig
		if err := convertJSON(spec.Data, &v); err != nil {
			return fmt.Errorf("converting spec.data: %w", err)
		}
		dst.Data = &v
	}
	if spec.Advanced != nil {
		var v AdvancedConfig
		if err := convertJSON(spec.Advanced, &v); err != nil {
			return fmt.Errorf("converting spec.advanced: %w", err)
		}
		dst.Advanced = &v
	}
	if spec.Timeout != nil {
		var v UnfilledTimeoutConfig
		if err := convertJSON(spec.Timeout, &v); err != nil {
			return fmt.Errorf("converting spec.timeout: %w", err)
		}
		dst.Timeout = &v
	}
	if spec.Internals != nil {
		var v InternalsConfig
		if err := convertJSON(spec.Internals, &v); err != nil {
			return fmt.Errorf("converting spec.internals: %w", err)
		}
		dst.Internals = &v
	}
	if spec.Pairlist != nil {
		var v PairListSpec
		if err := convertJSON(spec.Pairlist, &v); err != nil {
			return fmt.Errorf("converting spec.pairlist: %w", err)
		}
		dst.Pairlist = &v
	}
	if spec.PairlistMethod != nil {
		var v PairlistMethodsSpec
		if err := convertJSON(spec.PairlistMethod, &v); err != nil {
			return fmt.Errorf("converting spec.pairlist_method: %w", err)
		}
		dst.PairlistMethod = &v
	}
	if spec.EntryPricing != nil {
		var v PricingSpec
		if err := convertJSON(spec.EntryPricing, &v); err != nil {
			return fmt.Errorf("converting spec.entry_pricing: %w", err)
		}
		dst.EntryPricing = &v
	}
	if spec.ExitPricing != nil {
		var v PricingSpec
		if err := convertJSON(spec.ExitPricing, &v); err != nil {
			return fmt.Errorf("converting spec.exit_pricing: %w", err)
		}
		dst.ExitPricing = &v
	}
	if spec.Order != nil {
		var v OrderSpec
		if err := convertJSON(spec.Order, &v); err != nil {
			return fmt.Errorf("converting spec.order: %w", err)
		}
		dst.Order = &v
	}
	if spec.RiskManagement != nil {
		var v RiskManagementSpec
		if err := convertJSON(spec.RiskManagement, &v); err != nil {
			return fmt.Errorf("converting spec.risk_management: %w", err)
		}
		dst.RiskManagement = &v
	}
	if spec.Experimental != nil {
		var v ExperimentalConfig
		if err := convertJSON(spec.Experimental, &v); err != nil {
			return fmt.Errorf("converting spec.experimental: %w", err)
		}
		dst.Experimental = &v
	}
	if spec.Logging != nil {
		var v LoggingConfig
		if err := convertJSON(spec.Logging, &v); err != nil {
			return fmt.Errorf("converting spec.logging: %w", err)
		}
		dst.Logging = &v
	}
	return nil
}

// convertExchangeSpecToBeta converts everything ConvertTo's credential
// check has already confirmed carries no plaintext credential - only
// SecretRef (string -> typed reference) and UnkownFeeRate -> UnknownFeeRate
// (A1, closed at the API level here) need field-specific handling; the
// rest is a straight copy. Whitelist/Blacklist are structurally identical,
// so convertJSON handles those.
func convertExchangeSpecToBeta(e *ExchangeSpec) (*v1beta1.ExchangeSpec, error) {
	out := &v1beta1.ExchangeSpec{
		Name:                  e.Name,
		AccountID:             e.AccountID,
		SecretRef:             corev1.LocalObjectReference{Name: e.SecretRef},
		CcxtConfig:            e.CcxtConfig,
		CcxtAsyncConfig:       e.CcxtAsyncConfig,
		CcxtSyncConfig:        e.CcxtSyncConfig,
		LogResponses:          e.LogResponses,
		EnableWS:              e.EnableWS,
		UnknownFeeRate:        e.UnkownFeeRate,
		OutdatedOffset:        e.OutdatedOffset,
		MarketRefreshInterval: e.MarketRefreshInterval,
	}
	if e.Whitelist != nil {
		var v v1beta1.PairListSpec
		if err := convertJSON(e.Whitelist, &v); err != nil {
			return nil, fmt.Errorf("converting whitelist: %w", err)
		}
		out.Whitelist = &v
	}
	if e.Blacklist != nil {
		var v v1beta1.PairListSpec
		if err := convertJSON(e.Blacklist, &v); err != nil {
			return nil, fmt.Errorf("converting blacklist: %w", err)
		}
		out.Blacklist = &v
	}
	return out, nil
}

// convertExchangeSpecFromBeta is convertExchangeSpecToBeta's inverse -
// always succeeds, since v1alpha1 has a field for everything v1beta1 does
// (and more, for the plaintext credential fields ConvertFrom leaves unset).
func convertExchangeSpecFromBeta(e *v1beta1.ExchangeSpec) (*ExchangeSpec, error) {
	out := &ExchangeSpec{
		Name:                  e.Name,
		AccountID:             e.AccountID,
		SecretRef:             e.SecretRef.Name,
		CcxtConfig:            e.CcxtConfig,
		CcxtAsyncConfig:       e.CcxtAsyncConfig,
		CcxtSyncConfig:        e.CcxtSyncConfig,
		LogResponses:          e.LogResponses,
		EnableWS:              e.EnableWS,
		UnkownFeeRate:         e.UnknownFeeRate,
		OutdatedOffset:        e.OutdatedOffset,
		MarketRefreshInterval: e.MarketRefreshInterval,
	}
	if e.Whitelist != nil {
		var v PairListSpec
		if err := convertJSON(e.Whitelist, &v); err != nil {
			return nil, fmt.Errorf("converting whitelist: %w", err)
		}
		out.Whitelist = &v
	}
	if e.Blacklist != nil {
		var v PairListSpec
		if err := convertJSON(e.Blacklist, &v); err != nil {
			return nil, fmt.Errorf("converting blacklist: %w", err)
		}
		out.Blacklist = &v
	}
	return out, nil
}

// convertNotificationSpecToBeta handles Telegram's SecretRef field
// specifically (same reasoning as ExchangeSpec's); Webhook and Discord are
// structurally identical, so convertJSON handles those wholesale.
func convertNotificationSpecToBeta(n *NotificationSpec) (*v1beta1.NotificationSpec, error) {
	out := &v1beta1.NotificationSpec{}
	if n.Telegram != nil {
		out.Telegram = &v1beta1.NotificationTelegram{
			Enabled:             n.Telegram.Enabled,
			SecretRef:           corev1.LocalObjectReference{Name: n.Telegram.SecretRef},
			BalanceDustLevel:    n.Telegram.BalanceDustLevel,
			Reload:              n.Telegram.Reload,
			AllowCustomMessages: n.Telegram.AllowCustomMessages,
			ChatID:              n.Telegram.ChatID,
			TopicID:             n.Telegram.TopicID,
			AuthorizedUsers:     n.Telegram.AuthorizedUsers,
		}
		if n.Telegram.Settings != nil {
			var v v1beta1.NotificationTelegramSettings
			if err := convertJSON(n.Telegram.Settings, &v); err != nil {
				return nil, fmt.Errorf("converting telegram.settings: %w", err)
			}
			out.Telegram.Settings = &v
		}
	}
	if n.Webhook != nil {
		var v v1beta1.NotificationWebhook
		if err := convertJSON(n.Webhook, &v); err != nil {
			return nil, fmt.Errorf("converting webhook: %w", err)
		}
		out.Webhook = &v
	}
	if n.Discord != nil {
		var v v1beta1.NotificationDiscord
		if err := convertJSON(n.Discord, &v); err != nil {
			return nil, fmt.Errorf("converting discord: %w", err)
		}
		out.Discord = &v
	}
	return out, nil
}

// convertNotificationSpecFromBeta is convertNotificationSpecToBeta's
// inverse - always succeeds.
func convertNotificationSpecFromBeta(n *v1beta1.NotificationSpec) (*NotificationSpec, error) {
	out := &NotificationSpec{}
	if n.Telegram != nil {
		out.Telegram = &NotificationTelegram{
			Enabled:             n.Telegram.Enabled,
			SecretRef:           n.Telegram.SecretRef.Name,
			BalanceDustLevel:    n.Telegram.BalanceDustLevel,
			Reload:              n.Telegram.Reload,
			AllowCustomMessages: n.Telegram.AllowCustomMessages,
			ChatID:              n.Telegram.ChatID,
			TopicID:             n.Telegram.TopicID,
			AuthorizedUsers:     n.Telegram.AuthorizedUsers,
		}
		if n.Telegram.Settings != nil {
			var v NotificationTelegramSettings
			if err := convertJSON(n.Telegram.Settings, &v); err != nil {
				return nil, fmt.Errorf("converting telegram.settings: %w", err)
			}
			out.Telegram.Settings = &v
		}
	}
	if n.Webhook != nil {
		var v NotificationWebhook
		if err := convertJSON(n.Webhook, &v); err != nil {
			return nil, fmt.Errorf("converting webhook: %w", err)
		}
		out.Webhook = &v
	}
	if n.Discord != nil {
		var v NotificationDiscord
		if err := convertJSON(n.Discord, &v); err != nil {
			return nil, fmt.Errorf("converting discord: %w", err)
		}
		out.Discord = &v
	}
	return out, nil
}
