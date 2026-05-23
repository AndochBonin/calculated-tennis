package tennisabstract

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultHTTPTimeout = 30 * time.Second
	playerFragJSPrefix = "/jsfrags/"

	playerFragAssign = "var player_frag = `"
	playerFragEnd    = "`;"

	// userAgent identifies this project to Tennis Abstract.
	userAgent = "polymarket-tennisabstract/1.0 (+https://github.com/AndochBonin/polymarket)"
)

// newRequestWithContext is swappable in tests to cover request construction errors.
var newRequestWithContext = http.NewRequestWithContext

func playerFragJSPath(slug string) string {
	return playerFragJSPrefix + slug + ".js"
}

// fetchPlayerHTML loads the player page fragment JS and returns the embedded HTML.
// Tennis Abstract serves an empty shell from player.cgi and injects stats via
// jsfrags/{slug}.js (var player_frag = `...`).
func (c *Client) fetchPlayerHTML(ctx context.Context, slug string) (io.ReadCloser, error) {
	u, err := url.Parse(c.baseURL + playerFragJSPath(slug))
	if err != nil {
		return nil, fmt.Errorf("fetch player page: parse url: %w", err)
	}

	req, err := newRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("fetch player page: new request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch player page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch player page: unexpected status %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("fetch player page: read body: %w", err)
	}

	html, err := extractPlayerFrag(raw)
	if err != nil {
		return nil, fmt.Errorf("fetch player page: %w", err)
	}
	return io.NopCloser(bytes.NewReader(html)), nil
}

func extractPlayerFrag(js []byte) ([]byte, error) {
	s := string(js)
	start := strings.Index(s, playerFragAssign)
	if start < 0 {
		return nil, fmt.Errorf("player_frag assignment not found")
	}
	start += len(playerFragAssign)
	end := strings.LastIndex(s, playerFragEnd)
	if end < start {
		return nil, fmt.Errorf("player_frag template not terminated")
	}
	return []byte(s[start:end]), nil
}
