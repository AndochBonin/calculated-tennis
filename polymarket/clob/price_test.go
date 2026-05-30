package clob

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetMarketPriceSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/price" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("token_id"); got != "token-2" {
			t.Fatalf("unexpected token_id: %s", got)
		}
		if got := r.URL.Query().Get("side"); got != "buy" {
			t.Fatalf("unexpected side: %s", got)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"price":"0.42"}`))
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL))
	price, err := c.GetMarketPrice("token-2", "buy")
	if err != nil {
		t.Fatalf("GetMarketPrice returned error: %v", err)
	}
	if price == nil || price.Price.String() != "0.42" {
		t.Fatalf("unexpected market price: %#v", price)
	}
}

func TestGetMarketPriceNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL))
	_, err := c.GetMarketPrice("token-2", "buy")
	if err == nil || !strings.Contains(err.Error(), "unexpected status 418") {
		t.Fatalf("expected non-200 error, got: %v", err)
	}
}

func TestGetMarketPriceInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{"))
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL))
	_, err := c.GetMarketPrice("token-2", "buy")
	if err == nil || !strings.Contains(err.Error(), "decode marketPrice") {
		t.Fatalf("expected decode error, got: %v", err)
	}
}

func TestGetMarketPriceRequestErrorViaTransport(t *testing.T) {
	c := NewClient(
		WithBaseURL("http://example.com"),
		WithTransport(roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("boom")
		})),
	)

	_, err := c.GetMarketPrice("token-2", "buy")
	if err == nil || !strings.Contains(err.Error(), "get market price") {
		t.Fatalf("expected wrapped transport error, got: %v", err)
	}
}

func TestGetMarketPriceParseURLError(t *testing.T) {
	c := NewClient(WithBaseURL("http://example.com/%zz"))

	_, err := c.GetMarketPrice("token-2", "buy")
	if err == nil || !strings.Contains(err.Error(), "parse url") {
		t.Fatalf("expected parse url error, got: %v", err)
	}
}
