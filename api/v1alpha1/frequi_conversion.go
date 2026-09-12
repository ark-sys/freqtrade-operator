package v1alpha1

import (
	"fmt"

	"github.com/ark-sys/freqtrade-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo converts this v1alpha1 FreqUI to the v1beta1 Hub (B3).
// TradeBotRefs can't fail to convert - it's a list of bare names, and every
// bare name is representable as a corev1.LocalObjectReference with that
// name. spec.gateway (G1-1, GATEWAY-API-PLAN.md) goes through convertJSON
// like spec.app, since ParentRefs/Hostnames are the same upstream
// gatewayv1 types in both versions and only the surrounding struct type
// differs by package.
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
	dst.Spec.Exposure = v1beta1.FUExposureType(f.Spec.Exposure)
	if f.Spec.Gateway != nil {
		var gw v1beta1.FUGatewaySpec
		if err := convertJSON(f.Spec.Gateway, &gw); err != nil {
			return fmt.Errorf("converting spec.gateway: %w", err)
		}
		dst.Spec.Gateway = &gw
	}

	dst.Status.Phase = f.Status.Phase
	dst.Status.Message = f.Status.Message
	dst.Status.URL = f.Status.URL
	dst.Status.Conditions = f.Status.Conditions
	dst.Status.ObservedGeneration = f.Status.ObservedGeneration

	return nil
}

// ConvertFrom converts the v1beta1 Hub to this v1alpha1 FreqUI.
// TradeBotRefs can't fail to convert - every corev1.LocalObjectReference's
// Name is representable as a bare string. spec.gateway goes through
// convertJSON, same as ConvertTo.
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
	f.Spec.Exposure = FUExposureType(src.Spec.Exposure)
	if src.Spec.Gateway != nil {
		var gw FUGatewaySpec
		if err := convertJSON(src.Spec.Gateway, &gw); err != nil {
			return fmt.Errorf("converting spec.gateway: %w", err)
		}
		f.Spec.Gateway = &gw
	}

	f.Status.Phase = src.Status.Phase
	f.Status.Message = src.Status.Message
	f.Status.URL = src.Status.URL
	f.Status.Conditions = src.Status.Conditions
	f.Status.ObservedGeneration = src.Status.ObservedGeneration

	return nil
}
