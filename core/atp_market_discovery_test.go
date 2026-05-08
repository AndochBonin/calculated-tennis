package core

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/AndochBonin/polymarket/gamma"
	"github.com/AndochBonin/polymarket/models"
)

func discoveryMarketJSON(slug, conditionID, contextDescription string) string {
	return `[{
		"conditionId":"` + conditionID + `",
		"slug":"` + slug + `",
		"question":"Will player win?",
		"category":"sports",
		"endDate":"2026-01-01T00:00:00Z",
		"active":true,
		"closed":false,
		"archived":false,
		"acceptingOrders":true,
		"enableOrderBook":true,
		"clobTokenIds":"[\"yes\",\"no\"]",
		"outcomePrices":"[]",
		"outcomes":"[\"YES\",\"NO\"]",
		"liquidityNum":"0",
		"volumeNum":"0",
		"lastTradePrice":"0",
		"bestBid":"0",
		"bestAsk":"0",
		"orderMinSize":1,
		"orderPriceMinTickSize":"0.01",
		"makerBaseFee":0,
		"takerBaseFee":0,
		"gameStartTime":"",
		"events":[{"gameId":123,"eventMetadata":{"context_description":"` + contextDescription + `","league":"ATP Tour"}}],
		"tags":[]
	}]`
}

func TestATPMarketDiscoveryScreenAndHydrateRetry(t *testing.T) {
	t.Parallel()

	const slug = "atp-rome-open"
	const conditionID = "cond-1"
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if got := r.URL.Query().Get("slug"); got != slug {
			t.Fatalf("unexpected slug query: %q", got)
		}
		if got := r.URL.Query().Get("closed"); got != "false" {
			t.Fatalf("unexpected closed query: %q", got)
		}
		w.WriteHeader(http.StatusOK)
		if calls == 1 {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		_, _ = w.Write([]byte(discoveryMarketJSON(slug, conditionID, "ATP Masters match")))
	}))
	defer srv.Close()
	gammaClient := gamma.NewClient(gamma.WithBaseURL(srv.URL))

	var startCalls int
	d := NewATPMarketDiscovery(gammaClient, func(_ context.Context, market models.GammaMarket) error {
		startCalls++
		if market.ConditionID != conditionID {
			t.Fatalf("unexpected market passed to starter: %+v", market)
		}
		return nil
	}, nil)
	d.retryDelay = 1 * time.Millisecond

	d.HandleNewMarket(context.Background(), models.NewMarketEvent{
		Slug:             slug,
		SportsMarketType: "moneyline",
		ConditionID:      conditionID,
	})

	if calls != 2 {
		t.Fatalf("expected 2 Gamma calls (empty + retry), got %d", calls)
	}
	if startCalls != 1 {
		t.Fatalf("expected exactly one trader start call, got %d", startCalls)
	}
}

func TestATPMarketDiscoverySkipsNonATPOrNonMoneyline(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("GetMarkets should not be called for filtered events")
	}))
	defer srv.Close()
	gammaClient := gamma.NewClient(gamma.WithBaseURL(srv.URL))

	d := NewATPMarketDiscovery(gammaClient, func(context.Context, models.GammaMarket) error {
		t.Fatal("starter should not be called")
		return nil
	}, nil)

	d.HandleNewMarket(context.Background(), models.NewMarketEvent{
		Slug:             "nba-finals-market",
		SportsMarketType: "moneyline",
	})
	d.HandleNewMarket(context.Background(), models.NewMarketEvent{
		Slug:             "atp-rome-open",
		SportsMarketType: "spread",
	})

}

func TestATPMarketDiscoveryFiltersOutRejectedMarkets(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(discoveryMarketJSON("atp-challenger-wuxi", "cond-2", "ATP Challenger event")))
	}))
	defer srv.Close()
	gammaClient := gamma.NewClient(gamma.WithBaseURL(srv.URL))

	d := NewATPMarketDiscovery(gammaClient, func(context.Context, models.GammaMarket) error {
		t.Fatal("starter should not be called for filtered-out market")
		return nil
	}, nil)
	d.retryDelay = 1 * time.Millisecond

	d.HandleNewMarket(context.Background(), models.NewMarketEvent{
		Slug:             "atp-challenger-wuxi",
		SportsMarketType: "moneyline",
		ConditionID:      "cond-2",
	})
}

func TestATPMarketDiscoveryDedupesConditionID(t *testing.T) {
	t.Parallel()

	const slug = "atp-miami-open"
	const conditionID = "cond-3"
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(discoveryMarketJSON(slug, conditionID, "ATP ATP ATP")))
	}))
	defer srv.Close()
	gammaClient := gamma.NewClient(gamma.WithBaseURL(srv.URL))

	var startCalls int
	d := NewATPMarketDiscovery(gammaClient, func(context.Context, models.GammaMarket) error {
		startCalls++
		return nil
	}, nil)
	d.retryDelay = 1 * time.Millisecond

	ev := models.NewMarketEvent{
		Slug:             slug,
		SportsMarketType: "moneyline",
		ConditionID:      conditionID,
	}
	d.HandleNewMarket(context.Background(), ev)
	d.HandleNewMarket(context.Background(), ev)

	if startCalls != 1 {
		t.Fatalf("expected one start call after duplicate events, got %d", startCalls)
	}
	if calls != 2 {
		t.Fatalf("expected GetMarkets to be called for both events before dedupe mark, got %d", calls)
	}
}

func TestATPMarketDiscoveryUnmarksOnStartError(t *testing.T) {
	t.Parallel()

	const slug = "atp-paris-open"
	const conditionID = "cond-4"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(discoveryMarketJSON(slug, conditionID, "ATP Masters")))
	}))
	defer srv.Close()
	gammaClient := gamma.NewClient(gamma.WithBaseURL(srv.URL))

	var startCalls int
	d := NewATPMarketDiscovery(gammaClient, func(context.Context, models.GammaMarket) error {
		startCalls++
		if startCalls == 1 {
			return errors.New("boom")
		}
		return nil
	}, nil)
	d.retryDelay = 1 * time.Millisecond

	ev := models.NewMarketEvent{
		Slug:             slug,
		SportsMarketType: "moneyline",
		ConditionID:      conditionID,
	}
	d.HandleNewMarket(context.Background(), ev)
	d.HandleNewMarket(context.Background(), ev)

	if startCalls != 2 {
		t.Fatalf("expected two start attempts after first failure, got %d", startCalls)
	}
}

func TestATPMarketDiscoveryHandlesGammaError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	gammaClient := gamma.NewClient(gamma.WithBaseURL(srv.URL))

	d := NewATPMarketDiscovery(gammaClient, func(context.Context, models.GammaMarket) error {
		t.Fatal("starter should not be called when gamma request fails")
		return nil
	}, nil)
	d.retryDelay = 1 * time.Millisecond

	d.HandleNewMarket(context.Background(), models.NewMarketEvent{
		Slug:             "atp-doha-open",
		SportsMarketType: "moneyline",
		ConditionID:      "cond-x",
	})
}

func TestATPMarketDiscoverySeedsConditionIDDedupe(t *testing.T) {
	t.Parallel()

	const slug = "atp-barcelona-open"
	const conditionID = "seeded-cond"
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if got := r.URL.Query().Get("slug"); got != slug {
			t.Fatalf("unexpected slug query: %q", got)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(discoveryMarketJSON(slug, conditionID, "ATP seeded market")))
	}))
	defer srv.Close()
	gammaClient := gamma.NewClient(gamma.WithBaseURL(srv.URL))

	d := NewATPMarketDiscovery(gammaClient, func(context.Context, models.GammaMarket) error {
		t.Fatal("starter should not be called for seeded condition ID")
		return nil
	}, []string{conditionID})
	d.retryDelay = 1 * time.Millisecond

	d.HandleNewMarket(context.Background(), models.NewMarketEvent{
		Slug:             slug,
		SportsMarketType: "moneyline",
		ConditionID:      conditionID,
	})
	if calls != 1 {
		t.Fatalf("expected one hydrate call even when seeded, got %d", calls)
	}
}

func TestATPMarketDiscoveryNormalizesMultipleMarketsByConditionID(t *testing.T) {
	t.Parallel()

	const slug = "atp-monte-carlo-open"
	const wantConditionID = "cond-b"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[
			{
				"conditionId":"cond-a","slug":"atp-monte-carlo-open","question":"A?","category":"sports",
				"endDate":"2026-01-01T00:00:00Z","active":true,"closed":false,"archived":false,"acceptingOrders":true,
				"enableOrderBook":true,"clobTokenIds":"[\"yes\",\"no\"]","outcomePrices":"[]","outcomes":"[\"YES\",\"NO\"]",
				"liquidityNum":"0","volumeNum":"0","lastTradePrice":"0","bestBid":"0","bestAsk":"0","orderMinSize":1,
				"orderPriceMinTickSize":"0.01","makerBaseFee":0,"takerBaseFee":0,"gameStartTime":"",
				"events":[{"event":{"series":"ATP Tour","title":"Match A","slug":"a"},"contextDescription":"ATP singles"}],
				"tags":[]
			},
			{
				"conditionId":"cond-b","slug":"atp-monte-carlo-open","question":"B?","category":"sports",
				"endDate":"2026-01-01T00:00:00Z","active":true,"closed":false,"archived":false,"acceptingOrders":true,
				"enableOrderBook":true,"clobTokenIds":"[\"yes\",\"no\"]","outcomePrices":"[]","outcomes":"[\"YES\",\"NO\"]",
				"liquidityNum":"0","volumeNum":"0","lastTradePrice":"0","bestBid":"0","bestAsk":"0","orderMinSize":1,
				"orderPriceMinTickSize":"0.01","makerBaseFee":0,"takerBaseFee":0,"gameStartTime":"",
				"events":[{"event":{"series":"ATP Tour","title":"Match B","slug":"b"},"contextDescription":"ATP singles"}],
				"tags":[]
			}
		]`))
	}))
	defer srv.Close()
	gammaClient := gamma.NewClient(gamma.WithBaseURL(srv.URL))

	var startedConditionID string
	d := NewATPMarketDiscovery(gammaClient, func(_ context.Context, market models.GammaMarket) error {
		startedConditionID = market.ConditionID
		return nil
	}, nil)
	d.retryDelay = 1 * time.Millisecond

	d.HandleNewMarket(context.Background(), models.NewMarketEvent{
		Slug:             slug,
		SportsMarketType: "moneyline",
		ConditionID:      wantConditionID,
	})
	if startedConditionID != wantConditionID {
		t.Fatalf("expected normalized condition ID %q, got %q", wantConditionID, startedConditionID)
	}
}

func TestATPMarketDiscoveryNoConditionIDRequiresSingleResult(t *testing.T) {
	t.Parallel()

	slug := "atp-geneva-open"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[
			{
				"conditionId":"cond-1","slug":"atp-geneva-open","question":"A?","category":"sports",
				"endDate":"2026-01-01T00:00:00Z","active":true,"closed":false,"archived":false,"acceptingOrders":true,
				"enableOrderBook":true,"clobTokenIds":"[\"yes\",\"no\"]","outcomePrices":"[]","outcomes":"[\"YES\",\"NO\"]",
				"liquidityNum":"0","volumeNum":"0","lastTradePrice":"0","bestBid":"0","bestAsk":"0","orderMinSize":1,
				"orderPriceMinTickSize":"0.01","makerBaseFee":0,"takerBaseFee":0,"gameStartTime":"",
				"events":[{"event":{"series":"ATP Tour","title":"Match A","slug":"a"},"contextDescription":"ATP singles"}],
				"tags":[]
			},
			{
				"conditionId":"cond-2","slug":"atp-geneva-open","question":"B?","category":"sports",
				"endDate":"2026-01-01T00:00:00Z","active":true,"closed":false,"archived":false,"acceptingOrders":true,
				"enableOrderBook":true,"clobTokenIds":"[\"yes\",\"no\"]","outcomePrices":"[]","outcomes":"[\"YES\",\"NO\"]",
				"liquidityNum":"0","volumeNum":"0","lastTradePrice":"0","bestBid":"0","bestAsk":"0","orderMinSize":1,
				"orderPriceMinTickSize":"0.01","makerBaseFee":0,"takerBaseFee":0,"gameStartTime":"",
				"events":[{"event":{"series":"ATP Tour","title":"Match B","slug":"b"},"contextDescription":"ATP singles"}],
				"tags":[]
			}
		]`))
	}))
	defer srv.Close()
	gammaClient := gamma.NewClient(gamma.WithBaseURL(srv.URL))

	started := false
	d := NewATPMarketDiscovery(gammaClient, func(context.Context, models.GammaMarket) error {
		started = true
		return nil
	}, nil)
	d.retryDelay = 1 * time.Millisecond

	d.HandleNewMarket(context.Background(), models.NewMarketEvent{
		Slug:             slug,
		SportsMarketType: "moneyline",
	})
	if started {
		t.Fatal("expected no trader start when multiple markets and no condition ID")
	}
}

func TestATPMarketDiscoveryUsesContextCancellationDuringRetry(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()
	gammaClient := gamma.NewClient(gamma.WithBaseURL(srv.URL))

	d := NewATPMarketDiscovery(gammaClient, func(context.Context, models.GammaMarket) error {
		t.Fatal("starter should not be called")
		return nil
	}, nil)
	d.retryDelay = 50 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	d.HandleNewMarket(ctx, models.NewMarketEvent{
		Slug:             "atp-hamburg-open",
		SportsMarketType: "moneyline",
		ConditionID:      "cond-cancel",
	})
}

func TestATPMarketDiscoveryPassesSlugAndClosedQueryToGamma(t *testing.T) {
	t.Parallel()

	const slug = "atp-shanghai-open"
	var observed url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observed = r.URL.Query()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(discoveryMarketJSON(slug, "cond-closed", "ATP")))
	}))
	defer srv.Close()
	gammaClient := gamma.NewClient(gamma.WithBaseURL(srv.URL))

	d := NewATPMarketDiscovery(gammaClient, func(context.Context, models.GammaMarket) error { return nil }, nil)
	d.retryDelay = 1 * time.Millisecond
	d.HandleNewMarket(context.Background(), models.NewMarketEvent{
		Slug:             slug,
		SportsMarketType: "moneyline",
		ConditionID:      "cond-closed",
	})

	if got := observed.Get("slug"); got != slug {
		t.Fatalf("unexpected slug query value: %q", got)
	}
	if got := observed.Get("closed"); got != "false" {
		t.Fatalf("unexpected closed query value: %q", got)
	}
}
