package tennis

import (
	"errors"
	"math/rand/v2"
	"testing"
)

// winsA counts matches won by A.
func winsA(t *testing.T, matches []Match) int {
	t.Helper()
	n := 0
	for _, m := range matches {
		if *m.Winner == A {
			n++
		}
	}
	return n
}

func TestFirstServerCoinToss_deterministicRNG(t *testing.T) {

	rng := rand.New(rand.NewPCG(42, 7))
	var seq []Player
	for range 20 {
		p, err := FirstServerCoinToss(rng)
		if err != nil {
			t.Fatalf("FirstServerCoinToss: %v", err)
		}
		seq = append(seq, p)
	}

	rng2 := rand.New(rand.NewPCG(42, 7))
	for i := range seq {
		p, err := FirstServerCoinToss(rng2)
		if err != nil {
			t.Fatalf("FirstServerCoinToss: %v", err)
		}
		if p != seq[i] {
			t.Fatalf("toss %d = %v, want %v", i, p, seq[i])
		}
	}
}

func TestFirstServerCoinToss_bothSidesAppear(t *testing.T) {

	rng := rand.New(rand.NewPCG(1, 2))
	var sawA, sawB bool
	for range 500 {
		p, err := FirstServerCoinToss(rng)
		if err != nil {
			t.Fatalf("FirstServerCoinToss: %v", err)
		}
		switch p {
		case A:
			sawA = true
		case B:
			sawB = true
		default:
			t.Fatalf("unexpected player %v", p)
		}
	}
	if !sawA || !sawB {
		t.Fatalf("500 tosses: sawA=%v sawB=%v, want both", sawA, sawB)
	}
}

func TestFirstServerCoinToss_nilRNG(t *testing.T) {

	_, err := FirstServerCoinToss(nil)
	if !errors.Is(err, ErrNilRNG) {
		t.Fatalf("FirstServerCoinToss(nil) error = %v, want %v", err, ErrNilRNG)
	}
}

func TestSimulateFreshMatches_deterministicRNG(t *testing.T) {

	rates := [2]PlayerRates{
		{HoldPct: 0.62, BreakPct: 0.38},
		{HoldPct: 0.58, BreakPct: 0.42},
	}
	const alpha = 2
	const n = 200
	format := DefaultFormat()

	run := func(seed1, seed2 uint64) int {
		t.Helper()
		rng := rand.New(rand.NewPCG(seed1, seed2))
		matches, err := SimulateFreshMatches(format, rates, alpha, n, rng)
		if err != nil {
			t.Fatalf("SimulateFreshMatchesMatches: %v", err)
		}
		return winsA(t, matches)
	}

	a := run(42, 7)
	b := run(42, 7)
	if a != b {
		t.Fatalf("same seed: %+v vs %+v", a, b)
	}

	c := run(99, 1)
	if c == a {
		t.Fatalf("different seeds produced identical results %+v", a)
	}
}

func TestSimulateFreshMatches_zeroSimulations(t *testing.T) {

	rng := rand.New(rand.NewPCG(1, 1))
	matches, err := SimulateFreshMatches(DefaultFormat(), [2]PlayerRates{{0.5, 0.5}, {0.5, 0.5}}, 2, 0, rng)
	if err != nil {
		t.Fatalf("SimulateFreshMatchesMatches(n=0): %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("got %d matches, want none", len(matches))
	}
}

func TestSimulateFreshMatches_dominantServerRates(t *testing.T) {

	rates := [2]PlayerRates{
		{HoldPct: 1, BreakPct: 1},
		{HoldPct: 0, BreakPct: 0},
	}
	rng := rand.New(rand.NewPCG(123, 456))
	matches, err := SimulateFreshMatches(DefaultFormat(), rates, 2, 100, rng)
	if err != nil {
		t.Fatalf("SimulateFreshMatchesMatches: %v", err)
	}
	if w := winsA(t, matches); w != 100 {
		t.Fatalf("A wins = %d, want 100 with perfect hold/break asymmetry", w)
	}
}

func TestSimulateFreshMatches_validation(t *testing.T) {

	validRates := [2]PlayerRates{{0.5, 0.5}, {0.5, 0.5}}
	rng := rand.New(rand.NewPCG(1, 1))
	format := DefaultFormat()

	tests := []struct {
		name    string
		rates   [2]PlayerRates
		alpha   float64
		n       int
		rng     *rand.Rand
		wantErr error
	}{
		{name: "negative n", rates: validRates, alpha: 2, n: -1, rng: rng, wantErr: ErrNegativeSimulations},
		{name: "nil rng", rates: validRates, alpha: 2, n: 1, rng: nil, wantErr: ErrNilRNG},
		{name: "alpha zero", rates: validRates, alpha: 0, n: 1, rng: rng, wantErr: ErrInvalidAlpha},
		{name: "invalid hold", rates: [2]PlayerRates{{1.5, 0.5}, {0.5, 0.5}}, alpha: 2, n: 1, rng: rng, wantErr: ErrInvalidRate},
		{name: "invalid break", rates: [2]PlayerRates{{0.5, 0.5}, {0.5, -0.1}}, alpha: 2, n: 1, rng: rng, wantErr: ErrInvalidRate},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := SimulateFreshMatches(format, tt.rates, tt.alpha, tt.n, tt.rng)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("SimulateFreshMatchesMatches() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestApplySimulatedPoint_errors(t *testing.T) {

	t.Run("WinGame match complete", func(t *testing.T) {
		m := NewMatch(A)
		m.Done = true
		err := applySimulatedPoint(m, A)
		if !errors.Is(err, ErrMatchComplete) {
			t.Fatalf("applySimulatedPoint = %v, want %v", err, ErrMatchComplete)
		}
	})

	t.Run("WinTiebreakPoint match complete", func(t *testing.T) {
		m := NewMatch(A)
		gamesToSixSix(t, m)
		m.Done = true
		err := applySimulatedPoint(m, A)
		if !errors.Is(err, ErrMatchComplete) {
			t.Fatalf("applySimulatedPoint = %v, want %v", err, ErrMatchComplete)
		}
	})
}

func TestRunOnce_errors(t *testing.T) {
	rates := [2]PlayerRates{{0.6, 0.4}, {0.6, 0.4}}
	rng := rand.New(rand.NewPCG(1, 2))

	t.Run("invalid alpha", func(t *testing.T) {
		m := NewMatch(A)
		_, err := runOnce(m, rates, -1, rng)
		if !errors.Is(err, ErrInvalidAlpha) {
			t.Fatalf("runOnce = %v, want %v", err, ErrInvalidAlpha)
		}
	})
}

func TestRunOnce_applySimulatedPointError(t *testing.T) {
	rates := [2]PlayerRates{{0.6, 0.4}, {0.6, 0.4}}
	rng := rand.New(rand.NewPCG(1, 2))

	m := NewMatch(A)
	m.Current.Phase = Phase(99)
	_, err := runOnce(m, rates, 2, rng)
	if !errors.Is(err, ErrWrongPhase) {
		t.Fatalf("runOnce = %v, want %v", err, ErrWrongPhase)
	}
}

func TestSimulateFreshMatches_runOnceError(t *testing.T) {
	old := runOnceSim
	runOnceSim = func(*Match, [2]PlayerRates, float64, *rand.Rand) (Player, error) {
		return 0, ErrWrongPhase
	}
	t.Cleanup(func() { runOnceSim = old })

	rng := rand.New(rand.NewPCG(1, 2))
	_, err := SimulateFreshMatches(DefaultFormat(), [2]PlayerRates{{0.5, 0.5}, {0.5, 0.5}}, 2, 1, rng)
	if !errors.Is(err, ErrWrongPhase) {
		t.Fatalf("SimulateFreshMatches = %v, want %v", err, ErrWrongPhase)
	}
}

func TestSimulateFreshMatches_firstServerError(t *testing.T) {
	old := firstServerCoinToss
	firstServerCoinToss = func(*rand.Rand) (Player, error) {
		return 0, ErrNilRNG
	}
	t.Cleanup(func() { firstServerCoinToss = old })

	rng := rand.New(rand.NewPCG(1, 2))
	_, err := SimulateFreshMatches(DefaultFormat(), [2]PlayerRates{{0.5, 0.5}, {0.5, 0.5}}, 2, 1, rng)
	if !errors.Is(err, ErrNilRNG) {
		t.Fatalf("SimulateFreshMatches = %v, want %v", err, ErrNilRNG)
	}
}

func TestSimulateFreshMatches(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	rates := [2]PlayerRates{{HoldPct: 0.8, BreakPct: 0.25}, {HoldPct: 0.78, BreakPct: 0.22}}
	f := GrandSlamMenFormat()
	matches, err := SimulateFreshMatches(f, rates, 1, 50, rng)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 50 {
		t.Fatalf("len = %d, want 50", len(matches))
	}
	sawTB := false
	for i, m := range matches {
		if !m.Done || m.Winner == nil {
			t.Fatalf("match %d not done", i)
		}
		a, b := m.SetsWon()
		if max(a, b) != f.SetsToWin || (a > b) != (*m.Winner == A) {
			t.Fatalf("match %d: sets %d-%d winner %v", i, a, b, *m.Winner)
		}
		if len(m.SetTiebreaks) != len(m.CompletedSets) {
			t.Fatalf("match %d: %d tiebreaks for %d sets", i, len(m.SetTiebreaks), len(m.CompletedSets))
		}
		for j, s := range m.CompletedSets {
			tb := m.SetTiebreaks[j]
			isTB := s.GamesA+s.GamesB == 13
			if isTB != (tb != [2]int{}) {
				t.Fatalf("match %d set %d: games %+v tiebreak %v", i, j, s, tb)
			}
			if isTB {
				sawTB = true
				if (s.GamesA > s.GamesB) != (tb[0] > tb[1]) {
					t.Fatalf("match %d set %d: tiebreak winner disagrees with set winner", i, j)
				}
			}
		}
	}
	if !sawTB {
		t.Fatal("expected at least one tiebreak in 50 close best-of-5 matches")
	}
}
