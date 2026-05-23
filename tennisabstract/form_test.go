package tennisabstract

import (
	"errors"
	"math"
	"os"
	"testing"
	"time"

	"github.com/AndochBonin/polymarket/models"
)

func medvedevStats(t *testing.T) models.PlayerStats {
	t.Helper()
	f, err := os.Open("testdata/player_medvedev.html")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })
	stats, err := ParsePlayerHTML(f, "DaniilMedvedev")
	if err != nil {
		t.Fatalf("ParsePlayerHTML: %v", err)
	}
	return stats
}

func TestAdjustedHoldBreak_MedvedevFixture(t *testing.T) {
	t.Parallel()

	stats := medvedevStats(t)
	rates, err := AdjustedHoldBreak(stats, FormOptions{
		AsOf: time.Date(2026, time.May, 10, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("AdjustedHoldBreak: %v", err)
	}
	if rates.HoldPct <= 0 || rates.HoldPct >= 1 {
		t.Fatalf("HoldPct = %v, want in (0,1)", rates.HoldPct)
	}
	if rates.BreakPct <= 0 || rates.BreakPct >= 1 {
		t.Fatalf("BreakPct = %v, want in (0,1)", rates.BreakPct)
	}
	if rates.FormWeight <= 0 {
		t.Fatal("expected positive form weight with recent DR rows")
	}
	if rates.DRForm <= 0 {
		t.Fatal("expected positive DRForm")
	}
	// Recent DR on the fixture is below season DR → form ratio below 1 → lower adjusted hold.
	if rates.DRForm >= rates.DRSeason {
		t.Fatalf("DRForm %v should be below DRSeason %v on fixture", rates.DRForm, rates.DRSeason)
	}
	if rates.HoldPct >= rates.SeasonHold {
		t.Fatalf("HoldPct %v should be below season baseline %v on fixture", rates.HoldPct, rates.SeasonHold)
	}
}

func TestAdjustedHoldBreak_noRecentDR(t *testing.T) {
	t.Parallel()

	stats := models.PlayerStats{
		TourLevelSeasons: []models.TourLevelSeason{
			{Year: 2026, Matches: 30, HoldPct: 0.80, BreakPct: 0.25, DR: 1.10},
		},
		RecentResults: []models.RecentResult{
			{Date: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), DominanceRatio: nil},
		},
	}
	rates, err := AdjustedHoldBreak(stats, FormOptions{AsOf: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatalf("AdjustedHoldBreak: %v", err)
	}
	if rates.FormWeight != 0 {
		t.Fatalf("FormWeight = %v, want 0", rates.FormWeight)
	}
	if math.Abs(rates.HoldPct-rates.SeasonHold) > 1e-12 {
		t.Fatalf("HoldPct = %v, want season %v", rates.HoldPct, rates.SeasonHold)
	}
}

func TestAdjustedHoldBreak_seasonBlendLowMatches(t *testing.T) {
	t.Parallel()

	stats := models.PlayerStats{
		TourLevelSeasons: []models.TourLevelSeason{
			{Year: 2026, Matches: 10, HoldPct: 0.90, BreakPct: 0.10, DR: 1.20},
			{Year: 2025, Matches: 50, HoldPct: 0.70, BreakPct: 0.30, DR: 1.00},
		},
	}
	rates, err := AdjustedHoldBreak(stats, FormOptions{
		AsOf:               time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		MinSeasonMatches:   20,
		RecentMatchLimit:   0, // no recent rows considered
		FormWeightMax:      0,
	})
	if err != nil {
		t.Fatalf("AdjustedHoldBreak: %v", err)
	}
	// w = 10/60
	wantH := 10.0/60*0.90 + 50.0/60*0.70
	if math.Abs(rates.SeasonHold-wantH) > 1e-9 {
		t.Fatalf("SeasonHold = %v, want %v", rates.SeasonHold, wantH)
	}
	if rates.SeasonMatches != 60 {
		t.Fatalf("SeasonMatches = %d, want 60", rates.SeasonMatches)
	}
}

func TestAdjustedHoldBreak_formMonotonicity(t *testing.T) {
	t.Parallel()

	asOf := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	base := models.PlayerStats{
		TourLevelSeasons: []models.TourLevelSeason{
			{Year: 2026, Matches: 40, HoldPct: 0.75, BreakPct: 0.20, DR: 1.00},
		},
	}
	coldDR := 0.85
	hotDR := 1.25
	mkRecent := func(dr float64) []models.RecentResult {
		var rows []models.RecentResult
		for i := 0; i < 10; i++ {
			d := dr
			rows = append(rows, models.RecentResult{
				Date:           asOf.AddDate(0, 0, -i),
				DominanceRatio: &d,
			})
		}
		return rows
	}

	cold := base
	cold.RecentResults = mkRecent(coldDR)
	hot := base
	hot.RecentResults = mkRecent(hotDR)

	opts := FormOptions{AsOf: asOf}
	coldRates, err := AdjustedHoldBreak(cold, opts)
	if err != nil {
		t.Fatalf("cold: %v", err)
	}
	hotRates, err := AdjustedHoldBreak(hot, opts)
	if err != nil {
		t.Fatalf("hot: %v", err)
	}
	if hotRates.HoldPct <= coldRates.HoldPct {
		t.Fatalf("hot HoldPct %v should exceed cold %v", hotRates.HoldPct, coldRates.HoldPct)
	}
	if hotRates.BreakPct <= coldRates.BreakPct {
		t.Fatalf("hot BreakPct %v should exceed cold %v", hotRates.BreakPct, coldRates.BreakPct)
	}
}

func TestAdjustedHoldBreak_noSeasonData(t *testing.T) {
	t.Parallel()

	_, err := AdjustedHoldBreak(models.PlayerStats{}, FormOptions{
		AsOf: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrNoSeasonData) {
		t.Fatalf("expected ErrNoSeasonData, got %v", err)
	}
}
