package configbuilder

import (
	"github.com/ark-sys/freqtrade-operator/api/v1alpha1"
)

// BuildOrderTypesConfig builds the "order_types" section for Freqtrade config.json
func BuildOrderTypesConfig(order *v1alpha1.Order) map[string]interface{} {
	if order == nil || order.Spec.Types == nil {
		return nil
	}
	types := order.Spec.Types
	cfg := map[string]interface{}{}
	if types.Entry != "" {
		cfg["entry"] = types.Entry
	}
	if types.Exit != "" {
		cfg["exit"] = types.Exit
	}
	if types.EmergencyExit != "" {
		cfg["emergency_exit"] = types.EmergencyExit
	}
	if types.ForceEntry != "" {
		cfg["force_entry"] = types.ForceEntry
	}
	if types.ForceExit != "" {
		cfg["force_exit"] = types.ForceExit
	}
	if types.Stoploss != "" {
		cfg["stoploss"] = types.Stoploss
	}
	if types.StoplossOnExchange != nil {
		cfg["stoploss_on_exchange"] = *types.StoplossOnExchange
	}
	if types.StoplossOnExchangeInterval != nil {
		cfg["stoploss_on_exchange_interval"] = *types.StoplossOnExchangeInterval
	}
	if types.StoplossOnExchangeLimitRatio != nil {
		cfg["stoploss_on_exchange_limit_ratio"] = *types.StoplossOnExchangeLimitRatio
	}
	return cfg
}

// BuildOrderTimeInForceConfig builds the "order_time_in_force" section
func BuildOrderTimeInForceConfig(order *v1alpha1.Order) map[string]interface{} {
	if order == nil || order.Spec.TimeInForce == nil {
		return nil
	}
	tif := order.Spec.TimeInForce
	cfg := map[string]interface{}{}
	if tif.Entry != "" {
		cfg["entry"] = tif.Entry
	}
	if tif.Exit != "" {
		cfg["exit"] = tif.Exit
	}
	return cfg
}

// BuildOrderflowConfig builds the "orderflow" section
func BuildOrderflowConfig(order *v1alpha1.Order) map[string]interface{} {
	if order == nil || order.Spec.Flow == nil {
		return nil
	}
	flow := order.Spec.Flow
	cfg := map[string]interface{}{}
	if flow.CacheSize != 0 {
		cfg["cache_size"] = flow.CacheSize
	}
	if flow.MaxCandles != 0 {
		cfg["max_candles"] = flow.MaxCandles
	}
	if flow.Scale != 0 {
		cfg["scale"] = flow.Scale
	}
	if flow.StackedImbalanceRange != 0 {
		cfg["stacked_imbalance_range"] = flow.StackedImbalanceRange
	}
	if flow.ImbalanceVolume != 0 {
		cfg["imbalance_volume"] = flow.ImbalanceVolume
	}
	if flow.ImbalanceRatio != 0 {
		cfg["imbalance_ratio"] = flow.ImbalanceRatio
	}
	return cfg
}
