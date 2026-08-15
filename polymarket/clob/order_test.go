package clob

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mmclient "github.com/AndochBonin/calculated-tennis/moneymanager/pkg/client"
	"github.com/AndochBonin/calculated-tennis/moneymanager/pkg/testserver"
	"github.com/AndochBonin/calculated-tennis/polymarket/models"
	"github.com/shopspring/decimal"
)

func TestGetOrdersSetsAuthHeaders(t *testing.T) {
	t.Setenv("POLYMARKET_API_KEY", "my-key")
	t.Setenv("POLYMARKET_API_SECRET", "AQIDBA==")
	t.Setenv("POLYMARKET_PASSPHRASE", "my-pass")
	t.Setenv("POLYMARKET_ADDRESS", "0xabc")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method: got %s want GET", r.Method)
		}
		if r.URL.Path != "/data/orders" {
			t.Errorf("path: got %s want /data/orders", r.URL.Path)
		}
		for _, h := range []string{"POLY_ADDRESS", "POLY_API_KEY", "POLY_PASSPHRASE", "POLY_SIGNATURE", "POLY_TIMESTAMP"} {
			if r.Header.Get(h) == "" {
				t.Errorf("missing header %s", h)
			}
		}
		if r.Header.Get("POLY_ADDRESS") != "0xabc" {
			t.Errorf("POLY_ADDRESS: got %q", r.Header.Get("POLY_ADDRESS"))
		}
		if r.Header.Get("POLY_API_KEY") != "my-key" {
			t.Errorf("POLY_API_KEY: got %q", r.Header.Get("POLY_API_KEY"))
		}
		if r.Header.Get("POLY_PASSPHRASE") != "my-pass" {
			t.Errorf("POLY_PASSPHRASE: got %q", r.Header.Get("POLY_PASSPHRASE"))
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"limit":0,"next_cursor":"","count":0,"data":[]}`))
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL))
	_, err := c.GetOrders()
	if err != nil {
		t.Fatalf("GetOrders: %v", err)
	}
}

func TestGetOrdersAuthHeadersEmptySignatureWhenInvalidSecret(t *testing.T) {
	t.Setenv("POLYMARKET_API_KEY", "k")
	t.Setenv("POLYMARKET_API_SECRET", "@@@")
	t.Setenv("POLYMARKET_PASSPHRASE", "p")
	t.Setenv("POLYMARKET_ADDRESS", "0x1")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("POLY_SIGNATURE") != "" {
			t.Errorf("expected empty POLY_SIGNATURE for invalid base64 secret, got %q", r.Header.Get("POLY_SIGNATURE"))
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"limit":0,"next_cursor":"","count":0,"data":[]}`))
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL))
	if _, err := c.GetOrders(); err != nil {
		t.Fatalf("GetOrders: %v", err)
	}
}

func TestGetOrdersSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/data/orders" || r.Method != http.MethodGet {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"limit":10,"next_cursor":"","count":0,"data":[]}`))
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL))
	orders, err := c.GetOrders()
	if err != nil {
		t.Fatalf("GetOrders: %v", err)
	}
	if orders == nil || orders.Limit != 10 {
		t.Fatalf("unexpected orders: %#v", orders)
	}
}

func TestGetOrdersNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL))
	_, err := c.GetOrders()
	if err == nil || !strings.Contains(err.Error(), "unexpected status 502") {
		t.Fatalf("expected non-200 error, got: %v", err)
	}
}

func TestGetOrdersInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{"))
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL))
	_, err := c.GetOrders()
	if err == nil || !strings.Contains(err.Error(), "decode orders") {
		t.Fatalf("expected decode error, got: %v", err)
	}
}

func TestGetOrdersRequestErrorViaTransport(t *testing.T) {
	c := NewClient(
		WithBaseURL("http://example.com"),
		WithTransport(roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("boom")
		})),
	)

	_, err := c.GetOrders()
	if err == nil || !strings.Contains(err.Error(), "get orders") {
		t.Fatalf("expected wrapped transport error, got: %v", err)
	}
}

func TestGetOrdersCreateRequestError(t *testing.T) {
	c := NewClient(WithBaseURL("http://example.com/%zz"))

	_, err := c.GetOrders()
	if err == nil || !strings.Contains(err.Error(), "create request") {
		t.Fatalf("expected create request error, got: %v", err)
	}
}

func TestClobSignatureTypeFromEnv_Table(t *testing.T) {
	tests := []struct {
		name      string
		envVal    string
		want      uint8
		wantErr   string
		unsetEnv  bool
	}{
		{name: "unset_defaults_to_3", unsetEnv: true, want: 3},
		{name: "empty_defaults_to_3", envVal: "", want: 3},
		{name: "whitespace_defaults_to_3", envVal: "   \t", want: 3},
		{name: "zero", envVal: "0", want: 0},
		{name: "one", envVal: "1", want: 1},
		{name: "two", envVal: "2", want: 2},
		{name: "three", envVal: "3", want: 3},
		{name: "trimmed_three", envVal: "  3  ", want: 3},
		{name: "invalid_non_numeric", envVal: "abc", wantErr: "POLYMARKET_CLOB_SIGNATURE_TYPE"},
		{name: "invalid_float", envVal: "1.5", wantErr: "POLYMARKET_CLOB_SIGNATURE_TYPE"},
		{name: "invalid_negative", envVal: "-1", wantErr: "must be 0..3"},
		{name: "invalid_four", envVal: "4", wantErr: "must be 0..3"},
		{name: "invalid_large", envVal: "99", wantErr: "must be 0..3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.unsetEnv {
				t.Setenv("POLYMARKET_CLOB_SIGNATURE_TYPE", "")
			} else {
				t.Setenv("POLYMARKET_CLOB_SIGNATURE_TYPE", tt.envVal)
			}

			got, err := clobSignatureTypeFromEnv()
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("want error containing %q, got err=%v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %d want %d", got, tt.want)
			}
		})
	}
}

// Well-known Anvil/Hardhat default account #0 private key (tests only).
const testBuildLimitOrderPrivateKeyHex = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"

func startTestMoneyManagerGRPC(t *testing.T, deposit string) *mmclient.Client {
	t.Helper()
	addr, cleanup, err := testserver.Start(testserver.Config{
		PrivateKeyHex:        testBuildLimitOrderPrivateKeyHex,
		DefaultDepositWallet: deposit,
		DefaultSignatureType: 3,
	})
	if err != nil {
		t.Fatalf("testserver.Start: %v", err)
	}
	t.Cleanup(cleanup)

	c, err := mmclient.Dial(context.Background(), addr)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestBuildLimitOrder_ErrNoDepositWallet(t *testing.T) {
	t.Setenv("POLYMARKET_DEPOSIT_WALLET", "")
	t.Setenv("DEPOSIT_WALLET", "")

	mm := startTestMoneyManagerGRPC(t, "0x1111111111111111111111111111111111111111")
	c := NewClient(WithBaseURL("http://unused.example"))
	price := decimal.RequireFromString("0.5")
	size := decimal.RequireFromString("1")
	_, err := c.BuildLimitOrder(context.Background(), mm, "1", models.OrderSideBuy, price, size, false, 0)
	if err == nil || !strings.Contains(err.Error(), "deposit wallet not configured") {
		t.Fatalf("expected deposit wallet error, got: %v", err)
	}
}

func TestBuildLimitOrder_ErrInvalidSignatureTypeEnv(t *testing.T) {
	t.Setenv("POLYMARKET_CLOB_SIGNATURE_TYPE", "not-a-number")
	t.Setenv("POLYMARKET_DEPOSIT_WALLET", "")

	mm := startTestMoneyManagerGRPC(t, "0x1111111111111111111111111111111111111111")
	deposit := "0x1111111111111111111111111111111111111111"
	c := NewClient(WithDepositWallet(deposit))
	price := decimal.RequireFromString("0.5")
	size := decimal.RequireFromString("1")
	_, err := c.BuildLimitOrder(context.Background(), mm, "1", models.OrderSideBuy, price, size, false, 0)
	if err == nil || !strings.Contains(err.Error(), "POLYMARKET_CLOB_SIGNATURE_TYPE") {
		t.Fatalf("expected signature type env error, got: %v", err)
	}
}

func TestBuildLimitOrder_SuccessBuyAndSell(t *testing.T) {
	t.Setenv("POLYMARKET_CLOB_SIGNATURE_TYPE", "")
	t.Setenv("POLYMARKET_DEPOSIT_WALLET", "")

	deposit := "0x1111111111111111111111111111111111111111"
	mm := startTestMoneyManagerGRPC(t, deposit)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/time" {
			t.Fatalf("path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("1730000999"))
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL), WithServerSignedTime(true), WithDepositWallet(deposit))
	const exp int64 = 1800000000
	price := decimal.RequireFromString("0.55")
	size := decimal.RequireFromString("10")
	tokenID := "123456789"
	ctx := context.Background()

	t.Run("buy", func(t *testing.T) {
		p, err := c.BuildLimitOrder(ctx, mm, tokenID, models.OrderSideBuy, price, size, false, exp)
		if err != nil {
			t.Fatalf("BuildLimitOrder: %v", err)
		}
		if p == nil {
			t.Fatal("nil payload")
		}
		if p.Side != models.OrderSideBuy {
			t.Fatalf("Side: got %q", p.Side)
		}
		if !strings.EqualFold(p.Maker, deposit) || !strings.EqualFold(p.Signer, deposit) {
			t.Fatalf("maker/signer: got maker=%q signer=%q want %q", p.Maker, p.Signer, deposit)
		}
		if p.TokenID != tokenID {
			t.Fatalf("TokenID: got %q", p.TokenID)
		}
		if p.Expiration != "1800000000" || p.Timestamp != "1730000999000" {
			t.Fatalf("Expiration/Timestamp: got exp=%q ts=%q", p.Expiration, p.Timestamp)
		}
		if p.SignatureType != 3 {
			t.Fatalf("SignatureType: got %d want 3", p.SignatureType)
		}
		if !strings.HasPrefix(p.Signature, "0x") || len(p.Signature) < 10 {
			t.Fatalf("Signature: got %q", p.Signature)
		}
		makerAmt, takerAmt := p.MakerAmount, p.TakerAmount
		if makerAmt == "" || takerAmt == "" {
			t.Fatalf("amounts empty: maker=%q taker=%q", makerAmt, takerAmt)
		}
	})

	t.Run("sell", func(t *testing.T) {
		p, err := c.BuildLimitOrder(ctx, mm, tokenID, models.OrderSideSell, price, size, true, exp)
		if err != nil {
			t.Fatalf("BuildLimitOrder: %v", err)
		}
		if p.Side != models.OrderSideSell {
			t.Fatalf("Side: got %q", p.Side)
		}
		if !strings.HasPrefix(p.Signature, "0x") {
			t.Fatalf("Signature: got %q", p.Signature)
		}
	})
}

func placeOrderTestEnv(t *testing.T) {
	t.Helper()
	t.Setenv("POLYMARKET_API_KEY", "k")
	t.Setenv("POLYMARKET_API_SECRET", "AQIDBA==")
	t.Setenv("POLYMARKET_PASSPHRASE", "p")
	t.Setenv("POLYMARKET_ADDRESS", "0x1")
}

func minimalPlaceOrderPayload() *models.OrderPayload {
	return &models.OrderPayload{
		Maker:         "0xmaker",
		Signer:        "0xsigner",
		TokenID:       "token1",
		MakerAmount:   "1",
		TakerAmount:   "2",
		Side:          models.OrderSideBuy,
		Expiration:    "0",
		Timestamp:     "123",
		Signature:     "sig",
		Salt:          1,
		SignatureType: 0,
	}
}

func newPlaceOrderTestClient(t *testing.T, orderHandler http.HandlerFunc) *Client {
	t.Helper()
	placeOrderTestEnv(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/time", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("1730000999"))
	})
	mux.HandleFunc("/order", orderHandler)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return NewClient(WithBaseURL(srv.URL), WithServerSignedTime(true))
}

func TestPlaceOrder_HTTPMatrix(t *testing.T) {
	payload := minimalPlaceOrderPayload()

	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantErrSub string
	}{
		{
			name: "non_200",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadGateway)
				_, _ = w.Write([]byte("upstream"))
			},
			wantErrSub: "unexpected status 502",
		},
		{
			name: "invalid_json_body",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("{"))
			},
			wantErrSub: "decode place order response",
		},
		{
			name: "logical_failure_with_error_msg",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"success":false,"errorMsg":"something broke"}`))
			},
			wantErrSub: "something broke",
		},
		{
			name: "logical_failure_empty_error_msg",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"success":false}`))
			},
			wantErrSub: "rejected (success=false)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newPlaceOrderTestClient(t, tt.handler)
			_, err := c.PlaceOrder(payload, "owner-1", models.OrderTypeGTC)
			if err == nil || !strings.Contains(err.Error(), tt.wantErrSub) {
				t.Fatalf("want error containing %q, got: %v", tt.wantErrSub, err)
			}
		})
	}
}

func TestPlaceOrder_TransportError(t *testing.T) {
	placeOrderTestEnv(t)
	c := NewClient(
		WithBaseURL("http://example.com"),
		WithTransport(roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("boom")
		})),
	)
	_, err := c.PlaceOrder(minimalPlaceOrderPayload(), "owner", models.OrderTypeGTC)
	if err == nil || !strings.Contains(err.Error(), "place order") {
		t.Fatalf("expected wrapped transport error, got: %v", err)
	}
}

func TestPlaceOrder_CreateRequestError(t *testing.T) {
	placeOrderTestEnv(t)
	c := NewClient(WithBaseURL("http://example.com/%zz"))
	_, err := c.PlaceOrder(minimalPlaceOrderPayload(), "owner", models.OrderTypeGTC)
	if err == nil || !strings.Contains(err.Error(), "create request") {
		t.Fatalf("expected create request error, got: %v", err)
	}
}

func TestPlaceOrder_JSONMarshalError(t *testing.T) {
	prev := jsonMarshalFn
	jsonMarshalFn = func(any) ([]byte, error) {
		return nil, errors.New("marshal boom")
	}
	t.Cleanup(func() { jsonMarshalFn = prev })

	placeOrderTestEnv(t)
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	t.Cleanup(srv.Close)

	c := NewClient(WithBaseURL(srv.URL))
	_, err := c.PlaceOrder(minimalPlaceOrderPayload(), "owner", models.OrderTypeGTC)
	if err == nil || !strings.Contains(err.Error(), "marshal order") || !strings.Contains(err.Error(), "marshal boom") {
		t.Fatalf("PlaceOrder: got err=%v", err)
	}
}

func TestPlaceOrder_ownerFallsBackToTrimmedAPIKeyWhenOwnerWhitespaceOnly(t *testing.T) {
	payload := minimalPlaceOrderPayload()

	c := newPlaceOrderTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		var got models.PlaceOrderRequest
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if got.Owner != "k" {
			t.Fatalf("Owner: got %q want trimmed POLYMARKET_API_KEY", got.Owner)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true}`))
	})

	out, err := c.PlaceOrder(payload, "   \t", models.OrderTypeGTC)
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}
	if !out.Success {
		t.Fatalf("response: %+v", out)
	}
}

func TestPlaceOrder_SuccessViaMatrixHarness(t *testing.T) {
	payload := minimalPlaceOrderPayload()
	wantReq := models.PlaceOrderRequest{
		Order:     *payload,
		Owner:     "explicit-owner",
		OrderType: models.OrderTypeGTC,
		PostOnly:  false,
		DeferExec: false,
	}

	c := newPlaceOrderTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("want POST, got %s", r.Method)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		var got models.PlaceOrderRequest
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if got != wantReq {
			t.Fatalf("request: got %+v want %+v", got, wantReq)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true,"orderID":"ord-m","status":"live"}`))
	})

	out, err := c.PlaceOrder(payload, "explicit-owner", models.OrderTypeGTC)
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}
	if !out.Success || out.OrderID != "ord-m" {
		t.Fatalf("response: %+v", out)
	}
}

func cancelOrderTestEnv(t *testing.T) {
	t.Helper()
	t.Setenv("POLYMARKET_API_KEY", "k")
	t.Setenv("POLYMARKET_API_SECRET", "AQIDBA==")
	t.Setenv("POLYMARKET_PASSPHRASE", "p")
	t.Setenv("POLYMARKET_ADDRESS", "0x1")
}

func newCancelOrderTestClient(t *testing.T, orderHandler http.HandlerFunc) *Client {
	t.Helper()
	cancelOrderTestEnv(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/time", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("1730000999"))
	})
	mux.HandleFunc("/order", orderHandler)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return NewClient(WithBaseURL(srv.URL), WithServerSignedTime(true))
}

func TestCancelOrder_HTTPMatrix(t *testing.T) {
	const orderID = "cancel-target-1"

	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantErrSub string
	}{
		{
			name: "non_200",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadGateway)
			},
			wantErrSub: "unexpected status 502",
		},
		{
			name: "invalid_json_body",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("not-json"))
			},
			wantErrSub: "decode cancel order response",
		},
		{
			name: "not_canceled_reason",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"canceled":[],"not_canceled":{"` + orderID + `":"already filled"}}`))
			},
			wantErrSub: "already filled",
		},
		{
			name: "missing_from_canceled_list",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"canceled":["other-id"],"not_canceled":{}}`))
			},
			wantErrSub: "not listed in canceled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCancelOrderTestClient(t, tt.handler)
			err := c.CancelOrder(orderID)
			if err == nil || !strings.Contains(err.Error(), tt.wantErrSub) {
				t.Fatalf("want error containing %q, got: %v", tt.wantErrSub, err)
			}
		})
	}
}

func TestCancelOrder_TransportError(t *testing.T) {
	cancelOrderTestEnv(t)
	c := NewClient(
		WithBaseURL("http://example.com"),
		WithTransport(roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("boom")
		})),
	)
	err := c.CancelOrder("any")
	if err == nil || !strings.Contains(err.Error(), "cancel order") {
		t.Fatalf("expected wrapped transport error, got: %v", err)
	}
}

func TestCancelOrder_CreateRequestError(t *testing.T) {
	cancelOrderTestEnv(t)
	c := NewClient(WithBaseURL("http://example.com/%zz"))
	err := c.CancelOrder("any")
	if err == nil || !strings.Contains(err.Error(), "create request") {
		t.Fatalf("expected create request error, got: %v", err)
	}
}

func TestCancelOrder_JSONMarshalError(t *testing.T) {
	prev := jsonMarshalFn
	jsonMarshalFn = func(any) ([]byte, error) {
		return nil, errors.New("marshal cancel boom")
	}
	t.Cleanup(func() { jsonMarshalFn = prev })

	cancelOrderTestEnv(t)
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	t.Cleanup(srv.Close)

	c := NewClient(WithBaseURL(srv.URL))
	err := c.CancelOrder("order-1")
	if err == nil || !strings.Contains(err.Error(), "marshal cancel order") || !strings.Contains(err.Error(), "marshal cancel boom") {
		t.Fatalf("CancelOrder: got err=%v", err)
	}
}

func TestCancelOrder_SuccessViaMatrixHarness(t *testing.T) {
	const orderID = "cancel-ok-1"
	c := newCancelOrderTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("want DELETE, got %s", r.Method)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		var req models.CancelOrderRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if req.OrderID != orderID {
			t.Fatalf("orderID: got %q", req.OrderID)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"canceled":["` + orderID + `"],"not_canceled":{}}`))
	})

	if err := c.CancelOrder(orderID); err != nil {
		t.Fatalf("CancelOrder: %v", err)
	}
}
