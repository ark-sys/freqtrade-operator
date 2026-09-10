package v1alpha1

import (
	"fmt"

	"github.com/ark-sys/freqtrade-operator/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo converts this v1alpha1 Strategy to the v1beta1 Hub (B3).
// Strategy carries neither credentials nor references (unlike TradeBot/
// TradeBotConfig), so this direction can't fail - Spec is field-for-field
// identical between versions.
func (s *Strategy) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta1.Strategy)

	dst.ObjectMeta = s.ObjectMeta

	var spec v1beta1.StrategySpec
	if err := convertJSON(&s.Spec, &spec); err != nil {
		return fmt.Errorf("converting spec: %w", err)
	}
	dst.Spec = spec

	dst.Status.Phase = s.Status.Phase
	dst.Status.Message = s.Status.Message
	dst.Status.Conditions = s.Status.Conditions
	dst.Status.ObservedGeneration = s.Status.ObservedGeneration

	return nil
}

// ConvertFrom converts the v1beta1 Hub to this v1alpha1 Strategy - always
// succeeds, for the same reason ConvertTo always succeeds: identical Spec
// shape both ways.
func (s *Strategy) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta1.Strategy)

	s.ObjectMeta = src.ObjectMeta

	var spec StrategySpec
	if err := convertJSON(&src.Spec, &spec); err != nil {
		return fmt.Errorf("converting spec: %w", err)
	}
	s.Spec = spec

	s.Status.Phase = src.Status.Phase
	s.Status.Message = src.Status.Message
	s.Status.Conditions = src.Status.Conditions
	s.Status.ObservedGeneration = src.Status.ObservedGeneration

	return nil
}
