// Alpha calibration on historical ATP matches (per surface).
//
// Run (from repo root):
//
//	go run ./cmd/calibratealpha
//	go run ./cmd/calibratealpha -matches=tennisabstract/testdata/atp_matches_2025.csv \
//		-rates=tennisabstract/testdata/player_rates_2024.json -sims=5000 -seed=1
//
// Build rates first: make fetch-rates
//
// When using -use-recent-form, build career cache first: make fetch-career
// (or set TENNISABSTRACT_CAREER_DIR).
//
// Optional env (.env loaded automatically): TENNISABSTRACT_CAREER_DIR overrides the
// default career-match JSON directory (tennisabstract/testdata/career).
package main

import (
	"flag"
	"log/slog"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	os.Exit(exitRun())
}

func exitRun() int {
	_ = godotenv.Load()

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	matchesFlag := flag.String("matches", "tennisabstract/testdata/atp_matches_2025.csv", "ATP matches CSV path")
	ratesFlag := flag.String("rates", "tennisabstract/testdata/player_rates_2024.json", "player hold/break JSON cache")
	simsFlag := flag.Int("sims", 5000, "Monte Carlo simulations per match")
	alphaMinFlag := flag.Int("alpha-min", 0, "minimum alpha (inclusive)")
	alphaMaxFlag := flag.Int("alpha-max", 50, "maximum alpha (inclusive)")
	seedFlag := flag.Uint64("seed", 1, "PCG seed for reproducible simulations")
	useRecentFormFlag := flag.Bool("use-recent-form", false, "adjust hold/break with recent form as-of each match date (requires career JSON cache; make fetch-career)")
	jsonOutFlag := flag.String("json-out", "", "optional path for machine-readable full grid JSON")
	flag.Parse()

	cfg := CalibrateConfig{
		MatchesPath:   *matchesFlag,
		RatesPath:     *ratesFlag,
		Sims:          *simsFlag,
		AlphaMin:      *alphaMinFlag,
		AlphaMax:      *alphaMaxFlag,
		Seed:          *seedFlag,
		UseRecentForm: *useRecentFormFlag,
		Log:           log,
	}

	log.Info("calibrating alpha",
		"matches", cfg.MatchesPath,
		"rates", cfg.RatesPath,
		"sims", cfg.Sims,
		"alpha_min", cfg.AlphaMin,
		"alpha_max", cfg.AlphaMax,
		"seed", cfg.Seed,
		"use_recent_form", cfg.UseRecentForm,
	)

	surfaces, err := RunCalibration(cfg)
	if err != nil {
		log.Error("calibration failed", "err", err)
		return 1
	}

	PrintCalibrationSummary(os.Stdout, surfaces)

	if *jsonOutFlag != "" {
		if err := WriteCalibrationReportFile(*jsonOutFlag, cfg, surfaces); err != nil {
			log.Error("write json", "path", *jsonOutFlag, "err", err)
			return 1
		}
		log.Info("wrote json grid", "path", *jsonOutFlag)
	}

	return 0
}
