package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"os"
	"runtime"
	"sort"

	"github.com/AndochBonin/calculated-tennis/tennis/tennis"
	"github.com/AndochBonin/calculated-tennis/tennis/tennisabstract"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

const calibrationAlpha = 1.0

// CalibrateFormConfig controls the form-parameter grid search.
type CalibrateFormConfig struct {
	MatchesPath string
	RatesPath   string
	Sims        int
	Seed        uint64
	Workers     int // concurrent grid points; <=0 defaults to runtime.NumCPU()
	GridBounds  tennisabstract.FormGridBounds
	Log         *slog.Logger
}

func (cfg CalibrateFormConfig) log() *slog.Logger {
	if cfg.Log != nil {
		return cfg.Log
	}
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// FormMetrics holds aggregate metrics for one form grid point.
type FormMetrics struct {
	HalfLifeMatches float64 `json:"half_life_matches"`
	FormWeightMax   float64 `json:"form_weight_max"`
	FormRatioMin    float64 `json:"form_ratio_min"`
	FormRatioMax    float64 `json:"form_ratio_max"`
	MeanSimAccuracy float64 `json:"mean_sim_accuracy"`
	HitRate         float64 `json:"hit_rate"`
	MatchesIncluded int     `json:"matches_included"`
	MatchesSkipped  int     `json:"matches_skipped"`
}

// SurfaceFormCalibration is the full form grid for one court surface.
type SurfaceFormCalibration struct {
	Surface         tennisabstract.MatchSurface `json:"surface"`
	MatchesIncluded int                         `json:"matches_included"`
	MatchesSkipped  int                         `json:"matches_skipped"`
	Best            FormMetrics                 `json:"best"`
	Grid            []FormMetrics               `json:"grid"`
}

// FormCalibrationReport is the machine-readable full grid (-json-out).
type FormCalibrationReport struct {
	Config   CalibrateFormConfig      `json:"config"`
	Surfaces []SurfaceFormCalibration `json:"surfaces"`
}

var formCalibrationSurfaces = []tennisabstract.MatchSurface{
	tennisabstract.SurfaceHard,
	tennisabstract.SurfaceClay,
	tennisabstract.SurfaceGrass,
}

// RunCalibration loads matches and rates, runs the form grid per surface, and returns results.
func RunCalibration(cfg CalibrateFormConfig) ([]SurfaceFormCalibration, error) {
	if cfg.Sims <= 0 {
		return nil, fmt.Errorf("sims must be positive")
	}
	if cfg.GridBounds.HalfLifeStep <= 0 {
		cfg.GridBounds = tennisabstract.DefaultFormGridBounds()
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

	opts := tennisabstract.CareerClientOptionsFromEnv()
	taClient := tennisabstract.NewClient(opts...)
	log.Info("tennis abstract client ready", "career_dir", tennisabstract.CareerCacheDirFromEnv())

	combos := tennisabstract.FormGridCombos(cfg.GridBounds)
	if len(combos) == 0 {
		return nil, fmt.Errorf("form grid produced no combinations")
	}

	ctx := context.Background()

	out := make([]SurfaceFormCalibration, 0, len(formCalibrationSurfaces))
	for _, surface := range formCalibrationSurfaces {
		log.Info("surface start", "surface", surface, "matches_in_csv", len(load.BySurface[surface]))
		sc, err := calibrateSurface(ctx, cfg, surface, load.BySurface[surface], rates, taClient, combos)
		if err != nil {
			return nil, fmt.Errorf("surface %s: %w", surface, err)
		}
		log.Info("surface done",
			"surface", sc.Surface,
			"matches_included", sc.MatchesIncluded,
			"matches_skipped", sc.MatchesSkipped,
			"best_half_life", sc.Best.HalfLifeMatches,
			"best_weight_max", sc.Best.FormWeightMax,
			"best_ratio_min", sc.Best.FormRatioMin,
			"best_ratio_max", sc.Best.FormRatioMax,
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

// gridSeed derives the PCG stream seed for one grid index. Index 0 matches the
// legacy single-stream mixSeed(seed) so golden tests on the first combo stay stable.
func gridSeed(seed uint64, gridIdx int) uint64 {
	if gridIdx == 0 {
		return mixSeed(seed)
	}
	return mixSeed(seed ^ uint64(gridIdx)*0x9e3779b97f4a7c15)
}

func calibrationWorkers(workers int) int {
	if workers > 0 {
		return workers
	}
	return runtime.NumCPU()
}

func calibrateSurface(
	ctx context.Context,
	cfg CalibrateFormConfig,
	surface tennisabstract.MatchSurface,
	matches []tennisabstract.CalibrationMatch,
	rates tennisabstract.PlayerRatesMap,
	taClient *tennisabstract.Client,
	combos []tennisabstract.FormOptions,
) (SurfaceFormCalibration, error) {
	eligible, skipped := filterEligibleMatches(matches, rates)
	log := cfg.log()
	log.Info("surface eligible",
		"surface", surface,
		"matches_included", len(eligible),
		"matches_skipped", skipped,
		"grid_points", len(combos),
		"sims_per_match", cfg.Sims,
	)

	log.Info("preloading recent results", "surface", surface, "matches", len(eligible))
	preload := tennisabstract.PreloadCalibrationMatchRecent(ctx, taClient, eligible, 0)
	return calibrateSurfacePreloaded(ctx, cfg, surface, eligible, skipped, rates, preload, combos)
}

func calibrateSurfacePreloaded(
	ctx context.Context,
	cfg CalibrateFormConfig,
	surface tennisabstract.MatchSurface,
	eligible []tennisabstract.CalibrationMatch,
	skipped int,
	rates tennisabstract.PlayerRatesMap,
	preload tennisabstract.CalibrationRecentPreload,
	combos []tennisabstract.FormOptions,
) (SurfaceFormCalibration, error) {
	grid := make([]FormMetrics, len(combos))
	workers := calibrationWorkers(cfg.Workers)
	sem := semaphore.NewWeighted(int64(workers))

	g, ctx := errgroup.WithContext(ctx)
	for i, formOpts := range combos {
		if err := sem.Acquire(ctx, 1); err != nil {
			return SurfaceFormCalibration{}, err
		}
		i, formOpts := i, formOpts
		g.Go(func() error {
			defer sem.Release(1)

			rng := rand.New(rand.NewPCG(cfg.Seed, gridSeed(cfg.Seed, i)))
			m, err := evaluateFormPoint(ctx, cfg, surface, eligible, rates, preload, formOpts, cfg.Sims, rng)
			if err != nil {
				return err
			}
			m.MatchesIncluded = len(eligible)
			m.MatchesSkipped = skipped
			m.HalfLifeMatches = formOpts.HalfLifeMatches
			m.FormWeightMax = formOpts.FormWeightMax
			m.FormRatioMin = formOpts.FormRatioMin
			m.FormRatioMax = formOpts.FormRatioMax
			grid[i] = m
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return SurfaceFormCalibration{}, err
	}

	sortFormMetrics(grid)

	best := FormMetrics{}
	if len(grid) > 0 {
		best = grid[0]
	}

	return SurfaceFormCalibration{
		Surface:         surface,
		MatchesIncluded: len(eligible),
		MatchesSkipped:  skipped,
		Best:            best,
		Grid:            grid,
	}, nil
}

func sortFormMetrics(grid []FormMetrics) {
	sort.Slice(grid, func(i, j int) bool {
		if grid[i].MeanSimAccuracy != grid[j].MeanSimAccuracy {
			return grid[i].MeanSimAccuracy > grid[j].MeanSimAccuracy
		}
		if grid[i].HitRate != grid[j].HitRate {
			return grid[i].HitRate > grid[j].HitRate
		}
		if grid[i].HalfLifeMatches != grid[j].HalfLifeMatches {
			return grid[i].HalfLifeMatches < grid[j].HalfLifeMatches
		}
		if grid[i].FormWeightMax != grid[j].FormWeightMax {
			return grid[i].FormWeightMax > grid[j].FormWeightMax
		}
		return grid[i].FormRatioMin < grid[j].FormRatioMin
	})
}

func filterEligibleMatches(
	matches []tennisabstract.CalibrationMatch,
	rates tennisabstract.PlayerRatesMap,
) (eligible []tennisabstract.CalibrationMatch, skipped int) {
	for _, m := range matches {
		if _, okA := rates[m.PlayerASlug]; !okA {
			skipped++
			continue
		}
		if _, okB := rates[m.PlayerBSlug]; !okB {
			skipped++
			continue
		}
		eligible = append(eligible, m)
	}
	return eligible, skipped
}

func evaluateFormPoint(
	ctx context.Context,
	cfg CalibrateFormConfig,
	surface tennisabstract.MatchSurface,
	matches []tennisabstract.CalibrationMatch,
	rates tennisabstract.PlayerRatesMap,
	preload tennisabstract.CalibrationRecentPreload,
	formOpts tennisabstract.FormOptions,
	sims int,
	rng *rand.Rand,
) (FormMetrics, error) {
	if len(matches) == 0 {
		return FormMetrics{}, nil
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
				"half_life", formOpts.HalfLifeMatches,
				"match", fmt.Sprintf("%d/%d", i+1, len(matches)),
				"players", fmt.Sprintf("%s vs %s", m.PlayerA, m.PlayerB),
			)
		}
		playerRates, ok := tennisabstract.MatchPlayerRatesFromPreload(m, rates, preload, i, formOpts)
		if !ok {
			continue
		}
		result, err := tennis.SimulateFresh(m.Format, playerRates, calibrationAlpha, sims, rng)
		if err != nil {
			return FormMetrics{}, fmt.Errorf("simulate %s vs %s: %w", m.PlayerA, m.PlayerB, err)
		}
		winsA := result.WinCount(tennis.A)
		sumCorrectFrac += float64(winsA) / simsF
		if winsA > result.WinCount(tennis.B) {
			hits++
		}
		evaluated++
	}

	if evaluated == 0 {
		return FormMetrics{}, nil
	}
	n := float64(evaluated)
	return FormMetrics{
		MeanSimAccuracy: sumCorrectFrac / n,
		HitRate:         float64(hits) / n,
	}, nil
}

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

// WriteFormCalibrationReport writes the full grid as JSON.
func WriteFormCalibrationReport(w io.Writer, cfg CalibrateFormConfig, surfaces []SurfaceFormCalibration) error {
	rep := FormCalibrationReport{Config: cfg, Surfaces: surfaces}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rep)
}

// PrintFormCalibrationSummary writes human-readable per-surface tables to w.
func PrintFormCalibrationSummary(w io.Writer, surfaces []SurfaceFormCalibration) {
	for _, sc := range surfaces {
		fmt.Fprintf(w, "Surface: %s (n=%d matches, skipped=%d missing rates)\n",
			sc.Surface, sc.MatchesIncluded, sc.MatchesSkipped)
		if sc.MatchesIncluded == 0 {
			fmt.Fprintln(w)
			continue
		}
		fmt.Fprintf(w, "Best: half_life=%.0f weight_max=%.2f ratio_min=%.2f ratio_max=%.2f  mean_sim_accuracy=%.3f  hit_rate=%.3f\n\n",
			sc.Best.HalfLifeMatches, sc.Best.FormWeightMax, sc.Best.FormRatioMin, sc.Best.FormRatioMax,
			sc.Best.MeanSimAccuracy, sc.Best.HitRate)
		topN := min(10, len(sc.Grid))
		fmt.Fprintf(w, "Top %d of %d grid points (full grid in -json-out):\n", topN, len(sc.Grid))
		fmt.Fprintln(w, "half_life  weight_max  ratio_min  ratio_max  mean_sim_accuracy  hit_rate")
		for _, row := range sc.Grid[:topN] {
			fmt.Fprintf(w, "%-9.0f  %-10.2f  %-9.2f  %-9.2f  %.3f               %.3f\n",
				row.HalfLifeMatches, row.FormWeightMax, row.FormRatioMin, row.FormRatioMax,
				row.MeanSimAccuracy, row.HitRate)
		}
		fmt.Fprintln(w)
	}
}

// WriteFormCalibrationReportFile writes JSON to path.
func WriteFormCalibrationReportFile(path string, cfg CalibrateFormConfig, surfaces []SurfaceFormCalibration) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := WriteFormCalibrationReport(f, cfg, surfaces); err != nil {
		return err
	}
	return f.Close()
}
