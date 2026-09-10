package tradebot

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	"github.com/ark-sys/freqtrade-operator/controllers/tradebot/botclient"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// discardLogger is a no-op logr.Logger for tests that need to pass one but
// don't care about its output.
func discardLogger(t *testing.T) logr.Logger {
	t.Helper()
	return logr.Discard()
}

// stateRunning/stateStopped match BotStatus.State's enum values.
const (
	stateRunning = "running"
	stateStopped = "stopped"
)

// panicOnGetClient embeds client.Client so it satisfies the interface, but
// overrides Get to panic - the rest of the interface is never reached in
// this test, so a nil embedded value there is fine.
type panicOnGetClient struct{ client.Client }

func (panicOnGetClient) Get(context.Context, types.NamespacedName, client.Object, ...client.GetOption) error {
	panic("boom: simulated panic from a bot's poll attempt")
}

// TestPollOneSafely_RecoversPanic covers why pollOneSafely exists at all:
// a bug triggered by one bot's response must not crash the goroutine (and,
// unrecovered, the whole operator process) polling every other bot.
func TestPollOneSafely_RecoversPanic(t *testing.T) {
	p := &BotPoller{Client: panicOnGetClient{}, state: map[types.NamespacedName]*pollState{}}
	key := types.NamespacedName{Name: "my-bot", Namespace: "trading"}

	done := make(chan struct{})
	go func() {
		defer close(done)
		p.pollOneSafely(context.Background(), discardLogger(t), key)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("pollOneSafely did not return - the panic was not recovered")
	}
}

func TestShouldPoll(t *testing.T) {
	tests := []struct {
		name string
		spec freqtradev1alpha1.TradeBotSpec
		want bool
	}{
		{name: "trade mode, no introspection field at all", spec: freqtradev1alpha1.TradeBotSpec{}, want: true},
		{
			name: "trade mode, introspection enabled explicitly",
			spec: freqtradev1alpha1.TradeBotSpec{Introspection: &freqtradev1alpha1.IntrospectionSpec{Enabled: ptr.To(true)}},
			want: true,
		},
		{
			name: "trade mode, introspection explicitly disabled",
			spec: freqtradev1alpha1.TradeBotSpec{Introspection: &freqtradev1alpha1.IntrospectionSpec{Enabled: ptr.To(false)}},
			want: false,
		},
		{
			name: "backtesting mode is never polled",
			spec: freqtradev1alpha1.TradeBotSpec{FreqtradeCommand: "backtesting"},
			want: false,
		},
		{
			name: "hyperopt mode is never polled",
			spec: freqtradev1alpha1.TradeBotSpec{FreqtradeCommand: "hyperopt"},
			want: false,
		},
		{
			name: "explicit trade mode still polls",
			spec: freqtradev1alpha1.TradeBotSpec{FreqtradeCommand: "trade"},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tradeBot := &freqtradev1alpha1.TradeBot{Spec: tt.spec}
			if got := shouldPoll(tradeBot); got != tt.want {
				t.Errorf("shouldPoll() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPollInterval(t *testing.T) {
	noIntrospection := &freqtradev1alpha1.TradeBot{}
	if got := pollInterval(noIntrospection); got != 60*time.Second {
		t.Errorf("expected the 60s default with no introspection spec, got %v", got)
	}

	explicit := &freqtradev1alpha1.TradeBot{
		Spec: freqtradev1alpha1.TradeBotSpec{
			Introspection: &freqtradev1alpha1.IntrospectionSpec{Interval: metav1.Duration{Duration: 30 * time.Second}},
		},
	}
	if got := pollInterval(explicit); got != 30*time.Second {
		t.Errorf("expected the explicit 30s interval, got %v", got)
	}
}

func TestClassifyError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "401 is AuthFailed",
			err:  &botclient.StatusError{StatusCode: http.StatusUnauthorized},
			want: freqtradev1alpha1.ReasonAuthFailed,
		},
		{
			name: "403 is not AuthFailed",
			err:  &botclient.StatusError{StatusCode: http.StatusForbidden},
			want: freqtradev1alpha1.ReasonConnectionRefused,
		},
		{name: "context.DeadlineExceeded is Timeout", err: context.DeadlineExceeded, want: freqtradev1alpha1.ReasonTimeout},
		{
			name: "anything else is ConnectionRefused",
			err:  errors.New("dial tcp: connection refused"),
			want: freqtradev1alpha1.ReasonConnectionRefused,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyError(tt.err); got != tt.want {
				t.Errorf("classifyError() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMergeBotStatus(t *testing.T) {
	now := metav1.Now()

	t.Run("a successful poll fully replaces status.bot", func(t *testing.T) {
		existing := &freqtradev1alpha1.BotStatus{State: "stopped", LastPollError: "previous failure"}
		fresh := &freqtradev1alpha1.BotStatus{State: stateRunning, OpenTrades: ptr.To(3)}
		reachable := metav1.Condition{Status: metav1.ConditionTrue}

		got := mergeBotStatus(existing, fresh, reachable, now)

		if got.State != stateRunning || got.OpenTrades == nil || *got.OpenTrades != 3 {
			t.Errorf("expected the fresh status, got %+v", got)
		}
		if got.LastPollError != "" {
			t.Errorf("expected LastPollError cleared on a successful poll, got %q", got.LastPollError)
		}
		if got.LastPollTime == nil || !got.LastPollTime.Equal(&now) {
			t.Errorf("expected LastPollTime stamped to now, got %v", got.LastPollTime)
		}
	})

	t.Run("a failed poll preserves the last known-good state", func(t *testing.T) {
		existing := &freqtradev1alpha1.BotStatus{State: stateRunning, OpenTrades: ptr.To(2)}
		reachable := metav1.Condition{Status: metav1.ConditionFalse, Message: "connection refused"}

		got := mergeBotStatus(existing, nil, reachable, now)

		if got.State != stateRunning || got.OpenTrades == nil || *got.OpenTrades != 2 {
			t.Errorf("expected the previous state preserved, got %+v", got)
		}
		if got.LastPollError != "connection refused" {
			t.Errorf("expected LastPollError set to the failure message, got %q", got.LastPollError)
		}
		if got.LastPollTime == nil || !got.LastPollTime.Equal(&now) {
			t.Errorf("expected LastPollTime still stamped even on failure, got %v", got.LastPollTime)
		}
	})

	t.Run("a failed poll with no prior state starts an empty one, not nil", func(t *testing.T) {
		reachable := metav1.Condition{Status: metav1.ConditionFalse, Message: "timeout"}

		got := mergeBotStatus(nil, nil, reachable, now)

		if got == nil {
			t.Fatal("expected a non-nil BotStatus even for a bot never successfully polled")
		}
		if got.LastPollError != "timeout" {
			t.Errorf("expected LastPollError %q, got %q", "timeout", got.LastPollError)
		}
	})
}

func TestUpdateBackoff(t *testing.T) {
	key := types.NamespacedName{Name: "my-bot", Namespace: "trading"}
	tradeBot := &freqtradev1alpha1.TradeBot{
		ObjectMeta: metav1.ObjectMeta{Name: "my-bot", Namespace: "trading"},
		Spec: freqtradev1alpha1.TradeBotSpec{
			Introspection: &freqtradev1alpha1.IntrospectionSpec{Interval: metav1.Duration{Duration: 10 * time.Second}},
		},
	}

	p := &BotPoller{state: map[types.NamespacedName]*pollState{key: {}}}

	success := metav1.Condition{Status: metav1.ConditionTrue}
	p.updateBackoff(key, tradeBot, success)
	st := p.state[key]
	if st.consecutiveFailures != 0 {
		t.Errorf("expected consecutiveFailures reset to 0 on success, got %d", st.consecutiveFailures)
	}
	wantNext := time.Now().Add(10 * time.Second)
	if st.nextPollAt.Before(wantNext.Add(-time.Second)) || st.nextPollAt.After(wantNext.Add(time.Second)) {
		t.Errorf("expected nextPollAt ~10s from now on success, got %v (now=%v)", st.nextPollAt, time.Now())
	}

	// Growth is checked coarsely (a handful of tolerance-bounded checkpoints,
	// not "strictly larger every single attempt") because once backoff
	// plateaus at the cap, two consecutive nextPollAt computations can
	// legitimately differ by a few hundred nanoseconds either way purely
	// from when time.Now() was sampled - real jitter, not a regression.
	failure := metav1.Condition{Status: metav1.ConditionFalse}
	const tolerance = 50 * time.Millisecond
	backoffAfter := func(attempts int) time.Duration {
		var backoff time.Duration
		for i := 0; i < attempts; i++ {
			before := time.Now()
			p.updateBackoff(key, tradeBot, failure)
			backoff = p.state[key].nextPollAt.Sub(before)
		}
		return backoff
	}

	first := backoffAfter(1)
	if first < 20*time.Second-tolerance || first > 20*time.Second+tolerance {
		t.Errorf("expected the 1st failure to back off to ~2x the interval (20s), got %v", first)
	}

	third := backoffAfter(2) // consecutiveFailures now at 3
	if third <= first {
		t.Errorf("expected backoff to have grown by the 3rd consecutive failure, got %v (was %v)", third, first)
	}

	final := backoffAfter(5) // consecutiveFailures now at 8, well past where the cap takes over
	if st := p.state[key]; st.consecutiveFailures != 8 {
		t.Errorf("expected consecutiveFailures=8 after 8 failures, got %d", st.consecutiveFailures)
	}
	maxAllowed := 10 * time.Second * maxBackoffMultiple
	if final < maxAllowed-tolerance || final > maxAllowed+tolerance {
		t.Errorf("expected backoff capped at ~%v (10x the 10s interval), got %v", maxAllowed, final)
	}
}

func TestPollWithClient_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/ping":
			_, _ = w.Write([]byte(`{"status":"pong"}`))
		case "/api/v1/show_config":
			_, _ = w.Write([]byte(`{"state":"running","dry_run":true}`))
		case "/api/v1/version":
			_, _ = w.Write([]byte(`{"version":"2024.1"}`))
		case "/api/v1/count":
			_, _ = w.Write([]byte(`{"current":1,"max":3}`))
		case "/api/v1/profit":
			_, _ = w.Write([]byte(`{"profit_all_coin":5.5,"profit_all_percent":1.1}`))
		case "/api/v1/balance":
			_, _ = w.Write([]byte(`{"total":100}`))
		}
	}))
	defer srv.Close()

	tradeBot := &freqtradev1alpha1.TradeBot{
		ObjectMeta: metav1.ObjectMeta{Name: "my-bot", Namespace: "trading", Generation: 3},
	}
	c := botclient.New(srv.URL, "", "")

	status, reachable, balance := pollWithClient(context.Background(), c, tradeBot)

	if status == nil {
		t.Fatal("expected a non-nil BotStatus")
	}
	if status.State != stateRunning || status.DryRun == nil || !*status.DryRun {
		t.Errorf("expected state=running dryRun=true, got %+v", status)
	}
	if status.OpenTrades == nil || *status.OpenTrades != 1 || status.MaxOpenTrades == nil || *status.MaxOpenTrades != 3 {
		t.Errorf("expected openTrades=1 maxOpenTrades=3, got %+v", status)
	}
	if status.TotalProfitAbs != "5.5" || status.TotalProfitPct != "1.1" {
		t.Errorf("expected profit fields 5.5/1.1, got abs=%q pct=%q", status.TotalProfitAbs, status.TotalProfitPct)
	}
	if balance == nil || *balance != 100 {
		t.Errorf("expected balance 100, got %v", balance)
	}
	if reachable.Status != metav1.ConditionTrue || reachable.Reason != freqtradev1alpha1.ReasonAsExpected {
		t.Errorf("expected BotReachable=True/AsExpected, got %+v", reachable)
	}
	if reachable.ObservedGeneration != 3 {
		t.Errorf("expected ObservedGeneration 3, got %d", reachable.ObservedGeneration)
	}
	if status.LastPollError != "" {
		t.Errorf("expected no LastPollError when every endpoint succeeds, got %q", status.LastPollError)
	}
}

// TestPollWithClient_StoppedBotIsReachableWithBestEffortFieldsEmpty covers
// the actual freqtrade behavior this poller has to tolerate (verified
// directly against a real bot): a TradeBot with no explicit
// spec.advanced.initial_state defaults to state=stopped, and while stopped,
// /api/v1/count, /api/v1/profit, and /api/v1/balance all fail server-side
// ("trader is not running") even though /api/v1/ping, /api/v1/show_config,
// and /api/v1/version succeed fine. A stopped bot is reachable, just not
// trading - BotReachable must stay True, not read as unreachable.
func TestPollWithClient_StoppedBotIsReachableWithBestEffortFieldsEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/ping":
			_, _ = w.Write([]byte(`{"status":"pong"}`))
		case "/api/v1/show_config":
			_, _ = w.Write([]byte(`{"state":"stopped","dry_run":true}`))
		case "/api/v1/version":
			_, _ = w.Write([]byte(`{"version":"2024.1"}`))
		case "/api/v1/count", "/api/v1/profit", "/api/v1/balance":
			w.WriteHeader(http.StatusBadRequest) // freqtrade's "trader is not running" RPCException
		}
	}))
	defer srv.Close()

	tradeBot := &freqtradev1alpha1.TradeBot{ObjectMeta: metav1.ObjectMeta{Name: "my-bot", Namespace: "trading"}}
	c := botclient.New(srv.URL, "", "")

	status, reachable, balance := pollWithClient(context.Background(), c, tradeBot)

	if reachable.Status != metav1.ConditionTrue || reachable.Reason != freqtradev1alpha1.ReasonAsExpected {
		t.Errorf("expected BotReachable=True/AsExpected for a stopped-but-reachable bot, got %+v", reachable)
	}
	if status == nil {
		t.Fatal("expected a non-nil BotStatus for a stopped-but-reachable bot")
	}
	if status.State != stateStopped {
		t.Errorf("expected state=stopped, got %q", status.State)
	}
	if status.OpenTrades != nil || status.MaxOpenTrades != nil {
		t.Errorf("expected nil OpenTrades/MaxOpenTrades when count fails, got %+v", status)
	}
	if status.TotalProfitAbs != "" || status.TotalProfitPct != "" {
		t.Errorf("expected empty profit fields when profit fails, got %+v", status)
	}
	if balance != nil {
		t.Errorf("expected nil balance when the balance call fails, got %v", *balance)
	}
	for _, want := range []string{"count:", "profit:", "balance:"} {
		if !strings.Contains(status.LastPollError, want) {
			t.Errorf("expected LastPollError to mention %q, got %q", want, status.LastPollError)
		}
	}
}

// TestPollWithClient_PartialBestEffortFailureDoesNotAffectReachability
// covers the same tolerant handling on a bot that IS running: an
// unexpected failure of just one best-effort endpoint (as opposed to the
// bot being stopped) must not cost the other two, or BotReachable.
func TestPollWithClient_PartialBestEffortFailureDoesNotAffectReachability(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/ping":
			_, _ = w.Write([]byte(`{"status":"pong"}`))
		case "/api/v1/show_config":
			_, _ = w.Write([]byte(`{"state":"running","dry_run":true}`))
		case "/api/v1/version":
			_, _ = w.Write([]byte(`{"version":"2024.1"}`))
		case "/api/v1/count":
			_, _ = w.Write([]byte(`{"current":1,"max":3}`))
		case "/api/v1/profit":
			w.WriteHeader(http.StatusInternalServerError) // simulated one-off failure
		case "/api/v1/balance":
			_, _ = w.Write([]byte(`{"total":100}`))
		}
	}))
	defer srv.Close()

	tradeBot := &freqtradev1alpha1.TradeBot{ObjectMeta: metav1.ObjectMeta{Name: "my-bot", Namespace: "trading"}}
	c := botclient.New(srv.URL, "", "")

	status, reachable, balance := pollWithClient(context.Background(), c, tradeBot)

	if reachable.Status != metav1.ConditionTrue || reachable.Reason != freqtradev1alpha1.ReasonAsExpected {
		t.Errorf("expected BotReachable=True/AsExpected despite one best-effort failure, got %+v", reachable)
	}
	if status == nil {
		t.Fatal("expected a non-nil BotStatus")
	}
	if status.OpenTrades == nil || *status.OpenTrades != 1 || status.MaxOpenTrades == nil || *status.MaxOpenTrades != 3 {
		t.Errorf("expected count fields to still populate, got %+v", status)
	}
	if status.TotalProfitAbs != "" || status.TotalProfitPct != "" {
		t.Errorf("expected empty profit fields when profit fails, got %+v", status)
	}
	if balance == nil || *balance != 100 {
		t.Errorf("expected balance to still populate, got %v", balance)
	}
	if !strings.Contains(status.LastPollError, "profit:") {
		t.Errorf("expected LastPollError to mention the profit failure, got %q", status.LastPollError)
	}
	if strings.Contains(status.LastPollError, "count:") || strings.Contains(status.LastPollError, "balance:") {
		t.Errorf("expected LastPollError to mention only the failing endpoint, got %q", status.LastPollError)
	}
}

func TestPollWithClient_PingFailureReportsUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	tradeBot := &freqtradev1alpha1.TradeBot{ObjectMeta: metav1.ObjectMeta{Name: "my-bot", Namespace: "trading"}}
	c := botclient.New(srv.URL, "", "")

	status, reachable, _ := pollWithClient(context.Background(), c, tradeBot)

	if status != nil {
		t.Errorf("expected a nil BotStatus on failure, got %+v", status)
	}
	if reachable.Status != metav1.ConditionFalse {
		t.Errorf("expected BotReachable=False, got %v", reachable.Status)
	}
}

func TestPollWithClient_AuthFailureSetsAuthFailedReason(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/ping" {
			_, _ = w.Write([]byte(`{"status":"pong"}`))
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	tradeBot := &freqtradev1alpha1.TradeBot{ObjectMeta: metav1.ObjectMeta{Name: "my-bot", Namespace: "trading"}}
	c := botclient.New(srv.URL, "wrong", "creds")

	_, reachable, _ := pollWithClient(context.Background(), c, tradeBot)

	if reachable.Reason != freqtradev1alpha1.ReasonAuthFailed {
		t.Errorf("expected reason %q, got %q", freqtradev1alpha1.ReasonAuthFailed, reachable.Reason)
	}
}

func newPollerTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add corev1 to scheme: %v", err)
	}
	if err := freqtradev1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add v1alpha1 to scheme: %v", err)
	}
	return scheme
}

func TestResolveCredentials_SecretRefOverridesPlaintext(t *testing.T) {
	scheme := newPollerTestScheme(t)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "api-creds", Namespace: "trading"},
		Data:       map[string][]byte{"user": []byte("from-secret"), "password": []byte("secret-pass")},
	}
	tradeBotConfig := &freqtradev1alpha1.TradeBotConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "my-config", Namespace: "trading"},
		Spec: freqtradev1alpha1.TradeBotConfigSpec{
			Exchange: &freqtradev1alpha1.ExchangeSpec{Name: "binance"},
			APIServer: &freqtradev1alpha1.APIServerConfig{
				Username: "plaintext-user", Password: "plaintext-pass", SecretRef: "api-creds",
			},
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret, tradeBotConfig).Build()
	p := &BotPoller{Client: c}
	tradeBot := &freqtradev1alpha1.TradeBot{
		ObjectMeta: metav1.ObjectMeta{Name: "my-bot", Namespace: "trading"},
		Spec:       freqtradev1alpha1.TradeBotSpec{Config: "my-config"},
	}

	username, password, exchange, err := p.resolveCredentials(context.Background(), tradeBot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if username != "from-secret" || password != "secret-pass" {
		t.Errorf("expected the secretRef credentials to win, got user=%q pass=%q", username, password)
	}
	if exchange != "binance" {
		t.Errorf("expected exchange %q, got %q", "binance", exchange)
	}
}

func TestScheduleDue_SkipsIneligibleAndCleansUpDeleted(t *testing.T) {
	scheme := newPollerTestScheme(t)
	tradeMode := &freqtradev1alpha1.TradeBot{ObjectMeta: metav1.ObjectMeta{Name: "trade-bot", Namespace: "trading"}}
	jobMode := &freqtradev1alpha1.TradeBot{
		ObjectMeta: metav1.ObjectMeta{Name: "job-bot", Namespace: "trading"},
		Spec:       freqtradev1alpha1.TradeBotSpec{FreqtradeCommand: "backtesting"},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tradeMode, jobMode).Build()

	staleKey := types.NamespacedName{Name: "deleted-bot", Namespace: "trading"}
	p := &BotPoller{Client: c, state: map[types.NamespacedName]*pollState{
		staleKey: {strategy: "s", exchange: "binance", dryRun: "true"},
	}}

	queue := make(chan types.NamespacedName, 10)
	p.scheduleDue(context.Background(), discardLogger(t), queue)
	close(queue)

	enqueued := make([]types.NamespacedName, 0, len(queue))
	for key := range queue {
		enqueued = append(enqueued, key)
	}

	if len(enqueued) != 1 || enqueued[0].Name != "trade-bot" {
		t.Errorf("expected only trade-bot to be enqueued, got %v", enqueued)
	}
	if _, stillTracked := p.state[staleKey]; stillTracked {
		t.Error("expected a bot no longer in the List to be forgotten from state")
	}
	if _, tracked := p.state[types.NamespacedName{Name: "job-bot", Namespace: "trading"}]; tracked {
		t.Error("expected a Job-mode bot never to be tracked at all")
	}
}

func TestScheduleDue_InFlightBotIsNotReenqueued(t *testing.T) {
	scheme := newPollerTestScheme(t)
	tradeBot := &freqtradev1alpha1.TradeBot{ObjectMeta: metav1.ObjectMeta{Name: "my-bot", Namespace: "trading"}}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tradeBot).Build()

	key := types.NamespacedName{Name: "my-bot", Namespace: "trading"}
	p := &BotPoller{Client: c, state: map[types.NamespacedName]*pollState{
		key: {nextPollAt: time.Now().Add(-time.Minute), inFlight: true},
	}}

	queue := make(chan types.NamespacedName, 10)
	p.scheduleDue(context.Background(), discardLogger(t), queue)
	close(queue)

	for range queue {
		t.Error("expected an in-flight bot not to be enqueued again")
	}
}

func TestScheduleDue_SetsIntrospectionDisabledCondition(t *testing.T) {
	scheme := newPollerTestScheme(t)
	tradeBot := &freqtradev1alpha1.TradeBot{
		ObjectMeta: metav1.ObjectMeta{Name: "disabled-bot", Namespace: "trading"},
		Spec: freqtradev1alpha1.TradeBotSpec{
			Introspection: &freqtradev1alpha1.IntrospectionSpec{Enabled: ptr.To(false)},
		},
		// A stale True condition from before introspection was turned off -
		// this is exactly the case the new logic must not leave lingering.
		Status: freqtradev1alpha1.TradeBotStatus{Conditions: []metav1.Condition{{
			Type: freqtradev1alpha1.ConditionBotReachable, Status: metav1.ConditionTrue,
			Reason: freqtradev1alpha1.ReasonAsExpected, LastTransitionTime: metav1.Now(),
		}}},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tradeBot).WithStatusSubresource(tradeBot).Build()
	p := &BotPoller{Client: c, state: map[types.NamespacedName]*pollState{}}

	queue := make(chan types.NamespacedName, 10)
	p.scheduleDue(context.Background(), discardLogger(t), queue)
	close(queue)

	for range queue {
		t.Error("expected a bot with introspection disabled not to be enqueued")
	}

	key := types.NamespacedName{Name: "disabled-bot", Namespace: "trading"}
	if _, tracked := p.state[key]; tracked {
		t.Error("expected a bot with introspection disabled never to be tracked in poll state")
	}

	var got freqtradev1alpha1.TradeBot
	if err := c.Get(context.Background(), key, &got); err != nil {
		t.Fatalf("Get: %v", err)
	}
	cond := meta.FindStatusCondition(got.Status.Conditions, freqtradev1alpha1.ConditionBotReachable)
	if cond == nil {
		t.Fatal("expected a BotReachable condition to be set")
	}
	if cond.Status != metav1.ConditionUnknown {
		t.Errorf("expected BotReachable=Unknown, got %v", cond.Status)
	}
	if cond.Reason != freqtradev1alpha1.ReasonIntrospectionDisabled {
		t.Errorf("expected reason %q, got %q", freqtradev1alpha1.ReasonIntrospectionDisabled, cond.Reason)
	}
}

func TestHasIntrospectionDisabledCondition(t *testing.T) {
	tests := []struct {
		name     string
		tradeBot *freqtradev1alpha1.TradeBot
		want     bool
	}{
		{name: "no condition at all", tradeBot: &freqtradev1alpha1.TradeBot{}, want: false},
		{
			name: "condition present but wrong reason",
			tradeBot: &freqtradev1alpha1.TradeBot{Status: freqtradev1alpha1.TradeBotStatus{
				Conditions: []metav1.Condition{{
					Type: freqtradev1alpha1.ConditionBotReachable, Status: metav1.ConditionTrue,
					Reason: freqtradev1alpha1.ReasonAsExpected,
				}},
			}},
			want: false,
		},
		{
			name: "right reason but stale ObservedGeneration",
			tradeBot: &freqtradev1alpha1.TradeBot{
				ObjectMeta: metav1.ObjectMeta{Generation: 2},
				Status: freqtradev1alpha1.TradeBotStatus{
					Conditions: []metav1.Condition{{
						Type: freqtradev1alpha1.ConditionBotReachable, Status: metav1.ConditionUnknown,
						Reason: freqtradev1alpha1.ReasonIntrospectionDisabled, ObservedGeneration: 1,
					}},
				},
			},
			want: false,
		},
		{
			name: "right reason and current generation",
			tradeBot: &freqtradev1alpha1.TradeBot{
				ObjectMeta: metav1.ObjectMeta{Generation: 2},
				Status: freqtradev1alpha1.TradeBotStatus{
					Conditions: []metav1.Condition{{
						Type: freqtradev1alpha1.ConditionBotReachable, Status: metav1.ConditionUnknown,
						Reason: freqtradev1alpha1.ReasonIntrospectionDisabled, ObservedGeneration: 2,
					}},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasIntrospectionDisabledCondition(tt.tradeBot); got != tt.want {
				t.Errorf("hasIntrospectionDisabledCondition() = %v, want %v", got, tt.want)
			}
		})
	}
}
