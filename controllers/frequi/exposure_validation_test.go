package frequi

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	freqtradev1beta1 "github.com/ark-sys/freqtrade-operator/api/v1beta1"
)

// G1-1's three CEL rules on FreqUISpec (GATEWAY-API-PLAN.md). These are
// apply-time API server rejections, not admission-webhook ones - FreqUI
// carries no validating webhook (see api/v1alpha1/frequi_webhook.go) - so
// this exercises the CRD schema directly against a real apiserver, which
// is exactly what this suite's envtest is for.
var _ = Describe("FreqUI spec.exposure CEL validation (G1-1)", func() {
	gw := func() *gatewayv1.ParentReference {
		return &gatewayv1.ParentReference{Name: "my-gateway"}
	}

	It("rejects exposure: Gateway with no spec.gateway", func() {
		frequi := &freqtradev1beta1.FreqUI{
			ObjectMeta: metav1.ObjectMeta{Name: "gateway-no-spec", Namespace: testNamespace},
			Spec:       freqtradev1beta1.FreqUISpec{Exposure: freqtradev1beta1.FUExposureGateway},
		}
		err := k8sClient.Create(context.Background(), frequi)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("spec.gateway is required when spec.exposure is Gateway"))
	})

	It("rejects exposure: Gateway with spec.tls set", func() {
		frequi := &freqtradev1beta1.FreqUI{
			ObjectMeta: metav1.ObjectMeta{Name: "gateway-with-tls", Namespace: testNamespace},
			Spec: freqtradev1beta1.FreqUISpec{
				Exposure: freqtradev1beta1.FUExposureGateway,
				Gateway:  &freqtradev1beta1.FUGatewaySpec{ParentRefs: []gatewayv1.ParentReference{*gw()}},
				TLS:      []networkingv1.IngressTLS{{Hosts: []string{"frequi.example.com"}}},
			},
		}
		err := k8sClient.Create(context.Background(), frequi)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("spec.tls has no effect in Gateway mode"))
	})

	It("rejects spec.gateway set with exposure other than Gateway", func() {
		frequi := &freqtradev1beta1.FreqUI{
			ObjectMeta: metav1.ObjectMeta{Name: "ingress-with-gateway-spec", Namespace: testNamespace},
			Spec: freqtradev1beta1.FreqUISpec{
				Exposure: freqtradev1beta1.FUExposureIngress,
				Gateway:  &freqtradev1beta1.FUGatewaySpec{ParentRefs: []gatewayv1.ParentReference{*gw()}},
			},
		}
		err := k8sClient.Create(context.Background(), frequi)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("spec.gateway is only valid when spec.exposure is Gateway"))
	})

	It("accepts a well-formed Gateway-mode FreqUI", func() {
		frequi := &freqtradev1beta1.FreqUI{
			ObjectMeta: metav1.ObjectMeta{Name: "gateway-ok", Namespace: testNamespace},
			Spec: freqtradev1beta1.FreqUISpec{
				Exposure: freqtradev1beta1.FUExposureGateway,
				Gateway:  &freqtradev1beta1.FUGatewaySpec{ParentRefs: []gatewayv1.ParentReference{*gw()}},
			},
		}
		Expect(k8sClient.Create(context.Background(), frequi)).To(Succeed())
	})

	It("defaults exposure to Ingress when unset", func() {
		frequi := &freqtradev1beta1.FreqUI{
			ObjectMeta: metav1.ObjectMeta{Name: "exposure-default", Namespace: testNamespace},
		}
		Expect(k8sClient.Create(context.Background(), frequi)).To(Succeed())
		Expect(frequi.Spec.Exposure).To(Equal(freqtradev1beta1.FUExposureIngress))
	})
})
