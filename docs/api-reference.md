# API Reference

## Packages
- [freqtrade.io/v1alpha1](#freqtradeiov1alpha1)
- [freqtrade.io/v1beta1](#freqtradeiov1beta1)


## freqtrade.io/v1alpha1

Package v1alpha1 contains API Schema definitions for the freqtrade v1alpha1 API group

### Resource Types
- [FreqUI](#frequi)
- [FreqUIList](#frequilist)
- [Strategy](#strategy)
- [StrategyList](#strategylist)
- [TradeBot](#tradebot)
- [TradeBotConfig](#tradebotconfig)
- [TradeBotConfigList](#tradebotconfiglist)
- [TradeBotList](#tradebotlist)



#### AIConfig







_Appears in:_
- [TradeBotConfigSpec](#tradebotconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | AI configuration |  |  |
| `identifier` _string_ |  |  |  |
| `write_metrics_to_disk` _boolean_ |  |  |  |
| `purge_old_models` _integer_ |  |  |  |
| `conv_width` _integer_ |  |  |  |
| `train_period_days` _integer_ |  |  |  |
| `backtest_period_days` _integer_ |  |  |  |
| `live_retrain_hours` _integer_ |  |  |  |
| `expiration_hours` _integer_ |  |  |  |
| `save_backtest_models` _boolean_ |  |  |  |
| `fit_live_predictions_candles` _integer_ |  |  |  |
| `data_kitchen_thread_count` _integer_ |  |  |  |
| `activate_tensorboard` _boolean_ |  |  |  |
| `wait_for_training_iteration_on_reload` _boolean_ |  |  |  |
| `continue_learning` _boolean_ |  |  |  |
| `keras` _boolean_ |  |  |  |
| `feature_parameters` _[FeatureParameters](#featureparameters)_ |  |  |  |
| `data_split_parameters` _[DataSplitParameters](#datasplitparameters)_ |  |  |  |
| `model_training_parameters` _[ModelTrainingParameters](#modeltrainingparameters)_ |  |  |  |
| `rl_config` _[RLConfig](#rlconfig)_ |  |  |  |


#### APIServerConfig







_Appears in:_
- [TradeBotConfigSpec](#tradebotconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ |  |  |  |
| `listen_ip_address` _string_ |  |  |  |
| `listen_port` _integer_ |  |  | Maximum: 65535 <br />Minimum: 1 <br /> |
| `verbosity` _string_ |  |  |  |
| `enable_openapi` _boolean_ |  |  |  |
| `username` _string_ |  |  |  |
| `password` _string_ | Deprecated: stored unencrypted in etcd and readable by anyone who can<br />get this TradeBotConfig. Use secretRef instead (P3-1); this field is<br />rejected by the validating webhook unless<br />freqtrade.io/allow-plaintext-credentials is set, and will be removed<br />in v1beta1. |  |  |
| `jwtSecretKey` _string_ | Deprecated: see Password. |  |  |
| `secretRef` _string_ |  |  |  |
| `cors_origins` _string array_ |  |  |  |


#### AdvancedConfig







_Appears in:_
- [TradeBotConfigSpec](#tradebotconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `tradable_balance_ratio` _float_ | Advanced trading configuration |  |  |
| `cancel_open_orders_on_exit` _boolean_ |  |  |  |
| `margin_mode` _string_ |  |  |  |
| `initial_state` _string_ |  |  |  |
| `force_entry_enable` _boolean_ |  |  |  |


#### BotConfig







_Appears in:_
- [TradeBotConfigSpec](#tradebotconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `bot_name` _string_ | Bot configuration |  |  |
| `trading_mode` _string_ |  |  | Enum: [spot margin futures] <br /> |
| `dry_run` _boolean_ |  |  |  |
| `dry_run_wallet` _float_ |  |  | Minimum: 0 <br /> |
| `stake_currency` _string_ |  |  |  |
| `stake_amount` _string_ |  |  |  |
| `max_open_trades` _integer_ |  |  | Minimum: -1 <br /> |
| `fiat_display_currency` _string_ |  |  |  |
| `db_url` _string_ |  |  |  |
| `export` _string_ |  |  |  |
| `disable_param_export` _boolean_ |  |  |  |
| `disable_dataframe_checks` _boolean_ |  |  |  |


#### BotStatus



BotStatus is a snapshot of a bot's own freqtrade REST API responses
(P4-3) - polled, never pushed by the bot itself, so every field can be
stale by up to spec.introspection.interval.



_Appears in:_
- [TradeBotStatus](#tradebotstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `state` _string_ | State is freqtrade's own run state, from a successful poll's<br />show_config response: running or stopped. Stopped is a normal,<br />deliberate operational state (BotReachable stays True for it - see<br />pollWithClient) - not a failure, and not the same thing as<br />unreachable. unknown means the operator has never completed a poll<br />that reached show_config at all; check BotReachable for why. |  | Enum: [running stopped unknown] <br /> |
| `version` _string_ |  |  |  |
| `dryRun` _boolean_ |  |  |  |
| `openTrades` _integer_ |  |  |  |
| `maxOpenTrades` _integer_ |  |  |  |
| `totalProfitAbs` _string_ | TotalProfitAbs/TotalProfitPct are strings, not floats - API types<br />don't carry floats (see api/v1alpha1's own conventions elsewhere),<br />and a value straight from freqtrade's own JSON response is passed<br />through as text rather than round-tripped through float64. |  |  |
| `totalProfitPct` _string_ |  |  |  |
| `lastPollTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#time-v1-meta)_ | LastPollTime is when the poller last completed a poll attempt for<br />this bot, successful or not. |  |  |
| `lastPollError` _string_ | LastPollError is either the most recent poll failure's message (the<br />whole poll failed - BotReachable is False and the rest of this<br />struct is stale, unchanged from before this attempt), or a note<br />about which best-effort fields (OpenTrades/MaxOpenTrades,<br />TotalProfitAbs/TotalProfitPct, the freqtrade_bot_balance metric)<br />could not be refreshed on an otherwise-successful poll (BotReachable<br />is True and everything else here is fresh) - e.g. a reachable bot<br />that isn't currently running. Empty after a poll that refreshed<br />everything. |  |  |


#### DataCacheSpec







_Appears in:_
- [TradeBotSpec](#tradebotspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `pvcName` _string_ | PVCName is the name of a shared RWX PVC used as data cache for jobs.<br />If set, jobs will mount this PVC at /cache and use it as --datadir. |  |  |
| `downloadArgs` _string array_ | DownloadArgs are optional extra args for "freqtrade download-data".<br />Example: []string\{"--exchange","binance","-t","1m","5m","--days","30"\} |  |  |
| `downloadPolicy` _string_ | DownloadPolicy controls when the cache is refreshed by jobs. | always | Enum: [always ifMissing never] <br /> |


#### DataConfig







_Appears in:_
- [TradeBotConfigSpec](#tradebotconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `dataformat_ohlcv` _string_ |  |  |  |
| `dataformat_trades` _string_ |  |  |  |
| `position_adjustment` _string_ |  |  |  |
| `new_pairs_days_ago` _integer_ |  |  |  |
| `download_trades` _boolean_ |  |  |  |
| `max_entry_position_adjustment` _float_ |  |  |  |
| `available_capital` _float_ |  |  |  |
| `amend_last_stake_amount` _boolean_ |  |  |  |
| `last_stake_amount_min_ratio` _float_ |  |  |  |
| `process_only_new_candles` _boolean_ |  |  |  |
| `amount_reserve_percent` _float_ |  |  |  |
| `reduce_df_footprint` _boolean_ |  |  |  |
| `custom_price_max_distance_ratio` _float_ |  |  |  |


#### DataSplitParameters







_Appears in:_
- [AIConfig](#aiconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `test_size` _float_ |  |  |  |
| `random_state` _integer_ |  |  |  |
| `shuffle` _boolean_ |  |  |  |


#### ExchangeSpec







_Appears in:_
- [TradeBotConfigSpec](#tradebotconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ |  |  |  |
| `key` _string_ | Deprecated: stored unencrypted in etcd and readable by anyone who can<br />get this TradeBotConfig. Use secretRef instead (P3-1); this field is<br />rejected by the validating webhook unless<br />freqtrade.io/allow-plaintext-credentials is set, and will be removed<br />in v1beta1. |  |  |
| `secret` _string_ | Deprecated: see Key. |  |  |
| `password` _string_ | Deprecated: see Key. |  |  |
| `uid` _string_ | Deprecated: see Key. |  |  |
| `account_id` _string_ |  |  |  |
| `wallet_address` _string_ | Deprecated: see Key. |  |  |
| `private_key` _string_ | Deprecated: see Key. |  |  |
| `secretRef` _string_ |  |  |  |
| `ccxt_config` _[JSON](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#json-v1-apiextensions-k8s-io)_ |  |  |  |
| `ccxt_async_config` _[JSON](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#json-v1-apiextensions-k8s-io)_ |  |  |  |
| `ccxt_sync_config` _[JSON](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#json-v1-apiextensions-k8s-io)_ |  |  |  |
| `whitelist` _[PairListSpec](#pairlistspec)_ |  |  |  |
| `blacklist` _[PairListSpec](#pairlistspec)_ |  |  |  |
| `log_responses` _boolean_ |  |  |  |
| `enable_ws` _boolean_ |  |  |  |
| `unkown_fee_rate` _boolean_ |  |  |  |
| `outdated_offset` _integer_ |  |  |  |
| `market_refresh_interval` _integer_ |  |  |  |


#### ExperimentalConfig







_Appears in:_
- [TradeBotConfigSpec](#tradebotconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `block_bad_exchanges` _boolean_ |  |  |  |


#### FUAppConfig



FUAppConfig defines the configuration for the FreqUI application



_Appears in:_
- [FreqUISpec](#frequispec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `pod` _[FUPodSpec](#fupodspec)_ | PodSpec is the pod specification for FreqUI |  |  |
| `service` _[FUServiceSpec](#fuservicespec)_ | ServiceSpec is the service specification for FreqUI |  |  |
| `ingress` _[FUIngressSpec](#fuingressspec)_ | IngressSpec is the ingress specification for FreqUI |  |  |


#### FUIngressSpec



FUIngressSpec defines the ingress specification for FreqUI



_Appears in:_
- [FUAppConfig](#fuappconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `ingressClassName` _string_ | IngressClassName defines the ingress class name |  |  |
| `rules` _[IngressRule](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#ingressrule-v1-networking) array_ | Rules defines ingress rules |  |  |
| `tls` _[IngressTLS](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#ingresstls-v1-networking) array_ | TLS defines TLS configuration |  |  |
| `defaultBackend` _[IngressBackend](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#ingressbackend-v1-networking)_ | DefaultBackend defines the default backend |  |  |
| `annotations` _object (keys:string, values:string)_ | Annotations defines additional annotations for the ingress |  |  |


#### FUPodSpec



FUPodSpec defines the pod specification for FreqUI



_Appears in:_
- [FUAppConfig](#fuappconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `image` _string_ | Image is the container image for FreqUI |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#resourcerequirements-v1-core)_ | Resources defines the resource requirements for the pod |  |  |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#envvar-v1-core) array_ | Env defines environment variables for the pod |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#volumemount-v1-core) array_ | VolumeMounts defines volume mounts for the pod |  |  |
| `volumes` _[Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#volume-v1-core) array_ | Volumes defines volumes for the pod |  |  |
| `securityContext` _[PodSecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#podsecuritycontext-v1-core)_ | SecurityContext defines the security context for the pod |  |  |
| `initContainers` _[Container](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#container-v1-core) array_ | InitContainers defines init containers for the pod |  |  |
| `imagePullSecrets` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#localobjectreference-v1-core) array_ | ImagePullSecrets defines image pull secrets for the pod |  |  |
| `livenessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#probe-v1-core)_ | LivenessProbe defines the liveness probe for the pod |  |  |
| `readinessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#probe-v1-core)_ | ReadinessProbe defines the readiness probe for the pod |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#affinity-v1-core)_ | Affinity defines pod affinity rules |  |  |
| `antiAffinity` _[PodAntiAffinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#podantiaffinity-v1-core)_ | AntiAffinity defines pod anti-affinity rules |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | NodeSelector defines node selector for the pod |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#toleration-v1-core) array_ | Tolerations defines tolerations for the pod |  |  |
| `topologySpreadConstraints` _[TopologySpreadConstraint](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#topologyspreadconstraint-v1-core) array_ | TopologySpreadConstraints defines topology spread constraints |  |  |
| `replicas` _integer_ | Replicas defines the number of pod replicas |  |  |


#### FUServiceSpec



FUServiceSpec defines the service specification for FreqUI



_Appears in:_
- [FUAppConfig](#fuappconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[ServiceType](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#servicetype-v1-core)_ | Type defines the type of service (e.g., ClusterIP, NodePort, LoadBalancer) |  |  |
| `ports` _[ServicePort](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#serviceport-v1-core) array_ | Ports defines the ports for the service |  |  |
| `selector` _object (keys:string, values:string)_ | Selector defines the labels to select the pods for this service |  |  |
| `annotations` _object (keys:string, values:string)_ | Annotations defines additional annotations for the service |  |  |
| `loadBalancerSourceRanges` _string array_ | LoadBalancerSourceRanges defines source ranges for load balancer |  |  |
| `externalTrafficPolicy` _[ServiceExternalTrafficPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#serviceexternaltrafficpolicy-v1-core)_ | ExternalTrafficPolicy defines the external traffic policy |  |  |
| `sessionAffinity` _[ServiceAffinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#serviceaffinity-v1-core)_ | SessionAffinity defines session affinity |  |  |


#### FeatureParameters







_Appears in:_
- [AIConfig](#aiconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `include_corr_pairlist` _string array_ |  |  |  |
| `include_timeframes` _string array_ |  |  |  |
| `label_period_candles` _integer_ |  |  |  |
| `include_shifted_candles` _integer_ |  |  |  |
| `di_threshold` _float_ |  |  |  |
| `weight_factor` _float_ |  |  |  |
| `principal_component_analysis` _boolean_ |  |  |  |
| `indicator_periods_candles` _integer array_ |  |  |  |
| `use_svm_to_remove_outliers` _boolean_ |  |  |  |
| `plot_feature_importances` _integer_ |  |  |  |
| `svm_params` _[SVMParams](#svmparams)_ |  |  |  |
| `shuffle_after_split` _boolean_ |  |  |  |
| `buffer_train_data_candles` _integer_ |  |  |  |


#### Flow







_Appears in:_
- [OrderSpec](#orderspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `cache_size` _integer_ |  |  |  |
| `max_candles` _integer_ |  |  |  |
| `scale` _float_ |  |  |  |
| `stacked_imbalance_range` _integer_ |  |  |  |
| `imbalance_volume` _integer_ |  |  |  |
| `imbalance_ratio` _float_ |  |  |  |


#### FreqUI



FreqUI is the Schema for the frequis API



_Appears in:_
- [FreqUIList](#frequilist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `freqtrade.io/v1alpha1` | | |
| `kind` _string_ | `FreqUI` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[FreqUISpec](#frequispec)_ |  |  |  |
| `status` _[FreqUIStatus](#frequistatus)_ |  |  |  |


#### FreqUIList



FreqUIList contains a list of FreqUI





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `freqtrade.io/v1alpha1` | | |
| `kind` _string_ | `FreqUIList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[FreqUI](#frequi) array_ |  |  |  |


#### FreqUISpec



FreqUISpec defines the desired state of FreqUI



_Appears in:_
- [FreqUI](#frequi)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `host` _string_ | Host is the hostname for the FreqUI ingress |  |  |
| `tls` _[IngressTLS](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#ingresstls-v1-networking) array_ | TLS configuration for the FreqUI ingress |  |  |
| `ingressAnnotations` _object (keys:string, values:string)_ | IngressAnnotations are additional annotations for the FreqUI ingress |  |  |
| `app` _[FUAppConfig](#fuappconfig)_ | App is the configuration for the FreqUI application |  |  |
| `tradeBotRefs` _string array_ | TradeBotRefs is a list of TradeBot names that this FreqUI should manage,<br />resolved in this FreqUI's own namespace only (D3) - same-namespace-only<br />is a deliberate, documented constraint, not a TODO. A name that doesn't<br />resolve is surfaced via the TradeBotRefsResolved condition rather than<br />silently yielding no CORS entry for that bot. |  |  |


#### FreqUIStatus



FreqUIStatus defines the observed state of FreqUI



_Appears in:_
- [FreqUI](#frequi)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `phase` _string_ | Phase is a derived, human-facing summary for the printer column only -<br />it is never the source of truth. Conditions are. |  |  |
| `message` _string_ | Message is a human-readable message indicating details about the current phase |  |  |
| `url` _string_ | URL is the URL where FreqUI is accessible |  |  |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#condition-v1-meta) array_ |  |  |  |
| `observedGeneration` _integer_ | ObservedGeneration is the most recent metadata.generation this status<br />was computed from. |  |  |


#### InternalsConfig



InternalsConfig defines internal processing configuration



_Appears in:_
- [TradeBotConfigSpec](#tradebotconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `process_throttle_secs` _integer_ |  |  |  |
| `interval` _integer_ |  |  |  |
| `sd_notify` _boolean_ |  |  |  |


#### IntrospectionSpec



IntrospectionSpec controls whether and how often the operator polls this
bot's own freqtrade REST API (P4-3). Polling only works at all when
spec.app... has an api_server enabled with Basic Auth credentials the
operator can read back from apiServer.secretRef - see BotStatus.State
"unknown" and the BotReachable condition for what happens otherwise.



_Appears in:_
- [TradeBotSpec](#tradebotspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled turns polling on or off for this bot. Defaults to on: a<br />TradeBot with no api_server configured at all just polls, fails to<br />reach anything, and reports BotReachable=False - turn this off<br />explicitly to silence that for a bot that deliberately has no API<br />server (e.g. backtesting/hyperopt Jobs, which never expose one). | true |  |
| `interval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#duration-v1-meta)_ | Interval between polls. The admission webhook rejects anything under<br />10s - freqtrade's REST API is not built for tight polling loops, and<br />this operator is not a market-data source. | 60s |  |


#### LoggingConfig







_Appears in:_
- [TradeBotConfigSpec](#tradebotconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `version` _integer_ |  |  |  |


#### ModelRewardParameters







_Appears in:_
- [RLConfig](#rlconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `rr` _float_ |  |  |  |
| `profit_aim` _float_ |  |  |  |


#### ModelTrainingParameters







_Appears in:_
- [AIConfig](#aiconfig)



#### NotificationDiscord







_Appears in:_
- [NotificationSpec](#notificationspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ |  |  |  |
| `webhook_url` _string_ |  |  |  |
| `exit_fill` _object array_ |  |  |  |
| `entry_fill` _object array_ |  |  |  |


#### NotificationSpec







_Appears in:_
- [TradeBotConfigSpec](#tradebotconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `telegram` _[NotificationTelegram](#notificationtelegram)_ |  |  |  |
| `webhook` _[NotificationWebhook](#notificationwebhook)_ |  |  |  |
| `discord` _[NotificationDiscord](#notificationdiscord)_ |  |  |  |


#### NotificationTelegram







_Appears in:_
- [NotificationSpec](#notificationspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ |  |  |  |
| `token` _string_ | Deprecated: stored unencrypted in etcd and readable by anyone who can<br />get this TradeBotConfig. Use secretRef instead (P3-1); this field is<br />rejected by the validating webhook unless<br />freqtrade.io/allow-plaintext-credentials is set, and will be removed<br />in v1beta1. |  |  |
| `secretRef` _string_ |  |  |  |
| `balance_dust_level` _float_ |  |  |  |
| `reload` _boolean_ |  |  |  |
| `allow_custom_messages` _boolean_ |  |  |  |
| `chat_id` _string_ |  |  |  |
| `topic_id` _string_ |  |  |  |
| `authorized_users` _string array_ |  |  |  |
| `settings` _[NotificationTelegramSettings](#notificationtelegramsettings)_ |  |  |  |


#### NotificationTelegramSettings







_Appears in:_
- [NotificationTelegram](#notificationtelegram)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `status` _string_ |  |  |  |
| `warning` _string_ |  |  |  |
| `startup` _string_ |  |  |  |
| `entry` _string_ |  |  |  |
| `entry_fill` _string_ |  |  |  |
| `entry_cancel` _string_ |  |  |  |
| `exit` _string_ |  |  |  |
| `exit_fill` _string_ |  |  |  |
| `exit_cancel` _string_ |  |  |  |
| `protection_trigger` _string_ |  |  |  |
| `protection_trigger_global` _string_ |  |  |  |


#### NotificationWebhook







_Appears in:_
- [NotificationSpec](#notificationspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ |  |  |  |
| `url` _string_ |  |  |  |
| `entry` _string_ |  |  |  |
| `entry_cancel` _string_ |  |  |  |
| `entry_fill` _string_ |  |  |  |
| `exit` _string_ |  |  |  |
| `exit_cancel` _string_ |  |  |  |
| `exit_fill` _string_ |  |  |  |
| `status` _string_ |  |  |  |
| `allow_custom_messages` _boolean_ | Deprecated: not a real Freqtrade webhook option - allow_custom_messages<br />only exists under `telegram` (see NotificationTelegram.AllowCustomMessages).<br />configbuilder no longer renders this field into config.json; it has no<br />effect regardless of its value. |  |  |


#### OrderSpec







_Appears in:_
- [TradeBotConfigSpec](#tradebotconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `types` _[Types](#types)_ | Types contains configuration for different order types |  |  |
| `time_in_force` _[TimeInForce](#timeinforce)_ | TimeInForce contains configuration for order time in force options |  |  |
| `flow` _[Flow](#flow)_ | Flow contains configuration for order flow processing |  |  |


#### PVCSpec



PVCSpec defines the Persistent Volume Claim specification



_Appears in:_
- [TBAppConfig](#tbappconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `accessModes` _[PersistentVolumeAccessMode](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#persistentvolumeaccessmode-v1-core) array_ | AccessModes defines the access modes for the PVC (e.g., ReadWriteOnce, ReadOnlyMany) |  |  |
| `storageSize` _[Quantity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#quantity-resource-api)_ | StorageSize defines the size of the storage for the PVC. A quantity<br />value (e.g. "10Gi") is validated by the API server at admission time;<br />as a plain string this used to reach resource.MustParse in the<br />reconciler and panic the manager on anything malformed. |  |  |
| `storageClassName` _string_ | StorageClassName defines the storage class for the PVC |  |  |
| `volumeName` _string_ | VolumeName defines the name of the volume to bind to |  |  |
| `annotations` _object (keys:string, values:string)_ | Annotations defines additional annotations for the PVC |  |  |
| `labels` _object (keys:string, values:string)_ | Labels defines additional labels for the PVC |  |  |
| `fixVolumePermissions` _boolean_ | FixVolumePermissions restores the pre-P3-3 behavior of running the<br />init-user-data init container as root to chmod/chown this PVC before<br />the main container starts. Off by default: spec.securityContext.fsGroup<br />(already set on every pod this operator builds) already makes the<br />volume group-writable on most CSI drivers, and running as root here is<br />a real, if narrow, privilege escalation on an otherwise fully<br />non-root pod. Turn this on only if pods are actually crash-looping on<br />a permission-denied error under /freqtrade/user_data and changing<br />storage class isn't an option. |  |  |


#### PairListSpec



PairListSpec defines the desired state of PairList



_Appears in:_
- [ExchangeSpec](#exchangespec)
- [TradeBotConfigSpec](#tradebotconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `pairs` _string array_ |  |  |  |


#### PairlistConfig



PairlistConfig represents the configuration for a pairlist method



_Appears in:_
- [PairlistMethodsSpec](#pairlistmethodsspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `method` _[PairlistMethod](#pairlistmethod)_ | Method is the pairlist method to use |  |  |
| `number_assets` _integer_ | Common configuration options |  |  |
| `refresh_period` _integer_ |  |  |  |
| `allow_inactive` _boolean_ | StaticPairList specific options |  |  |
| `sort_key` _string_ | VolumePairList specific options |  |  |
| `lookback_timeframe` _string_ | PercentChangePairList specific options |  |  |
| `lookback_period` _integer_ |  |  |  |
| `lookback_days` _integer_ |  |  |  |
| `min_change_rate` _float_ |  |  |  |
| `producer_name` _string_ | ProducerPairList specific options |  |  |
| `mode` _string_ | RemotePairList specific options |  |  |
| `processing_mode` _string_ |  |  |  |
| `pairlist_url` _string_ |  |  |  |
| `keep_pairlist_on_failure` _boolean_ |  |  |  |
| `read_timeout` _integer_ |  |  |  |
| `bearer_token` _string_ |  |  |  |
| `save_to_file` _string_ |  |  |  |
| `max_rank` _integer_ | MarketCapPairList specific options |  |  |
| `categories` _string array_ |  |  |  |
| `min_days_listed` _integer_ | AgeFilter specific options |  |  |
| `max_days_listed` _integer_ |  |  |  |
| `min_price` _float_ | PriceFilter specific options |  |  |
| `max_price` _float_ |  |  |  |
| `max_spread_ratio` _float_ | SpreadFilter specific options |  |  |
| `lookback_days_range` _integer_ | RangeStabilityFilter specific options |  |  |
| `min_rate_of_change` _float_ |  |  |  |
| `max_rate_of_change` _float_ |  |  |  |
| `lookback_days_volatility` _integer_ | VolatilityFilter specific options |  |  |
| `min_volatility` _float_ |  |  |  |
| `max_volatility` _float_ |  |  |  |
| `offset` _integer_ | OffsetFilter specific options |  |  |


#### PairlistMethod

_Underlying type:_ _string_

PairlistMethod represents the method used for pair selection



_Appears in:_
- [PairlistConfig](#pairlistconfig)

| Field | Description |
| --- | --- |
| `StaticPairList` | StaticPairList uses a statically defined pair whitelist from the configuration<br /> |
| `VolumePairList` | VolumePairList employs sorting/filtering of pairs by their trading volume<br /> |
| `PercentChangePairList` | PercentChangePairList selects pairs based on percent change<br /> |
| `ProducerPairList` | ProducerPairList reuses the pairlist from a Producer<br /> |
| `RemotePairList` | RemotePairList fetches a pairlist from a remote server or a locally stored json file<br /> |
| `MarketCapPairList` | MarketCapPairList selects pairs based on market capitalization<br /> |
| `AgeFilter` | AgeFilter removes pairs that have been listed on the exchange for less than min_days_listed<br /> |
| `FullTradesFilter` | FullTradesFilter shrinks whitelist to consist only in-trade pairs when the trade slots are full<br /> |
| `OffsetFilter` | OffsetFilter applies an offset to the pairlist<br /> |
| `PerformanceFilter` | PerformanceFilter filters pairs by their performance<br /> |
| `PrecisionFilter` | PrecisionFilter filters pairs by their precision<br /> |
| `PriceFilter` | PriceFilter filters pairs by their price<br /> |
| `ShuffleFilter` | ShuffleFilter shuffles the pairlist<br /> |
| `SpreadFilter` | SpreadFilter filters pairs by their spread<br /> |
| `RangeStabilityFilter` | RangeStabilityFilter filters pairs by their range stability<br /> |
| `VolatilityFilter` | VolatilityFilter filters pairs by their volatility<br /> |


#### PairlistMethodsSpec



PairlistMethodsSpec defines the desired state of PairlistMethods



_Appears in:_
- [TradeBotConfigSpec](#tradebotconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `methods` _[PairlistConfig](#pairlistconfig) array_ | Methods is an array of pairlist configurations |  |  |


#### PodSpec







_Appears in:_
- [TBAppConfig](#tbappconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `image` _string_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#resourcerequirements-v1-core)_ |  |  |  |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#envvar-v1-core) array_ |  |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#volumemount-v1-core) array_ |  |  |  |
| `volumes` _[Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#volume-v1-core) array_ |  |  |  |
| `securityContext` _[PodSecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#podsecuritycontext-v1-core)_ | SecurityContext defines the security context for the pod |  |  |
| `initContainers` _[Container](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#container-v1-core) array_ | InitContainers defines the init containers for the pod |  |  |
| `serviceName` _string_ |  |  |  |
| `imagePullSecrets` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#localobjectreference-v1-core) array_ | ImagePullSecrets defines the image pull secrets for the pod |  |  |
| `livenessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#probe-v1-core)_ |  |  |  |
| `readinessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#probe-v1-core)_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#affinity-v1-core)_ | Scheduling parameters |  |  |
| `antiAffinity` _[PodAntiAffinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#podantiaffinity-v1-core)_ |  |  |  |
| `nodeSelector` _object (keys:string, values:string)_ |  |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#toleration-v1-core) array_ |  |  |  |
| `topologySpreadConstraints` _[TopologySpreadConstraint](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#topologyspreadconstraint-v1-core) array_ |  |  |  |


#### PricingCheckDepthOfMarket



PricingCheckDepthOfMarket defines the check_depth_of_market settings



_Appears in:_
- [PricingSpec](#pricingspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ |  |  |  |
| `bids_to_ask_delta` _float_ |  |  |  |


#### PricingSpec



PricingSpec defines the desired state of Pricing



_Appears in:_
- [TradeBotConfigSpec](#tradebotconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `price_side` _string_ |  |  |  |
| `price_last_balance` _float_ |  |  |  |
| `use_order_book` _boolean_ |  |  |  |
| `order_book_top` _integer_ |  |  |  |
| `check_depth_of_market` _[PricingCheckDepthOfMarket](#pricingcheckdepthofmarket)_ |  |  |  |


#### RLConfig







_Appears in:_
- [AIConfig](#aiconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `drop_ohlc_from_features` _boolean_ |  |  |  |
| `train_cycles` _integer_ |  |  |  |
| `max_trade_duration_candles` _integer_ |  |  |  |
| `add_state_info` _boolean_ |  |  |  |
| `max_training_drawdown_pct` _float_ |  |  |  |
| `cpu_count` _integer_ |  |  |  |
| `model_type` _string_ |  |  |  |
| `policy_type` _string_ |  |  |  |
| `net_arch` _integer array_ |  |  |  |
| `randomize_starting_position` _boolean_ |  |  |  |
| `progress_bar` _boolean_ |  |  |  |
| `model_reward_parameters` _[ModelRewardParameters](#modelrewardparameters)_ |  |  |  |


#### RiskManagementSpec



RiskManagementSpec defines the desired state of RiskManagement



_Appears in:_
- [TradeBotConfigSpec](#tradebotconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `minimal_roi` _object (keys:string, values:float)_ |  |  |  |
| `stoploss` _float_ |  |  |  |
| `trailing_stop` _boolean_ |  |  |  |
| `trailing_stop_positive` _float_ |  |  |  |
| `trailing_stop_positive_offset` _float_ |  |  |  |
| `trailing_only_offset_is_reached` _boolean_ |  |  |  |
| `use_exit_signal` _boolean_ |  |  |  |
| `exit_profit_only` _boolean_ |  |  |  |
| `exit_profit_offset` _float_ |  |  |  |
| `fee` _float_ |  |  |  |
| `ignore_roi_if_entry_signal` _boolean_ |  |  |  |
| `ignore_buying_expired_candle_after` _integer_ |  |  |  |
| `minimum_trade_amount` _integer_ |  |  |  |
| `targeted_trade_amount` _integer_ |  |  |  |
| `lookahead_analysis_export_filename` _string_ |  |  |  |
| `startup_candle` _integer_ |  |  |  |
| `liquidation_buffer` _float_ |  |  |  |
| `backtest_breakdown` _string array_ |  |  |  |


#### SVMParams







_Appears in:_
- [FeatureParameters](#featureparameters)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `shuffle` _boolean_ |  |  |  |
| `nu` _float_ |  |  |  |


#### ServiceSpec







_Appears in:_
- [TBAppConfig](#tbappconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[ServiceType](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#servicetype-v1-core)_ | Type defines the type of service (e.g., ClusterIP, NodePort, LoadBalancer) |  |  |
| `ports` _[ServicePort](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#serviceport-v1-core) array_ | Ports defines the ports for the service |  |  |
| `selector` _object (keys:string, values:string)_ | Selector defines the labels to select the pods for this service |  |  |
| `annotations` _object (keys:string, values:string)_ | Annotations defines additional annotations for the service |  |  |


#### Strategy







_Appears in:_
- [StrategyList](#strategylist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `freqtrade.io/v1alpha1` | | |
| `kind` _string_ | `Strategy` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[StrategySpec](#strategyspec)_ |  |  |  |
| `status` _[StrategyStatus](#strategystatus)_ |  |  |  |




#### StrategyList









| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `freqtrade.io/v1alpha1` | | |
| `kind` _string_ | `StrategyList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[Strategy](#strategy) array_ |  |  |  |


#### StrategySpec



StrategySpec defines the desired state of Strategy



_Appears in:_
- [Strategy](#strategy)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the strategy (e.g., "SampleStrategy") |  |  |
| `script` _string_ | Script content or reference to a ConfigMap/Secret |  |  |


#### StrategyStatus



StrategyStatus defines the observed state of Strategy



_Appears in:_
- [Strategy](#strategy)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `phase` _string_ | Phase is a derived, human-facing summary for the printer column only -<br />it is never the source of truth. Conditions are. |  |  |
| `message` _string_ |  |  |  |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#condition-v1-meta) array_ |  |  |  |
| `observedGeneration` _integer_ | ObservedGeneration is the most recent metadata.generation this status<br />was computed from. |  |  |


#### TBAppConfig







_Appears in:_
- [TradeBotSpec](#tradebotspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `pod` _[PodSpec](#podspec)_ |  |  |  |
| `service` _[ServiceSpec](#servicespec)_ |  |  |  |
| `pvc` _[PVCSpec](#pvcspec)_ |  |  |  |


#### TimeInForce



TimeInForce defines options for order time in force



_Appears in:_
- [OrderSpec](#orderspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `entry` _string_ |  |  |  |
| `exit` _string_ |  |  |  |


#### TradeBot







_Appears in:_
- [TradeBotList](#tradebotlist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `freqtrade.io/v1alpha1` | | |
| `kind` _string_ | `TradeBot` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[TradeBotSpec](#tradebotspec)_ |  |  |  |
| `status` _[TradeBotStatus](#tradebotstatus)_ |  |  |  |


#### TradeBotConfig







_Appears in:_
- [TradeBotConfigList](#tradebotconfiglist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `freqtrade.io/v1alpha1` | | |
| `kind` _string_ | `TradeBotConfig` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[TradeBotConfigSpec](#tradebotconfigspec)_ |  |  |  |
| `status` _[TradeBotConfigStatus](#tradebotconfigstatus)_ |  |  |  |




#### TradeBotConfigList









| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `freqtrade.io/v1alpha1` | | |
| `kind` _string_ | `TradeBotConfigList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[TradeBotConfig](#tradebotconfig) array_ |  |  |  |


#### TradeBotConfigSpec



TradeBotConfigSpec defines the desired state of TradeBotConfig



_Appears in:_
- [TradeBotConfig](#tradebotconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `bot` _[BotConfig](#botconfig)_ |  |  |  |
| `ai` _[AIConfig](#aiconfig)_ |  |  |  |
| `data` _[DataConfig](#dataconfig)_ |  |  |  |
| `advanced` _[AdvancedConfig](#advancedconfig)_ |  |  |  |
| `timeout` _[UnfilledTimeoutConfig](#unfilledtimeoutconfig)_ |  |  |  |
| `internals` _[InternalsConfig](#internalsconfig)_ |  |  |  |
| `exchange` _[ExchangeSpec](#exchangespec)_ |  |  |  |
| `pairlist` _[PairListSpec](#pairlistspec)_ |  |  |  |
| `pairlist_method` _[PairlistMethodsSpec](#pairlistmethodsspec)_ |  |  |  |
| `entry_pricing` _[PricingSpec](#pricingspec)_ |  |  |  |
| `exit_pricing` _[PricingSpec](#pricingspec)_ |  |  |  |
| `order` _[OrderSpec](#orderspec)_ |  |  |  |
| `risk_management` _[RiskManagementSpec](#riskmanagementspec)_ |  |  |  |
| `notification` _[NotificationSpec](#notificationspec)_ |  |  |  |
| `apiServer` _[APIServerConfig](#apiserverconfig)_ |  |  |  |
| `experimental` _[ExperimentalConfig](#experimentalconfig)_ |  |  |  |
| `logging` _[LoggingConfig](#loggingconfig)_ |  |  |  |


#### TradeBotConfigStatus



TradeBotConfigStatus defines the observed state of TradeBotConfig



_Appears in:_
- [TradeBotConfig](#tradebotconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `phase` _string_ | Phase is a derived, human-facing summary for the printer column only -<br />it is never the source of truth. Conditions are. |  |  |
| `message` _string_ |  |  |  |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#condition-v1-meta) array_ |  |  |  |
| `observedGeneration` _integer_ | ObservedGeneration is the most recent metadata.generation this status<br />was computed from. |  |  |




#### TradeBotList









| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `freqtrade.io/v1alpha1` | | |
| `kind` _string_ | `TradeBotList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[TradeBot](#tradebot) array_ |  |  |  |


#### TradeBotSpec



TradeBotSpec defines the desired state of TradeBot



_Appears in:_
- [TradeBot](#tradebot)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `freqtrade_command` _string_ | Default is "trade". Can be "backtesting" or "hyperopt" | trade | Enum: [trade backtesting hyperopt download-data lookahead-analysis] <br /> |
| `freqtrade_arguments` _string array_ |  |  | MaxItems: 64 <br /> |
| `config` _string_ | Reference to the TradeBotConfig resource, same namespace only (D3) |  | MinLength: 1 <br />Pattern: `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$` <br /> |
| `strategy` _string_ | Reference to the Strategy resource, same namespace only (D3) |  | MinLength: 1 <br />Pattern: `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$` <br /> |
| `app` _[TBAppConfig](#tbappconfig)_ |  |  |  |
| `data` _[DataCacheSpec](#datacachespec)_ | Deprecated: no effect since v1beta1 - TradeBot is trade-only there and<br />has no Job-mode pod to cache data for; use a Backtest instead. Absent<br />from v1beta1 entirely; kept here only because it's a served v1alpha1<br />field. |  |  |
| `updateStrategy` _string_ | UpdateStrategy controls what happens when a config change can't take<br />effect without a restart (config.json is mounted from a Secret and<br />read once at freqtrade startup, so rewriting the Secret alone doesn't<br />change what a running bot is doing). "Manual" (the default, per D2)<br />leaves the running StatefulSet pod template untouched and reports the<br />pending restart via the ConfigDrift condition instead - a bot may be<br />holding open positions, so an unrequested restart is not this<br />controller's call to make. "Auto" writes the new config hash onto the<br />pod template, letting Kubernetes' own StatefulSet rolling update<br />carry out the restart. Only meaningful for freqtrade_command: trade;<br />a Job's pod template is already immutable after creation regardless<br />(see the WorkloadImmutable condition). | Manual | Enum: [Manual Auto] <br /> |
| `introspection` _[IntrospectionSpec](#introspectionspec)_ | Introspection controls the operator's own polling of this bot's<br />freqtrade REST API for live trading state (P4-3, implements D4).<br />Read-only: nothing here can start, stop, or otherwise act on the bot<br />(see P4-4 for that, a deliberately separate task). |  |  |


#### TradeBotStatus



TradeBotStatus defines the observed state of TradeBot



_Appears in:_
- [TradeBot](#tradebot)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `phase` _string_ | Phase is a derived, human-facing summary for the printer column only -<br />it is never the source of truth. Conditions are. |  |  |
| `message` _string_ |  |  |  |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#condition-v1-meta) array_ |  |  |  |
| `observedGeneration` _integer_ | ObservedGeneration is the most recent metadata.generation this status<br />was computed from, so a client can tell whether it reflects the spec<br />it just applied. |  |  |
| `appliedConfigHash` _string_ | AppliedConfigHash is sha256(config.json bytes + strategy script<br />bytes), truncated to 16 hex characters: the hash of the config<br />currently rendered into this TradeBot's Secret, regardless of<br />whether it has been rolled out to the running StatefulSet pods yet<br />(see the ConfigDrift condition). |  |  |
| `resolvedImage` _string_ | ResolvedImage is the exact freqtrade image reference (normally<br />digest-pinned) the running workload was built with - the manager's<br />--default-freqtrade-image flag unless spec.app.pod.image overrides it<br />(P3-3). A floating tag would let a routine pod restart silently pick<br />up a new freqtrade version mid-trading; this makes what's actually<br />running visible regardless of which source set it. |  |  |
| `bot` _[BotStatus](#botstatus)_ | Bot is this bot's own live trading state, as last observed by the<br />operator's poller (P4-3) - never written by the TradeBot reconciler<br />itself, and can lag behind spec.introspection.interval seconds.<br />LastPollTime/LastPollError say how current (or not) the rest of it<br />is; the BotReachable condition is the authoritative "can we trust<br />this at all right now" signal. |  |  |


#### Types



Types defines various order type configurations



_Appears in:_
- [OrderSpec](#orderspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `entry` _string_ |  |  |  |
| `exit` _string_ |  |  |  |
| `emergency_exit` _string_ |  |  |  |
| `force_entry` _string_ |  |  |  |
| `force_exit` _string_ |  |  |  |
| `stoploss` _string_ |  |  |  |
| `stoploss_on_exchange` _boolean_ |  |  |  |
| `stoploss_on_exchange_interval` _integer_ |  |  |  |
| `stoploss_on_exchange_limit_ratio` _float_ |  |  |  |


#### UnfilledTimeoutConfig



UnfilledTimeoutConfig defines timeout settings for unfilled orders



_Appears in:_
- [TradeBotConfigSpec](#tradebotconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `entry` _integer_ |  |  |  |
| `exit` _integer_ |  |  |  |
| `exit_timeout_count` _integer_ |  |  |  |
| `unit` _string_ |  |  |  |



## freqtrade.io/v1beta1

Package v1beta1 contains API Schema definitions for the freqtrade v1beta1 API group

### Resource Types
- [Backtest](#backtest)
- [BacktestList](#backtestlist)
- [TradeBot](#tradebot)
- [TradeBotConfig](#tradebotconfig)
- [TradeBotConfigList](#tradebotconfiglist)
- [TradeBotList](#tradebotlist)



#### AIConfig







_Appears in:_
- [TradeBotConfigSpec](#tradebotconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ |  |  |  |
| `identifier` _string_ |  |  |  |
| `write_metrics_to_disk` _boolean_ |  |  |  |
| `purge_old_models` _integer_ |  |  |  |
| `conv_width` _integer_ |  |  |  |
| `train_period_days` _integer_ |  |  |  |
| `backtest_period_days` _integer_ |  |  |  |
| `live_retrain_hours` _integer_ |  |  |  |
| `expiration_hours` _integer_ |  |  |  |
| `save_backtest_models` _boolean_ |  |  |  |
| `fit_live_predictions_candles` _integer_ |  |  |  |
| `data_kitchen_thread_count` _integer_ |  |  |  |
| `activate_tensorboard` _boolean_ |  |  |  |
| `wait_for_training_iteration_on_reload` _boolean_ |  |  |  |
| `continue_learning` _boolean_ |  |  |  |
| `keras` _boolean_ |  |  |  |
| `feature_parameters` _[FeatureParameters](#featureparameters)_ |  |  |  |
| `data_split_parameters` _[DataSplitParameters](#datasplitparameters)_ |  |  |  |
| `model_training_parameters` _[ModelTrainingParameters](#modeltrainingparameters)_ |  |  |  |
| `rl_config` _[RLConfig](#rlconfig)_ |  |  |  |


#### APIServerConfig



APIServerConfig: Password/JWTSecretKey are gone - see ExchangeSpec's own
doc comment, same B2 reasoning. SecretRef is the only path.



_Appears in:_
- [TradeBotConfigSpec](#tradebotconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ |  |  |  |
| `listen_ip_address` _string_ |  |  |  |
| `listen_port` _integer_ |  |  | Maximum: 65535 <br />Minimum: 1 <br /> |
| `verbosity` _string_ |  |  |  |
| `enable_openapi` _boolean_ |  |  |  |
| `username` _string_ |  |  |  |
| `secretRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#localobjectreference-v1-core)_ | SecretRef references a Secret in this TradeBotConfig's own namespace<br />only (D3). |  |  |
| `cors_origins` _string array_ |  |  |  |


#### AdvancedConfig







_Appears in:_
- [TradeBotConfigSpec](#tradebotconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `tradable_balance_ratio` _float_ |  |  |  |
| `cancel_open_orders_on_exit` _boolean_ |  |  |  |
| `margin_mode` _string_ |  |  |  |
| `initial_state` _string_ |  |  |  |
| `force_entry_enable` _boolean_ |  |  |  |


#### Backtest







_Appears in:_
- [BacktestList](#backtestlist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `freqtrade.io/v1beta1` | | |
| `kind` _string_ | `Backtest` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[BacktestSpec](#backtestspec)_ |  |  |  |
| `status` _[BacktestStatus](#backteststatus)_ |  |  |  |




#### BacktestList









| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `freqtrade.io/v1beta1` | | |
| `kind` _string_ | `BacktestList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[Backtest](#backtest) array_ |  |  |  |


#### BacktestResults



BacktestResults summarizes one run's outcome. All numeric results are
strings, never float64 - no stable JSON round-trip, and every other
numeric field across this API follows the same rule (see e.g.
v1alpha1.BotStatus.TotalProfitAbs).



_Appears in:_
- [BacktestStatus](#backteststatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `totalTrades` _integer_ |  |  |  |
| `profitAbs` _string_ |  |  |  |
| `profitPct` _string_ |  |  |  |
| `winRatePct` _string_ |  |  |  |
| `maxDrawdownPct` _string_ |  |  |  |
| `sharpeRatio` _string_ |  |  |  |
| `sortinoRatio` _string_ |  |  |  |
| `cagrPct` _string_ |  |  |  |
| `bestPair` _string_ |  |  |  |
| `worstPair` _string_ |  |  |  |
| `resultFile` _string_ | ResultFile is the path on the results PVC the full result JSON was<br />read from. |  |  |


#### BacktestSpec



BacktestSpec defines the desired state of Backtest. Immutable after
creation in full (CEL self == oldSelf, below) - a Backtest is an
immutable fact about a (strategy, config, timerange, data) tuple, not a
thing you edit in place. To change any parameter, create a new Backtest.



_Appears in:_
- [Backtest](#backtest)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `configRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#localobjectreference-v1-core)_ | ConfigRef is the TradeBotConfig this run renders config.json from,<br />same namespace only (D3). |  |  |
| `strategyRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#localobjectreference-v1-core)_ | StrategyRef is the Strategy this run executes, same namespace only (D3). |  |  |
| `timerange` _string_ | Timerange is freqtrade's --timerange value, e.g. "20230101-20230201"<br />or "20230101-" for open-ended. |  | Pattern: `^\d\{8\}-(\d\{8\})?$` <br /> |
| `timeframe` _string_ | Timeframe is freqtrade's --timeframe value, e.g. "5m", "1h", "1d". |  | Pattern: `^\d+[mhdw]$` <br /> |
| `pairs` _string array_ |  |  | MaxItems: 256 <br /> |
| `data` _[DataSourceSpec](#datasourcespec)_ | Data optionally mounts a shared read-only cache PVC as --datadir,<br />with an init container downloading into it first per DownloadPolicy -<br />moved here from TradeBot.Spec.Data (v1alpha1), which only ever made<br />sense for a one-shot run in the first place (see pod.go's isTrade gate<br />on it in the old package). |  |  |
| `results` _[ResultsSpec](#resultsspec)_ | Results configures the per-run PVC this run's output is written to<br />(D1) - the CR is the run's identity, so unlike v1alpha1's Job mode<br />there is no orphaned-volume or spec-hash-in-the-name problem to work<br />around. |  |  |
| `pod` _[PodSpec](#podspec)_ | Pod carries resource requests/limits and scheduling overrides for the<br />run's pod. |  |  |
| `ttlSecondsAfterFinished` _integer_ | TTLSecondsAfterFinished reaps the finished Job (not the results PVC -<br />see ResultsSpec.RetentionPolicy, and D10 on why nothing prunes results<br />automatically) this many seconds after it completes. | 86400 |  |
| `extraArgs` _string array_ | ExtraArgs appends raw freqtrade CLI arguments (D8/D5): typed fields<br />above remain the documented surface for anything they cover. This is<br />a pressure valve for flags this API hasn't caught up to yet, not an<br />alternative to them - guarded by the freqtrade.io/allow-extra-args:<br />"true" annotation (rejected by the admission webhook without it) and<br />checked against a denylist of flags the operator itself controls<br />(--config, --strategy, --strategy-path, --db-url, --logfile,<br />--userdir, --datadir); the controller emits a warning Event whenever<br />it's used so its usage stays visible rather than quietly load-bearing. |  | MaxItems: 64 <br /> |
| `timeframeDetail` _string_ | TimeframeDetail is freqtrade's --timeframe-detail value, for<br />sub-timeframe order execution detail. |  |  |
| `maxOpenTrades` _integer_ | MaxOpenTrades overrides the rendered config's max_open_trades for this run. |  |  |
| `stakeAmount` _string_ | StakeAmount is freqtrade's --stake-amount value: "unlimited", or a<br />quantity in the exchange's stake currency. |  |  |
| `dryRunWallet` _[Quantity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#quantity-resource-api)_ | DryRunWallet overrides the rendered config's dry_run_wallet starting balance. |  |  |
| `fee` _string_ | Fee overrides the rendered config's trading fee (a fraction, e.g. "0.001").<br />A pointer so an explicit "0" is distinguishable from unset. |  |  |
| `enableProtections` _boolean_ | EnableProtections turns on freqtrade's --enable-protections for this run. |  |  |
| `breakdown` _string array_ | Breakdown requests freqtrade's --breakdown output at these granularities. |  | Enum: [day week month] <br /> |
| `cache` _string_ | Cache is freqtrade's --cache value, controlling its own internal<br />signal-calculation caching across repeated backtests - unrelated to<br />RunSpec.Data, the operator's own shared data-download PVC feature. |  | Enum: [none day week month] <br /> |


#### BacktestStatus



BacktestStatus defines the observed state of Backtest.



_Appears in:_
- [Backtest](#backtest)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `phase` _string_ | Phase is a derived, human-facing summary for the printer column only -<br />it is never the source of truth. Conditions are. |  |  |
| `message` _string_ |  |  |  |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#condition-v1-meta) array_ |  |  |  |
| `observedGeneration` _integer_ | ObservedGeneration is the most recent metadata.generation this status<br />was computed from. |  |  |
| `jobName` _string_ |  |  |  |
| `resultsPVCName` _string_ |  |  |  |
| `startTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#time-v1-meta)_ |  |  |  |
| `completionTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#time-v1-meta)_ |  |  |  |
| `results` _[BacktestResults](#backtestresults)_ | Results is filled in once the run's sidecar has extracted them (P6-2)<br />- nil until then, and left as the last successful run's data if a<br />later parse ever fails (see ConditionResultsAvailable). |  |  |


#### BotConfig







_Appears in:_
- [TradeBotConfigSpec](#tradebotconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `bot_name` _string_ |  |  |  |
| `trading_mode` _string_ |  |  | Enum: [spot margin futures] <br /> |
| `dry_run` _boolean_ |  |  |  |
| `dry_run_wallet` _float_ |  |  | Minimum: 0 <br /> |
| `stake_currency` _string_ |  |  |  |
| `stake_amount` _string_ |  |  |  |
| `max_open_trades` _integer_ |  |  | Minimum: -1 <br /> |
| `fiat_display_currency` _string_ |  |  |  |
| `db_url` _string_ |  |  |  |
| `export` _string_ |  |  |  |
| `disable_param_export` _boolean_ |  |  |  |
| `disable_dataframe_checks` _boolean_ |  |  |  |


#### BotStatus



BotStatus is a snapshot of a bot's own freqtrade REST API responses
(P4-3) - identical to v1alpha1's.



_Appears in:_
- [TradeBotStatus](#tradebotstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `state` _string_ |  |  | Enum: [running stopped unknown] <br /> |
| `version` _string_ |  |  |  |
| `dryRun` _boolean_ |  |  |  |
| `openTrades` _integer_ |  |  |  |
| `maxOpenTrades` _integer_ |  |  |  |
| `totalProfitAbs` _string_ |  |  |  |
| `totalProfitPct` _string_ |  |  |  |
| `lastPollTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#time-v1-meta)_ |  |  |  |
| `lastPollError` _string_ |  |  |  |


#### DataConfig







_Appears in:_
- [TradeBotConfigSpec](#tradebotconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `dataformat_ohlcv` _string_ |  |  |  |
| `dataformat_trades` _string_ |  |  |  |
| `position_adjustment` _string_ |  |  |  |
| `new_pairs_days_ago` _integer_ |  |  |  |
| `download_trades` _boolean_ |  |  |  |
| `max_entry_position_adjustment` _float_ |  |  |  |
| `available_capital` _float_ |  |  |  |
| `amend_last_stake_amount` _boolean_ |  |  |  |
| `last_stake_amount_min_ratio` _float_ |  |  |  |
| `process_only_new_candles` _boolean_ |  |  |  |
| `amount_reserve_percent` _float_ |  |  |  |
| `reduce_df_footprint` _boolean_ |  |  |  |
| `custom_price_max_distance_ratio` _float_ |  |  |  |


#### DataSourceSpec



DataSourceSpec optionally mounts a shared, read-only data cache for a run
(renamed from v1alpha1's DataCacheSpec, which this replaces for one-shot
runs - see RunSpec.Data's doc comment).



_Appears in:_
- [BacktestSpec](#backtestspec)
- [RunSpec](#runspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `pvcName` _string_ | PVCName is the name of a pre-existing, shared RWX PVC to mount<br />read-only at /cache and pass as --datadir. |  |  |
| `downloadArgs` _string array_ | DownloadArgs are optional extra args for "freqtrade download-data",<br />e.g. []string\{"--exchange", "binance", "-t", "1m", "5m", "--days", "30"\}. |  |  |
| `downloadPolicy` _string_ | DownloadPolicy controls when the cache is refreshed before the run. | always | Enum: [always ifMissing never] <br /> |


#### DataSplitParameters







_Appears in:_
- [AIConfig](#aiconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `test_size` _float_ |  |  |  |
| `random_state` _integer_ |  |  |  |
| `shuffle` _boolean_ |  |  |  |


#### ExchangeSpec



ExchangeSpec is B2's primary target: every plaintext credential field
(Key/Secret/Password/UID/WalletAddress/PrivateKey) v1alpha1 carried
behind the allow-plaintext-credentials gate is gone - secretRef is the
only path. AccountID stays: it's an identifier, not a secret, and was
never gated (see configbuilder/exchange.go).



_Appears in:_
- [TradeBotConfigSpec](#tradebotconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ |  |  |  |
| `account_id` _string_ |  |  |  |
| `secretRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#localobjectreference-v1-core)_ | SecretRef references a Secret in this TradeBotConfig's own namespace<br />only (D3) - cross-namespace references are a deliberate, documented<br />constraint, not a TODO. |  |  |
| `ccxt_config` _[JSON](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#json-v1-apiextensions-k8s-io)_ |  |  |  |
| `ccxt_async_config` _[JSON](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#json-v1-apiextensions-k8s-io)_ |  |  |  |
| `ccxt_sync_config` _[JSON](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#json-v1-apiextensions-k8s-io)_ |  |  |  |
| `whitelist` _[PairListSpec](#pairlistspec)_ |  |  |  |
| `blacklist` _[PairListSpec](#pairlistspec)_ |  |  |  |
| `log_responses` _boolean_ |  |  |  |
| `enable_ws` _boolean_ |  |  |  |
| `unknown_fee_rate` _boolean_ | UnknownFeeRate closes A1's bug at the API level: v1alpha1 keeps its<br />original spelling (a served field can't be renamed without a<br />breaking change - see B2), but this fresh v1beta1 field gets the<br />correct name and JSON tag from the start. |  |  |
| `outdated_offset` _integer_ |  |  |  |
| `market_refresh_interval` _integer_ |  |  |  |


#### ExperimentalConfig







_Appears in:_
- [TradeBotConfigSpec](#tradebotconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `block_bad_exchanges` _boolean_ |  |  |  |


#### FeatureParameters







_Appears in:_
- [AIConfig](#aiconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `include_corr_pairlist` _string array_ |  |  |  |
| `include_timeframes` _string array_ |  |  |  |
| `label_period_candles` _integer_ |  |  |  |
| `include_shifted_candles` _integer_ |  |  |  |
| `di_threshold` _float_ |  |  |  |
| `weight_factor` _float_ |  |  |  |
| `principal_component_analysis` _boolean_ |  |  |  |
| `indicator_periods_candles` _integer array_ |  |  |  |
| `use_svm_to_remove_outliers` _boolean_ |  |  |  |
| `plot_feature_importances` _integer_ |  |  |  |
| `svm_params` _[SVMParams](#svmparams)_ |  |  |  |
| `shuffle_after_split` _boolean_ |  |  |  |
| `buffer_train_data_candles` _integer_ |  |  |  |


#### Flow







_Appears in:_
- [OrderSpec](#orderspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `cache_size` _integer_ |  |  |  |
| `max_candles` _integer_ |  |  |  |
| `scale` _float_ |  |  |  |
| `stacked_imbalance_range` _integer_ |  |  |  |
| `imbalance_volume` _integer_ |  |  |  |
| `imbalance_ratio` _float_ |  |  |  |


#### InternalsConfig



InternalsConfig defines internal processing configuration



_Appears in:_
- [TradeBotConfigSpec](#tradebotconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `process_throttle_secs` _integer_ |  |  |  |
| `interval` _integer_ |  |  |  |
| `sd_notify` _boolean_ |  |  |  |


#### IntrospectionSpec



IntrospectionSpec controls whether and how often the operator polls
this bot's own freqtrade REST API (P4-3) - identical to v1alpha1's.



_Appears in:_
- [TradeBotSpec](#tradebotspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ |  | true |  |
| `interval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#duration-v1-meta)_ |  | 60s |  |


#### LoggingConfig







_Appears in:_
- [TradeBotConfigSpec](#tradebotconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `version` _integer_ |  |  |  |


#### ModelRewardParameters







_Appears in:_
- [RLConfig](#rlconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `rr` _float_ |  |  |  |
| `profit_aim` _float_ |  |  |  |


#### ModelTrainingParameters







_Appears in:_
- [AIConfig](#aiconfig)



#### NotificationDiscord







_Appears in:_
- [NotificationSpec](#notificationspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ |  |  |  |
| `webhook_url` _string_ |  |  |  |
| `exit_fill` _object array_ |  |  |  |
| `entry_fill` _object array_ |  |  |  |


#### NotificationSpec







_Appears in:_
- [TradeBotConfigSpec](#tradebotconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `telegram` _[NotificationTelegram](#notificationtelegram)_ |  |  |  |
| `webhook` _[NotificationWebhook](#notificationwebhook)_ |  |  |  |
| `discord` _[NotificationDiscord](#notificationdiscord)_ |  |  |  |


#### NotificationTelegram



NotificationTelegram: Token is gone - see ExchangeSpec's own doc comment,
same B2 reasoning. SecretRef is the only path. ChatID/TopicID stay
plain strings: identifiers, not secrets, same as ExchangeSpec.AccountID.



_Appears in:_
- [NotificationSpec](#notificationspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ |  |  |  |
| `secretRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#localobjectreference-v1-core)_ | SecretRef references a Secret in this TradeBotConfig's own namespace<br />only (D3). |  |  |
| `balance_dust_level` _float_ |  |  |  |
| `reload` _boolean_ |  |  |  |
| `allow_custom_messages` _boolean_ |  |  |  |
| `chat_id` _string_ |  |  |  |
| `topic_id` _string_ |  |  |  |
| `authorized_users` _string array_ |  |  |  |
| `settings` _[NotificationTelegramSettings](#notificationtelegramsettings)_ |  |  |  |


#### NotificationTelegramSettings







_Appears in:_
- [NotificationTelegram](#notificationtelegram)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `status` _string_ |  |  |  |
| `warning` _string_ |  |  |  |
| `startup` _string_ |  |  |  |
| `entry` _string_ |  |  |  |
| `entry_fill` _string_ |  |  |  |
| `entry_cancel` _string_ |  |  |  |
| `exit` _string_ |  |  |  |
| `exit_fill` _string_ |  |  |  |
| `exit_cancel` _string_ |  |  |  |
| `protection_trigger` _string_ |  |  |  |
| `protection_trigger_global` _string_ |  |  |  |


#### NotificationWebhook







_Appears in:_
- [NotificationSpec](#notificationspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ |  |  |  |
| `url` _string_ |  |  |  |
| `entry` _string_ |  |  |  |
| `entry_cancel` _string_ |  |  |  |
| `entry_fill` _string_ |  |  |  |
| `exit` _string_ |  |  |  |
| `exit_cancel` _string_ |  |  |  |
| `exit_fill` _string_ |  |  |  |
| `status` _string_ |  |  |  |
| `allow_custom_messages` _boolean_ | Deprecated: not a real Freqtrade webhook option - see<br />v1alpha1.NotificationWebhook.AllowCustomMessages. Carried over<br />unchanged rather than dropped - B2's scope is credential fields and<br />typed references, not a general API cleanup. |  |  |


#### OrderSpec







_Appears in:_
- [TradeBotConfigSpec](#tradebotconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `types` _[Types](#types)_ |  |  |  |
| `time_in_force` _[TimeInForce](#timeinforce)_ |  |  |  |
| `flow` _[Flow](#flow)_ |  |  |  |


#### PVCSpec







_Appears in:_
- [TBAppConfig](#tbappconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `accessModes` _[PersistentVolumeAccessMode](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#persistentvolumeaccessmode-v1-core) array_ |  |  |  |
| `storageSize` _[Quantity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#quantity-resource-api)_ |  |  |  |
| `storageClassName` _string_ |  |  |  |
| `volumeName` _string_ |  |  |  |
| `annotations` _object (keys:string, values:string)_ |  |  |  |
| `labels` _object (keys:string, values:string)_ |  |  |  |
| `fixVolumePermissions` _boolean_ |  |  |  |


#### PairListSpec



PairListSpec defines the desired state of PairList



_Appears in:_
- [ExchangeSpec](#exchangespec)
- [TradeBotConfigSpec](#tradebotconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `pairs` _string array_ |  |  |  |


#### PairlistConfig



PairlistConfig represents the configuration for a pairlist method



_Appears in:_
- [PairlistMethodsSpec](#pairlistmethodsspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `method` _[PairlistMethod](#pairlistmethod)_ |  |  |  |
| `number_assets` _integer_ |  |  |  |
| `refresh_period` _integer_ |  |  |  |
| `allow_inactive` _boolean_ |  |  |  |
| `sort_key` _string_ |  |  |  |
| `lookback_timeframe` _string_ |  |  |  |
| `lookback_period` _integer_ |  |  |  |
| `lookback_days` _integer_ |  |  |  |
| `min_change_rate` _float_ |  |  |  |
| `producer_name` _string_ |  |  |  |
| `mode` _string_ |  |  |  |
| `processing_mode` _string_ |  |  |  |
| `pairlist_url` _string_ |  |  |  |
| `keep_pairlist_on_failure` _boolean_ |  |  |  |
| `read_timeout` _integer_ |  |  |  |
| `bearer_token` _string_ |  |  |  |
| `save_to_file` _string_ |  |  |  |
| `max_rank` _integer_ |  |  |  |
| `categories` _string array_ |  |  |  |
| `min_days_listed` _integer_ |  |  |  |
| `max_days_listed` _integer_ |  |  |  |
| `min_price` _float_ |  |  |  |
| `max_price` _float_ |  |  |  |
| `max_spread_ratio` _float_ |  |  |  |
| `lookback_days_range` _integer_ |  |  |  |
| `min_rate_of_change` _float_ |  |  |  |
| `max_rate_of_change` _float_ |  |  |  |
| `lookback_days_volatility` _integer_ |  |  |  |
| `min_volatility` _float_ |  |  |  |
| `max_volatility` _float_ |  |  |  |
| `offset` _integer_ |  |  |  |


#### PairlistMethod

_Underlying type:_ _string_

PairlistMethod represents the method used for pair selection



_Appears in:_
- [PairlistConfig](#pairlistconfig)

| Field | Description |
| --- | --- |
| `StaticPairList` |  |
| `VolumePairList` |  |
| `PercentChangePairList` |  |
| `ProducerPairList` |  |
| `RemotePairList` |  |
| `MarketCapPairList` |  |
| `AgeFilter` |  |
| `FullTradesFilter` |  |
| `OffsetFilter` |  |
| `PerformanceFilter` |  |
| `PrecisionFilter` |  |
| `PriceFilter` |  |
| `ShuffleFilter` |  |
| `SpreadFilter` |  |
| `RangeStabilityFilter` |  |
| `VolatilityFilter` |  |


#### PairlistMethodsSpec



PairlistMethodsSpec defines the desired state of PairlistMethods



_Appears in:_
- [TradeBotConfigSpec](#tradebotconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `methods` _[PairlistConfig](#pairlistconfig) array_ |  |  |  |


#### PodSpec



PodSpec carries resource/scheduling overrides for a run's pod - a trimmed
version of v1alpha1's PodSpec: no ServiceName (StatefulSet-only) and no
probes (nothing polls a one-shot Job's api_server, because it never has one).



_Appears in:_
- [BacktestSpec](#backtestspec)
- [RunSpec](#runspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `image` _string_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#resourcerequirements-v1-core)_ |  |  |  |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#envvar-v1-core) array_ |  |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#volumemount-v1-core) array_ |  |  |  |
| `volumes` _[Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#volume-v1-core) array_ |  |  |  |
| `securityContext` _[PodSecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#podsecuritycontext-v1-core)_ |  |  |  |
| `initContainers` _[Container](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#container-v1-core) array_ |  |  |  |
| `imagePullSecrets` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#localobjectreference-v1-core) array_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#affinity-v1-core)_ |  |  |  |
| `nodeSelector` _object (keys:string, values:string)_ |  |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#toleration-v1-core) array_ |  |  |  |
| `topologySpreadConstraints` _[TopologySpreadConstraint](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#topologyspreadconstraint-v1-core) array_ |  |  |  |


#### PricingCheckDepthOfMarket



PricingCheckDepthOfMarket defines the check_depth_of_market settings



_Appears in:_
- [PricingSpec](#pricingspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ |  |  |  |
| `bids_to_ask_delta` _float_ |  |  |  |


#### PricingSpec



PricingSpec defines the desired state of Pricing



_Appears in:_
- [TradeBotConfigSpec](#tradebotconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `price_side` _string_ |  |  |  |
| `price_last_balance` _float_ |  |  |  |
| `use_order_book` _boolean_ |  |  |  |
| `order_book_top` _integer_ |  |  |  |
| `check_depth_of_market` _[PricingCheckDepthOfMarket](#pricingcheckdepthofmarket)_ |  |  |  |


#### RLConfig







_Appears in:_
- [AIConfig](#aiconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `drop_ohlc_from_features` _boolean_ |  |  |  |
| `train_cycles` _integer_ |  |  |  |
| `max_trade_duration_candles` _integer_ |  |  |  |
| `add_state_info` _boolean_ |  |  |  |
| `max_training_drawdown_pct` _float_ |  |  |  |
| `cpu_count` _integer_ |  |  |  |
| `model_type` _string_ |  |  |  |
| `policy_type` _string_ |  |  |  |
| `net_arch` _integer array_ |  |  |  |
| `randomize_starting_position` _boolean_ |  |  |  |
| `progress_bar` _boolean_ |  |  |  |
| `model_reward_parameters` _[ModelRewardParameters](#modelrewardparameters)_ |  |  |  |


#### ResultsSpec



ResultsSpec configures the PVC a run's results are written to (D1).



_Appears in:_
- [BacktestSpec](#backtestspec)
- [RunSpec](#runspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `size` _[Quantity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#quantity-resource-api)_ |  | 1Gi |  |
| `storageClassName` _string_ | StorageClassName defaults to the cluster's default storage class when unset. |  |  |
| `retentionPolicy` _string_ | RetentionPolicy decided once, at creation, like every other field<br />(the whole spec is immutable) - Delete GCs the results PVC when this<br />CR is deleted (the working manual reaper D10 documents); Retain<br />strips the owner reference instead, the same pattern<br />TradeBot.finalizers.go uses for freqtrade.io/preserve-data. | Delete | Enum: [Delete Retain] <br /> |


#### RiskManagementSpec



RiskManagementSpec defines the desired state of RiskManagement



_Appears in:_
- [TradeBotConfigSpec](#tradebotconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `minimal_roi` _object (keys:string, values:float)_ |  |  |  |
| `stoploss` _float_ |  |  |  |
| `trailing_stop` _boolean_ |  |  |  |
| `trailing_stop_positive` _float_ |  |  |  |
| `trailing_stop_positive_offset` _float_ |  |  |  |
| `trailing_only_offset_is_reached` _boolean_ |  |  |  |
| `use_exit_signal` _boolean_ |  |  |  |
| `exit_profit_only` _boolean_ |  |  |  |
| `exit_profit_offset` _float_ |  |  |  |
| `fee` _float_ |  |  |  |
| `ignore_roi_if_entry_signal` _boolean_ |  |  |  |
| `ignore_buying_expired_candle_after` _integer_ |  |  |  |
| `minimum_trade_amount` _integer_ |  |  |  |
| `targeted_trade_amount` _integer_ |  |  |  |
| `lookahead_analysis_export_filename` _string_ |  |  |  |
| `startup_candle` _integer_ |  |  |  |
| `liquidation_buffer` _float_ |  |  |  |
| `backtest_breakdown` _string array_ |  |  |  |


#### RunSpec



RunSpec is the set of fields a one-shot freqtrade run needs regardless of
which command it runs - embedded with json:",inline" (invisible in the
serialized JSON) so Hyperopt (D7, a later minor) can reuse it with zero
API churn on Backtest. Retrofitting this split after v1beta1 is stored
would be a breaking change for no benefit, so it's done now even though
Hyperopt itself isn't built yet.



_Appears in:_
- [BacktestSpec](#backtestspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `configRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#localobjectreference-v1-core)_ | ConfigRef is the TradeBotConfig this run renders config.json from,<br />same namespace only (D3). |  |  |
| `strategyRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#localobjectreference-v1-core)_ | StrategyRef is the Strategy this run executes, same namespace only (D3). |  |  |
| `timerange` _string_ | Timerange is freqtrade's --timerange value, e.g. "20230101-20230201"<br />or "20230101-" for open-ended. |  | Pattern: `^\d\{8\}-(\d\{8\})?$` <br /> |
| `timeframe` _string_ | Timeframe is freqtrade's --timeframe value, e.g. "5m", "1h", "1d". |  | Pattern: `^\d+[mhdw]$` <br /> |
| `pairs` _string array_ |  |  | MaxItems: 256 <br /> |
| `data` _[DataSourceSpec](#datasourcespec)_ | Data optionally mounts a shared read-only cache PVC as --datadir,<br />with an init container downloading into it first per DownloadPolicy -<br />moved here from TradeBot.Spec.Data (v1alpha1), which only ever made<br />sense for a one-shot run in the first place (see pod.go's isTrade gate<br />on it in the old package). |  |  |
| `results` _[ResultsSpec](#resultsspec)_ | Results configures the per-run PVC this run's output is written to<br />(D1) - the CR is the run's identity, so unlike v1alpha1's Job mode<br />there is no orphaned-volume or spec-hash-in-the-name problem to work<br />around. |  |  |
| `pod` _[PodSpec](#podspec)_ | Pod carries resource requests/limits and scheduling overrides for the<br />run's pod. |  |  |
| `ttlSecondsAfterFinished` _integer_ | TTLSecondsAfterFinished reaps the finished Job (not the results PVC -<br />see ResultsSpec.RetentionPolicy, and D10 on why nothing prunes results<br />automatically) this many seconds after it completes. | 86400 |  |
| `extraArgs` _string array_ | ExtraArgs appends raw freqtrade CLI arguments (D8/D5): typed fields<br />above remain the documented surface for anything they cover. This is<br />a pressure valve for flags this API hasn't caught up to yet, not an<br />alternative to them - guarded by the freqtrade.io/allow-extra-args:<br />"true" annotation (rejected by the admission webhook without it) and<br />checked against a denylist of flags the operator itself controls<br />(--config, --strategy, --strategy-path, --db-url, --logfile,<br />--userdir, --datadir); the controller emits a warning Event whenever<br />it's used so its usage stays visible rather than quietly load-bearing. |  | MaxItems: 64 <br /> |


#### SVMParams







_Appears in:_
- [FeatureParameters](#featureparameters)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `shuffle` _boolean_ |  |  |  |
| `nu` _float_ |  |  |  |


#### ServiceSpec







_Appears in:_
- [TBAppConfig](#tbappconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[ServiceType](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#servicetype-v1-core)_ |  |  |  |
| `ports` _[ServicePort](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#serviceport-v1-core) array_ |  |  |  |
| `selector` _object (keys:string, values:string)_ |  |  |  |
| `annotations` _object (keys:string, values:string)_ |  |  |  |


#### TBAppConfig



TBAppConfig, PodSpec/ServiceSpec/PVCSpec, IntrospectionSpec and
BotStatus below are field-for-field identical to their v1alpha1
namesakes - P6-4/P6-5 only touches reference types and Job-mode fields,
neither of which any of these have. Redeclared here rather than
imported: v1alpha1's own conversion functions (ConvertTo/ConvertFrom)
must live in package v1alpha1 (Go methods have to live alongside their
receiver type), which already means api/v1alpha1 imports api/v1beta1 -
the reverse import this package aliasing them from v1alpha1 would need
is therefore not available; that would be a cycle. The conversion
functions use a JSON-roundtrip helper for exactly these types
(api/v1alpha1/tradebot_conversion.go), which is what the round-trip
fuzz tests are for: a tag mismatch between the two copies would fail a
fuzz round-trip immediately, not sit undetected.


PodSpec here is TradeBotPodSpec, not PodSpec - api/v1beta1/backtest_types.go
(P6-1) already uses the bare name for Backtest's own, differently-shaped
(no probes, no ServiceName) one-shot-run pod spec.



_Appears in:_
- [TradeBotSpec](#tradebotspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `pod` _[TradeBotPodSpec](#tradebotpodspec)_ |  |  |  |
| `service` _[ServiceSpec](#servicespec)_ |  |  |  |
| `pvc` _[PVCSpec](#pvcspec)_ |  |  |  |


#### TimeInForce



TimeInForce defines options for order time in force



_Appears in:_
- [OrderSpec](#orderspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `entry` _string_ |  |  |  |
| `exit` _string_ |  |  |  |


#### TradeBot







_Appears in:_
- [TradeBotList](#tradebotlist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `freqtrade.io/v1beta1` | | |
| `kind` _string_ | `TradeBot` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[TradeBotSpec](#tradebotspec)_ |  |  |  |
| `status` _[TradeBotStatus](#tradebotstatus)_ |  |  |  |


#### TradeBotConfig







_Appears in:_
- [TradeBotConfigList](#tradebotconfiglist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `freqtrade.io/v1beta1` | | |
| `kind` _string_ | `TradeBotConfig` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[TradeBotConfigSpec](#tradebotconfigspec)_ |  |  |  |
| `status` _[TradeBotConfigStatus](#tradebotconfigstatus)_ |  |  |  |




#### TradeBotConfigList









| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `freqtrade.io/v1beta1` | | |
| `kind` _string_ | `TradeBotConfigList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[TradeBotConfig](#tradebotconfig) array_ |  |  |  |


#### TradeBotConfigSpec



TradeBotConfigSpec defines the desired state of TradeBotConfig. B2
(REMAINING-WORK.md): every plaintext credential field v1alpha1 carried
behind the freqtrade.io/allow-plaintext-credentials gate is gone here -
secretRef (now a typed, same-namespace-only corev1.LocalObjectReference
rather than a bare string) is the only path. Everything else is
field-for-field identical to v1alpha1.TradeBotConfigSpec.



_Appears in:_
- [TradeBotConfig](#tradebotconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `bot` _[BotConfig](#botconfig)_ |  |  |  |
| `ai` _[AIConfig](#aiconfig)_ |  |  |  |
| `data` _[DataConfig](#dataconfig)_ |  |  |  |
| `advanced` _[AdvancedConfig](#advancedconfig)_ |  |  |  |
| `timeout` _[UnfilledTimeoutConfig](#unfilledtimeoutconfig)_ |  |  |  |
| `internals` _[InternalsConfig](#internalsconfig)_ |  |  |  |
| `exchange` _[ExchangeSpec](#exchangespec)_ |  |  |  |
| `pairlist` _[PairListSpec](#pairlistspec)_ |  |  |  |
| `pairlist_method` _[PairlistMethodsSpec](#pairlistmethodsspec)_ |  |  |  |
| `entry_pricing` _[PricingSpec](#pricingspec)_ |  |  |  |
| `exit_pricing` _[PricingSpec](#pricingspec)_ |  |  |  |
| `order` _[OrderSpec](#orderspec)_ |  |  |  |
| `risk_management` _[RiskManagementSpec](#riskmanagementspec)_ |  |  |  |
| `notification` _[NotificationSpec](#notificationspec)_ |  |  |  |
| `apiServer` _[APIServerConfig](#apiserverconfig)_ |  |  |  |
| `experimental` _[ExperimentalConfig](#experimentalconfig)_ |  |  |  |
| `logging` _[LoggingConfig](#loggingconfig)_ |  |  |  |


#### TradeBotConfigStatus



TradeBotConfigStatus defines the observed state of TradeBotConfig -
identical to v1alpha1's.



_Appears in:_
- [TradeBotConfig](#tradebotconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `phase` _string_ | Phase is a derived, human-facing summary for the printer column only -<br />it is never the source of truth. Conditions are. |  |  |
| `message` _string_ |  |  |  |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#condition-v1-meta) array_ |  |  |  |
| `observedGeneration` _integer_ | ObservedGeneration is the most recent metadata.generation this status<br />was computed from. |  |  |


#### TradeBotList









| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `freqtrade.io/v1beta1` | | |
| `kind` _string_ | `TradeBotList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[TradeBot](#tradebot) array_ |  |  |  |


#### TradeBotPodSpec







_Appears in:_
- [TBAppConfig](#tbappconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `image` _string_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#resourcerequirements-v1-core)_ |  |  |  |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#envvar-v1-core) array_ |  |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#volumemount-v1-core) array_ |  |  |  |
| `volumes` _[Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#volume-v1-core) array_ |  |  |  |
| `securityContext` _[PodSecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#podsecuritycontext-v1-core)_ |  |  |  |
| `initContainers` _[Container](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#container-v1-core) array_ |  |  |  |
| `serviceName` _string_ |  |  |  |
| `imagePullSecrets` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#localobjectreference-v1-core) array_ |  |  |  |
| `livenessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#probe-v1-core)_ |  |  |  |
| `readinessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#probe-v1-core)_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#affinity-v1-core)_ |  |  |  |
| `antiAffinity` _[PodAntiAffinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#podantiaffinity-v1-core)_ |  |  |  |
| `nodeSelector` _object (keys:string, values:string)_ |  |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#toleration-v1-core) array_ |  |  |  |
| `topologySpreadConstraints` _[TopologySpreadConstraint](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#topologyspreadconstraint-v1-core) array_ |  |  |  |


#### TradeBotSpec



TradeBotSpec defines the desired state of TradeBot. Trade-only (P6-4,
D5): FreqtradeCommand/FreqtradeArguments/Data - the whole one-shot-run
branch - are gone, moved to Backtest (D1, P6-1). A v1alpha1 TradeBot
using them has no v1beta1 equivalent at all; the conversion webhook
(api/v1alpha1/tradebot_conversion.go) rejects converting one rather
than silently dropping what it was actually configured to do.



_Appears in:_
- [TradeBot](#tradebot)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `configRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#localobjectreference-v1-core)_ | ConfigRef references the TradeBotConfig this bot renders config.json<br />from, same namespace only (D3/P6-5). |  |  |
| `strategyRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#localobjectreference-v1-core)_ | StrategyRef references the Strategy this bot runs, same namespace<br />only (D3/P6-5). |  |  |
| `app` _[TBAppConfig](#tbappconfig)_ |  |  |  |
| `updateStrategy` _string_ | UpdateStrategy controls what happens when a config change can't take<br />effect without a restart - see v1alpha1.TradeBotSpec's identical<br />field for the full explanation (P2-4). | Manual | Enum: [Manual Auto] <br /> |
| `introspection` _[IntrospectionSpec](#introspectionspec)_ | Introspection controls the operator's own polling of this bot's<br />freqtrade REST API for live trading state (P4-3, D4). |  |  |


#### TradeBotStatus



TradeBotStatus defines the observed state of TradeBot.



_Appears in:_
- [TradeBot](#tradebot)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `phase` _string_ | Phase is a derived, human-facing summary for the printer column only -<br />it is never the source of truth. Conditions are. |  |  |
| `message` _string_ |  |  |  |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v/#condition-v1-meta) array_ |  |  |  |
| `observedGeneration` _integer_ | ObservedGeneration is the most recent metadata.generation this status<br />was computed from. |  |  |
| `appliedConfigHash` _string_ | AppliedConfigHash is sha256(config.json bytes + strategy script<br />bytes), truncated to 16 hex characters - see v1alpha1.TradeBotStatus's<br />identical field. |  |  |
| `resolvedImage` _string_ | ResolvedImage is the exact freqtrade image reference the running<br />workload was built with (P3-3). |  |  |
| `bot` _[BotStatus](#botstatus)_ | Bot is this bot's own live trading state, as last observed by the<br />operator's poller (P4-3). |  |  |


#### Types



Types defines various order type configurations



_Appears in:_
- [OrderSpec](#orderspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `entry` _string_ |  |  |  |
| `exit` _string_ |  |  |  |
| `emergency_exit` _string_ |  |  |  |
| `force_entry` _string_ |  |  |  |
| `force_exit` _string_ |  |  |  |
| `stoploss` _string_ |  |  |  |
| `stoploss_on_exchange` _boolean_ |  |  |  |
| `stoploss_on_exchange_interval` _integer_ |  |  |  |
| `stoploss_on_exchange_limit_ratio` _float_ |  |  |  |


#### UnfilledTimeoutConfig



UnfilledTimeoutConfig defines timeout settings for unfilled orders



_Appears in:_
- [TradeBotConfigSpec](#tradebotconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `entry` _integer_ |  |  |  |
| `exit` _integer_ |  |  |  |
| `exit_timeout_count` _integer_ |  |  |  |
| `unit` _string_ |  |  |  |


