package tennisabstract

import (
	"context"
	"fmt"
	"time"

	"github.com/AndochBonin/polymarket/models"
	"github.com/AndochBonin/polymarket/tennis"
)

// MatchPlayerRates returns hold/break for both players in a calibration match.
// When useRecentForm is true, career recent results are fetched as-of the match date
// and AdjustedHoldBreakAsOf is applied with formOpts (AsOf is set to the tourney date);
// fetch or form errors yield ok=false. When useRecentForm is false, formOpts is ignored.
func MatchPlayerRates(
	ctx context.Context,
	m CalibrationMatch,
	rates PlayerRatesMap,
	client *Client,
	useRecentForm bool,
	formOpts FormOptions,
) ([2]tennis.PlayerRates, bool) {
	a, okA := rates[m.PlayerASlug]
	b, okB := rates[m.PlayerBSlug]
	if !okA || !okB {
		return [2]tennis.PlayerRates{}, false
	}
	if !useRecentForm {
		return [2]tennis.PlayerRates{
			{HoldPct: a.Hold2024, BreakPct: a.Break2024},
			{HoldPct: b.Hold2024, BreakPct: b.Break2024},
		}, true
	}
	if client == nil {
		return [2]tennis.PlayerRates{}, false
	}
	asOf, ok := TourneyDateAsTime(m.TourneyDate)
	if !ok {
		return [2]tennis.PlayerRates{}, false
	}
	opts := formOpts
	opts.AsOf = asOf

	ra, ok, err := playerRatesWithForm(ctx, client, m.PlayerA, m.PlayerASlug, a, asOf, opts)
	if err != nil || !ok {
		return [2]tennis.PlayerRates{}, false
	}
	rb, ok, err := playerRatesWithForm(ctx, client, m.PlayerB, m.PlayerBSlug, b, asOf, opts)
	if err != nil || !ok {
		return [2]tennis.PlayerRates{}, false
	}
	return [2]tennis.PlayerRates{ra, rb}, true
}

// MatchPlayerRatesFromPreload applies form adjustment using PreloadCalibrationMatchRecent
// data for matchIndex in preload. No HTTP is performed.
func MatchPlayerRatesFromPreload(
	m CalibrationMatch,
	rates PlayerRatesMap,
	preload CalibrationRecentPreload,
	matchIndex int,
	formOpts FormOptions,
) ([2]tennis.PlayerRates, bool) {
	a, okA := rates[m.PlayerASlug]
	b, okB := rates[m.PlayerBSlug]
	if !okA || !okB {
		return [2]tennis.PlayerRates{}, false
	}
	if matchIndex < 0 || matchIndex >= len(preload.PerMatch) {
		return [2]tennis.PlayerRates{}, false
	}
	pr := preload.PerMatch[matchIndex]
	if !pr.OK {
		return [2]tennis.PlayerRates{}, false
	}
	asOf, ok := TourneyDateAsTime(m.TourneyDate)
	if !ok {
		return [2]tennis.PlayerRates{}, false
	}
	opts := formOpts
	opts.AsOf = asOf

	ra, ok, err := playerRatesWithPreloadedRecent(m.PlayerASlug, a, asOf, pr.RecentA, opts)
	if err != nil || !ok {
		return [2]tennis.PlayerRates{}, false
	}
	rb, ok, err := playerRatesWithPreloadedRecent(m.PlayerBSlug, b, asOf, pr.RecentB, opts)
	if err != nil || !ok {
		return [2]tennis.PlayerRates{}, false
	}
	return [2]tennis.PlayerRates{ra, rb}, true
}

// MatchWithOddsPlayerRates is MatchPlayerRates for backtest rows with odds.
func MatchWithOddsPlayerRates(
	ctx context.Context,
	m MatchWithOdds,
	rates PlayerRatesMap,
	client *Client,
	useRecentForm bool,
	formOpts FormOptions,
) ([2]tennis.PlayerRates, bool) {
	return MatchPlayerRates(ctx, CalibrationMatch{
		PlayerA:     m.PlayerA,
		PlayerB:     m.PlayerB,
		PlayerASlug: m.PlayerASlug,
		PlayerBSlug: m.PlayerBSlug,
		TourneyDate: m.TourneyDate,
	}, rates, client, useRecentForm, formOpts)
}

func playerRatesWithForm(
	ctx context.Context,
	client *Client,
	playerName, slug string,
	season PlayerRates2024,
	asOf time.Time,
	opts FormOptions,
) (tennis.PlayerRates, bool, error) {
	opts = opts.withDefaults()
	recent, err := client.GetRecentResultsAsOf(ctx, playerName, asOf, opts.RecentMatchLimit)
	if err != nil {
		return tennis.PlayerRates{}, false, fmt.Errorf("recent results %s: %w", slug, err)
	}
	return playerRatesWithPreloadedRecent(slug, season, asOf, recent, opts)
}

func playerRatesWithPreloadedRecent(
	slug string,
	season PlayerRates2024,
	asOf time.Time,
	recent []models.RecentResult,
	opts FormOptions,
) (tennis.PlayerRates, bool, error) {
	stats := PlayerStatsForBacktestForm(slug, asOf, season.Hold2024, season.Break2024)
	adj, err := AdjustedHoldBreakAsOf(stats, recent, opts)
	if err != nil {
		return tennis.PlayerRates{}, false, err
	}
	return tennis.PlayerRates{HoldPct: adj.HoldPct, BreakPct: adj.BreakPct}, true, nil
}
