package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"sort"

	"github.com/AndochBonin/E3/tennis/tennis"
	"github.com/AndochBonin/E3/tennis/tennisabstract"
)

// BacktestConfig controls the chronological betting walk.
type BacktestConfig struct {
	MatchesPath   string
	RatesPath     string
	Sims          int
	Alpha         float64
	MinPick       float64
	Stake         float64 // amount risked per bet (win pays stake*decimal odds)
	Seed uint64
	Log  *slog.Logger
}

func (cfg BacktestConfig) log() *slog.Logger {
	if cfg.Log != nil {
		return cfg.Log
	}
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// surfaceBetStats counts wins and settled bets on one court surface.
type surfaceBetStats struct {
	Wins int
	Bets int
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
	Hard         surfaceBetStats
	Clay         surfaceBetStats
	Grass        surfaceBetStats
}

// BacktestRunResult holds sim (season rates), sim with recent form, and favorite baseline.
// Favorite uses the same bet/no-bet decisions as Sim (min-pick); only the side differs.
type BacktestRunResult struct {
	Sim      BacktestStats
	SimForm  BacktestStats
	Favorite BacktestStats
}

// RunBacktests walks eligible matches once: season sim, recent-form sim, and favorite
// (favorite mirrors Sim bet/no-bet; SimForm decides independently).
func RunBacktests(cfg BacktestConfig) (BacktestRunResult, error) {
	if err := (&cfg).validate(); err != nil {
		return BacktestRunResult{}, err
	}

	eligible, err := loadEligibleMatches(cfg)
	if err != nil {
		return BacktestRunResult{}, err
	}

	return walkBacktests(context.Background(), cfg, eligible)
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

func walkBacktests(ctx context.Context, cfg BacktestConfig, eligible []tennisabstract.MatchWithOdds) (BacktestRunResult, error) {
	stake := cfg.normalizedStake()
	rates, err := tennisabstract.ReadPlayerRatesFile(cfg.RatesPath)
	if err != nil {
		return BacktestRunResult{}, fmt.Errorf("load rates: %w", err)
	}

	log := cfg.log()
	opts := tennisabstract.CareerClientOptionsFromEnv()
	taClient := tennisabstract.NewClient(opts...)
	log.Info("recent form walk enabled", "career_dir", tennisabstract.CareerCacheDirFromEnv())

	rng := rand.New(rand.NewPCG(cfg.Seed, mixSeed(cfg.Seed)))

	var out BacktestRunResult
	n := len(eligible)
	out.Sim.MatchesWalk = n
	out.SimForm.MatchesWalk = n
	out.Favorite.MatchesWalk = n

	for _, m := range eligible {
		baseRates, ok := tennisabstract.MatchWithOddsPlayerRates(ctx, m, rates, taClient, false, tennisabstract.FormOptions{})
		if !ok {
			out.Sim.Skipped++
			out.SimForm.Skipped++
			out.Favorite.Skipped++
			continue
		}

		_, simSide, simOdds, bet, err := simulateAndDecide(m, baseRates, cfg, rng)
		if err != nil {
			return out, err
		}
		if !bet {
			out.Sim.Skipped++
			out.Favorite.Skipped++
		} else {
			applyBet(&out.Sim, m.Surface, simSide, simOdds, stake, historicalWinnerSide())
			favSide, favOdds, ok := tennisabstract.DecideFavoriteBet(m.AvgW, m.AvgL)
			if !ok {
				return out, fmt.Errorf("favorite bet %s vs %s: invalid odds", m.PlayerA, m.PlayerB)
			}
			applyBet(&out.Favorite, m.Surface, favSide, favOdds, stake, historicalWinnerSide())
		}

		formOpts := tennisabstract.FormOptionsFromEnv(m.Surface)
		formRates, formOK := tennisabstract.MatchWithOddsPlayerRates(ctx, m, rates, taClient, true, formOpts)
		if !formOK {
			out.SimForm.Skipped++
			continue
		}
		_, formSide, formOdds, formBet, err := simulateAndDecide(m, formRates, cfg, rng)
		if err != nil {
			return out, err
		}
		if !formBet {
			out.SimForm.Skipped++
			continue
		}
		applyBet(&out.SimForm, m.Surface, formSide, formOdds, stake, historicalWinnerSide())
	}

	return out, nil
}

func simulateAndDecide(
	m tennisabstract.MatchWithOdds,
	playerRates [2]tennis.PlayerRates,
	cfg BacktestConfig,
	rng *rand.Rand,
) (winsA int, side tennisabstract.BetSide, odds float64, bet bool, err error) {
	result, err := tennis.SimulateFresh(m.Format, playerRates, cfg.Alpha, cfg.Sims, rng)
	if err != nil {
		return 0, tennisabstract.BetSideNone, 0, false, fmt.Errorf("simulate %s vs %s: %w", m.PlayerA, m.PlayerB, err)
	}
	winsA = result.WinCount(tennis.A)
	side, odds, ok := tennisabstract.DecideBet(winsA, cfg.Sims, cfg.MinPick, m.AvgW, m.AvgL)
	if !ok {
		return 0, tennisabstract.BetSideNone, 0, false, fmt.Errorf("decide bet %s vs %s: invalid inputs", m.PlayerA, m.PlayerB)
	}
	if side == tennisabstract.BetSideNone {
		return winsA, side, odds, false, nil
	}
	return winsA, side, odds, true, nil
}

// historicalWinnerSide is the match winner in Sackmann rows (winner_name = player A).
func historicalWinnerSide() tennisabstract.BetSide {
	return tennisabstract.BetSideA
}

func applyBet(stats *BacktestStats, surface tennisabstract.MatchSurface, picked tennisabstract.BetSide, odds, stake float64, actualWinner tennisabstract.BetSide) {
	stats.Bets++
	surf := stats.surfaceStats(surface)
	surf.Bets++
	pnl := settleBet(picked, actualWinner, odds, stake)
	stats.FinalBalance += pnl
	if pnl > 0 {
		stats.Wins++
		surf.Wins++
		stats.GrossProfit += pnl
	} else {
		stats.Losses++
		stats.GrossLoss += -pnl
	}
	stats.setSurfaceStats(surface, surf)
}

func (s *BacktestStats) surfaceStats(surface tennisabstract.MatchSurface) surfaceBetStats {
	switch surface {
	case tennisabstract.SurfaceHard:
		return s.Hard
	case tennisabstract.SurfaceClay:
		return s.Clay
	case tennisabstract.SurfaceGrass:
		return s.Grass
	default:
		return surfaceBetStats{}
	}
}

func (s *BacktestStats) setSurfaceStats(surface tennisabstract.MatchSurface, st surfaceBetStats) {
	switch surface {
	case tennisabstract.SurfaceHard:
		s.Hard = st
	case tennisabstract.SurfaceClay:
		s.Clay = st
	case tennisabstract.SurfaceGrass:
		s.Grass = st
	}
}

func mixSeed(seed uint64) uint64 {
	return seed ^ 0x9e3779b97f4a7c15
}

// settleBet credits stake×decimal odds when the picked player won; otherwise −stake.
func settleBet(picked, actualWinner tennisabstract.BetSide, odds, stake float64) float64 {
	if picked == actualWinner {
		return stake * odds
	}
	return -stake
}
