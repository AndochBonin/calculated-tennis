package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/AndochBonin/E3/tennis/internal/prompt"
)

const (
	defaultStake   = 1.0
	defaultMinPick = 0.5
	defaultSims    = 5000
)

var (
	errInvalidStake   = errors.New("stake must be positive")
	errInvalidMinPick = errors.New("min-pick must be in (0,1]")
	errInvalidSims    = errors.New("sims must be positive")
)

type backtestInputs struct {
	stake   float64
	minPick float64
	sims    int
}

func resolveBacktestInputs(stdin *os.File, stakeFlag, minPickFlag, simsFlag string, interactive func(*os.File) bool) (backtestInputs, error) {
	var br *bufio.Reader
	if interactive(stdin) {
		br = bufio.NewReader(stdin)
	}

	stake, err := resolveStake(stakeFlag, br)
	if err != nil {
		return backtestInputs{}, err
	}
	minPick, err := resolveMinPick(minPickFlag, br)
	if err != nil {
		return backtestInputs{}, err
	}
	sims, err := resolveSims(simsFlag, br)
	if err != nil {
		return backtestInputs{}, err
	}
	return backtestInputs{stake: stake, minPick: minPick, sims: sims}, nil
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
