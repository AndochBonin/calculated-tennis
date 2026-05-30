package tennisabstract

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestDoHTTP_retries429ThenOK(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n <= 2 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(srv.Close)

	c := NewClient(
		WithBaseURL(srv.URL),
		WithMinRequestInterval(0),
		WithHTTPMaxRetries(5),
		WithHTTPBackoff(time.Millisecond),
	)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/test", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	resp, err := c.doHTTP(context.Background(), req)
	if err != nil {
		t.Fatalf("doHTTP: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if calls.Load() != 3 {
		t.Fatalf("calls = %d, want 3 (two 429s then success)", calls.Load())
	}
}

func TestDoHTTP_429Exhausted(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	t.Cleanup(srv.Close)

	c := NewClient(
		WithBaseURL(srv.URL),
		WithMinRequestInterval(0),
		WithHTTPMaxRetries(2),
		WithHTTPBackoff(time.Millisecond),
	)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/test", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	_, err = c.doHTTP(context.Background(), req)
	if err == nil || !strings.Contains(err.Error(), "429") {
		t.Fatalf("expected 429 exhausted error, got %v", err)
	}
}

func TestWaitRequestInterval_spacing(t *testing.T) {
	httpRequestGate.Lock()
	lastHTTPRequest = time.Time{}
	httpRequestGate.Unlock()

	ctx := context.Background()
	start := time.Now()
	if err := waitRequestInterval(ctx, 50*time.Millisecond); err != nil {
		t.Fatalf("first wait: %v", err)
	}
	if err := waitRequestInterval(ctx, 50*time.Millisecond); err != nil {
		t.Fatalf("second wait: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed < 45*time.Millisecond {
		t.Fatalf("elapsed = %v, want at least ~50ms between two spaced requests", elapsed)
	}
}

func TestParseRetryAfter(t *testing.T) {
	t.Parallel()

	if d, ok := parseRetryAfter("30"); !ok || d != 30*time.Second {
		t.Fatalf("seconds: d=%v ok=%v", d, ok)
	}
	future := time.Now().Add(2 * time.Second).UTC().Format(http.TimeFormat)
	if d, ok := parseRetryAfter(future); !ok || d < time.Second {
		t.Fatalf("http date: d=%v ok=%v", d, ok)
	}
	if _, ok := parseRetryAfter("not-a-date"); ok {
		t.Fatal("expected invalid Retry-After")
	}
}

func TestRetryBackoff(t *testing.T) {
	t.Parallel()

	if got := retryBackoff(0, time.Second, time.Minute); got != time.Second {
		t.Fatalf("attempt 0 = %v", got)
	}
	if got := retryBackoff(2, time.Second, time.Minute); got != 4*time.Second {
		t.Fatalf("attempt 2 = %v", got)
	}
	if got := retryBackoff(10, time.Second, time.Minute); got != time.Minute {
		t.Fatalf("capped = %v", got)
	}
}
