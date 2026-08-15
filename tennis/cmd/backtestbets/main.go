// Walk 2025 ATP matches with odds: season sim, recent-form sim, and favorite baseline.
//
// Run (from repo root):
//
//	go run ./cmd/backtestbets
//	go run ./cmd/backtestbets -stake=5 -min-pick=0.6 -sims=10000
//	go run ./cmd/backtestbets -money-manager -initial-balance=1000 -max-pct-balance=0.05
// Per-run bet CSVs are written to backtest-logs/ (sim.csv, sim-form.csv, favorite.csv).
//
// On a TTY, stake, min-pick, sims, min odds, and optional money-manager params are prompted (Enter keeps defaults).
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

	"github.com/AndochBonin/calculated-tennis/tennis/internal/prompt"
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
	moneyManagerFlag := flag.Bool("money-manager", false, "use money manager for dynamic stake sizing")
	initialBalanceFlag := flag.String("initial-balance", "", "starting USDC bankroll when money manager is enabled")
	maxOrderUSDCFlag := flag.String("max-order-usdc", "", "optional absolute USDC cap per bet (empty = no cap)")
	maxPctBalanceFlag := flag.String("max-pct-balance", "", "fraction of available balance to allocate per bet (default 0.05)")
	minShareSizeFlag := flag.String("min-share-size", "", "minimum share size after allocation (default 5)")
	minOddsFlag := flag.String("min-odds", "", "minimum decimal odds to bet (0 = no minimum)")
	requirePositiveEVFlag := flag.Bool("require-positive-ev", false, "reject bets when sim E = p×odds−1 ≤ 0")
	parlayFlag := flag.Bool("parlay", false, "parlay mode (greedy non-overlapping windows) instead of singles")
	maxParlayMatchesFlag := flag.String("max-parlay-matches", "", "max legs per parlay (required with -parlay or implies parlay when set)")
	flag.Parse()

	in, err := resolveBacktestInputs(os.Stdin, backtestInputFlags{
		stake:              *stakeFlag,
		minPick:            *minPickFlag,
		minOdds:            *minOddsFlag,
		sims:               *simsFlag,
		requirePositiveEV:  *requirePositiveEVFlag,
		parlay:             *parlayFlag,
		maxParlayMatches:   *maxParlayMatchesFlag,
		moneyManager:       *moneyManagerFlag,
		initialBalance:    *initialBalanceFlag,
		maxOrderUSDC:      *maxOrderUSDCFlag,
		maxPctBalance:     *maxPctBalanceFlag,
		minShareSize:      *minShareSizeFlag,
	}, prompt.IsInteractive)
	if err != nil {
		log.Error("input", "err", err)
		if errors.Is(err, errInvalidStake) || errors.Is(err, errInvalidMinPick) || errors.Is(err, errInvalidSims) ||
			errors.Is(err, errInvalidMinOdds) || errors.Is(err, errInvalidBetMode) ||
			errors.Is(err, errInvalidMaxParlayMatches) ||
			errors.Is(err, errInvalidInitialBalance) || errors.Is(err, errInvalidMaxPctBalance) ||
			errors.Is(err, errInvalidMinShareSize) || errors.Is(err, errInvalidMaxOrderUSDC) {
			return 2
		}
		return 1
	}

	cfg := BacktestConfig{
		MatchesPath:       *matchesFlag,
		RatesPath:         *ratesFlag,
		Sims:              in.sims,
		Alpha:             *alphaFlag,
		MinPick:           in.minPick,
		MinOdds:           in.minOdds,
		RequirePositiveEV: in.requirePositiveEV,
		Stake:             in.stake,
		Seed:              *seedFlag,
		Log:               log,
		MoneyManager:      in.moneyManager,
		BetLogDir:         DefaultBetLogDir,
		BetMode:           in.betMode,
		MaxParlayMatches:  in.maxParlayMatches,
	}

	result, err := RunBacktests(cfg)
	if err != nil {
		log.Error("backtest failed", "err", err)
		return 1
	}

	printBacktestResults(os.Stdout, result, cfg.Stake, cfg.MoneyManager)
	log.Info("bet logs written", "dir", cfg.BetLogDir)

	return 0
}
