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

// returnPct is net profit as a percentage of total amount wagered.
// Uses TotalWagered when set (variable stakes); otherwise bets × stake.
func (s BacktestStats) returnPct(stake float64) float64 {
	wagered := s.TotalWagered
	if wagered == 0 {
		wagered = float64(s.Bets) * stake
	}
	if wagered == 0 {
		return math.NaN()
	}
	return 100 * s.FinalBalance / wagered
}

func printBacktestResults(w io.Writer, result BacktestRunResult, stake float64, mm *MoneyManagerConfig) {
	printLabeledBacktestResults(w, "sim", result.Sim, stake, mm)
	fmt.Fprintln(w)
	printLabeledBacktestResults(w, "sim-form", result.SimForm, stake, mm)
	fmt.Fprintln(w)
	printLabeledBacktestResults(w, "favorite", result.Favorite, stake, mm)
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

// notionalBankroll is initial USDC plus net PnL, floored at zero (matches money-manager sizing).
func notionalBankroll(initial, netPnL float64) float64 {
	b := initial + netPnL
	if b < 0 {
		return 0
	}
	return b
}

func printLabeledBacktestResults(w io.Writer, label string, stats BacktestStats, stake float64, mm *MoneyManagerConfig) {
	fmt.Fprintf(w, "[%s]\n", label)
	fmt.Fprintf(w, "net_pnl=%.2f\n", stats.FinalBalance)
	if mm != nil {
		fmt.Fprintf(w, "bankroll=%.2f\n", notionalBankroll(mm.InitialBalance, stats.FinalBalance))
	}
	fmt.Fprintf(w, "gross_profit=%.2f gross_loss=%.2f\n", stats.GrossProfit, stats.GrossLoss)
	fmt.Fprintf(w, "bets=%d wins=%d losses=%d skipped=%d\n",
		stats.Bets, stats.Wins, stats.Losses, stats.Skipped)
	if stats.ParlayLegs > 0 {
		fmt.Fprintf(w, "parlay_legs=%d\n", stats.ParlayLegs)
	}
	if stats.Bets == 0 {
		fmt.Fprintln(w, "win_rate=n/a")
		fmt.Fprintln(w, "return_pct=n/a")
		fmt.Fprintln(w, stats.surfaceResultsLine())
		return
	}
	fmt.Fprintf(w, "win_rate=%.2f%%\n", 100*stats.winRate())
	fmt.Fprintf(w, "return_pct=%.2f%%\n", stats.returnPct(stake))
	if mm != nil {
		fmt.Fprintf(w, "total_wagered=%.2f\n", stats.TotalWagered)
	}
	fmt.Fprintln(w, stats.surfaceResultsLine())
}
