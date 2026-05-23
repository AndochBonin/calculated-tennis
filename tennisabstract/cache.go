package tennisabstract

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/AndochBonin/polymarket/models"
	"github.com/redis/go-redis/v9"
)

// jsonMarshal is swappable in tests to cover marshal error handling.
var jsonMarshal = json.Marshal

const (
	cacheKeyPrefix     = "tennisabstract:player:"
	defaultCacheTTL    = 6 * time.Hour
	cacheTTLEnv        = "TENNISABSTRACT_CACHE_TTL"
	redisAddrEnv       = "REDIS_ADDR"
	redisURLEnv        = "REDIS_URL"
	defaultRedisAddr   = "localhost:6379"
)

// Cache stores opaque byte values with a TTL. A miss returns ok == false and no error.
type Cache interface {
	Get(ctx context.Context, key string) (val []byte, ok bool, err error)
	Set(ctx context.Context, key string, val []byte, ttl time.Duration) error
}

// RedisCache implements Cache with a go-redis client.
type RedisCache struct {
	client *redis.Client
}

// NewRedisCache wraps an existing redis client.
func NewRedisCache(client *redis.Client) *RedisCache {
	if client == nil {
		return nil
	}
	return &RedisCache{client: client}
}

// NewRedisClientFromEnv builds a redis client from REDIS_URL or REDIS_ADDR.
// REDIS_URL takes precedence. REDIS_ADDR defaults to localhost:6379 when unset.
func NewRedisClientFromEnv() (*redis.Client, error) {
	if raw := strings.TrimSpace(os.Getenv(redisURLEnv)); raw != "" {
		opt, err := redis.ParseURL(raw)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", redisURLEnv, err)
		}
		return redis.NewClient(opt), nil
	}
	addr := strings.TrimSpace(os.Getenv(redisAddrEnv))
	if addr == "" {
		addr = defaultRedisAddr
	}
	return redis.NewClient(&redis.Options{Addr: addr}), nil
}

// NewRedisCacheFromEnv combines NewRedisClientFromEnv and NewRedisCache.
func NewRedisCacheFromEnv() (*RedisCache, error) {
	client, err := NewRedisClientFromEnv()
	if err != nil {
		return nil, err
	}
	return NewRedisCache(client), nil
}

// CacheTTLFromEnv reads TENNISABSTRACT_CACHE_TTL (e.g. "6h"). Invalid or empty values use 6h.
func CacheTTLFromEnv() time.Duration {
	raw := strings.TrimSpace(os.Getenv(cacheTTLEnv))
	if raw == "" {
		return defaultCacheTTL
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return defaultCacheTTL
	}
	return d
}

// PlayerCacheKey is the Redis key for parsed player stats (slug is lowercased).
func PlayerCacheKey(slug string) string {
	return cacheKeyPrefix + strings.ToLower(strings.TrimSpace(slug))
}

func (c *RedisCache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	if c == nil || c.client == nil {
		return nil, false, nil
	}
	val, err := c.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return val, true, nil
}

func (c *RedisCache) Set(ctx context.Context, key string, val []byte, ttl time.Duration) error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Set(ctx, key, val, ttl).Err()
}

// GetCachedPlayerStats loads JSON-marshaled stats from cache. ok is false on miss.
func GetCachedPlayerStats(ctx context.Context, cache Cache, slug string) (models.PlayerStats, bool, error) {
	if cache == nil {
		return models.PlayerStats{}, false, nil
	}
	raw, ok, err := cache.Get(ctx, PlayerCacheKey(slug))
	if err != nil || !ok {
		return models.PlayerStats{}, ok, err
	}
	var stats models.PlayerStats
	if err := json.Unmarshal(raw, &stats); err != nil {
		return models.PlayerStats{}, false, fmt.Errorf("unmarshal cached player stats: %w", err)
	}
	return stats, true, nil
}

// SetCachedPlayerStats stores parsed stats as JSON with the given TTL.
func SetCachedPlayerStats(ctx context.Context, cache Cache, slug string, stats models.PlayerStats, ttl time.Duration) error {
	if cache == nil {
		return nil
	}
	raw, err := jsonMarshal(stats)
	if err != nil {
		return fmt.Errorf("marshal player stats: %w", err)
	}
	return cache.Set(ctx, PlayerCacheKey(slug), raw, ttl)
}
