package clob

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetTradesSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/data/trades" || r.Method != http.MethodGet {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"limit":0,"next_cursor":"","count":0,"data":[]}`))
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL))
	trades, err := c.GetTrades()
	if err != nil {
		t.Fatalf("GetTrades: %v", err)
	}
	if trades == nil {
		t.Fatal("expected non-nil trades")
	}
}

func TestGetTradesNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL))
	_, err := c.GetTrades()
	if err == nil || !strings.Contains(err.Error(), "unexpected status 502") {
		t.Fatalf("expected non-200 error, got: %v", err)
	}
}

func TestGetTradesInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{"))
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL))
	_, err := c.GetTrades()
	if err == nil || !strings.Contains(err.Error(), "decode trades") {
		t.Fatalf("expected decode error, got: %v", err)
	}
}

func TestGetTradesRequestErrorViaTransport(t *testing.T) {
	c := NewClient(
		WithBaseURL("http://example.com"),
		WithTransport(roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("boom")
		})),
	)

	_, err := c.GetTrades()
	if err == nil || !strings.Contains(err.Error(), "get trades") {
		t.Fatalf("expected wrapped transport error, got: %v", err)
	}
}

func TestGetTradesCreateRequestError(t *testing.T) {
	c := NewClient(WithBaseURL("http://example.com/%zz"))

	_, err := c.GetTrades()
	if err == nil || !strings.Contains(err.Error(), "create request") {
		t.Fatalf("expected create request error, got: %v", err)
	}
}
