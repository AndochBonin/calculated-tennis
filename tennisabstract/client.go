package tennisabstract

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/AndochBonin/polymarket/models"
)

const defaultBaseURL = "https://www.tennisabstract.com"

// Client fetches and parses Tennis Abstract player pages.
type Client struct {
	http     *http.Client
	baseURL  string
	cache    Cache
	cacheTTL time.Duration
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL overrides the Tennis Abstract site root (for tests).
func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		if baseURL != "" {
			c.baseURL = baseURL
		}
	}
}

// WithHTTPClient sets the HTTP client used for requests.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		if httpClient != nil {
			c.http = httpClient
		}
	}
}

// WithTransport sets the HTTP transport on the client's client.
func WithTransport(transport http.RoundTripper) Option {
	return func(c *Client) {
		if transport != nil {
			c.http.Transport = transport
		}
	}
}

// WithCache enables Redis (or other) caching of parsed player stats.
func WithCache(cache Cache) Option {
	return func(c *Client) {
		c.cache = cache
	}
}

// WithCacheTTL sets how long cached stats are kept. Zero uses the 6h default.
func WithCacheTTL(ttl time.Duration) Option {
	return func(c *Client) {
		if ttl > 0 {
			c.cacheTTL = ttl
		}
	}
}

// NewClient builds a Client with optional configuration.
func NewClient(opts ...Option) *Client {
	c := &Client{
		http:     &http.Client{Timeout: defaultHTTPTimeout},
		baseURL:  defaultBaseURL,
		cacheTTL: defaultCacheTTL,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// GetPlayerStats loads and parses stats for playerName (display name or slug).
// When a cache is configured, it returns cached JSON on hit; on miss it fetches
// the page, parses, stores parsed stats, and returns them.
func (c *Client) GetPlayerStats(ctx context.Context, playerName string) (models.PlayerStats, error) {
	slug := PlayerSlug(playerName)
	if slug == "" {
		return models.PlayerStats{}, fmt.Errorf("get player stats: empty player name")
	}

	if stats, ok, err := GetCachedPlayerStats(ctx, c.cache, slug); err != nil {
		return models.PlayerStats{}, fmt.Errorf("get player stats: %w", err)
	} else if ok {
		return stats, nil
	}

	body, err := c.fetchPlayerHTML(ctx, slug)
	if err != nil {
		return models.PlayerStats{}, err
	}
	defer body.Close()

	stats, err := ParsePlayerHTML(body, slug)
	if err != nil {
		return models.PlayerStats{}, err
	}
	stats.FetchedAt = time.Now().UTC()

	if err := SetCachedPlayerStats(ctx, c.cache, slug, stats, c.cacheTTL); err != nil {
		return models.PlayerStats{}, fmt.Errorf("get player stats: %w", err)
	}
	return stats, nil
}
