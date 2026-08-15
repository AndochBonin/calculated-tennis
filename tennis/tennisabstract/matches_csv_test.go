package tennisabstract

import (
	"strings"
	"testing"

	"github.com/AndochBonin/calculated-tennis/tennis/tennis"
)

func TestUniquePlayerNamesFromMatchesCSV(t *testing.T) {
	t.Parallel()

	csv := strings.Join([]string{
		"tourney_id,winner_name,loser_name,score",
		"1,Alice Bob,Carol Dan,6-4 6-3",
		"2,Alice Bob,Eve Fay,6-2 6-1",
		"3,Carol Dan,Eve Fay,7-5 6-4",
	}, "\n")

	names, err := UniquePlayerNamesFromMatchesCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("UniquePlayerNamesFromMatchesCSV: %v", err)
	}
	want := []string{"Alice Bob", "Carol Dan", "Eve Fay"}
	if len(names) != len(want) {
		t.Fatalf("got %d names %v, want %v", len(names), names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("names[%d] = %q, want %q (full %v)", i, names[i], want[i], names)
		}
	}
}

func TestUniquePlayerNamesFromMatchesCSV_missingColumns(t *testing.T) {
	t.Parallel()

	_, err := UniquePlayerNamesFromMatchesCSV(strings.NewReader("a,b\n1,2\n"))
	if err == nil {
		t.Fatal("expected error for missing name columns")
	}
}

func TestScoreIsIncomplete(t *testing.T) {
	t.Parallel()

	cases := []struct {
		score string
		want  bool
	}{
		{"6-4 6-3", false},
		{"6-7(4) 6-5 RET", true},
		{"W/O", true},
		{"6-0 DEF", true},
		{"7-6(3) 7-5", false},
	}
	for _, tc := range cases {
		if got := scoreIsIncomplete(tc.score); got != tc.want {
			t.Errorf("scoreIsIncomplete(%q) = %v, want %v", tc.score, got, tc.want)
		}
	}
}

func TestLoadCalibrationMatchesCSV_synthetic(t *testing.T) {
	t.Parallel()

	csv := strings.Join([]string{
		"winner_name,loser_name,surface,score,best_of,tourney_date",
		"Taylor Fritz,Tomas Machac,Hard,6-4 6-3,3,20250110",
		"Daniil Medvedev,Novak Djokovic,Clay,6-7(4) 6-5 RET,3,20250111",
		"Carlos Alcaraz,Jannik Sinner,Grass,W/O,3,20250112",
		"Alexander Zverev,Rafael Nadal,Carpet,6-2 6-2,3,20250113",
		"Roger Federer,Andy Murray,Hard,6-1 6-2,4,20250114",
		"Felix Auger Aliassime,Hubert Hurkacz,Hard,7-5 6-4,5,20250115",
	}, "\n")

	got, err := LoadCalibrationMatchesCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("LoadCalibrationMatchesCSV: %v", err)
	}
	if got.SkippedIncomplete != 2 {
		t.Fatalf("SkippedIncomplete = %d, want 2", got.SkippedIncomplete)
	}
	if got.SkippedInvalid != 2 {
		t.Fatalf("SkippedInvalid = %d, want 2", got.SkippedInvalid)
	}

	hard := got.BySurface[SurfaceHard]
	if len(hard) != 2 {
		t.Fatalf("hard matches = %d, want 2", len(hard))
	}
	if hard[0].PlayerA != "Taylor Fritz" || hard[0].PlayerB != "Tomas Machac" {
		t.Fatalf("first hard match players: %q vs %q", hard[0].PlayerA, hard[0].PlayerB)
	}
	if hard[0].PlayerASlug != "TaylorFritz" || hard[0].PlayerBSlug != "TomasMachac" {
		t.Fatalf("slugs: %q %q", hard[0].PlayerASlug, hard[0].PlayerBSlug)
	}
	if hard[0].Format != tennis.DefaultFormat() {
		t.Fatal("best-of-3 should use DefaultFormat")
	}
	if hard[0].TourneyDate != 20250110 {
		t.Fatalf("TourneyDate = %d, want 20250110", hard[0].TourneyDate)
	}
	if hard[1].BestOf != 5 || hard[1].Format != tennis.GrandSlamMenFormat() {
		t.Fatalf("bo5 match: best_of=%d format=%+v", hard[1].BestOf, hard[1].Format)
	}
	if len(got.BySurface[SurfaceClay]) != 0 || len(got.BySurface[SurfaceGrass]) != 0 {
		t.Fatalf("unexpected clay/grass rows: clay=%d grass=%d",
			len(got.BySurface[SurfaceClay]), len(got.BySurface[SurfaceGrass]))
	}
}

func TestLoadCalibrationMatchesCSV_2025Fixture(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping full CSV in -short")
	}

	got, err := LoadCalibrationMatchesCSVFile("testdata/atp_matches_2025.csv")
	if err != nil {
		t.Fatalf("LoadCalibrationMatchesCSVFile: %v", err)
	}
	if got.SkippedIncomplete != 131 {
		t.Fatalf("SkippedIncomplete = %d, want 131", got.SkippedIncomplete)
	}
	if got.SkippedInvalid != 0 {
		t.Fatalf("SkippedInvalid = %d, want 0", got.SkippedInvalid)
	}
	wantCounts := map[MatchSurface]int{
		SurfaceHard:  1739,
		SurfaceClay:  787,
		SurfaceGrass: 287,
	}
	total := 0
	for surf, want := range wantCounts {
		n := len(got.BySurface[surf])
		total += n
		if n != want {
			t.Errorf("%s: %d matches, want %d", surf, n, want)
		}
	}
	if total != 2813 {
		t.Fatalf("total eligible = %d, want 2813", total)
	}
}

func TestLoadCalibrationMatchesCSV_missingColumns(t *testing.T) {
	t.Parallel()

	_, err := LoadCalibrationMatchesCSV(strings.NewReader("a,b\n1,2\n"))
	if err == nil {
		t.Fatal("expected error for missing columns")
	}
}
