package core

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/AndochBonin/E3/polymarket/gamma"
	"github.com/AndochBonin/E3/polymarket/models"
	"golang.org/x/sync/singleflight"
)

const (
	defaultDiscoveryEmptyRetries = 3
	defaultDiscoveryRetryDelay   = 200 * time.Millisecond
)

type atpDiscoveryGammaClient interface {
	GetMarkets(context.Context, gamma.MarketsParams) ([]models.GammaMarket, error)
}

type ATPMarketDiscovery struct {
	gammaClient        atpDiscoveryGammaClient
	startTrader        func(context.Context, models.GammaMarket) error
	emptyResultRetries int
	retryDelay         time.Duration
	logger             *slog.Logger

	startedMu  sync.Mutex
	startedSet map[string]struct{}

	sf singleflight.Group
}

func NewATPMarketDiscovery(
	gammaClient atpDiscoveryGammaClient,
	startTrader func(context.Context, models.GammaMarket) error,
	seedConditionIDs []string,
) *ATPMarketDiscovery {
	started := make(map[string]struct{}, len(seedConditionIDs))
	for _, conditionID := range seedConditionIDs {
		id := strings.TrimSpace(conditionID)
		if id == "" {
			continue
		}
		started[id] = struct{}{}
	}

	return &ATPMarketDiscovery{
		gammaClient:        gammaClient,
		startTrader:        startTrader,
		emptyResultRetries: defaultDiscoveryEmptyRetries,
		retryDelay:         defaultDiscoveryRetryDelay,
		logger:             slog.Default(),
		startedSet:         started,
	}
}

func (d *ATPMarketDiscovery) HandleNewMarket(ctx context.Context, ev models.NewMarketEvent) {
	if d == nil || d.gammaClient == nil || d.startTrader == nil {
		return
	}

	slug := strings.TrimSpace(ev.Slug)
	if !strings.HasPrefix(slug, "atp-") {
		return
	}
	if !strings.EqualFold(strings.TrimSpace(ev.SportsMarketType), "moneyline") {
		return
	}

	_, _, _ = d.sf.Do(slug, func() (any, error) {
		d.handleNewMarket(ctx, ev)
		return nil, nil
	})
}

func (d *ATPMarketDiscovery) handleNewMarket(ctx context.Context, ev models.NewMarketEvent) {
	market, ok := d.fetchAndNormalize(ctx, ev)
	if !ok {
		return
	}

	filtered := FilterATPMarkets([]models.GammaMarket{market})
	if len(filtered) != 1 {
		return
	}
	market = filtered[0]

	if !d.markStarted(market.ConditionID) {
		return
	}

	if err := d.startTrader(ctx, market); err != nil {
		d.unmarkStarted(market.ConditionID)
		d.logger.Error("failed to start ATP trader from new_market",
			append([]any{"slug", market.Slug, "err", err}, AppendVerboseIDs("condition_id", market.ConditionID)...)...)
	} else {
		d.logger.Warn("started ATP trader from new_market",
			append([]any{"slug", market.Slug}, AppendVerboseIDs("condition_id", market.ConditionID)...)...)
	}
}

func (d *ATPMarketDiscovery) fetchAndNormalize(ctx context.Context, ev models.NewMarketEvent) (models.GammaMarket, bool) {
	slug := strings.TrimSpace(ev.Slug)
	markets, ok := d.hydrateBySlug(ctx, slug, ev.ConditionID)
	if !ok {
		return models.GammaMarket{}, false
	}

	return d.normalizeMarkets(markets, ev)
}

func (d *ATPMarketDiscovery) hydrateBySlug(
	ctx context.Context,
	slug string,
	conditionID string,
) ([]models.GammaMarket, bool) {
	closed := false
	for attempt := 0; attempt <= d.emptyResultRetries; attempt++ {
		markets, err := d.gammaClient.GetMarkets(ctx, gamma.MarketsParams{
			Slug:   slug,
			Closed: &closed,
		})
		if err != nil {
			d.logger.Warn("failed to hydrate ATP market by slug",
				append([]any{"slug", slug, "attempt", attempt + 1, "err", err}, AppendVerboseIDs("condition_id", conditionID)...)...)
			return nil, false
		}
		if len(markets) > 0 {
			return markets, true
		}
		if attempt == d.emptyResultRetries {
			break
		}
		select {
		case <-ctx.Done():
			return nil, false
		case <-time.After(d.retryDelay):
		}
	}

	return nil, false
}

func (d *ATPMarketDiscovery) normalizeMarkets(markets []models.GammaMarket, ev models.NewMarketEvent) (models.GammaMarket, bool) {
	if len(markets) == 0 {
		return models.GammaMarket{}, false
	}

	wantConditionID := strings.TrimSpace(ev.ConditionID)
	if wantConditionID == "" {
		wantConditionID = strings.TrimSpace(ev.Market)
	}

	if wantConditionID != "" {
		for _, m := range markets {
			if strings.EqualFold(strings.TrimSpace(m.ConditionID), wantConditionID) {
				return m, true
			}
		}
		return models.GammaMarket{}, false
	}

	if len(markets) == 1 {
		return markets[0], true
	}
	return models.GammaMarket{}, false
}

func (d *ATPMarketDiscovery) markStarted(conditionID string) bool {
	id := strings.TrimSpace(conditionID)
	if id == "" {
		return false
	}

	d.startedMu.Lock()
	defer d.startedMu.Unlock()

	if _, exists := d.startedSet[id]; exists {
		return false
	}
	d.startedSet[id] = struct{}{}
	return true
}

func (d *ATPMarketDiscovery) unmarkStarted(conditionID string) {
	id := strings.TrimSpace(conditionID)
	if id == "" {
		return
	}

	d.startedMu.Lock()
	delete(d.startedSet, id)
	d.startedMu.Unlock()
}
