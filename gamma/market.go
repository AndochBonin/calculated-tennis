package gamma

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/AndochBonin/polymarket/models"
)

const gammaBaseURL = "https://gamma-api.polymarket.com"

type Client struct {
	http *http.Client
}

func NewClient() *Client {
	return &Client{http: &http.Client{}}
}

type MarketsParams struct {
	TagID             int 
	Closed            *bool
	Limit             int
	Offset            int
	SportsMarketTypes []string
	StartDateMin      *time.Time
	StartDateMax      *time.Time
	EndDateMin        *time.Time
	EndDateMax        *time.Time
}

func (c *Client) GetMarkets(ctx context.Context, params MarketsParams) ([]models.GammaMarket, error) {
	u, err := url.Parse(gammaBaseURL + "/markets")
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}

	q := u.Query()

	// could use reflection to do this dynamically instead of this long series of ifs 
	if params.TagID > 0 {
		q.Set("tag_id", fmt.Sprintf("%d", params.TagID))
	}

	if params.Closed != nil {
		q.Set("closed", fmt.Sprintf("%t", *params.Closed))
	}
	if params.Limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", params.Limit))
	}
	if params.Offset > 0 {
		q.Set("offset", fmt.Sprintf("%d", params.Offset))
	}

	if params.SportsMarketTypes != nil || len(params.SportsMarketTypes) != 0 {
		for _, smt := range params.SportsMarketTypes {
			q.Set("sports_market_types", smt)
		}
	}

	if params.StartDateMin != nil {
		q.Set("start_date_min", fmt.Sprintf("%v", params.StartDateMin))
	}

	if params.StartDateMax != nil {
		q.Set("start_date_max", fmt.Sprintf("%v", params.StartDateMax))
	}

	if params.EndDateMin != nil {
		q.Set("end_date_min", fmt.Sprintf("%v", params.EndDateMin))
	}

	if params.EndDateMax != nil {
		q.Set("end_date_max", fmt.Sprintf("%v", params.EndDateMax))
	}

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
