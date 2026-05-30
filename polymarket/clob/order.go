package clob

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/shopspring/decimal"

	"github.com/AndochBonin/E3/polymarket/models"
)

// jsonMarshalFn is swappable in tests to exercise marshal error paths (normally unreachable for these structs).
var jsonMarshalFn = json.Marshal

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

func clobSignatureTypeFromEnv() (uint8, error) {
	v := strings.TrimSpace(os.Getenv("POLYMARKET_CLOB_SIGNATURE_TYPE"))
	if v == "" {
		return 3, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("build limit order: POLYMARKET_CLOB_SIGNATURE_TYPE: %w", err)
	}
	if n < 0 || n > 3 {
		return 0, fmt.Errorf("build limit order: POLYMARKET_CLOB_SIGNATURE_TYPE=%d is invalid (must be 0..3)", n)
	}
	return uint8(n), nil
}

// BuildLimitOrder signs a limit order using this client's timestamp policy (local clock or
// CLOB GET /time when WithServerSignedTime / POLYMARKET_CLOB_SERVER_TIME is enabled).
func (c *Client) BuildLimitOrder(s *Signer, tokenID string, side models.OrderSide, price, size decimal.Decimal, negRisk bool, expiration int64) (*models.OrderPayload, error) {
	depositWallet := strings.TrimSpace(c.depositWallet)
	if depositWallet == "" {
		return nil, fmt.Errorf("build limit order: deposit wallet not configured (set POLYMARKET_DEPOSIT_WALLET or DEPOSIT_WALLET, or use WithDepositWallet)")
	}
	sigType, err := clobSignatureTypeFromEnv()
	if err != nil {
		return nil, err
	}
	return s.BuildOrder(tokenID, side, price, size, negRisk, expiration, c.orderMessageTimestampMillis(), depositWallet, sigType)
}

// PlaceOrder submits a signed order to POST /order. owner is the API key UUID for the order owner.
func (c *Client) PlaceOrder(payload *models.OrderPayload, owner string, orderType models.OrderType) (*models.PlaceOrderResponse, error) {
	owner = strings.TrimSpace(owner)
	if owner == "" {
		owner = strings.TrimSpace(c.apiKey)
	}
	if owner == "" {
		return nil, fmt.Errorf("place order: owner is empty and POLYMARKET_API_KEY is not configured")
	}

	req := models.PlaceOrderRequest{
		Order:     *payload,
		Owner:     owner,
		OrderType: orderType,
		PostOnly:  false,
		DeferExec: false,
	}

	body, err := jsonMarshalFn(req)
	if err != nil {
		return nil, fmt.Errorf("marshal order: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, c.baseURL+"/order", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	c.addAuthHeaders(httpReq, string(body))

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("place order: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errUnexpectedHTTP("place order", resp)
	}

	var out models.PlaceOrderResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode place order response: %w", err)
	}

	if !out.Success || out.ErrorMsg != "" {
		if out.ErrorMsg != "" {
			return nil, fmt.Errorf("place order: %s", out.ErrorMsg)
		}
		return nil, fmt.Errorf("place order: rejected (success=false)")
	}

	return &out, nil
}

func (c *Client) CancelOrder(orderID string) error {
	body, err := jsonMarshalFn(models.CancelOrderRequest{OrderID: orderID})
	if err != nil {
		return fmt.Errorf("marshal cancel order: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodDelete, c.baseURL+"/order", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	c.addAuthHeaders(httpReq, string(body))

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return fmt.Errorf("cancel order: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errUnexpectedHTTP("cancel order", resp)
	}

	var out models.CancelOrderResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return fmt.Errorf("decode cancel order response: %w", err)
	}

	if reason, ok := out.NotCanceled[orderID]; ok {
		return fmt.Errorf("cancel order: %s", reason)
	}

	for _, id := range out.Canceled {
		if id == orderID {
			return nil
		}
	}

	return fmt.Errorf("cancel order: order %q not listed in canceled", orderID)
}
