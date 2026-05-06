package gamma

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func assertQueryValue(t *testing.T, q map[string][]string, key string, want string) {
	t.Helper()
	if got := q[key][0]; got != want {
		t.Fatalf("unexpected %s: %s", key, got)
	}
}

func assertSportsMarketTypes(t *testing.T, q map[string][]string, want []string) {
	t.Helper()
	got := q["sports_market_types"]
	if len(got) != len(want) {
		t.Fatalf("expected %d sports_market_types values, got %d (%v)", len(want), len(got), got)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected sports_market_types values: %v", got)
		}
	}
}

func TestGetMarketsSuccess(t *testing.T) {
	closed := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/markets" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		q := r.URL.Query()
		if got := q.Get("tag_id"); got != "42" {
			t.Fatalf("unexpected tag_id: %s", got)
		}
		if got := q.Get("closed"); got != "false" {
			t.Fatalf("unexpected closed: %s", got)
		}
		if got := q.Get("limit"); got != "10" {
			t.Fatalf("unexpected limit: %s", got)
		}
		if got := q.Get("offset"); got != "20" {
			t.Fatalf("unexpected offset: %s", got)
		}
		if got := q.Get("sports_market_types"); got != "moneyline" {
			t.Fatalf("unexpected sports_market_types: %s", got)
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"conditionId":"c1","slug":"atp-foo","question":"Q?","category":"sports","endDate":"2026-01-01T00:00:00Z","active":true,"closed":false,"archived":false,"acceptingOrders":true,"enableOrderBook":true,"clobTokenIds":"[]","outcomePrices":"[]","outcomes":"[]","liquidityNum":"0","volumeNum":"0","lastTradePrice":"0","bestBid":"0","bestAsk":"0","orderMinSize":1,"orderPriceMinTickSize":"0.01","makerBaseFee":0,"takerBaseFee":0,"gameStartTime":"","tags":[]}]`))
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL))
	markets, err := c.GetMarkets(context.Background(), MarketsParams{
		TagID:             42,
		Closed:            &closed,
		Limit:             10,
		Offset:            20,
		SportsMarketTypes: []string{"moneyline"},
	})
	if err != nil {
		t.Fatalf("GetMarkets returned error: %v", err)
	}
	if len(markets) != 1 || markets[0].ConditionID != "c1" {
		t.Fatalf("unexpected markets: %#v", markets)
	}
}

func TestGetMarketsNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL))
	_, err := c.GetMarkets(context.Background(), MarketsParams{})
	if err == nil || !strings.Contains(err.Error(), "unexpected status 503") {
		t.Fatalf("expected non-200 error, got: %v", err)
	}
}

func TestGetMarketsInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{"))
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL))
	_, err := c.GetMarkets(context.Background(), MarketsParams{})
	if err == nil || !strings.Contains(err.Error(), "decode markets") {
		t.Fatalf("expected decode error, got: %v", err)
	}
}

func TestGetMarketsRequestErrorViaTransport(t *testing.T) {
	c := NewClient(
		WithBaseURL("http://example.com"),
		WithTransport(roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("boom")
		})),
	)

	_, err := c.GetMarkets(context.Background(), MarketsParams{})
	if err == nil || !strings.Contains(err.Error(), "get markets") {
		t.Fatalf("expected wrapped transport error, got: %v", err)
	}
}

func TestGetMarketsInvalidBaseURL(t *testing.T) {
	c := NewClient(WithBaseURL("http://[::1"))

	_, err := c.GetMarkets(context.Background(), MarketsParams{})
	if err == nil || !strings.Contains(err.Error(), "parse url") {
		t.Fatalf("expected parse url error, got: %v", err)
	}
}

func TestGetMarketsNilContext(t *testing.T) {
	c := NewClient(WithBaseURL("http://example.com"))

	_, err := c.GetMarkets(nil, MarketsParams{})
	if err == nil || !strings.Contains(err.Error(), "new request") {
		t.Fatalf("expected new request error, got: %v", err)
	}
}

func TestWithHTTPClientUsesInjectedClient(t *testing.T) {
	usedInjectedClient := false
	injected := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			usedInjectedClient = true
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`[]`)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	c := NewClient(
		WithBaseURL("http://example.com"),
		WithHTTPClient(injected),
	)

	_, err := c.GetMarkets(context.Background(), MarketsParams{})
	if err != nil {
		t.Fatalf("GetMarkets returned error: %v", err)
	}
	if !usedInjectedClient {
		t.Fatal("expected injected http.Client transport to be used")
	}
}

func TestGetMarketsIncludesDatePaginationAndRepeatedSportsQueryParams(t *testing.T) {
	closed := true
	startMin := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	startMax := time.Date(2026, time.January, 3, 3, 4, 5, 0, time.UTC)
	endMin := time.Date(2026, time.February, 2, 3, 4, 5, 0, time.UTC)
	endMax := time.Date(2026, time.February, 3, 3, 4, 5, 0, time.UTC)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		assertQueryValue(t, q, "closed", "true")
		assertQueryValue(t, q, "limit", "25")
		assertQueryValue(t, q, "offset", "5")
		assertQueryValue(t, q, "start_date_min", startMin.Format(time.RFC3339))
		assertQueryValue(t, q, "start_date_max", startMax.Format(time.RFC3339))
		assertQueryValue(t, q, "end_date_min", endMin.Format(time.RFC3339))
		assertQueryValue(t, q, "end_date_max", endMax.Format(time.RFC3339))
		assertSportsMarketTypes(t, q, []string{"moneyline", "spread", "moneyline"})

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL))
	_, err := c.GetMarkets(context.Background(), MarketsParams{
		Closed:            &closed,
		Limit:             25,
		Offset:            5,
		SportsMarketTypes: []string{"moneyline", "spread", "moneyline"},
		StartDateMin:      &startMin,
		StartDateMax:      &startMax,
		EndDateMin:        &endMin,
		EndDateMax:        &endMax,
	})
	if err != nil {
		t.Fatalf("GetMarkets returned error: %v", err)
	}
}

