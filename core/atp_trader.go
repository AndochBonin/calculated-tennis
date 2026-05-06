package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/AndochBonin/polymarket/clob"
	"github.com/AndochBonin/polymarket/gamma"
	"github.com/AndochBonin/polymarket/models"
)

type atpSubscription struct {
	tokenID string
	ch      chan any
}

type ATPTrader struct {
	gammaClient *gamma.Client
	clobClient  *clob.Client
	feed        *CategoryFeed
	market      models.GammaMarket
	subs        []atpSubscription
	signals     chan TradeSignal
	stop        chan struct{}
	listenersWg sync.WaitGroup
	stopOnce    sync.Once
}

func NewATPTrader(gammaClient *gamma.Client, clobClient *clob.Client, categoryFeed *CategoryFeed, market models.GammaMarket) *ATPTrader {
	return &ATPTrader{
		gammaClient: gammaClient,
		clobClient:  clobClient,
		feed:        categoryFeed,
		market:      market,
		signals:     make(chan TradeSignal, 100),
		stop:        make(chan struct{}),
	}
}

// FilterATPMarkets returns markets that pass ATP discovery filters used at startup.
func FilterATPMarkets(markets []models.GammaMarket) []models.GammaMarket {
	out := make([]models.GammaMarket, 0, len(markets))
	for _, m := range markets {
		if !m.EnableOrderBook || !strings.HasPrefix(m.Slug, "atp-") {
			continue
		}
		var tokenIDs []string
		if err := json.Unmarshal([]byte(m.ClobTokenIds), &tokenIDs); err != nil || len(tokenIDs) == 0 {
			continue
		}
		var outcomeNames []string
		if err := json.Unmarshal([]byte(m.Outcomes), &outcomeNames); err != nil {
			continue
		}
		out = append(out, m)
	}
	return out
}

func (t *ATPTrader) Start(ctx context.Context) error {
	market := t.market

	var tokenIDs []string
	if err := json.Unmarshal([]byte(market.ClobTokenIds), &tokenIDs); err != nil {
		return fmt.Errorf("parse clob token ids for %s: %w", market.ConditionID, err)
	}
	if len(tokenIDs) == 0 {
		return fmt.Errorf("zero clob tokens for market %s", market.ConditionID)
	}

	var outcomeNames []string
	if err := json.Unmarshal([]byte(market.Outcomes), &outcomeNames); err != nil {
		return fmt.Errorf("parse outcomes for %s: %w", market.ConditionID, err)
	}

	slog.Info("start ATP market",
		append([]any{
			"slug", market.Slug,
			"tokens", len(tokenIDs),
		}, AppendVerboseIDs("condition_id", market.ConditionID)...)...,
	)

	for i, tokenID := range tokenIDs {
		name := market.Question
		if i < len(outcomeNames) {
			name = market.Question + " — " + outcomeNames[i]
		}

		ch := make(chan any, 100)
		t.feed.Subscribe(tokenID, name, ch)

		t.subs = append(t.subs, atpSubscription{tokenID: tokenID, ch: ch})

		t.listenersWg.Add(1)
		go func(tokenID, name string, recv <-chan any) {
			defer t.listenersWg.Done()
			t.listen(ctx, tokenID, name, recv)
		}(tokenID, name, ch)
	}

	return nil
}

func (t *ATPTrader) listen(ctx context.Context, tokenID string, name string, ch <-chan any) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.stop:
			return
		case event, ok := <-ch:
			if !ok {
				return
			}

			t.handle(tokenID, name, event)
		}
	}
}

func (t *ATPTrader) handle(tokenID string, name string, event any) {
	switch e := event.(type) {
	case models.PriceEvent:
		for _, change := range e.PriceChanges {
			if change.AssetID != tokenID {
				continue
			}
			// calculate what to trade signal to send
			slog.Info("price event",
				append([]any{
					"name", name,
					"side", change.Side,
					"price", change.Price,
				}, AppendVerboseIDs("token_id", tokenID)...)...,
			)
		}
	case models.SportEvent:
		slog.Info("sport event",
			append([]any{
				"name", name,
				"slug", e.Slug,
				"score", e.Score,
				"period", e.Period,
				"elapsed", e.Elapsed,
				"live", e.Live,
				"ended", e.Ended,
			}, AppendVerboseIDs("token_id", tokenID)...)...,
		)
	case models.BookEvent:
		slog.Info("book event",
			append([]any{
				"name", name,
				"bids", len(e.Bids),
				"asks", len(e.Asks),
			}, AppendVerboseIDs("market", e.Market)...)...,
		)
	case models.MarketResolvedEvent:
		if !strings.EqualFold(e.Market, t.market.ConditionID) {
			return
		}
		slog.Info("market resolved, stopping ATP trader",
			append([]any{
				"slug", t.market.Slug,
				"winning_asset_id", e.WinningAssetID,
				"winning_outcome", e.WinningOutcome,
				"timestamp", e.Timestamp,
			}, AppendVerboseIDs(
				"condition_id", t.market.ConditionID,
				"market", e.Market,
			)...)...,
		)
		go t.Stop()
	case error:
		slog.Error("error event",
			append([]any{
				"name", name,
				"err", e,
			}, AppendVerboseIDs("token_id", tokenID)...)...,
		)
	default:
		slog.Error("unknown event type",
			append([]any{
				"name", name,
				"event_type", fmt.Sprintf("%T", e),
			}, AppendVerboseIDs("token_id", tokenID)...)...,
		)
	}
}

func (t *ATPTrader) Stop() {
	t.stopOnce.Do(func() {
		for _, sub := range t.subs {
			if err := t.feed.Unsubscribe(sub.tokenID, sub.ch); err != nil {
				slog.Warn("failed to unsubscribe",
					append([]any{
						"err", err,
					}, AppendVerboseIDs("token_id", sub.tokenID)...)...,
				)
			} else {
				slog.Info("unsubscribed",
					AppendVerboseIDs("token_id", sub.tokenID)...,
				)
			}
		}
		close(t.stop)
		t.listenersWg.Wait()
		close(t.signals)
	})
}

func (t *ATPTrader) Signals() <-chan TradeSignal {
	return t.signals
}
