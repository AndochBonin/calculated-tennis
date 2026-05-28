// Join average match odds (AvgW, AvgL) onto a Sackmann ATP matches CSV.
//
//	go run ./cmd/joinmatchodds
package main

import (
	"flag"
	"log/slog"
	"os"

	"github.com/AndochBonin/polymarket/tennisabstract"
)

func main() {
	os.Exit(run())
}

func run() int {
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	matches := flag.String("matches", "tennisabstract/testdata/atp_matches_2025.csv", "Sackmann matches CSV")
	odds := flag.String("odds", "tennisabstract/testdata/odds_2025.csv", "tennis-data odds CSV")
	out := flag.String("out", "tennisabstract/testdata/atp_matches_2025_odds.csv", "output CSV path")
	flag.Parse()

	stats, err := tennisabstract.JoinMatchesWithAvgOddsCSV(*matches, *odds, *out)
	if err != nil {
		log.Error("join failed", "err", err)
		return 1
	}
	log.Info("wrote joined matches",
		"path", *out,
		"rows", stats.RowsWritten,
		"matched", stats.RowsMatched,
		"unmatched", stats.RowsUnmatched,
	)
	return 0
}
