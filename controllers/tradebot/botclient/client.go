// Package botclient is a minimal, read-only freqtrade REST API client
// (P4-3). It never runs inside the reconcile loop - see
// controllers/tradebot's BotPoller, the only caller - so a slow or hung bot
// can never stall a reconcile.
package botclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// requestTimeout bounds every single request. freqtrade's REST API is
// local-cluster traffic, not a public API with unpredictable latency - 5s
// is generous, not tight.
const requestTimeout = 5 * time.Second

// maxResponseBody caps how much of a response this client will ever read
// into memory, regardless of what Content-Length claims. A malicious or
// misbehaving bot returning an unbounded body must not be able to exhaust
// the operator's own memory - 1 MiB is far more than any of the endpoints
// this client calls legitimately returns.
const maxResponseBody = 1 << 20

// Client is a freqtrade REST API client scoped to one bot. Read-only for
// now (P4-3 is poll-only, per D2/D9) - but every typed method goes through
// do(), a general method+path+body primitive, specifically so P4-4
// (start/stop) can add POST calls without reworking this type (D9).
type Client struct {
	baseURL    string
	username   string
	password   string
	httpClient *http.Client
}

// New returns a Client for a bot's REST API at baseURL (e.g.
// "http://my-bot.trading.svc.cluster.local:8080"). username/password are
// sent as HTTP Basic auth on every request; leave both empty for Ping, the
// one endpoint freqtrade never requires auth for.
func New(baseURL, username, password string) *Client {
	return &Client{
		baseURL:  baseURL,
		username: username,
		password: password,
		httpClient: &http.Client{
			Timeout: requestTimeout,
			// freqtrade's API never legitimately redirects; refusing to
			// follow one (rather than, say, silently sending Basic Auth
			// credentials to wherever it points) is the safe default.
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
			// Never honor HTTP_PROXY/HTTPS_PROXY/NO_PROXY for bot traffic -
			// this is in-cluster Service-to-Service, and an operator-wide
			// proxy env var (set for reaching the Kubernetes API, say) has
			// no business intercepting it.
			Transport: &http.Transport{Proxy: nil},
		},
	}
}

// StatusError is returned by do() when a request completed but with a
// non-2xx status - the caller (BotPoller) is what turns this into a
// BotReachable reason (401 -> AuthFailed, otherwise -> ConnectionRefused),
// since only it knows about that condition's vocabulary; this package
// deliberately doesn't import api/v1alpha1.
type StatusError struct {
	StatusCode int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("unexpected HTTP status %d", e.StatusCode)
}

// do is the general primitive every typed method below is built on (D9).
// It never logs or returns the raw response body - callers get it only via
// the unmarshaled struct their own typed method returns - because freqtrade
// responses carry balances and open positions (P4-3's own "never log
// response bodies" constraint starts here, not as a rule callers have to
// separately remember).
func (c *Client) do(ctx context.Context, method, path string, body []byte, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	if c.username != "" || c.password != "" {
		req.SetBasicAuth(c.username, c.password)
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody+1))
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}
	if len(data) > maxResponseBody {
		return fmt.Errorf("response body exceeds %d bytes", maxResponseBody)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &StatusError{StatusCode: resp.StatusCode}
	}

	if out == nil {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}
	return nil
}

// Ping calls the one endpoint freqtrade never requires auth for - suitable
// as a cheap first liveness check before attempting anything authenticated.
func (c *Client) Ping(ctx context.Context) error {
	return c.do(ctx, http.MethodGet, "/api/v1/ping", nil, nil)
}

// VersionResponse is /api/v1/version's response shape.
type VersionResponse struct {
	Version string `json:"version"`
}

func (c *Client) Version(ctx context.Context) (*VersionResponse, error) {
	var out VersionResponse
	if err := c.do(ctx, http.MethodGet, "/api/v1/version", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ShowConfigResponse is the subset of /api/v1/show_config's (much larger)
// response this client actually reads.
type ShowConfigResponse struct {
	State  string `json:"state"`
	DryRun bool   `json:"dry_run"`
}

func (c *Client) ShowConfig(ctx context.Context) (*ShowConfigResponse, error) {
	var out ShowConfigResponse
	if err := c.do(ctx, http.MethodGet, "/api/v1/show_config", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CountResponse is /api/v1/count's response shape.
type CountResponse struct {
	Current int `json:"current"`
	Max     int `json:"max"`
}

func (c *Client) Count(ctx context.Context) (*CountResponse, error) {
	var out CountResponse
	if err := c.do(ctx, http.MethodGet, "/api/v1/count", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ProfitResponse is the subset of /api/v1/profit's response this client
// reads - the all-time (not just currently-open-trade) totals.
type ProfitResponse struct {
	ProfitAllCoin    float64 `json:"profit_all_coin"`
	ProfitAllPercent float64 `json:"profit_all_percent"`
}

func (c *Client) Profit(ctx context.Context) (*ProfitResponse, error) {
	var out ProfitResponse
	if err := c.do(ctx, http.MethodGet, "/api/v1/profit", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// BalanceResponse is the subset of /api/v1/balance's response this client
// reads - not one of the plan's originally-listed endpoints, but needed to
// produce freqtrade_bot_balance (from that same P4-3 bullet's metrics
// list), which no other endpoint here provides.
type BalanceResponse struct {
	Total float64 `json:"total"`
}

func (c *Client) Balance(ctx context.Context) (*BalanceResponse, error) {
	var out BalanceResponse
	if err := c.do(ctx, http.MethodGet, "/api/v1/balance", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
