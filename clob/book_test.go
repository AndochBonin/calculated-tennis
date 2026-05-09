package clob

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetOrderBookSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/book" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("token_id"); got != "token-1" {
			t.Fatalf("unexpected token_id: %s", got)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"market":"m1","asset_id":"token-1","timestamp":"ts","hash":"h","bids":[],"asks":[],"min_order_size":"0.01","tick_size":"0.01","neg_risk":false,"last_trade_price":"0.52"}`))
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL))
	book, err := c.GetOrderBook("token-1")
	if err != nil {
		t.Fatalf("GetOrderBook returned error: %v", err)
	}
	if book == nil || book.AssetID != "token-1" {
		t.Fatalf("unexpected book: %#v", book)
	}
}

func TestGetOrderBookNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL))
	_, err := c.GetOrderBook("token-1")
	if err == nil || !strings.Contains(err.Error(), "unexpected status 502") {
		t.Fatalf("expected non-200 error, got: %v", err)
	}
}

func TestGetOrderBookInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{"))
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL))
	_, err := c.GetOrderBook("token-1")
	if err == nil || !strings.Contains(err.Error(), "decode order book") {
		t.Fatalf("expected decode error, got: %v", err)
	}
}

func TestGetOrderBookRequestErrorViaTransport(t *testing.T) {
	c := NewClient(
		WithBaseURL("http://example.com"),
		WithTransport(roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("boom")
		})),
	)

	_, err := c.GetOrderBook("token-1")
	if err == nil || !strings.Contains(err.Error(), "get order book") {
		t.Fatalf("expected wrapped transport error, got: %v", err)
	}
}

func TestGetOrderBookParseURLError(t *testing.T) {
	c := NewClient(WithBaseURL("http://example.com/%zz"))

	_, err := c.GetOrderBook("token-1")
	if err == nil || !strings.Contains(err.Error(), "parse url") {
		t.Fatalf("expected parse url error, got: %v", err)
	}
}

func TestWithHTTPClientUsesInjectedClient(t *testing.T) {
	usedInjectedClient := false
	injected := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			usedInjectedClient = true
			return &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(
					`{"market":"m1","asset_id":"token-1","timestamp":"ts","hash":"h","bids":[],"asks":[],"min_order_size":"0.01","tick_size":"0.01","neg_risk":false,"last_trade_price":"0.52"}`,
				)),
				Header: make(http.Header),
			}, nil
		}),
	}

	c := NewClient(
		WithBaseURL("http://example.com"),
		WithHTTPClient(injected),
	)

	_, err := c.GetOrderBook("token-1")
	if err != nil {
		t.Fatalf("GetOrderBook returned error: %v", err)
	}
	if !usedInjectedClient {
		t.Fatal("expected injected http.Client transport to be used")
	}
}
