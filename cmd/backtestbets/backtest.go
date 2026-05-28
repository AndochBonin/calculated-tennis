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
	Skipped      int // walked matches with no bet (threshold or DecideBet failure)
	MatchesWalk  int
}

// RunBacktest loads data, walks matches in chronological order, and returns P&L stats.
func RunBacktest(cfg BacktestConfig) (BacktestStats, error) {
	if cfg.Sims <= 0 {
		return BacktestStats{}, fmt.Errorf("sims must be positive")
	}
	if cfg.Alpha <= 0 {
		return BacktestStats{}, fmt.Errorf("alpha must be positive")
	}
	if cfg.MinPick <= 0 || cfg.MinPick > 1 {
		return BacktestStats{}, fmt.Errorf("min-pick must be in (0,1]")
	}
	if cfg.Stake == 0 {
		cfg.Stake = 1
	}
	if cfg.Stake <= 0 {
		return BacktestStats{}, fmt.Errorf("stake must be positive")
	}

	log := cfg.log()

	rows, err := tennisabstract.LoadMatchesWithOddsCSVFile(cfg.MatchesPath)
	if err != nil {
		return BacktestStats{}, fmt.Errorf("load matches: %w", err)
	}
	rates, err := tennisabstract.ReadPlayerRatesFile(cfg.RatesPath)
	if err != nil {
		return BacktestStats{}, fmt.Errorf("load rates: %w", err)
	}

	eligible := tennisabstract.FilterBacktestMatches(rows, rates)
	sort.SliceStable(eligible, func(i, j int) bool {
		if eligible[i].TourneyDate != eligible[j].TourneyDate {
			return eligible[i].TourneyDate < eligible[j].TourneyDate
		}
		return eligible[i].MatchNum < eligible[j].MatchNum
	})
	log.Info("eligible matches", "count", len(eligible), "filtered_out", len(rows)-len(eligible))

	rng := rand.New(rand.NewPCG(cfg.Seed, mixSeed(cfg.Seed)))

	var stats BacktestStats
	stats.MatchesWalk = len(eligible)

	for _, m := range eligible {
		playerRates, ok := matchPlayerRates(m, rates)
		if !ok {
			stats.Skipped++
			continue
		}

		result, err := tennis.SimulateFresh(m.Format, playerRates, cfg.Alpha, cfg.Sims, rng)
		if err != nil {
			return stats, fmt.Errorf("simulate %s vs %s: %w", m.PlayerA, m.PlayerB, err)
		}

		winsA := result.WinCount(tennis.A)
		side, odds, ok := tennisabstract.DecideBet(winsA, cfg.Sims, cfg.MinPick, m.AvgW, m.AvgL)
		if !ok {
			return stats, fmt.Errorf("decide bet %s vs %s: invalid inputs", m.PlayerA, m.PlayerB)
		}
		if side == tennisabstract.BetSideNone {
			stats.Skipped++
			continue
		}

		stats.Bets++
		pnl := settleHistoricalBet(side, odds, cfg.Stake)
		stats.FinalBalance += pnl
		if pnl > 0 {
			stats.Wins++
			stats.GrossProfit += pnl
		} else {
			stats.Losses++
			stats.GrossLoss += -pnl
		}
	}

	return stats, nil
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

// settleHistoricalBet applies P&L for a completed match where player A (CSV winner) won.
func settleHistoricalBet(side tennisabstract.BetSide, odds, stake float64) float64 {
	switch side {
	case tennisabstract.BetSideA:
		return stake * odds
	case tennisabstract.BetSideB:
		return -stake
	default:
		return 0
	}
}
