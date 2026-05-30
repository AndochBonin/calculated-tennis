package main

import (
	"math"
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
