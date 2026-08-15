package main

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/binary"
	"fmt"
	"math/rand/v2"

	"github.com/AndochBonin/calculated-tennis/tennis/tennis"
	"github.com/AndochBonin/calculated-tennis/tennis/tennisabstract"
	tea "github.com/charmbracelet/bubbletea"
)

// simInputs is the subset of collected form values the simulation needs.
type simInputs struct {
	format            tennis.MatchFormat
	surface           tennisabstract.MatchSurface
	alpha             float64
	sims              int
	score             string
	firstServer       tennis.Player
	firstServerChosen bool
}

// Messages passed back to the Bubble Tea Update loop.
type (
	ratesMsg  struct{ rates [2]tennis.PlayerRates }
	resultMsg struct{ result tennis.SimulationResult }
	errMsg    struct{ err error }
)

// fetchRatesCmd fetches both players' form-adjusted hold/break rates for the
// surface. It runs off the UI goroutine and returns a ratesMsg or errMsg.
func fetchRatesCmd(ctx context.Context, client *tennisabstract.Client, names [2]string, surface tennisabstract.MatchSurface) tea.Cmd {
	return func() tea.Msg {
		formOpts := tennisabstract.FormOptionsFromEnv(surface)
		var rates [2]tennis.PlayerRates
		for i, name := range names {
			stats, err := client.GetPlayerStats(ctx, name)
			if err != nil {
				return errMsg{fmt.Errorf("%s: %w", name, err)}
			}
			adj, err := tennisabstract.AdjustedHoldBreak(stats, formOpts)
			if err != nil {
				return errMsg{fmt.Errorf("%s: %w", name, err)}
			}
			rates[i] = tennis.PlayerRates{HoldPct: adj.HoldPct, BreakPct: adj.BreakPct}
		}
		return ratesMsg{rates}
	}
}

// runSimCmd runs the Monte Carlo projection, dispatching to Simulate (from a
// score or a chosen first server) or SimulateFresh (coin toss each run), the
// same way cmd/simmatch does. Returns a resultMsg or errMsg.
func runSimCmd(in simInputs, rates [2]tennis.PlayerRates) tea.Cmd {
	return func() tea.Msg {
		rng, err := newSeededRNG()
		if err != nil {
			return errMsg{err}
		}
		var result tennis.SimulationResult
		switch {
		case in.score != "":
			initial, e := tennis.MatchFromScore(in.format, in.firstServer, in.score)
			if e != nil {
				return errMsg{e}
			}
			result, err = tennis.Simulate(initial, rates, in.alpha, in.sims, rng)
		case in.firstServerChosen:
			initial := tennis.NewMatch(in.firstServer, in.format)
			result, err = tennis.Simulate(initial, rates, in.alpha, in.sims, rng)
		default:
			result, err = tennis.SimulateFresh(in.format, rates, in.alpha, in.sims, rng)
		}
		if err != nil {
			return errMsg{err}
		}
		return resultMsg{result}
	}
}

// newSeededRNG returns a PCG RNG seeded from crypto/rand (mirrors cmd/simmatch).
func newSeededRNG() (*rand.Rand, error) {
	var buf [16]byte
	if _, err := cryptorand.Read(buf[:]); err != nil {
		return nil, err
	}
	seed1 := binary.LittleEndian.Uint64(buf[0:8])
	seed2 := binary.LittleEndian.Uint64(buf[8:16])
	return rand.New(rand.NewPCG(seed1, seed2)), nil
}
