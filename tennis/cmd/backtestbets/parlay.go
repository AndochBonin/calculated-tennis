package main

import (
	"context"
	"fmt"
	"math/rand/v2"

	"github.com/AndochBonin/E3/moneymanager/pkg/risk"
	"github.com/AndochBonin/E3/tennis/tennis"
	"github.com/AndochBonin/E3/tennis/tennisabstract"
)

type parlayStrategy int

const (
	parlayStrategySim parlayStrategy = iota
	parlayStrategyFavorite
	parlayStrategySimForm
)

// parlayLeg is one evaluable leg in a parlay window.
type parlayLeg struct {
	match   tennisabstract.MatchWithOdds
	pick    tennisabstract.BetSide
	odds    float64
	winProb float64
	winner  tennisabstract.BetSide
}

type parlayMatchKey struct {
	tourneyID   string
	tourneyDate int
	matchNum    int
}

func parlayMatchKeyOf(m tennisabstract.MatchWithOdds) parlayMatchKey {
	return parlayMatchKey{
		tourneyID:   m.TourneyID,
		tourneyDate: m.TourneyDate,
		matchNum:    m.MatchNum,
	}
}

// parlayEvalDeps loads and simulates legs for one strategy track during window selection.
type parlayEvalDeps struct {
	ctx      context.Context
	cfg      BacktestConfig
	rates    tennisabstract.PlayerRatesMap
	client   *tennisabstract.Client
	rng      *rand.Rand
	strategy parlayStrategy
	winner   tennisabstract.BetSide
	cache    map[parlayMatchKey]parlayLeg

	// testEvalLeg overrides buildLeg when set (tests only).
	testEvalLeg func(m tennisabstract.MatchWithOdds) (parlayLeg, bool, error)
}

func newParlayEvalDeps(
	ctx context.Context,
	cfg BacktestConfig,
	rates tennisabstract.PlayerRatesMap,
	client *tennisabstract.Client,
	rng *rand.Rand,
	strategy parlayStrategy,
	winner tennisabstract.BetSide,
) *parlayEvalDeps {
	return &parlayEvalDeps{
		ctx:      ctx,
		cfg:      cfg,
		rates:    rates,
		client:   client,
		rng:      rng,
		strategy: strategy,
		winner:   winner,
		cache:    make(map[parlayMatchKey]parlayLeg),
	}
}

func (d *parlayEvalDeps) evalLeg(m tennisabstract.MatchWithOdds) (parlayLeg, bool, error) {
	key := parlayMatchKeyOf(m)
	if leg, ok := d.cache[key]; ok {
		return leg, true, nil
	}
	if d.testEvalLeg != nil {
		leg, ok, err := d.testEvalLeg(m)
		if err != nil || !ok {
			return leg, ok, err
		}
		if d.cache == nil {
			d.cache = make(map[parlayMatchKey]parlayLeg)
		}
		d.cache[key] = leg
		return leg, true, nil
	}
	leg, ok, err := d.buildLeg(m)
	if err != nil {
		return parlayLeg{}, false, err
	}
	if !ok {
		return parlayLeg{}, false, nil
	}
	d.cache[key] = leg
	return leg, true, nil
}

func (d *parlayEvalDeps) buildLeg(m tennisabstract.MatchWithOdds) (parlayLeg, bool, error) {
	useForm := d.strategy == parlayStrategySimForm
	formOpts := tennisabstract.FormOptions{}
	if useForm {
		formOpts = tennisabstract.FormOptionsFromEnv(m.Surface)
	}
	playerRates, ok := tennisabstract.MatchWithOddsPlayerRates(d.ctx, m, d.rates, d.client, useForm, formOpts)
	if !ok {
		return parlayLeg{}, false, nil
	}

	var pick tennisabstract.BetSide
	var odds, winProb float64
	var legOK bool
	var err error

	switch d.strategy {
	case parlayStrategySim, parlayStrategySimForm:
		pick, odds, winProb, legOK, err = simulateParlayLeg(m, playerRates, d.cfg, d.rng)
	case parlayStrategyFavorite:
		pick, odds, winProb, legOK, err = simulateFavoriteParlayLeg(m, playerRates, d.cfg, d.rng)
	default:
		return parlayLeg{}, false, fmt.Errorf("unknown parlay strategy %d", d.strategy)
	}
	if err != nil {
		return parlayLeg{}, false, err
	}
	if !legOK {
		return parlayLeg{}, false, nil
	}
	return parlayLeg{
		match:   m,
		pick:    pick,
		odds:    odds,
		winProb: winProb,
		winner:  d.winner,
	}, true, nil
}

// buildParlayLegs evaluates eligible[anchor:anchor+k) as consecutive parlay legs.
func buildParlayLegs(anchor, k int, eligible []tennisabstract.MatchWithOdds, deps *parlayEvalDeps) ([]parlayLeg, bool, error) {
	legs := make([]parlayLeg, 0, k)
	for j := 0; j < k; j++ {
		leg, ok, err := deps.evalLeg(eligible[anchor+j])
		if err != nil {
			return nil, false, err
		}
		if !ok {
			return nil, false, nil
		}
		legs = append(legs, leg)
	}
	return legs, true, nil
}

// combinedParlayProbOdds returns ∏ winProb and ∏ decimal odds across legs.
func combinedParlayProbOdds(legs []parlayLeg) (winProb, odds float64) {
	if len(legs) == 0 {
		return 0, 0
	}
	winProb = 1
	odds = 1
	for _, leg := range legs {
		winProb *= leg.winProb
		odds *= leg.odds
	}
	return winProb, odds
}

// parlayEVAllowed applies RequirePositiveEV to combined parlay win probability and odds.
func parlayEVAllowed(cfg BacktestConfig, combinedWinProb, combinedOdds float64) bool {
	return cfg.positiveEVAllowed(combinedWinProb, combinedOdds)
}

// selectParlayWindow greedily picks the largest k ≤ maxParlay where every leg in
// eligible[anchor:anchor+k) is evaluable and combined EV passes when required.
func selectParlayWindow(
	anchor int,
	eligible []tennisabstract.MatchWithOdds,
	cfg BacktestConfig,
	maxParlay int,
	deps *parlayEvalDeps,
) (legs []parlayLeg, size int, ok bool, err error) {
	if maxParlay < 1 {
		return nil, 0, false, nil
	}
	remaining := len(eligible) - anchor
	maxK := maxParlay
	if remaining < maxK {
		maxK = remaining
	}
	for k := maxK; k >= 1; k-- {
		window, built, err := buildParlayLegs(anchor, k, eligible, deps)
		if err != nil {
			return nil, 0, false, err
		}
		if !built {
			continue
		}
		p, combinedOdds := combinedParlayProbOdds(window)
		if !parlayEVAllowed(cfg, p, combinedOdds) {
			continue
		}
		return window, k, true, nil
	}
	return nil, 0, false, nil
}

func parlayWins(legs []parlayLeg) bool {
	for _, leg := range legs {
		if leg.pick != leg.winner {
			return false
		}
	}
	return len(legs) > 0
}

// settleParlay returns net PnL for one parlay stake at combined decimal odds.
func settleParlay(legs []parlayLeg, stake float64) float64 {
	_, combinedOdds := combinedParlayProbOdds(legs)
	if parlayWins(legs) {
		return stake * (combinedOdds - 1)
	}
	return -stake
}

// placeParlay allocates stake at combined odds, updates stats, and logs the anchor row.
func placeParlay(
	ctx context.Context,
	cfg BacktestConfig,
	alloc *risk.Allocator,
	log *strategyBetLog,
	legs []parlayLeg,
	stats *BacktestStats,
	fixedStake float64,
) {
	if len(legs) == 0 {
		return
	}
	combinedWinProb, combinedOdds := combinedParlayProbOdds(legs)
	anchor := legs[0]

	if alloc != nil {
		tryPlaceParlayWithAllocator(ctx, alloc, log, legs, stats, anchor, combinedWinProb, combinedOdds, fixedStake)
		return
	}
	applyParlayWithLog(log, legs, stats, anchor, combinedWinProb, combinedOdds, fixedStake)
}

func tryPlaceParlayWithAllocator(
	ctx context.Context,
	alloc *risk.Allocator,
	log *strategyBetLog,
	legs []parlayLeg,
	stats *BacktestStats,
	anchor parlayLeg,
	combinedWinProb, combinedOdds, fixedStake float64,
) {
	stake, ok := allocateStake(ctx, alloc, combinedOdds)
	if !ok {
		stats.Skipped++
		if log != nil {
			log.recordParlaySkip(anchor.match, skipReasonALLOC, combinedWinProb, combinedOdds)
		}
		return
	}
	applyParlayWithLog(log, legs, stats, anchor, combinedWinProb, combinedOdds, stake)
}

func applyParlayWithLog(
	log *strategyBetLog,
	legs []parlayLeg,
	stats *BacktestStats,
	anchor parlayLeg,
	combinedWinProb, combinedOdds, stake float64,
) {
	if log != nil {
		log.recordParlayBet(legs, stats, combinedWinProb, combinedOdds, stake)
	}
	applyParlay(stats, legs, combinedOdds, stake)
}

func applyParlay(stats *BacktestStats, legs []parlayLeg, combinedOdds, stake float64) {
	stats.Bets++
	stats.ParlayLegs += len(legs)
	stats.TotalWagered += stake
	surf := stats.surfaceStats(legs[0].match.Surface)
	surf.Bets++
	pnl := settleParlay(legs, stake)
	stats.FinalBalance += pnl
	if pnl > 0 {
		stats.Wins++
		surf.Wins++
		stats.GrossProfit += pnl
	} else {
		stats.Losses++
		stats.GrossLoss += -pnl
	}
	stats.setSurfaceStats(legs[0].match.Surface, surf)
}

// simulateParlayLeg runs a fresh sim and backs the higher simulated win-rate side.
func simulateParlayLeg(
	m tennisabstract.MatchWithOdds,
	playerRates [2]tennis.PlayerRates,
	cfg BacktestConfig,
	rng *rand.Rand,
) (pick tennisabstract.BetSide, odds, winProb float64, ok bool, err error) {
	winsA, err := simulateParlayWinsA(m, playerRates, cfg, rng)
	if err != nil {
		return tennisabstract.BetSideNone, 0, 0, false, err
	}
	pick, odds, ok = tennisabstract.DecideSimFavoredSide(winsA, cfg.Sims, m.AvgW, m.AvgL)
	if !ok {
		return tennisabstract.BetSideNone, 0, 0, false, nil
	}
	return pick, odds, winProbForSide(winsA, cfg.Sims, pick), true, nil
}

// simulateFavoriteParlayLeg backs the pre-match favorite; win probability comes from the sim.
func simulateFavoriteParlayLeg(
	m tennisabstract.MatchWithOdds,
	playerRates [2]tennis.PlayerRates,
	cfg BacktestConfig,
	rng *rand.Rand,
) (pick tennisabstract.BetSide, odds, winProb float64, ok bool, err error) {
	winsA, err := simulateParlayWinsA(m, playerRates, cfg, rng)
	if err != nil {
		return tennisabstract.BetSideNone, 0, 0, false, err
	}
	pick, odds, ok = tennisabstract.DecideFavoriteBet(m.AvgW, m.AvgL)
	if !ok {
		return tennisabstract.BetSideNone, 0, 0, false, nil
	}
	return pick, odds, winProbForSide(winsA, cfg.Sims, pick), true, nil
}

func simulateParlayWinsA(
	m tennisabstract.MatchWithOdds,
	playerRates [2]tennis.PlayerRates,
	cfg BacktestConfig,
	rng *rand.Rand,
) (int, error) {
	result, err := tennis.SimulateFresh(m.Format, playerRates, cfg.Alpha, cfg.Sims, rng)
	if err != nil {
		return 0, fmt.Errorf("simulate %s vs %s: %w", m.PlayerA, m.PlayerB, err)
	}
	return result.WinCount(tennis.A), nil
}

// combinedParlayEV is E = p×odds − 1 for a parlay at combined decimal odds.
func combinedParlayEV(legs []parlayLeg) float64 {
	p, odds := combinedParlayProbOdds(legs)
	return risk.ExpectedValueDecimalOdds(p, odds)
}
