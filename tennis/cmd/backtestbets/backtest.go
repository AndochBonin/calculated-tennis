package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"math/rand/v2"
	"sort"

	"github.com/AndochBonin/E3/moneymanager/pkg/risk"
	"github.com/AndochBonin/E3/tennis/tennis"
	"github.com/AndochBonin/E3/tennis/tennisabstract"
	"github.com/ethereum/go-ethereum/common"
	"github.com/shopspring/decimal"
)

// BetMode selects singles (one bet per match) or parlay (greedy non-overlapping windows).
type BetMode string

const (
	BetModeSingles BetMode = "singles"
	BetModeParlay  BetMode = "parlay"
)

// MoneyManagerConfig holds bankroll and risk limits for dynamic stake sizing.
type MoneyManagerConfig struct {
	InitialBalance float64
	MaxOrderUSDC   string // empty = no absolute cap
	MaxPctBalance  float64
	MinShareSize   float64
}

// BacktestConfig controls the chronological betting walk.
type BacktestConfig struct {
	MatchesPath   string
	RatesPath     string
	Sims          int
	Alpha         float64
	MinPick       float64
	MinOdds           float64 // skip bets below this decimal odds; 0 = no minimum
	RequirePositiveEV bool    // reject bets when sim E = p·odds − 1 ≤ 0; default false
	Stake             float64 // amount risked per bet (win profit is stake×(odds−1)); ignored when MoneyManager is set
	Seed          uint64
	Log           *slog.Logger
	MoneyManager      *MoneyManagerConfig // nil = fixed stake mode
	BetLogDir         string              // directory for per-match CSV logs; empty disables logging (tests)
	BetMode           BetMode             // singles (default) or parlay
	MaxParlayMatches  int                 // max legs per parlay; required when BetMode is parlay
}

func (cfg BacktestConfig) log() *slog.Logger {
	if cfg.Log != nil {
		return cfg.Log
	}
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// surfaceBetStats counts wins and settled bets on one court surface.
type surfaceBetStats struct {
	Wins int
	Bets int
}

// BacktestStats aggregates walk outcomes.
//
// Singles mode: one row walked per eligible match; Bets counts settled singles;
// Bets+Skipped equals MatchesWalk when every match is either bet or skipped.
//
// Parlay mode: Bets counts parlays placed (not legs); ParlayLegs is the sum of leg
// counts in those parlays. Skipped counts failed anchors and rate-missing rows;
// Bets+Skipped does not equal MatchesWalk. Surface wins/bets attribute each parlay
// to the first leg's surface.
type BacktestStats struct {
	FinalBalance  float64
	GrossProfit   float64 // sum of net profit on wins (stake × (odds − 1))
	GrossLoss     float64 // sum of stakes lost
	TotalWagered  float64 // sum of stakes placed (money manager mode)
	Bets          int
	Wins          int
	Losses        int
	Skipped       int // singles: no bet; parlay: failed anchor or missing rates
	ParlayLegs    int // parlay mode: total legs in placed parlays
	MatchesWalk   int
	Hard         surfaceBetStats
	Clay         surfaceBetStats
	Grass        surfaceBetStats
}

// BacktestRunResult holds sim (season rates), sim with recent form, and favorite baseline.
// Favorite uses the same bet/no-bet decisions as Sim (min-pick); only the side differs.
type BacktestRunResult struct {
	Sim      BacktestStats
	SimForm  BacktestStats
	Favorite BacktestStats
	BetLogs  *backtestBetLogs // nil when BetLogDir is empty
}

// RunBacktests walks eligible matches once: season sim, recent-form sim, and favorite
// (favorite mirrors Sim bet/no-bet; SimForm decides independently).
func RunBacktests(cfg BacktestConfig) (BacktestRunResult, error) {
	if err := (&cfg).validate(); err != nil {
		return BacktestRunResult{}, err
	}

	eligible, err := loadEligibleMatches(cfg)
	if err != nil {
		return BacktestRunResult{}, err
	}

	res, err := walkBacktests(context.Background(), cfg, eligible)
	if err != nil {
		return BacktestRunResult{}, err
	}
	if cfg.BetLogDir != "" {
		if err := res.BetLogs.writeDir(cfg.BetLogDir); err != nil {
			return BacktestRunResult{}, err
		}
	}
	return res, nil
}

// RunBacktest runs only the sim strategy (kept for tests).
func RunBacktest(cfg BacktestConfig) (BacktestStats, error) {
	res, err := RunBacktests(cfg)
	if err != nil {
		return BacktestStats{}, err
	}
	return res.Sim, nil
}

func (cfg *BacktestConfig) validate() error {
	if cfg.Sims <= 0 {
		return fmt.Errorf("sims must be positive")
	}
	if cfg.Alpha <= 0 {
		return fmt.Errorf("alpha must be positive")
	}
	if cfg.MinPick <= 0 || cfg.MinPick > 1 {
		return fmt.Errorf("min-pick must be in (0,1]")
	}
	if cfg.MinOdds < 0 {
		return fmt.Errorf("min-odds must be non-negative")
	}
	if cfg.MinOdds > 0 && cfg.MinOdds < 1 {
		return fmt.Errorf("min-odds must be >= 1 when set")
	}
	if cfg.Stake == 0 {
		cfg.Stake = 1
	}
	if cfg.Stake <= 0 {
		return fmt.Errorf("stake must be positive")
	}
	if cfg.MoneyManager != nil {
		mm := cfg.MoneyManager
		if mm.InitialBalance <= 0 {
			return fmt.Errorf("initial balance must be positive")
		}
		if mm.MaxPctBalance <= 0 || mm.MaxPctBalance > 1 {
			return fmt.Errorf("max pct balance must be in (0,1]")
		}
		if mm.MinShareSize <= 0 {
			return fmt.Errorf("min share size must be positive")
		}
		if mm.MaxOrderUSDC != "" {
			cap, err := decimal.NewFromString(mm.MaxOrderUSDC)
			if err != nil {
				return fmt.Errorf("max order USDC: %w", err)
			}
			if cap.IsNegative() {
				return fmt.Errorf("max order USDC must be non-negative")
			}
		}
	}
	if cfg.BetMode == "" {
		cfg.BetMode = BetModeSingles
	}
	switch cfg.BetMode {
	case BetModeSingles:
	case BetModeParlay:
		if cfg.MaxParlayMatches < 1 {
			return fmt.Errorf("max parlay matches must be >= 1 in parlay mode")
		}
	default:
		return fmt.Errorf("bet mode must be %q or %q", BetModeSingles, BetModeParlay)
	}
	return nil
}

func (cfg *BacktestConfig) normalizedStake() float64 {
	if cfg.Stake == 0 {
		return 1
	}
	return cfg.Stake
}

func (cfg BacktestConfig) oddsAllowed(odds float64) bool {
	if cfg.MinOdds <= 0 {
		return true
	}
	return odds >= cfg.MinOdds
}

func evAllowed(winProb, odds float64) bool {
	return risk.ExpectedValueDecimalOdds(winProb, odds) > 0
}

func (cfg BacktestConfig) positiveEVAllowed(winProb, odds float64) bool {
	if !cfg.RequirePositiveEV {
		return true
	}
	return evAllowed(winProb, odds)
}

func winProbForSide(winsA, sims int, side tennisabstract.BetSide) float64 {
	switch side {
	case tennisabstract.BetSideA:
		return float64(winsA) / float64(sims)
	case tennisabstract.BetSideB:
		return float64(sims-winsA) / float64(sims)
	default:
		return 0
	}
}

func loadEligibleMatches(cfg BacktestConfig) ([]tennisabstract.MatchWithOdds, error) {
	log := cfg.log()

	rows, err := tennisabstract.LoadMatchesWithOddsCSVFile(cfg.MatchesPath)
	if err != nil {
		return nil, fmt.Errorf("load matches: %w", err)
	}
	rates, err := tennisabstract.ReadPlayerRatesFile(cfg.RatesPath)
	if err != nil {
		return nil, fmt.Errorf("load rates: %w", err)
	}

	eligible := tennisabstract.FilterBacktestMatches(rows, rates)
	sort.SliceStable(eligible, func(i, j int) bool {
		if eligible[i].TourneyDate != eligible[j].TourneyDate {
			return eligible[i].TourneyDate < eligible[j].TourneyDate
		}
		if eligible[i].TourneyID != eligible[j].TourneyID {
			return eligible[i].TourneyID < eligible[j].TourneyID
		}
		return eligible[i].MatchNum < eligible[j].MatchNum
	})
	log.Info("eligible matches", "count", len(eligible), "filtered_out", len(rows)-len(eligible))
	return eligible, nil
}

func walkBacktests(ctx context.Context, cfg BacktestConfig, eligible []tennisabstract.MatchWithOdds) (BacktestRunResult, error) {
	rates, err := tennisabstract.ReadPlayerRatesFile(cfg.RatesPath)
	if err != nil {
		return BacktestRunResult{}, fmt.Errorf("load rates: %w", err)
	}

	log := cfg.log()
	opts := tennisabstract.CareerClientOptionsFromEnv()
	taClient := tennisabstract.NewClient(opts...)
	log.Info("recent form walk enabled", "career_dir", tennisabstract.CareerCacheDirFromEnv())

	rng := rand.New(rand.NewPCG(cfg.Seed, mixSeed(cfg.Seed)))

	var out BacktestRunResult
	n := len(eligible)
	out.Sim.MatchesWalk = n
	out.SimForm.MatchesWalk = n
	out.Favorite.MatchesWalk = n

	mmAllocs, err := newMMWalkAllocators(cfg.MoneyManager, &out)
	if err != nil {
		return BacktestRunResult{}, err
	}
	fixedStake := cfg.normalizedStake()
	winner := historicalWinnerSide()

	var logs *backtestBetLogs
	if cfg.BetLogDir != "" {
		logs = newBacktestBetLogs(cfg.MoneyManager)
		out.BetLogs = logs
	}

	if cfg.BetMode == BetModeParlay {
		return walkBacktestsParlay(ctx, cfg, eligible, rates, taClient, rng, &out, mmAllocs, logs, fixedStake, winner)
	}

	return walkBacktestsSingles(ctx, cfg, eligible, rates, taClient, rng, &out, mmAllocs, logs, fixedStake, winner)
}

func walkBacktestsSingles(
	ctx context.Context,
	cfg BacktestConfig,
	eligible []tennisabstract.MatchWithOdds,
	rates tennisabstract.PlayerRatesMap,
	taClient *tennisabstract.Client,
	rng *rand.Rand,
	out *BacktestRunResult,
	mmAllocs *mmWalkAllocators,
	logs *backtestBetLogs,
	fixedStake float64,
	winner tennisabstract.BetSide,
) (BacktestRunResult, error) {
	for _, m := range eligible {
		baseRates, ok := tennisabstract.MatchWithOddsPlayerRates(ctx, m, rates, taClient, false, tennisabstract.FormOptions{})
		if !ok {
			out.Sim.Skipped++
			out.SimForm.Skipped++
			out.Favorite.Skipped++
			if logs != nil {
				logs.sim.recordSkip(m, "", 0, 0)
				logs.simForm.recordSkip(m, "", 0, 0)
				logs.favorite.recordSkip(m, "", 0, 0)
			}
			continue
		}

		winsA, simSide, simOdds, simWinProb, bet, err := simulateAndDecide(m, baseRates, cfg, rng)
		if err != nil {
			return BacktestRunResult{}, err
		}
		var simAlloc, favAlloc, formAlloc *risk.Allocator
		if mmAllocs != nil {
			simAlloc, favAlloc, formAlloc = mmAllocs.sim, mmAllocs.favorite, mmAllocs.simForm
		}
		tryPlaceSimBet(ctx, cfg, simAlloc, logs.simTrack(), m, &out.Sim, simSide, simOdds, simWinProb, bet, winner, fixedStake)
		if err := tryPlaceFavoriteBet(ctx, cfg, favAlloc, logs.favoriteTrack(), m, &out.Favorite, winsA, bet, winner, fixedStake); err != nil {
			return BacktestRunResult{}, err
		}

		formOpts := tennisabstract.FormOptionsFromEnv(m.Surface)
		formRates, formOK := tennisabstract.MatchWithOddsPlayerRates(ctx, m, rates, taClient, true, formOpts)
		if !formOK {
			out.SimForm.Skipped++
			if logs != nil {
				logs.simForm.recordSkip(m, "", 0, 0)
			}
			continue
		}
		_, formSide, formOdds, formWinProb, formBet, err := simulateAndDecide(m, formRates, cfg, rng)
		if err != nil {
			return BacktestRunResult{}, err
		}
		tryPlaceSimBet(ctx, cfg, formAlloc, logs.simFormTrack(), m, &out.SimForm, formSide, formOdds, formWinProb, formBet, winner, fixedStake)
	}

	return *out, nil
}

func walkBacktestsParlay(
	ctx context.Context,
	cfg BacktestConfig,
	eligible []tennisabstract.MatchWithOdds,
	rates tennisabstract.PlayerRatesMap,
	taClient *tennisabstract.Client,
	rng *rand.Rand,
	out *BacktestRunResult,
	mmAllocs *mmWalkAllocators,
	logs *backtestBetLogs,
	fixedStake float64,
	winner tennisabstract.BetSide,
) (BacktestRunResult, error) {
	var simAlloc, favAlloc, formAlloc *risk.Allocator
	if mmAllocs != nil {
		simAlloc, favAlloc, formAlloc = mmAllocs.sim, mmAllocs.favorite, mmAllocs.simForm
	}
	tracks := []struct {
		strategy parlayStrategy
		stats    *BacktestStats
		alloc    *risk.Allocator
		log      *strategyBetLog
	}{
		{parlayStrategySim, &out.Sim, simAlloc, logs.simTrack()},
		{parlayStrategyFavorite, &out.Favorite, favAlloc, logs.favoriteTrack()},
		{parlayStrategySimForm, &out.SimForm, formAlloc, logs.simFormTrack()},
	}
	for _, track := range tracks {
		if err := walkParlayTrack(ctx, cfg, eligible, rates, taClient, rng, track.strategy, track.stats, track.alloc, track.log, fixedStake, winner); err != nil {
			return BacktestRunResult{}, err
		}
	}
	return *out, nil
}

func walkParlayTrack(
	ctx context.Context,
	cfg BacktestConfig,
	eligible []tennisabstract.MatchWithOdds,
	rates tennisabstract.PlayerRatesMap,
	client *tennisabstract.Client,
	rng *rand.Rand,
	strategy parlayStrategy,
	stats *BacktestStats,
	alloc *risk.Allocator,
	log *strategyBetLog,
	fixedStake float64,
	winner tennisabstract.BetSide,
) error {
	deps := newParlayEvalDeps(ctx, cfg, rates, client, rng, strategy, winner)
	i := 0
	for i < len(eligible) {
		m := eligible[i]
		legs, k, ok, err := selectParlayWindow(i, eligible, cfg, cfg.MaxParlayMatches, deps)
		if err != nil {
			return err
		}
		if ok {
			placeParlay(ctx, cfg, alloc, log, legs, stats, fixedStake)
			i += k
			continue
		}
		reason := skipReasonNOValidParlay
		if _, legOK, err := deps.evalLeg(m); err != nil {
			return err
		} else if !legOK {
			reason = skipReasonNORates
		}
		stats.Skipped++
		if log != nil {
			log.recordParlaySkip(m, reason, 0, 0)
		}
		i++
	}
	return nil
}

// simBalance implements risk.BalanceReader for backtest bankrolls.
// Available USDC is initial balance plus cumulative PnL (FinalBalance).
type simBalance struct {
	initial float64
	stats   *BacktestStats
}

func (s *simBalance) USDCTotal(ctx context.Context, _ common.Address) (*big.Int, error) {
	_ = ctx
	available := decimal.NewFromFloat(s.initial).Add(decimal.NewFromFloat(s.stats.FinalBalance))
	if available.IsNegative() {
		available = decimal.Zero
	}
	raw := available.Mul(decimal.NewFromInt(1_000_000)).Round(0).BigInt()
	return raw, nil
}

type mmWalkAllocators struct {
	sim      *risk.Allocator
	favorite *risk.Allocator
	simForm  *risk.Allocator
}

func newMMWalkAllocators(mm *MoneyManagerConfig, out *BacktestRunResult) (*mmWalkAllocators, error) {
	if mm == nil {
		return nil, nil
	}
	riskCfg, err := riskConfigFromMoneyManager(mm)
	if err != nil {
		return nil, err
	}
	return &mmWalkAllocators{
		sim: risk.NewAllocator(&simBalance{initial: mm.InitialBalance, stats: &out.Sim}, riskCfg),
		favorite: risk.NewAllocator(&simBalance{initial: mm.InitialBalance, stats: &out.Favorite}, riskCfg),
		simForm: risk.NewAllocator(&simBalance{initial: mm.InitialBalance, stats: &out.SimForm}, riskCfg),
	}, nil
}

func riskConfigFromMoneyManager(mm *MoneyManagerConfig) (risk.Config, error) {
	cfg := risk.Config{
		MaxPctBalance: decimal.NewFromFloat(mm.MaxPctBalance),
		MinShareSize:  decimal.NewFromFloat(mm.MinShareSize),
	}
	if mm.MaxOrderUSDC != "" {
		cap, err := decimal.NewFromString(mm.MaxOrderUSDC)
		if err != nil {
			return risk.Config{}, fmt.Errorf("max order USDC: %w", err)
		}
		cfg.MaxOrderUSDC = cap
	}
	return cfg, nil
}

func tryPlaceSimBet(
	ctx context.Context,
	cfg BacktestConfig,
	alloc *risk.Allocator,
	log *strategyBetLog,
	m tennisabstract.MatchWithOdds,
	stats *BacktestStats,
	side tennisabstract.BetSide,
	odds, winProb float64,
	bet bool,
	winner tennisabstract.BetSide,
	fixedStake float64,
) {
	if !bet {
		stats.Skipped++
		if log != nil {
			log.recordSkip(m, skipReasonMINPICK, 0, 0)
		}
		return
	}
	if !cfg.positiveEVAllowed(winProb, odds) {
		stats.Skipped++
		if log != nil {
			log.recordSkip(m, skipReasonNEGEV, winProb, odds)
		}
		return
	}
	if !cfg.oddsAllowed(odds) {
		stats.Skipped++
		if log != nil {
			log.recordSkip(m, skipReasonMINOdds, winProb, odds)
		}
		return
	}
	placeBet(ctx, alloc, log, m, stats, side, odds, winProb, winner, fixedStake)
}

func tryPlaceFavoriteBet(
	ctx context.Context,
	cfg BacktestConfig,
	alloc *risk.Allocator,
	log *strategyBetLog,
	m tennisabstract.MatchWithOdds,
	stats *BacktestStats,
	winsA int,
	simBet bool,
	winner tennisabstract.BetSide,
	fixedStake float64,
) error {
	if !simBet {
		stats.Skipped++
		if log != nil {
			log.recordSkip(m, skipReasonMINPICK, 0, 0)
		}
		return nil
	}
	favSide, favOdds, ok := tennisabstract.DecideFavoriteBet(m.AvgW, m.AvgL)
	if !ok {
		return fmt.Errorf("favorite bet %s vs %s: invalid odds", m.PlayerA, m.PlayerB)
	}
	favWinProb := winProbForSide(winsA, cfg.Sims, favSide)
	if !cfg.positiveEVAllowed(favWinProb, favOdds) {
		stats.Skipped++
		if log != nil {
			log.recordSkip(m, skipReasonNEGEV, favWinProb, favOdds)
		}
		return nil
	}
	if !cfg.oddsAllowed(favOdds) {
		stats.Skipped++
		if log != nil {
			log.recordSkip(m, skipReasonMINOdds, favWinProb, favOdds)
		}
		return nil
	}
	placeBet(ctx, alloc, log, m, stats, favSide, favOdds, favWinProb, winner, fixedStake)
	return nil
}

func placeBet(
	ctx context.Context,
	alloc *risk.Allocator,
	log *strategyBetLog,
	m tennisabstract.MatchWithOdds,
	stats *BacktestStats,
	picked tennisabstract.BetSide,
	odds, winProb float64,
	winner tennisabstract.BetSide,
	fixedStake float64,
) {
	if alloc != nil {
		tryApplyBet(ctx, alloc, log, m, stats, picked, odds, winProb, winner)
		return
	}
	applyBetWithLog(log, m, stats, picked, odds, fixedStake, winProb, winner)
}

func tryApplyBet(
	ctx context.Context,
	alloc *risk.Allocator,
	log *strategyBetLog,
	m tennisabstract.MatchWithOdds,
	stats *BacktestStats,
	picked tennisabstract.BetSide,
	odds, winProb float64,
	actualWinner tennisabstract.BetSide,
) {
	stake, ok := allocateStake(ctx, alloc, odds)
	if !ok {
		stats.Skipped++
		if log != nil {
			log.recordSkip(m, skipReasonALLOC, winProb, odds)
		}
		return
	}
	applyBetWithLog(log, m, stats, picked, odds, stake, winProb, actualWinner)
}

func applyBetWithLog(
	log *strategyBetLog,
	m tennisabstract.MatchWithOdds,
	stats *BacktestStats,
	picked tennisabstract.BetSide,
	odds, stake, winProb float64,
	actualWinner tennisabstract.BetSide,
) {
	if log != nil {
		log.recordBet(m, stats, picked, odds, stake, winProb, actualWinner)
	}
	applyBet(stats, m.Surface, picked, odds, stake, actualWinner)
}

func allocateStake(ctx context.Context, alloc *risk.Allocator, decimalOdds float64) (float64, bool) {
	price := decimal.NewFromInt(1).Div(decimal.NewFromFloat(decimalOdds))
	size, err := alloc.Allocate(ctx, risk.SideBuy, price)
	if err != nil {
		return 0, false
	}
	stake, _ := size.Mul(price).Float64()
	return stake, true
}

func simulateAndDecide(
	m tennisabstract.MatchWithOdds,
	playerRates [2]tennis.PlayerRates,
	cfg BacktestConfig,
	rng *rand.Rand,
) (winsA int, side tennisabstract.BetSide, odds, winProb float64, bet bool, err error) {
	result, err := tennis.SimulateFresh(m.Format, playerRates, cfg.Alpha, cfg.Sims, rng)
	if err != nil {
		return 0, tennisabstract.BetSideNone, 0, 0, false, fmt.Errorf("simulate %s vs %s: %w", m.PlayerA, m.PlayerB, err)
	}
	winsA = result.WinCount(tennis.A)
	side, odds, ok := tennisabstract.DecideBet(winsA, cfg.Sims, cfg.MinPick, m.AvgW, m.AvgL)
	if !ok {
		return 0, tennisabstract.BetSideNone, 0, 0, false, fmt.Errorf("decide bet %s vs %s: invalid inputs", m.PlayerA, m.PlayerB)
	}
	if side == tennisabstract.BetSideNone {
		return winsA, side, odds, 0, false, nil
	}
	return winsA, side, odds, winProbForSide(winsA, cfg.Sims, side), true, nil
}

// historicalWinnerSide is the match winner in Sackmann rows (winner_name = player A).
func historicalWinnerSide() tennisabstract.BetSide {
	return tennisabstract.BetSideA
}

func applyBet(stats *BacktestStats, surface tennisabstract.MatchSurface, picked tennisabstract.BetSide, odds, stake float64, actualWinner tennisabstract.BetSide) {
	stats.Bets++
	stats.TotalWagered += stake
	surf := stats.surfaceStats(surface)
	surf.Bets++
	pnl := settleBet(picked, actualWinner, odds, stake)
	stats.FinalBalance += pnl
	if pnl > 0 {
		stats.Wins++
		surf.Wins++
		stats.GrossProfit += pnl
	} else {
		stats.Losses++
		stats.GrossLoss += -pnl
	}
	stats.setSurfaceStats(surface, surf)
}

func (s *BacktestStats) surfaceStats(surface tennisabstract.MatchSurface) surfaceBetStats {
	switch surface {
	case tennisabstract.SurfaceHard:
		return s.Hard
	case tennisabstract.SurfaceClay:
		return s.Clay
	case tennisabstract.SurfaceGrass:
		return s.Grass
	default:
		return surfaceBetStats{}
	}
}

func (s *BacktestStats) setSurfaceStats(surface tennisabstract.MatchSurface, st surfaceBetStats) {
	switch surface {
	case tennisabstract.SurfaceHard:
		s.Hard = st
	case tennisabstract.SurfaceClay:
		s.Clay = st
	case tennisabstract.SurfaceGrass:
		s.Grass = st
	}
}

func mixSeed(seed uint64) uint64 {
	return seed ^ 0x9e3779b97f4a7c15
}

// settleBet returns net PnL: stake×(decimal odds−1) on a win, −stake on a loss.
func settleBet(picked, actualWinner tennisabstract.BetSide, odds, stake float64) float64 {
	if picked == actualWinner {
		return stake * (odds - 1)
	}
	return -stake
}
