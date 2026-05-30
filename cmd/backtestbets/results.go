package main

import (
	"fmt"
	"io"
	"math"
)

func (s BacktestStats) winRate() float64 {
	if s.Bets == 0 {
		return math.NaN()
	}
	return float64(s.Wins) / float64(s.Bets)
}

// returnPct is net profit as a percentage of total amount wagered (bets × stake).
func (s BacktestStats) returnPct(stake float64) float64 {
	wagered := float64(s.Bets) * stake
	if wagered == 0 {
		return math.NaN()
	}
	return 100 * s.FinalBalance / wagered
}

func printBacktestResults(w io.Writer, result BacktestRunResult, stake float64) {
	printLabeledBacktestResults(w, "sim", result.Sim, stake)
	fmt.Fprintln(w)
	printLabeledBacktestResults(w, "sim-form", result.SimForm, stake)
	fmt.Fprintln(w)
	printLabeledBacktestResults(w, "favorite", result.Favorite, stake)
}

func (s BacktestStats) surfaceResultsLine() string {
	return fmt.Sprintf("hard=%s clay=%s grass=%s",
		formatSurfacePart(s.Hard),
		formatSurfacePart(s.Clay),
		formatSurfacePart(s.Grass),
	)
}

func formatSurfacePart(st surfaceBetStats) string {
	if st.Bets == 0 {
		return "0/0(n/a)"
	}
	pct := 100 * float64(st.Wins) / float64(st.Bets)
	return fmt.Sprintf("%d/%d(%.0f%%)", st.Wins, st.Bets, pct)
}

func printLabeledBacktestResults(w io.Writer, label string, stats BacktestStats, stake float64) {
	fmt.Fprintf(w, "[%s]\n", label)
	fmt.Fprintf(w, "final_balance=%.2f\n", stats.FinalBalance)
	fmt.Fprintf(w, "gross_profit=%.2f gross_loss=%.2f\n", stats.GrossProfit, stats.GrossLoss)
	fmt.Fprintf(w, "bets=%d wins=%d losses=%d skipped=%d\n",
		stats.Bets, stats.Wins, stats.Losses, stats.Skipped)
	if stats.Bets == 0 {
		fmt.Fprintln(w, "win_rate=n/a")
		fmt.Fprintln(w, "return_pct=n/a")
		fmt.Fprintln(w, stats.surfaceResultsLine())
		return
	}
	fmt.Fprintf(w, "win_rate=%.2f%%\n", 100*stats.winRate())
	fmt.Fprintf(w, "return_pct=%.2f%%\n", stats.returnPct(stake))
	fmt.Fprintln(w, stats.surfaceResultsLine())
}
