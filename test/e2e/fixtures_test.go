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

package e2e

import (
	"time"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	freqtradev1beta1 "github.com/ark-sys/freqtrade-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Fixture builders shared by every e2e spec file that needs a real TradeBot/
// TradeBotConfig/Strategy/Backtest/FreqUI object (P5-4: fixture builders so
// tests don't hand-construct 60-line CRs). Field values here are modeled on
// controllers/tradebotconfig/configbuilder/golden_test.go's own fixture,
// which is exercised against a golden rendered-config file - reusing that
// shape rather than a hand-guessed one.

func ptrBool(b bool) *bool { return &b }
func ptrInt(i int) *int    { return &i }

// sandboxCcxtConfig puts the exchange in ccxt's generic sandbox/testnet mode
// instead of hitting production. Verified directly (real freqtrade image,
// `trade --dry-run`, no other changes): without this, freqtrade's own
// market-data load fails at startup and its REST API server never starts
// listening at all ("connection reset by peer", not just an auth failure) -
// dry_run:true alone does not avoid needing a real, reachable exchange. The
// sandbox is exactly what "never a real exchange" (P5-3) means in practice:
// no real funds, no real credentials, but a real, freqtrade-supported
// market-data source, since freqtrade cannot start against nothing at all.
//
// The exchange itself is Bybit (api-testnet.bybit.com), not Binance
// (testnet.binance.vision) - this spec passed locally against Binance's
// testnet every time but failed in every CI run so far with the freqtrade
// container's API port never opening, consistent with Binance's
// well-documented blocking/rate-limiting of API access from major cloud
// provider IP ranges (AWS/GCP/Azure) - GitHub Actions runners are Azure VMs.
// Not yet independently confirmed against the real image the way the
// sandbox-vs-no-sandbox finding above was; if CI still fails identically
// against Bybit, that points at CI egress/DNS in general rather than at
// Binance specifically.
var sandboxCcxtConfig = apiextensionsv1.JSON{Raw: []byte(`{"sandbox":true}`)}

// sampleStrategyClassName must match sampleStrategySource's actual `class ...(IStrategy):` name -
// Strategy.spec.name isn't a free-form label, the admission webhook requires it to be a valid
// Python identifier, and freqtrade actually loads the strategy BY this name at runtime, so it
// has to be this exact class, not e.g. the CR's own (hyphenated) Kubernetes object name.
// Verified directly: a first pass reusing the K8s object name for spec.name too was rejected by
// vstrategy.kb.io with "is not a valid Python class name" before ever reaching the exchange.
const sampleStrategyClassName = "E2EProbeStrategy"

// sampleStrategySource is a minimal, deliberately indicator-library-free
// freqtrade strategy (rolling mean crossover on close price alone) - no
// ta-lib/pandas-ta dependency to go wrong, just enough logic to be a valid,
// loadable IStrategy for e2e purposes. Not meant to be a good strategy.
const sampleStrategySource = `
from freqtrade.strategy import IStrategy
from pandas import DataFrame


class E2EProbeStrategy(IStrategy):
    INTERFACE_VERSION = 3
    timeframe = "5m"
    minimal_roi = {"0": 0.10}
    stoploss = -0.10
    startup_candle_count = 20

    def populate_indicators(self, dataframe: DataFrame, metadata: dict) -> DataFrame:
        dataframe["sma_fast"] = dataframe["close"].rolling(5).mean()
        dataframe["sma_slow"] = dataframe["close"].rolling(20).mean()
        return dataframe

    def populate_entry_trend(self, dataframe: DataFrame, metadata: dict) -> DataFrame:
        dataframe.loc[dataframe["sma_fast"] > dataframe["sma_slow"], "enter_long"] = 1
        return dataframe

    def populate_exit_trend(self, dataframe: DataFrame, metadata: dict) -> DataFrame:
        dataframe.loc[dataframe["sma_fast"] < dataframe["sma_slow"], "exit_long"] = 1
        return dataframe
`

// newExchangeSecret is reused as both TradeBotConfig.Spec.Exchange.SecretRef and
// Spec.APIServer.SecretRef in these fixtures. api-key/secret are blank - the sandbox's public
// market-data endpoints (all a dry-run bot ever calls) need no real credentials, verified
// directly against the real image. user/password are real (if trivial) values instead of also
// leaving those blank: unlike the exchange side, this wasn't independently verified against a
// bare freqtrade process, so there's no direct evidence an empty Basic Auth pair is tolerated by
// api_server.enabled:true - not worth the risk for two throwaway values in a test fixture.
// Always in tradingNamespace, like every other fixture here (D3: Config/Strategy/exchange Secret
// references are same-namespace only) - no namespace parameter of its own to pass by mistake.
func newExchangeSecret(name string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: tradingNamespace},
		StringData: map[string]string{
			"api-key": "", "secret": "",
			"user": "e2e", "password": "e2e-probe-password",
		},
	}
}

// newDryRunTradeBotConfig builds a complete, dry-run, testnet-sandboxed TradeBotConfig.
func newDryRunTradeBotConfig(name, exchangeSecretRef string) *freqtradev1alpha1.TradeBotConfig {
	return &freqtradev1alpha1.TradeBotConfig{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: tradingNamespace},
		Spec: freqtradev1alpha1.TradeBotConfigSpec{
			Bot: &freqtradev1alpha1.BotConfig{
				BotName:       name,
				TradingMode:   "spot",
				DryRun:        ptrBool(true),
				StakeCurrency: "USDT",
				StakeAmount:   "100",
				MaxOpenTrades: ptrInt(1),
			},
			// freqtrade defaults to state=stopped otherwise (a deliberate freqtrade safety
			// default: don't trade until told to). Introspection calls more than just /api/v1/ping
			// (controllers/tradebot/botclient/client.go also calls count/profit/balance), and those
			// error with "trader is not running" while stopped - verified directly: a bot left at
			// the default state logged exactly that on every poll, which is what an operator-level
			// "unexpected HTTP status" actually turned out to mean, not a network/proxy problem.
			Advanced: &freqtradev1alpha1.AdvancedConfig{InitialState: "running"},
			Exchange: &freqtradev1alpha1.ExchangeSpec{
				Name:            "bybit",
				SecretRef:       exchangeSecretRef,
				CcxtConfig:      sandboxCcxtConfig,
				CcxtAsyncConfig: sandboxCcxtConfig,
				Whitelist:       &freqtradev1alpha1.PairListSpec{Pairs: []string{"BTC/USDT"}},
			},
			PairlistMethod: &freqtradev1alpha1.PairlistMethodsSpec{
				Methods: []freqtradev1alpha1.PairlistConfig{{Method: freqtradev1alpha1.StaticPairList}},
			},
			EntryPricing: &freqtradev1alpha1.PricingSpec{
				PriceSide:    "same",
				UseOrderBook: ptrBool(true),
				OrderBookTop: ptrInt(1),
			},
			ExitPricing: &freqtradev1alpha1.PricingSpec{
				PriceSide:    "other",
				UseOrderBook: ptrBool(true),
				OrderBookTop: ptrInt(1),
			},
			APIServer: &freqtradev1alpha1.APIServerConfig{
				Enabled:       ptrBool(true),
				ListenIP:      "0.0.0.0",
				ListenPort:    ptrInt(8080),
				Verbosity:     "error",
				EnableOpenAPI: ptrBool(false),
				SecretRef:     exchangeSecretRef,
			},
		},
	}
}

// newStrategy's own K8s object name can be any valid Kubernetes name (e.g. hyphenated) - it's
// spec.Name that freqtrade actually loads the strategy by at runtime, via --strategy on the
// command line (controllers/tradebot/resources/pod.go), so it's always sampleStrategyClassName
// regardless of what this object itself is called. Every fixture that needs a Strategy at all
// uses this same sampleStrategySource - there's only the one probe strategy in this suite - so
// unlike name, script content isn't a parameter here.
func newStrategy(name string) *freqtradev1alpha1.Strategy {
	return &freqtradev1alpha1.Strategy{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: tradingNamespace},
		Spec:       freqtradev1alpha1.StrategySpec{Name: sampleStrategyClassName, Script: sampleStrategySource},
	}
}

// newTradeBot builds a v1alpha1, trade-mode TradeBot (FreqtradeCommand left blank - "trade" is
// the zero value, the only mode v1beta1 has at all as of P6-4). Introspection interval is pinned
// to 10s (the admission webhook's own documented minimum) rather than left at the 60s default:
// the poller backs off exponentially up to 10x its interval (controllers/tradebot/poller.go) after
// a failed poll, and a pod's freqtrade API can be briefly unreachable right around when the pod
// first reports Ready (verified directly: a real run's first poll hit a transient 502 during that
// window, then backed off) - at the 60s default, one unlucky first poll can push the next retry out
// past what a 3-minute Eventually budget can wait for, even though the bot is healthy moments later.
func newTradeBot(name, namespace, configRef, strategyRef string) *freqtradev1alpha1.TradeBot {
	return &freqtradev1alpha1.TradeBot{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: freqtradev1alpha1.TradeBotSpec{
			Config:   configRef,
			Strategy: strategyRef,
			Introspection: &freqtradev1alpha1.IntrospectionSpec{
				Interval: metav1.Duration{Duration: 10 * time.Second},
			},
		},
	}
}

func newFreqUI(name, namespace string, tradeBotRefs []string) *freqtradev1alpha1.FreqUI {
	return &freqtradev1alpha1.FreqUI{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec:       freqtradev1alpha1.FreqUISpec{TradeBotRefs: tradeBotRefs},
	}
}

// recentTimerange returns a "YYYYMMDD-" open-ended window starting today (UTC), computed at
// call time. testnet.binance.vision's own candle history does not behave like a simple rolling
// window relative to "now" - confirmed directly, twice (curl against its own /api/v3/klines with
// startTime=0, and freqtrade's own exchange adapter logging the identical timestamp): its
// BTC/USDT 5m history began at a fixed point earlier the same day (an apparent testnet data
// reset), with nothing at all before it. A closed window computed relative to "yesterday" (this
// function's first version) can therefore land entirely before wherever the testnet's history
// actually starts on any given day, and fail with "No data found" through no fault of the
// operator's own download-data init container - which is exactly what happened. "Today,
// open-ended" is the narrowest window expressible by Timerange's own day-granularity validation
// (`^\d{8}-(\d{8})?$` - no time-of-day component) that's still guaranteed not to ask for
// anything before a same-day reset.
func recentTimerange() string {
	const dateFmt = "20060102"
	return time.Now().UTC().Format(dateFmt) + "-"
}

// newDataCachePVC is the pre-existing PVC newBacktest's spec.data.pvcName points at. Verified
// directly (read controllers/backtest/resources/pod.go and pvc.go): BuildPod only ever mounts
// spec.data.pvcName, nothing in this codebase creates the PVC it names, so a Backtest whose
// init container should actually download data has to bring its own cache PVC, exactly as a
// real user must (see also README's "Running many backtests" guide, which documents this).
// ReadWriteOnce is enough: within one Backtest's own pod, only that pod's own init container
// (RW) and main container (RO) ever mount it - RWX only matters for sharing one cache across
// multiple Backtests, which this test doesn't need.
func newDataCachePVC(name, namespace string) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("500Mi")},
			},
		},
	}
}

// newBacktest builds a v1beta1 Backtest against an already-created TradeBotConfig/Strategy and
// cache PVC (see newDataCachePVC) - same sandbox exchange requirement as a live TradeBot
// (verified: backtesting also loads market metadata from the exchange at startup, before ever
// touching historical OHLCV data). DownloadPolicy is spelled out as "always" even though that's
// also RunSpec.Data's own default - explicit here since it's the whole reason this test's cache
// PVC starts out empty and still ends up with data downloaded into it.
func newBacktest(name, namespace, configRef, strategyRef, cachePVCName string) *freqtradev1beta1.Backtest {
	return &freqtradev1beta1.Backtest{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: freqtradev1beta1.BacktestSpec{
			RunSpec: freqtradev1beta1.RunSpec{
				ConfigRef:   corev1.LocalObjectReference{Name: configRef},
				StrategyRef: corev1.LocalObjectReference{Name: strategyRef},
				Timerange:   recentTimerange(),
				Timeframe:   "5m",
				Data: &freqtradev1beta1.DataSourceSpec{
					PVCName:        cachePVCName,
					DownloadPolicy: "always",
				},
			},
			StakeAmount: "100",
		},
	}
}
