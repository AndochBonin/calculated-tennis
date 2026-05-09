package clob

import (
	"encoding/json"
	"fmt"
	"github.com/AndochBonin/polymarket/models"
	"net/http"
)

func (c *Client) GetTrades() (*models.TradesResponse, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/data/trades", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	c.addAuthHeaders(req, "")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get trades: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errUnexpectedHTTP("get trades", resp)
	}

	var trades models.TradesResponse
	if err := json.NewDecoder(resp.Body).Decode(&trades); err != nil {
		return nil, fmt.Errorf("decode trades: %w", err)
	}

	return &trades, nil
}
