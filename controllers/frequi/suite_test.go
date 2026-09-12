package frequi

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	freqtradev1beta1 "github.com/ark-sys/freqtrade-operator/api/v1beta1"
)

// This suite is scoped to the FreqUI controller's own behavior (P2-5's
// TradeBotRefsResolved condition, added here since it needs a real
// apiserver: server-side apply, P2-1, isn't supported by the fake client).
// No admission webhooks are registered - nothing here needs P1-4's
// validation - but as of G0-1 the reconciler reads v1beta1.FreqUI (the
// storage version/Hub), so this suite now DOES need v1beta1 registered and
// a running conversion webhook server: envtest's CRDInstallOptions
// auto-detects conversion.Convertible types already registered in the
// scheme at CRD-install time (Start()) and wires the /convert endpoint to
// whatever webhook server WebhookInstallOptions describes - see
// controllers/tradebot/suite_test.go, whose shape this copies, and
// api/v1alpha1/frequi_webhook.go for why FreqUI's own SetupWebhookWithManager
// call still has to happen even though FreqUI has no admission validation.
var (
	testCtx        context.Context
	testCancel     context.CancelFunc
	testEnv        *envtest.Environment
	testCfg        *rest.Config
	k8sClient      client.Client
	testReconciler *Reconciler
)

func TestControllers(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping envtest-backed suite in -short mode (make test-unit)")
	}
	RegisterFailHandler(Fail)
	RunSpecs(t, "FreqUI Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	testCtx, testCancel = context.WithCancel(context.Background())

	// Registered before testEnv.Start(), not after - see
	// controllers/tradebot/suite_test.go's identical comment for why.
	Expect(freqtradev1alpha1.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(freqtradev1beta1.AddToScheme(scheme.Scheme)).To(Succeed())

	By("bootstrapping the envtest environment")
	// No Paths here, deliberately: this suite only needs the local serving
	// cert/host/port that WebhookInstallOptions generates regardless of
	// Paths (envtest's own conversion-webhook auto-wiring, see this
	// file's own top comment, uses those directly) - it must NOT install
	// config/webhook's admission ValidatingWebhookConfigurations, which
	// cover TradeBot/TradeBotConfig/Strategy and would otherwise reject
	// this suite's plain fixture objects against a webhook server that
	// never registers their handlers.
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
		WebhookInstallOptions: envtest.WebhookInstallOptions{},
	}

	var err error
	testCfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(testCfg).NotTo(BeNil())

	k8sClient, err = client.New(testCfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	By("starting the FreqUI controller")
	webhookInstallOpts := &testEnv.WebhookInstallOptions
	mgr, err := ctrl.NewManager(testCfg, ctrl.Options{
		Scheme:  scheme.Scheme,
		Metrics: metricsserver.Options{BindAddress: "0"},
		WebhookServer: webhook.NewServer(webhook.Options{
			Host:    webhookInstallOpts.LocalServingHost,
			Port:    webhookInstallOpts.LocalServingPort,
			CertDir: webhookInstallOpts.LocalServingCertDir,
		}),
	})
	Expect(err).NotTo(HaveOccurred())

	testReconciler = &Reconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor("frequi-controller"),
	}
	Expect(testReconciler.SetupWithManager(mgr)).To(Succeed())

	// Registers the /convert endpoint (api/v1alpha1/frequi_webhook.go) -
	// FreqUI has no admission validation, but conversion still needs a
	// Convertible type wired to the manager's webhook server.
	Expect((&freqtradev1alpha1.FreqUI{}).SetupWebhookWithManager(mgr)).To(Succeed())

	go func() {
		defer GinkgoRecover()
		Expect(mgr.Start(testCtx)).To(Succeed(), "failed to run manager")
	}()

	By("waiting for the webhook server to be ready")
	caCertPool := x509.NewCertPool()
	Expect(caCertPool.AppendCertsFromPEM(webhookInstallOpts.LocalServingCAData)).To(BeTrue())
	dialer := &net.Dialer{Timeout: time.Second}
	addrPort := fmt.Sprintf("%s:%d", webhookInstallOpts.LocalServingHost, webhookInstallOpts.LocalServingPort)
	Eventually(func() error {
		conn, err := tls.DialWithDialer(dialer, "tcp", addrPort, &tls.Config{RootCAs: caCertPool})
		if err != nil {
			return err
		}
		return conn.Close()
	}).Should(Succeed())
})

var _ = AfterSuite(func() {
	testCancel()
	By("tearing down the envtest environment")
	Expect(testEnv.Stop()).To(Succeed())
})
