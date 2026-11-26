package main

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"

	"github.com/AndochBonin/calculated-tennis/tennis/tennisabstract"
)

func TestBetLog_skippedRow(t *testing.T) {
	t.Parallel()

	log := newStrategyBetLog(1000)
	m := tennisabstract.MatchWithOdds{
		PlayerA: "Alice", PlayerB: "Bob", TourneyDate: 20250115,
	}
	log.recordSkip(m, skipReasonMINPICK, 0, 0)
	row := log.rows[0]
	if row[0] != "2025-01-15" || row[3] != "" || row[11] != skipReasonMINPICK {
		t.Fatalf("skipped row: %v", row)
	}
	if row[12] != "" || row[13] != "" || row[14] != "" || row[15] != "" {
		t.Fatalf("singles skip should have empty parlay columns: %v", row[12:])
	}
}

func TestBetLog_negEVSkip(t *testing.T) {
	t.Parallel()

	log := newStrategyBetLog(1000)
	m := tennisabstract.MatchWithOdds{PlayerA: "A", PlayerB: "B", TourneyDate: 20250101}
	log.recordSkip(m, skipReasonNEGEV, 0.5, 1.9)
	row := log.rows[0]
	if row[9] != "0.5000" || row[10] != "-0.05" || row[11] != skipReasonNEGEV {
		t.Fatalf("NEG_EV row: %v", row)
	}
}

func TestBetLog_recordBet(t *testing.T) {
	t.Parallel()

	log := newStrategyBetLog(1000)
	m := tennisabstract.MatchWithOdds{
		PlayerA: "Alice", PlayerB: "Bob", TourneyDate: 20250115, Surface: tennisabstract.SurfaceHard,
	}
	stats := &BacktestStats{}
	winProb := 0.6
	applyBetWithLog(log, m, stats, tennisabstract.BetSideA, 2.0, 10, winProb, tennisabstract.BetSideA)

	row := log.rows[0]
	if row[3] != "Alice" || row[4] != "Alice" || row[5] != "2.00" || row[6] != "10.00" {
		t.Fatalf("pick/winner/odds/stake: %v", row)
	}
	if row[7] != "1000.00" || row[8] != "1010.00" {
		t.Fatalf("balance before/after: %s / %s", row[7], row[8])
	}
	if row[9] != "0.6000" || row[10] != "0.20" || row[11] != "" {
		t.Fatalf("win_prob/ev/skip_reason: %s / %s / %s", row[9], row[10], row[11])
	}
	if row[12] != "" || row[13] != "" || row[14] != "" || row[15] != "" {
		t.Fatalf("singles bet should have empty parlay columns: %v", row[12:])
	}
}

func TestBetLog_recordParlayBet(t *testing.T) {
	t.Parallel()

	log := newStrategyBetLog(1000)
	legs := []parlayLeg{
		{
			match: tennisabstract.MatchWithOdds{
				PlayerA: "Alice", PlayerB: "Bob", TourneyDate: 20250115,
				Surface: tennisabstract.SurfaceClay,
			},
			pick:    tennisabstract.BetSideA,
			odds:    2.0,
			winProb: 0.6,
			winner:  tennisabstract.BetSideA,
		},
		{
			match: tennisabstract.MatchWithOdds{
				PlayerA: "Carol", PlayerB: "Dave", TourneyDate: 20250116,
			},
			pick:    tennisabstract.BetSideB,
			odds:    1.5,
			winProb: 0.7,
			winner:  tennisabstract.BetSideB,
		},
	}
	stats := &BacktestStats{}
	log.recordParlayBet(legs, stats, 0.42, 3.0, 10)

	if len(log.rows) != 4 {
		t.Fatalf("got %d rows, want START + 2 legs + END", len(log.rows))
	}
	start := log.rows[0]
	if start[11] != parlayMarkerStart || start[12] != "2" {
		t.Fatalf("START row: skip=%s size=%s", start[11], start[12])
	}
	if start[6] != "" || start[7] != "" {
		t.Fatalf("START should not have stake/balance: %s %s", start[6], start[7])
	}

	leg1 := log.rows[1]
	if leg1[0] != "2025-01-15" || leg1[3] != "Alice" || leg1[4] != "Alice" || leg1[5] != "2.00" {
		t.Fatalf("leg1: %v", leg1[:6])
	}
	if leg1[6] != "" || leg1[11] != "" {
		t.Fatalf("leg1 should have no stake or skip_reason")
	}

	leg2 := log.rows[2]
	if leg2[0] != "2025-01-16" || leg2[3] != "Dave" || leg2[5] != "1.50" {
		t.Fatalf("leg2 pick/odds: %s %s %s", leg2[3], leg2[4], leg2[5])
	}

	end := log.rows[3]
	if end[11] != parlayMarkerEnd || end[4] != "WON" {
		t.Fatalf("END row: skip=%s result=%s", end[11], end[4])
	}
	if end[6] != "10.00" || end[7] != "1000.00" || end[8] != "1020.00" {
		t.Fatalf("END stake/balances: %s / %s / %s", end[6], end[7], end[8])
	}
	if end[13] != "3.00" || end[14] != "0.4200" {
		t.Fatalf("END combined: odds=%s prob=%s", end[13], end[14])
	}
}

func TestBetLog_recordParlaySkip(t *testing.T) {
	t.Parallel()

	log := newStrategyBetLog(1000)
	m := tennisabstract.MatchWithOdds{PlayerA: "A", PlayerB: "B", TourneyDate: 20250101}
	log.recordParlaySkip(m, skipReasonNOValidParlay, 0, 0)
	row := log.rows[0]
	if row[11] != skipReasonNOValidParlay {
		t.Fatalf("skip_reason = %q", row[11])
	}
	if row[12] != "" || row[15] != "" {
		t.Fatalf("parlay skip extras: size=%q legs=%q", row[12], row[15])
	}

	log.recordParlaySkip(m, skipReasonALLOC, 0.3, 4.5)
	row = log.rows[1]
	if row[11] != skipReasonALLOC || row[13] != "4.50" || row[14] != "0.3000" {
		t.Fatalf("ALLOC skip row: %v", row[11:])
	}
}

func TestRunBacktests_writesBetLogs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	matchesPath := filepath.Join(dir, "matches.csv")
	if err := os.WriteFile(matchesPath, []byte(smokeMatchesCSV), 0o644); err != nil {
		t.Fatal(err)
	}
	ratesPath := filepath.Join(dir, "rates.json")
	if err := os.WriteFile(ratesPath, []byte(smokeRatesJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	logDir := filepath.Join(dir, "logs")

	res, err := RunBacktests(BacktestConfig{
		MatchesPath: matchesPath,
		RatesPath:   ratesPath,
		Sims:        100,
		Alpha:       1,
		MinPick:     0.5,
		Seed:        42,
		BetLogDir:   logDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.BetLogs == nil {
		t.Fatal("expected BetLogs on result")
	}

	for _, name := range []string{betLogFileSim, betLogFileSimForm, betLogFileFavorite} {
		path := filepath.Join(logDir, name)
		rows, err := readBetLogCSV(path)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(rows) != 2 {
			t.Fatalf("%s: %d data rows, want 2 matches", name, len(rows))
		}
	}
}

func readBetLogCSV(path string) ([][]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	all, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, err
	}
	if len(all) < 2 {
		return nil, nil
	}
	return all[1:], nil
}
