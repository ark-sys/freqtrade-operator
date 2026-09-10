package v1alpha1

import (
	"fmt"

	"github.com/ark-sys/freqtrade-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo converts this v1alpha1 FreqUI to the v1beta1 Hub (B3). Unlike
// TradeBot/TradeBotConfig, this can't fail - TradeBotRefs is a list of
// bare names, and every bare name is representable as a
// corev1.LocalObjectReference with that name.
func (f *FreqUI) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta1.FreqUI)

	dst.ObjectMeta = f.ObjectMeta

	dst.Spec.Host = f.Spec.Host
	dst.Spec.TLS = f.Spec.TLS
	dst.Spec.IngressAnnotations = f.Spec.IngressAnnotations
	if f.Spec.App != nil {
		var app v1beta1.FUAppConfig
		if err := convertJSON(f.Spec.App, &app); err != nil {
			return fmt.Errorf("converting spec.app: %w", err)
		}
		dst.Spec.App = &app
	}
	if f.Spec.TradeBotRefs != nil {
		refs := make([]corev1.LocalObjectReference, len(f.Spec.TradeBotRefs))
		for i, name := range f.Spec.TradeBotRefs {
			refs[i] = corev1.LocalObjectReference{Name: name}
		}
		dst.Spec.TradeBotRefs = refs
	}

	dst.Status.Phase = f.Status.Phase
	dst.Status.Message = f.Status.Message
	dst.Status.URL = f.Status.URL
	dst.Status.Conditions = f.Status.Conditions
	dst.Status.ObservedGeneration = f.Status.ObservedGeneration

	return nil
}

// ConvertFrom converts the v1beta1 Hub to this v1alpha1 FreqUI - always
// succeeds: every corev1.LocalObjectReference's Name is representable as
// a bare string.
func (f *FreqUI) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta1.FreqUI)

	f.ObjectMeta = src.ObjectMeta

	f.Spec.Host = src.Spec.Host
	f.Spec.TLS = src.Spec.TLS
	f.Spec.IngressAnnotations = src.Spec.IngressAnnotations
	if src.Spec.App != nil {
		var app FUAppConfig
		if err := convertJSON(src.Spec.App, &app); err != nil {
			return fmt.Errorf("converting spec.app: %w", err)
		}
		f.Spec.App = &app
	}
	if src.Spec.TradeBotRefs != nil {
		refs := make([]string, len(src.Spec.TradeBotRefs))
		for i, ref := range src.Spec.TradeBotRefs {
			refs[i] = ref.Name
		}
		f.Spec.TradeBotRefs = refs
	}

	f.Status.Phase = src.Status.Phase
	f.Status.Message = src.Status.Message
	f.Status.URL = src.Status.URL
	f.Status.Conditions = src.Status.Conditions
	f.Status.ObservedGeneration = src.Status.ObservedGeneration

	return nil
}
