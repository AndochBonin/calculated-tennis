package tennisabstract

import (
	"errors"
	"math"
	"os"
	"testing"
	"time"

	"github.com/AndochBonin/calculated-tennis/tennis/models"
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

func TestSeasonHoldBreak_MedvedevFixture(t *testing.T) {
	t.Parallel()

	stats := medvedevStats(t)
	hold, brk, err := SeasonHoldBreak(stats, 2024)
	if err != nil {
		t.Fatalf("SeasonHoldBreak: %v", err)
	}
	// 2024 tour row from testdata/player_medvedev.html: Hold 80.1%, Break 27.0%.
	wantHold, wantBreak := 0.801, 0.27
	if math.Abs(hold-wantHold) > 1e-9 {
		t.Fatalf("hold = %v, want %v", hold, wantHold)
	}
	if math.Abs(brk-wantBreak) > 1e-9 {
		t.Fatalf("break = %v, want %v", brk, wantBreak)
	}
}

func TestSeasonBaseline_MedvedevFixture(t *testing.T) {
	t.Parallel()

	stats := medvedevStats(t)
	hold, brk, dr, err := SeasonBaseline(stats, 2024)
	if err != nil {
		t.Fatalf("SeasonBaseline: %v", err)
	}
	if math.Abs(hold-0.801) > 1e-9 || math.Abs(brk-0.27) > 1e-9 {
		t.Fatalf("hold/break = %v/%v", hold, brk)
	}
	if math.Abs(dr-1.10) > 1e-9 {
		t.Fatalf("dr = %v, want 1.10", dr)
	}
}

func TestSeasonHoldBreak_seasonBlendLowMatches(t *testing.T) {
	t.Parallel()

	stats := models.PlayerStats{
		TourLevelSeasons: []models.TourLevelSeason{
			{Year: 2024, Matches: 10, HoldPct: 0.90, BreakPct: 0.10, DR: 1.20},
			{Year: 2023, Matches: 50, HoldPct: 0.70, BreakPct: 0.30, DR: 1.00},
		},
	}
	hold, brk, err := SeasonHoldBreak(stats, 2024)
	if err != nil {
		t.Fatalf("SeasonHoldBreak: %v", err)
	}
	wantH := 10.0/60*0.90 + 50.0/60*0.70
	wantB := 10.0/60*0.10 + 50.0/60*0.30
	if math.Abs(hold-wantH) > 1e-9 {
		t.Fatalf("hold = %v, want %v", hold, wantH)
	}
	if math.Abs(brk-wantB) > 1e-9 {
		t.Fatalf("break = %v, want %v", brk, wantB)
	}
}

func TestSeasonHoldBreak_noSeasonData(t *testing.T) {
	t.Parallel()

	_, _, err := SeasonHoldBreak(models.PlayerStats{}, 2024)
	if !errors.Is(err, ErrNoSeasonData) {
		t.Fatalf("err = %v, want ErrNoSeasonData", err)
	}
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

func TestAdjustedHoldBreak_explicitZeroFormWeightMax(t *testing.T) {
	t.Parallel()

	asOf := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	dr := 1.15
	var recent []models.RecentResult
	for i := 0; i < 10; i++ {
		d := dr
		recent = append(recent, models.RecentResult{
			Date:           asOf.AddDate(0, 0, -i),
			DominanceRatio: &d,
		})
	}
	stats := models.PlayerStats{
		TourLevelSeasons: []models.TourLevelSeason{
			{Year: 2026, Matches: 40, HoldPct: 0.75, BreakPct: 0.20, DR: 1.00},
		},
		RecentResults: recent,
	}
	rates, err := AdjustedHoldBreak(stats, FormOptions{
		AsOf:            asOf,
		HalfLifeMatches: 5,
		FormWeightMax:   0,
		FormRatioMin:    0.92,
		FormRatioMax:    1.08,
	})
	if err != nil {
		t.Fatalf("AdjustedHoldBreak: %v", err)
	}
	if rates.FormWeight != 0 {
		t.Fatalf("FormWeight = %v, want 0 (explicit FormWeightMax=0)", rates.FormWeight)
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
		AsOf:             time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		MinSeasonMatches: 20,
		RecentMatchLimit: 0, // no recent rows considered
		HalfLifeMatches:  5,
		FormWeightMax:    0,
		FormRatioMin:     0.92,
		FormRatioMax:     1.08,
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

func TestAdjustedHoldBreak_clampsHoldBreakToUnitInterval(t *testing.T) {
	t.Parallel()

	asOf := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	hotDR := 1.5
	stats := models.PlayerStats{
		TourLevelSeasons: []models.TourLevelSeason{
			{Year: 2025, Matches: 40, HoldPct: 0.90, BreakPct: 0.22, DR: 1.0},
		},
		RecentResults: []models.RecentResult{
			{Date: time.Date(2025, 5, 20, 0, 0, 0, 0, time.UTC), DominanceRatio: &hotDR},
		},
	}
	rates, err := AdjustedHoldBreak(stats, FormOptions{
		AsOf:            asOf,
		RecentMatchLimit: 15,
		FormWeightMax:   0.9,
		FormRatioMin:    0.92,
		FormRatioMax:    1.20,
	})
	if err != nil {
		t.Fatalf("AdjustedHoldBreak: %v", err)
	}
	if rates.HoldPct > 1 || rates.HoldPct < 0 || rates.BreakPct > 1 || rates.BreakPct < 0 {
		t.Fatalf("hold/break out of [0,1]: hold=%v break=%v", rates.HoldPct, rates.BreakPct)
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

func TestAdjustedHoldBreak_challengerSupplement(t *testing.T) {
	t.Parallel()

	asOf := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	const chalWeight = 0.7
	opts := FormOptions{
		AsOf:             asOf,
		MinSeasonMatches: 20,
		RecentMatchLimit: 0,
		HalfLifeMatches:  5,
		FormWeightMax:    0,
		FormRatioMin:     0.92,
		FormRatioMax:     1.08,
		ChallengerWeight: chalWeight,
	}

	t.Run("tour thin plus challenger same year", func(t *testing.T) {
		t.Parallel()
		stats := models.PlayerStats{
			TourLevelSeasons: []models.TourLevelSeason{
				{Year: 2026, Matches: 10, HoldPct: 0.80, BreakPct: 0.20, DR: 1.00},
			},
			ChallengerSeasons: []models.TourLevelSeason{
				{Year: 2026, Matches: 20, HoldPct: 0.70, BreakPct: 0.30, DR: 0.90},
			},
		}
		rates, err := AdjustedHoldBreak(stats, opts)
		if err != nil {
			t.Fatalf("AdjustedHoldBreak: %v", err)
		}
		scaledChalH := 0.70 * chalWeight
		w := 10.0 / (10.0 + 20.0)
		wantH := w*0.80 + (1-w)*scaledChalH
		if math.Abs(rates.SeasonHold-wantH) > 1e-9 {
			t.Fatalf("SeasonHold = %v, want %v", rates.SeasonHold, wantH)
		}
		if rates.SeasonMatches != 30 {
			t.Fatalf("SeasonMatches = %d, want 30", rates.SeasonMatches)
		}
	})

	t.Run("tour sufficient ignores challenger", func(t *testing.T) {
		t.Parallel()
		stats := models.PlayerStats{
			TourLevelSeasons: []models.TourLevelSeason{
				{Year: 2026, Matches: 30, HoldPct: 0.80, BreakPct: 0.20, DR: 1.00},
			},
			ChallengerSeasons: []models.TourLevelSeason{
				{Year: 2026, Matches: 20, HoldPct: 0.50, BreakPct: 0.50, DR: 0.50},
			},
		}
		rates, err := AdjustedHoldBreak(stats, opts)
		if err != nil {
			t.Fatalf("AdjustedHoldBreak: %v", err)
		}
		if math.Abs(rates.SeasonHold-0.80) > 1e-9 {
			t.Fatalf("SeasonHold = %v, want tour-only 0.80", rates.SeasonHold)
		}
		if rates.SeasonMatches != 30 {
			t.Fatalf("SeasonMatches = %d, want 30", rates.SeasonMatches)
		}
	})

	t.Run("latest challenger not eval year", func(t *testing.T) {
		t.Parallel()
		stats := models.PlayerStats{
			TourLevelSeasons: []models.TourLevelSeason{
				{Year: 2026, Matches: 10, HoldPct: 0.80, BreakPct: 0.20, DR: 1.00},
			},
			ChallengerSeasons: []models.TourLevelSeason{
				{Year: 2025, Matches: 40, HoldPct: 0.50, BreakPct: 0.50, DR: 0.50},
			},
		}
		rates, err := AdjustedHoldBreak(stats, opts)
		if err != nil {
			t.Fatalf("AdjustedHoldBreak: %v", err)
		}
		if math.Abs(rates.SeasonHold-0.80) > 1e-9 {
			t.Fatalf("SeasonHold = %v, want tour-only 0.80", rates.SeasonHold)
		}
		if rates.SeasonMatches != 10 {
			t.Fatalf("SeasonMatches = %d, want 10", rates.SeasonMatches)
		}
	})

	t.Run("no tour challenger only", func(t *testing.T) {
		t.Parallel()
		stats := models.PlayerStats{
			ChallengerSeasons: []models.TourLevelSeason{
				{Year: 2026, Matches: 15, HoldPct: 0.75, BreakPct: 0.25, DR: 1.05},
			},
		}
		rates, err := AdjustedHoldBreak(stats, opts)
		if err != nil {
			t.Fatalf("AdjustedHoldBreak: %v", err)
		}
		wantH := 0.75 * chalWeight
		if math.Abs(rates.SeasonHold-wantH) > 1e-9 {
			t.Fatalf("SeasonHold = %v, want scaled %v", rates.SeasonHold, wantH)
		}
		if rates.SeasonMatches != 15 {
			t.Fatalf("SeasonMatches = %d, want 15", rates.SeasonMatches)
		}
	})

	t.Run("neither usable", func(t *testing.T) {
		t.Parallel()
		_, err := AdjustedHoldBreak(models.PlayerStats{
			ChallengerSeasons: []models.TourLevelSeason{
				{Year: 2025, Matches: 40, HoldPct: 0.50, BreakPct: 0.50, DR: 0.50},
			},
		}, opts)
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, ErrNoSeasonData) {
			t.Fatalf("expected ErrNoSeasonData, got %v", err)
		}
	})

	t.Run("challenger gate uses max year not row order", func(t *testing.T) {
		t.Parallel()
		stats := models.PlayerStats{
			TourLevelSeasons: []models.TourLevelSeason{
				{Year: 2026, Matches: 10, HoldPct: 0.80, BreakPct: 0.20, DR: 1.00},
			},
			ChallengerSeasons: []models.TourLevelSeason{
				{Year: 2025, Matches: 50, HoldPct: 0.50, BreakPct: 0.50, DR: 0.50},
				{Year: 2026, Matches: 20, HoldPct: 0.70, BreakPct: 0.30, DR: 0.90},
			},
		}
		rates, err := AdjustedHoldBreak(stats, opts)
		if err != nil {
			t.Fatalf("AdjustedHoldBreak: %v", err)
		}
		scaledChalH := 0.70 * chalWeight
		w := 10.0 / (10.0 + 20.0)
		wantH := w*0.80 + (1-w)*scaledChalH
		if math.Abs(rates.SeasonHold-wantH) > 1e-9 {
			t.Fatalf("SeasonHold = %v, want blended %v", rates.SeasonHold, wantH)
		}
	})

	t.Run("newer challenger year blocks supplement", func(t *testing.T) {
		t.Parallel()
		stats := models.PlayerStats{
			TourLevelSeasons: []models.TourLevelSeason{
				{Year: 2026, Matches: 10, HoldPct: 0.80, BreakPct: 0.20, DR: 1.00},
			},
			ChallengerSeasons: []models.TourLevelSeason{
				{Year: 2026, Matches: 20, HoldPct: 0.50, BreakPct: 0.50, DR: 0.50},
				{Year: 2027, Matches: 5, HoldPct: 0.60, BreakPct: 0.40, DR: 0.80},
			},
		}
		rates, err := AdjustedHoldBreak(stats, opts)
		if err != nil {
			t.Fatalf("AdjustedHoldBreak: %v", err)
		}
		if math.Abs(rates.SeasonHold-0.80) > 1e-9 {
			t.Fatalf("SeasonHold = %v, want tour-only 0.80", rates.SeasonHold)
		}
		if rates.SeasonMatches != 10 {
			t.Fatalf("SeasonMatches = %d, want 10", rates.SeasonMatches)
		}
	})

	t.Run("challenger merge then prior tour blend", func(t *testing.T) {
		t.Parallel()
		stats := models.PlayerStats{
			TourLevelSeasons: []models.TourLevelSeason{
				{Year: 2026, Matches: 5, HoldPct: 0.90, BreakPct: 0.10, DR: 1.20},
				{Year: 2025, Matches: 50, HoldPct: 0.70, BreakPct: 0.30, DR: 1.00},
			},
			ChallengerSeasons: []models.TourLevelSeason{
				{Year: 2026, Matches: 5, HoldPct: 0.70, BreakPct: 0.30, DR: 0.90},
			},
		}
		rates, err := AdjustedHoldBreak(stats, opts)
		if err != nil {
			t.Fatalf("AdjustedHoldBreak: %v", err)
		}
		scaledChalH := 0.70 * chalWeight
		wTourChal := 5.0 / (5.0 + 5.0)
		hCurr := wTourChal*0.90 + (1-wTourChal)*scaledChalH
		wantH := 10.0/60*hCurr + 50.0/60*0.70
		if math.Abs(rates.SeasonHold-wantH) > 1e-9 {
			t.Fatalf("SeasonHold = %v, want %v", rates.SeasonHold, wantH)
		}
		if rates.SeasonMatches != 60 {
			t.Fatalf("SeasonMatches = %d, want 60", rates.SeasonMatches)
		}
	})
}

func TestBuildEffectiveCurrentSeason(t *testing.T) {
	t.Parallel()

	year := 2026
	minM := 20
	weight := 0.9

	t.Run("tour only sufficient", func(t *testing.T) {
		t.Parallel()
		tour := []models.TourLevelSeason{{Year: 2026, Matches: 30, HoldPct: 0.8, BreakPct: 0.2, DR: 1.0}}
		chal := []models.TourLevelSeason{{Year: 2026, Matches: 10, HoldPct: 0.5, BreakPct: 0.5, DR: 0.5}}
		s, m, ok := buildEffectiveCurrentSeason(tour, chal, year, minM, weight)
		if !ok || m != 30 || math.Abs(s.HoldPct-0.8) > 1e-9 {
			t.Fatalf("got %+v ok=%v m=%d", s, ok, m)
		}
	})

	t.Run("career row ignored for gate", func(t *testing.T) {
		t.Parallel()
		chal := []models.TourLevelSeason{
			{Year: 2026, Matches: 15, HoldPct: 0.75, BreakPct: 0.25, DR: 1.0},
			{IsCareer: true, Matches: 100, HoldPct: 0.5, BreakPct: 0.5, DR: 0.5},
		}
		s, m, ok := buildEffectiveCurrentSeason(nil, chal, year, minM, weight)
		wantH := 0.75 * weight
		if !ok || m != 15 || math.Abs(s.HoldPct-wantH) > 1e-9 {
			t.Fatalf("got HoldPct=%v ok=%v m=%d, want hold %v", s.HoldPct, ok, m, wantH)
		}
	})

	t.Run("thin tour plus challenger match blend", func(t *testing.T) {
		t.Parallel()
		tour := []models.TourLevelSeason{{Year: 2026, Matches: 10, HoldPct: 0.80, BreakPct: 0.20, DR: 1.0}}
		chal := []models.TourLevelSeason{{Year: 2026, Matches: 20, HoldPct: 0.70, BreakPct: 0.30, DR: 0.90}}
		s, m, ok := buildEffectiveCurrentSeason(tour, chal, year, minM, weight)
		scaledChalH := 0.70 * weight
		w := 10.0 / 30.0
		wantH := w*0.80 + (1-w)*scaledChalH
		if !ok || m != 30 || math.Abs(s.HoldPct-wantH) > 1e-9 {
			t.Fatalf("got HoldPct=%v ok=%v m=%d, want hold %v", s.HoldPct, ok, m, wantH)
		}
	})

	t.Run("challenger 100 pct scaled by weight 0.3", func(t *testing.T) {
		t.Parallel()
		tour := []models.TourLevelSeason{{Year: 2026, Matches: 10, HoldPct: 0.50, BreakPct: 0.50, DR: 1.0}}
		chal := []models.TourLevelSeason{{Year: 2026, Matches: 10, HoldPct: 1.0, BreakPct: 1.0, DR: 1.0}}
		const w = 0.3
		s, m, ok := buildEffectiveCurrentSeason(tour, chal, year, minM, w)
		scaledChal := 1.0 * w
		blendW := 10.0 / 20.0
		wantH := blendW*0.50 + (1-blendW)*scaledChal
		wantB := blendW*0.50 + (1-blendW)*scaledChal
		if !ok || m != 20 {
			t.Fatalf("got ok=%v m=%d, want ok=true m=20", ok, m)
		}
		if math.Abs(s.HoldPct-wantH) > 1e-9 || math.Abs(s.BreakPct-wantB) > 1e-9 {
			t.Fatalf("got hold=%v break=%v, want hold=%v break=%v", s.HoldPct, s.BreakPct, wantH, wantB)
		}
		if math.Abs(s.HoldPct-1.0) < 1e-9 {
			t.Fatal("hold must use scaled challenger 0.3, not raw 1.0")
		}
	})
}

func TestFormOptions_withDefaultsFromEnv(t *testing.T) {
	t.Setenv(formMinSeasonMatchesEnv, "")
	t.Setenv(formRecentMatchLimitEnv, "")
	t.Setenv(formHalfLifeMatchesEnv, "")
	t.Setenv(formWeightMaxEnv, "")
	t.Setenv(formRatioMinEnv, "")
	t.Setenv(formRatioMaxEnv, "")
	t.Setenv(formChallengerWeightEnv, "")

	got := (FormOptions{}).withDefaults()
	if got.MinSeasonMatches != defaultMinSeasonMatches ||
		got.RecentMatchLimit != defaultRecentMatchLimit ||
		got.HalfLifeMatches != defaultHalfLifeMatches ||
		got.FormWeightMax != defaultFormWeightMax ||
		got.FormRatioMin != defaultFormRatioMin ||
		got.FormRatioMax != defaultFormRatioMax ||
		got.ChallengerWeight != defaultChallengerWeight {
		t.Fatalf("empty env defaults: %+v", got)
	}

	t.Setenv(formMinSeasonMatchesEnv, "25")
	t.Setenv(formRecentMatchLimitEnv, "10")
	t.Setenv(formHalfLifeMatchesEnv, "4")
	t.Setenv(formWeightMaxEnv, "0.25")
	t.Setenv(formRatioMinEnv, "0.90")
	t.Setenv(formRatioMaxEnv, "1.10")
	t.Setenv(formChallengerWeightEnv, "0.85")

	got = (FormOptions{}).withDefaults()
	if got.MinSeasonMatches != 25 || got.RecentMatchLimit != 10 ||
		got.HalfLifeMatches != 4 || got.FormWeightMax != 0.25 ||
		got.FormRatioMin != 0.90 || got.FormRatioMax != 1.10 ||
		got.ChallengerWeight != 0.85 {
		t.Fatalf("env overrides: %+v", got)
	}

	t.Setenv(formMinSeasonMatchesEnv, "bad")
	got = (FormOptions{}).withDefaults()
	if got.MinSeasonMatches != defaultMinSeasonMatches {
		t.Fatalf("invalid env = %d, want %d", got.MinSeasonMatches, defaultMinSeasonMatches)
	}

	explicit := FormOptions{MinSeasonMatches: 7}.withDefaults()
	if explicit.MinSeasonMatches != 7 {
		t.Fatalf("explicit option kept: %d", explicit.MinSeasonMatches)
	}

	zeroWeight := FormOptions{
		HalfLifeMatches: 5,
		FormWeightMax:   0,
		FormRatioMin:    0.92,
		FormRatioMax:    1.08,
	}.withDefaults()
	if zeroWeight.FormWeightMax != 0 {
		t.Fatalf("explicit FormWeightMax=0 kept: %v", zeroWeight.FormWeightMax)
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
