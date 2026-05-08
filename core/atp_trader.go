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

type atpMarketSubscription struct {
	tokenID string
	ch      chan any
}

type atpSportsSubscription struct {
	gameID int64
	ch     chan any
}

type ATPTrader struct {
	gammaClient *gamma.Client
	clobClient  *clob.Client
	marketFeed  *MarketFeed
	sportsFeed  *SportsFeed
	market      models.GammaMarket
	marketSubs  []atpMarketSubscription
	sportsSub   *atpSportsSubscription
	signals     chan TradeSignal
	stop        chan struct{}
	listenersWg sync.WaitGroup
	stopOnce    sync.Once
}

func NewATPTrader(
	gammaClient *gamma.Client,
	clobClient *clob.Client,
	marketFeed *MarketFeed,
	sportsFeed *SportsFeed,
	market models.GammaMarket,
) *ATPTrader {
	return &ATPTrader{
		gammaClient: gammaClient,
		clobClient:  clobClient,
		marketFeed:  marketFeed,
		sportsFeed:  sportsFeed,
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
		if marketContextDescribesATPChallenger(m) {
			continue
		}
		out = append(out, m)
	}
	return out
}

func marketContextDescribesATPChallenger(m models.GammaMarket) bool {
	for _, event := range m.Events {
		league := strings.TrimSpace(event.EventMetadata.League)
		if league != "" && strings.Contains(strings.ToLower(league), "challenger") {
			return true
		}
		contextDescription := strings.TrimSpace(event.EventMetadata.ContextDescription)
		if contextDescription == "" {
			continue
		}
		if strings.Contains(strings.ToLower(contextDescription), "atp challenger") || 
			strings.Contains(strings.ToLower(contextDescription), strings.ToLower(league) + " challenger") {
			return true
		}
	}
	return false
}

func sportsGameIDFromMarket(market models.GammaMarket) int64 {
	if len(market.Events) == 0 {
		return 0
	}
	gameID := market.Events[0].GameID
	if gameID == 0 {
		return 0
	}
	return gameID
}

func (t *ATPTrader) Start(ctx context.Context) error {
	market := t.market

	if marketContextDescribesATPChallenger(market) {
		return fmt.Errorf("reject challenger market %s (%s): context_description indicates ATP Challenger", market.ConditionID, market.Slug)
	}

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

	slog.Warn("start ATP market",
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
		t.marketFeed.Subscribe(tokenID, name, ch)

		t.marketSubs = append(t.marketSubs, atpMarketSubscription{tokenID: tokenID, ch: ch})

		t.listenersWg.Add(1)
		go func(tokenID, name string, recv <-chan any) {
			defer t.listenersWg.Done()
			t.listenMarket(ctx, tokenID, name, recv)
		}(tokenID, name, ch)
	}

	sportsGameID := sportsGameIDFromMarket(market)
	if t.sportsFeed != nil && sportsGameID != 0 {
		sportsCh := make(chan any, 100)
		t.sportsFeed.Subscribe(sportsGameID, market.Question, sportsCh)
		t.sportsSub = &atpSportsSubscription{
			gameID: sportsGameID,
			ch:     sportsCh,
		}

		t.listenersWg.Add(1)
		go func(gameID int64, name string, recv <-chan any) {
			defer t.listenersWg.Done()
			t.listenSports(ctx, gameID, name, recv)
		}(sportsGameID, market.Question, sportsCh)
	}

	return nil
}

func (t *ATPTrader) listenMarket(ctx context.Context, tokenID string, name string, ch <-chan any) {
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

			t.handleMarket(tokenID, name, event)
		}
	}
}

func (t *ATPTrader) handleMarket(tokenID string, name string, event any) {
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

func (t *ATPTrader) listenSports(ctx context.Context, gameID int64, name string, ch <-chan any) {
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
			t.handleSports(gameID, name, event)
		}
	}
}

func (t *ATPTrader) handleSports(gameID int64, name string, event any) {
	switch e := event.(type) {
	case models.SportsEvent:
		slog.Info("sport event",
			[]any{
				"name", name,
				"game_id", e.GameID,
				"league", e.LeagueAbbreviation,
				"home", e.HomeTeam,
				"away", e.AwayTeam,
				"status", e.Status,
				"score", e.Score,
				"period", e.Period,
				"live", e.Live,
				"ended", e.Ended,
				"event_state_type", e.EventState.Type,
				"tournament", e.EventState.TournamentName,
				"tennis_round", e.EventState.TennisRound,
			}...,
		)
	case error:
		slog.Error("sports error event", "name", name, "err", e, "game_id", gameID)
	default:
		slog.Error("unknown sports event type", "name", name, "event_type", fmt.Sprintf("%T", e), "game_id", gameID)
	}
}

func (t *ATPTrader) Stop() {
	t.stopOnce.Do(func() {
		for _, sub := range t.marketSubs {
			if err := t.marketFeed.Unsubscribe(sub.tokenID, sub.ch); err != nil {
				slog.Warn("failed to unsubscribe",
					append([]any{
						"err", err,
					}, AppendVerboseIDs("token_id", sub.tokenID)...)...,
				)
			} else {
				slog.Warn("unsubscribed",
					AppendVerboseIDs("token_id", sub.tokenID)...,
				)
			}
		}
		if t.sportsSub != nil && t.sportsFeed != nil {
			t.sportsFeed.Unsubscribe(t.sportsSub.gameID, t.sportsSub.ch)
		}
		close(t.stop)
		t.listenersWg.Wait()
		close(t.signals)
	})
}

func (t *ATPTrader) Signals() <-chan TradeSignal {
	return t.signals
}
