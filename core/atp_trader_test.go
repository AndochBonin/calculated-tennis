package core

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/AndochBonin/polymarket/models"
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
