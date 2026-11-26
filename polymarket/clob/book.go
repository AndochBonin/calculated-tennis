package clob

import (
	"encoding/json"
	"fmt"
	"github.com/AndochBonin/calculated-tennis/polymarket/models"
	"net/http"
	"net/url"
)

func (c *Client) GetOrderBook(tokenID string) (*models.OrderBook, error) {
	u, err := url.Parse(c.baseURL + "/book")
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}

	q := u.Query()
	q.Set("token_id", tokenID)
	u.RawQuery = q.Encode()

	resp, err := c.http.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("get order book: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errUnexpectedHTTP("get order book", resp)
	}

	var book models.OrderBook
	if err := json.NewDecoder(resp.Body).Decode(&book); err != nil {
		return nil, fmt.Errorf("decode order book: %w", err)
	}

	return &book, nil
}
