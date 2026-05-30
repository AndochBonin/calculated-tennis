package tennisabstract

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/AndochBonin/E3/tennis/models"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

type errCache struct {
	getErr error
	setErr error
}

func (e errCache) Get(context.Context, string) ([]byte, bool, error) {
	return nil, false, e.getErr
}

func (e errCache) Set(context.Context, string, []byte, time.Duration) error {
	return e.setErr
}

func TestNewRedisCache_nilClient(t *testing.T) {
	t.Parallel()
	if got := NewRedisCache(nil); got != nil {
		t.Fatalf("NewRedisCache(nil) = %v, want nil", got)
	}
}

func TestNewRedisCacheFromEnv(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	t.Setenv(redisURLEnv, "redis://"+mr.Addr())
	t.Setenv(redisAddrEnv, "")

	cache, err := NewRedisCacheFromEnv()
	if err != nil {
		t.Fatalf("NewRedisCacheFromEnv: %v", err)
	}
	if cache == nil || cache.client == nil {
		t.Fatal("expected non-nil cache")
	}
}

func TestNewRedisCacheFromEnv_clientError(t *testing.T) {
	t.Setenv(redisURLEnv, "://bad")
	t.Setenv(redisAddrEnv, "")
	_, err := NewRedisCacheFromEnv()
	if err == nil {
		t.Fatal("expected error from bad redis URL")
	}
}

func TestRedisCache_nilReceiver(t *testing.T) {
	t.Parallel()
	var cache *RedisCache
	ctx := context.Background()
	val, ok, err := cache.Get(ctx, "k")
	if err != nil || ok || val != nil {
		t.Fatalf("nil Get: val=%v ok=%v err=%v", val, ok, err)
	}
	if err := cache.Set(ctx, "k", []byte("x"), time.Hour); err != nil {
		t.Fatalf("nil Set: %v", err)
	}
}

func TestRedisCacheGet_connectionError(t *testing.T) {
	t.Parallel()

	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	t.Cleanup(func() { _ = client.Close() })

	cache := NewRedisCache(client)
	_, ok, err := cache.Get(context.Background(), "k")
	if err == nil {
		t.Fatal("expected connection error")
	}
	if ok {
		t.Fatal("ok should be false on error")
	}
}

func TestSetCachedPlayerStats_marshalError(t *testing.T) {
	old := jsonMarshal
	t.Cleanup(func() { jsonMarshal = old })
	jsonMarshal = func(any) ([]byte, error) {
		return nil, errors.New("marshal failed")
	}

	err := SetCachedPlayerStats(context.Background(), errCache{}, "slug", models.PlayerStats{}, time.Hour)
	if err == nil || !strings.Contains(err.Error(), "marshal player stats") {
		t.Fatalf("expected marshal error, got %v", err)
	}
}

func TestSetCachedPlayerStats_setError(t *testing.T) {
	t.Parallel()

	err := SetCachedPlayerStats(
		context.Background(),
		errCache{setErr: errors.New("set failed")},
		"slug",
		models.PlayerStats{PlayerSlug: "slug"},
		time.Hour,
	)
	if err == nil {
		t.Fatal("expected set error")
	}
}

func TestGetCachedPlayerStats_cacheGetError(t *testing.T) {
	t.Parallel()

	_, ok, err := GetCachedPlayerStats(context.Background(), errCache{getErr: errors.New("get failed")}, "x")
	if err == nil || ok {
		t.Fatalf("expected get error, ok=%v err=%v", ok, err)
	}
}

func TestWithTransport(t *testing.T) {
	t.Parallel()

	var called bool
	c := NewClient(WithTransport(roundTripperFunc(func(*http.Request) (*http.Response, error) {
		called = true
		return nil, errors.New("stop")
	})))
	if c.http.Transport == nil {
		t.Fatal("expected transport to be set")
	}
	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	_, _ = c.http.Transport.RoundTrip(req)
	if !called {
		t.Fatal("expected custom transport to be invoked")
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestGetPlayerStats_cacheGetError(t *testing.T) {
	t.Parallel()

	c := NewClient(WithCache(errCache{getErr: errors.New("cache down")}))
	_, err := c.GetPlayerStats(context.Background(), "DaniilMedvedev")
	if err == nil || !strings.Contains(err.Error(), "cache down") {
		t.Fatalf("expected cache error, got %v", err)
	}
}

func TestGetPlayerStats_cacheSetError(t *testing.T) {
	t.Parallel()

	fixture, err := os.ReadFile("testdata/player_medvedev.html")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		servePlayerFragJS(t, w, fixture)
	}))
	t.Cleanup(srv.Close)

	c := NewClient(
		WithBaseURL(srv.URL),
		WithCache(errCache{setErr: errors.New("set failed")}),
	)
	_, err = c.GetPlayerStats(context.Background(), "DaniilMedvedev")
	if err == nil || !strings.Contains(err.Error(), "set failed") {
		t.Fatalf("expected set error, got %v", err)
	}
}

func TestFetchPlayerHTML_invalidURL(t *testing.T) {
	t.Parallel()

	c := NewClient(WithBaseURL("://bad"))
	_, err := c.fetchPlayerHTML(context.Background(), "Test")
	if err == nil || !strings.Contains(err.Error(), "parse url") {
		t.Fatalf("expected parse url error, got %v", err)
	}
}

type errReadCloser struct{}

func (errReadCloser) Read([]byte) (int, error) { return 0, errors.New("read failed") }
func (errReadCloser) Close() error           { return nil }

func TestFetchPlayerHTML_readBodyError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	c := NewClient(
		WithBaseURL(srv.URL),
		WithHTTPClient(&http.Client{
			Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       errReadCloser{},
					Header:     make(http.Header),
				}, nil
			}),
		}),
	)
	_, err := c.fetchPlayerHTML(context.Background(), "Test")
	if err == nil || !strings.Contains(err.Error(), "read body") {
		t.Fatalf("expected read body error, got %v", err)
	}
}

func TestFetchPlayerHTML_newRequestError(t *testing.T) {
	old := newRequestWithContext
	t.Cleanup(func() { newRequestWithContext = old })
	newRequestWithContext = func(context.Context, string, string, io.Reader) (*http.Request, error) {
		return nil, errors.New("new request failed")
	}

	c := NewClient(WithBaseURL("http://example.com"))
	_, err := c.fetchPlayerHTML(context.Background(), "Test")
	if err == nil || !strings.Contains(err.Error(), "new request") {
		t.Fatalf("expected new request error, got %v", err)
	}
}

func TestGetPlayerStats_parseError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		servePlayerFragJS(t, w, []byte(`<html><body><h2>Recent Results</h2></body></html>`))
	}))
	t.Cleanup(srv.Close)

	c := NewClient(WithBaseURL(srv.URL))
	_, err := c.GetPlayerStats(context.Background(), "DaniilMedvedev")
	if err == nil || !errors.Is(err, ErrTableNotFound) {
		t.Fatalf("expected parse/table error, got %v", err)
	}
}

func TestFetchPlayerHTML_badFragment(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not a player frag"))
	}))
	t.Cleanup(srv.Close)

	c := NewClient(WithBaseURL(srv.URL))
	_, err := c.fetchPlayerHTML(context.Background(), "Test")
	if err == nil || !strings.Contains(err.Error(), "player_frag") {
		t.Fatalf("expected extract error, got %v", err)
	}
}

func TestAdjustedHoldBreak_asOfFromFetchedAt(t *testing.T) {
	t.Parallel()

	fetched := time.Date(2024, 8, 1, 0, 0, 0, 0, time.UTC)
	stats := models.PlayerStats{
		FetchedAt: fetched,
		TourLevelSeasons: []models.TourLevelSeason{
			{Year: 2024, Matches: 30, HoldPct: 0.80, BreakPct: 0.25, DR: 1.10},
		},
	}
	_, err := AdjustedHoldBreak(stats, FormOptions{})
	if err != nil {
		t.Fatalf("AdjustedHoldBreak: %v", err)
	}
}

func TestAdjustedHoldBreak_asOfNow(t *testing.T) {
	t.Parallel()

	year := time.Now().Year()
	stats := models.PlayerStats{
		TourLevelSeasons: []models.TourLevelSeason{
			{Year: year, Matches: 30, HoldPct: 0.80, BreakPct: 0.25, DR: 1.10},
		},
	}
	_, err := AdjustedHoldBreak(stats, FormOptions{})
	if err != nil {
		t.Fatalf("AdjustedHoldBreak: %v", err)
	}
}

func TestAdjustedHoldBreak_zeroDRSeasonUsesNeutralDenom(t *testing.T) {
	t.Parallel()

	stats := models.PlayerStats{
		TourLevelSeasons: []models.TourLevelSeason{
			{Year: 2026, Matches: 30, HoldPct: 0.80, BreakPct: 0.25, DR: 0},
		},
		RecentResults: []models.RecentResult{
			{Date: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), DominanceRatio: ptrFloat(1.05)},
		},
	}
	rates, err := AdjustedHoldBreak(stats, FormOptions{AsOf: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatalf("AdjustedHoldBreak: %v", err)
	}
	if rates.FormRatio != 1.05 {
		t.Fatalf("FormRatio = %v, want drForm/1.0 when season DR is 0", rates.FormRatio)
	}
}

func ptrFloat(v float64) *float64 { return &v }

func TestParsePlayerHTML_invalidHTML(t *testing.T) {
	t.Parallel()

	r := io.NopCloser(errReader{})
	_, err := ParsePlayerHTML(r, "X")
	if err == nil || !strings.Contains(err.Error(), "parse html") {
		t.Fatalf("expected parse html error, got %v", err)
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("read error") }

func TestParseHelpers(t *testing.T) {
	t.Parallel()

	if parseInt("12") != 12 {
		t.Fatal("parseInt valid")
	}
	if parseInt("-") != 0 || parseInt("") != 0 || parseInt("x") != 0 {
		t.Fatal("parseInt invalid")
	}
	if parseFloat("1.5") != 1.5 {
		t.Fatal("parseFloat valid")
	}
	if parseFloat("-") != 0 || parseFloat("bad") != 0 {
		t.Fatal("parseFloat invalid")
	}
	if parsePercent("50%") != 0.5 {
		t.Fatal("parsePercent percent")
	}
	if parsePercent("bad%") != 0 || parsePercent("-") != 0 || parsePercent("0.25") != 0.25 {
		t.Fatal("parsePercent edge cases")
	}
	if got := parseOptionalFloat("-"); got != nil {
		t.Fatal("parseOptionalFloat dash")
	}
	if v := parseOptionalFloat("1.2"); v == nil || *v != 1.2 {
		t.Fatal("parseOptionalFloat value")
	}
}

func TestNormalizeHeading_multiline(t *testing.T) {
	t.Parallel()
	got := normalizeHeading("Recent Results\nextra")
	if got != "Recent Results" {
		t.Fatalf("normalizeHeading = %q", got)
	}
}
