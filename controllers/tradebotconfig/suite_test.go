package tradebotconfig

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

// This suite exists for A5 (REMAINING-WORK.md): it proves SetupWithManager's
// predicate.GenerationChangedPredicate{} actually lets a spec-changing
// Update event through to Reconcile, the thing a mistake in that predicate
// would silently break. B2 added a webhook server here (both admission
// webhooks and, implicitly, the /convert endpoint) - TradeBotConfig became
// a multi-version kind, and the apiserver needs somewhere to actually send
// v1alpha1<->v1beta1 conversion requests even for a test that never
// exercises admission rejection directly.

var (
	testCtx    context.Context
	testCancel context.CancelFunc
	testEnv    *envtest.Environment
	testCfg    *rest.Config
	k8sClient  client.Client
)

func TestControllers(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping envtest-backed suite in -short mode (make test-unit)")
	}
	RegisterFailHandler(Fail)
	RunSpecs(t, "TradeBotConfig Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	testCtx, testCancel = context.WithCancel(context.Background())

	// Registered before testEnv.Start(), not after: envtest's own
	// CRDInstallOptions auto-detects conversion.Convertible types (P6-5,
	// B2) by inspecting the scheme it defaults to at CRD-install time,
	// which happens inside Start() itself - registering these afterward
	// would leave TradeBotConfig's own conversion webhook (B2 made it a
	// multi-version kind) unwired for this suite's own tests with no
	// error, just silently not applied. See
	// controllers/tradebot/suite_test.go's identical comment.
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

	By("starting the TradeBotConfig controller")
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

	testReconciler := &Reconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor("tradebotconfig-controller"),
	}
	Expect(testReconciler.SetupWithManager(mgr)).To(Succeed())

	// Both versions' validating webhooks, plus (as a side effect of either
	// being registered, since the manager's webhook server is shared) the
	// /convert endpoint the CRD's conversion strategy actually calls.
	Expect((&freqtradev1alpha1.TradeBotConfig{}).SetupWebhookWithManager(mgr)).To(Succeed())
	Expect((&freqtradev1beta1.TradeBotConfig{}).SetupWebhookWithManager(mgr)).To(Succeed())

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
