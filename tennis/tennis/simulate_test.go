package tennis

import (
	"errors"
	"math/rand/v2"
	"testing"
)

func gamesToFiveFour(t *testing.T, m *Match, leader Player) {
	t.Helper()
	for range 4 {
		if err := m.WinGame(A); err != nil {
			t.Fatalf("WinGame(A): %v", err)
		}
		if err := m.WinGame(B); err != nil {
			t.Fatalf("WinGame(B): %v", err)
		}
	}
	if err := m.WinGame(leader); err != nil {
		t.Fatalf("WinGame(%v) to 5-4: %v", leader, err)
	}
	a, b := m.CurrentSetGames()
	if leader == A {
		if a != 5 || b != 4 {
			t.Fatalf("games = %d-%d, want 5-4 (A leads)", a, b)
		}
	} else if a != 4 || b != 5 {
		t.Fatalf("games = %d-%d, want 4-5 (B leads)", a, b)
	}
	if m.Phase() != Regular {
		t.Fatalf("phase = %v, want Regular", m.Phase())
	}
}

func matchSnapshot(m *Match) (setsA, setsB, gamesA, gamesB, tbA, tbB int, phase Phase, server Player, done bool, completed int) {
	setsA, setsB = m.SetsWon()
	gamesA, gamesB = m.CurrentSetGames()
	tbA, tbB = m.TiebreakPoints()
	phase = m.Phase()
	server = m.Server()
	done = m.Done
	completed = len(m.CompletedSets)
	return
}

func assertMatchUnchanged(t *testing.T, m *Match, before func() (int, int, int, int, int, int, Phase, Player, bool, int)) {
	t.Helper()
	sa, sb, ga, gb, ta, tb, ph, srv, done, nSets := before()
	csa, csb, cga, cgb, cta, ctb, cph, csrv, cdone, cnSets := matchSnapshot(m)
	if sa != csa || sb != csb || ga != cga || gb != cgb || ta != cta || tb != ctb ||
		ph != cph || srv != csrv || done != cdone || nSets != cnSets {
		t.Fatalf("match changed after Simulate: before sets=%d-%d games=%d-%d tb=%d-%d phase=%v server=%v done=%v completed=%d; after sets=%d-%d games=%d-%d tb=%d-%d phase=%v server=%v done=%v completed=%d",
			sa, sb, ga, gb, ta, tb, ph, srv, done, nSets,
			csa, csb, cga, cgb, cta, ctb, cph, csrv, cdone, cnSets)
	}
}

func TestSimulate_deterministicRNG(t *testing.T) {

	rates := [2]PlayerRates{
		{HoldPct: 0.62, BreakPct: 0.38},
		{HoldPct: 0.58, BreakPct: 0.42},
	}
	const alpha = 2
	const n = 200

	run := func(seed1, seed2 uint64) SimulationResult {
		t.Helper()
		initial := NewMatch(A)
		rng := rand.New(rand.NewPCG(seed1, seed2))
		result, err := Simulate(initial, rates, alpha, n, rng)
		if err != nil {
			t.Fatalf("Simulate: %v", err)
		}
		return result
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

func TestSimulate_zeroSimulations(t *testing.T) {

	m := NewMatch(A)
	rng := rand.New(rand.NewPCG(1, 1))
	result, err := Simulate(m, [2]PlayerRates{{0.5, 0.5}, {0.5, 0.5}}, 2, 0, rng)
	if err != nil {
		t.Fatalf("Simulate(n=0): %v", err)
	}
	if result.Wins != [2]int{} {
		t.Fatalf("result = %+v, want empty wins", result)
	}
}

func TestSimulate_completedMatch(t *testing.T) {

	fmt := DefaultFormat()
	fmt.SetsToWin = 1
	m := NewMatch(A, fmt)
	mustWinGames(t, m, B, 4)
	mustWinGames(t, m, A, 6)
	if !m.Done {
		t.Fatal("setup: match should be complete")
	}

	_, err := Simulate(m, [2]PlayerRates{{0.5, 0.5}, {0.5, 0.5}}, 2, 1, rand.New(rand.NewPCG(1, 1)))
	if !errors.Is(err, ErrInitialMatchDone) {
		t.Fatalf("Simulate() error = %v, want %v", err, ErrInitialMatchDone)
	}
}

func TestSimulate_midMatch_fiveFour(t *testing.T) {

	m := NewMatch(A)
	gamesToFiveFour(t, m, A)
	snap := func() (int, int, int, int, int, int, Phase, Player, bool, int) {
		return matchSnapshot(m)
	}

	rates := [2]PlayerRates{
		{HoldPct: 0.7, BreakPct: 0.3},
		{HoldPct: 0.65, BreakPct: 0.35},
	}
	rng := rand.New(rand.NewPCG(11, 22))
	result, err := Simulate(m, rates, 2, 25, rng)
	if err != nil {
		t.Fatalf("Simulate: %v", err)
	}
	if result.WinCount(A)+result.WinCount(B) != 25 {
		t.Fatalf("wins = %d+%d, want 25 total", result.WinCount(A), result.WinCount(B))
	}
	assertMatchUnchanged(t, m, snap)
}

func TestSimulate_midMatch_sixSixTiebreak(t *testing.T) {

	m := NewMatch(A)
	gamesToSixSix(t, m)
	if m.Phase() != Tiebreak {
		t.Fatalf("phase = %v, want Tiebreak", m.Phase())
	}
	snap := func() (int, int, int, int, int, int, Phase, Player, bool, int) {
		return matchSnapshot(m)
	}

	rates := [2]PlayerRates{
		{HoldPct: 0.55, BreakPct: 0.45},
		{HoldPct: 0.55, BreakPct: 0.45},
	}
	rng := rand.New(rand.NewPCG(33, 44))
	result, err := Simulate(m, rates, 2, 15, rng)
	if err != nil {
		t.Fatalf("Simulate: %v", err)
	}
	if result.WinCount(A)+result.WinCount(B) != 15 {
		t.Fatalf("wins = %d+%d, want 15 total", result.WinCount(A), result.WinCount(B))
	}
	assertMatchUnchanged(t, m, snap)
}

func TestSimulate_immutability(t *testing.T) {

	m := NewMatch(B)
	mustWinGames(t, m, A, 3)
	mustWinGames(t, m, B, 2)

	beforeSets := append([]SetResult(nil), m.CompletedSets...)
	snap := func() (int, int, int, int, int, int, Phase, Player, bool, int) {
		return matchSnapshot(m)
	}

	rates := [2]PlayerRates{
		{HoldPct: 0.6, BreakPct: 0.4},
		{HoldPct: 0.6, BreakPct: 0.4},
	}
	rng := rand.New(rand.NewPCG(5, 6))
	if _, err := Simulate(m, rates, 2, 50, rng); err != nil {
		t.Fatalf("Simulate: %v", err)
	}

	assertMatchUnchanged(t, m, snap)
	if len(m.CompletedSets) != len(beforeSets) {
		t.Fatalf("CompletedSets len = %d, want %d", len(m.CompletedSets), len(beforeSets))
	}
	for i := range beforeSets {
		if m.CompletedSets[i] != beforeSets[i] {
			t.Fatalf("CompletedSets[%d] = %+v, want %+v", i, m.CompletedSets[i], beforeSets[i])
		}
	}
}

func TestSimulate_dominantServerRates(t *testing.T) {

	// A always holds; A always breaks B's serve (B hold 0, A break 1).
	rates := [2]PlayerRates{
		{HoldPct: 1, BreakPct: 1},
		{HoldPct: 0, BreakPct: 0},
	}
	m := NewMatch(A)
	rng := rand.New(rand.NewPCG(123, 456))
	result, err := Simulate(m, rates, 2, 100, rng)
	if err != nil {
		t.Fatalf("Simulate: %v", err)
	}
	if result.WinCount(A) != 100 {
		t.Fatalf("A wins = %d, want 100 with perfect hold/break asymmetry", result.WinCount(A))
	}
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

func TestSimulateFresh_deterministicRNG(t *testing.T) {

	rates := [2]PlayerRates{
		{HoldPct: 0.62, BreakPct: 0.38},
		{HoldPct: 0.58, BreakPct: 0.42},
	}
	const alpha = 2
	const n = 200
	format := DefaultFormat()

	run := func(seed1, seed2 uint64) SimulationResult {
		t.Helper()
		rng := rand.New(rand.NewPCG(seed1, seed2))
		result, err := SimulateFresh(format, rates, alpha, n, rng)
		if err != nil {
			t.Fatalf("SimulateFresh: %v", err)
		}
		return result
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

func TestSimulateFresh_zeroSimulations(t *testing.T) {

	rng := rand.New(rand.NewPCG(1, 1))
	result, err := SimulateFresh(DefaultFormat(), [2]PlayerRates{{0.5, 0.5}, {0.5, 0.5}}, 2, 0, rng)
	if err != nil {
		t.Fatalf("SimulateFresh(n=0): %v", err)
	}
	if result.Wins != [2]int{} {
		t.Fatalf("result = %+v, want empty wins", result)
	}
}

func TestSimulateFresh_dominantServerRates(t *testing.T) {

	rates := [2]PlayerRates{
		{HoldPct: 1, BreakPct: 1},
		{HoldPct: 0, BreakPct: 0},
	}
	rng := rand.New(rand.NewPCG(123, 456))
	result, err := SimulateFresh(DefaultFormat(), rates, 2, 100, rng)
	if err != nil {
		t.Fatalf("SimulateFresh: %v", err)
	}
	if result.WinCount(A) != 100 {
		t.Fatalf("A wins = %d, want 100 with perfect hold/break asymmetry", result.WinCount(A))
	}
}

func TestSimulateFresh_validation(t *testing.T) {

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
			_, err := SimulateFresh(format, tt.rates, tt.alpha, tt.n, tt.rng)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("SimulateFresh() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestSimulate_validation(t *testing.T) {

	validRates := [2]PlayerRates{{0.5, 0.5}, {0.5, 0.5}}
	rng := rand.New(rand.NewPCG(1, 1))
	m := NewMatch(A)

	tests := []struct {
		name    string
		initial *Match
		rates   [2]PlayerRates
		alpha   float64
		n       int
		rng     *rand.Rand
		wantErr error
	}{
		{name: "nil initial", initial: nil, rates: validRates, alpha: 2, n: 1, rng: rng, wantErr: ErrNilInitialMatch},
		{name: "negative n", initial: m, rates: validRates, alpha: 2, n: -1, rng: rng, wantErr: ErrNegativeSimulations},
		{name: "nil rng", initial: m, rates: validRates, alpha: 2, n: 1, rng: nil, wantErr: ErrNilRNG},
		{name: "alpha zero", initial: m, rates: validRates, alpha: 0, n: 1, rng: rng, wantErr: ErrInvalidAlpha},
		{name: "invalid hold", initial: m, rates: [2]PlayerRates{{1.5, 0.5}, {0.5, 0.5}}, alpha: 2, n: 1, rng: rng, wantErr: ErrInvalidRate},
		{name: "invalid break", initial: m, rates: [2]PlayerRates{{0.5, 0.5}, {0.5, -0.1}}, alpha: 2, n: 1, rng: rng, wantErr: ErrInvalidRate},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Simulate(tt.initial, tt.rates, tt.alpha, tt.n, tt.rng)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Simulate() error = %v, want %v", err, tt.wantErr)
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

func TestSimulate_runOnceErrorPropagation(t *testing.T) {
	old := runOnceSim
	runOnceSim = func(*Match, [2]PlayerRates, float64, *rand.Rand) (Player, error) {
		return 0, ErrWrongPhase
	}
	t.Cleanup(func() { runOnceSim = old })

	m := NewMatch(A)
	rng := rand.New(rand.NewPCG(1, 2))
	_, err := Simulate(m, [2]PlayerRates{{0.5, 0.5}, {0.5, 0.5}}, 2, 1, rng)
	if !errors.Is(err, ErrWrongPhase) {
		t.Fatalf("Simulate = %v, want %v", err, ErrWrongPhase)
	}
}

func TestSimulateFresh_runOnceError(t *testing.T) {
	old := runOnceSim
	runOnceSim = func(*Match, [2]PlayerRates, float64, *rand.Rand) (Player, error) {
		return 0, ErrWrongPhase
	}
	t.Cleanup(func() { runOnceSim = old })

	rng := rand.New(rand.NewPCG(1, 2))
	_, err := SimulateFresh(DefaultFormat(), [2]PlayerRates{{0.5, 0.5}, {0.5, 0.5}}, 2, 1, rng)
	if !errors.Is(err, ErrWrongPhase) {
		t.Fatalf("SimulateFresh = %v, want %v", err, ErrWrongPhase)
	}
}

func TestSimulateFresh_firstServerError(t *testing.T) {
	old := firstServerCoinToss
	firstServerCoinToss = func(*rand.Rand) (Player, error) {
		return 0, ErrNilRNG
	}
	t.Cleanup(func() { firstServerCoinToss = old })

	rng := rand.New(rand.NewPCG(1, 2))
	_, err := SimulateFresh(DefaultFormat(), [2]PlayerRates{{0.5, 0.5}, {0.5, 0.5}}, 2, 1, rng)
	if !errors.Is(err, ErrNilRNG) {
		t.Fatalf("SimulateFresh = %v, want %v", err, ErrNilRNG)
	}
}
