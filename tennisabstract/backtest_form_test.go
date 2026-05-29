package tennisabstract

import (
	"testing"
	"time"

	"github.com/AndochBonin/polymarket/models"
)

func TestTourneyDateAsTime(t *testing.T) {
	t.Parallel()

	got, ok := TourneyDateAsTime(20250115)
	if !ok {
		t.Fatal("expected ok")
	}
	want := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if _, ok := TourneyDateAsTime(0); ok {
		t.Fatal("expected invalid for zero")
	}
}

func TestAdjustedHoldBreakAsOf_filtersRecent(t *testing.T) {
	t.Parallel()

	asOf := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	dr := 1.2
	career := []models.RecentResult{
		{Date: time.Date(2025, 5, 20, 0, 0, 0, 0, time.UTC), DominanceRatio: &dr},
		{Date: time.Date(2025, 6, 5, 0, 0, 0, 0, time.UTC), DominanceRatio: &dr}, // on/after asOf → excluded
	}
	stats := PlayerStatsForBacktestForm("TestPlayer", asOf, 0.80, 0.25)
	stats.TourLevelSeasons[0].DR = 1.0

	rates, err := AdjustedHoldBreakAsOf(stats, career, FormOptions{AsOf: asOf, RecentMatchLimit: 15})
	if err != nil {
		t.Fatalf("AdjustedHoldBreakAsOf: %v", err)
	}
	if rates.DRForm == 0 || rates.FormWeight == 0 {
		t.Fatalf("expected form from pre-asOf row only: DRForm=%v FormWeight=%v", rates.DRForm, rates.FormWeight)
	}
}
