package main

import (
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"sort"

	"github.com/AndochBonin/polymarket/tennis"
	"github.com/AndochBonin/polymarket/tennisabstract"
)

// BacktestConfig controls the chronological betting walk.
type BacktestConfig struct {
	MatchesPath string
	RatesPath   string
	Sims        int
	Alpha       float64
	MinPick     float64
	Stake       float64 // amount risked per bet (win pays stake*decimal odds)
	Seed        uint64
	Log         *slog.Logger
}

func (cfg BacktestConfig) log() *slog.Logger {
	if cfg.Log != nil {
		return cfg.Log
	}
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// BacktestStats aggregates walk outcomes.
type BacktestStats struct {
	FinalBalance float64
	GrossProfit  float64 // sum of stake*odds on wins (total return credited)
	GrossLoss    float64 // sum of stakes lost
	Bets         int
	Wins         int
	Losses       int
	Skipped      int // walked matches with no bet
	MatchesWalk  int
}

// BacktestRunResult holds sim strategy and favorite baseline stats.
// Favorite uses the same bet/no-bet decisions as sim (min-pick); only the side differs.
type BacktestRunResult struct {
	Sim      BacktestStats
	Favorite BacktestStats
}

// RunBacktests walks eligible matches once: sim bets when min-pick clears; favorite
// bets the market favorite on the same matches (skips when sim skips).
func RunBacktests(cfg BacktestConfig) (BacktestRunResult, error) {
	if err := (&cfg).validate(); err != nil {
		return BacktestRunResult{}, err
	}

	eligible, err := loadEligibleMatches(cfg)
	if err != nil {
		return BacktestRunResult{}, err
	}

	return walkBacktests(cfg, eligible)
}

// RunBacktest runs only the sim strategy (kept for tests).
func RunBacktest(cfg BacktestConfig) (BacktestStats, error) {
	res, err := RunBacktests(cfg)
	if err != nil {
		return BacktestStats{}, err
	}
	return res.Sim, nil
}

func (cfg *BacktestConfig) validate() error {
	if cfg.Sims <= 0 {
		return fmt.Errorf("sims must be positive")
	}
	if cfg.Alpha <= 0 {
		return fmt.Errorf("alpha must be positive")
	}
	if cfg.MinPick <= 0 || cfg.MinPick > 1 {
		return fmt.Errorf("min-pick must be in (0,1]")
	}
	if cfg.Stake == 0 {
		cfg.Stake = 1
	}
	if cfg.Stake <= 0 {
		return fmt.Errorf("stake must be positive")
	}
	return nil
}

func (cfg *BacktestConfig) normalizedStake() float64 {
	if cfg.Stake == 0 {
		return 1
	}
	return cfg.Stake
}

func loadEligibleMatches(cfg BacktestConfig) ([]tennisabstract.MatchWithOdds, error) {
	log := cfg.log()

	rows, err := tennisabstract.LoadMatchesWithOddsCSVFile(cfg.MatchesPath)
	if err != nil {
		return nil, fmt.Errorf("load matches: %w", err)
	}
	rates, err := tennisabstract.ReadPlayerRatesFile(cfg.RatesPath)
	if err != nil {
		return nil, fmt.Errorf("load rates: %w", err)
	}

	eligible := tennisabstract.FilterBacktestMatches(rows, rates)
	sort.SliceStable(eligible, func(i, j int) bool {
		if eligible[i].TourneyDate != eligible[j].TourneyDate {
			return eligible[i].TourneyDate < eligible[j].TourneyDate
		}
		return eligible[i].MatchNum < eligible[j].MatchNum
	})
	log.Info("eligible matches", "count", len(eligible), "filtered_out", len(rows)-len(eligible))
	return eligible, nil
}

func walkBacktests(cfg BacktestConfig, eligible []tennisabstract.MatchWithOdds) (BacktestRunResult, error) {
	stake := cfg.normalizedStake()
	rates, err := tennisabstract.ReadPlayerRatesFile(cfg.RatesPath)
	if err != nil {
		return BacktestRunResult{}, fmt.Errorf("load rates: %w", err)
	}

	rng := rand.New(rand.NewPCG(cfg.Seed, mixSeed(cfg.Seed)))

	var out BacktestRunResult
	out.Sim.MatchesWalk = len(eligible)
	out.Favorite.MatchesWalk = len(eligible)

	for _, m := range eligible {
		playerRates, ok := matchPlayerRates(m, rates)
		if !ok {
			out.Sim.Skipped++
			out.Favorite.Skipped++
			continue
		}

		result, err := tennis.SimulateFresh(m.Format, playerRates, cfg.Alpha, cfg.Sims, rng)
		if err != nil {
			return out, fmt.Errorf("simulate %s vs %s: %w", m.PlayerA, m.PlayerB, err)
		}

		winsA := result.WinCount(tennis.A)
		simSide, simOdds, ok := tennisabstract.DecideBet(winsA, cfg.Sims, cfg.MinPick, m.AvgW, m.AvgL)
		if !ok {
			return out, fmt.Errorf("decide bet %s vs %s: invalid inputs", m.PlayerA, m.PlayerB)
		}
		if simSide == tennisabstract.BetSideNone {
			out.Sim.Skipped++
			out.Favorite.Skipped++
			continue
		}

		applyBet(&out.Sim, simSide, simOdds, stake, historicalWinnerSide())

		favSide, favOdds, ok := tennisabstract.DecideFavoriteBet(m.AvgW, m.AvgL)
		if !ok {
			return out, fmt.Errorf("favorite bet %s vs %s: invalid odds", m.PlayerA, m.PlayerB)
		}
		applyBet(&out.Favorite, favSide, favOdds, stake, historicalWinnerSide())
	}

	return out, nil
}

// historicalWinnerSide is the match winner in Sackmann rows (winner_name = player A).
func historicalWinnerSide() tennisabstract.BetSide {
	return tennisabstract.BetSideA
}

func applyBet(stats *BacktestStats, picked tennisabstract.BetSide, odds, stake float64, actualWinner tennisabstract.BetSide) {
	stats.Bets++
	pnl := settleBet(picked, actualWinner, odds, stake)
	stats.FinalBalance += pnl
	if pnl > 0 {
		stats.Wins++
		stats.GrossProfit += pnl
	} else {
		stats.Losses++
		stats.GrossLoss += -pnl
	}
}

func mixSeed(seed uint64) uint64 {
	return seed ^ 0x9e3779b97f4a7c15
}

func matchPlayerRates(m tennisabstract.MatchWithOdds, rates tennisabstract.PlayerRatesMap) ([2]tennis.PlayerRates, bool) {
	a, okA := rates[m.PlayerASlug]
	b, okB := rates[m.PlayerBSlug]
	if !okA || !okB {
		return [2]tennis.PlayerRates{}, false
	}
	return [2]tennis.PlayerRates{
		{HoldPct: a.Hold2024, BreakPct: a.Break2024},
		{HoldPct: b.Hold2024, BreakPct: b.Break2024},
	}, true
}

// settleBet credits stake×decimal odds when the picked player won; otherwise −stake.
func settleBet(picked, actualWinner tennisabstract.BetSide, odds, stake float64) float64 {
	if picked == actualWinner {
		return stake * odds
	}
	return -stake
}
