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

	"github.com/AndochBonin/calculated-tennis/tennis/models"
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

func TestGetCareerMatchesCacheHit(t *testing.T) {
	t.Parallel()

	var fetchCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetchCount.Add(1)
		switch r.URL.Path {
		case "/cgi-bin/player-classic.cgi":
			_, _ = w.Write(medvedevClassicSnippet(t))
		case "/jsmatches/DaniilMedvedevCareer.js":
			_, _ = w.Write(medvedevCareerJSSnippet(t))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	cacheDir := t.TempDir()

	c := NewClient(
		WithBaseURL(srv.URL),
		WithCareerCacheDir(cacheDir),
	)

	ctx := context.Background()
	career1, err := c.GetCareerMatches(ctx, "Daniil Medvedev")
	if err != nil {
		t.Fatalf("first GetCareerMatches: %v", err)
	}
	if fetchCount.Load() != 2 {
		t.Fatalf("fetch count after first call = %d, want 2 (classic + career js)", fetchCount.Load())
	}
	if career1.FetchedAt.IsZero() {
		t.Fatal("FetchedAt is zero on first call")
	}
	if len(career1.Matches) != 7 {
		t.Fatalf("Matches len = %d, want 7 (5 matchmx + 2 morematchmx)", len(career1.Matches))
	}
	if _, ok, err := ReadCareerMatchesFile(cacheDir, career1.PlayerSlug); err != nil || !ok {
		t.Fatalf("career file on disk after first fetch: ok=%v err=%v", ok, err)
	}

	career2, err := c.GetCareerMatches(ctx, "DaniilMedvedev")
	if err != nil {
		t.Fatalf("second GetCareerMatches: %v", err)
	}
	if fetchCount.Load() != 2 {
		t.Fatalf("fetch count after cache hit = %d, want 2", fetchCount.Load())
	}
	if career2.PlayerSlug != career1.PlayerSlug {
		t.Fatalf("PlayerSlug = %q, want %q", career2.PlayerSlug, career1.PlayerSlug)
	}
	if !career2.FetchedAt.Equal(career1.FetchedAt) {
		t.Fatalf("FetchedAt = %v, want cached %v", career2.FetchedAt, career1.FetchedAt)
	}
}

func TestGetRecentResultsAsOf(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/player-classic.cgi":
			_, _ = w.Write(medvedevClassicSnippet(t))
		case "/jsmatches/DaniilMedvedevCareer.js":
			_, _ = w.Write(medvedevCareerJSSnippet(t))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	c := NewClient(WithBaseURL(srv.URL))
	ctx := context.Background()

	asOf := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	recent, err := c.GetRecentResultsAsOf(ctx, "DaniilMedvedev", asOf, 15)
	if err != nil {
		t.Fatalf("GetRecentResultsAsOf: %v", err)
	}
	if len(recent) != 5 {
		t.Fatalf("recent len = %d, want 5 (excludes all 2026-05-06 Rome rows)", len(recent))
	}
	if recent[0].Tournament != "Madrid Masters" || recent[0].Round != "R32" {
		t.Fatalf("newest = %q %q, want Madrid Masters R32", recent[0].Tournament, recent[0].Round)
	}

	all, err := c.GetRecentResultsAsOf(ctx, "DaniilMedvedev", time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC), 15)
	if err != nil {
		t.Fatalf("GetRecentResultsAsOf later date: %v", err)
	}
	if len(all) != 7 {
		t.Fatalf("recent len = %d, want 7 merged rows", len(all))
	}
	if all[0].Tournament != "Rome Masters" {
		t.Fatalf("newest Tournament = %q, want Rome Masters", all[0].Tournament)
	}
}

func TestGetCareerMatchesEmptyName(t *testing.T) {
	t.Parallel()

	c := NewClient()
	_, err := c.GetCareerMatches(context.Background(), "  ")
	if err == nil || !strings.Contains(err.Error(), "empty player name") {
		t.Fatalf("expected empty name error, got: %v", err)
	}
}

func TestGetCareerMatchesNoCareerJS(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/player-classic.cgi":
			_, _ = w.Write(medvedevClassicSnippet(t))
		case "/jsmatches/DaniilMedvedevCareer.js":
			http.NotFound(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	c := NewClient(WithBaseURL(srv.URL))
	career, err := c.GetCareerMatches(context.Background(), "DaniilMedvedev")
	if err != nil {
		t.Fatalf("GetCareerMatches: %v", err)
	}
	if len(career.Matches) != 5 {
		t.Fatalf("Matches len = %d, want 5 (matchmx only)", len(career.Matches))
	}
	if career.PlayerSlug != "DaniilMedvedev" {
		t.Fatalf("PlayerSlug = %q", career.PlayerSlug)
	}
}

func TestGetRecentResultsAsOfUsesCache(t *testing.T) {
	t.Parallel()

	var fetchCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetchCount.Add(1)
		switch r.URL.Path {
		case "/cgi-bin/player-classic.cgi":
			_, _ = w.Write(medvedevClassicSnippet(t))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	cacheDir := t.TempDir()
	preload := models.CareerMatches{
		PlayerSlug: "DaniilMedvedev",
		Matches: []models.RecentResult{
			{Date: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), Tournament: "Cached"},
		},
		FetchedAt: time.Now().UTC(),
	}
	if err := WriteCareerMatchesFile(cacheDir, "DaniilMedvedev", preload); err != nil {
		t.Fatalf("seed career file: %v", err)
	}

	c := NewClient(
		WithBaseURL(srv.URL),
		WithCareerCacheDir(cacheDir),
	)

	ctx := context.Background()

	recent, err := c.GetRecentResultsAsOf(ctx, "DaniilMedvedev", time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC), 15)
	if err != nil {
		t.Fatalf("GetRecentResultsAsOf: %v", err)
	}
	if fetchCount.Load() != 0 {
		t.Fatalf("fetch count = %d, want 0 (cache hit)", fetchCount.Load())
	}
	if len(recent) != 1 || recent[0].Tournament != "Cached" {
		t.Fatalf("recent = %#v, want single Cached row", recent)
	}
}

func TestWithHTTPClientOverridesTimeout(t *testing.T) {
	t.Parallel()

	custom := &http.Client{Timeout: 5 * time.Second}
	c := NewClient(WithHTTPClient(custom))
	if c.http != custom {
		t.Fatal("expected custom http client")
	}
}
