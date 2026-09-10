package backtest

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
	RunSpecs(t, "Backtest Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	testCtx, testCancel = context.WithCancel(context.Background())

	// Registered before testEnv.Start(), not after: envtest's own
	// CRDInstallOptions auto-detects conversion.Convertible types (P6-5,
	// B2) by inspecting the scheme it defaults to (client-go's global
	// scheme.Scheme, same singleton as below) at CRD-install time, which
	// happens inside Start() itself - registering these afterward would
	// leave the conversion webhook unwired for this suite's own tests
	// with no error, just silently not applied. See
	// controllers/tradebot/suite_test.go's identical comment - this
	// suite's own fixtures create TradeBotConfig objects (B2 made it a
	// second multi-version kind alongside TradeBot), so it needs the same
	// ordering.
	Expect(freqtradev1alpha1.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(freqtradev1beta1.AddToScheme(scheme.Scheme)).To(Succeed())

	By("bootstrapping the envtest environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
		WebhookInstallOptions: envtest.WebhookInstallOptions{
			Paths: []string{filepath.Join("..", "..", "config", "webhook")},
		},
	}

	var err error
	testCfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(testCfg).NotTo(BeNil())

	k8sClient, err = client.New(testCfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	By("starting the Backtest controller")
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
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		Recorder:      mgr.GetEventRecorderFor("backtest-controller"),
		OperatorImage: "test-operator-image:latest",
	}
	Expect(testReconciler.SetupWithManager(mgr)).To(Succeed())

	// The Strategy and TradeBotConfig controllers don't run in this suite,
	// but this package's own fixtures create both kinds of object, and the
	// envtest-installed webhook config (shared config/webhook, covering
	// every CRD this operator has) expects a live handler for each one's
	// path regardless of which controller reconciles it - see
	// controllers/tradebot/suite_test.go's identical comment. Both of
	// TradeBotConfig's webhooks (v1alpha1's credential gate, v1beta1's
	// structural checks - B2) are needed: an admission webhook registered
	// with the default Equivalent matchPolicy applies to a request
	// regardless of which version the client used, so even a v1alpha1
	// write can reach v1beta1's handler.
	Expect((&freqtradev1alpha1.Strategy{}).SetupWebhookWithManager(mgr)).To(Succeed())
	Expect((&freqtradev1alpha1.TradeBotConfig{}).SetupWebhookWithManager(mgr)).To(Succeed())
	Expect((&freqtradev1beta1.TradeBotConfig{}).SetupWebhookWithManager(mgr)).To(Succeed())
	Expect((&freqtradev1beta1.Backtest{}).SetupWebhookWithManager(mgr)).To(Succeed())

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
