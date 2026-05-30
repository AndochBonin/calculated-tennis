package tennisabstract

import (
	"math"
	"os"
	"testing"
	"time"

	"github.com/AndochBonin/E3/tennis/models"
)

// Rome SF row from player-classic.cgi (live values; DR ~0.73 vs jsfrags HTML).
var medvedevRomeSFMatchMX = []string{
	"20260506", "Rome Masters", "Clay", "M", "L", "9", "7", "", "SF", "6-2 5-7 6-4",
	"3", "Jannik Sinner", "1", "1", "", "R", "20010816", "191", "ITA", "0",
	"157", "4", "7", "104", "66", "44", "15", "15", "6", "10", "7", "2", "92",
	"55", "41", "22", "15", "5", "7", "2", "", "", "", "2026-0416-368", "", "", "", "206173",
}

var medvedevWalkoverMatchMX = []string{
	"20260501", "Madrid Masters", "Clay", "M", "W", "9", "", "", "R32", "W/O",
	"3", "Opponent", "50", "", "", "R", "", "", "", "0",
	"", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "",
	"2026-520-100", "", "", "",
}

func TestParseMatchMXArrays_RomeSF(t *testing.T) {
	t.Parallel()

	got, err := ParseMatchMXArrays([][]string{medvedevRomeSFMatchMX}, nil)
	if err != nil {
		t.Fatalf("ParseMatchMXArrays: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	first := got[0]
	wantDate := time.Date(2026, time.May, 6, 0, 0, 0, 0, time.UTC)
	if !first.Date.Equal(wantDate) {
		t.Fatalf("date = %v, want %v", first.Date, wantDate)
	}
	if first.Tournament != "Rome Masters" || first.Surface != "Clay" || first.Round != "SF" {
		t.Fatalf("metadata: %+v", first)
	}
	if first.Rank != 9 || first.OpponentRank != 1 {
		t.Fatalf("Rk/vRk = %d/%d, want 9/1", first.Rank, first.OpponentRank)
	}
	if first.Score != "6-2 5-7 6-4" {
		t.Fatalf("Score = %q", first.Score)
	}
	if first.DominanceRatio == nil || math.Abs(*first.DominanceRatio-0.73) > 0.01 {
		t.Fatalf("DR = %v, want ~0.73", first.DominanceRatio)
	}
	if math.Abs(first.AcePct-0.038461538461538464) > 1e-9 {
		t.Fatalf("AcePct = %v", first.AcePct)
	}
	if math.Abs(first.DFPct-0.0673076923076923) > 1e-9 {
		t.Fatalf("DFPct = %v", first.DFPct)
	}
	if math.Abs(first.FirstServeIn-0.6346153846153846) > 1e-9 {
		t.Fatalf("FirstServeIn = %v", first.FirstServeIn)
	}
	if math.Abs(first.FirstServeWon-0.6666666666666666) > 1e-9 {
		t.Fatalf("FirstServeWon = %v", first.FirstServeWon)
	}
	if math.Abs(first.SecondServeWon-0.39473684210526316) > 1e-9 {
		t.Fatalf("SecondServeWon = %v", first.SecondServeWon)
	}
	if first.BPSaved != "6/10" {
		t.Fatalf("BPSaved = %q, want 6/10", first.BPSaved)
	}
	if first.Duration != "2:37" {
		t.Fatalf("Duration = %q, want 2:37", first.Duration)
	}
}

func TestParseMatchMXArrays_Walkover(t *testing.T) {
	t.Parallel()

	got, err := ParseMatchMXArrays([][]string{medvedevWalkoverMatchMX}, nil)
	if err != nil {
		t.Fatalf("ParseMatchMXArrays: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].DominanceRatio != nil {
		t.Fatalf("DR = %v, want nil for W/O", got[0].DominanceRatio)
	}
	if got[0].BPSaved != "" || got[0].Duration != "" {
		t.Fatalf("stats should be empty: BPSaved=%q Duration=%q", got[0].BPSaved, got[0].Duration)
	}
}

func TestParseMatchMXArrays_mergeAndDedupe(t *testing.T) {
	t.Parallel()

	recent := [][]string{{
		"20260506", "Rome Masters", "Clay", "M", "L", "9", "", "", "SF", "6-2 5-7 6-4",
		"3", "Jannik Sinner", "1", "", "", "R", "", "", "ITA", "0",
		"157", "4", "7", "104", "66", "44", "15", "15", "6", "10", "7", "2", "92",
		"55", "41", "22", "15", "5", "7", "2", "", "", "", "dup-id", "", "", "",
	}}
	older := [][]string{{
		"20110314", "Russia F1", "Hard", "S", "L", "", "", "Q", "R16", "6-4 6-2",
		"3", "Andis Juska", "373", "", "", "R", "", "", "", "0",
		"", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "",
		"dup-id", "", "", "",
	}, {
		"20110314", "Russia F1", "Hard", "S", "W", "", "", "Q", "R32", "6-3 6-3",
		"3", "Dmitri Sitak", "863", "", "", "R", "", "", "", "0",
		"", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "",
		"unique-old", "", "", "",
	}}

	got, err := ParseMatchMXArrays(recent, older)
	if err != nil {
		t.Fatalf("ParseMatchMXArrays: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3 (recent + unique old, dup skipped)", len(got))
	}
	if got[0].Tournament != "Rome Masters" {
		t.Fatalf("first = %q, want Rome Masters", got[0].Tournament)
	}
	if got[1].Tournament != "Russia F1" || got[1].Round != "R16" {
		t.Fatalf("second = %+v", got[1])
	}
	if got[2].Round != "R32" {
		t.Fatalf("third round = %q, want R32", got[2].Round)
	}
}

func TestParseClassicDate(t *testing.T) {
	t.Parallel()

	d, err := parseClassicDate("20260506")
	if err != nil {
		t.Fatal(err)
	}
	if d.Year() != 2026 || d.Month() != time.May || d.Day() != 6 {
		t.Fatalf("date = %v", d)
	}
	if d.Location() != time.UTC {
		t.Fatalf("location = %v, want UTC", d.Location())
	}
}

func TestParseMatchMXArrays_fixture_crossCheckHTML(t *testing.T) {
	t.Parallel()

	classicBody := loadTestdata(t, "player_classic_medvedev_snip.html")
	matchmx, err := extractJSArray(classicBody, "matchmx")
	if err != nil {
		t.Fatalf("extractJSArray matchmx: %v", err)
	}
	classic, err := ParseMatchMXArrays(matchmx, nil)
	if err != nil {
		t.Fatalf("ParseMatchMXArrays: %v", err)
	}

	f, err := os.Open("testdata/player_medvedev.html")
	if err != nil {
		t.Fatalf("open player_medvedev.html: %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })

	stats, err := ParsePlayerHTML(f, "DaniilMedvedev")
	if err != nil {
		t.Fatalf("ParsePlayerHTML: %v", err)
	}

	htmlSF := findRecentResult(stats.RecentResults, "Rome Masters", "SF")
	classicSF := findRecentResult(classic, "Rome Masters", "SF")
	if htmlSF == nil {
		t.Fatal("Rome SF not found in jsfrags HTML fixture")
	}
	if classicSF == nil {
		t.Fatal("Rome SF not found in classic matchmx fixture")
	}

	assertRecentResultNear(t, *htmlSF, *classicSF)

	htmlWO := findRecentResult(stats.RecentResults, "Rome Masters", "R64")
	classicWO := findRecentResult(classic, "Rome Masters", "R64")
	if htmlWO == nil || classicWO == nil {
		t.Fatal("Rome R64 W/O row missing in HTML or classic fixture")
	}
	if classicWO.DominanceRatio != nil {
		t.Fatalf("classic W/O DR = %v, want nil", *classicWO.DominanceRatio)
	}
	if htmlWO.DominanceRatio != nil {
		t.Fatalf("HTML W/O DR = %v, want nil", *htmlWO.DominanceRatio)
	}
}

func findRecentResult(results []models.RecentResult, tournament, round string) *models.RecentResult {
	for i := range results {
		if results[i].Tournament == tournament && results[i].Round == round {
			return &results[i]
		}
	}
	return nil
}

func assertRecentResultNear(t *testing.T, html, classic models.RecentResult) {
	t.Helper()

	if !html.Date.Equal(classic.Date) {
		t.Fatalf("date: html=%v classic=%v", html.Date, classic.Date)
	}
	if html.Tournament != classic.Tournament || html.Surface != classic.Surface || html.Round != classic.Round {
		t.Fatalf("metadata: html=%+v classic=%+v", html, classic)
	}
	if html.Rank != classic.Rank || html.OpponentRank != classic.OpponentRank || html.Score != classic.Score {
		t.Fatalf("rank/score: html=%d/%d/%q classic=%d/%d/%q",
			html.Rank, html.OpponentRank, html.Score, classic.Rank, classic.OpponentRank, classic.Score)
	}
	if html.DominanceRatio == nil || classic.DominanceRatio == nil {
		t.Fatalf("DR nil: html=%v classic=%v", html.DominanceRatio, classic.DominanceRatio)
	}
	if math.Abs(*html.DominanceRatio-*classic.DominanceRatio) > 0.01 {
		t.Fatalf("DR: html=%.2f classic=%.2f", *html.DominanceRatio, *classic.DominanceRatio)
	}
	if math.Abs(html.AcePct-classic.AcePct) > 0.001 {
		t.Fatalf("AcePct: html=%v classic=%v", html.AcePct, classic.AcePct)
	}
	if math.Abs(html.FirstServeIn-classic.FirstServeIn) > 0.001 {
		t.Fatalf("FirstServeIn: html=%v classic=%v", html.FirstServeIn, classic.FirstServeIn)
	}
	if html.BPSaved != classic.BPSaved {
		t.Fatalf("BPSaved: html=%q classic=%q", html.BPSaved, classic.BPSaved)
	}
	if html.Duration != classic.Duration {
		t.Fatalf("Duration: html=%q classic=%q", html.Duration, classic.Duration)
	}
}
