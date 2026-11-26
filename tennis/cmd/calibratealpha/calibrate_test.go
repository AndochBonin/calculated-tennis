package main

import (
	"context"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AndochBonin/calculated-tennis/tennis/tennis"
	"github.com/AndochBonin/calculated-tennis/tennis/tennisabstract"
)

func TestEvaluateAlpha_metrics(t *testing.T) {
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
	rng := rand.New(rand.NewPCG(42, mixSeed(42)))

	cfg := CalibrateConfig{Sims: 100, AlphaMin: 1, AlphaMax: 1, Seed: 42}
	m, err := evaluateAlpha(context.Background(), cfg, tennisabstract.SurfaceHard, matches, rates, nil, 1, 100, rng)
	if err != nil {
		t.Fatalf("evaluateAlpha: %v", err)
	}
	// Seed 42: 39/100 sim wins for A → mean 0.39; B is plurality so hit_rate 0.
	const wantMean, wantHit = 0.39, 0.0
	if math.Abs(m.MeanSimAccuracy-wantMean) > 1e-12 {
		t.Fatalf("MeanSimAccuracy = %v, want %v", m.MeanSimAccuracy, wantMean)
	}
	if math.Abs(m.HitRate-wantHit) > 1e-12 {
		t.Fatalf("HitRate = %v, want %v", m.HitRate, wantHit)
	}
}

func TestEvaluateAlpha_excludesSkippedFromDenominator(t *testing.T) {
	t.Parallel()

	evaluated := []tennisabstract.CalibrationMatch{{
		PlayerASlug: "DaniilMedvedev",
		PlayerBSlug: "JannikSinner",
		Format:      tennis.DefaultFormat(),
		TourneyDate: 20250115,
	}}
	skipped := tennisabstract.CalibrationMatch{
		PlayerASlug: "UnknownA",
		PlayerBSlug: "UnknownB",
		Format:      tennis.DefaultFormat(),
		TourneyDate: 20250116,
	}
	rates := tennisabstract.PlayerRatesMap{
		"DaniilMedvedev": {Hold2024: 0.801, Break2024: 0.27},
		"JannikSinner":   {Hold2024: 0.85, Break2024: 0.25},
	}
	cfg := CalibrateConfig{Sims: 100, Seed: 42}

	want, err := evaluateAlpha(context.Background(), cfg, tennisabstract.SurfaceHard, evaluated, rates, nil, 1, 100, rand.New(rand.NewPCG(42, mixSeed(42))))
	if err != nil {
		t.Fatalf("single match evaluateAlpha: %v", err)
	}

	got, err := evaluateAlpha(context.Background(), cfg, tennisabstract.SurfaceHard, append(evaluated, skipped), rates, nil, 1, 100, rand.New(rand.NewPCG(42, mixSeed(42))))
	if err != nil {
		t.Fatalf("evaluateAlpha with skipped match: %v", err)
	}
	if math.Abs(got.MeanSimAccuracy-want.MeanSimAccuracy) > 1e-12 {
		t.Fatalf("MeanSimAccuracy = %v, want %v (skipped match must not deflate denominator)", got.MeanSimAccuracy, want.MeanSimAccuracy)
	}
	if math.Abs(got.HitRate-want.HitRate) > 1e-12 {
		t.Fatalf("HitRate = %v, want %v", got.HitRate, want.HitRate)
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

	cfg := CalibrateConfig{
		MatchesPath: csvPath,
		RatesPath:   ratesPath,
		Sims:        100,
		AlphaMin:    1,
		AlphaMax:    3,
		Seed:        42,
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
		if sc.MatchesSkipped != 0 {
			t.Fatalf("surface %s: matches_skipped=%d, want 0", sc.Surface, sc.MatchesSkipped)
		}
		if len(sc.Grid) != 3 {
			t.Fatalf("surface %s: grid len=%d, want 3", sc.Surface, len(sc.Grid))
		}
		if sc.Best.Alpha < cfg.AlphaMin || sc.Best.Alpha > cfg.AlphaMax {
			t.Fatalf("surface %s: best alpha %d out of range", sc.Surface, sc.Best.Alpha)
		}
		for i := 1; i < len(sc.Grid); i++ {
			if sc.Grid[i].MeanSimAccuracy > sc.Grid[i-1].MeanSimAccuracy {
				t.Fatalf("surface %s: grid not sorted by mean_sim_accuracy desc", sc.Surface)
			}
		}
		for _, row := range sc.Grid {
			if row.MeanSimAccuracy < 0 || row.MeanSimAccuracy > 1 {
				t.Fatalf("surface %s alpha %d: mean_sim_accuracy=%v out of [0,1]", sc.Surface, row.Alpha, row.MeanSimAccuracy)
			}
			if row.HitRate < 0 || row.HitRate > 1 {
				t.Fatalf("surface %s alpha %d: hit_rate=%v out of [0,1]", sc.Surface, row.Alpha, row.HitRate)
			}
		}
	}

	// Same seed → identical best metrics on Hard.
	cfg2 := cfg
	surfaces2, err := RunCalibration(cfg2)
	if err != nil {
		t.Fatal(err)
	}
	var hard1, hard2 SurfaceCalibration
	for _, sc := range surfaces {
		if sc.Surface == tennisabstract.SurfaceHard {
			hard1 = sc
		}
	}
	for _, sc := range surfaces2 {
		if sc.Surface == tennisabstract.SurfaceHard {
			hard2 = sc
		}
	}
	if math.Abs(hard1.Best.MeanSimAccuracy-hard2.Best.MeanSimAccuracy) > 1e-12 {
		t.Fatalf("non-reproducible mean_sim_accuracy: %v vs %v", hard1.Best.MeanSimAccuracy, hard2.Best.MeanSimAccuracy)
	}

	// Hard / alpha=1 matches TestEvaluateAlpha_metrics golden (seed 42, 100 sims).
	var hardAlpha1 AlphaMetrics
	for _, row := range hard1.Grid {
		if row.Alpha == 1 {
			hardAlpha1 = row
			break
		}
	}
	if math.Abs(hardAlpha1.MeanSimAccuracy-0.39) > 1e-12 || math.Abs(hardAlpha1.HitRate) > 1e-12 {
		t.Fatalf("Hard alpha=1: mean=%v hit=%v, want 0.39 / 0.0", hardAlpha1.MeanSimAccuracy, hardAlpha1.HitRate)
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

	surfaces, err := RunCalibration(CalibrateConfig{
		MatchesPath: csvPath,
		RatesPath:   ratesPath,
		Sims:        10,
		AlphaMin:    1,
		AlphaMax:    1,
		Seed:        1,
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
