package main

import (
	"context"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/AndochBonin/calculated-tennis/moneymanager/pkg/risk"
	"github.com/AndochBonin/calculated-tennis/tennis/tennis"
	"github.com/AndochBonin/calculated-tennis/tennis/tennisabstract"
)

func TestRunBacktest_smoke(t *testing.T) {
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

	cfg := BacktestConfig{
		MatchesPath: matchesPath,
		RatesPath:   ratesPath,
		Sims:        100,
		Alpha:       1,
		MinPick:     0.5,
		Seed:        42,
	}

	res1, err := RunBacktests(cfg)
	if err != nil {
		t.Fatalf("RunBacktests: %v", err)
	}
	res2, err := RunBacktests(cfg)
	if err != nil {
		t.Fatalf("RunBacktests repeat: %v", err)
	}
	if res1.Sim != res2.Sim {
		t.Fatalf("non-deterministic sim: %+v vs %+v", res1.Sim, res2.Sim)
	}
	if res1.Favorite != res2.Favorite {
		t.Fatalf("non-deterministic favorite: %+v vs %+v", res1.Favorite, res2.Favorite)
	}
	if res1.SimForm != res2.SimForm {
		t.Fatalf("non-deterministic sim-form: %+v vs %+v", res1.SimForm, res2.SimForm)
	}

	stats := res1.Sim
	if stats.MatchesWalk != 2 {
		t.Fatalf("MatchesWalk = %d, want 2", stats.MatchesWalk)
	}
	if stats.Bets+stats.Skipped != stats.MatchesWalk {
		t.Fatalf("bets(%d)+skipped(%d) != walked(%d)", stats.Bets, stats.Skipped, stats.MatchesWalk)
	}
	if stats.Wins+stats.Losses != stats.Bets {
		t.Fatalf("wins(%d)+losses(%d) != bets(%d)", stats.Wins, stats.Losses, stats.Bets)
	}
	if stats.Hard.Wins != stats.Wins || stats.Hard.Bets != stats.Bets {
		t.Fatalf("hard surface wins/bets = %d/%d, want aggregate %d/%d (smoke CSV is all Hard)",
			stats.Hard.Wins, stats.Hard.Bets, stats.Wins, stats.Bets)
	}
	if stats.GrossProfit-stats.GrossLoss != stats.FinalBalance {
		t.Fatalf("gross_profit(%.2f) - gross_loss(%.2f) != final_balance(%.2f)",
			stats.GrossProfit, stats.GrossLoss, stats.FinalBalance)
	}

	fav := res1.Favorite
	if fav.MatchesWalk != stats.MatchesWalk {
		t.Fatalf("favorite MatchesWalk = %d, sim = %d", fav.MatchesWalk, stats.MatchesWalk)
	}
	if fav.Bets+fav.Skipped != fav.MatchesWalk {
		t.Fatalf("favorite bets(%d)+skipped(%d) != walked(%d)", fav.Bets, fav.Skipped, fav.MatchesWalk)
	}

	form := res1.SimForm
	if form.MatchesWalk != stats.MatchesWalk {
		t.Fatalf("sim-form MatchesWalk = %d, sim = %d", form.MatchesWalk, stats.MatchesWalk)
	}
	if form.Bets+form.Skipped != form.MatchesWalk {
		t.Fatalf("sim-form bets(%d)+skipped(%d) != walked(%d)", form.Bets, form.Skipped, form.MatchesWalk)
	}
}

func TestRunBacktests_minOddsSkipsLowPrices(t *testing.T) {
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

	base := BacktestConfig{
		MatchesPath: matchesPath,
		RatesPath:   ratesPath,
		Sims:        100,
		Alpha:       1,
		MinPick:     0.5,
		Seed:        42,
	}
	noMin, err := RunBacktests(base)
	if err != nil {
		t.Fatal(err)
	}
	withMin, err := RunBacktests(BacktestConfig{
		MatchesPath: matchesPath,
		RatesPath:   ratesPath,
		Sims:        100,
		Alpha:       1,
		MinPick:     0.5,
		MinOdds:     2.0,
		Seed:        42,
	})
	if err != nil {
		t.Fatal(err)
	}
	if withMin.Sim.Bets > noMin.Sim.Bets {
		t.Fatalf("min-odds=2: sim bets %d > unfiltered %d", withMin.Sim.Bets, noMin.Sim.Bets)
	}
}

func TestEvAllowed(t *testing.T) {
	t.Parallel()

	if !evAllowed(0.6, 2.0) {
		t.Fatal("p=0.6 odds=2.0 should pass")
	}
	if evAllowed(0.5, 1.9) {
		t.Fatal("p=0.5 odds=1.9 should fail (E=-0.05)")
	}
	cfg := BacktestConfig{RequirePositiveEV: false}
	if !cfg.positiveEVAllowed(0.3, 1.0) {
		t.Fatal("flag off should allow any EV")
	}
	cfg.RequirePositiveEV = true
	if cfg.positiveEVAllowed(0.5, 1.9) {
		t.Fatal("flag on should reject non-positive EV")
	}
}

func TestWinProbForSide(t *testing.T) {
	t.Parallel()

	if got := winProbForSide(3000, 5000, tennisabstract.BetSideA); got != 0.6 {
		t.Fatalf("side A: got %v want 0.6", got)
	}
	if got := winProbForSide(3000, 5000, tennisabstract.BetSideB); got != 0.4 {
		t.Fatalf("side B: got %v want 0.4", got)
	}
}

func TestRunBacktests_requirePositiveEVFewerBets(t *testing.T) {
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

	base := BacktestConfig{
		MatchesPath: matchesPath,
		RatesPath:   ratesPath,
		Sims:        500,
		Alpha:       1,
		MinPick:     0.5,
		Seed:        42,
		BetLogDir:   logDir,
	}
	off, err := RunBacktests(base)
	if err != nil {
		t.Fatal(err)
	}
	on, err := RunBacktests(BacktestConfig{
		MatchesPath:       matchesPath,
		RatesPath:         ratesPath,
		Sims:              500,
		Alpha:             1,
		MinPick:           0.5,
		Seed:              42,
		RequirePositiveEV: true,
		BetLogDir:         logDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if on.Sim.Bets > off.Sim.Bets {
		t.Fatalf("require-positive-ev: sim bets %d > unfiltered %d", on.Sim.Bets, off.Sim.Bets)
	}
	rows, err := readBetLogCSV(filepath.Join(logDir, betLogFileSim))
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row[3] == "" {
			continue
		}
		ev, err := strconv.ParseFloat(row[10], 64)
		if err != nil {
			t.Fatalf("parse ev %q: %v", row[10], err)
		}
		if ev <= 0 {
			t.Fatalf("placed bet with ev=%v: %v", ev, row)
		}
	}
}

func TestBacktestConfig_oddsAllowed(t *testing.T) {
	t.Parallel()

	cfg := BacktestConfig{MinOdds: 1.5}
	if !cfg.oddsAllowed(1.5) || !cfg.oddsAllowed(2) {
		t.Fatal("expected odds >= min to be allowed")
	}
	if cfg.oddsAllowed(1.49) {
		t.Fatal("expected odds below min to be rejected")
	}
	disabled := BacktestConfig{MinOdds: 0}
	if !disabled.oddsAllowed(1.01) {
		t.Fatal("zero min-odds should allow any positive odds")
	}
}

func TestRunBacktests_moneyManagerSkipsWhenBalanceTooLow(t *testing.T) {
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

	cfg := BacktestConfig{
		MatchesPath: matchesPath,
		RatesPath:   ratesPath,
		Sims:        100,
		Alpha:       1,
		MinPick:     0.5,
		Seed:        42,
		MoneyManager: &MoneyManagerConfig{
			InitialBalance: 1,
			MaxPctBalance:  0.05,
			MinShareSize:   5,
		},
	}

	res, err := RunBacktests(cfg)
	if err != nil {
		t.Fatalf("RunBacktests: %v", err)
	}
	stats := res.Sim
	if stats.Bets != 0 {
		t.Fatalf("expected no bets with 1 USDC bankroll and min 5 shares, got %d bets", stats.Bets)
	}
	if stats.Skipped != stats.MatchesWalk {
		t.Fatalf("expected all matches skipped after allocation failure, skipped=%d walk=%d", stats.Skipped, stats.MatchesWalk)
	}
	if stats.TotalWagered != 0 {
		t.Fatalf("TotalWagered = %v, want 0", stats.TotalWagered)
	}
	if stats.GrossProfit-stats.GrossLoss != stats.FinalBalance {
		t.Fatalf("gross_profit(%.2f) - gross_loss(%.2f) != final_balance(%.2f)",
			stats.GrossProfit, stats.GrossLoss, stats.FinalBalance)
	}
}

func TestRunBacktests_moneyManagerSmoke(t *testing.T) {
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

	cfg := BacktestConfig{
		MatchesPath: matchesPath,
		RatesPath:   ratesPath,
		Sims:        100,
		Alpha:       1,
		MinPick:     0.5,
		Seed:        42,
		MoneyManager: &MoneyManagerConfig{
			InitialBalance: 10_000,
			MaxPctBalance:  0.05,
			MinShareSize:   5,
		},
	}

	fixed, err := RunBacktests(BacktestConfig{
		MatchesPath: matchesPath,
		RatesPath:   ratesPath,
		Sims:        100,
		Alpha:       1,
		MinPick:     0.5,
		Seed:        42,
		Stake:       1,
	})
	if err != nil {
		t.Fatalf("RunBacktests fixed stake: %v", err)
	}

	res, err := RunBacktests(cfg)
	if err != nil {
		t.Fatalf("RunBacktests: %v", err)
	}

	assertMMStats := func(name string, stats BacktestStats) {
		t.Helper()
		if stats.Bets == 0 {
			t.Fatalf("%s: expected bets with 10000 USDC bankroll, got 0", name)
		}
		if stats.TotalWagered <= 0 {
			t.Fatalf("%s: TotalWagered = %v, want > 0", name, stats.TotalWagered)
		}
		if stats.GrossProfit-stats.GrossLoss != stats.FinalBalance {
			t.Fatalf("%s: gross_profit(%.2f) - gross_loss(%.2f) != final_balance(%.2f)",
				name, stats.GrossProfit, stats.GrossLoss, stats.FinalBalance)
		}
	}

	assertMMStats("sim", res.Sim)
	assertMMStats("favorite", res.Favorite)
	if fixed.SimForm.Bets > 0 {
		assertMMStats("sim-form", res.SimForm)
		if res.SimForm.Bets != fixed.SimForm.Bets {
			t.Fatalf("sim-form bets = %d, fixed = %d", res.SimForm.Bets, fixed.SimForm.Bets)
		}
	}

	// Same bet/no-bet decisions as fixed stake; only sizing differs.
	if res.Sim.Bets != fixed.Sim.Bets || res.Sim.Skipped != fixed.Sim.Skipped {
		t.Fatalf("sim bets/skipped = %d/%d, fixed = %d/%d",
			res.Sim.Bets, res.Sim.Skipped, fixed.Sim.Bets, fixed.Sim.Skipped)
	}
	if res.Favorite.Bets != fixed.Favorite.Bets {
		t.Fatalf("favorite bets = %d, fixed = %d", res.Favorite.Bets, fixed.Favorite.Bets)
	}

	// Net PnL + bankroll feedback must stay in a plausible range (guards gross-return bug).
	const maxAbsPnL = 1_000_000
	if math.Abs(res.Sim.FinalBalance) > maxAbsPnL || math.IsInf(res.Sim.FinalBalance, 0) || math.IsNaN(res.Sim.FinalBalance) {
		t.Fatalf("sim FinalBalance = %v, want finite and |x| <= %d", res.Sim.FinalBalance, maxAbsPnL)
	}
}

func TestSettleBet(t *testing.T) {
	t.Parallel()

	winner := tennisabstract.BetSideA
	tests := []struct {
		picked, actual tennisabstract.BetSide
		stake, odds    float64
		want           float64
	}{
		{picked: winner, actual: winner, stake: 1, odds: 2.0, want: 1.0},
		{picked: winner, actual: winner, stake: 5, odds: 1.8, want: 4.0},
		{picked: tennisabstract.BetSideB, actual: winner, stake: 1, odds: 3.0, want: -1},
		{picked: tennisabstract.BetSideB, actual: winner, stake: 2.5, odds: 3.0, want: -2.5},
	}
	for _, tc := range tests {
		got := settleBet(tc.picked, tc.actual, tc.odds, tc.stake)
		if got != tc.want {
			t.Errorf("settleBet = %v, want %v", got, tc.want)
		}
	}
}

func writeParlaySmokeFiles(t *testing.T, dir string) (matchesPath, ratesPath string) {
	t.Helper()
	matchesPath = filepath.Join(dir, "matches.csv")
	if err := os.WriteFile(matchesPath, []byte(parlaySmokeMatchesCSV), 0o644); err != nil {
		t.Fatal(err)
	}
	ratesPath = filepath.Join(dir, "rates.json")
	if err := os.WriteFile(ratesPath, []byte(smokeRatesJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	return matchesPath, ratesPath
}

func parlayBacktestConfig(matchesPath, ratesPath string) BacktestConfig {
	return BacktestConfig{
		MatchesPath:      matchesPath,
		RatesPath:        ratesPath,
		Sims:             100,
		Alpha:            1,
		MinPick:          0.5,
		Seed:             42,
		BetMode:          BetModeParlay,
		MaxParlayMatches: 3,
	}
}

// runParlaySimTrack walks the sim parlay strategy only (no sim-form HTTP).
func runParlaySimTrack(t *testing.T, cfg BacktestConfig) BacktestStats {
	t.Helper()

	eligible, err := loadEligibleMatches(cfg)
	if err != nil {
		t.Fatal(err)
	}
	rates, err := tennisabstract.ReadPlayerRatesFile(cfg.RatesPath)
	if err != nil {
		t.Fatal(err)
	}

	var out BacktestRunResult
	mmAllocs, err := newMMWalkAllocators(cfg.MoneyManager, &out)
	if err != nil {
		t.Fatal(err)
	}
	var alloc *risk.Allocator
	if mmAllocs != nil {
		alloc = mmAllocs.sim
	}

	var log *strategyBetLog
	if cfg.BetLogDir != "" {
		logs := newBacktestBetLogs(cfg.MoneyManager)
		log = logs.sim
	}

	stats := BacktestStats{MatchesWalk: len(eligible)}
	ctx := context.Background()
	client := tennisabstract.NewClient(tennisabstract.CareerClientOptionsFromEnv()...)
	rng := rand.New(rand.NewPCG(cfg.Seed, mixSeed(cfg.Seed)))
	if err := walkParlayTrack(ctx, cfg, eligible, rates, client, rng, parlayStrategySim, &stats, alloc, log, cfg.normalizedStake(), historicalWinnerSide()); err != nil {
		t.Fatal(err)
	}
	if cfg.BetLogDir != "" {
		if err := os.MkdirAll(cfg.BetLogDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := writeBetLogCSV(filepath.Join(cfg.BetLogDir, betLogFileSim), log.rows); err != nil {
			t.Fatal(err)
		}
	}
	return stats
}

func TestRunBacktests_parlay_deterministic(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	matchesPath, ratesPath := writeParlaySmokeFiles(t, dir)
	cfg := parlayBacktestConfig(matchesPath, ratesPath)

	res1 := runParlaySimTrack(t, cfg)
	res2 := runParlaySimTrack(t, cfg)
	if res1 != res2 {
		t.Fatalf("non-deterministic sim parlay: %+v vs %+v", res1, res2)
	}
}

func TestRunBacktests_parlay_smokeInvariants(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	matchesPath, ratesPath := writeParlaySmokeFiles(t, dir)
	stats := runParlaySimTrack(t, parlayBacktestConfig(matchesPath, ratesPath))

	assertParlayStats := func(name string, stats BacktestStats) {
		t.Helper()
		if stats.MatchesWalk != 4 {
			t.Fatalf("%s: MatchesWalk = %d, want 4", name, stats.MatchesWalk)
		}
		if stats.Bets == 0 {
			t.Fatalf("%s: expected at least one parlay placed", name)
		}
		if stats.Wins+stats.Losses != stats.Bets {
			t.Fatalf("%s: wins(%d)+losses(%d) != bets(%d)", name, stats.Wins, stats.Losses, stats.Bets)
		}
		if stats.ParlayLegs < stats.Bets {
			t.Fatalf("%s: ParlayLegs(%d) < Bets(%d)", name, stats.ParlayLegs, stats.Bets)
		}
		if stats.GrossProfit-stats.GrossLoss != stats.FinalBalance {
			t.Fatalf("%s: gross_profit(%.2f) - gross_loss(%.2f) != final_balance(%.2f)",
				name, stats.GrossProfit, stats.GrossLoss, stats.FinalBalance)
		}
	}

	assertParlayStats("sim", stats)
	if stats.Bets+stats.Skipped == stats.MatchesWalk {
		t.Fatalf("sim: parlay mode must not satisfy singles bets+skipped==walked (bets=%d skipped=%d)",
			stats.Bets, stats.Skipped)
	}
}

func TestRunBacktests_parlay_requirePositiveEVFewerParlays(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	matchesPath, ratesPath := writeParlaySmokeFiles(t, dir)
	logDir := filepath.Join(dir, "logs")

	off := runParlaySimTrack(t, parlayBacktestConfig(matchesPath, ratesPath))
	onCfg := parlayBacktestConfig(matchesPath, ratesPath)
	onCfg.RequirePositiveEV = true
	onCfg.BetLogDir = logDir
	on := runParlaySimTrack(t, onCfg)
	if on.Bets > off.Bets {
		t.Fatalf("require-positive-ev: sim parlays %d > unfiltered %d", on.Bets, off.Bets)
	}

	rows, err := readBetLogCSV(filepath.Join(logDir, betLogFileSim))
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row[11] != parlayMarkerEnd {
			continue
		}
		ev, err := strconv.ParseFloat(row[10], 64)
		if err != nil {
			t.Fatalf("parse ev %q: %v", row[10], err)
		}
		if ev <= 0 {
			t.Fatalf("placed parlay with ev=%v: %v", ev, row)
		}
	}
}

func TestRunBacktests_parlay_moneyManagerOneAllocationPerParlay(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	matchesPath, ratesPath := writeParlaySmokeFiles(t, dir)
	logDir := filepath.Join(dir, "logs")

	fixed := runParlaySimTrack(t, parlayBacktestConfig(matchesPath, ratesPath))

	mmCfg := parlayBacktestConfig(matchesPath, ratesPath)
	mmCfg.BetLogDir = logDir
	mmCfg.MoneyManager = &MoneyManagerConfig{
		InitialBalance: 10_000,
		MaxPctBalance:  0.05,
		MinShareSize:   5,
	}
	mm := runParlaySimTrack(t, mmCfg)

	if mm.Bets == 0 {
		t.Fatal("expected parlays with money manager bankroll")
	}
	if mm.Bets != fixed.Bets {
		t.Fatalf("sim parlays = %d, fixed stake = %d (same placement decisions)", mm.Bets, fixed.Bets)
	}
	if mm.ParlayLegs != fixed.ParlayLegs {
		t.Fatalf("sim ParlayLegs = %d, fixed = %d", mm.ParlayLegs, fixed.ParlayLegs)
	}
	if mm.TotalWagered <= 0 {
		t.Fatalf("TotalWagered = %v, want > 0", mm.TotalWagered)
	}
	if mm.ParlayLegs < mm.Bets {
		t.Fatalf("ParlayLegs(%d) < Bets(%d)", mm.ParlayLegs, mm.Bets)
	}

	rows, err := readBetLogCSV(filepath.Join(logDir, betLogFileSim))
	if err != nil {
		t.Fatal(err)
	}
	placed := 0
	for _, row := range rows {
		if row[11] != parlayMarkerEnd {
			continue
		}
		placed++
		stake, err := strconv.ParseFloat(row[6], 64)
		if err != nil || stake <= 0 {
			t.Fatalf("parlay END row stake %q: %v", row[6], err)
		}
	}
	if placed != mm.Bets {
		t.Fatalf("bet log END PARLAY rows = %d, stats.Bets = %d", placed, mm.Bets)
	}
}

func TestWalkParlayTrack_ratesGapMiddleRow(t *testing.T) {
	t.Parallel()

	eligible := parlayRatesGapEligible()
	dir := t.TempDir()
	ratesPath := filepath.Join(dir, "rates.json")
	if err := os.WriteFile(ratesPath, []byte(parlayRatesGapJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	rates, err := tennisabstract.ReadPlayerRatesFile(ratesPath)
	if err != nil {
		t.Fatal(err)
	}
	cfg := BacktestConfig{
		Sims:             200,
		Alpha:            1,
		MinPick:          0.5,
		Seed:             42,
		MaxParlayMatches: 1,
	}
	ctx := context.Background()
	client := tennisabstract.NewClient()
	rng := rand.New(rand.NewPCG(cfg.Seed, mixSeed(cfg.Seed)))
	log := newStrategyBetLog(0)
	var stats BacktestStats
	stats.MatchesWalk = len(eligible)

	if err := walkParlayTrack(ctx, cfg, eligible, rates, client, rng, parlayStrategySim, &stats, nil, log, 1, historicalWinnerSide()); err != nil {
		t.Fatal(err)
	}
	if stats.Bets != 3 {
		t.Fatalf("Bets = %d, want 3 (one-leg at 0,1 and after gap)", stats.Bets)
	}
	if stats.Skipped != 1 {
		t.Fatalf("Skipped = %d, want 1 (middle row missing rates)", stats.Skipped)
	}
	if stats.ParlayLegs != 3 {
		t.Fatalf("ParlayLegs = %d, want 3", stats.ParlayLegs)
	}

	var noRates int
	for _, row := range log.rows {
		if row[11] == skipReasonNORates {
			noRates++
			if !strings.Contains(row[1], "Eve") {
				t.Fatalf("NO_RATES skip expected Eve anchor, got players %s vs %s", row[1], row[2])
			}
		}
	}
	if noRates != 1 {
		t.Fatalf("NO_RATES log rows = %d, want 1", noRates)
	}
}

func parlayRatesGapEligible() []tennisabstract.MatchWithOdds {
	fmt := tennis.DefaultFormat()
	return []tennisabstract.MatchWithOdds{
		{
			Surface: tennisabstract.SurfaceHard, Format: fmt, Score: "6-3 6-4", BestOf: 3,
			TourneyDate: 20250101, MatchNum: 1,
			PlayerA: "Alice Strong", PlayerB: "Bob Weak",
			PlayerASlug: "AliceStrong", PlayerBSlug: "BobWeak",
			AvgW: 2.0, AvgL: 3.0,
		},
		{
			Surface: tennisabstract.SurfaceHard, Format: fmt, Score: "7-6(5) 6-2", BestOf: 3,
			TourneyDate: 20250102, MatchNum: 1,
			PlayerA: "Carol Ace", PlayerB: "Dan Base",
			PlayerASlug: "CarolAce", PlayerBSlug: "DanBase",
			AvgW: 1.5, AvgL: 2.5,
		},
		{
			Surface: tennisabstract.SurfaceHard, Format: fmt, Score: "6-4 6-3", BestOf: 3,
			TourneyDate: 20250103, MatchNum: 1,
			PlayerA: "Eve Missing", PlayerB: "Frank Out",
			PlayerASlug: "EveMissing", PlayerBSlug: "FrankOut",
			AvgW: 1.8, AvgL: 2.0,
		},
		{
			Surface: tennisabstract.SurfaceHard, Format: fmt, Score: "6-2 6-1", BestOf: 3,
			TourneyDate: 20250104, MatchNum: 1,
			PlayerA: "Carol Ace", PlayerB: "Dan Base",
			PlayerASlug: "CarolAce", PlayerBSlug: "DanBase",
			AvgW: 1.5, AvgL: 2.5,
		},
	}
}

const parlayRatesGapJSON = `{
  "AliceStrong": {"hold_2024": 0.85, "break_2024": 0.15},
  "BobWeak": {"hold_2024": 0.55, "break_2024": 0.45},
  "CarolAce": {"hold_2024": 0.80, "break_2024": 0.20},
  "DanBase": {"hold_2024": 0.70, "break_2024": 0.30},
  "FrankOut": {"hold_2024": 0.65, "break_2024": 0.35}
}`

const smokeMatchesCSV = `tourney_id,tourney_name,surface,draw_size,tourney_level,tourney_date,match_num,winner_id,winner_seed,winner_entry,winner_name,winner_hand,winner_ht,winner_ioc,winner_age,loser_id,loser_seed,loser_entry,loser_name,loser_hand,loser_ht,loser_ioc,loser_age,score,best_of,round,minutes,w_ace,w_df,w_svpt,w_1stIn,w_1stWon,w_2ndWon,w_SvGms,w_bpSaved,w_bpFaced,l_ace,l_df,l_svpt,l_1stIn,l_1stWon,l_2ndWon,l_SvGms,l_bpSaved,l_bpFaced,winner_rank,winner_rank_points,loser_rank,loser_rank_points,AvgW,AvgL
2025-001,Test Open,Hard,32,A,20250101,1,1,,,Alice Strong,R,180,USA,25,2,,,Bob Weak,R,180,USA,26,6-3 6-4,3,R32,90,,,,,,,,,,,,,,,,,,,,,,,2.0,3.0
2025-001,Test Open,Hard,32,A,20250102,1,3,,,Carol Ace,R,175,USA,24,4,,,Dan Base,R,175,USA,27,7-6(5) 6-2,3,R32,100,,,,,,,,,,,,,,,,,,,,,,,1.5,2.5
`

const parlaySmokeMatchesCSV = smokeMatchesCSV + `2025-001,Test Open,Hard,32,A,20250103,1,5,,,Eve Edge,R,175,USA,23,6,,,Finn Fair,R,175,USA,28,6-1 6-2,3,R32,80,,,,,,,,,,,,,,,,,,,,,,,1.6,2.2
2025-001,Test Open,Hard,32,A,20250104,1,3,,,Carol Ace,R,175,USA,24,4,,,Dan Base,R,175,USA,27,6-3 6-4,3,R32,95,,,,,,,,,,,,,,,,,,,,,,,1.55,2.45
`

const smokeRatesJSON = `{
  "AliceStrong": {"hold_2024": 0.85, "break_2024": 0.15},
  "BobWeak": {"hold_2024": 0.55, "break_2024": 0.45},
  "CarolAce": {"hold_2024": 0.80, "break_2024": 0.20},
  "DanBase": {"hold_2024": 0.70, "break_2024": 0.30},
  "EveEdge": {"hold_2024": 0.78, "break_2024": 0.22},
  "FinnFair": {"hold_2024": 0.68, "break_2024": 0.32}
}
`
