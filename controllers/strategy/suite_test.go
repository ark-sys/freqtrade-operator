package strategy

import (
	"context"
	"path/filepath"
	"testing"

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

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
)

// This suite exists for A5 (REMAINING-WORK.md): it proves SetupWithManager's
// predicate.GenerationChangedPredicate{} actually lets a spec-changing
// Update event through to Reconcile, the thing a mistake in that predicate
// would silently break. No webhooks are registered - nothing here needs
// P1-4's admission validation.

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
	RunSpecs(t, "Strategy Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	testCtx, testCancel = context.WithCancel(context.Background())

	// v1beta1 is deliberately NOT registered in this suite's scheme: this
	// package's fixtures only ever create/read v1alpha1.Strategy directly
	// (Strategy has no v1beta1 yet), and registering an unrelated Hub type
	// (e.g. TradeBot/TradeBotConfig) without also standing up
	// WebhookInstallOptions + a running webhook server (see
	// controllers/tradebot/suite_test.go for what that takes) makes
	// envtest try to wire a conversion webhook this suite has nowhere to
	// serve. See controllers/frequi/suite_test.go's identical comment.
	Expect(freqtradev1alpha1.AddToScheme(scheme.Scheme)).To(Succeed())

	By("bootstrapping the envtest environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	var err error
	testCfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(testCfg).NotTo(BeNil())

	k8sClient, err = client.New(testCfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	By("starting the Strategy controller")
	mgr, err := ctrl.NewManager(testCfg, ctrl.Options{
		Scheme:  scheme.Scheme,
		Metrics: metricsserver.Options{BindAddress: "0"},
	})
	Expect(err).NotTo(HaveOccurred())

	testReconciler := &Reconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor("strategy-controller"),
	}
	Expect(testReconciler.SetupWithManager(mgr)).To(Succeed())

	go func() {
		defer GinkgoRecover()
		Expect(mgr.Start(testCtx)).To(Succeed(), "failed to run manager")
	}()
})

var _ = AfterSuite(func() {
	testCancel()
	By("tearing down the envtest environment")
	Expect(testEnv.Stop()).To(Succeed())
})
