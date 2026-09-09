package configbuilder

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var updateGolden = flag.Bool("update", false, "update golden files instead of comparing against them")

func ptrBool(b bool) *bool          { return &b }
func ptrInt(i int) *int             { return &i }
func ptrInt64(i int64) *int64       { return &i }
func ptrFloat64(f float64) *float64 { return &f }

// fullTradeBotConfigFixture returns a TradeBotConfig with every optional
// section populated, exercising as much of BuildConfig's rendering logic as
// a single example can. It's the basis for the golden-file test below.
func fullTradeBotConfigFixture() *v1alpha1.TradeBotConfig {
	return &v1alpha1.TradeBotConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "full-config", Namespace: "trading"},
		Spec: v1alpha1.TradeBotConfigSpec{
			Bot: &v1alpha1.BotConfig{
				BotName:                "my-bot",
				TradingMode:            "spot",
				DryRun:                 ptrBool(true),
				DryRunWallet:           ptrFloat64(1000),
				StakeCurrency:          "USDT",
				StakeAmount:            "unlimited",
				MaxOpenTrades:          ptrInt(5),
				FiatDisplayCurrency:    "USD",
				DBUrl:                  "sqlite:////freqtrade/user_data/tradesv3.sqlite",
				Export:                 "trades",
				DisableParamExport:     ptrBool(false),
				DisableDataframeChecks: ptrBool(false),
			},
			AI: &v1alpha1.AIConfig{
				Enabled:                          ptrBool(true),
				Identifier:                       "freqai-model",
				WriteMetricsToDisk:               ptrBool(true),
				PurgeOldModels:                   ptrInt(2),
				ConvWidth:                        ptrInt(2),
				TrainPeriodDays:                  ptrInt(30),
				BacktestPeriodDays:               ptrInt(7),
				LiveRetrainHours:                 ptrInt(0),
				ExpirationHours:                  ptrInt(1),
				SaveBacktestModels:               ptrBool(true),
				FitLivePredictionsCandles:        ptrInt(100),
				DataKitchenThreadCount:           ptrInt(4),
				ActivateTensorboard:              ptrBool(false),
				WaitForTrainingIterationOnReload: ptrBool(true),
				ContinueLearning:                 ptrBool(false),
				Keras:                            ptrBool(false),
				FeatureParameters: &v1alpha1.FeatureParameters{
					IncludeCorrPairlist:        []string{"BTC/USDT", "ETH/USDT"},
					IncludeTimeframes:          []string{"5m", "15m"},
					LabelPeriodCandles:         ptrInt(20),
					IncludeShiftedCandles:      ptrInt(2),
					DIThreshold:                ptrFloat64(0.5),
					WeightFactor:               ptrFloat64(0.9),
					PrincipalComponentAnalysis: ptrBool(false),
					IndicatorPeriodsCandles:    []int{10, 20},
					UseSVMToRemoveOutliers:     ptrBool(true),
					PlotFeatureImportances:     ptrInt(1),
					SVMParams: &v1alpha1.SVMParams{
						Shuffle: ptrBool(true),
						Nu:      ptrFloat64(0.1),
					},
					ShuffleAfterSplit:      ptrBool(false),
					BufferTrainDataCandles: ptrInt(5),
				},
				DataSplitParameters: &v1alpha1.DataSplitParameters{
					TestSize:    ptrFloat64(0.25),
					RandomState: ptrInt(42),
					Shuffle:     ptrBool(false),
				},
				ModelTrainingParameters: &v1alpha1.ModelTrainingParameters{},
				RLConfig: &v1alpha1.RLConfig{
					DropOHLCFromFeatures:      ptrBool(false),
					TrainCycles:               ptrInt(10),
					MaxTradeDurationCandles:   ptrInt(300),
					AddStateInfo:              ptrBool(true),
					MaxTrainingDrawdownPct:    ptrFloat64(0.8),
					CPUCount:                  ptrInt(4),
					ModelType:                 "PPO",
					PolicyType:                "MlpPolicy",
					NetArch:                   []int{128, 128},
					RandomizeStartingPosition: ptrBool(true),
					ProgressBar:               ptrBool(true),
					ModelRewardParameters: &v1alpha1.ModelRewardParameters{
						RR:        ptrFloat64(1),
						ProfitAim: ptrFloat64(0.02),
					},
				},
			},
			Data: &v1alpha1.DataConfig{
				DataformatOHLCV:             "feather",
				DataformatTrades:            "jsongz",
				PositionAdjustment:          "adjust",
				NewPairsDaysAgo:             ptrInt(60),
				DownloadTrades:              ptrBool(false),
				MaxEntryPositionAdjustment:  ptrFloat64(1),
				AvailableCapital:            ptrFloat64(10000),
				AmendLastStakeAmount:        ptrBool(true),
				LastStakeAmountMinRatio:     ptrFloat64(0.5),
				ProcessOnlyNewCandles:       ptrBool(true),
				AmountReservePercent:        ptrFloat64(0.05),
				ReduceDfFootprint:           ptrBool(false),
				CustomPriceMaxDistanceRatio: ptrFloat64(0.02),
			},
			Advanced: &v1alpha1.AdvancedConfig{
				TradableBalanceRatio:   ptrFloat64(0.99),
				CancelOpenOrdersOnExit: ptrBool(true),
				MarginMode:             "isolated",
				InitialState:           "running",
				ForceEntryEnable:       ptrBool(false),
			},
			Timeout: &v1alpha1.UnfilledTimeoutConfig{
				Entry:            ptrInt(10),
				Exit:             ptrInt(10),
				ExitTimeoutCount: ptrInt(0),
				Unit:             "minutes",
			},
			Internals: &v1alpha1.InternalsConfig{
				ProcessThrottleSecs: ptrInt(5),
				Interval:            ptrInt(60),
				SdNotify:            ptrBool(false),
			},
			Exchange: &v1alpha1.ExchangeSpec{
				Name:                  "binance",
				SecretRef:             "exchange-creds",
				LogResponses:          ptrBool(false),
				EnableWS:              ptrBool(true),
				UnkownFeeRate:         ptrBool(false),
				OutdatedOffset:        ptrInt(5),
				MarketRefreshInterval: ptrInt(60),
				CcxtConfig:            apiextensionsv1.JSON{Raw: []byte(`{"enableRateLimit":true}`)},
				Whitelist:             &v1alpha1.PairListSpec{Pairs: []string{"BTC/USDT", "ETH/USDT"}},
				Blacklist:             &v1alpha1.PairListSpec{Pairs: []string{"SCAM/USDT"}},
			},
			PairlistMethod: &v1alpha1.PairlistMethodsSpec{
				Methods: []v1alpha1.PairlistConfig{
					{Method: v1alpha1.StaticPairList, AllowInactive: true},
					{Method: v1alpha1.VolumePairList, NumberAssets: ptrInt(20), RefreshPeriod: ptrInt64(60), SortKey: "quoteVolume"},
					{Method: v1alpha1.AgeFilter, MinDaysListed: ptrInt(10)},
				},
			},
			EntryPricing: &v1alpha1.PricingSpec{
				PriceSide:        "same",
				PriceLastBalance: ptrFloat64(0),
				UseOrderBook:     ptrBool(true),
				OrderBookTop:     ptrInt(1),
				CheckDepthOfMarket: &v1alpha1.PricingCheckDepthOfMarket{
					Enabled:        ptrBool(true),
					BidsToAskDelta: ptrFloat64(1),
				},
			},
			ExitPricing: &v1alpha1.PricingSpec{
				PriceSide:    "other",
				UseOrderBook: ptrBool(true),
				OrderBookTop: ptrInt(1),
			},
			Order: &v1alpha1.OrderSpec{
				Types: &v1alpha1.Types{
					Entry:                        "limit",
					Exit:                         "limit",
					EmergencyExit:                "market",
					ForceEntry:                   "market",
					ForceExit:                    "market",
					Stoploss:                     "market",
					StoplossOnExchange:           ptrBool(false),
					StoplossOnExchangeInterval:   ptrInt(60),
					StoplossOnExchangeLimitRatio: ptrFloat64(0.99),
				},
				TimeInForce: &v1alpha1.TimeInForce{Entry: "GTC", Exit: "GTC"},
				Flow: &v1alpha1.Flow{
					CacheSize:             ptrInt(1000),
					MaxCandles:            ptrInt(1500),
					Scale:                 ptrFloat64(0.5),
					StackedImbalanceRange: ptrInt(3),
					ImbalanceVolume:       ptrInt(1),
					ImbalanceRatio:        ptrFloat64(3),
				},
			},
			RiskManagement: &v1alpha1.RiskManagementSpec{
				MinimalROI:                      map[string]*float64{"0": ptrFloat64(0.1)},
				Stoploss:                        ptrFloat64(-0.1),
				TrailingStop:                    true,
				TrailingStopPositive:            ptrFloat64(0.01),
				TrailingStopPositiveOffset:      ptrFloat64(0.02),
				TrailingOnlyOffsetIsReached:     true,
				UseExitSignal:                   true,
				ExitProfitOnly:                  true,
				ExitProfitOffset:                ptrFloat64(0.01),
				Fee:                             ptrFloat64(0.001),
				IgnoreRoiIfEntrySignal:          true,
				IgnoreBuyingExpiredCandleAfter:  ptrInt(300),
				MinimumTradeAmount:              ptrInt(10),
				TargetedTradeAmount:             ptrInt(20),
				LookaheadAnalysisExportFilename: "lookahead.txt",
				StartupCandle:                   &[]int{200, 400},
				LiquidationBuffer:               ptrFloat64(0.05),
				BacktestBreakdown:               []string{"day", "week"},
			},
			Notification: &v1alpha1.NotificationSpec{
				Telegram: &v1alpha1.NotificationTelegram{
					Enabled:             ptrBool(true),
					SecretRef:           "telegram-creds",
					BalanceDustLevel:    ptrFloat64(0.01),
					Reload:              ptrBool(true),
					AllowCustomMessages: ptrBool(true),
					TopicID:             "42",
					AuthorizedUsers:     []string{"12345"},
					Settings: &v1alpha1.NotificationTelegramSettings{
						Status:                  "on",
						Warning:                 "on",
						Startup:                 "on",
						Entry:                   "on",
						EntryFill:               "on",
						EntryCancel:             "on",
						Exit:                    "on",
						ExitFill:                "on",
						ExitCancel:              "on",
						ProtectionTrigger:       "on",
						ProtectionTriggerGlobal: "on",
					},
				},
				Webhook: &v1alpha1.NotificationWebhook{
					Enabled:             ptrBool(true),
					URL:                 "https://hooks.example.com/notify",
					Entry:               "entry",
					EntryCancel:         "entry_cancel",
					EntryFill:           "entry_fill",
					Exit:                "exit",
					ExitCancel:          "exit_cancel",
					ExitFill:            "exit_fill",
					Status:              "status",
					AllowCustomMessages: ptrBool(true),
				},
				Discord: &v1alpha1.NotificationDiscord{
					Enabled:    ptrBool(true),
					WebhookURL: "https://discord.example.com/webhook",
					ExitFill:   []map[string]string{{"Trade ID": "{trade_id}"}},
					EntryFill:  []map[string]string{{"Trade ID": "{trade_id}"}},
				},
			},
			APIServer: &v1alpha1.APIServerConfig{
				Enabled:       ptrBool(true),
				ListenIP:      "0.0.0.0",
				ListenPort:    ptrInt(8080),
				Verbosity:     "error",
				EnableOpenAPI: ptrBool(false),
				SecretRef:     "api-creds",
				CORSOrigins:   []string{"http://localhost:8080"},
			},
			Experimental: &v1alpha1.ExperimentalConfig{BlockBadExchanges: ptrBool(true)},
			Logging:      &v1alpha1.LoggingConfig{Version: ptrInt(1)},
		},
	}
}

// TestBuildConfig_GoldenFullExample renders a TradeBotConfig with every
// optional section populated and compares it against a checked-in golden
// file. Run `go test ./controllers/tradebotconfig/configbuilder/... -run
// GoldenFullExample -update` to regenerate the golden file after a
// deliberate rendering change.
func TestBuildConfig_GoldenFullExample(t *testing.T) {
	scheme := newTestScheme(t)
	exchangeSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "exchange-creds", Namespace: "trading"},
		Data: map[string][]byte{
			"api-key": []byte("test-api-key"),
			"secret":  []byte("test-api-secret"),
		},
	}
	telegramSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "telegram-creds", Namespace: "trading"},
		Data: map[string][]byte{
			"token":   []byte("test-telegram-token"),
			"chat-id": []byte("test-chat-id"),
		},
	}
	apiSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "api-creds", Namespace: "trading"},
		Data: map[string][]byte{
			"user":           []byte("test-user"),
			"password":       []byte("test-password"),
			"jwt_secret_key": []byte("test-jwt-secret-well-over-the-minimum-length"),
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(exchangeSecret, telegramSecret, apiSecret).Build()

	tradeBot := &v1alpha1.TradeBot{
		ObjectMeta: metav1.ObjectMeta{Name: "full-bot", Namespace: "trading"},
		Spec:       v1alpha1.TradeBotSpec{Config: "full-config", Strategy: "SampleStrategy"},
	}
	tradeBotConfig := fullTradeBotConfigFixture()
	extraCorsHosts := []string{"https://full-bot.frequi.example.com"}

	data, err := BuildConfig(context.Background(), c, tradeBot, tradeBotConfig, extraCorsHosts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := data["config.json"]
	goldenPath := filepath.Join("testdata", "full_config.golden.json")

	if *updateGolden {
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatalf("failed to update golden file: %v", err)
		}
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("failed to read golden file (run with -update to create it): %v", err)
	}

	var gotJSON, wantJSON interface{}
	if err := json.Unmarshal([]byte(got), &gotJSON); err != nil {
		t.Fatalf("rendered config.json is not valid JSON: %v", err)
	}
	if err := json.Unmarshal(want, &wantJSON); err != nil {
		t.Fatalf("golden file is not valid JSON: %v", err)
	}

	gotCanon, _ := json.Marshal(gotJSON)
	wantCanon, _ := json.Marshal(wantJSON)
	if string(gotCanon) != string(wantCanon) {
		t.Errorf("rendered config.json does not match golden file %s.\ngot:\n%s\n\nwant:\n%s", goldenPath, got, string(want))
	}
}
