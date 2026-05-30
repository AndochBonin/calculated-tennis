package tennisabstract

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/AndochBonin/E3/tennis/models"
)

const defaultBaseURL = "https://www.tennisabstract.com"

// Client fetches and parses Tennis Abstract player pages.
type Client struct {
	http               *http.Client
	baseURL            string
	cache              Cache
	cacheTTL           time.Duration
	careerCacheDir     string
	minRequestInterval time.Duration
	httpMaxRetries     int
	httpBackoffInitial time.Duration
	httpBackoffMax     time.Duration
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

// WithCareerCacheDir enables on-disk JSON caching of career match lists.
// An empty dir disables disk read/write (live fetch only).
func WithCareerCacheDir(dir string) Option {
	return func(c *Client) {
		c.careerCacheDir = dir
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

// GetCareerMatches loads the full merged career match list for playerName.
// When careerCacheDir is set, it returns cached JSON on hit; on miss it fetches
// player-classic.cgi and optional Career.js, parses matchmx arrays, writes to disk, and returns.
func (c *Client) GetCareerMatches(ctx context.Context, playerName string) (models.CareerMatches, error) {
	slug := PlayerSlug(playerName)
	if slug == "" {
		return models.CareerMatches{}, fmt.Errorf("get career matches: empty player name")
	}

	if c.careerCacheDir != "" {
		if career, ok, err := ReadCareerMatchesFile(c.careerCacheDir, slug); err != nil {
			return models.CareerMatches{}, fmt.Errorf("get career matches: %w", err)
		} else if ok {
			return career, nil
		}
	}

	career, err := c.fetchCareerMatches(ctx, slug)
	if err != nil {
		return models.CareerMatches{}, err
	}

	if c.careerCacheDir != "" {
		if err := WriteCareerMatchesFile(c.careerCacheDir, slug, career); err != nil {
			return models.CareerMatches{}, fmt.Errorf("get career matches: %w", err)
		}
	}
	return career, nil
}

// GetRecentResultsAsOf returns up to limit matches strictly before asOf (UTC date-only).
// It uses GetCareerMatches and filters in-process so one on-disk career list serves every as-of date.
func (c *Client) GetRecentResultsAsOf(ctx context.Context, playerName string, asOf time.Time, limit int) ([]models.RecentResult, error) {
	career, err := c.GetCareerMatches(ctx, playerName)
	if err != nil {
		return nil, fmt.Errorf("get recent results as of: %w", err)
	}
	return RecentResultsBefore(career.Matches, asOf, limit), nil
}

func (c *Client) fetchCareerMatches(ctx context.Context, slug string) (models.CareerMatches, error) {
	classicBody, err := c.fetchPlayerClassicPage(ctx, slug, defaultClassicFilter)
	if err != nil {
		return models.CareerMatches{}, fmt.Errorf("get career matches: %w", err)
	}

	matchmx, err := extractJSArray(classicBody, "matchmx")
	if err != nil {
		return models.CareerMatches{}, fmt.Errorf("get career matches: %w", err)
	}

	var morematchmx [][]string
	careerBody, err := c.fetchCareerMatchesJS(ctx, slug)
	if err != nil {
		return models.CareerMatches{}, fmt.Errorf("get career matches: %w", err)
	}
	if len(careerBody) > 0 {
		morematchmx, err = extractJSArray(careerBody, "morematchmx")
		if err != nil {
			return models.CareerMatches{}, fmt.Errorf("get career matches: %w", err)
		}
	}

	matches, err := ParseMatchMXArrays(matchmx, morematchmx)
	if err != nil {
		return models.CareerMatches{}, fmt.Errorf("get career matches: %w", err)
	}

	return models.CareerMatches{
		PlayerSlug: slug,
		Matches:    matches,
		FetchedAt:  time.Now().UTC(),
	}, nil
}
