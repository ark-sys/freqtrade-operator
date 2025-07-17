package configbuilder

import (
	"github.com/ark-sys/freqtrade-operator/api/v1alpha1"
)

// BuildOrderTypesConfig builds the "order_types" section for Freqtrade config.json
func BuildOrderTypesConfig(orderTypes *v1alpha1.OrderTypes) map[string]interface{} {
	if orderTypes == nil {
		return nil
	}

	cfg := map[string]interface{}{}

	// Set basic order types
	if orderTypes.Spec.Entry != "" {
		cfg["entry"] = orderTypes.Spec.Entry
	}
	if orderTypes.Spec.Exit != "" {
		cfg["exit"] = orderTypes.Spec.Exit
	}
	if orderTypes.Spec.EmergencyExit != "" {
		cfg["emergency_exit"] = orderTypes.Spec.EmergencyExit
	}
	if orderTypes.Spec.ForceEntry != "" {
		cfg["force_entry"] = orderTypes.Spec.ForceEntry
	}
	if orderTypes.Spec.ForceExit != "" {
		cfg["force_exit"] = orderTypes.Spec.ForceExit
	}
	if orderTypes.Spec.Stoploss != "" {
		cfg["stoploss"] = orderTypes.Spec.Stoploss
	}

	// Set stoploss on exchange settings
	if orderTypes.Spec.StoplossOnExchange != nil {
		cfg["stoploss_on_exchange"] = *orderTypes.Spec.StoplossOnExchange
	}
	if orderTypes.Spec.StoplossOnExchangeInterval != nil {
		cfg["stoploss_on_exchange_interval"] = *orderTypes.Spec.StoplossOnExchangeInterval
	}
	if orderTypes.Spec.StoplossOnExchangeLimitRatio != nil {
		cfg["stoploss_on_exchange_limit_ratio"] = *orderTypes.Spec.StoplossOnExchangeLimitRatio
	}

	return cfg
}
