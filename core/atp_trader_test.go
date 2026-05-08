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

	marketFeed := newMarketFeed(CategoryATP)

	marketA := testMarket(t, "atp-a", "0xabc123", []string{"a_yes", "a_no"})
	marketB := testMarket(t, "atp-b", "0xdef456", []string{"b_yes", "b_no"})

	traderA := NewATPTrader(nil, nil, marketFeed, nil, marketA)
	traderB := NewATPTrader(nil, nil, marketFeed, nil, marketB)

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
		marketFeed.broadcastTo(assetID, resolved)
	}

	waitClosed(t, traderA.signals, "traderA signals close")

	select {
	case <-traderB.stop:
		t.Fatal("traderB unexpectedly stopped")
	default:
	}

	marketFeed.mu.RLock()
	remainingA := len(marketFeed.subscribers["a_yes"]) + len(marketFeed.subscribers["a_no"])
	remainingB := len(marketFeed.subscribers["b_yes"]) + len(marketFeed.subscribers["b_no"])
	marketFeed.mu.RUnlock()

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
	validNoEvents := models.GammaMarket{
		EnableOrderBook: true,
		Slug:            "atp-valid-no-events",
		ConditionID:     "valid-no-events",
		ClobTokenIds:    `["yes-token","no-token"]`,
		Outcomes:        `["YES","NO"]`,
	}
	validEmptyContext := models.GammaMarket{
		EnableOrderBook: true,
		Slug:            "atp-valid-empty-context",
		ConditionID:     "valid-empty-context",
		ClobTokenIds:    `["yes-token","no-token"]`,
		Outcomes:        `["YES","NO"]`,
		Events: []models.GammaMarketEvent{
			{
				EventMetadata: models.GammaMarketEventMetadata{
					ContextDescription: "   ",
				},
			},
		},
	}

	markets := []models.GammaMarket{
		valid,
		validNoEvents,
		validEmptyContext,
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
		{
			EnableOrderBook: true,
			Slug:            "atp-challenger-uppercase",
			ConditionID:     "challenger-uppercase",
			ClobTokenIds:    `["yes-token","no-token"]`,
			Outcomes:        `["YES","NO"]`,
			Events: []models.GammaMarketEvent{
				{
					EventMetadata: models.GammaMarketEventMetadata{
						ContextDescription: "Wuxi ATP Challenger round of 16",
					},
				},
			},
		},
		{
			EnableOrderBook: true,
			Slug:            "atp-challenger-lowercase",
			ConditionID:     "challenger-lowercase",
			ClobTokenIds:    `["yes-token","no-token"]`,
			Outcomes:        `["YES","NO"]`,
			Events: []models.GammaMarketEvent{
				{
					EventMetadata: models.GammaMarketEventMetadata{
						ContextDescription: "wuxi challenger quarterfinal",
					},
				},
			},
		},
		{
			EnableOrderBook: true,
			Slug:            "atp-challenger-league-only",
			ConditionID:     "challenger-league-only",
			ClobTokenIds:    `["yes-token","no-token"]`,
			Outcomes:        `["YES","NO"]`,
			Events: []models.GammaMarketEvent{
				{
					EventMetadata: models.GammaMarketEventMetadata{
						League: "ATP Challenger - Wuxi",
					},
				},
			},
		},
	}

	got := FilterATPMarkets(markets)
	if len(got) != 3 {
		t.Fatalf("expected exactly three valid markets, got %d", len(got))
	}
	gotByConditionID := make(map[string]models.GammaMarket, len(got))
	for _, market := range got {
		gotByConditionID[market.ConditionID] = market
	}

	for _, want := range []string{valid.ConditionID, validNoEvents.ConditionID, validEmptyContext.ConditionID} {
		if _, ok := gotByConditionID[want]; !ok {
			t.Fatalf("expected market %q to be included", want)
		}
	}

	for _, unwanted := range []string{"challenger-uppercase", "challenger-lowercase", "challenger-league-only"} {
		if _, ok := gotByConditionID[unwanted]; ok {
			t.Fatalf("expected market %q to be excluded", unwanted)
		}
	}
}

func TestATPTraderStartValidationErrors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		clobTokenIDs   string
		outcomes       string
		events         []models.GammaMarketEvent
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
		{
			name:         "challenger context rejected",
			clobTokenIDs: `["yes-token","no-token"]`,
			outcomes:     `["YES","NO"]`,
			events: []models.GammaMarketEvent{
				{
					EventMetadata: models.GammaMarketEventMetadata{
						ContextDescription: "Wuxi Challenger round of 16 ATP Challenger clash",
					},
				},
			},
			wantErrSubstrs: []string{"reject challenger market", "ATP Challenger"},
		},
		{
			name:         "challenger league rejected",
			clobTokenIDs: `["yes-token","no-token"]`,
			outcomes:     `["YES","NO"]`,
			events: []models.GammaMarketEvent{
				{
					EventMetadata: models.GammaMarketEventMetadata{
						League: "ATP Challenger - Wuxi",
					},
				},
			},
			wantErrSubstrs: []string{"reject challenger market", "ATP Challenger"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			trader := NewATPTrader(nil, nil, newMarketFeed(CategoryATP), nil, models.GammaMarket{
				EnableOrderBook: true,
				Slug:            "atp-start-error",
				ConditionID:     "0xstart-error",
				Question:        "start error question",
				ClobTokenIds:    tc.clobTokenIDs,
				Outcomes:        tc.outcomes,
				Events:          tc.events,
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

func TestATPTraderStartAllowsOutcomeTokenLengthMismatch(t *testing.T) {
	t.Parallel()

	marketFeed := newMarketFeed(CategoryATP)
	market := testMarketWithOutcomes(
		t,
		"atp-mismatch",
		"0xoutcome-token-mismatch",
		[]string{"token-yes", "token-no", "token-draw"},
		[]string{"YES"},
	)
	trader := NewATPTrader(nil, nil, marketFeed, nil, market)

	if err := trader.Start(context.Background()); err != nil {
		t.Fatalf("start trader with mismatched outcomes/tokens: %v", err)
	}

	if len(trader.marketSubs) != 3 {
		t.Fatalf("expected 3 subscriptions, got %d", len(trader.marketSubs))
	}

	marketFeed.mu.RLock()
	yesListeners := marketFeed.subscribers["token-yes"]
	noListeners := marketFeed.subscribers["token-no"]
	drawListeners := marketFeed.subscribers["token-draw"]
	marketFeed.mu.RUnlock()

	if len(yesListeners) != 1 || len(noListeners) != 1 || len(drawListeners) != 1 {
		t.Fatalf(
			"expected one listener per token, got yes=%d no=%d draw=%d",
			len(yesListeners),
			len(noListeners),
			len(drawListeners),
		)
	}

	if got := yesListeners[0].name; got != "atp-mismatch question — YES" {
		t.Fatalf("expected first listener name with outcome suffix, got %q", got)
	}
	if got := noListeners[0].name; got != "atp-mismatch question" {
		t.Fatalf("expected second listener fallback question name, got %q", got)
	}
	if got := drawListeners[0].name; got != "atp-mismatch question" {
		t.Fatalf("expected third listener fallback question name, got %q", got)
	}

	trader.Stop()
	waitClosed(t, trader.signals, "trader signals close after mismatch start")
}

func TestATPTraderHandleBranchesAndSignals(t *testing.T) {
	t.Parallel()

	trader := NewATPTrader(nil, nil, newMarketFeed(CategoryATP), nil, models.GammaMarket{
		Slug:        "atp-handle-test",
		ConditionID: "0xabc123",
		Question:    "handle test question",
	})

	if trader.Signals() == nil {
		t.Fatal("expected non-nil signals channel")
	}

	// Price event with non-matching token should continue without panic.
	trader.handleMarket("target-token", "target name", models.PriceEvent{
		EventType: "price_change",
		Market:    "0xabc123",
		PriceChanges: []models.PriceChange{
			{AssetID: "different-token", Side: models.OrderSideBuy, Price: "0.51"},
		},
	})

	// Price event with matching token should execute matching branch.
	trader.handleMarket("target-token", "target name", models.PriceEvent{
		EventType: "price_change",
		Market:    "0xabc123",
		PriceChanges: []models.PriceChange{
			{AssetID: "target-token", Side: models.OrderSideSell, Price: "0.49"},
		},
	})

	trader.handleSports(5428186, "target name", models.SportsEvent{
		GameID:             5428186,
		LeagueAbbreviation: "atp",
		HomeTeam:           "Home",
		AwayTeam:           "Away",
		Status:             "inprogress",
		Score:              "1-0",
		Period:             "set2",
		Live:               true,
		Ended:              false,
		EventState: models.SportsEventState{
			Type:           "tennis",
			TournamentName: "Test Open",
			TennisRound:    "QF",
		},
	})
	trader.handleMarket("target-token", "target name", models.BookEvent{
		EventType: "book",
		Market:    "0xabc123",
	})

	// Non-matching market resolved event should not stop trader.
	trader.handleMarket("target-token", "target name", models.MarketResolvedEvent{
		EventType: "market_resolved",
		Market:    "0xdef456",
	})
	select {
	case <-trader.stop:
		t.Fatal("trader unexpectedly stopped on non-matching market resolution")
	default:
	}

	trader.handleMarket("target-token", "target name", errors.New("test error event"))
	trader.handleMarket("target-token", "target name", struct{ Event string }{Event: "unknown"})

	// Matching market resolved event should trigger Stop.
	trader.handleMarket("target-token", "target name", models.MarketResolvedEvent{
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

		trader := NewATPTrader(nil, nil, newMarketFeed(CategoryATP), nil, models.GammaMarket{})
		ctx, cancel := context.WithCancel(context.Background())
		recv := make(chan any)
		done := make(chan struct{})

		go func() {
			defer close(done)
			trader.listenMarket(ctx, "token", "name", recv)
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

		trader := NewATPTrader(nil, nil, newMarketFeed(CategoryATP), nil, models.GammaMarket{})
		ctx := context.Background()
		recv := make(chan any)
		done := make(chan struct{})

		go func() {
			defer close(done)
			trader.listenMarket(ctx, "token", "name", recv)
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

		trader := NewATPTrader(nil, nil, newMarketFeed(CategoryATP), nil, models.GammaMarket{})
		ctx := context.Background()
		recv := make(chan any)
		done := make(chan struct{})

		go func() {
			defer close(done)
			trader.listenMarket(ctx, "token", "name", recv)
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

	marketFeed := newMarketFeed(CategoryATP)
	marketFeed.conn = conn

	subCh := make(chan any, 1)
	tokenID := "warn-token"
	marketFeed.subscribers[tokenID] = []tokenMeta{{name: "warn name", ch: subCh}}

	trader := NewATPTrader(nil, nil, marketFeed, nil, models.GammaMarket{})
	trader.marketSubs = []atpMarketSubscription{{tokenID: tokenID, ch: subCh}}

	// Close the active websocket so Unsubscribe's sendUnsubscribe write fails.
	if err := conn.Close(); err != nil {
		t.Fatalf("close websocket conn: %v", err)
	}

	trader.Stop()
	waitClosed(t, trader.signals, "trader signals close after stop")
}

func TestSportsGameIDFromMarket(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		market models.GammaMarket
		want   int64
	}{
		{
			name: "returns first event game id",
			market: models.GammaMarket{
				Events: []models.GammaMarketEvent{
					{GameID: 5428186},
					{GameID: 9999999},
				},
			},
			want: 5428186,
		},
		{
			name: "returns zero when events empty",
			market: models.GammaMarket{
				Events: nil,
			},
			want: 0,
		},
		{
			name: "returns zero when first event game id missing",
			market: models.GammaMarket{
				Events: []models.GammaMarketEvent{
					{GameID: 0},
				},
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := sportsGameIDFromMarket(tt.market); got != tt.want {
				t.Fatalf("sportsGameIDFromMarket()=%d, want=%d", got, tt.want)
			}
		})
	}
}

func TestATPTraderStartAndStopSportsPath(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	marketFeed := newMarketFeed(CategoryATP)
	sportsFeed := NewSportsFeed()

	const gameID int64 = 5428186
	market := testMarket(t, "atp-sports", "0xsports", []string{"yes-token", "no-token"})
	market.Events = []models.GammaMarketEvent{{GameID: gameID}}

	trader := NewATPTrader(nil, nil, marketFeed, sportsFeed, market)
	if err := trader.Start(ctx); err != nil {
		t.Fatalf("start trader: %v", err)
	}

	if trader.sportsSub == nil {
		t.Fatal("expected sportsSub to be set after Start with sports-eligible market")
	}
	if trader.sportsSub.gameID != gameID {
		t.Fatalf("expected sportsSub gameID %d, got %d", gameID, trader.sportsSub.gameID)
	}

	sportsFeed.mu.RLock()
	gameSubs := len(sportsFeed.subscribers[gameID])
	sportsFeed.mu.RUnlock()
	if gameSubs != 1 {
		t.Fatalf("expected 1 sports subscriber for gameID %d, got %d", gameID, gameSubs)
	}

	trader.Stop()
	waitClosed(t, trader.signals, "trader signals close after sports stop")

	sportsFeed.mu.RLock()
	remaining := len(sportsFeed.subscribers[gameID])
	sportsFeed.mu.RUnlock()
	if remaining != 0 {
		t.Fatalf("expected sports subscriber removed after Stop, got %d", remaining)
	}
}

func TestATPTraderListenSportsExitsOnContextCancelStopAndClosedChannel(t *testing.T) {
	t.Parallel()

	t.Run("context canceled", func(t *testing.T) {
		t.Parallel()

		trader := NewATPTrader(nil, nil, newMarketFeed(CategoryATP), nil, models.GammaMarket{})
		ctx, cancel := context.WithCancel(context.Background())
		recv := make(chan any)
		done := make(chan struct{})

		go func() {
			defer close(done)
			trader.listenSports(ctx, 5428186, "name", recv)
		}()

		cancel()

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("listenSports did not exit after context cancel")
		}
	})

	t.Run("stop channel closed", func(t *testing.T) {
		t.Parallel()

		trader := NewATPTrader(nil, nil, newMarketFeed(CategoryATP), nil, models.GammaMarket{})
		ctx := context.Background()
		recv := make(chan any)
		done := make(chan struct{})

		go func() {
			defer close(done)
			trader.listenSports(ctx, 5428186, "name", recv)
		}()

		close(trader.stop)

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("listenSports did not exit after stop channel close")
		}
	})

	t.Run("recv channel closed", func(t *testing.T) {
		t.Parallel()

		trader := NewATPTrader(nil, nil, newMarketFeed(CategoryATP), nil, models.GammaMarket{})
		ctx := context.Background()
		recv := make(chan any)
		done := make(chan struct{})

		go func() {
			defer close(done)
			trader.listenSports(ctx, 5428186, "name", recv)
		}()

		close(recv)

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("listenSports did not exit after recv channel close")
		}
	})

	t.Run("processes received event before exit", func(t *testing.T) {
		t.Parallel()

		trader := NewATPTrader(nil, nil, newMarketFeed(CategoryATP), nil, models.GammaMarket{})
		ctx := context.Background()
		recv := make(chan any, 1)
		done := make(chan struct{})

		go func() {
			defer close(done)
			trader.listenSports(ctx, 5428186, "name", recv)
		}()

		recv <- models.SportsEvent{
			GameID:             5428186,
			LeagueAbbreviation: "atp",
		}
		close(recv)

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("listenSports did not exit after processing event and recv close")
		}
	})
}

func TestATPTraderHandleSportsErrorAndDefault(t *testing.T) {
	t.Parallel()

	trader := NewATPTrader(nil, nil, newMarketFeed(CategoryATP), nil, models.GammaMarket{
		Slug:        "atp-handle-sports",
		ConditionID: "0xsports-handle",
		Question:    "sports handle question",
	})

	trader.handleSports(5428186, "sports name", errors.New("sports stream failure"))
	trader.handleSports(5428186, "sports name", struct{ Event string }{Event: "unknown sports event"})
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
	return testMarketWithOutcomes(t, slug, conditionID, tokenIDs, []string{"YES", "NO"})
}

func testMarketWithOutcomes(t *testing.T, slug, conditionID string, tokenIDs []string, outcomes []string) models.GammaMarket {
	t.Helper()

	tokenIDsJSON, err := json.Marshal(tokenIDs)
	if err != nil {
		t.Fatalf("marshal token ids: %v", err)
	}
	outcomesJSON, err := json.Marshal(outcomes)
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
