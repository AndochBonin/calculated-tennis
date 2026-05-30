package clob

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/AndochBonin/E3/polymarket/models"
)

var parsePositionsURL = url.Parse

func (c *Client) GetPositions() ([]models.Position, error) {
	root := strings.TrimSpace(c.dataAPIBaseURL)
	if root == "" {
		root = defaultDataAPIBaseURL
	}

	uPath, err := url.JoinPath(root, "positions")
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	u, err := parsePositionsURL(uPath)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	q := u.Query()
	if v := strings.TrimSpace(c.userAddress); v != "" {
		q.Set("user", v)
	}
	u.RawQuery = q.Encode()

	uc := *u
	req := &http.Request{
		Method:     http.MethodGet,
		URL:        &uc,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     make(http.Header),
		Host:       uc.Host,
	}

	c.addAuthHeaders(req, "")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get positions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errUnexpectedHTTP("get positions", resp)
	}

	var positions []models.Position
	if err := json.NewDecoder(resp.Body).Decode(&positions); err != nil {
		return nil, fmt.Errorf("decode positions: %w", err)
	}

	return positions, nil
}
