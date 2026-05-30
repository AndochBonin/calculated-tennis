package tennisabstract

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/AndochBonin/E3/tennis/models"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestPlayerCacheKey(t *testing.T) {
	t.Parallel()

	if got := PlayerCacheKey("DaniilMedvedev"); got != "tennisabstract:player:daniilmedvedev" {
		t.Fatalf("PlayerCacheKey() = %q", got)
	}
	if got := PlayerCacheKey("  FooBar  "); got != "tennisabstract:player:foobar" {
		t.Fatalf("PlayerCacheKey(trim) = %q", got)
	}
}

func TestCacheTTLFromEnv(t *testing.T) {
	t.Setenv(cacheTTLEnv, "")
	if got := CacheTTLFromEnv(); got != defaultCacheTTL {
		t.Fatalf("empty env = %v, want %v", got, defaultCacheTTL)
	}

	t.Setenv(cacheTTLEnv, "30m")
	if got := CacheTTLFromEnv(); got != 30*time.Minute {
		t.Fatalf("30m = %v", got)
	}

	t.Setenv(cacheTTLEnv, "not-a-duration")
	if got := CacheTTLFromEnv(); got != defaultCacheTTL {
		t.Fatalf("invalid = %v, want default", got)
	}

	t.Setenv(cacheTTLEnv, "-1h")
	if got := CacheTTLFromEnv(); got != defaultCacheTTL {
		t.Fatalf("negative = %v, want default", got)
	}
}

func TestRedisCacheGetSet(t *testing.T) {
	t.Parallel()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	cache := NewRedisCache(client)
	ctx := context.Background()
	key := "tennisabstract:player:test"

	val, ok, err := cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get miss err: %v", err)
	}
	if ok || val != nil {
		t.Fatalf("Get miss: ok=%v val=%q", ok, val)
	}

	payload := []byte(`{"PlayerSlug":"Test"}`)
	if err := cache.Set(ctx, key, payload, time.Hour); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, ok, err := cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get hit err: %v", err)
	}
	if !ok {
		t.Fatal("expected cache hit")
	}
	if string(got) != string(payload) {
		t.Fatalf("Get = %q, want %q", got, payload)
	}
}

func TestRedisCacheGetRedisNil(t *testing.T) {
	t.Parallel()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	cache := NewRedisCache(client)
	_, ok, err := cache.Get(context.Background(), "missing")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if ok {
		t.Fatal("expected miss")
	}
}

func TestGetSetCachedPlayerStats(t *testing.T) {
	t.Parallel()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	cache := NewRedisCache(client)
	ctx := context.Background()
	slug := "DaniilMedvedev"

	_, ok, err := GetCachedPlayerStats(ctx, cache, slug)
	if err != nil {
		t.Fatalf("GetCached miss err: %v", err)
	}
	if ok {
		t.Fatal("expected cache miss")
	}

	want := models.PlayerStats{
		PlayerSlug: slug,
		FetchedAt:  time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC),
		RecentResults: []models.RecentResult{
			{Tournament: "Rome"},
		},
		ChallengerSeasons: []models.TourLevelSeason{
			{Year: 2026, Matches: 15, HoldPct: 0.75, BreakPct: 0.25, DR: 1.05},
		},
	}
	ttl := 2 * time.Hour
	if err := SetCachedPlayerStats(ctx, cache, slug, want, ttl); err != nil {
		t.Fatalf("SetCached: %v", err)
	}

	got, ok, err := GetCachedPlayerStats(ctx, cache, slug)
	if err != nil {
		t.Fatalf("GetCached hit err: %v", err)
	}
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got.PlayerSlug != want.PlayerSlug {
		t.Fatalf("PlayerSlug = %q, want %q", got.PlayerSlug, want.PlayerSlug)
	}
	if !got.FetchedAt.Equal(want.FetchedAt) {
		t.Fatalf("FetchedAt = %v, want %v", got.FetchedAt, want.FetchedAt)
	}
	if len(got.RecentResults) != 1 || got.RecentResults[0].Tournament != "Rome" {
		t.Fatalf("RecentResults = %+v", got.RecentResults)
	}
	if len(got.ChallengerSeasons) != 1 || got.ChallengerSeasons[0].Year != 2026 || got.ChallengerSeasons[0].Matches != 15 {
		t.Fatalf("ChallengerSeasons = %+v", got.ChallengerSeasons)
	}

	stored, err := mr.Get(PlayerCacheKey(slug))
	if err != nil {
		t.Fatalf("miniredis get: %v", err)
	}
	var roundTrip models.PlayerStats
	if err := json.Unmarshal([]byte(stored), &roundTrip); err != nil {
		t.Fatalf("stored json: %v", err)
	}
	if roundTrip.PlayerSlug != slug {
		t.Fatalf("stored slug = %q", roundTrip.PlayerSlug)
	}

	ttlRemaining := mr.TTL(PlayerCacheKey(slug))
	if ttlRemaining <= 0 || ttlRemaining > ttl {
		t.Fatalf("TTL = %v, want positive and <= %v", ttlRemaining, ttl)
	}
}

func TestGetCachedPlayerStatsInvalidJSON(t *testing.T) {
	t.Parallel()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	cache := NewRedisCache(client)
	ctx := context.Background()
	key := PlayerCacheKey("bad")
	if err := cache.Set(ctx, key, []byte("not json"), time.Hour); err != nil {
		t.Fatalf("Set: %v", err)
	}

	_, ok, err := GetCachedPlayerStats(ctx, cache, "bad")
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
	if ok {
		t.Fatal("ok should be false on corrupt value")
	}
}

func TestNilCacheNoOps(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	_, ok, err := GetCachedPlayerStats(ctx, nil, "x")
	if err != nil || ok {
		t.Fatalf("nil cache get: ok=%v err=%v", ok, err)
	}
	if err := SetCachedPlayerStats(ctx, nil, "x", models.PlayerStats{}, time.Hour); err != nil {
		t.Fatalf("nil cache set: %v", err)
	}
}

func TestNewRedisClientFromEnv(t *testing.T) {
	t.Setenv(redisURLEnv, "")
	t.Setenv(redisAddrEnv, "")

	client, err := NewRedisClientFromEnv()
	if err != nil {
		t.Fatalf("NewRedisClientFromEnv: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Skipf("redis not available at %s: %v", defaultRedisAddr, err)
	}
}

func TestNewRedisClientFromEnvURL(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	t.Setenv(redisURLEnv, "redis://"+mr.Addr())
	t.Setenv(redisAddrEnv, "")

	client, err := NewRedisClientFromEnv()
	if err != nil {
		t.Fatalf("NewRedisClientFromEnv: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestNewRedisClientFromEnvBadURL(t *testing.T) {
	t.Setenv(redisURLEnv, "://bad")
	_, err := NewRedisClientFromEnv()
	if err == nil {
		t.Fatal("expected error for bad REDIS_URL")
	}
}
