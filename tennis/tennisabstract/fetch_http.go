package tennisabstract

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultMinRequestInterval = 2 * time.Second
	defaultHTTPMaxRetries     = 6
	defaultHTTPBackoffInitial = 5 * time.Second
	defaultHTTPBackoffMax     = 2 * time.Minute

	requestIntervalEnv = "TENNISABSTRACT_REQUEST_INTERVAL"
	httpMaxRetriesEnv    = "TENNISABSTRACT_HTTP_MAX_RETRIES"
	httpBackoffEnv       = "TENNISABSTRACT_HTTP_BACKOFF"
)

var (
	httpRequestGate sync.Mutex
	lastHTTPRequest time.Time
)

// WithMinRequestInterval sets the minimum spacing between outbound HTTP requests.
// Zero disables throttling (default for NewClient; use HTTPClientOptionsFromEnv for CLI tools).
func WithMinRequestInterval(d time.Duration) Option {
	return func(c *Client) {
		if d >= 0 {
			c.minRequestInterval = d
		}
	}
}

// WithHTTPMaxRetries sets how many times to retry after HTTP 429. Zero uses the default (6).
func WithHTTPMaxRetries(n int) Option {
	return func(c *Client) {
		if n >= 0 {
			c.httpMaxRetries = n
		}
	}
}

// WithHTTPBackoff sets the initial backoff after 429; doubles each retry up to WithHTTPBackoffMax.
func WithHTTPBackoff(initial time.Duration) Option {
	return func(c *Client) {
		if initial > 0 {
			c.httpBackoffInitial = initial
		}
	}
}

// WithHTTPBackoffMax caps exponential backoff after 429.
func WithHTTPBackoffMax(max time.Duration) Option {
	return func(c *Client) {
		if max > 0 {
			c.httpBackoffMax = max
		}
	}
}

// HTTPClientOptionsFromEnv returns rate-limit and retry options for live Tennis Abstract fetches.
// Env: TENNISABSTRACT_REQUEST_INTERVAL (default 2s), TENNISABSTRACT_HTTP_MAX_RETRIES (default 6),
// TENNISABSTRACT_HTTP_BACKOFF (default 5s, initial backoff on 429).
func HTTPClientOptionsFromEnv() []Option {
	return []Option{
		WithMinRequestInterval(MinRequestIntervalFromEnv()),
		WithHTTPMaxRetries(HTTPMaxRetriesFromEnv()),
		WithHTTPBackoff(HTTPBackoffInitialFromEnv()),
		WithHTTPBackoffMax(defaultHTTPBackoffMax),
	}
}

// MinRequestIntervalFromEnv reads TENNISABSTRACT_REQUEST_INTERVAL (default 2s).
func MinRequestIntervalFromEnv() time.Duration {
	return durationFromEnv(requestIntervalEnv, defaultMinRequestInterval)
}

// HTTPMaxRetriesFromEnv reads TENNISABSTRACT_HTTP_MAX_RETRIES (default 6).
func HTTPMaxRetriesFromEnv() int {
	raw := strings.TrimSpace(os.Getenv(httpMaxRetriesEnv))
	if raw == "" {
		return defaultHTTPMaxRetries
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return defaultHTTPMaxRetries
	}
	return n
}

// HTTPBackoffInitialFromEnv reads TENNISABSTRACT_HTTP_BACKOFF (default 5s).
func HTTPBackoffInitialFromEnv() time.Duration {
	return durationFromEnv(httpBackoffEnv, defaultHTTPBackoffInitial)
}

func durationFromEnv(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d < 0 {
		return fallback
	}
	return d
}

func waitRequestInterval(ctx context.Context, minInterval time.Duration) error {
	if minInterval <= 0 {
		return nil
	}
	for {
		httpRequestGate.Lock()
		wait := minInterval - time.Since(lastHTTPRequest)
		if wait <= 0 {
			lastHTTPRequest = time.Now()
			httpRequestGate.Unlock()
			return nil
		}
		httpRequestGate.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
}

func (c *Client) effectiveHTTPMaxRetries() int {
	if c.httpMaxRetries > 0 {
		return c.httpMaxRetries
	}
	return defaultHTTPMaxRetries
}

func (c *Client) effectiveHTTPBackoffInitial() time.Duration {
	if c.httpBackoffInitial > 0 {
		return c.httpBackoffInitial
	}
	return defaultHTTPBackoffInitial
}

func (c *Client) effectiveHTTPBackoffMax() time.Duration {
	if c.httpBackoffMax > 0 {
		return c.httpBackoffMax
	}
	return defaultHTTPBackoffMax
}

// doHTTP performs a GET with global request spacing and retries on HTTP 429.
func (c *Client) doHTTP(ctx context.Context, req *http.Request) (*http.Response, error) {
	maxRetries := c.effectiveHTTPMaxRetries()
	backoffInitial := c.effectiveHTTPBackoffInitial()
	backoffMax := c.effectiveHTTPBackoffMax()

	for attempt := 0; ; attempt++ {
		if err := waitRequestInterval(ctx, c.minRequestInterval); err != nil {
			return nil, err
		}
		resp, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusTooManyRequests {
			return resp, nil
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()

		if attempt >= maxRetries {
			return nil, fmt.Errorf("fetch %s: rate limited (429) after %d retries", req.URL.Path, maxRetries)
		}

		backoff := retryBackoff(attempt, backoffInitial, backoffMax)
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if d, ok := parseRetryAfter(ra); ok && d > backoff {
				backoff = d
			}
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
	}
}

func retryBackoff(attempt int, initial, max time.Duration) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	d := initial
	for i := 0; i < attempt; i++ {
		if d >= max/2 {
			return max
		}
		d *= 2
	}
	if d > max {
		return max
	}
	return d
}

func parseRetryAfter(v string) (time.Duration, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, false
	}
	if sec, err := strconv.Atoi(v); err == nil && sec >= 0 {
		return time.Duration(sec) * time.Second, true
	}
	if t, err := http.ParseTime(v); err == nil {
		d := time.Until(t)
		if d < 0 {
			d = 0
		}
		return d, true
	}
	return 0, false
}
