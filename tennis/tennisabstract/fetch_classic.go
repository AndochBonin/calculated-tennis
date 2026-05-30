package tennisabstract

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	playerClassicPathPrefix = "/cgi-bin/player-classic.cgi"
	careerMatchesJSPrefix   = "/jsmatches/"
	defaultClassicFilter    = "ACareerqq"
)

func playerClassicPath(slug, filter string) string {
	if filter == "" {
		filter = defaultClassicFilter
	}
	return fmt.Sprintf("%s?p=%s&f=%s", playerClassicPathPrefix, url.QueryEscape(slug), url.QueryEscape(filter))
}

func careerMatchesPath(slug string) string {
	return careerMatchesJSPrefix + slug + "Career.js"
}

// fetchPlayerClassicPage loads the player-classic.cgi page (embeds matchmx).
func (c *Client) fetchPlayerClassicPage(ctx context.Context, slug, filter string) ([]byte, error) {
	return c.fetchGET(ctx, playerClassicPath(slug, filter))
}

// fetchCareerMatchesJS loads supplemental career match rows (morematchmx).
// A missing file returns (nil, nil) — not all players have older rows split out.
func (c *Client) fetchCareerMatchesJS(ctx context.Context, slug string) ([]byte, error) {
	body, err := c.fetchGETAllowNotFound(ctx, careerMatchesPath(slug))
	if err != nil {
		return nil, fmt.Errorf("fetch career matches js: %w", err)
	}
	return body, nil
}

func (c *Client) fetchGET(ctx context.Context, path string) ([]byte, error) {
	body, status, err := c.fetchGETStatus(ctx, path)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: unexpected status %d", path, status)
	}
	return body, nil
}

func (c *Client) fetchGETAllowNotFound(ctx context.Context, path string) ([]byte, error) {
	body, status, err := c.fetchGETStatus(ctx, path)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound {
		return nil, nil
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: unexpected status %d", path, status)
	}
	return body, nil
}

func (c *Client) fetchGETStatus(ctx context.Context, path string) ([]byte, int, error) {
	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return nil, 0, fmt.Errorf("fetch: parse url: %w", err)
	}

	req, err := newRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, 0, fmt.Errorf("fetch: new request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.doHTTP(ctx, req)
	if err != nil {
		return nil, 0, fmt.Errorf("fetch %s: %w", path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("fetch %s: read body: %w", path, err)
	}
	return body, resp.StatusCode, nil
}

// extractJSArray finds var {name} = [[...], ...] in body and parses it into rows of strings.
func extractJSArray(body []byte, name string) ([][]string, error) {
	s := string(body)
	start := -1
	for _, marker := range []string{"var " + name + " = ", name + " = "} {
		if i := strings.Index(s, marker); i >= 0 {
			start = i + len(marker)
			break
		}
	}
	if start < 0 {
		return nil, fmt.Errorf("%s assignment not found", name)
	}

	rows, err := parseJSMatchMatrix(s[start:])
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", name, err)
	}
	return rows, nil
}

func parseJSMatchMatrix(s string) ([][]string, error) {
	p := &jsArrayParser{s: strings.TrimLeft(s, " \t\r\n")}
	rows, err := p.parseMatrix()
	if err != nil {
		return nil, err
	}
	return rows, nil
}

type jsArrayParser struct {
	s string
	i int
}

func (p *jsArrayParser) skipWS() {
	for p.i < len(p.s) {
		switch p.s[p.i] {
		case ' ', '\t', '\n', '\r':
			p.i++
		default:
			return
		}
	}
}

func (p *jsArrayParser) peek() byte {
	if p.i >= len(p.s) {
		return 0
	}
	return p.s[p.i]
}

func (p *jsArrayParser) parseString() (string, error) {
	if p.peek() != '"' {
		return "", fmt.Errorf("expected '\"' at offset %d", p.i)
	}
	p.i++
	var b strings.Builder
	for p.i < len(p.s) {
		c := p.s[p.i]
		if c == '\\' {
			p.i++
			if p.i >= len(p.s) {
				return "", fmt.Errorf("unterminated escape at offset %d", p.i)
			}
			switch p.s[p.i] {
			case '"', '\\', '/':
				b.WriteByte(p.s[p.i])
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			default:
				b.WriteByte(p.s[p.i])
			}
			p.i++
			continue
		}
		if c == '"' {
			p.i++
			return b.String(), nil
		}
		b.WriteByte(c)
		p.i++
	}
	return "", fmt.Errorf("unterminated string at offset %d", p.i)
}

func (p *jsArrayParser) parseAtom() (string, error) {
	p.skipWS()
	switch p.peek() {
	case '"':
		return p.parseString()
	case '[':
		return "", fmt.Errorf("nested array not expected at offset %d", p.i)
	default:
		start := p.i
		for p.i < len(p.s) {
			c := p.s[p.i]
			if c == ',' || c == ']' {
				break
			}
			p.i++
		}
		return strings.TrimSpace(p.s[start:p.i]), nil
	}
}

func (p *jsArrayParser) parseRow() ([]string, error) {
	p.skipWS()
	if p.peek() != '[' {
		return nil, fmt.Errorf("expected '[' for row at offset %d", p.i)
	}
	p.i++
	var row []string
	for {
		p.skipWS()
		if p.peek() == ']' {
			p.i++
			return row, nil
		}
		atom, err := p.parseAtom()
		if err != nil {
			return nil, err
		}
		row = append(row, atom)
		p.skipWS()
		if p.peek() == ',' {
			p.i++
			continue
		}
		if p.peek() == ']' {
			p.i++
			return row, nil
		}
		return nil, fmt.Errorf("expected ',' or ']' in row at offset %d", p.i)
	}
}

func (p *jsArrayParser) parseMatrix() ([][]string, error) {
	p.skipWS()
	if p.peek() != '[' {
		return nil, fmt.Errorf("expected '[' for matrix at offset %d", p.i)
	}
	p.i++
	var rows [][]string
	for {
		p.skipWS()
		if p.peek() == ']' {
			p.i++
			return rows, nil
		}
		row, err := p.parseRow()
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
		p.skipWS()
		if p.peek() == ',' {
			p.i++
			continue
		}
		if p.peek() == ']' {
			p.i++
			return rows, nil
		}
		return nil, fmt.Errorf("expected ',' or ']' in matrix at offset %d", p.i)
	}
}
