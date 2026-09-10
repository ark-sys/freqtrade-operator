package tradebot

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	"github.com/ark-sys/freqtrade-operator/controllers/shared"
	"github.com/ark-sys/freqtrade-operator/controllers/tradebot/botclient"
	"github.com/ark-sys/freqtrade-operator/controllers/tradebot/resources"
	"github.com/ark-sys/freqtrade-operator/controllers/tradebotconfig/configbuilder"
)

const (
	// defaultPollWorkers bounds how many bots can be polled concurrently
	// when BotPoller.Workers is unset.
	defaultPollWorkers = 4
	// schedulerTick is how often BotPoller re-checks which bots are due -
	// much shorter than any bot's own interval (the webhook enforces a
	// 10s floor on that), so a bot becomes eligible soon after it's
	// actually due rather than waiting out a long fixed cycle.
	schedulerTick = 5 * time.Second
	// maxBackoffMultiple caps a struggling bot's backoff at this multiple
	// of its own configured interval (the plan's own "~10x the interval").
	maxBackoffMultiple = 10
	// pollTimeout bounds one full poll attempt (ping + up to 5 more
	// sequential calls), not just a single HTTP request - botclient's own
	// 5s per-request timeout already bounds each of those individually.
	pollTimeout = 20 * time.Second
)

// BotPoller polls every trade-mode TradeBot's own freqtrade REST API on its
// own interval (P4-3, implements D4) and writes what it learns into
// status.bot, the BotReachable condition, and the freqtrade_bot_* metrics -
// never into anything spec, since this is read-only (D2/D9: P4-4 is a
// deliberately separate, later task for anything that writes to a bot).
//
// A manager.Runnable, not a reconciler: polling N bots inside Reconcile
// would tie reconcile latency to how responsive those bots' own APIs
// happen to be, and one hung bot would block the TradeBot controller's
// entire (size-1) work queue. NeedLeaderElection is true, so only the
// leader replica ever polls - two replicas polling the same bot would
// double its request rate and race on whose status write wins, for no
// benefit.
type BotPoller struct {
	Client   client.Client
	Recorder record.EventRecorder
	// Workers bounds how many bots can be polled concurrently. Zero means
	// defaultPollWorkers.
	Workers int

	mu    sync.Mutex
	state map[types.NamespacedName]*pollState
}

// pollState is this poller's own per-bot memory - nothing else reads or
// writes it, and losing it (a leader failover, a restart) only means
// backoff resets and every bot looks newly-due, not any actual data loss.
type pollState struct {
	nextPollAt          time.Time
	consecutiveFailures int
	inFlight            bool
	// strategy/exchange/dryRun are remembered so DeleteBotMetrics can
	// reconstruct the exact label set a since-deleted bot's series was
	// last recorded under - by the time a bot disappears from the List,
	// nothing else can supply them anymore.
	strategy, exchange, dryRun string
}

func (p *BotPoller) NeedLeaderElection() bool { return true }

func (p *BotPoller) workers() int {
	if p.Workers > 0 {
		return p.Workers
	}
	return defaultPollWorkers
}

// Start implements manager.Runnable. Blocks until ctx is cancelled.
func (p *BotPoller) Start(ctx context.Context) error {
	logger := log.FromContext(ctx).WithName("bot-poller")
	p.mu.Lock()
	p.state = make(map[types.NamespacedName]*pollState)
	p.mu.Unlock()

	queue := make(chan types.NamespacedName, p.workers()*2)

	var wg sync.WaitGroup
	for i := 0; i < p.workers(); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for key := range queue {
				p.pollOneSafely(ctx, logger, key)
			}
		}()
	}

	ticker := time.NewTicker(schedulerTick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			close(queue)
			wg.Wait()
			return nil
		case <-ticker.C:
			p.scheduleDue(ctx, logger, queue)
		}
	}
}

// scheduleDue lists every TradeBot, enqueues whichever are due for a poll,
// marks any trade-mode bot with introspection explicitly disabled so its
// BotReachable condition doesn't linger stale at whatever it last was, and
// forgets (state and metrics both) any bot this poller was tracking that no
// longer exists or no longer wants polling.
func (p *BotPoller) scheduleDue(ctx context.Context, logger logr.Logger, queue chan<- types.NamespacedName) {
	var list freqtradev1alpha1.TradeBotList
	if err := p.Client.List(ctx, &list); err != nil {
		logger.Error(err, "failed to list TradeBots for polling")
		return
	}

	now := time.Now()
	seen := make(map[types.NamespacedName]struct{}, len(list.Items))
	// Collected under the lock below, patched after releasing it - a status
	// write is a network call, and nothing here needs pollState's mutex.
	var needsDisabledCondition []*freqtradev1alpha1.TradeBot

	p.mu.Lock()
	for i := range list.Items {
		tradeBot := &list.Items[i]
		key := types.NamespacedName{Name: tradeBot.Name, Namespace: tradeBot.Namespace}

		if !isTradeMode(tradeBot) {
			continue // a Job never runs an api_server (see pod.go); BotReachable doesn't apply to it at all
		}
		if !introspectionEnabled(tradeBot) {
			// Checked against the List snapshot already in hand, not a fresh
			// Get, so a bot that's already correctly marked costs nothing
			// here - only an actual transition reaches PatchStatus below.
			if !hasIntrospectionDisabledCondition(tradeBot) {
				needsDisabledCondition = append(needsDisabledCondition, tradeBot)
			}
			continue // never "seen" - if it was previously tracked, the cleanup pass below forgets it
		}
		seen[key] = struct{}{}

		st, ok := p.state[key]
		if !ok {
			st = &pollState{nextPollAt: now} // poll a newly-seen bot on the very next tick
			p.state[key] = st
		}
		if st.inFlight || now.Before(st.nextPollAt) {
			continue
		}
		st.inFlight = true
		select {
		case queue <- key:
		default:
			st.inFlight = false // queue is momentarily full; this bot is picked up again next tick
		}
	}
	for key, st := range p.state {
		if _, ok := seen[key]; ok {
			continue
		}
		shared.DeleteBotMetrics(key.Namespace, key.Name, st.strategy, st.exchange, st.dryRun)
		delete(p.state, key)
	}
	p.mu.Unlock()

	for _, tradeBot := range needsDisabledCondition {
		p.setIntrospectionDisabledCondition(ctx, logger, tradeBot)
	}
}

// isTradeMode reports whether tradeBot runs "trade" (explicitly or via the
// empty-string default) rather than a one-shot Job command - only a trade
// bot ever runs an api_server for the poller to reach, or has a
// BotReachable condition that means anything.
func isTradeMode(tradeBot *freqtradev1alpha1.TradeBot) bool {
	cmd := strings.TrimSpace(tradeBot.Spec.FreqtradeCommand)
	return cmd == "" || cmd == "trade"
}

// introspectionEnabled reports spec.introspection.enabled (defaulted true,
// but explicitly checked here since a plain Go literal in a test won't have
// CRD defaulting applied).
func introspectionEnabled(tradeBot *freqtradev1alpha1.TradeBot) bool {
	if tradeBot.Spec.Introspection == nil || tradeBot.Spec.Introspection.Enabled == nil {
		return true
	}
	return *tradeBot.Spec.Introspection.Enabled
}

// shouldPoll reports whether tradeBot is eligible for polling at all.
func shouldPoll(tradeBot *freqtradev1alpha1.TradeBot) bool {
	return isTradeMode(tradeBot) && introspectionEnabled(tradeBot)
}

// hasIntrospectionDisabledCondition reports whether tradeBot's BotReachable
// condition already reflects this generation's introspection-disabled
// state, so scheduleDue can skip the PatchStatus call on the (very common)
// steady-state tick where there's nothing new to write.
func hasIntrospectionDisabledCondition(tradeBot *freqtradev1alpha1.TradeBot) bool {
	cond := meta.FindStatusCondition(tradeBot.Status.Conditions, freqtradev1alpha1.ConditionBotReachable)
	return cond != nil && cond.Reason == freqtradev1alpha1.ReasonIntrospectionDisabled &&
		cond.ObservedGeneration == tradeBot.Generation
}

// setIntrospectionDisabledCondition marks tradeBot's BotReachable Unknown
// with ReasonIntrospectionDisabled - the doc comment on that reason
// promises this stays neither True nor False, distinct from an actual poll
// failure, and status.bot is deliberately left untouched (possibly stale,
// but the condition itself is what tells a reader not to trust it).
func (p *BotPoller) setIntrospectionDisabledCondition(
	ctx context.Context, logger logr.Logger, tradeBot *freqtradev1alpha1.TradeBot,
) {
	err := patchTradeBotStatus(ctx, p.Client, tradeBot, func() {
		meta.SetStatusCondition(&tradeBot.Status.Conditions, metav1.Condition{
			Type:               freqtradev1alpha1.ConditionBotReachable,
			Status:             metav1.ConditionUnknown,
			Reason:             freqtradev1alpha1.ReasonIntrospectionDisabled,
			Message:            "spec.introspection.enabled is false; the operator is not polling this bot",
			ObservedGeneration: tradeBot.Generation,
		})
	})
	if err != nil {
		logger.Error(err, "failed to set IntrospectionDisabled condition", "tradebot", client.ObjectKeyFromObject(tradeBot))
	}
}

// pollInterval returns spec.introspection.interval, or the +kubebuilder
// default (60s) for a bot with no explicit value - the same "an admitted
// object always has this defaulted, but don't assume every caller went
// through admission" stance as shouldPoll above.
func pollInterval(tradeBot *freqtradev1alpha1.TradeBot) time.Duration {
	if tradeBot.Spec.Introspection != nil && tradeBot.Spec.Introspection.Interval.Duration > 0 {
		return tradeBot.Spec.Introspection.Interval.Duration
	}
	return 60 * time.Second
}

// pollOneSafely recovers a panic from a single bot's poll attempt, logs it,
// and moves on - the rest of the operator (every other TradeBot's own
// reconciliation, unrelated to this best-effort background feature) must
// not go down because one bot's response triggered a bug in this code.
// Nothing else in this manager's Runnables gets that protection for free:
// controller-runtime's own panic recovery is specific to its reconcile
// loop, not to arbitrary manager.Runnable implementations like this one.
func (p *BotPoller) pollOneSafely(ctx context.Context, logger logr.Logger, key types.NamespacedName) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error(fmt.Errorf("panic: %v", r), "recovered from a panic while polling a bot", "tradebot", key)
		}
	}()
	p.pollOne(ctx, logger, key)
}

// pollOne performs one full poll attempt for key end to end: fetch the
// current TradeBot, poll its API, update this poller's own backoff state,
// write status.bot/BotReachable, and push the freqtrade_bot_* gauges.
func (p *BotPoller) pollOne(ctx context.Context, logger logr.Logger, key types.NamespacedName) {
	defer func() {
		p.mu.Lock()
		if st, ok := p.state[key]; ok {
			st.inFlight = false
		}
		p.mu.Unlock()
	}()

	var tradeBot freqtradev1alpha1.TradeBot
	if err := p.Client.Get(ctx, key, &tradeBot); err != nil {
		// Deleted between being scheduled and now, or a transient read
		// error - either way there's nothing to poll or write status onto;
		// the next scheduler tick's List will notice a real deletion and
		// clean up state/metrics on its own.
		return
	}

	pollCtx, cancel := context.WithTimeout(ctx, pollTimeout)
	freshStatus, reachable, exchange, balance := p.poll(pollCtx, &tradeBot)
	cancel()

	p.recordMetrics(key, &tradeBot, freshStatus, reachable, exchange, balance)
	p.updateBackoff(key, &tradeBot, reachable)

	if err := patchTradeBotStatus(ctx, p.Client, &tradeBot, func() {
		tradeBot.Status.Bot = mergeBotStatus(tradeBot.Status.Bot, freshStatus, reachable, metav1.Now())
		meta.SetStatusCondition(&tradeBot.Status.Conditions, reachable)
	}); err != nil {
		logger.Error(err, "failed to patch TradeBot status after poll", "tradebot", key)
	}
}

// mergeBotStatus computes the new status.bot from this poll attempt's
// outcome and whatever was already there. A successful poll (fresh != nil)
// always fully replaces it - nothing should ever mix data from two
// different poll attempts. A failed one must not erase the last known-good
// state (open trades, profit, ...) just because this one attempt couldn't
// refresh it - only LastPollTime/LastPollError move, so a reader can tell
// it's now stale without losing it entirely.
func mergeBotStatus(
	existing, fresh *freqtradev1alpha1.BotStatus, reachable metav1.Condition, now metav1.Time,
) *freqtradev1alpha1.BotStatus {
	if fresh != nil {
		fresh.LastPollTime = &now
		return fresh
	}
	if existing == nil {
		existing = &freqtradev1alpha1.BotStatus{}
	}
	existing.LastPollTime = &now
	existing.LastPollError = reachable.Message
	return existing
}

func (p *BotPoller) updateBackoff(
	key types.NamespacedName, tradeBot *freqtradev1alpha1.TradeBot, reachable metav1.Condition,
) {
	interval := pollInterval(tradeBot)

	p.mu.Lock()
	defer p.mu.Unlock()
	st, ok := p.state[key]
	if !ok {
		return // forgotten by scheduleDue's cleanup pass while this poll was in flight
	}
	if reachable.Status == metav1.ConditionTrue {
		st.consecutiveFailures = 0
		st.nextPollAt = time.Now().Add(interval)
		return
	}
	st.consecutiveFailures++
	shift := st.consecutiveFailures
	if shift > 4 { // 1<<4 = 16x already exceeds the 10x cap below; no need to grow shift (or overflow) further
		shift = 4
	}
	backoff := interval * time.Duration(uint(1)<<uint(shift))
	if maxBackoff := interval * maxBackoffMultiple; backoff > maxBackoff {
		backoff = maxBackoff
	}
	st.nextPollAt = time.Now().Add(backoff)
}

// poll resolves credentials and this bot's own base URL, then delegates
// the actual HTTP work to pollWithClient. Split out specifically so tests
// can exercise pollWithClient directly against an httptest.Server - a real
// bot's base URL is in-cluster DNS (svc.cluster.local), which nothing in a
// unit test can make resolve to a local test server.
func (p *BotPoller) poll(
	ctx context.Context, tradeBot *freqtradev1alpha1.TradeBot,
) (botStatus *freqtradev1alpha1.BotStatus, reachable metav1.Condition, exchange string, balance *float64) {
	username, password, exchange, err := resolveCredentials(ctx, p.Client, tradeBot)
	if err != nil {
		return nil, unreachableCondition(tradeBot, freqtradev1alpha1.ReasonConnectionRefused, err), exchange, nil
	}

	botStatus, reachable, balance = pollWithClient(ctx, botclient.New(botBaseURL(tradeBot), username, password), tradeBot)
	return botStatus, reachable, exchange, balance
}

// botBaseURL returns tradeBot's own in-cluster freqtrade REST API base URL
// - shared by the poller (P4-3) and the TradeBot reconciler's state
// reconciliation (P4-4), so both talk to a bot exactly the same way.
func botBaseURL(tradeBot *freqtradev1alpha1.TradeBot) string {
	return fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", tradeBot.Name, tradeBot.Namespace, resources.FreqtradeAPIPort)
}

// pollWithClient does the actual HTTP work for one bot: ping, then (if
// reachable) show_config and version. These three are required and
// determine BotReachable - all three succeed regardless of whether the
// bot is actively trading (verified directly against a real bot: an
// api_server that's up but stopped still answers all three fine). Count,
// Profit, and Balance are best-effort from there: freqtrade errors all
// three whenever the bot isn't running - state == "stopped" is a normal,
// deliberate operational state, not a failure - so a failure on any of
// them just leaves that one field unset (noted in LastPollError) without
// touching BotReachable or blocking the other two. Only a Ping/ShowConfig/
// Version failure returns a nil status and a False BotReachable condition
// - a bot whose API itself doesn't answer gets no partial/stale
// status.bot, rather than a mix of fresh and hours-old fields with no way
// to tell which is which. balance is returned separately since it isn't
// one of BotStatus's own fields (see BalanceResponse's doc comment) - only
// the metric needs it; nil (as opposed to a pointer to 0) means the
// balance call itself failed, so recordMetrics knows not to overwrite the
// gauge's last known value with a false zero.
func pollWithClient(
	ctx context.Context, c *botclient.Client, tradeBot *freqtradev1alpha1.TradeBot,
) (botStatus *freqtradev1alpha1.BotStatus, reachable metav1.Condition, balance *float64) {
	if err := c.Ping(ctx); err != nil {
		return nil, unreachableCondition(tradeBot, classifyError(err), err), nil
	}
	config, err := c.ShowConfig(ctx)
	if err != nil {
		return nil, unreachableCondition(tradeBot, classifyError(err), err), nil
	}
	version, err := c.Version(ctx)
	if err != nil {
		return nil, unreachableCondition(tradeBot, classifyError(err), err), nil
	}

	// LastPollTime is stamped by the caller (pollOne), for both this
	// success path and the failure paths above uniformly.
	botStatus = &freqtradev1alpha1.BotStatus{
		State:   config.State,
		Version: version.Version,
		DryRun:  ptr.To(config.DryRun),
	}

	var degraded []string
	if count, err := c.Count(ctx); err != nil {
		degraded = append(degraded, "count: "+err.Error())
	} else {
		botStatus.OpenTrades = ptr.To(count.Current)
		botStatus.MaxOpenTrades = ptr.To(count.Max)
	}
	if profit, err := c.Profit(ctx); err != nil {
		degraded = append(degraded, "profit: "+err.Error())
	} else {
		botStatus.TotalProfitAbs = formatFloat(profit.ProfitAllCoin)
		botStatus.TotalProfitPct = formatFloat(profit.ProfitAllPercent)
	}
	if balanceResp, err := c.Balance(ctx); err != nil {
		degraded = append(degraded, "balance: "+err.Error())
	} else {
		balance = ptr.To(balanceResp.Total)
	}
	if len(degraded) > 0 {
		botStatus.LastPollError = "best-effort fields unavailable: " + strings.Join(degraded, "; ")
	}

	reachable = metav1.Condition{
		Type: freqtradev1alpha1.ConditionBotReachable, Status: metav1.ConditionTrue,
		Reason: freqtradev1alpha1.ReasonAsExpected, ObservedGeneration: tradeBot.Generation,
	}
	return botStatus, reachable, balance
}

// recordMetrics pushes the freqtrade_bot_* gauges (P4-3) after one poll
// attempt, successful or not - BotUp/BotLastPollTimestampSeconds always
// update; the rest only carry meaning (and are only set) on success, since
// a failed poll has no fresher state to report than what's already there.
// Also remembers this bot's exact label set in pollState, the only way
// scheduleDue's later cleanup pass can call DeleteBotMetrics with the
// right labels once the bot itself is gone.
func (p *BotPoller) recordMetrics(
	key types.NamespacedName, tradeBot *freqtradev1alpha1.TradeBot,
	botStatus *freqtradev1alpha1.BotStatus, reachable metav1.Condition, exchange string, balance *float64,
) {
	dryRun := "unknown"
	if botStatus != nil && botStatus.DryRun != nil {
		dryRun = strconv.FormatBool(*botStatus.DryRun)
	}
	labels := prometheus.Labels{
		"namespace": tradeBot.Namespace, "tradebot": tradeBot.Name,
		"strategy": tradeBot.Spec.Strategy, "exchange": exchange, "dry_run": dryRun,
	}

	p.mu.Lock()
	if st, ok := p.state[key]; ok {
		st.strategy, st.exchange, st.dryRun = tradeBot.Spec.Strategy, exchange, dryRun
	}
	p.mu.Unlock()

	up := 0.0
	if reachable.Status == metav1.ConditionTrue {
		up = 1
	}
	shared.BotUp.With(labels).Set(up)
	shared.BotLastPollTimestampSeconds.With(labels).Set(float64(time.Now().Unix()))

	if botStatus == nil {
		return
	}
	state := 0.0
	if botStatus.State == "running" {
		state = 1
	}
	shared.BotState.With(labels).Set(state)
	if botStatus.OpenTrades != nil {
		shared.BotOpenTrades.With(labels).Set(float64(*botStatus.OpenTrades))
	}
	if botStatus.MaxOpenTrades != nil {
		shared.BotMaxOpenTrades.With(labels).Set(float64(*botStatus.MaxOpenTrades))
	}
	if v, err := strconv.ParseFloat(botStatus.TotalProfitAbs, 64); err == nil {
		shared.BotProfitAbs.With(labels).Set(v)
	}
	if v, err := strconv.ParseFloat(botStatus.TotalProfitPct, 64); err == nil {
		shared.BotProfitRatio.With(labels).Set(v / 100)
	}
	if balance != nil {
		shared.BotBalance.With(labels).Set(*balance)
	}
}

func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// resolveCredentials reads back the same Basic Auth credentials
// configbuilder already renders into this bot's config.json, applying the
// identical secretRef-overrides-plaintext precedence (P3-1) so a bot
// configured either way is still reachable. Also returns the exchange name
// (from the same TradeBotConfig fetch), a metric label the poller has no
// other cheap source for. A package-level function, not a *BotPoller
// method (P4-4): shared verbatim by the poller and the TradeBot
// reconciler's state reconciliation, which talks to a bot exactly the
// same way but isn't a BotPoller.
func resolveCredentials(
	ctx context.Context, c client.Client, tradeBot *freqtradev1alpha1.TradeBot,
) (username, password, exchange string, err error) {
	var tradeBotConfig freqtradev1alpha1.TradeBotConfig
	key := types.NamespacedName{Name: tradeBot.Spec.Config, Namespace: tradeBot.Namespace}
	if err := c.Get(ctx, key, &tradeBotConfig); err != nil {
		return "", "", "", fmt.Errorf("failed to get TradeBotConfig %s: %w", tradeBot.Spec.Config, err)
	}
	if tradeBotConfig.Spec.Exchange != nil {
		exchange = tradeBotConfig.Spec.Exchange.Name
	}
	if tradeBotConfig.Spec.APIServer == nil {
		return "", "", exchange, fmt.Errorf("TradeBotConfig %s has no spec.apiServer", tradeBot.Spec.Config)
	}

	username = tradeBotConfig.Spec.APIServer.Username
	password = tradeBotConfig.Spec.APIServer.Password
	if tradeBotConfig.Spec.APIServer.SecretRef != "" {
		secretData, err := configbuilder.GetSecretData(
			ctx, c, tradeBot.Namespace, tradeBotConfig.Spec.APIServer.SecretRef,
		)
		if err != nil {
			return "", "", exchange, fmt.Errorf("failed to get API credentials secret: %w", err)
		}
		if v := secretData["user"]; len(v) > 0 {
			username = string(v)
		}
		if v := secretData["password"]; len(v) > 0 {
			password = string(v)
		}
	}
	return username, password, exchange, nil
}

// classifyError maps a botclient error to a BotReachable=False reason.
// Never touches the error's payload - only its type/status code - so this
// can't be the thing that ends up logging a response body (P4-3's own
// "never log response bodies" constraint).
func classifyError(err error) string {
	var statusErr *botclient.StatusError
	if errors.As(err, &statusErr) && statusErr.StatusCode == http.StatusUnauthorized {
		return freqtradev1alpha1.ReasonAuthFailed
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return freqtradev1alpha1.ReasonTimeout
	}
	var timeoutErr interface{ Timeout() bool }
	if errors.As(err, &timeoutErr) && timeoutErr.Timeout() {
		return freqtradev1alpha1.ReasonTimeout
	}
	return freqtradev1alpha1.ReasonConnectionRefused
}

func unreachableCondition(tradeBot *freqtradev1alpha1.TradeBot, reason string, err error) metav1.Condition {
	return metav1.Condition{
		Type: freqtradev1alpha1.ConditionBotReachable, Status: metav1.ConditionFalse,
		Reason: reason, Message: err.Error(), ObservedGeneration: tradeBot.Generation,
	}
}
