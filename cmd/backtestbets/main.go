// Walk 2025 ATP matches with odds: season sim, recent-form sim, and favorite baseline.
//
// Run (from repo root):
//
//	go run ./cmd/backtestbets
//	go run ./cmd/backtestbets -stake=5 -min-pick=0.6 -sims=10000
//
// On a TTY, stake, min-pick, and sims are prompted when omitted (Enter keeps defaults).
// Or: make backtest-bets
//
// Career JSON cache: make fetch-career (or set TENNISABSTRACT_CAREER_DIR).
//
// Optional env (.env loaded automatically): TENNISABSTRACT_CAREER_DIR overrides the
// default career-match JSON directory (tennisabstract/testdata/career).
package main

import (
	"errors"
	"flag"
	"log/slog"
	"os"

	"github.com/AndochBonin/polymarket/internal/prompt"
	"github.com/joho/godotenv"
)

func main() {
	os.Exit(exitRun())
}

func exitRun() int {
	_ = godotenv.Load()

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	matchesFlag := flag.String("matches", "tennisabstract/testdata/atp_matches_2025_odds.csv", "ATP matches+odds CSV path")
	ratesFlag := flag.String("rates", "tennisabstract/testdata/player_rates_2024.json", "player hold/break JSON cache")
	simsFlag := flag.String("sims", "", "Monte Carlo simulations per match")
	alphaFlag := flag.Float64("alpha", 1, "alpha parameter for SimulateFresh")
	minPickFlag := flag.String("min-pick", "", "minimum sim win rate to place a bet (0,1]")
	stakeFlag := flag.String("stake", "", "amount risked per bet (win pays stake × decimal odds)")
	seedFlag := flag.Uint64("seed", 1, "PCG seed for reproducible simulations")
	flag.Parse()

	in, err := resolveBacktestInputs(os.Stdin, *stakeFlag, *minPickFlag, *simsFlag, prompt.IsInteractive)
	if err != nil {
		log.Error("input", "err", err)
		if errors.Is(err, errInvalidStake) || errors.Is(err, errInvalidMinPick) || errors.Is(err, errInvalidSims) {
			return 2
		}
		return 1
	}

	cfg := BacktestConfig{
		MatchesPath: *matchesFlag,
		RatesPath:   *ratesFlag,
		Sims:        in.sims,
		Alpha:       *alphaFlag,
		MinPick:     in.minPick,
		Stake:       in.stake,
		Seed:        *seedFlag,
		Log:         log,
	}

	result, err := RunBacktests(cfg)
	if err != nil {
		log.Error("backtest failed", "err", err)
		return 1
	}

	printBacktestResults(os.Stdout, result, cfg.Stake)

	return 0
}
