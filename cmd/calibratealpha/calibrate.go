package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"os"
	"sort"

	"github.com/AndochBonin/polymarket/tennis"
	"github.com/AndochBonin/polymarket/tennisabstract"
)

// CalibrateConfig controls the alpha grid search.
type CalibrateConfig struct {
	MatchesPath    string
	RatesPath      string
	Sims           int
	AlphaMin       int
	AlphaMax       int
	Seed           uint64
	UseRecentForm  bool
	Log            *slog.Logger // progress to stderr; nil → no logging
}

func (cfg CalibrateConfig) log() *slog.Logger {
	if cfg.Log != nil {
		return cfg.Log
	}
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// AlphaMetrics holds aggregate metrics for one (surface, alpha) pair.
type AlphaMetrics struct {
	Alpha           int     `json:"alpha"`
	MeanSimAccuracy float64 `json:"mean_sim_accuracy"`
	HitRate         float64 `json:"hit_rate"`
	MatchesIncluded int     `json:"matches_included"`
	MatchesSkipped  int     `json:"matches_skipped"`
}

// SurfaceCalibration is the full alpha grid for one court surface.
type SurfaceCalibration struct {
	Surface         tennisabstract.MatchSurface `json:"surface"`
	MatchesIncluded int                         `json:"matches_included"`
	MatchesSkipped  int                         `json:"matches_skipped"`
	Best            AlphaMetrics                `json:"best"`
	Grid            []AlphaMetrics              `json:"grid"`
}

// CalibrationReport is the machine-readable full grid (-json-out).
type CalibrationReport struct {
	Config   CalibrateConfig      `json:"config"`
	Surfaces []SurfaceCalibration `json:"surfaces"`
}

var calibrationSurfaces = []tennisabstract.MatchSurface{
	tennisabstract.SurfaceHard,
	tennisabstract.SurfaceClay,
	tennisabstract.SurfaceGrass,
}

// RunCalibration loads matches and rates, runs the alpha grid per surface, and returns results.
func RunCalibration(cfg CalibrateConfig) ([]SurfaceCalibration, error) {
	if cfg.Sims <= 0 {
		return nil, fmt.Errorf("sims must be positive")
	}
	if cfg.AlphaMin <= 0 || cfg.AlphaMax <= 0 || cfg.AlphaMin > cfg.AlphaMax {
		return nil, fmt.Errorf("alpha range must be positive with alpha-min <= alpha-max")
	}

	log := cfg.log()

	log.Info("loading matches", "path", cfg.MatchesPath)
	load, err := tennisabstract.LoadCalibrationMatchesCSVFile(cfg.MatchesPath)
	if err != nil {
		return nil, fmt.Errorf("load matches: %w", err)
	}
	log.Info("matches loaded",
		"skipped_incomplete", load.SkippedIncomplete,
		"skipped_invalid", load.SkippedInvalid,
		"hard", len(load.BySurface[tennisabstract.SurfaceHard]),
		"clay", len(load.BySurface[tennisabstract.SurfaceClay]),
		"grass", len(load.BySurface[tennisabstract.SurfaceGrass]),
	)

	log.Info("loading rates", "path", cfg.RatesPath)
	rates, err := tennisabstract.ReadPlayerRatesFile(cfg.RatesPath)
	if err != nil {
		return nil, fmt.Errorf("load rates: %w", err)
	}
	log.Info("rates loaded", "players", len(rates))

	rng := rand.New(rand.NewPCG(cfg.Seed, mixSeed(cfg.Seed)))

	var taClient *tennisabstract.Client
	if cfg.UseRecentForm {
		opts := tennisabstract.CareerClientOptionsFromEnv()
		taClient = tennisabstract.NewClient(opts...)
		log.Info("recent form enabled", "career_dir", tennisabstract.CareerCacheDirFromEnv())
	}

	ctx := context.Background()
	out := make([]SurfaceCalibration, 0, len(calibrationSurfaces))
	for _, surface := range calibrationSurfaces {
		log.Info("surface start", "surface", surface, "matches_in_csv", len(load.BySurface[surface]))
		sc, err := calibrateSurface(ctx, cfg, surface, load.BySurface[surface], rates, taClient, rng)
		if err != nil {
			return nil, fmt.Errorf("surface %s: %w", surface, err)
		}
		log.Info("surface done",
			"surface", sc.Surface,
			"matches_included", sc.MatchesIncluded,
			"matches_skipped", sc.MatchesSkipped,
			"best_alpha", sc.Best.Alpha,
			"mean_sim_accuracy", fmt.Sprintf("%.4f", sc.Best.MeanSimAccuracy),
			"hit_rate", fmt.Sprintf("%.4f", sc.Best.HitRate),
		)
		out = append(out, sc)
	}
	return out, nil
}

func mixSeed(seed uint64) uint64 {
	return seed ^ 0x9e3779b97f4a7c15
}

func calibrateSurface(
	ctx context.Context,
	cfg CalibrateConfig,
	surface tennisabstract.MatchSurface,
	matches []tennisabstract.CalibrationMatch,
	rates tennisabstract.PlayerRatesMap,
	taClient *tennisabstract.Client,
	rng *rand.Rand,
) (SurfaceCalibration, error) {
	eligible, skipped := filterEligibleMatches(matches, rates)
	log := cfg.log()
	log.Info("surface eligible",
		"surface", surface,
		"matches_included", len(eligible),
		"matches_skipped", skipped,
		"alpha_count", cfg.AlphaMax-cfg.AlphaMin+1,
		"sims_per_match", cfg.Sims,
	)

	grid := make([]AlphaMetrics, 0, cfg.AlphaMax-cfg.AlphaMin+1)
	alphaTotal := cfg.AlphaMax - cfg.AlphaMin + 1

	for alpha := cfg.AlphaMin; alpha <= cfg.AlphaMax; alpha++ {
		alphaIdx := alpha - cfg.AlphaMin + 1
		log.Info("alpha start",
			"surface", surface,
			"alpha", alpha,
			"progress", fmt.Sprintf("%d/%d", alphaIdx, alphaTotal),
		)
		m, err := evaluateAlpha(ctx, cfg, surface, eligible, rates, taClient, float64(alpha), cfg.Sims, rng)
		if err != nil {
			return SurfaceCalibration{}, err
		}
		m.MatchesIncluded = len(eligible)
		m.MatchesSkipped = skipped
		m.Alpha = alpha
		grid = append(grid, m)
		log.Info("alpha done",
			"surface", surface,
			"alpha", alpha,
			"progress", fmt.Sprintf("%d/%d", alphaIdx, alphaTotal),
			"mean_sim_accuracy", fmt.Sprintf("%.4f", m.MeanSimAccuracy),
			"hit_rate", fmt.Sprintf("%.4f", m.HitRate),
		)
	}

	sort.Slice(grid, func(i, j int) bool {
		if grid[i].MeanSimAccuracy != grid[j].MeanSimAccuracy {
			return grid[i].MeanSimAccuracy > grid[j].MeanSimAccuracy
		}
		return grid[i].Alpha > grid[j].Alpha
	})

	best := AlphaMetrics{}
	if len(grid) > 0 {
		best = grid[0]
	}

	return SurfaceCalibration{
		Surface:         surface,
		MatchesIncluded: len(eligible),
		MatchesSkipped:  skipped,
		Best:            best,
		Grid:            grid,
	}, nil
}

func filterEligibleMatches(
	matches []tennisabstract.CalibrationMatch,
	rates tennisabstract.PlayerRatesMap,
) (eligible []tennisabstract.CalibrationMatch, skipped int) {
	for _, m := range matches {
		if _, ok := tennisabstract.MatchPlayerRates(
			context.Background(), m, rates, nil, false, tennisabstract.FormOptions{},
		); ok {
			eligible = append(eligible, m)
		} else {
			skipped++
		}
	}
	return eligible, skipped
}

func evaluateAlpha(
	ctx context.Context,
	cfg CalibrateConfig,
	surface tennisabstract.MatchSurface,
	matches []tennisabstract.CalibrationMatch,
	rates tennisabstract.PlayerRatesMap,
	taClient *tennisabstract.Client,
	alpha float64,
	sims int,
	rng *rand.Rand,
) (AlphaMetrics, error) {
	if len(matches) == 0 {
		return AlphaMetrics{}, nil
	}

	log := cfg.log()
	var sumCorrectFrac float64
	var hits, evaluated int
	simsF := float64(sims)
	logEvery := matchLogInterval(len(matches))

	for i, m := range matches {
		if logEvery > 0 && (i == 0 || (i+1)%logEvery == 0 || i+1 == len(matches)) {
			log.Info("match progress",
				"surface", surface,
				"alpha", int(alpha),
				"match", fmt.Sprintf("%d/%d", i+1, len(matches)),
				"players", fmt.Sprintf("%s vs %s", m.PlayerA, m.PlayerB),
			)
		}
		formOpts := tennisabstract.FormOptions{}
		if cfg.UseRecentForm {
			formOpts = tennisabstract.FormOptionsFromEnv(m.Surface)
		}
		playerRates, ok := tennisabstract.MatchPlayerRates(ctx, m, rates, taClient, cfg.UseRecentForm, formOpts)
		if !ok {
			continue
		}
		result, err := tennis.SimulateFresh(m.Format, playerRates, alpha, sims, rng)
		if err != nil {
			return AlphaMetrics{}, fmt.Errorf("simulate %s vs %s: %w", m.PlayerA, m.PlayerB, err)
		}
		winsA := result.WinCount(tennis.A)
		sumCorrectFrac += float64(winsA) / simsF
		if winsA > result.WinCount(tennis.B) {
			hits++
		}
		evaluated++
	}

	if evaluated == 0 {
		return AlphaMetrics{}, nil
	}
	n := float64(evaluated)
	return AlphaMetrics{
		MeanSimAccuracy: sumCorrectFrac / n,
		HitRate:         float64(hits) / n,
	}, nil
}

// matchLogInterval returns how often to log match progress (0 = only alpha-level logs).
func matchLogInterval(n int) int {
	switch {
	case n <= 20:
		return 0
	case n <= 200:
		return 50
	default:
		return 250
	}
}

// WriteCalibrationReport writes the full grid as JSON.
func WriteCalibrationReport(w io.Writer, cfg CalibrateConfig, surfaces []SurfaceCalibration) error {
	rep := CalibrationReport{Config: cfg, Surfaces: surfaces}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rep)
}

// PrintCalibrationSummary writes human-readable per-surface tables to w.
func PrintCalibrationSummary(w io.Writer, surfaces []SurfaceCalibration) {
	for _, sc := range surfaces {
		fmt.Fprintf(w, "Surface: %s (n=%d matches, skipped=%d missing rates)\n",
			sc.Surface, sc.MatchesIncluded, sc.MatchesSkipped)
		if sc.MatchesIncluded == 0 {
			fmt.Fprintln(w)
			continue
		}
		fmt.Fprintf(w, "Best alpha: %d  mean_sim_accuracy=%.3f  hit_rate=%.3f\n\n",
			sc.Best.Alpha, sc.Best.MeanSimAccuracy, sc.Best.HitRate)
		fmt.Fprintln(w, "alpha  mean_sim_accuracy  hit_rate")
		for _, row := range sc.Grid {
			fmt.Fprintf(w, "%-5d  %.3f               %.3f\n",
				row.Alpha, row.MeanSimAccuracy, row.HitRate)
		}
		fmt.Fprintln(w)
	}
}

// WriteCalibrationReportFile writes JSON to path.
func WriteCalibrationReportFile(path string, cfg CalibrateConfig, surfaces []SurfaceCalibration) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := WriteCalibrationReport(f, cfg, surfaces); err != nil {
		return err
	}
	return f.Close()
}
