package clob

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
