package main

import (
	"bytes"
	"math"
	"strings"
	"testing"
)

func TestBacktestStats_winRateAndReturnPct(t *testing.T) {
	t.Parallel()

	stats := BacktestStats{Bets: 10, Wins: 6, Losses: 4, FinalBalance: 2.5}
	if got := stats.winRate(); got != 0.6 {
		t.Fatalf("winRate = %v, want 0.6", got)
	}
	// 2.5 profit on 10 × $1 wagered = 25% ROI
	if got := stats.returnPct(1); got != 25 {
		t.Fatalf("returnPct = %v, want 25", got)
	}

	empty := BacktestStats{}
	if !math.IsNaN(empty.winRate()) {
		t.Fatal("expected NaN win rate with no bets")
	}
	if !math.IsNaN(empty.returnPct(1)) {
		t.Fatal("expected NaN return with no bets")
	}
}

func TestBacktestStats_returnPct_usesTotalWagered(t *testing.T) {
	t.Parallel()

	stats := BacktestStats{Bets: 10, FinalBalance: 5, TotalWagered: 100}
	// 5 profit on 100 wagered = 5%; must not use bets×stake (would be 50% with stake=1).
	if got := stats.returnPct(1); got != 5 {
		t.Fatalf("returnPct = %v, want 5", got)
	}
}

func TestPrintBacktestResults_moneyManagerShowsTotalWagered(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	printBacktestResults(&buf, BacktestRunResult{
		Sim: BacktestStats{
			Bets:         2,
			Wins:         1,
			Losses:       1,
			FinalBalance: 1,
			TotalWagered: 20,
			GrossProfit:  11,
			GrossLoss:    10,
		},
	}, 1, &MoneyManagerConfig{InitialBalance: 100})

	out := buf.String()
	if strings.Contains(out, "final_balance=") {
		t.Fatalf("output should use net_pnl, not final_balance:\n%s", out)
	}
	if !strings.Contains(out, "net_pnl=1.00") {
		t.Fatalf("output missing net_pnl:\n%s", out)
	}
	if !strings.Contains(out, "bankroll=101.00") {
		t.Fatalf("output missing bankroll:\n%s", out)
	}
	if !strings.Contains(out, "total_wagered=20.00") {
		t.Fatalf("output missing total_wagered:\n%s", out)
	}
	if !strings.Contains(out, "return_pct=5.00%") {
		t.Fatalf("output missing correct return_pct:\n%s", out)
	}
}

func TestPrintBacktestResults_fixedStakeOmitsTotalWagered(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	printBacktestResults(&buf, BacktestRunResult{
		Sim: BacktestStats{
			Bets:         2,
			Wins:         1,
			Losses:       1,
			FinalBalance: 1,
			TotalWagered: 2,
		},
	}, 1, nil)

	out := buf.String()
	if strings.Contains(out, "total_wagered") {
		t.Fatalf("fixed-stake output should not include total_wagered:\n%s", out)
	}
	if !strings.Contains(out, "net_pnl=1.00") {
		t.Fatalf("output missing net_pnl:\n%s", out)
	}
	if strings.Contains(out, "bankroll=") {
		t.Fatalf("fixed-stake output should not include bankroll:\n%s", out)
	}
}

func TestBacktestStats_surfaceResultsLine(t *testing.T) {
	t.Parallel()

	got := (BacktestStats{
		Hard:  surfaceBetStats{Wins: 40, Bets: 50},
		Clay:  surfaceBetStats{Wins: 50, Bets: 50},
		Grass: surfaceBetStats{Wins: 20, Bets: 50},
	}).surfaceResultsLine()
	want := "hard=40/50(80%) clay=50/50(100%) grass=20/50(40%)"
	if got != want {
		t.Fatalf("surfaceResultsLine = %q, want %q", got, want)
	}

	empty := BacktestStats{}.surfaceResultsLine()
	if empty != "hard=0/0(n/a) clay=0/0(n/a) grass=0/0(n/a)" {
		t.Fatalf("empty surface line = %q", empty)
	}
}
