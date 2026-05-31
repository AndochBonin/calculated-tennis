package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/AndochBonin/E3/moneymanager/pkg/risk"
	"github.com/AndochBonin/E3/tennis/tennisabstract"
)

const (
	// DefaultBetLogDir is where per-run CSV bet logs are written (overwritten each run).
	DefaultBetLogDir = "backtest-logs"

	betLogFileSim      = "sim.csv"
	betLogFileSimForm  = "sim-form.csv"
	betLogFileFavorite = "favorite.csv"

	skipReasonMINPICK        = "MINPICK"
	skipReasonNEGEV          = "NEG_EV"
	skipReasonMINOdds        = "MIN_ODDS"
	skipReasonALLOC          = "ALLOC"
	skipReasonNORates        = "NO_RATES"
	skipReasonNOValidParlay  = "NO_VALID_PARLAY"

	parlayMarkerStart = "START PARLAY"
	parlayMarkerEnd   = "END PARLAY"
)

var betLogHeader = []string{
	"date",
	"player_a",
	"player_b",
	"pick",
	"winner",
	"odds",
	"stake",
	"balance_before",
	"balance_after",
	"win_prob",
	"ev",
	"skip_reason",
	"parlay_size",
	"combined_odds",
	"combined_win_prob",
	"legs",
}

// strategyBetLog records one CSV row per match walked for a single strategy track.
type strategyBetLog struct {
	initial float64
	rows    [][]string
}

func newStrategyBetLog(initial float64) *strategyBetLog {
	return &strategyBetLog{initial: initial}
}

type backtestBetLogs struct {
	sim      *strategyBetLog
	simForm  *strategyBetLog
	favorite *strategyBetLog
}

func (logs *backtestBetLogs) simTrack() *strategyBetLog {
	if logs == nil {
		return nil
	}
	return logs.sim
}

func (logs *backtestBetLogs) simFormTrack() *strategyBetLog {
	if logs == nil {
		return nil
	}
	return logs.simForm
}

func (logs *backtestBetLogs) favoriteTrack() *strategyBetLog {
	if logs == nil {
		return nil
	}
	return logs.favorite
}

func newBacktestBetLogs(mm *MoneyManagerConfig) *backtestBetLogs {
	var initial float64
	if mm != nil {
		initial = mm.InitialBalance
	}
	return &backtestBetLogs{
		sim:      newStrategyBetLog(initial),
		simForm:  newStrategyBetLog(initial),
		favorite: newStrategyBetLog(initial),
	}
}

func (l *strategyBetLog) bankroll(stats *BacktestStats) float64 {
	return notionalBankroll(l.initial, stats.FinalBalance)
}

// recordSkip logs a non-bet row. reason is empty for rate-missing skips; winProb and odds
// are used to compute ev when both are positive (e.g. NEG_EV skips).
func (l *strategyBetLog) recordSkip(m tennisabstract.MatchWithOdds, reason string, winProb, odds float64) {
	l.rows = append(l.rows, betLogRow(m, "", "", "", "", "", "", winProb, odds, reason, betLogParlayExtras{}))
}

// recordParlaySkip logs a parlay anchor that did not place. combinedWinProb and combinedOdds
// populate ev when positive (e.g. ALLOC). parlay_size is left empty.
func (l *strategyBetLog) recordParlaySkip(m tennisabstract.MatchWithOdds, reason string, combinedWinProb, combinedOdds float64) {
	l.rows = append(l.rows, betLogRow(m, "", "", "", "", "", "", combinedWinProb, combinedOdds, reason, betLogParlayExtras{
		combinedOdds:    combinedOdds,
		combinedWinProb: combinedWinProb,
	}))
}

func (l *strategyBetLog) recordBet(
	m tennisabstract.MatchWithOdds,
	stats *BacktestStats,
	picked tennisabstract.BetSide,
	odds, stake, winProb float64,
	winner tennisabstract.BetSide,
) {
	before := l.bankroll(stats)
	pnl := settleBet(picked, winner, odds, stake)
	after := before + pnl
	l.rows = append(l.rows, betLogRow(
		m,
		betSidePlayerName(m, picked),
		betSidePlayerName(m, winner),
		formatBetFloat(odds),
		formatBetFloat(stake),
		formatBetFloat(before),
		formatBetFloat(after),
		winProb,
		odds,
		"",
		betLogParlayExtras{},
	))
}

// recordParlayBet logs a placed parlay for human review: START PARLAY, one row per leg,
// then END PARLAY with stake, balances, and combined odds/probability.
func (l *strategyBetLog) recordParlayBet(
	legs []parlayLeg,
	stats *BacktestStats,
	combinedWinProb, combinedOdds, stake float64,
) {
	if len(legs) == 0 {
		return
	}
	anchor := legs[0].match
	before := l.bankroll(stats)
	pnl := settleParlay(legs, stake)
	after := before + pnl
	size := len(legs)

	l.rows = append(l.rows, betLogRow(
		anchor, "", "", "", "", "", "",
		0, 0, parlayMarkerStart,
		betLogParlayExtras{parlaySize: size},
	))
	for _, leg := range legs {
		l.rows = append(l.rows, betLogParlayLegRow(leg))
	}
	result := "LOST"
	if parlayWins(legs) {
		result = "WON"
	}
	l.rows = append(l.rows, betLogRow(
		anchor,
		"",
		result,
		"",
		formatBetFloat(stake),
		formatBetFloat(before),
		formatBetFloat(after),
		combinedWinProb,
		combinedOdds,
		parlayMarkerEnd,
		betLogParlayExtras{
			parlaySize:      size,
			combinedOdds:    combinedOdds,
			combinedWinProb: combinedWinProb,
		},
	))
}

func betLogParlayLegRow(leg parlayLeg) []string {
	m := leg.match
	return betLogRow(
		m,
		betSidePlayerName(m, leg.pick),
		betSidePlayerName(m, leg.winner),
		formatBetFloat(leg.odds),
		"", "", "",
		leg.winProb,
		leg.odds,
		"",
		betLogParlayExtras{},
	)
}

type betLogParlayExtras struct {
	parlaySize      int
	combinedOdds    float64
	combinedWinProb float64
}

func betLogRow(
	m tennisabstract.MatchWithOdds,
	pick, winner, odds, stake, before, after string,
	winProb, decimalOdds float64,
	skipReason string,
	parlay betLogParlayExtras,
) []string {
	row := []string{
		formatTourneyDate(m.TourneyDate),
		m.PlayerA,
		m.PlayerB,
		pick,
		winner,
		odds,
		stake,
		before,
		after,
		formatWinProb(winProb),
		formatEV(winProb, decimalOdds),
		skipReason,
	}
	return append(row, formatParlayExtras(parlay)...)
}

func formatParlayExtras(p betLogParlayExtras) []string {
	size := ""
	if p.parlaySize > 0 {
		size = strconv.Itoa(p.parlaySize)
	}
	combinedOdds := ""
	if p.combinedOdds > 0 {
		combinedOdds = formatBetFloat(p.combinedOdds)
	}
	return []string{size, combinedOdds, formatWinProb(p.combinedWinProb), ""}
}

func formatWinProb(p float64) string {
	if p <= 0 {
		return ""
	}
	return strconv.FormatFloat(p, 'f', 4, 64)
}

func formatEV(winProb, decimalOdds float64) string {
	if winProb <= 0 || decimalOdds <= 0 {
		return ""
	}
	return formatBetFloat(risk.ExpectedValueDecimalOdds(winProb, decimalOdds))
}

func formatTourneyDate(yyyymmdd int) string {
	if t, ok := tennisabstract.TourneyDateAsTime(yyyymmdd); ok {
		return t.Format("2006-01-02")
	}
	return strconv.Itoa(yyyymmdd)
}

func betSidePlayerName(m tennisabstract.MatchWithOdds, side tennisabstract.BetSide) string {
	switch side {
	case tennisabstract.BetSideA:
		return m.PlayerA
	case tennisabstract.BetSideB:
		return m.PlayerB
	default:
		return ""
	}
}

func formatBetFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}

func (logs *backtestBetLogs) writeDir(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create bet log dir: %w", err)
	}
	if err := writeBetLogCSV(filepath.Join(dir, betLogFileSim), logs.sim.rows); err != nil {
		return err
	}
	if err := writeBetLogCSV(filepath.Join(dir, betLogFileSimForm), logs.simForm.rows); err != nil {
		return err
	}
	if err := writeBetLogCSV(filepath.Join(dir, betLogFileFavorite), logs.favorite.rows); err != nil {
		return err
	}
	return nil
}

func writeBetLogCSV(path string, rows [][]string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.Write(betLogHeader); err != nil {
		return fmt.Errorf("write header %s: %w", path, err)
	}
	if err := w.WriteAll(rows); err != nil {
		return fmt.Errorf("write rows %s: %w", path, err)
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return fmt.Errorf("flush %s: %w", path, err)
	}
	return nil
}
