package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/AndochBonin/calculated-tennis/tennis/internal/prompt"
)

const (
	defaultStake             = 1.0
	defaultMinPick           = 0.5
	defaultSims              = 5000
	defaultMaxPctBalance     = 0.05
	defaultMinShareSize      = 5.0
	defaultMinOdds           = 0.0
	defaultMaxParlayMatches  = 5
)

var (
	errInvalidStake          = errors.New("stake must be positive")
	errInvalidMinPick        = errors.New("min-pick must be in (0,1]")
	errInvalidSims           = errors.New("sims must be positive")
	errInvalidInitialBalance = errors.New("initial balance must be positive")
	errInvalidMaxPctBalance  = errors.New("max pct balance must be in (0,1]")
	errInvalidMinShareSize   = errors.New("min share size must be positive")
	errInvalidMaxOrderUSDC   = errors.New("max order USDC must be non-negative")
	errInvalidMinOdds            = errors.New("min-odds must be 0 (disabled) or >= 1")
	errInvalidBetMode            = errors.New("bet mode must be singles or parlay")
	errInvalidMaxParlayMatches   = errors.New("max parlay matches must be >= 1 in parlay mode")
)

type backtestInputFlags struct {
	stake              string
	minPick            string
	minOdds            string
	sims               string
	requirePositiveEV  bool
	parlay             bool
	maxParlayMatches   string
	moneyManager       bool
	initialBalance     string
	maxOrderUSDC       string
	maxPctBalance      string
	minShareSize       string
}

type backtestInputs struct {
	stake              float64
	minPick            float64
	minOdds            float64
	sims               int
	requirePositiveEV  bool
	betMode            BetMode
	maxParlayMatches   int
	moneyManager       *MoneyManagerConfig
}

func resolveBacktestInputs(stdin *os.File, flags backtestInputFlags, interactive func(*os.File) bool) (backtestInputs, error) {
	var br *bufio.Reader
	if interactive(stdin) {
		br = bufio.NewReader(stdin)
	}

	stake, err := resolveStake(flags.stake, br)
	if err != nil {
		return backtestInputs{}, err
	}
	minPick, err := resolveMinPick(flags.minPick, br)
	if err != nil {
		return backtestInputs{}, err
	}
	sims, err := resolveSims(flags.sims, br)
	if err != nil {
		return backtestInputs{}, err
	}
	minOdds, err := resolveMinOdds(flags.minOdds, br)
	if err != nil {
		return backtestInputs{}, err
	}
	requireEV, err := resolveRequirePositiveEV(flags.requirePositiveEV, br)
	if err != nil {
		return backtestInputs{}, err
	}
	betMode, err := resolveBetMode(flags.parlay, flags.maxParlayMatches, br)
	if err != nil {
		return backtestInputs{}, err
	}
	maxParlay, err := resolveMaxParlayMatches(betMode, flags.maxParlayMatches, br)
	if err != nil {
		return backtestInputs{}, err
	}

	useMM, err := resolveUseMoneyManager(flags.moneyManager, br)
	if err != nil {
		return backtestInputs{}, err
	}
	mm, err := resolveMoneyManagerConfig(useMM, flags, br)
	if err != nil {
		return backtestInputs{}, err
	}

	return backtestInputs{
		stake:              stake,
		minPick:            minPick,
		minOdds:            minOdds,
		sims:               sims,
		requirePositiveEV:  requireEV,
		betMode:            betMode,
		maxParlayMatches:   maxParlay,
		moneyManager:       mm,
	}, nil
}

func resolveBetMode(parlayFlag bool, maxParlayFlag string, br *bufio.Reader) (BetMode, error) {
	if parlayFlag {
		return BetModeParlay, nil
	}
	if strings.TrimSpace(maxParlayFlag) != "" {
		return BetModeParlay, nil
	}
	if br == nil {
		return BetModeSingles, nil
	}
	raw, err := prompt.ReadLineFrom(os.Stderr, br, "Bet mode: singles or parlay? [singles]: ")
	if err != nil {
		return "", err
	}
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "singles", "single", "s":
		return BetModeSingles, nil
	case "parlay", "parlays", "p":
		return BetModeParlay, nil
	default:
		return "", errInvalidBetMode
	}
}

func resolveMaxParlayMatches(mode BetMode, flagVal string, br *bufio.Reader) (int, error) {
	if mode != BetModeParlay {
		return 0, nil
	}
	raw := strings.TrimSpace(flagVal)
	if raw == "" && br != nil {
		var err error
		raw, err = prompt.ReadLineFrom(os.Stderr, br, fmt.Sprintf("Max legs per parlay [%d]: ", defaultMaxParlayMatches))
		if err != nil {
			return 0, err
		}
	}
	if raw == "" {
		if br != nil {
			return defaultMaxParlayMatches, nil
		}
		return 0, errInvalidMaxParlayMatches
	}
	maxLegs, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", errInvalidMaxParlayMatches, err)
	}
	if maxLegs < 1 {
		return 0, errInvalidMaxParlayMatches
	}
	return maxLegs, nil
}

func resolveRequirePositiveEV(flagSet bool, br *bufio.Reader) (bool, error) {
	if flagSet {
		return true, nil
	}
	if br == nil {
		return false, nil
	}
	raw, err := prompt.ReadLineFrom(os.Stderr, br, "Require positive EV (p×odds−1>0)? [y/N]: ")
	if err != nil {
		return false, err
	}
	return parseYesNo(raw), nil
}

func resolveMinOdds(flagVal string, br *bufio.Reader) (float64, error) {
	raw := strings.TrimSpace(flagVal)
	if raw == "" && br != nil {
		var err error
		raw, err = prompt.ReadLineFrom(os.Stderr, br, "Minimum decimal odds (0 = no minimum) [0]: ")
		if err != nil {
			return 0, err
		}
	}
	if raw == "" {
		return defaultMinOdds, nil
	}
	minOdds, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", errInvalidMinOdds, err)
	}
	if minOdds < 0 || (minOdds > 0 && minOdds < 1) {
		return 0, errInvalidMinOdds
	}
	return minOdds, nil
}

func resolveUseMoneyManager(mmFlag bool, br *bufio.Reader) (bool, error) {
	if mmFlag {
		return true, nil
	}
	if br == nil {
		return false, nil
	}
	raw, err := prompt.ReadLineFrom(os.Stderr, br, "Use money manager? [y/N]: ")
	if err != nil {
		return false, err
	}
	return parseYesNo(raw), nil
}

func parseYesNo(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "y", "yes":
		return true
	default:
		return false
	}
}

func resolveMoneyManagerConfig(useMM bool, flags backtestInputFlags, br *bufio.Reader) (*MoneyManagerConfig, error) {
	if !useMM {
		return nil, nil
	}

	initialBalance, err := resolveInitialBalance(flags.initialBalance, br)
	if err != nil {
		return nil, err
	}
	maxOrderUSDC, err := resolveMaxOrderUSDC(flags.maxOrderUSDC, br)
	if err != nil {
		return nil, err
	}
	maxPctBalance, err := resolveMaxPctBalance(flags.maxPctBalance, br)
	if err != nil {
		return nil, err
	}
	minShareSize, err := resolveMinShareSize(flags.minShareSize, br)
	if err != nil {
		return nil, err
	}

	return &MoneyManagerConfig{
		InitialBalance: initialBalance,
		MaxOrderUSDC:   maxOrderUSDC,
		MaxPctBalance:  maxPctBalance,
		MinShareSize:   minShareSize,
	}, nil
}

func resolveInitialBalance(flagVal string, br *bufio.Reader) (float64, error) {
	raw := strings.TrimSpace(flagVal)
	if raw == "" && br != nil {
		var err error
		raw, err = prompt.ReadLineFrom(os.Stderr, br, "Initial balance (USDC): ")
		if err != nil {
			return 0, err
		}
	}
	if raw == "" {
		return 0, errInvalidInitialBalance
	}
	balance, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", errInvalidInitialBalance, err)
	}
	if balance <= 0 {
		return 0, errInvalidInitialBalance
	}
	return balance, nil
}

func resolveMaxOrderUSDC(flagVal string, br *bufio.Reader) (string, error) {
	raw := strings.TrimSpace(flagVal)
	if raw == "" && br != nil {
		var err error
		raw, err = prompt.ReadLineFrom(os.Stderr, br, "MONEYMANAGER_MAX_ORDER_USDC (Enter = no cap): ")
		if err != nil {
			return "", err
		}
	}
	if raw == "" {
		return "", nil
	}
	cap, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return "", fmt.Errorf("%w: %v", errInvalidMaxOrderUSDC, err)
	}
	if cap < 0 {
		return "", errInvalidMaxOrderUSDC
	}
	return raw, nil
}

func resolveMaxPctBalance(flagVal string, br *bufio.Reader) (float64, error) {
	raw := strings.TrimSpace(flagVal)
	if raw == "" && br != nil {
		var err error
		raw, err = prompt.ReadLineFrom(os.Stderr, br, fmt.Sprintf("MONEYMANAGER_MAX_PCT_BALANCE [%.2f]: ", defaultMaxPctBalance))
		if err != nil {
			return 0, err
		}
	}
	if raw == "" {
		return defaultMaxPctBalance, nil
	}
	maxPct, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", errInvalidMaxPctBalance, err)
	}
	if maxPct <= 0 || maxPct > 1 {
		return 0, errInvalidMaxPctBalance
	}
	return maxPct, nil
}

func resolveMinShareSize(flagVal string, br *bufio.Reader) (float64, error) {
	raw := strings.TrimSpace(flagVal)
	if raw == "" && br != nil {
		var err error
		raw, err = prompt.ReadLineFrom(os.Stderr, br, fmt.Sprintf("MONEYMANAGER_MIN_SHARE_SIZE [%.0f]: ", defaultMinShareSize))
		if err != nil {
			return 0, err
		}
	}
	if raw == "" {
		return defaultMinShareSize, nil
	}
	minSize, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", errInvalidMinShareSize, err)
	}
	if minSize <= 0 {
		return 0, errInvalidMinShareSize
	}
	return minSize, nil
}

func resolveStake(flagVal string, br *bufio.Reader) (float64, error) {
	raw := strings.TrimSpace(flagVal)
	if raw == "" && br != nil {
		var err error
		raw, err = prompt.ReadLineFrom(os.Stderr, br, fmt.Sprintf("Stake amount [%.0f]: ", defaultStake))
		if err != nil {
			return 0, err
		}
	}
	if raw == "" {
		return defaultStake, nil
	}
	stake, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", errInvalidStake, err)
	}
	if stake <= 0 {
		return 0, errInvalidStake
	}
	return stake, nil
}

func resolveMinPick(flagVal string, br *bufio.Reader) (float64, error) {
	raw := strings.TrimSpace(flagVal)
	if raw == "" && br != nil {
		var err error
		raw, err = prompt.ReadLineFrom(os.Stderr, br, fmt.Sprintf("Min pick (0-1, sim win rate to bet) [%.2f]: ", defaultMinPick))
		if err != nil {
			return 0, err
		}
	}
	if raw == "" {
		return defaultMinPick, nil
	}
	minPick, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", errInvalidMinPick, err)
	}
	if minPick <= 0 || minPick > 1 {
		return 0, errInvalidMinPick
	}
	return minPick, nil
}

func resolveSims(flagVal string, br *bufio.Reader) (int, error) {
	raw := strings.TrimSpace(flagVal)
	if raw == "" && br != nil {
		var err error
		raw, err = prompt.ReadLineFrom(os.Stderr, br, fmt.Sprintf("Number of simulations [%d]: ", defaultSims))
		if err != nil {
			return 0, err
		}
	}
	if raw == "" {
		return defaultSims, nil
	}
	sims, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", errInvalidSims, err)
	}
	if sims <= 0 {
		return 0, errInvalidSims
	}
	return sims, nil
}
