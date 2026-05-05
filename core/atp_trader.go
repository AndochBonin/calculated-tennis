package core

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"sync"

	"github.com/AndochBonin/polymarket/clob"
	"github.com/AndochBonin/polymarket/gamma"
	"github.com/AndochBonin/polymarket/models"
)

type ATPTrader struct {
	gammaClient *gamma.Client
	clobClient  *clob.Client
	feed        *CategoryFeed
	signals     chan TradeSignal
	stop        chan struct{}
	listenersWg sync.WaitGroup
	stopOnce    sync.Once
}

func NewATPTrader(gammaClient *gamma.Client, clobClient *clob.Client, categoryFeed *CategoryFeed) *ATPTrader {
	return &ATPTrader{
		gammaClient: gammaClient,
		clobClient:  clobClient,
		feed:        categoryFeed,
		signals:     make(chan TradeSignal, 100),
		stop:        make(chan struct{}),
	}
}

func (t *ATPTrader) Start(ctx context.Context) error {
	closed := false
	markets, err := t.gammaClient.GetMarkets(ctx, gamma.MarketsParams{
		TagID:             int(TagATP),
		Closed:            &closed,
		SportsMarketTypes: []string{"moneyline"},
	})
	if err != nil {
		return err
	}

	log.Printf("[ATP] fetched %d markets", len(markets))

	for _, market := range markets {
		if !market.EnableOrderBook {
			continue
		}

		if !strings.HasPrefix(market.Slug, "atp-") {
			continue
		}

		var tokenIDs []string
		if err := json.Unmarshal([]byte(market.ClobTokenIds), &tokenIDs); err != nil {
			log.Printf("[ATP] failed to parse token ids for market %s: %v", market.ConditionID, err)
			continue
		}

		var outcomeNames []string
		if err := json.Unmarshal([]byte(market.Outcomes), &outcomeNames); err != nil {
			log.Printf("[ATP] failed to parse outcomes for market %s: %v", market.ConditionID, err)
			continue
		}

		for i, tokenID := range tokenIDs {
			name := market.Question
			if i < len(outcomeNames) {
				name = market.Question + " — " + outcomeNames[i]
			}

			ch := make(chan any, 100)
			if err := t.feed.Subscribe(tokenID, name, ch); err != nil {
				log.Printf("[ATP] failed to subscribe | name=%s error=%v", name, err)
				continue
			}

			t.listenersWg.Add(1)
			t.listenersWg.Go(func() {
				defer t.listenersWg.Done()
				t.listen(ctx, tokenID, name, ch)
			})
		}
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
			log.Printf("[ATP] price event | name=%s side=%s price=%s",
				name, change.Side, change.Price)
		}
	case models.SportEvent:
		log.Printf("[ATP] sport event | name=%s slug=%s score=%s period=%s elapsed=%s live=%v ended=%v",
			name, e.Slug, e.Score, e.Period, e.Elapsed, e.Live, e.Ended)
	case error:
		log.Printf("[DEBUG] go to error event: %v", e)
	default:
		log.Printf("[DEBUG] event type: %v", e)
	}
}

func (t *ATPTrader) Stop() {
	t.stopOnce.Do(func() {
		for token, metas := range t.feed.subscribers {
			for _, m := range metas {
				err := t.feed.Unsubscribe(token, m.ch)
				if err != nil {
					log.Printf("[ATP] failed to unsubscribe | name=%s", m.name)
				} else {
					log.Printf("[ATP] unsubscribed | name=%s", m.name)
				}
			}
		}
		close(t.stop)
		t.listenersWg.Wait()
	})
}

func (t *ATPTrader) Signals() <-chan TradeSignal {
	return t.signals
}
