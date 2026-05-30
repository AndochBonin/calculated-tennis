package gamma

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/AndochBonin/E3/polymarket/models"
)

const gammaBaseURL = "https://gamma-api.polymarket.com"

type Client struct {
	http    *http.Client
	baseURL string
}

type Option func(*Client)

func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		if baseURL != "" {
			c.baseURL = baseURL
		}
	}
}

func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		if httpClient != nil {
			c.http = httpClient
		}
	}
}

func WithTransport(transport http.RoundTripper) Option {
	return func(c *Client) {
		if transport != nil {
			c.http.Transport = transport
		}
	}
}

func NewClient(opts ...Option) *Client {
	c := &Client{
		http:    &http.Client{},
		baseURL: gammaBaseURL,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

type MarketsParams struct {
	TagID             int 
	Slug              string
	Closed            *bool
	Limit             int
	Offset            int
	SportsMarketTypes []string
	StartDateMin      *time.Time
	StartDateMax      *time.Time
	EndDateMin        *time.Time
	EndDateMax        *time.Time
}

func addIntParam(q url.Values, key string, value int) {
	if value > 0 {
		q.Set(key, fmt.Sprintf("%d", value))
	}
}

func addBoolParam(q url.Values, key string, value *bool) {
	if value != nil {
		q.Set(key, fmt.Sprintf("%t", *value))
	}
}

func addTimeParam(q url.Values, key string, value *time.Time) {
	if value != nil {
		q.Set(key, value.Format(time.RFC3339))
	}
}

func addStringParam(q url.Values, key string, value string) {
	if strings.TrimSpace(value) != "" {
		q.Set(key, strings.TrimSpace(value))
	}
}

func (c *Client) GetMarkets(ctx context.Context, params MarketsParams) ([]models.GammaMarket, error) {
	u, err := url.Parse(c.baseURL + "/markets")
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}

	q := u.Query()

	addIntParam(q, "tag_id", params.TagID)
	addStringParam(q, "slug", params.Slug)
	addBoolParam(q, "closed", params.Closed)
	addIntParam(q, "limit", params.Limit)
	addIntParam(q, "offset", params.Offset)

	if params.SportsMarketTypes != nil || len(params.SportsMarketTypes) != 0 {
		for _, smt := range params.SportsMarketTypes {
			q.Add("sports_market_types", smt)
		}
	}

	addTimeParam(q, "start_date_min", params.StartDateMin)
	addTimeParam(q, "start_date_max", params.StartDateMax)
	addTimeParam(q, "end_date_min", params.EndDateMin)
	addTimeParam(q, "end_date_max", params.EndDateMax)

	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get markets: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get markets: unexpected status %d", resp.StatusCode)
	}

	var markets []models.GammaMarket
	if err := json.NewDecoder(resp.Body).Decode(&markets); err != nil {
		return nil, fmt.Errorf("decode markets: %w", err)
	}

	return markets, nil
}
