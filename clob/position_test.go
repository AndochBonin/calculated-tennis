package clob

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestGetPositionsSuccess(t *testing.T) {
	user := "0xtestuser"
	c := NewClient(
		WithUserAddress(user),
		WithTransport(roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/positions" || req.Method != http.MethodGet {
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			}
			if req.URL.Query().Get("user") != user {
				t.Fatalf("user query: got %q", req.URL.RawQuery)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`[]`)),
			}, nil
		})),
	)
	pos, err := c.GetPositions()
	if err != nil {
		t.Fatalf("GetPositions: %v", err)
	}
	if pos == nil {
		t.Fatal("expected non-nil slice")
	}
}

func TestGetPositionsNon200(t *testing.T) {
	c := NewClient(
		WithUserAddress("0x1"),
		WithTransport(roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusBadGateway,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("{}")),
			}, nil
		})),
	)
	_, err := c.GetPositions()
	if err == nil || !strings.Contains(err.Error(), "unexpected status 502") {
		t.Fatalf("expected non-200 error, got: %v", err)
	}
}

func TestGetPositionsNon200EmptyBody(t *testing.T) {
	c := NewClient(
		WithTransport(roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusBadGateway,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		})),
	)
	_, err := c.GetPositions()
	if err == nil || err.Error() != "get positions: unexpected status 502" {
		t.Fatalf("expected status-only error, got: %v", err)
	}
}

func TestGetPositionsNon200BodyReadError(t *testing.T) {
	c := NewClient(
		WithTransport(roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Header:     make(http.Header),
				Body:       io.NopCloser(errReader{}),
			}, nil
		})),
	)
	_, err := c.GetPositions()
	if err == nil || err.Error() != "get positions: unexpected status 500" {
		t.Fatalf("expected read-body-fallback error, got: %v", err)
	}
}

func TestGetPositionsInvalidDataAPIBaseURL(t *testing.T) {
	c := NewClient(WithDataAPIBaseURL("http://[::1]:namedport"))
	_, err := c.GetPositions()
	if err == nil || !strings.Contains(err.Error(), "create request") {
		t.Fatalf("expected create request error, got: %v", err)
	}
}

func TestGetPositionsParseJoinedURLFails(t *testing.T) {
	prev := parsePositionsURL
	parsePositionsURL = func(string) (*url.URL, error) {
		return nil, errors.New("parse joined URL")
	}
	t.Cleanup(func() { parsePositionsURL = prev })

	c := NewClient(
		WithDataAPIBaseURL("https://example.com"),
		WithTransport(roundTripFunc(func(*http.Request) (*http.Response, error) {
			t.Fatal("HTTP round trip should not run when URL parse fails")
			return nil, nil
		})),
	)
	_, err := c.GetPositions()
	if err == nil || !strings.Contains(err.Error(), "create request") || !strings.Contains(err.Error(), "parse joined URL") {
		t.Fatalf("expected wrapped parse error, got: %v", err)
	}
}

func TestGetPositionsInvalidJSON(t *testing.T) {
	c := NewClient(
		WithUserAddress("0x1"),
		WithTransport(roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("{")),
			}, nil
		})),
	)
	_, err := c.GetPositions()
	if err == nil || !strings.Contains(err.Error(), "decode positions") {
		t.Fatalf("expected decode error, got: %v", err)
	}
}

func TestGetPositionsRequestErrorViaTransport(t *testing.T) {
	c := NewClient(
		WithBaseURL("http://example.com"),
		WithTransport(roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("boom")
		})),
	)

	_, err := c.GetPositions()
	if err == nil || !strings.Contains(err.Error(), "get positions") {
		t.Fatalf("expected wrapped transport error, got: %v", err)
	}
}

func TestGetPositionsSetsUserQuery(t *testing.T) {
	wantUser := "0xAbC"
	c := NewClient(
		WithUserAddress(wantUser),
		WithTransport(roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/positions" {
				t.Fatalf("path: %s", req.URL.Path)
			}
			if got := req.URL.Query().Get("user"); got != wantUser {
				t.Fatalf("user param: got %q want %q raw=%q", got, wantUser, req.URL.RawQuery)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`[]`)),
			}, nil
		})),
	)
	if _, err := c.GetPositions(); err != nil {
		t.Fatal(err)
	}
}

func TestGetPositionsOmitsUserQueryWhenUnset(t *testing.T) {
	c := NewClient(
		WithTransport(roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/positions" {
				t.Fatalf("path: %s", req.URL.Path)
			}
			if req.URL.Query().Has("user") {
				t.Fatalf("unexpected user query: %q", req.URL.RawQuery)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`[]`)),
			}, nil
		})),
	)
	if _, err := c.GetPositions(); err != nil {
		t.Fatal(err)
	}
}
