package botclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClient_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "admin" || pass != "secret" {
			t.Errorf("expected Basic Auth admin:secret, got ok=%v user=%q pass=%q", ok, user, pass)
		}
		switch r.URL.Path {
		case "/api/v1/ping":
			_, _ = w.Write([]byte(`{"status":"pong"}`))
		case "/api/v1/version":
			_, _ = w.Write([]byte(`{"version":"2024.1"}`))
		case "/api/v1/show_config":
			_, _ = w.Write([]byte(`{"state":"running","dry_run":true}`))
		case "/api/v1/count":
			_, _ = w.Write([]byte(`{"current":2,"max":5}`))
		case "/api/v1/profit":
			_, _ = w.Write([]byte(`{"profit_all_coin":12.5,"profit_all_percent":3.2}`))
		case "/api/v1/balance":
			_, _ = w.Write([]byte(`{"total":1042.75}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := New(srv.URL, "admin", "secret")
	ctx := context.Background()

	if err := c.Ping(ctx); err != nil {
		t.Errorf("Ping: unexpected error: %v", err)
	}
	version, err := c.Version(ctx)
	if err != nil || version.Version != "2024.1" {
		t.Errorf("Version: got (%+v, %v)", version, err)
	}
	config, err := c.ShowConfig(ctx)
	if err != nil || config.State != "running" || !config.DryRun {
		t.Errorf("ShowConfig: got (%+v, %v)", config, err)
	}
	count, err := c.Count(ctx)
	if err != nil || count.Current != 2 || count.Max != 5 {
		t.Errorf("Count: got (%+v, %v)", count, err)
	}
	profit, err := c.Profit(ctx)
	if err != nil || profit.ProfitAllCoin != 12.5 || profit.ProfitAllPercent != 3.2 {
		t.Errorf("Profit: got (%+v, %v)", profit, err)
	}
	balance, err := c.Balance(ctx)
	if err != nil || balance.Total != 1042.75 {
		t.Errorf("Balance: got (%+v, %v)", balance, err)
	}
}

func TestClient_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error": "unauthorized"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "wrong", "creds")
	_, err := c.Version(context.Background())

	var statusErr *StatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("expected a *StatusError, got %v (%T)", err, err)
	}
	if statusErr.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", statusErr.StatusCode)
	}
}

func TestClient_Timeout(t *testing.T) {
	// The handler blocks until this closes - which must happen before
	// srv.Close() is called, or Close() (which waits for every in-flight
	// handler to return) deadlocks forever. t.Cleanup runs in LIFO order
	// like defer, so registering this first makes it the *last* cleanup to
	// run - the opposite of what's needed - hence closing it explicitly
	// right after the call below, ahead of any cleanup at all.
	unblockHandler := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-unblockHandler
	}))
	defer srv.Close()

	c := New(srv.URL, "", "")
	c.httpClient.Timeout = 50 * time.Millisecond

	_, err := c.Version(context.Background())
	close(unblockHandler)
	if err == nil {
		t.Fatal("expected a timeout error, got nil")
	}
	var netErr interface{ Timeout() bool }
	if !errors.As(err, &netErr) || !netErr.Timeout() {
		t.Errorf("expected a timeout-classified error, got %v (%T)", err, err)
	}
}

func TestClient_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"version": not valid json`))
	}))
	defer srv.Close()

	c := New(srv.URL, "", "")
	_, err := c.Version(context.Background())
	if err == nil {
		t.Fatal("expected a decode error, got nil")
	}
	if !strings.Contains(err.Error(), "decoding response") {
		t.Errorf("expected a decode error, got %v", err)
	}
}

func TestClient_OversizedBody(t *testing.T) {
	oversized := strings.Repeat("a", maxResponseBody+1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"version":"` + oversized + `"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "", "")
	_, err := c.Version(context.Background())
	if err == nil {
		t.Fatal("expected an oversized-body error, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("expected an oversized-body error, got %v", err)
	}
}

// TestClient_ErrorNeverIncludesResponseBody covers P4-3's "never log
// response bodies" constraint (balances and open positions live there) at
// the one place this package could accidentally leak one: a decode-failure
// error message. do() must never interpolate the raw body into an error.
func TestClient_ErrorNeverIncludesResponseBody(t *testing.T) {
	const secretLookingBody = `{"balance": 999999.42, "position": "not-json-shhh`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(secretLookingBody))
	}))
	defer srv.Close()

	c := New(srv.URL, "", "")
	_, err := c.Version(context.Background())
	if err == nil {
		t.Fatal("expected a decode error, got nil")
	}
	if strings.Contains(err.Error(), "999999.42") {
		t.Errorf("error message must never contain the raw response body, got %v", err)
	}
}
