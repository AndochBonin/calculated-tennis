package main

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AndochBonin/E3/tennis/tennis"
	"github.com/AndochBonin/E3/tennis/tennisabstract"
)

func TestFormGridCombos_defaultBounds(t *testing.T) {
	t.Parallel()

	combos := tennisabstract.FormGridCombos(tennisabstract.DefaultFormGridBounds())
	if len(combos) != 1925 {
		t.Fatalf("combos len=%d, want 1925", len(combos))
	}
	for i, o := range combos {
		if o.FormRatioMin >= o.FormRatioMax {
			t.Fatalf("combo %d: ratio_min=%v >= ratio_max=%v", i, o.FormRatioMin, o.FormRatioMax)
		}
	}
}

func TestSortFormMetrics(t *testing.T) {
	t.Parallel()

	grid := []FormMetrics{
		{MeanSimAccuracy: 0.40, HitRate: 0.5, HalfLifeMatches: 5, FormWeightMax: 0.2, FormRatioMin: 0.9},
		{MeanSimAccuracy: 0.50, HitRate: 0.0, HalfLifeMatches: 10, FormWeightMax: 0.1, FormRatioMin: 0.88},
		{MeanSimAccuracy: 0.50, HitRate: 0.5, HalfLifeMatches: 3, FormWeightMax: 0.3, FormRatioMin: 0.92},
		{MeanSimAccuracy: 0.50, HitRate: 0.5, HalfLifeMatches: 3, FormWeightMax: 0.4, FormRatioMin: 0.90},
	}
	sortFormMetrics(grid)

	if grid[0].MeanSimAccuracy != 0.50 || grid[0].HitRate != 0.5 || grid[0].HalfLifeMatches != 3 || grid[0].FormWeightMax != 0.4 {
		t.Fatalf("first row: %+v", grid[0])
	}
	if grid[1].FormWeightMax != 0.3 {
		t.Fatalf("second row weight: %+v", grid[1])
	}
	if grid[2].HitRate != 0.0 {
		t.Fatalf("third row hit: %+v", grid[2])
	}
	if grid[3].MeanSimAccuracy != 0.40 {
		t.Fatalf("last row: %+v", grid[3])
	}
}

func TestEvaluateFormPoint_metrics(t *testing.T) {
	t.Parallel()

	matches := []tennisabstract.CalibrationMatch{{
		PlayerASlug: "DaniilMedvedev",
		PlayerBSlug: "JannikSinner",
		Format:      tennis.DefaultFormat(),
		TourneyDate: 20250115,
	}}
	rates := tennisabstract.PlayerRatesMap{
		"DaniilMedvedev": {Hold2024: 0.801, Break2024: 0.27},
		"JannikSinner":   {Hold2024: 0.85, Break2024: 0.25},
	}
	preload := tennisabstract.CalibrationRecentPreload{
		PerMatch: []tennisabstract.CalibrationMatchRecent{{OK: true}},
	}
	rng := rand.New(rand.NewPCG(42, mixSeed(42)))
	cfg := CalibrateFormConfig{Sims: 100, Seed: 42}
	formOpts := tennisabstract.FormOptions{
		HalfLifeMatches: 5,
		FormWeightMax:   0,
		FormRatioMin:    0.92,
		FormRatioMax:    1.08,
	}

	m, err := evaluateFormPoint(context.Background(), cfg, tennisabstract.SurfaceHard, matches, rates, preload, formOpts, 100, rng)
	if err != nil {
		t.Fatalf("evaluateFormPoint: %v", err)
	}
	const wantMean, wantHit = 0.39, 0.0
	if math.Abs(m.MeanSimAccuracy-wantMean) > 1e-12 {
		t.Fatalf("MeanSimAccuracy = %v, want %v", m.MeanSimAccuracy, wantMean)
	}
	if math.Abs(m.HitRate-wantHit) > 1e-12 {
		t.Fatalf("HitRate = %v, want %v", m.HitRate, wantHit)
	}
}

func TestEvaluateFormPoint_usesPreload(t *testing.T) {
	t.Parallel()

	matches := []tennisabstract.CalibrationMatch{{
		PlayerASlug: "DaniilMedvedev",
		PlayerBSlug: "JannikSinner",
		Format:      tennis.DefaultFormat(),
		TourneyDate: 20250115,
	}}
	rates := tennisabstract.PlayerRatesMap{
		"DaniilMedvedev": {Hold2024: 0.801, Break2024: 0.27},
		"JannikSinner":   {Hold2024: 0.85, Break2024: 0.25},
	}
	preload := tennisabstract.CalibrationRecentPreload{
		PerMatch: []tennisabstract.CalibrationMatchRecent{{OK: false}},
	}
	rng := rand.New(rand.NewPCG(42, mixSeed(42)))
	cfg := CalibrateFormConfig{Sims: 50, Seed: 42}
	formOpts := tennisabstract.FormOptions{
		HalfLifeMatches: 5,
		FormWeightMax:   0,
		FormRatioMin:    0.92,
		FormRatioMax:    1.08,
	}

	m, err := evaluateFormPoint(context.Background(), cfg, tennisabstract.SurfaceHard, matches, rates, preload, formOpts, 50, rng)
	if err != nil {
		t.Fatalf("evaluateFormPoint: %v", err)
	}
	if m.MeanSimAccuracy != 0 || m.HitRate != 0 {
		t.Fatalf("skipped preload should yield zero metrics: %+v", m)
	}
}

func TestEvaluateFormPoint_excludesSkippedFromDenominator(t *testing.T) {
	t.Parallel()

	evaluated := []tennisabstract.CalibrationMatch{{
		PlayerASlug: "DaniilMedvedev",
		PlayerBSlug: "JannikSinner",
		Format:      tennis.DefaultFormat(),
		TourneyDate: 20250115,
	}}
	skipped := tennisabstract.CalibrationMatch{
		PlayerASlug: "DaniilMedvedev",
		PlayerBSlug: "JannikSinner",
		Format:      tennis.DefaultFormat(),
		TourneyDate: 20250116,
	}
	rates := tennisabstract.PlayerRatesMap{
		"DaniilMedvedev": {Hold2024: 0.801, Break2024: 0.27},
		"JannikSinner":   {Hold2024: 0.85, Break2024: 0.25},
	}
	formOpts := tennisabstract.FormOptions{
		HalfLifeMatches: 5,
		FormWeightMax:   0,
		FormRatioMin:    0.92,
		FormRatioMax:    1.08,
	}
	cfg := CalibrateFormConfig{Sims: 100, Seed: 42}

	wantPreload := tennisabstract.CalibrationRecentPreload{
		PerMatch: []tennisabstract.CalibrationMatchRecent{{OK: true}},
	}
	want, err := evaluateFormPoint(context.Background(), cfg, tennisabstract.SurfaceHard, evaluated, rates, wantPreload, formOpts, 100, rand.New(rand.NewPCG(42, mixSeed(42))))
	if err != nil {
		t.Fatalf("single match evaluateFormPoint: %v", err)
	}

	bothPreload := tennisabstract.CalibrationRecentPreload{
		PerMatch: []tennisabstract.CalibrationMatchRecent{
			{OK: true},
			{OK: false},
		},
	}
	got, err := evaluateFormPoint(context.Background(), cfg, tennisabstract.SurfaceHard, append(evaluated, skipped), rates, bothPreload, formOpts, 100, rand.New(rand.NewPCG(42, mixSeed(42))))
	if err != nil {
		t.Fatalf("evaluateFormPoint with skipped match: %v", err)
	}
	if math.Abs(got.MeanSimAccuracy-want.MeanSimAccuracy) > 1e-12 {
		t.Fatalf("MeanSimAccuracy = %v, want %v (skipped match must not deflate denominator)", got.MeanSimAccuracy, want.MeanSimAccuracy)
	}
	if math.Abs(got.HitRate-want.HitRate) > 1e-12 {
		t.Fatalf("HitRate = %v, want %v", got.HitRate, want.HitRate)
	}
}

func TestCalibrateSurface_smoke(t *testing.T) {
	t.Parallel()

	matches := []tennisabstract.CalibrationMatch{{
		PlayerA:     "Daniil Medvedev",
		PlayerB:     "Jannik Sinner",
		PlayerASlug: "DaniilMedvedev",
		PlayerBSlug: "JannikSinner",
		Format:      tennis.DefaultFormat(),
		TourneyDate: 20250115,
	}}
	rates := tennisabstract.PlayerRatesMap{
		"DaniilMedvedev": {Hold2024: 0.801, Break2024: 0.27},
		"JannikSinner":   {Hold2024: 0.85, Break2024: 0.25},
	}
	eligible, skipped := filterEligibleMatches(matches, rates)
	if skipped != 0 || len(eligible) != 1 {
		t.Fatalf("eligible=%d skipped=%d", len(eligible), skipped)
	}

	preload := tennisabstract.CalibrationRecentPreload{
		PerMatch: []tennisabstract.CalibrationMatchRecent{{OK: true}},
	}
	combos := tennisabstract.FormGridCombos(tennisabstract.FormGridBounds{
		HalfLifeStart: 5, HalfLifeStop: 7, HalfLifeStep: 2,
		WeightMaxStart: 0, WeightMaxStop: 0, WeightMaxStep: 0.10,
		RatioMinStart: 0.92, RatioMinStop: 0.92, RatioMinStep: 0.04,
		RatioMaxStart: 1.08, RatioMaxStop: 1.08, RatioMaxStep: 0.04,
	})
	if len(combos) != 2 {
		t.Fatalf("combos len=%d, want 2", len(combos))
	}

	cfg := CalibrateFormConfig{Sims: 100, Seed: 42}

	sc, err := calibrateSurfacePreloaded(
		context.Background(), cfg, tennisabstract.SurfaceHard,
		eligible, skipped, rates, preload, combos,
	)
	if err != nil {
		t.Fatalf("calibrateSurfacePreloaded: %v", err)
	}
	if sc.MatchesIncluded != 1 || sc.MatchesSkipped != 0 {
		t.Fatalf("included=%d skipped=%d", sc.MatchesIncluded, sc.MatchesSkipped)
	}
	if len(sc.Grid) != 2 {
		t.Fatalf("grid len=%d, want 2", len(sc.Grid))
	}
	for i := 1; i < len(sc.Grid); i++ {
		if sc.Grid[i].MeanSimAccuracy > sc.Grid[i-1].MeanSimAccuracy {
			t.Fatal("grid not sorted by mean_sim_accuracy desc")
		}
		if sc.Grid[i].MeanSimAccuracy == sc.Grid[i-1].MeanSimAccuracy && sc.Grid[i].HitRate > sc.Grid[i-1].HitRate {
			t.Fatal("grid not sorted by hit_rate desc on tie")
		}
	}
	for _, row := range sc.Grid {
		if row.MeanSimAccuracy < 0 || row.MeanSimAccuracy > 1 {
			t.Fatalf("mean_sim_accuracy=%v out of [0,1]", row.MeanSimAccuracy)
		}
	}
	if sc.Best.HalfLifeMatches != sc.Grid[0].HalfLifeMatches {
		t.Fatalf("best %+v != grid[0] %+v", sc.Best, sc.Grid[0])
	}

	// Same seed → identical metrics (half-life 5 point matches evaluateFormPoint golden).
	var hl5 FormMetrics
	for _, row := range sc.Grid {
		if row.HalfLifeMatches == 5 {
			hl5 = row
			break
		}
	}
	if math.Abs(hl5.MeanSimAccuracy-0.39) > 1e-12 || math.Abs(hl5.HitRate) > 1e-12 {
		t.Fatalf("half_life=5: mean=%v hit=%v, want 0.39 / 0.0", hl5.MeanSimAccuracy, hl5.HitRate)
	}
}

func TestCalibrateSurface_parallelMatchesSerial(t *testing.T) {
	t.Parallel()

	matches := []tennisabstract.CalibrationMatch{{
		PlayerA:     "Daniil Medvedev",
		PlayerB:     "Jannik Sinner",
		PlayerASlug: "DaniilMedvedev",
		PlayerBSlug: "JannikSinner",
		Format:      tennis.DefaultFormat(),
		TourneyDate: 20250115,
	}}
	rates := tennisabstract.PlayerRatesMap{
		"DaniilMedvedev": {Hold2024: 0.801, Break2024: 0.27},
		"JannikSinner":   {Hold2024: 0.85, Break2024: 0.25},
	}
	eligible, skipped := filterEligibleMatches(matches, rates)
	preload := tennisabstract.CalibrationRecentPreload{
		PerMatch: []tennisabstract.CalibrationMatchRecent{{OK: true}},
	}
	combos := tennisabstract.FormGridCombos(tennisabstract.FormGridBounds{
		HalfLifeStart: 5, HalfLifeStop: 7, HalfLifeStep: 2,
		WeightMaxStart: 0, WeightMaxStop: 0, WeightMaxStep: 0.10,
		RatioMinStart: 0.92, RatioMinStop: 0.92, RatioMinStep: 0.04,
		RatioMaxStart: 1.08, RatioMaxStop: 1.08, RatioMaxStep: 0.04,
	})
	if len(combos) != 2 {
		t.Fatalf("combos len=%d, want 2", len(combos))
	}

	ctx := context.Background()
	base := CalibrateFormConfig{Sims: 100, Seed: 42}

	run := func(workers int) SurfaceFormCalibration {
		t.Helper()
		cfg := base
		cfg.Workers = workers
		sc, err := calibrateSurfacePreloaded(
			ctx, cfg, tennisabstract.SurfaceHard,
			eligible, skipped, rates, preload, combos,
		)
		if err != nil {
			t.Fatalf("workers=%d: %v", workers, err)
		}
		return sc
	}

	serial := run(1)
	parallel := run(4)

	if len(parallel.Grid) != len(serial.Grid) {
		t.Fatalf("grid len parallel=%d serial=%d", len(parallel.Grid), len(serial.Grid))
	}
	for i := range serial.Grid {
		a, b := serial.Grid[i], parallel.Grid[i]
		if a.HalfLifeMatches != b.HalfLifeMatches ||
			a.FormWeightMax != b.FormWeightMax ||
			a.FormRatioMin != b.FormRatioMin ||
			a.FormRatioMax != b.FormRatioMax {
			t.Fatalf("grid[%d] form params: serial=%+v parallel=%+v", i, a, b)
		}
		if math.Abs(a.MeanSimAccuracy-b.MeanSimAccuracy) > 1e-12 {
			t.Fatalf("grid[%d] mean: serial=%v parallel=%v", i, a.MeanSimAccuracy, b.MeanSimAccuracy)
		}
		if math.Abs(a.HitRate-b.HitRate) > 1e-12 {
			t.Fatalf("grid[%d] hit: serial=%v parallel=%v", i, a.HitRate, b.HitRate)
		}
	}
}

func TestRunCalibration_smoke(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	csvPath := filepath.Join(dir, "matches.csv")
	csv := strings.Join([]string{
		"tourney_id,winner_name,loser_name,surface,score,best_of,tourney_date",
		"1,Daniil Medvedev,Jannik Sinner,Hard,6-4 6-3,3,20250115",
		"2,Carlos Alcaraz,Novak Djokovic,Clay,6-2 6-4,3,20250601",
		"3,Roger Federer,Rafael Nadal,Grass,7-6(5) 6-4,3,20250701",
	}, "\n")
	if err := os.WriteFile(csvPath, []byte(csv), 0o644); err != nil {
		t.Fatal(err)
	}

	ratesPath := filepath.Join(dir, "rates.json")
	rates := tennisabstract.PlayerRatesMap{
		"DaniilMedvedev": {Hold2024: 0.801, Break2024: 0.27},
		"JannikSinner":   {Hold2024: 0.85, Break2024: 0.25},
		"CarlosAlcaraz":  {Hold2024: 0.82, Break2024: 0.28},
		"NovakDjokovic":  {Hold2024: 0.88, Break2024: 0.22},
		"RogerFederer":   {Hold2024: 0.83, Break2024: 0.24},
		"RafaelNadal":    {Hold2024: 0.79, Break2024: 0.30},
	}
	if err := tennisabstract.WritePlayerRatesFile(ratesPath, rates); err != nil {
		t.Fatal(err)
	}

	cfg := CalibrateFormConfig{
		MatchesPath: csvPath,
		RatesPath:   ratesPath,
		Sims:        100,
		Seed:        42,
		GridBounds: tennisabstract.FormGridBounds{
			HalfLifeStart: 5, HalfLifeStop: 5, HalfLifeStep: 1,
			WeightMaxStart: 0, WeightMaxStop: 0, WeightMaxStep: 0.10,
			RatioMinStart: 0.92, RatioMinStop: 0.92, RatioMinStep: 0.04,
			RatioMaxStart: 1.08, RatioMaxStop: 1.08, RatioMaxStep: 0.04,
		},
	}

	if testing.Short() {
		t.Skip("skipping network smoke in -short")
	}

	surfaces, err := RunCalibration(cfg)
	if err != nil {
		t.Fatalf("RunCalibration: %v", err)
	}
	if len(surfaces) != 3 {
		t.Fatalf("got %d surfaces, want 3", len(surfaces))
	}
	for _, sc := range surfaces {
		if sc.MatchesIncluded != 1 {
			t.Fatalf("surface %s: matches_included=%d, want 1", sc.Surface, sc.MatchesIncluded)
		}
		if len(sc.Grid) != 1 {
			t.Fatalf("surface %s: grid len=%d, want 1", sc.Surface, len(sc.Grid))
		}
		for i := 1; i < len(sc.Grid); i++ {
			if sc.Grid[i].MeanSimAccuracy > sc.Grid[i-1].MeanSimAccuracy {
				t.Fatalf("surface %s: grid not sorted by mean_sim_accuracy desc", sc.Surface)
			}
		}
	}
}

func TestRunCalibration_skipsMissingRates(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	csvPath := filepath.Join(dir, "matches.csv")
	csv := "tourney_id,winner_name,loser_name,surface,score,best_of,tourney_date\n" +
		"1,Alice Bob,Carol Dan,Hard,6-4 6-3,3,20250101\n"
	if err := os.WriteFile(csvPath, []byte(csv), 0o644); err != nil {
		t.Fatal(err)
	}
	ratesPath := filepath.Join(dir, "rates.json")
	if err := tennisabstract.WritePlayerRatesFile(ratesPath, tennisabstract.PlayerRatesMap{}); err != nil {
		t.Fatal(err)
	}

	if testing.Short() {
		t.Skip("skipping network smoke in -short")
	}

	surfaces, err := RunCalibration(CalibrateFormConfig{
		MatchesPath: csvPath,
		RatesPath:   ratesPath,
		Sims:        10,
		Seed:        1,
		GridBounds: tennisabstract.FormGridBounds{
			HalfLifeStart: 5, HalfLifeStop: 5, HalfLifeStep: 1,
			WeightMaxStart: 0, WeightMaxStop: 0, WeightMaxStep: 0.10,
			RatioMinStart: 0.92, RatioMinStop: 0.92, RatioMinStep: 0.04,
			RatioMaxStart: 1.08, RatioMaxStop: 1.08, RatioMaxStep: 0.04,
		},
	})
	if err != nil {
		t.Fatalf("RunCalibration: %v", err)
	}
	for _, sc := range surfaces {
		if sc.Surface != tennisabstract.SurfaceHard {
			continue
		}
		if sc.MatchesIncluded != 0 || sc.MatchesSkipped != 1 {
			t.Fatalf("Hard: included=%d skipped=%d, want 0/1", sc.MatchesIncluded, sc.MatchesSkipped)
		}
	}
}

func TestPrintFormCalibrationSummary_top10(t *testing.T) {
	t.Parallel()

	grid := make([]FormMetrics, 15)
	for i := range grid {
		grid[i] = FormMetrics{
			HalfLifeMatches: float64(i + 1),
			FormWeightMax:   0.1,
			FormRatioMin:    0.92,
			FormRatioMax:    1.08,
			MeanSimAccuracy: 0.5 - float64(i)*0.01,
			HitRate:         0.5,
		}
	}
	sortFormMetrics(grid)

	surfaces := []SurfaceFormCalibration{{
		Surface:         tennisabstract.SurfaceHard,
		MatchesIncluded: 100,
		MatchesSkipped:  0,
		Best:            grid[0],
		Grid:            grid,
	}}

	var buf bytes.Buffer
	PrintFormCalibrationSummary(&buf, surfaces)
	out := buf.String()

	if !strings.Contains(out, "Top 10 of 15 grid points (full grid in -json-out):") {
		t.Fatalf("missing top-10 header:\n%s", out)
	}

	const colHeader = "half_life  weight_max  ratio_min  ratio_max  mean_sim_accuracy  hit_rate"
	idx := strings.Index(out, colHeader)
	if idx < 0 {
		t.Fatalf("missing column header:\n%s", out)
	}
	rest := out[idx+len(colHeader)+1:]
	var dataRows int
	for _, line := range strings.Split(rest, "\n") {
		if strings.TrimSpace(line) == "" {
			break
		}
		dataRows++
	}
	if dataRows != 10 {
		t.Fatalf("printed %d grid data rows, want 10:\n%s", dataRows, out)
	}
}

func TestWriteFormCalibrationReport_roundTrip(t *testing.T) {
	t.Parallel()

	cfg := CalibrateFormConfig{Sims: 10, Seed: 1, GridBounds: tennisabstract.DefaultFormGridBounds()}
	surfaces := []SurfaceFormCalibration{{
		Surface:         tennisabstract.SurfaceHard,
		MatchesIncluded: 1,
		Best: FormMetrics{
			HalfLifeMatches: 5, FormWeightMax: 0.3,
			FormRatioMin: 0.92, FormRatioMax: 1.08,
			MeanSimAccuracy: 0.51, HitRate: 1.0,
		},
		Grid: []FormMetrics{{
			HalfLifeMatches: 5, FormWeightMax: 0.3,
			FormRatioMin: 0.92, FormRatioMax: 1.08,
			MeanSimAccuracy: 0.51, HitRate: 1.0, MatchesIncluded: 1,
		}},
	}}

	var buf bytes.Buffer
	if err := WriteFormCalibrationReport(&buf, cfg, surfaces); err != nil {
		t.Fatal(err)
	}
	var decoded FormCalibrationReport
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Surfaces) != 1 || decoded.Surfaces[0].Best.MeanSimAccuracy != 0.51 {
		t.Fatalf("decoded: %+v", decoded)
	}
}
