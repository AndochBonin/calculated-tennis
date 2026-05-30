package tennis

import (
	"errors"
	"math/rand/v2"
)

var (
	ErrNilInitialMatch     = errors.New("tennis: initial match is nil")
	ErrInitialMatchDone    = errors.New("tennis: initial match is already complete")
	ErrNegativeSimulations = errors.New("tennis: number of simulations must be non-negative")
	ErrNilRNG              = errors.New("tennis: random number generator is nil")
)

// PlayerRates holds hold and break percentages (fractions in [0,1]) for one player.
type PlayerRates struct {
	HoldPct  float64
	BreakPct float64
}

// SimulationResult aggregates win counts over many simulated matches.
// Wins is indexed by Player (A=0, B=1).
type SimulationResult struct {
	Wins [2]int
}

// WinCount returns how many simulations player p won.
func (r SimulationResult) WinCount(p Player) int {
	return r.Wins[p.index()]
}

func validatePlayerRates(r PlayerRates) error {
	if r.HoldPct < 0 || r.HoldPct > 1 || r.BreakPct < 0 || r.BreakPct > 1 {
		return ErrInvalidRate
	}
	return nil
}

func validateSimulationParams(rates [2]PlayerRates, alpha float64, n int, rng *rand.Rand) error {
	if n < 0 {
		return ErrNegativeSimulations
	}
	if n == 0 {
		return nil
	}
	if alpha <= 0 {
		return ErrInvalidAlpha
	}
	for i := range rates {
		if err := validatePlayerRates(rates[i]); err != nil {
			return err
		}
	}
	if rng == nil {
		return ErrNilRNG
	}
	return nil
}

// firstServerCoinToss and runOnceSim are variables so tests can inject failures.
var (
	firstServerCoinToss = FirstServerCoinToss
	runOnceSim          = runOnce
)

// FirstServerCoinToss returns A or B with equal probability.
func FirstServerCoinToss(rng *rand.Rand) (Player, error) {
	if rng == nil {
		return 0, ErrNilRNG
	}
	if rng.Float64() < 0.5 {
		return A, nil
	}
	return B, nil
}

// SimulateFresh runs n Monte Carlo completions of a new match in format.
// Each simulation coin-tosses the first server, then plays to completion.
func SimulateFresh(format MatchFormat, rates [2]PlayerRates, alpha float64, n int, rng *rand.Rand) (SimulationResult, error) {
	if err := validateSimulationParams(rates, alpha, n, rng); err != nil {
		return SimulationResult{}, err
	}
	if n == 0 {
		return SimulationResult{}, nil
	}

	var result SimulationResult
	for i := 0; i < n; i++ {
		first, err := firstServerCoinToss(rng)
		if err != nil {
			return SimulationResult{}, err
		}
		m := NewMatch(first, format)
		winner, err := runOnceSim(m, rates, alpha, rng)
		if err != nil {
			return SimulationResult{}, err
		}
		result.Wins[winner.index()]++
	}
	return result, nil
}

// Simulate runs n Monte Carlo match completions from initial without mutating initial.
// Each run starts from initial.Clone(). alpha must be positive; rates must be in [0,1].
// n == 0 returns an empty result with no error.
func Simulate(initial *Match, rates [2]PlayerRates, alpha float64, n int, rng *rand.Rand) (SimulationResult, error) {
	if initial == nil {
		return SimulationResult{}, ErrNilInitialMatch
	}
	if initial.Done {
		return SimulationResult{}, ErrInitialMatchDone
	}
	if err := validateSimulationParams(rates, alpha, n, rng); err != nil {
		return SimulationResult{}, err
	}
	if n == 0 {
		return SimulationResult{}, nil
	}

	var result SimulationResult
	for i := 0; i < n; i++ {
		m := initial.Clone()
		winner, err := runOnceSim(&m, rates, alpha, rng)
		if err != nil {
			return SimulationResult{}, err
		}
		result.Wins[winner.index()]++
	}
	return result, nil
}

func applySimulatedPoint(m *Match, winner Player) error {
	if m.Phase() == Regular {
		return m.WinGame(winner)
	}
	return m.WinTiebreakPoint(winner)
}

func runOnce(m *Match, rates [2]PlayerRates, alpha float64, rng *rand.Rand) (Player, error) {
	for !m.Done {
		server := m.Server()
		opp := server.Opponent()
		p, err := GameWinProb(rates[server].HoldPct, rates[opp].BreakPct, alpha)
		if err != nil {
			return 0, err
		}
		winner := server.Opponent()
		if rng.Float64() < p {
			winner = server
		}
		if err := applySimulatedPoint(m, winner); err != nil {
			return 0, err
		}
	}
	return *m.Winner, nil
}
