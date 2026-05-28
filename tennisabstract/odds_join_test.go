package tennisabstract

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJoinMatchesWithAvgOddsCSV_synthetic(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	matches := filepath.Join(dir, "matches.csv")
	odds := filepath.Join(dir, "odds.csv")
	out := filepath.Join(dir, "out.csv")

	mCSV := strings.Join([]string{
		"tourney_id,tourney_date,winner_name,loser_name,surface,score,best_of",
		"1,20241230,Alex Michelsen,Christopher Oconnell,Hard,6-4 6-3,3",
		"2,20250101,Nobody A,Nobody B,Hard,6-0 6-0,3",
	}, "\n")
	oCSV := strings.Join([]string{
		"Date,Surface,Winner,Loser,Comment,AvgW,AvgL",
		"12/30/2024,Hard,Michelsen A.,O Connell C.,Completed,1.43,2.74",
	}, "\n")
	if err := os.WriteFile(matches, []byte(mCSV), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(odds, []byte(oCSV), 0o644); err != nil {
		t.Fatal(err)
	}

	stats, err := JoinMatchesWithAvgOddsCSV(matches, odds, out)
	if err != nil {
		t.Fatalf("JoinMatchesWithAvgOddsCSV: %v", err)
	}
	if stats.RowsWritten != 2 || stats.RowsMatched != 1 || stats.RowsUnmatched != 1 {
		t.Fatalf("stats: %+v", stats)
	}

	f, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(rows))
	}
	if rows[1][len(rows[1])-2] != "1.43" || rows[1][len(rows[1])-1] != "2.74" {
		t.Fatalf("row1 odds: %v", rows[1][len(rows[1])-2:])
	}
	if rows[2][len(rows[2])-2] != "" || rows[2][len(rows[2])-1] != "" {
		t.Fatalf("row2 should have empty odds: %v", rows[2][len(rows[2])-2:])
	}
}

func TestJoinMatchesWithAvgOddsCSV_2025Fixture(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping full join in -short")
	}
	dir := t.TempDir()
	out := filepath.Join(dir, "joined.csv")
	stats, err := JoinMatchesWithAvgOddsCSV(
		"testdata/atp_matches_2025.csv",
		"testdata/odds_2025.csv",
		out,
	)
	if err != nil {
		t.Fatalf("join: %v", err)
	}
	if stats.RowsWritten != 2944 {
		t.Fatalf("rows written = %d, want 2944", stats.RowsWritten)
	}
	if stats.RowsMatched < 2000 {
		t.Fatalf("matched = %d, expected at least ~2000", stats.RowsMatched)
	}
	t.Logf("matched %d / %d (%.1f%%)", stats.RowsMatched, stats.RowsWritten,
		100*float64(stats.RowsMatched)/float64(stats.RowsWritten))
}
