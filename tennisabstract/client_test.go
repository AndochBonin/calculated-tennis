package tennisabstract

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestGetPlayerStatsCacheHit(t *testing.T) {
	t.Parallel()

	fixture, err := os.ReadFile("testdata/player_medvedev.html")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var fetchCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetchCount.Add(1)
		servePlayerFragJS(t, w, fixture)
	}))
	t.Cleanup(srv.Close)

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	c := NewClient(
		WithBaseURL(srv.URL),
		WithCache(NewRedisCache(client)),
		WithCacheTTL(time.Hour),
	)

	ctx := context.Background()
	stats1, err := c.GetPlayerStats(ctx, "Daniil Medvedev")
	if err != nil {
		t.Fatalf("first GetPlayerStats: %v", err)
	}
	if fetchCount.Load() != 1 {
		t.Fatalf("fetch count after first call = %d, want 1", fetchCount.Load())
	}
	if stats1.FetchedAt.IsZero() {
		t.Fatal("FetchedAt is zero on first call")
	}

	stats2, err := c.GetPlayerStats(ctx, "DaniilMedvedev")
	if err != nil {
		t.Fatalf("second GetPlayerStats: %v", err)
	}
	if fetchCount.Load() != 1 {
		t.Fatalf("fetch count after cache hit = %d, want 1", fetchCount.Load())
	}
	if stats2.PlayerSlug != stats1.PlayerSlug {
		t.Fatalf("PlayerSlug = %q, want %q", stats2.PlayerSlug, stats1.PlayerSlug)
	}
	if !stats2.FetchedAt.Equal(stats1.FetchedAt) {
		t.Fatalf("FetchedAt = %v, want cached %v", stats2.FetchedAt, stats1.FetchedAt)
	}
	if len(stats2.RecentResults) != len(stats1.RecentResults) {
		t.Fatalf("RecentResults len = %d, want %d", len(stats2.RecentResults), len(stats1.RecentResults))
	}
}

func TestGetPlayerStatsSuccess(t *testing.T) {
	t.Parallel()

	fixture, err := os.ReadFile("testdata/player_medvedev.html")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != playerFragJSPath("DaniilMedvedev") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		gotUA = r.Header.Get("User-Agent")
		servePlayerFragJS(t, w, fixture)
	}))
	t.Cleanup(srv.Close)

	c := NewClient(WithBaseURL(srv.URL))
	stats, err := c.GetPlayerStats(context.Background(), "Daniil Medvedev")
	if err != nil {
		t.Fatalf("GetPlayerStats: %v", err)
	}
	if gotUA != userAgent {
		t.Fatalf("User-Agent = %q, want %q", gotUA, userAgent)
	}
	if stats.PlayerSlug != "DaniilMedvedev" {
		t.Fatalf("PlayerSlug = %q, want DaniilMedvedev", stats.PlayerSlug)
	}
	if len(stats.RecentResults) != 22 {
		t.Fatalf("RecentResults len = %d, want 22", len(stats.RecentResults))
	}
	if stats.FetchedAt.IsZero() {
		t.Fatal("FetchedAt is zero")
	}
}

func TestGetPlayerStatsEmptyName(t *testing.T) {
	t.Parallel()

	c := NewClient()
	_, err := c.GetPlayerStats(context.Background(), "  ")
	if err == nil || !strings.Contains(err.Error(), "empty player name") {
		t.Fatalf("expected empty name error, got: %v", err)
	}
}

func TestGetPlayerStatsNon200(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	c := NewClient(WithBaseURL(srv.URL))
	_, err := c.GetPlayerStats(context.Background(), "DaniilMedvedev")
	if err == nil || !strings.Contains(err.Error(), "unexpected status 503") {
		t.Fatalf("expected non-200 error, got: %v", err)
	}
}

func TestGetPlayerStatsContextCanceled(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)

	c := NewClient(WithBaseURL(srv.URL))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.GetPlayerStats(ctx, "DaniilMedvedev")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
}

func TestNewClientDefaultTimeout(t *testing.T) {
	t.Parallel()

	c := NewClient()
	if c.http.Timeout != defaultHTTPTimeout {
		t.Fatalf("default timeout = %v, want %v", c.http.Timeout, defaultHTTPTimeout)
	}
	if c.baseURL != defaultBaseURL {
		t.Fatalf("baseURL = %q, want %q", c.baseURL, defaultBaseURL)
	}
}

func servePlayerFragJS(t *testing.T, w http.ResponseWriter, html []byte) {
	t.Helper()
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(playerFragAssign))
	_, _ = w.Write(html)
	_, _ = w.Write([]byte(playerFragEnd))
}

func TestWithHTTPClientOverridesTimeout(t *testing.T) {
	t.Parallel()

	custom := &http.Client{Timeout: 5 * time.Second}
	c := NewClient(WithHTTPClient(custom))
	if c.http != custom {
		t.Fatal("expected custom http client")
	}
}
