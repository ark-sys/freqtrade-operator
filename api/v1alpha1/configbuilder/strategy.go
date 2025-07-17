package configbuilder

import (
	"github.com/ark-sys/freqtrade-operator/api/v1alpha1"
)

// BuildStrategyConfig builds the strategy-related configuration for Freqtrade config.json
func BuildStrategyConfig(strategy *v1alpha1.Strategy) map[string]interface{} {
	if strategy == nil {
		return nil
	}

	cfg := map[string]interface{}{
		"strategy": strategy.Spec.Name,
	}

	return cfg
}
