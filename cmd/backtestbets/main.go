// Walk 2025 ATP matches with odds: simulate, bet when sim win rate clears -min-pick.
//
// Run (from repo root):
//
//	go run ./cmd/backtestbets
//	go run ./cmd/backtestbets -stake=5 -min-pick=0.6 -sims=10000
//
// On a TTY, stake, min-pick, and sims are prompted when omitted (Enter keeps defaults).
// Or: make backtest-bets
package main

import (
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/AndochBonin/polymarket/internal/prompt"
)

func main() {
	os.Exit(exitRun())
}

func exitRun() int {
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

	stats, err := RunBacktest(cfg)
	if err != nil {
		log.Error("backtest failed", "err", err)
		return 1
	}

	fmt.Printf("final_balance=%.2f\n", stats.FinalBalance)
	fmt.Printf("gross_profit=%.2f gross_loss=%.2f\n", stats.GrossProfit, stats.GrossLoss)
	fmt.Printf("bets=%d wins=%d losses=%d skipped=%d\n",
		stats.Bets, stats.Wins, stats.Losses, stats.Skipped)

	log.Info("backtest done",
		"final_balance", stats.FinalBalance,
		"gross_profit", stats.GrossProfit,
		"gross_loss", stats.GrossLoss,
		"bets", stats.Bets,
		"wins", stats.Wins,
		"losses", stats.Losses,
		"skipped", stats.Skipped,
		"matches_walked", stats.MatchesWalk,
	)

	return 0
}
