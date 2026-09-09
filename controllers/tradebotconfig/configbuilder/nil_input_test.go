package configbuilder

import "testing"

// TestBuildXConfig_NilInputReturnsNil covers the documented contract shared
// by every Build*Config function: a nil section pointer renders nothing,
// rather than panicking or emitting an empty section.
func TestBuildXConfig_NilInputReturnsNil(t *testing.T) {
	if got, err := BuildTradeBotConfig("bot", nil, nil, "", nil); got != nil || err != nil {
		t.Errorf("BuildTradeBotConfig(nil) = (%v, %v), want (nil, nil)", got, err)
	}
	if got, err := BuildExchangeConfig(nil, nil); got != nil || err != nil {
		t.Errorf("BuildExchangeConfig(nil) = (%v, %v), want (nil, nil)", got, err)
	}
	if got, err := BuildNotificationConfig(nil, nil); got != nil || err != nil {
		t.Errorf("BuildNotificationConfig(nil) = (%v, %v), want (nil, nil)", got, err)
	}
	if got := BuildOrderTypesConfig(nil); got != nil {
		t.Errorf("BuildOrderTypesConfig(nil) = %v, want nil", got)
	}
	if got := BuildOrderTimeInForceConfig(nil); got != nil {
		t.Errorf("BuildOrderTimeInForceConfig(nil) = %v, want nil", got)
	}
	if got := BuildOrderflowConfig(nil); got != nil {
		t.Errorf("BuildOrderflowConfig(nil) = %v, want nil", got)
	}
	if got := BuildPairlistMethodsConfig(nil); got != nil {
		t.Errorf("BuildPairlistMethodsConfig(nil) = %v, want nil", got)
	}
	if got := BuildPricingConfig(nil); got != nil {
		t.Errorf("BuildPricingConfig(nil) = %v, want nil", got)
	}
	if got := BuildRiskManagementConfig(nil); got != nil {
		t.Errorf("BuildRiskManagementConfig(nil) = %v, want nil", got)
	}
}
