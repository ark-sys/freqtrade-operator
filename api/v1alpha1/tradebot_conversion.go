package v1alpha1

import (
	"encoding/json"
	"fmt"

	"github.com/ark-sys/freqtrade-operator/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo converts this v1alpha1 TradeBot to the v1beta1 Hub (P6-5).
// Fails loudly, per the plan's own instruction, rather than silently
// dropping data a v1beta1 TradeBot has no field for: a Job-mode TradeBot
// (freqtrade_command anything but "trade") has no v1beta1 equivalent at
// all - Backtest is that equivalent now (D1, P6-1). Converting one needs
// a real migration (create an equivalent Backtest) before it can become
// a v1beta1 object, not an automatic, lossy one. (TradeBotConfig's own
// conversion - api/v1alpha1/tradebotconfig_conversion.go - is the one
// that similarly rejects plaintext credential fields; TradeBot itself
// never carries any.)
func (t *TradeBot) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta1.TradeBot)

	cmd := t.Spec.FreqtradeCommand
	if cmd != "" && cmd != "trade" {
		return fmt.Errorf(
			"TradeBot %s/%s has spec.freqtrade_command=%q - v1beta1 TradeBot is trade-only (P6-4); "+
				"one-shot runs (backtesting, hyperopt, ...) have no v1beta1 TradeBot equivalent, use a Backtest instead",
			t.Namespace, t.Name, cmd,
		)
	}

	dst.ObjectMeta = t.ObjectMeta

	dst.Spec.ConfigRef.Name = t.Spec.Config
	dst.Spec.StrategyRef.Name = t.Spec.Strategy
	dst.Spec.UpdateStrategy = t.Spec.UpdateStrategy

	if t.Spec.App != nil {
		var app v1beta1.TBAppConfig
		if err := convertJSON(t.Spec.App, &app); err != nil {
			return fmt.Errorf("converting spec.app: %w", err)
		}
		dst.Spec.App = &app
	}
	if t.Spec.Introspection != nil {
		var introspection v1beta1.IntrospectionSpec
		if err := convertJSON(t.Spec.Introspection, &introspection); err != nil {
			return fmt.Errorf("converting spec.introspection: %w", err)
		}
		dst.Spec.Introspection = &introspection
	}

	dst.Status.Phase = t.Status.Phase
	dst.Status.Message = t.Status.Message
	dst.Status.Conditions = t.Status.Conditions
	dst.Status.ObservedGeneration = t.Status.ObservedGeneration
	dst.Status.AppliedConfigHash = t.Status.AppliedConfigHash
	dst.Status.ResolvedImage = t.Status.ResolvedImage
	if t.Status.Bot != nil {
		var bot v1beta1.BotStatus
		if err := convertJSON(t.Status.Bot, &bot); err != nil {
			return fmt.Errorf("converting status.bot: %w", err)
		}
		dst.Status.Bot = &bot
	}

	return nil
}

// ConvertFrom converts the v1beta1 Hub to this v1alpha1 TradeBot -
// unlike ConvertTo, always succeeds: everything a v1beta1 TradeBot can
// express, a v1alpha1 one already could (trade mode was always its
// default), so there's no lossy or rejected direction converting down.
func (t *TradeBot) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta1.TradeBot)

	t.ObjectMeta = src.ObjectMeta

	t.Spec.Config = src.Spec.ConfigRef.Name
	t.Spec.Strategy = src.Spec.StrategyRef.Name
	t.Spec.UpdateStrategy = src.Spec.UpdateStrategy
	// FreqtradeCommand/FreqtradeArguments/Data have no v1beta1 source -
	// left at their zero value, which is exactly "trade" (the default a
	// blank FreqtradeCommand already means).
	//
	// src.Spec.State (P4-4) has no v1alpha1 destination at all - silently
	// dropped here, same as ConvertTo below never sets it going the other
	// way. This is genuinely lossy, unlike everything else in this
	// function: a v1alpha1 write to an object that has State=Stopped set
	// would silently convert it back to the default (Running) once
	// persisted. Nothing at this layer can prevent that - ConvertTo, given
	// only the object being converted, has no way to know a different
	// value was ever stored. The actual guard lives one layer up, in
	// api/v1alpha1/tradebot_webhook.go's validate, which rejects a
	// v1alpha1 write outright once State has ever been set to anything
	// but the default (verified directly: without that guard, an
	// unrelated field edit was enough to silently reset a Stopped bot).

	if src.Spec.App != nil {
		var app TBAppConfig
		if err := convertJSON(src.Spec.App, &app); err != nil {
			return fmt.Errorf("converting spec.app: %w", err)
		}
		t.Spec.App = &app
	}
	if src.Spec.Introspection != nil {
		var introspection IntrospectionSpec
		if err := convertJSON(src.Spec.Introspection, &introspection); err != nil {
			return fmt.Errorf("converting spec.introspection: %w", err)
		}
		t.Spec.Introspection = &introspection
	}

	t.Status.Phase = src.Status.Phase
	t.Status.Message = src.Status.Message
	t.Status.Conditions = src.Status.Conditions
	t.Status.ObservedGeneration = src.Status.ObservedGeneration
	t.Status.AppliedConfigHash = src.Status.AppliedConfigHash
	t.Status.ResolvedImage = src.Status.ResolvedImage
	if src.Status.Bot != nil {
		var bot BotStatus
		if err := convertJSON(src.Status.Bot, &bot); err != nil {
			return fmt.Errorf("converting status.bot: %w", err)
		}
		t.Status.Bot = &bot
	}

	return nil
}

// convertJSON round-trips src through JSON to populate dst - used for
// sub-structs that are field-for-field identical between v1alpha1 and
// v1beta1 (TBAppConfig and everything under it, IntrospectionSpec,
// BotStatus): each version needs its own locally-defined Go type (a type
// alias across the two packages isn't available - see
// api/v1beta1/tradebot_types.go's own doc comment on why), but the actual
// conversion is a pure pass-through as long as both copies' json tags
// agree, which is true by construction for every use here and is exactly
// what this file's round-trip fuzz tests verify - a tag mismatch fails a
// fuzz round-trip immediately rather than sitting undetected.
func convertJSON(src, dst any) error {
	data, err := json.Marshal(src)
	if err != nil {
		return fmt.Errorf("marshaling for conversion: %w", err)
	}
	return json.Unmarshal(data, dst)
}
