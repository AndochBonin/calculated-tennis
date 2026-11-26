package tennisabstract

import (
	"testing"
	"time"

	"github.com/AndochBonin/calculated-tennis/tennis/models"
)

func TestRecentResultsBefore_cutoffAndLimit(t *testing.T) {
	t.Parallel()

	asOf := time.Date(2026, 5, 10, 15, 30, 0, 0, time.UTC)
	results := []models.RecentResult{
		{Tournament: "future", Date: time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC)},
		{Tournament: "same-day", Date: time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)},
		{Tournament: "d1", Date: time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC)},
		{Tournament: "d2", Date: time.Date(2026, 5, 8, 0, 0, 0, 0, time.UTC)},
		{Tournament: "d3", Date: time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC)},
		{Tournament: "d4", Date: time.Date(2026, 5, 6, 0, 0, 0, 0, time.UTC)},
	}

	got := RecentResultsBefore(results, asOf, 3)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	want := []string{"d1", "d2", "d3"}
	for i, name := range want {
		if got[i].Tournament != name {
			t.Fatalf("got[%d].Tournament = %q, want %q", i, got[i].Tournament, name)
		}
	}
}

func TestRecentResultsBefore_strictBeforeSameDay(t *testing.T) {
	t.Parallel()

	asOf := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
	results := []models.RecentResult{
		{Tournament: "on-day", Date: asOf},
		{Tournament: "before", Date: time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC)},
	}

	got := RecentResultsBefore(results, asOf, 15)
	if len(got) != 1 || got[0].Tournament != "before" {
		t.Fatalf("got = %+v, want only match before asOf day", got)
	}
}

func TestRecentResultsBefore_nonUTCTimesNormalized(t *testing.T) {
	t.Parallel()

	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}
	// 2026-05-10 08:00 JST is 2026-05-09 23:00 UTC → May 9 UTC calendar day.
	asOf := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
	results := []models.RecentResult{
		{Tournament: "tokyo-may9-utc", Date: time.Date(2026, 5, 10, 8, 0, 0, 0, loc)},
		{Tournament: "utc-may10", Date: time.Date(2026, 5, 10, 1, 0, 0, 0, time.UTC)},
	}

	got := RecentResultsBefore(results, asOf, 15)
	if len(got) != 1 || got[0].Tournament != "tokyo-may9-utc" {
		t.Fatalf("got = %+v, want tokyo-may9-utc only (UTC date-only cutoff)", got)
	}
}

func TestRecentResultsBefore_stableTieBreak(t *testing.T) {
	t.Parallel()

	same := time.Date(2026, 5, 5, 0, 0, 0, 0, time.UTC)
	results := []models.RecentResult{
		{Tournament: "first", Date: same},
		{Tournament: "second", Date: same},
		{Tournament: "older", Date: time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)},
	}
	asOf := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)

	got := RecentResultsBefore(results, asOf, 15)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	if got[0].Tournament != "first" || got[1].Tournament != "second" {
		t.Fatalf("stable order on tie: got %q, %q; want first, second", got[0].Tournament, got[1].Tournament)
	}
	if got[2].Tournament != "older" {
		t.Fatalf("got[2] = %q, want older", got[2].Tournament)
	}
}

func TestRecentResultsBefore_zeroLimit(t *testing.T) {
	t.Parallel()

	results := []models.RecentResult{
		{Date: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)},
	}
	if got := RecentResultsBefore(results, time.Now(), 0); got != nil {
		t.Fatalf("got = %#v, want nil for limit <= 0", got)
	}
}

func TestRecentResultsBefore_limit15(t *testing.T) {
	t.Parallel()

	results := make([]models.RecentResult, 20)
	for i := range results {
		results[i] = models.RecentResult{
			Tournament: "m",
			Date:       time.Date(2026, 5, 20-i, 0, 0, 0, 0, time.UTC),
		}
	}

	asOf := time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC)
	got := RecentResultsBefore(results, asOf, 15)
	if len(got) != 15 {
		t.Fatalf("len = %d, want 15", len(got))
	}
	if got[0].Date.Day() != 20 || got[14].Date.Day() != 6 {
		t.Fatalf("range: first day=%d last day=%d, want 20..6", got[0].Date.Day(), got[14].Date.Day())
	}
}

func TestRecentResultsBefore_medvedevFixtures(t *testing.T) {
	t.Parallel()

	matchmx, err := extractJSArray(loadTestdata(t, "player_classic_medvedev_snip.html"), "matchmx")
	if err != nil {
		t.Fatalf("extractJSArray: %v", err)
	}
	morematchmx, err := extractJSArray(loadTestdata(t, "medvedev_career_snip.js"), "morematchmx")
	if err != nil {
		t.Fatalf("extractJSArray morematchmx: %v", err)
	}
	career, err := ParseMatchMXArrays(matchmx, morematchmx)
	if err != nil {
		t.Fatalf("ParseMatchMXArrays: %v", err)
	}
	if len(career) != 7 {
		t.Fatalf("career len = %d, want 7 (5 matchmx + 2 morematchmx)", len(career))
	}

	asOf := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	got := RecentResultsBefore(career, asOf, 15)
	if len(got) != 5 {
		t.Fatalf("len = %d, want 5 (exclude all 2026-05-06 Rome rows)", len(got))
	}
	if got[0].Tournament != "Madrid Masters" || got[0].Round != "R32" {
		t.Fatalf("newest = %q %q, want Madrid Masters R32", got[0].Tournament, got[0].Round)
	}
	if got[4].Tournament != "Russia F1" || got[4].Round != "R32" {
		t.Fatalf("oldest = %q %q, want Russia F1 R32", got[4].Tournament, got[4].Round)
	}

	later := RecentResultsBefore(career, time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC), 15)
	if len(later) != 7 {
		t.Fatalf("len = %d, want full fixture (7 rows)", len(later))
	}
	if later[0].Round != "SF" || later[0].Tournament != "Rome Masters" {
		t.Fatalf("newest = %+v, want Rome SF", later[0])
	}
}
