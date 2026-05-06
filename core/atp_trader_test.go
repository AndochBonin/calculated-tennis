package core

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/AndochBonin/polymarket/models"
	"github.com/gorilla/websocket"
)

func TestATPTraderStopsOnlyMatchingMarketOnResolvedEvent(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	feed := newCategoryFeed(CategoryATP)

	marketA := testMarket(t, "atp-a", "0xabc123", []string{"a_yes", "a_no"})
	marketB := testMarket(t, "atp-b", "0xdef456", []string{"b_yes", "b_no"})

	traderA := NewATPTrader(nil, nil, feed, marketA)
	traderB := NewATPTrader(nil, nil, feed, marketB)

	if err := traderA.Start(ctx); err != nil {
		t.Fatalf("start traderA: %v", err)
	}
	if err := traderB.Start(ctx); err != nil {
		t.Fatalf("start traderB: %v", err)
	}

	resolved := models.MarketResolvedEvent{
		EventType:      "market_resolved",
		Market:         "0xABC123", // uppercase to verify case-insensitive matching
		AssetIDs:       []string{"a_yes", "a_no"},
		WinningAssetID: "a_yes",
		WinningOutcome: "YES",
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
	}
	for _, assetID := range resolved.AssetIDs {
		feed.broadcastTo(assetID, resolved)
	}

	waitClosed(t, traderA.signals, "traderA signals close")

	select {
	case <-traderB.stop:
		t.Fatal("traderB unexpectedly stopped")
	default:
	}

	feed.mu.RLock()
	remainingA := len(feed.subscribers["a_yes"]) + len(feed.subscribers["a_no"])
	remainingB := len(feed.subscribers["b_yes"]) + len(feed.subscribers["b_no"])
	feed.mu.RUnlock()

	if remainingA != 0 {
		t.Fatalf("expected traderA subscriptions removed, got %d listeners", remainingA)
	}
	if remainingB == 0 {
		t.Fatal("expected traderB subscriptions to remain active")
	}

	traderB.Stop()
	waitClosed(t, traderB.signals, "traderB signals close")
}

func TestFilterATPMarkets(t *testing.T) {
	t.Parallel()

	valid := models.GammaMarket{
		EnableOrderBook: true,
		Slug:            "atp-valid",
		ConditionID:     "valid-condition",
		ClobTokenIds:    `["yes-token","no-token"]`,
		Outcomes:        `["YES","NO"]`,
	}

	markets := []models.GammaMarket{
		valid,
		{
			EnableOrderBook: false,
			Slug:            "atp-disabled-orderbook",
			ConditionID:     "disabled",
			ClobTokenIds:    `["a","b"]`,
			Outcomes:        `["YES","NO"]`,
		},
		{
			EnableOrderBook: true,
			Slug:            "nba-playoff-market",
			ConditionID:     "wrong-prefix",
			ClobTokenIds:    `["a","b"]`,
			Outcomes:        `["YES","NO"]`,
		},
		{
			EnableOrderBook: true,
			Slug:            "atp-invalid-token-json",
			ConditionID:     "bad-token-json",
			ClobTokenIds:    `not-json`,
			Outcomes:        `["YES","NO"]`,
		},
		{
			EnableOrderBook: true,
			Slug:            "atp-empty-token-list",
			ConditionID:     "empty-token-list",
			ClobTokenIds:    `[]`,
			Outcomes:        `["YES","NO"]`,
		},
		{
			EnableOrderBook: true,
			Slug:            "atp-invalid-outcomes",
			ConditionID:     "bad-outcomes-json",
			ClobTokenIds:    `["a","b"]`,
			Outcomes:        `oops`,
		},
	}

	got := FilterATPMarkets(markets)
	if len(got) != 1 {
		t.Fatalf("expected exactly one valid market, got %d", len(got))
	}
	if got[0].ConditionID != valid.ConditionID {
		t.Fatalf("expected condition %q, got %q", valid.ConditionID, got[0].ConditionID)
	}
}

func TestATPTraderStartValidationErrors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		clobTokenIDs   string
		outcomes       string
		wantErrSubstrs []string
	}{
		{
			name:           "invalid clob token ids json",
			clobTokenIDs:   `not-json`,
			outcomes:       `["YES","NO"]`,
			wantErrSubstrs: []string{"parse clob token ids", "invalid character"},
		},
		{
			name:           "zero clob tokens",
			clobTokenIDs:   `[]`,
			outcomes:       `["YES","NO"]`,
			wantErrSubstrs: []string{"zero clob tokens"},
		},
		{
			name:           "invalid outcomes json",
			clobTokenIDs:   `["yes-token","no-token"]`,
			outcomes:       `not-json`,
			wantErrSubstrs: []string{"parse outcomes", "invalid character"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			trader := NewATPTrader(nil, nil, newCategoryFeed(CategoryATP), models.GammaMarket{
				EnableOrderBook: true,
				Slug:            "atp-start-error",
				ConditionID:     "0xstart-error",
				Question:        "start error question",
				ClobTokenIds:    tc.clobTokenIDs,
				Outcomes:        tc.outcomes,
			})

			err := trader.Start(context.Background())
			if err == nil {
				t.Fatal("expected start to fail")
			}
			for _, want := range tc.wantErrSubstrs {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("expected error %q to contain %q", err.Error(), want)
				}
			}
		})
	}
}

func TestATPTraderHandleBranchesAndSignals(t *testing.T) {
	t.Parallel()

	trader := NewATPTrader(nil, nil, newCategoryFeed(CategoryATP), models.GammaMarket{
		Slug:        "atp-handle-test",
		ConditionID: "0xabc123",
		Question:    "handle test question",
	})

	if trader.Signals() == nil {
		t.Fatal("expected non-nil signals channel")
	}

	// Price event with non-matching token should continue without panic.
	trader.handle("target-token", "target name", models.PriceEvent{
		EventType: "price_change",
		Market:    "0xabc123",
		PriceChanges: []models.PriceChange{
			{AssetID: "different-token", Side: models.OrderSideBuy, Price: "0.51"},
		},
	})

	// Price event with matching token should execute matching branch.
	trader.handle("target-token", "target name", models.PriceEvent{
		EventType: "price_change",
		Market:    "0xabc123",
		PriceChanges: []models.PriceChange{
			{AssetID: "target-token", Side: models.OrderSideSell, Price: "0.49"},
		},
	})

	trader.handle("target-token", "target name", models.SportEvent{
		Slug:    "atp-handle-test",
		Score:   "1-0",
		Period:  "2",
		Elapsed: "45:00",
		Live:    true,
		Ended:   false,
	})
	trader.handle("target-token", "target name", models.BookEvent{
		EventType: "book",
		Market:    "0xabc123",
	})

	// Non-matching market resolved event should not stop trader.
	trader.handle("target-token", "target name", models.MarketResolvedEvent{
		EventType: "market_resolved",
		Market:    "0xdef456",
	})
	select {
	case <-trader.stop:
		t.Fatal("trader unexpectedly stopped on non-matching market resolution")
	default:
	}

	trader.handle("target-token", "target name", errors.New("test error event"))
	trader.handle("target-token", "target name", struct{ Event string }{Event: "unknown"})

	// Matching market resolved event should trigger Stop.
	trader.handle("target-token", "target name", models.MarketResolvedEvent{
		EventType:      "market_resolved",
		Market:         "0xABC123",
		WinningAssetID: "target-token",
		WinningOutcome: "YES",
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
	})
	waitClosed(t, trader.signals, "trader signals close after resolved event")
}

func TestATPTraderListenExitsOnContextCancelStopAndClosedChannel(t *testing.T) {
	t.Parallel()

	t.Run("context canceled", func(t *testing.T) {
		t.Parallel()

		trader := NewATPTrader(nil, nil, newCategoryFeed(CategoryATP), models.GammaMarket{})
		ctx, cancel := context.WithCancel(context.Background())
		recv := make(chan any)
		done := make(chan struct{})

		go func() {
			defer close(done)
			trader.listen(ctx, "token", "name", recv)
		}()

		cancel()

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("listen did not exit after context cancel")
		}
	})

	t.Run("stop channel closed", func(t *testing.T) {
		t.Parallel()

		trader := NewATPTrader(nil, nil, newCategoryFeed(CategoryATP), models.GammaMarket{})
		ctx := context.Background()
		recv := make(chan any)
		done := make(chan struct{})

		go func() {
			defer close(done)
			trader.listen(ctx, "token", "name", recv)
		}()

		close(trader.stop)

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("listen did not exit after stop channel close")
		}
	})

	t.Run("recv channel closed", func(t *testing.T) {
		t.Parallel()

		trader := NewATPTrader(nil, nil, newCategoryFeed(CategoryATP), models.GammaMarket{})
		ctx := context.Background()
		recv := make(chan any)
		done := make(chan struct{})

		go func() {
			defer close(done)
			trader.listen(ctx, "token", "name", recv)
		}()

		close(recv)

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("listen did not exit after recv channel close")
		}
	})
}

func TestATPTraderStopLogsUnsubscribeWarnPath(t *testing.T) {
	t.Parallel()

	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		<-r.Context().Done()
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket server: %v", err)
	}

	feed := newCategoryFeed(CategoryATP)
	feed.conn = conn

	subCh := make(chan any, 1)
	tokenID := "warn-token"
	feed.subscribers[tokenID] = []tokenMeta{{name: "warn name", ch: subCh}}

	trader := NewATPTrader(nil, nil, feed, models.GammaMarket{})
	trader.subs = []atpSubscription{{tokenID: tokenID, ch: subCh}}

	// Close the active websocket so Unsubscribe's sendUnsubscribe write fails.
	if err := conn.Close(); err != nil {
		t.Fatalf("close websocket conn: %v", err)
	}

	trader.Stop()
	waitClosed(t, trader.signals, "trader signals close after stop")
}

func waitClosed[T any](t *testing.T, ch <-chan T, name string) {
	t.Helper()
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatalf("%s expected closed channel", name)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("%s timed out waiting for close", name)
	}
}

func testMarket(t *testing.T, slug, conditionID string, tokenIDs []string) models.GammaMarket {
	t.Helper()

	tokenIDsJSON, err := json.Marshal(tokenIDs)
	if err != nil {
		t.Fatalf("marshal token ids: %v", err)
	}
	outcomesJSON, err := json.Marshal([]string{"YES", "NO"})
	if err != nil {
		t.Fatalf("marshal outcomes: %v", err)
	}

	return models.GammaMarket{
		EnableOrderBook: true,
		Slug:            slug,
		ConditionID:     conditionID,
		Question:        slug + " question",
		ClobTokenIds:    string(tokenIDsJSON),
		Outcomes:        string(outcomesJSON),
	}
}
