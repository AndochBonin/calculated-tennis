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
