package clob

import (
	"encoding/json"
	"fmt"
	"github.com/AndochBonin/polymarket/models"
	"net/http"
)

func (c *Client) GetOrders() (*models.OrdersResponse, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/data/orders", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	c.addAuthHeaders(req, "")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get orders: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errUnexpectedHTTP("get orders", resp)
	}

	var orders models.OrdersResponse
	if err := json.NewDecoder(resp.Body).Decode(&orders); err != nil {
		return nil, fmt.Errorf("decode orders: %w", err)
	}

	return &orders, nil
}
