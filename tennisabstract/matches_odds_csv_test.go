package tennisabstract

import (
	"strings"
	"testing"

	"github.com/AndochBonin/polymarket/tennis"
)

func TestLoadMatchesWithOddsCSV_synthetic(t *testing.T) {
	t.Parallel()

	csv := strings.Join([]string{
		"tourney_date,match_num,winner_name,loser_name,surface,score,best_of,AvgW,AvgL",
		"20250110,100,Taylor Fritz,Tomas Machac,Hard,6-4 6-3,3,1.5,2.5",
		"20250111,101,Daniil Medvedev,Novak Djokovic,Clay,6-7(4) 6-5 RET,3,1.4,2.6",
		"20250112,102,Carlos Alcaraz,Jannik Sinner,Grass,6-2 6-2,3,,2.0",
		"20250113,103,Roger Federer,Andy Murray,Hard,6-1 6-2,5,1.8,2.2",
	}, "\n")

	rows, err := LoadMatchesWithOddsCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("LoadMatchesWithOddsCSV: %v", err)
	}
	if len(rows) != 4 {
		t.Fatalf("got %d rows, want 4", len(rows))
	}

	m := rows[0]
	if m.TourneyDate != 20250110 || m.MatchNum != 100 {
		t.Fatalf("date/num: %d %d", m.TourneyDate, m.MatchNum)
	}
	if m.PlayerA != "Taylor Fritz" || m.PlayerB != "Tomas Machac" {
		t.Fatalf("players: %q vs %q", m.PlayerA, m.PlayerB)
	}
	if m.AvgW != 1.5 || m.AvgL != 2.5 {
		t.Fatalf("odds: %v %v", m.AvgW, m.AvgL)
	}
	if m.Format != tennis.DefaultFormat() {
		t.Fatal("bo3 should use DefaultFormat")
	}

	if rows[2].AvgW != 0 || rows[2].AvgL != 2.0 {
		t.Fatalf("partial odds row: AvgW=%v AvgL=%v", rows[2].AvgW, rows[2].AvgL)
	}
	if rows[3].BestOf != 5 || rows[3].Format != tennis.GrandSlamMenFormat() {
		t.Fatalf("bo5: best_of=%d format=%+v", rows[3].BestOf, rows[3].Format)
	}
}

func TestFilterBacktestMatches_synthetic(t *testing.T) {
	t.Parallel()

	rates := PlayerRatesMap{
		"TaylorFritz": {Hold2024: 0.8, Break2024: 0.2},
		"TomasMachac": {Hold2024: 0.75, Break2024: 0.25},
	}
	rows := []MatchWithOdds{
		{
			Surface: SurfaceHard, PlayerA: "Taylor Fritz", PlayerB: "Tomas Machac",
			PlayerASlug: "TaylorFritz", PlayerBSlug: "TomasMachac",
			Score: "6-4 6-3", AvgW: 1.5, AvgL: 2.5,
		},
		{
			Surface: SurfaceHard, PlayerA: "Taylor Fritz", PlayerB: "Tomas Machac",
			PlayerASlug: "TaylorFritz", PlayerBSlug: "TomasMachac",
			Score: "6-7(4) 6-5 RET", AvgW: 1.5, AvgL: 2.5,
		},
		{
			Surface: SurfaceHard, PlayerA: "Nobody", PlayerB: "Tomas Machac",
			PlayerASlug: "Nobody", PlayerBSlug: "TomasMachac",
			Score: "6-4 6-3", AvgW: 1.5, AvgL: 2.5,
		},
		{
			Surface: SurfaceHard, PlayerA: "Taylor Fritz", PlayerB: "Tomas Machac",
			PlayerASlug: "TaylorFritz", PlayerBSlug: "TomasMachac",
			Score: "6-4 6-3", AvgW: 0, AvgL: 2.5,
		},
	}

	got := FilterBacktestMatches(rows, rates)
	if len(got) != 1 {
		t.Fatalf("eligible = %d, want 1", len(got))
	}
	if got[0].PlayerA != "Taylor Fritz" {
		t.Fatalf("kept %q", got[0].PlayerA)
	}
}

func TestLoadMatchesWithOddsCSV_2025Fixture(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping full CSV in -short")
	}

	rows, err := LoadMatchesWithOddsCSVFile("testdata/atp_matches_2025_odds.csv")
	if err != nil {
		t.Fatalf("LoadMatchesWithOddsCSVFile: %v", err)
	}
	if len(rows) != 2944 {
		t.Fatalf("loaded %d rows, want 2944", len(rows))
	}

	withOdds := 0
	for _, m := range rows {
		if m.AvgW > 0 && m.AvgL > 0 {
			withOdds++
		}
	}
	if withOdds != 2149 {
		t.Fatalf("rows with both odds = %d, want 2149", withOdds)
	}
}

func TestFilterBacktestMatches_2025Fixture(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping full CSV in -short")
	}

	rows, err := LoadMatchesWithOddsCSVFile("testdata/atp_matches_2025_odds.csv")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	rates, err := ReadPlayerRatesFile("testdata/player_rates_2024.json")
	if err != nil {
		t.Fatalf("rates: %v", err)
	}

	eligible := FilterBacktestMatches(rows, rates)
	if len(eligible) != 1988 {
		t.Fatalf("eligible = %d, want 1988", len(eligible))
	}
	for _, m := range eligible {
		if scoreIsIncomplete(m.Score) {
			t.Fatalf("incomplete score in filter output: %q", m.Score)
		}
		if m.AvgW <= 0 || m.AvgL <= 0 {
			t.Fatalf("missing odds in filter output: %v %v", m.AvgW, m.AvgL)
		}
		if _, ok := rates[m.PlayerASlug]; !ok {
			t.Fatalf("missing rates for A %q", m.PlayerASlug)
		}
		if _, ok := rates[m.PlayerBSlug]; !ok {
			t.Fatalf("missing rates for B %q", m.PlayerBSlug)
		}
	}
}

func TestLoadMatchesWithOddsCSV_missingColumns(t *testing.T) {
	t.Parallel()

	_, err := LoadMatchesWithOddsCSV(strings.NewReader("winner_name,loser_name,surface,score,best_of\na,b,Hard,6-4,3\n"))
	if err == nil {
		t.Fatal("expected error for missing columns")
	}
}
