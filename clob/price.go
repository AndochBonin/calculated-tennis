package clob

import (
	"encoding/json"
	"fmt"
	"github.com/AndochBonin/polymarket/models"
	"net/http"
	"net/url"
)

func (c *Client) GetMarketPrice(tokenID string, side string) (*models.ClobMarketPrice, error) {
	u, err := url.Parse(c.baseURL + "/price")
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}

	q := u.Query()
	q.Set("token_id", tokenID)
	q.Set("side", side)
	u.RawQuery = q.Encode()

	resp, err := c.http.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("get market price: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get market price: unexpected status %d", resp.StatusCode)
	}

	var marketPrice models.ClobMarketPrice
	if err := json.NewDecoder(resp.Body).Decode(&marketPrice); err != nil {
		return nil, fmt.Errorf("decode marketPrice : %w", err)
	}

	return &marketPrice, nil
}
