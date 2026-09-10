package v1alpha1

import (
	ctrl "sigs.k8s.io/controller-runtime"
)

// SetupWebhookWithManager registers FreqUI with the manager's webhook
// server. FreqUI has no admission validation (no +kubebuilder:webhook
// marker here, unlike TradeBot/TradeBotConfig/Strategy) - this exists
// solely so the shared /convert endpoint (config/crd/patches/
// conversion_in_frequis.yaml) has a Convertible type registered to answer
// v1alpha1<->v1beta1 conversion requests for (B3). See
// api/v1alpha1/frequi_conversion.go.
func (f *FreqUI) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(f).
		Complete()
}
