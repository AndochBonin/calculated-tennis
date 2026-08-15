// Form-parameter calibration on historical ATP matches (per surface).
//
// Run (from repo root):
//
//	go run ./cmd/calibrateform
//	go run ./cmd/calibrateform -matches=tennisabstract/testdata/atp_matches_2025.csv \
//		-rates=tennisabstract/testdata/player_rates_2024.json -sims=500
//
// Stdout shows the best grid point and top 10 rows per surface; use -json-out for the full grid.
// -workers sets concurrent grid points (default: NumCPU); results are reproducible per grid index.
//
// Build rates first: make fetch-rates
// Career JSON cache: make fetch-career (or set TENNISABSTRACT_CAREER_DIR).
//
// Optional env (.env loaded automatically): TENNISABSTRACT_CAREER_DIR overrides the
// default career-match JSON directory (tennisabstract/testdata/career).
// Live fetches use TENNISABSTRACT_REQUEST_INTERVAL (default 2s) and 429 backoff env vars.
package main

import (
	"flag"
	"log/slog"
	"os"

	"github.com/AndochBonin/calculated-tennis/tennis/tennisabstract"
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
	seedFlag := flag.Uint64("seed", 1, "PCG seed for reproducible simulations")
	workersFlag := flag.Int("workers", 0, "concurrent grid points (default: NumCPU)")
	jsonOutFlag := flag.String("json-out", "", "optional path for machine-readable full grid JSON")

	halfLifeMin := flag.Float64("half-life-min", 1, "half-life grid start")
	halfLifeMax := flag.Float64("half-life-max", 10, "half-life grid stop")
	halfLifeStep := flag.Float64("half-life-step", 2, "half-life grid step")
	weightMaxMin := flag.Float64("weight-max-min", 0, "form weight max grid start")
	weightMaxMax := flag.Float64("weight-max-max", 1, "form weight max grid stop")
	weightMaxStep := flag.Float64("weight-max-step", 0.10, "form weight max grid step")
	ratioMinMin := flag.Float64("ratio-min-min", 0.80, "form ratio min grid start")
	ratioMinMax := flag.Float64("ratio-min-max", 1.00, "form ratio min grid stop")
	ratioMinStep := flag.Float64("ratio-min-step", 0.04, "form ratio min grid step")
	ratioMaxMin := flag.Float64("ratio-max-min", 1.00, "form ratio max grid start")
	ratioMaxMax := flag.Float64("ratio-max-max", 1.20, "form ratio max grid stop")
	ratioMaxStep := flag.Float64("ratio-max-step", 0.04, "form ratio max grid step")

	flag.Parse()

	cfg := CalibrateFormConfig{
		MatchesPath: *matchesFlag,
		RatesPath:   *ratesFlag,
		Sims:        *simsFlag,
		Seed:        *seedFlag,
		Workers:     *workersFlag,
		Log:         log,
		GridBounds: tennisabstract.FormGridBounds{
			HalfLifeStart: *halfLifeMin, HalfLifeStop: *halfLifeMax, HalfLifeStep: *halfLifeStep,
			WeightMaxStart: *weightMaxMin, WeightMaxStop: *weightMaxMax, WeightMaxStep: *weightMaxStep,
			RatioMinStart: *ratioMinMin, RatioMinStop: *ratioMinMax, RatioMinStep: *ratioMinStep,
			RatioMaxStart: *ratioMaxMin, RatioMaxStop: *ratioMaxMax, RatioMaxStep: *ratioMaxStep,
		},
	}

	log.Info("calibrating form",
		"matches", cfg.MatchesPath,
		"rates", cfg.RatesPath,
		"sims", cfg.Sims,
		"seed", cfg.Seed,
		"workers", calibrationWorkers(cfg.Workers),
		"grid_points", len(tennisabstract.FormGridCombos(cfg.GridBounds)),
	)

	surfaces, err := RunCalibration(cfg)
	if err != nil {
		log.Error("calibration failed", "err", err)
		return 1
	}

	PrintFormCalibrationSummary(os.Stdout, surfaces)

	if *jsonOutFlag != "" {
		if err := WriteFormCalibrationReportFile(*jsonOutFlag, cfg, surfaces); err != nil {
			log.Error("write json", "path", *jsonOutFlag, "err", err)
			return 1
		}
		log.Info("wrote json grid", "path", *jsonOutFlag)
	}

	return 0
}
