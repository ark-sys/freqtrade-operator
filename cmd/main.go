/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/certwatcher"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	freqtradev1beta1 "github.com/ark-sys/freqtrade-operator/api/v1beta1"
	"github.com/ark-sys/freqtrade-operator/controllers/backtest"
	"github.com/ark-sys/freqtrade-operator/controllers/backtest/collectresults"
	backtestresources "github.com/ark-sys/freqtrade-operator/controllers/backtest/resources"
	"github.com/ark-sys/freqtrade-operator/controllers/frequi"
	"github.com/ark-sys/freqtrade-operator/controllers/shared"
	"github.com/ark-sys/freqtrade-operator/controllers/strategy"
	"github.com/ark-sys/freqtrade-operator/controllers/tradebot"
	"github.com/ark-sys/freqtrade-operator/controllers/tradebotconfig"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")

	// version is stamped in at build time via -ldflags "-X main.version=..."
	// (see the Dockerfile and Makefile's docker-build target) - P3-5, so
	// what's actually running is identifiable without a shell to run
	// `manager --version` against (the distroless final image has none).
	// "dev" is what a plain `go build`/`go run` (no ldflags) gets.
	version = "dev"
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(freqtradev1alpha1.AddToScheme(scheme))
	utilruntime.Must(freqtradev1beta1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	// The operator's own image also runs the P6-2 results-collection
	// sidecar, via this same binary rather than a second image to build,
	// scan, and release - dispatched on argv[1] rather than a flag mixed
	// into the manager's own flag.CommandLine, so `--help` and every
	// existing flag stay exactly as they were for the normal manager path.
	if len(os.Args) > 1 && os.Args[1] == "collect-results" {
		if err := runCollectResults(os.Args[2:]); err != nil {
			ctrl.Log.WithName("collect-results").Error(err, "failed")
			os.Exit(1)
		}
		return
	}
	runManager()
}

// nolint:gocyclo
func runManager() {
	var metricsAddr string
	var metricsCertPath, metricsCertName, metricsCertKey string
	var webhookCertPath, webhookCertName, webhookCertKey string
	var enableLeaderElection bool
	var probeAddr string
	var secureMetrics bool
	var enableHTTP2 bool
	var tlsOpts []func(*tls.Config)
	var tradeBotFinalizerGracePeriod time.Duration
	var tradeBotMaxConcurrentReconciles int
	var defaultFreqtradeImage string
	var botPollWorkers int
	flag.DurationVar(&tradeBotFinalizerGracePeriod, "tradebot-finalizer-grace-period", 2*time.Minute,
		"How long to wait for a deleted TradeBot's StatefulSet to scale down before removing its finalizer anyway.")
	flag.IntVar(&tradeBotMaxConcurrentReconciles, "tradebot-max-concurrent-reconciles", 4,
		"How many TradeBots the TradeBot controller reconciles in parallel.")
	flag.StringVar(&defaultFreqtradeImage, "default-freqtrade-image", shared.DefaultFreqtradeImage,
		"The freqtrade image reference used when a TradeBot's spec.app.pod.image doesn't override it. "+
			"Should be digest-pinned so a pod restart can't silently change what version is running.")
	flag.IntVar(&botPollWorkers, "bot-poll-workers", 4,
		"How many TradeBots' freqtrade REST APIs the bot poller (P4-3) can poll concurrently.")
	var operatorImage string
	flag.StringVar(&operatorImage, "operator-image", "",
		"The operator's own image reference, used to run the Backtest results-collection sidecar (P6-2). "+
			"Empty means auto-detect from this Pod's own \"manager\" container via the Kubernetes API "+
			"(POD_NAME/POD_NAMESPACE, set automatically in the shipped manifests).")
	flag.StringVar(&metricsAddr, "metrics-bind-address", "0", "The address the metrics endpoint binds to. "+
		"Use :8443 for HTTPS or :8080 for HTTP, or leave as 0 to disable the metrics service.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", true,
		"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
	flag.StringVar(&webhookCertPath, "webhook-cert-path", "", "The directory that contains the webhook certificate.")
	flag.StringVar(&webhookCertName, "webhook-cert-name", "tls.crt", "The name of the webhook certificate file.")
	flag.StringVar(&webhookCertKey, "webhook-cert-key", "tls.key", "The name of the webhook key file.")
	flag.StringVar(&metricsCertPath, "metrics-cert-path", "",
		"The directory that contains the metrics server certificate.")
	flag.StringVar(&metricsCertName, "metrics-cert-name", "tls.crt", "The name of the metrics server certificate file.")
	flag.StringVar(&metricsCertKey, "metrics-cert-key", "tls.key", "The name of the metrics server key file.")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	// Development: false is the default deployed to production - structured
	// JSON output, info level. Pass -zap-devel=true for human-readable
	// console output while developing locally; opts.BindFlags below already
	// registers that flag (along with -zap-log-level, -zap-encoder, etc.).
	opts := zap.Options{
		Development: false,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	// Create watchers for metrics and webhooks certificates
	var metricsCertWatcher, webhookCertWatcher *certwatcher.CertWatcher

	// Initial webhook TLS options
	webhookTLSOpts := tlsOpts

	if len(webhookCertPath) > 0 {
		setupLog.Info("Initializing webhook certificate watcher using provided certificates",
			"webhook-cert-path", webhookCertPath, "webhook-cert-name", webhookCertName, "webhook-cert-key", webhookCertKey)

		var err error
		webhookCertWatcher, err = certwatcher.New(
			filepath.Join(webhookCertPath, webhookCertName),
			filepath.Join(webhookCertPath, webhookCertKey),
		)
		if err != nil {
			setupLog.Error(err, "Failed to initialize webhook certificate watcher")
			os.Exit(1)
		}

		webhookTLSOpts = append(webhookTLSOpts, func(config *tls.Config) {
			config.GetCertificate = webhookCertWatcher.GetCertificate
		})
	}

	webhookServer := webhook.NewServer(webhook.Options{
		TLSOpts: webhookTLSOpts,
	})

	// Metrics endpoint is enabled in 'config/default/kustomization.yaml'. The Metrics options configure the server.
	// More info:
	// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/metrics/server
	// - https://book.kubebuilder.io/reference/metrics.html
	metricsServerOptions := metricsserver.Options{
		BindAddress:   metricsAddr,
		SecureServing: secureMetrics,
		TLSOpts:       tlsOpts,
	}

	if secureMetrics {
		// FilterProvider is used to protect the metrics endpoint with authn/authz.
		// These configurations ensure that only authorized users and service accounts
		// can access the metrics endpoint. The RBAC are configured in 'config/rbac/kustomization.yaml'. More info:
		// https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/metrics/filters#WithAuthenticationAndAuthorization
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	// If the certificate is not specified, controller-runtime will automatically
	// generate self-signed certificates for the metrics server. While convenient for development and testing,
	// this setup is not recommended for production.
	//
	// TODO(user): If you enable certManager, uncomment the following lines:
	// - [METRICS-WITH-CERTS] at config/default/kustomization.yaml to generate and use certificates
	// managed by cert-manager for the metrics server.
	// - [PROMETHEUS-WITH-CERTS] at config/prometheus/kustomization.yaml for TLS certification.
	if len(metricsCertPath) > 0 {
		setupLog.Info("Initializing metrics certificate watcher using provided certificates",
			"metrics-cert-path", metricsCertPath, "metrics-cert-name", metricsCertName, "metrics-cert-key", metricsCertKey)

		var err error
		metricsCertWatcher, err = certwatcher.New(
			filepath.Join(metricsCertPath, metricsCertName),
			filepath.Join(metricsCertPath, metricsCertKey),
		)
		if err != nil {
			setupLog.Error(err, "to initialize metrics certificate watcher", "error", err)
			os.Exit(1)
		}

		metricsServerOptions.TLSOpts = append(metricsServerOptions.TLSOpts, func(config *tls.Config) {
			config.GetCertificate = metricsCertWatcher.GetCertificate
		})
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsServerOptions,
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "bca373d0.freqtrade.io",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	metrics.Registry.MustRegister(shared.NewTradeBotCollector(mgr.GetClient()))

	resolvedOperatorImage := operatorImage
	if resolvedOperatorImage == "" {
		resolvedOperatorImage = resolveOwnImage(mgr.GetAPIReader())
	}

	// Setup TradeBot controller
	if err = (&tradebot.Reconciler{
		Client:                  mgr.GetClient(),
		Scheme:                  mgr.GetScheme(),
		FinalizerGracePeriod:    tradeBotFinalizerGracePeriod,
		MaxConcurrentReconciles: tradeBotMaxConcurrentReconciles,
		Recorder:                mgr.GetEventRecorderFor("tradebot-controller"),
		DefaultImage:            defaultFreqtradeImage,
		OperatorNamespace:       os.Getenv("POD_NAMESPACE"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "TradeBot")
		os.Exit(1)
	}

	// Setup FreqUI controller
	if err = (&frequi.Reconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor("frequi-controller"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "FreqUI")
		os.Exit(1)
	}

	// Setup Strategy controller
	if err = (&strategy.Reconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor("strategy-controller"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Strategy")
		os.Exit(1)
	}

	// Setup TradeBotConfig controller
	if err = (&tradebotconfig.Reconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor("tradebotconfig-controller"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "TradeBotConfig")
		os.Exit(1)
	}

	// Setup Backtest controller (P6-1)
	if err = (&backtest.Reconciler{
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		Recorder:      mgr.GetEventRecorderFor("backtest-controller"),
		DefaultImage:  defaultFreqtradeImage,
		OperatorImage: resolvedOperatorImage,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Backtest")
		os.Exit(1)
	}

	// Set up admission webhooks
	if err = (&freqtradev1alpha1.TradeBot{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "TradeBot")
		os.Exit(1)
	}
	if err = (&freqtradev1alpha1.TradeBotConfig{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "TradeBotConfig (v1alpha1)")
		os.Exit(1)
	}
	// B2: v1beta1 carries the structural validation the v1alpha1 webhook
	// used to own (see api/v1beta1/tradebotconfig_webhook.go's own doc
	// comment) - both are registered, since both versions are still served.
	if err = (&freqtradev1beta1.TradeBotConfig{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "TradeBotConfig (v1beta1)")
		os.Exit(1)
	}
	if err = (&freqtradev1alpha1.Strategy{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "Strategy")
		os.Exit(1)
	}
	// FreqUI has no admission validation of its own - this registers it
	// with the webhook server solely to activate the shared /convert
	// endpoint (B3), same reasoning as api/v1alpha1/frequi_webhook.go's
	// own doc comment.
	if err = (&freqtradev1alpha1.FreqUI{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "FreqUI")
		os.Exit(1)
	}
	if err = (&freqtradev1beta1.Backtest{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "Backtest")
		os.Exit(1)
	}

	// +kubebuilder:scaffold:builder

	if metricsCertWatcher != nil {
		setupLog.Info("Adding metrics certificate watcher to manager")
		if err := mgr.Add(metricsCertWatcher); err != nil {
			setupLog.Error(err, "unable to add metrics certificate watcher to manager")
			os.Exit(1)
		}
	}

	if webhookCertWatcher != nil {
		setupLog.Info("Adding webhook certificate watcher to manager")
		if err := mgr.Add(webhookCertWatcher); err != nil {
			setupLog.Error(err, "unable to add webhook certificate watcher to manager")
			os.Exit(1)
		}
	}

	if err := mgr.Add(&tradebot.BotPoller{
		Client:   mgr.GetClient(),
		Recorder: mgr.GetEventRecorderFor("tradebot-poller"),
		Workers:  botPollWorkers,
	}); err != nil {
		setupLog.Error(err, "unable to add TradeBot poller to manager")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager", "version", version)
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

// runCollectResults is the P6-2 results-collection sidecar's entry point -
// see controllers/backtest/collectresults for what it actually does. A
// fresh flag.FlagSet, not the package-level flag.CommandLine the manager
// path already populated: this process only ever runs one or the other,
// never both, so there's no risk of the two flag sets colliding, and
// keeping them separate means `manager --help` (the normal path) never
// lists sidecar-only flags nobody running it as a manager would recognize.
func runCollectResults(args []string) error {
	ctrl.SetLogger(zap.New())

	fs := flag.NewFlagSet("collect-results", flag.ExitOnError)
	backtestName := fs.String("backtest-name", "", "The Backtest this run belongs to (required).")
	strategyName := fs.String("strategy-name", "", "The strategy name to look up in the result file (required).")
	resultsDir := fs.String("results-dir", "/freqtrade/user_data/backtest_results",
		"Where freqtrade writes backtest-result-*.json and .last_result.json.")
	resultsPVCDir := fs.String("results-pvc-dir", backtestresources.ResultsPVCMountPath,
		"Where the results PVC is mounted - the raw result file is copied here so it survives the Job pod.")
	mainContainer := fs.String("main-container", "freqtrade", "The main container this waits to see exit.")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *backtestName == "" || *strategyName == "" {
		return fmt.Errorf("--backtest-name and --strategy-name are both required")
	}

	return collectresults.Run(context.Background(), collectresults.Options{
		BacktestName:      *backtestName,
		StrategyName:      *strategyName,
		PodName:           os.Getenv("POD_NAME"),
		Namespace:         os.Getenv("POD_NAMESPACE"),
		MainContainerName: *mainContainer,
		ResultsDir:        *resultsDir,
		ResultsPVCDir:     *resultsPVCDir,
	})
}

// resolveOwnImage reads back this manager's own "manager" container image
// from its own Pod (POD_NAME/POD_NAMESPACE, downward API - set in both the
// kustomize and Helm manifests) - used only when --operator-image isn't
// set explicitly, to run the Backtest results-collection sidecar (P6-2)
// from the exact same image without hardcoding it anywhere.
//
// reader is the manager's uncached API reader, not its cached client: this
// runs before mgr.Start(), when the cache isn't populated yet. A failure
// here is never fatal to the rest of the operator - only logged - since
// every other controller works fine without it; an empty result just means
// Backtest's own sidecar container ends up with an empty image, which
// fails loudly at Job creation rather than silently doing the wrong thing.
func resolveOwnImage(reader client.Reader) string {
	podName, podNamespace := os.Getenv("POD_NAME"), os.Getenv("POD_NAMESPACE")
	if podName == "" || podNamespace == "" {
		setupLog.Info("POD_NAME/POD_NAMESPACE not set - skipping self-image detection for the Backtest sidecar " +
			"(pass --operator-image explicitly if this manager isn't running as a normal Deployment-managed Pod)")
		return ""
	}

	var pod corev1.Pod
	key := client.ObjectKey{Name: podName, Namespace: podNamespace}
	if err := reader.Get(context.Background(), key, &pod); err != nil {
		setupLog.Error(err, "failed to read own Pod for self-image detection")
		return ""
	}
	for _, c := range pod.Spec.Containers {
		if c.Name == "manager" {
			return c.Image
		}
	}
	setupLog.Info("no \"manager\" container found on own Pod - skipping self-image detection")
	return ""
}
