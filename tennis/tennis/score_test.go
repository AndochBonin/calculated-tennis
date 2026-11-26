package tennis

import (
	"errors"
	"math/rand/v2"
	"testing"
)

func callMatchFromScoreWithHydrateStub(
	t *testing.T,
	format MatchFormat,
	first Player,
	score string,
	fn func(*Match, SetScore, MatchFormat, bool) error,
) (*Match, error) {
	t.Helper()
	old := hydrateSetFn
	hydrateSetFn = fn
	t.Cleanup(func() { hydrateSetFn = old })
	return MatchFromScore(format, first, score)
}

func callMatchFromScoreWithValidateStub(
	t *testing.T,
	format MatchFormat,
	first Player,
	score string,
	validateFn func(MatchFormat, []SetScore) error,
) (*Match, error) {
	t.Helper()
	old := validateScoreSequenceFn
	validateScoreSequenceFn = validateFn
	t.Cleanup(func() { validateScoreSequenceFn = old })
	return MatchFromScore(format, first, score)
}

func matchFromScoreForTest(t *testing.T, format MatchFormat, first Player, score string) *Match {
	t.Helper()
	m, err := MatchFromScore(format, first, score)
	if err != nil {
		t.Fatalf("MatchFromScore: %v", err)
	}
	return m
}

func matchFromScoreExpect(t *testing.T, format MatchFormat, first Player, score string, want error) {
	t.Helper()
	_, err := MatchFromScore(format, first, score)
	if !errors.Is(err, want) {
		t.Fatalf("MatchFromScore = %v, want %v", err, want)
	}
}

func TestParseScore(t *testing.T) {

	tests := []struct {
		name    string
		score   string
		want    []SetScore
		wantErr error
	}{
		{
			name:  "multi set",
			score: "7-5 4-6 2-3",
			want: []SetScore{
				{GamesA: 7, GamesB: 5},
				{GamesA: 4, GamesB: 6},
				{GamesA: 2, GamesB: 3},
			},
		},
		{
			name:  "in progress tiebreak",
			score: "6-6(3-2)",
			want: []SetScore{
				{GamesA: 6, GamesB: 6, TiebreakA: intPtr(3), TiebreakB: intPtr(2)},
			},
		},
		{
			name:  "bare six all",
			score: "6-6",
			want: []SetScore{
				{GamesA: 6, GamesB: 6},
			},
		},
		{
			name:    "empty",
			score:   "",
			wantErr: ErrScoreEmpty,
		},
		{
			name:    "invalid token",
			score:   "7-5 foo",
			wantErr: ErrScoreInvalidToken,
		},
		{
			name:    "tiebreak on unequal games",
			score:   "5-4(1-0)",
			wantErr: ErrScoreInvalidSet,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseScore(tt.score)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("ParseScore() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseScore() unexpected error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i].GamesA != tt.want[i].GamesA || got[i].GamesB != tt.want[i].GamesB {
					t.Fatalf("[%d] games = %d-%d, want %d-%d",
						i, got[i].GamesA, got[i].GamesB, tt.want[i].GamesA, tt.want[i].GamesB)
				}
				if !intPtrEqual(got[i].TiebreakA, tt.want[i].TiebreakA) ||
					!intPtrEqual(got[i].TiebreakB, tt.want[i].TiebreakB) {
					t.Fatalf("[%d] tiebreak = %v-%v, want %v-%v",
						i, got[i].TiebreakA, got[i].TiebreakB, tt.want[i].TiebreakA, tt.want[i].TiebreakB)
				}
			}
		})
	}
}

func TestValidateScoreSequence(t *testing.T) {
	format := DefaultFormat()

	shortFormat := MatchFormat{
		SetsToWin: 2, GamesPerSet: 4, GameMargin: 2,
		TiebreakAtGamesEach: 3, TiebreakPointsToWin: 7, TiebreakPointMargin: 2,
	}

	tests := []struct {
		name       string
		score      string
		format     MatchFormat
		skipParse  bool
		sets       []SetScore
		wantErr    error
	}{
		{name: "completed and in progress", score: "7-5 4-6 2-3"},
		{name: "one completed set", score: "6-4"},
		{name: "match complete", score: "6-4 6-3"},
		{name: "tiebreak in progress", score: "6-6(3-2)"},
		{name: "bare six all", score: "6-6"},
		{name: "six five in progress", score: "6-5"},
		{name: "completed tiebreak set", score: "7-6"},
		{name: "empty sets", skipParse: true, wantErr: ErrScoreEmpty},
		{
			name:    "illegal completed score",
			score:   "7-4",
			wantErr: ErrScoreInvalidSet,
		},
		{
			name:    "eight six completed",
			score:   "8-6",
			wantErr: ErrScoreInvalidSet,
		},
		{
			name:    "completed set with tiebreak parens",
			score:   "6-4(1-0)",
			wantErr: ErrScoreInvalidSet,
		},
		{
			name:    "completed seven six with tiebreak parens",
			score:   "7-6(1-0)",
			wantErr: ErrScoreInvalidSet,
		},
		{
			name:    "completed six seven with tiebreak parens",
			score:   "6-7(1-0)",
			wantErr: ErrScoreInvalidSet,
		},
		{
			name:    "five five with tiebreak parens",
			score:   "5-5(1-0)",
			wantErr: ErrScoreInvalidSet,
		},
		{
			name:    "in progress tiebreak at wrong games",
			score:   "5-4(1-0)",
			wantErr: ErrScoreInvalidSet,
		},
		{
			name:    "six five as completed middle set",
			score:   "6-5 6-4",
			wantErr: ErrScoreInvalidSet,
		},
		{
			name:    "sets after match complete",
			score:   "6-4 6-3 2-1",
			wantErr: ErrScoreMatchComplete,
		},
		{
			name:    "match already complete extra in progress",
			score:   "6-4 4-6 6-2 1-0",
			wantErr: ErrScoreTooManySets,
		},
		{
			name:    "completed tiebreak with parens",
			score:   "6-6(7-5)",
			wantErr: ErrScoreInvalidSet,
		},
		{
			name:    "tiebreak at wrong threshold",
			score:   "2-2(1-0)",
			format:  shortFormat,
			wantErr: ErrScoreInvalidSet,
		},
		{
			name:    "too many sets bo3",
			score:   "6-4 4-6 6-3 7-5",
			wantErr: ErrScoreTooManySets,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := format
			if tt.format.SetsToWin != 0 {
				f = tt.format
			}
			var sets []SetScore
			var err error
			if tt.skipParse {
				sets = tt.sets
			} else {
				sets, err = ParseScore(tt.score)
				if err != nil {
					if tt.wantErr != nil && errors.Is(err, tt.wantErr) {
						return
					}
					if tt.wantErr != nil {
						t.Fatalf("ParseScore: %v", err)
					}
					t.Fatalf("ParseScore: %v", err)
				}
			}
			err = ValidateScoreSequence(f, sets)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("ValidateScoreSequence() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateScoreSequence() unexpected error: %v", err)
			}
		})
	}

	t.Run("in progress set already complete", func(t *testing.T) {
		err := validateInProgressSet(
			SetScore{GamesA: 6, GamesB: 4},
			DefaultFormat(),
			[2]int{0, 0},
		)
		if !errors.Is(err, ErrScoreInvalidSet) {
			t.Fatalf("error = %v, want %v", err, ErrScoreInvalidSet)
		}
	})

	t.Run("in progress tiebreak at wrong games line", func(t *testing.T) {
		err := validateInProgressSet(
			SetScore{GamesA: 5, GamesB: 4, TiebreakA: intPtr(1), TiebreakB: intPtr(0)},
			DefaultFormat(),
			[2]int{0, 0},
		)
		if !errors.Is(err, ErrScoreInvalidSet) {
			t.Fatalf("error = %v, want %v", err, ErrScoreInvalidSet)
		}
	})

	t.Run("classify error in validate in progress", func(t *testing.T) {
		err := validateInProgressSet(
			SetScore{GamesA: 2, GamesB: 2, TiebreakA: intPtr(1), TiebreakB: intPtr(0)},
			shortFormat,
			[2]int{0, 0},
		)
		if !errors.Is(err, ErrScoreInvalidSet) {
			t.Fatalf("error = %v, want %v", err, ErrScoreInvalidSet)
		}
	})

	t.Run("match already won last set in progress", func(t *testing.T) {
		bo3 := MatchFormat{
			SetsToWin: 2, GamesPerSet: 6, GameMargin: 2,
			TiebreakAtGamesEach: 6, TiebreakPointsToWin: 7, TiebreakPointMargin: 2,
		}
		sets := []SetScore{
			{GamesA: 6, GamesB: 4},
			{GamesA: 6, GamesB: 4},
			{GamesA: 3, GamesB: 2},
		}
		err := ValidateScoreSequence(bo3, sets)
		if !errors.Is(err, ErrScoreMatchComplete) {
			t.Fatalf("error = %v, want %v", err, ErrScoreMatchComplete)
		}
	})

	t.Run("validate game totals tiebreak at unequal games", func(t *testing.T) {
		err := validateGameTotals(
			SetScore{GamesA: 7, GamesB: 6, TiebreakA: intPtr(1), TiebreakB: intPtr(0)},
			format,
		)
		if !errors.Is(err, ErrScoreInvalidSet) {
			t.Fatalf("error = %v, want %v", err, ErrScoreInvalidSet)
		}
	})

	t.Run("classify completed tiebreak set with parens", func(t *testing.T) {
		_, _, err := classifySet(
			SetScore{GamesA: 7, GamesB: 6, TiebreakA: intPtr(1), TiebreakB: intPtr(0)},
			format,
			[2]int{0, 0},
		)
		if !errors.Is(err, ErrScoreInvalidSet) {
			t.Fatalf("7-6(1-0) error = %v, want %v", err, ErrScoreInvalidSet)
		}
		_, _, err = classifySet(
			SetScore{GamesA: 6, GamesB: 7, TiebreakA: intPtr(1), TiebreakB: intPtr(0)},
			format,
			[2]int{0, 0},
		)
		if !errors.Is(err, ErrScoreInvalidSet) {
			t.Fatalf("6-7(1-0) error = %v, want %v", err, ErrScoreInvalidSet)
		}
	})

	t.Run("classify completed regular set with tiebreak parens", func(t *testing.T) {
		_, _, err := classifySet(
			SetScore{GamesA: 6, GamesB: 4, TiebreakA: intPtr(1), TiebreakB: intPtr(0)},
			format,
			[2]int{0, 0},
		)
		if !errors.Is(err, ErrScoreInvalidSet) {
			t.Fatalf("error = %v, want %v", err, ErrScoreInvalidSet)
		}
	})

	t.Run("classify tiebreak at non threshold equal games", func(t *testing.T) {
		_, _, err := classifySet(
			SetScore{GamesA: 5, GamesB: 5, TiebreakA: intPtr(1), TiebreakB: intPtr(0)},
			format,
			[2]int{0, 0},
		)
		if !errors.Is(err, ErrScoreInvalidSet) {
			t.Fatalf("error = %v, want %v", err, ErrScoreInvalidSet)
		}
	})

	t.Run("validate in progress completed tiebreak at six all", func(t *testing.T) {
		err := validateInProgressSet(
			SetScore{GamesA: 6, GamesB: 6, TiebreakA: intPtr(7), TiebreakB: intPtr(5)},
			format,
			[2]int{0, 0},
		)
		if !errors.Is(err, ErrScoreInvalidSet) {
			t.Fatalf("error = %v, want %v", err, ErrScoreInvalidSet)
		}
	})

	t.Run("validate score sequence skip parse paths", func(t *testing.T) {
		cases := []struct {
			name    string
			format  MatchFormat
			sets    []SetScore
			wantErr error
		}{
			{
				name: "completed seven six with tiebreak parens",
				sets: []SetScore{{GamesA: 7, GamesB: 6, TiebreakA: intPtr(1), TiebreakB: intPtr(0)}},
				wantErr: ErrScoreInvalidSet,
			},
			{
				name: "completed six seven with tiebreak parens",
				sets: []SetScore{{GamesA: 6, GamesB: 7, TiebreakA: intPtr(1), TiebreakB: intPtr(0)}},
				wantErr: ErrScoreInvalidSet,
			},
			{
				name: "completed set with tiebreak parens",
				sets: []SetScore{{GamesA: 6, GamesB: 4, TiebreakA: intPtr(1), TiebreakB: intPtr(0)}},
				wantErr: ErrScoreInvalidSet,
			},
			{
				name: "in progress tiebreak at wrong games",
				sets: []SetScore{{GamesA: 5, GamesB: 4, TiebreakA: intPtr(1), TiebreakB: intPtr(0)}},
				wantErr: ErrScoreInvalidSet,
			},
			{
				name: "completed tiebreak with parens",
				sets: []SetScore{{GamesA: 6, GamesB: 6, TiebreakA: intPtr(7), TiebreakB: intPtr(5)}},
				wantErr: ErrScoreInvalidSet,
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				f := format
				if tc.format.SetsToWin != 0 {
					f = tc.format
				}
				err := ValidateScoreSequence(f, tc.sets)
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("ValidateScoreSequence() error = %v, want %v", err, tc.wantErr)
				}
			})
		}
	})
}

func TestValidateScoreSequence_bo5(t *testing.T) {
	format := GrandSlamMenFormat()

	sets, err := ParseScore("6-4 4-6 6-7 6-3 7-6")
	if err != nil {
		t.Fatalf("ParseScore: %v", err)
	}
	if err := ValidateScoreSequence(format, sets); err != nil {
		t.Fatalf("ValidateScoreSequence() = %v", err)
	}
}

func intPtr(v int) *int { return &v }

func intPtrEqual(a, b *int) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func hydrateReference(t *testing.T, first Player, format MatchFormat, score string) *Match {
	t.Helper()
	return matchFromScoreForTest(t, format, first, score)
}

func TestMatchFromScore(t *testing.T) {
	format := DefaultFormat()

	tests := []struct {
		name       string
		first      Player
		score      string
		wantSets   [2]int
		wantGames  [2]int
		wantPhase  Phase
		wantTB     [2]int
		wantDone   bool
		wantResult []SetResult
	}{
		{
			name:       "mid match third set",
			first:      A,
			score:      "7-5 4-6 2-3",
			wantSets:   [2]int{1, 1},
			wantGames:  [2]int{2, 3},
			wantPhase:  Regular,
			wantResult: []SetResult{{7, 5}, {4, 6}},
		},
		{
			name:      "one completed set",
			first:     B,
			score:     "6-4",
			wantSets:  [2]int{1, 0},
			wantGames: [2]int{0, 0},
			wantPhase: Regular,
			wantResult: []SetResult{
				{6, 4},
			},
		},
		{
			name:      "tiebreak in progress",
			first:     A,
			score:     "6-6(3-2)",
			wantPhase: Tiebreak,
			wantGames: [2]int{6, 6},
			wantTB:    [2]int{3, 2},
		},
		{
			name:      "bare six all",
			first:     A,
			score:     "6-6",
			wantPhase: Tiebreak,
			wantGames: [2]int{6, 6},
			wantTB:    [2]int{0, 0},
		},
		{
			name:      "six five in progress",
			first:     A,
			score:     "6-5",
			wantPhase: Regular,
			wantGames: [2]int{6, 5},
		},
		{
			name:       "completed tiebreak set",
			first:      A,
			score:      "7-6",
			wantSets:   [2]int{1, 0},
			wantGames:  [2]int{0, 0},
			wantPhase:  Regular,
			wantResult: []SetResult{{7, 6}},
		},
		{
			name:       "seven five completed",
			first:      B,
			score:      "7-5",
			wantSets:   [2]int{1, 0},
			wantResult: []SetResult{{7, 5}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := hydrateReference(t, tt.first, format, tt.score)

			sa, sb := m.SetsWon()
			if sa != tt.wantSets[0] || sb != tt.wantSets[1] {
				t.Fatalf("sets won = %d-%d, want %d-%d", sa, sb, tt.wantSets[0], tt.wantSets[1])
			}
			ga, gb := m.CurrentSetGames()
			if ga != tt.wantGames[0] || gb != tt.wantGames[1] {
				t.Fatalf("games = %d-%d, want %d-%d", ga, gb, tt.wantGames[0], tt.wantGames[1])
			}
			if m.Phase() != tt.wantPhase {
				t.Fatalf("phase = %v, want %v", m.Phase(), tt.wantPhase)
			}
			tba, tbb := m.TiebreakPoints()
			if tba != tt.wantTB[0] || tbb != tt.wantTB[1] {
				t.Fatalf("tiebreak = %d-%d, want %d-%d", tba, tbb, tt.wantTB[0], tt.wantTB[1])
			}
			if m.Done != tt.wantDone {
				t.Fatalf("Done = %v, want %v", m.Done, tt.wantDone)
			}
			if len(m.CompletedSets) != len(tt.wantResult) {
				t.Fatalf("completed sets = %d, want %d", len(m.CompletedSets), len(tt.wantResult))
			}
			for i, want := range tt.wantResult {
				if m.CompletedSets[i] != want {
					t.Fatalf("set %d = %+v, want %+v", i+1, m.CompletedSets[i], want)
				}
			}
			if m.FirstServer != tt.first {
				t.Fatalf("FirstServer = %v, want %v", m.FirstServer, tt.first)
			}
		})
	}
}

func TestMatchFromScore_serverMatchesManualPath(t *testing.T) {

	t.Run("five four", func(t *testing.T) {
		fromScore, err := MatchFromScore(DefaultFormat(), A, "5-4")
		if err != nil {
			t.Fatalf("MatchFromScore: %v", err)
		}
		manual := NewMatch(A)
		gamesToFiveFour(t, manual, A)
		if manual.Server() != fromScore.Server() {
			t.Fatalf("Server() manual=%v fromScore=%v", manual.Server(), fromScore.Server())
		}
	})

	t.Run("six six tiebreak points", func(t *testing.T) {
		fromScore, err := MatchFromScore(DefaultFormat(), A, "6-6(3-2)")
		if err != nil {
			t.Fatalf("MatchFromScore: %v", err)
		}
		manual := NewMatch(A)
		gamesToSixSix(t, manual)
		replayTiebreakPointsTo(manual, 3, 2)
		if manual.Server() != fromScore.Server() {
			t.Fatalf("Server() manual=%v fromScore=%v", manual.Server(), fromScore.Server())
		}
	})

	t.Run("after completed tiebreak set", func(t *testing.T) {
		fromScore, err := MatchFromScore(DefaultFormat(), B, "7-6 2-1")
		if err != nil {
			t.Fatalf("MatchFromScore: %v", err)
		}
		manual := NewMatch(B)
		gamesToSixSix(t, manual)
		mustWinTBPoints(t, manual, A, 7)
		replayRegularGamesTo(manual, 2, 1)
		if manual.Server() != fromScore.Server() {
			t.Fatalf("Server() manual=%v fromScore=%v", manual.Server(), fromScore.Server())
		}
	})
}

func TestMatchFromScore_matchAlreadyComplete(t *testing.T) {

	_, err := MatchFromScore(DefaultFormat(), A, "6-4 6-3")
	if !errors.Is(err, ErrScoreMatchComplete) {
		t.Fatalf("MatchFromScore() error = %v, want %v", err, ErrScoreMatchComplete)
	}
}

func TestMatchFromScore_SimulateImmutability(t *testing.T) {

	initial, err := MatchFromScore(DefaultFormat(), A, "7-5 4-6 2-3")
	if err != nil {
		t.Fatalf("MatchFromScore: %v", err)
	}
	snap := func() (int, int, int, int, int, int, Phase, Player, bool, int) {
		return matchSnapshot(initial)
	}

	rates := [2]PlayerRates{
		{HoldPct: 0.7, BreakPct: 0.3},
		{HoldPct: 0.65, BreakPct: 0.35},
	}
	rng := rand.New(rand.NewPCG(3, 4))
	result, err := Simulate(initial, rates, 2, 10, rng)
	if err != nil {
		t.Fatalf("Simulate: %v", err)
	}
	if result.WinCount(A)+result.WinCount(B) != 10 {
		t.Fatalf("wins total = %d, want 10", result.WinCount(A)+result.WinCount(B))
	}
	assertMatchUnchanged(t, initial, snap)
}

func TestMatchFromScore_grandSlamDecidingSetTB(t *testing.T) {

	format := GrandSlamMenFormat()
	score := "6-4 4-6 6-7 6-3 6-6(7-5)"
	m, err := MatchFromScore(format, A, score)
	if err != nil {
		t.Fatalf("MatchFromScore: %v", err)
	}
	sa, sb := m.SetsWon()
	if sa != 2 || sb != 2 {
		t.Fatalf("sets won = %d-%d, want 2-2", sa, sb)
	}
	if m.Phase() != Tiebreak {
		t.Fatalf("phase = %v, want Tiebreak", m.Phase())
	}
	tba, tbb := m.TiebreakPoints()
	if tba != 7 || tbb != 5 {
		t.Fatalf("tiebreak = %d-%d, want 7-5", tba, tbb)
	}
}

func TestScoreHydration(t *testing.T) {
	format := DefaultFormat()

	t.Run("pick needed winners", func(t *testing.T) {
		if pickNeededGameWinner(6, 3, 6, 6) != B {
			t.Fatal("pickNeededGameWinner B-only")
		}
		if pickNeededGameWinner(3, 6, 6, 6) != A {
			t.Fatal("pickNeededGameWinner A-only")
		}
		if pickNeededTiebreakWinner(6, 3, 7, 6) != B {
			t.Fatal("pickNeededTiebreakWinner B-only (both need)")
		}
		if pickNeededTiebreakWinner(7, 3, 7, 6) != B {
			t.Fatal("pickNeededTiebreakWinner B-only (A at target)")
		}
		if pickNeededTiebreakWinner(3, 6, 7, 6) != A {
			t.Fatal("pickNeededTiebreakWinner A-only")
		}
		if canonicalGameWinner(2, 2) != A {
			t.Fatal("canonicalGameWinner tie -> A")
		}
		if canonicalGameWinner(3, 1) != B {
			t.Fatal("canonicalGameWinner fewer -> B")
		}
	})

	t.Run("hydrateSet tiebreak path replay error", func(t *testing.T) {
		m := NewMatch(A)
		mustWinGames(t, m, B, 4)
		mustWinGames(t, m, A, 6)
		mustWinGames(t, m, B, 4)
		mustWinGames(t, m, A, 6)
		err := hydrateSet(m, SetScore{GamesA: 7, GamesB: 6}, DefaultFormat(), true)
		if !errors.Is(err, ErrMatchComplete) {
			t.Fatalf("hydrateSet = %v, want %v", err, ErrMatchComplete)
		}
	})

	t.Run("hydrateSet tiebreak path replayRegularGamesTo error", func(t *testing.T) {
		m := NewMatch(A)
		gamesToSixSix(t, m)
		m.Current.Games = [2]int{7, 5}
		err := hydrateSet(m, SetScore{
			GamesA: 6, GamesB: 6,
			TiebreakA: intPtr(3), TiebreakB: intPtr(2),
		}, format, false)
		if !errors.Is(err, ErrScoreHydration) {
			t.Fatalf("hydrateSet = %v, want %v", err, ErrScoreHydration)
		}
	})

	t.Run("hydrateSet on complete match", func(t *testing.T) {
		m := NewMatch(A)
		mustWinGames(t, m, B, 4)
		mustWinGames(t, m, A, 6)
		mustWinGames(t, m, B, 4)
		mustWinGames(t, m, A, 6)
		err := hydrateSet(m, SetScore{GamesA: 6, GamesB: 4}, format, true)
		if !errors.Is(err, ErrMatchComplete) {
			t.Fatalf("hydrateSet = %v, want %v", err, ErrMatchComplete)
		}
	})

	t.Run("replayRegularGamesTo in tiebreak", func(t *testing.T) {
		m := NewMatch(A)
		gamesToSixSix(t, m)
		err := replayRegularGamesTo(m, 5, 4)
		if !errors.Is(err, ErrScoreHydration) {
			t.Fatalf("replayRegularGamesTo = %v, want %v", err, ErrScoreHydration)
		}
	})

	t.Run("replayRegularGamesTo overshoot", func(t *testing.T) {
		m := NewMatch(A)
		if err := replayRegularGamesTo(m, 5, 4); err != nil {
			t.Fatal(err)
		}
		err := replayRegularGamesTo(m, 3, 3)
		if !errors.Is(err, ErrScoreHydration) {
			t.Fatalf("replayRegularGamesTo = %v, want %v", err, ErrScoreHydration)
		}
	})

	t.Run("replayRegularGamesTo WinGame error", func(t *testing.T) {
		m := NewMatch(A)
		m.Done = true
		err := replayRegularGamesTo(m, 1, 0)
		if !errors.Is(err, ErrMatchComplete) {
			t.Fatalf("replayRegularGamesTo = %v, want %v", err, ErrMatchComplete)
		}
	})

	t.Run("replayRegularGamesUntilSetComplete in tiebreak", func(t *testing.T) {
		m := NewMatch(A)
		gamesToSixSix(t, m)
		err := replayRegularGamesUntilSetComplete(m, 7, 6)
		if !errors.Is(err, ErrScoreHydration) {
			t.Fatalf("replayRegularGamesUntilSetComplete = %v, want %v", err, ErrScoreHydration)
		}
	})

	t.Run("replayRegularGamesUntilSetComplete WinGame error", func(t *testing.T) {
		m := NewMatch(A)
		m.Done = true
		err := replayRegularGamesUntilSetComplete(m, 6, 4)
		if !errors.Is(err, ErrMatchComplete) {
			t.Fatalf("replayRegularGamesUntilSetComplete = %v, want %v", err, ErrMatchComplete)
		}
	})

	t.Run("replayTiebreakPointsTo not in tiebreak", func(t *testing.T) {
		m := NewMatch(A)
		err := replayTiebreakPointsTo(m, 3, 2)
		if !errors.Is(err, ErrScoreHydration) {
			t.Fatalf("replayTiebreakPointsTo = %v, want %v", err, ErrScoreHydration)
		}
	})

	t.Run("replayTiebreakPointsTo overshoot", func(t *testing.T) {
		m := NewMatch(A)
		gamesToSixSix(t, m)
		if err := replayTiebreakPointsTo(m, 3, 2); err != nil {
			t.Fatal(err)
		}
		err := replayTiebreakPointsTo(m, 1, 1)
		if !errors.Is(err, ErrScoreHydration) {
			t.Fatalf("replayTiebreakPointsTo = %v, want %v", err, ErrScoreHydration)
		}
	})

	t.Run("replayTiebreakPointsTo WinTiebreakPoint error", func(t *testing.T) {
		m := NewMatch(A)
		gamesToSixSix(t, m)
		m.Done = true
		err := replayTiebreakPointsTo(m, 3, 2)
		if !errors.Is(err, ErrMatchComplete) {
			t.Fatalf("replayTiebreakPointsTo = %v, want %v", err, ErrMatchComplete)
		}
	})

	t.Run("replayTiebreakPointsTo set completes early", func(t *testing.T) {
		m := NewMatch(A)
		gamesToSixSix(t, m)
		m.Current.TiebreakPoints = [2]int{6, 4}
		err := replayTiebreakPointsTo(m, 7, 5)
		if !errors.Is(err, ErrScoreHydration) {
			t.Fatalf("replayTiebreakPointsTo = %v, want %v", err, ErrScoreHydration)
		}
	})

	t.Run("replayTiebreakUntilSetComplete not tiebreak", func(t *testing.T) {
		m := NewMatch(A)
		err := replayTiebreakUntilSetComplete(m, 7, 6)
		if !errors.Is(err, ErrScoreHydration) {
			t.Fatalf("replayTiebreakUntilSetComplete = %v, want %v", err, ErrScoreHydration)
		}
	})

	t.Run("replayTiebreakUntilSetComplete WinTiebreakPoint error", func(t *testing.T) {
		m := NewMatch(A)
		gamesToSixSix(t, m)
		m.Done = true
		err := replayTiebreakUntilSetComplete(m, 7, 6)
		if !errors.Is(err, ErrMatchComplete) {
			t.Fatalf("replayTiebreakUntilSetComplete = %v, want %v", err, ErrMatchComplete)
		}
	})

	t.Run("replayRegularGamesUntilSetComplete wrong final line", func(t *testing.T) {
		noLongTB := MatchFormat{
			SetsToWin: 2, GamesPerSet: 3, GameMargin: 2,
			TiebreakAtGamesEach: 10, TiebreakPointsToWin: 7, TiebreakPointMargin: 2,
		}
		m := NewMatch(A, noLongTB)
		m.Current.Games = [2]int{2, 0}
		err := replayRegularGamesUntilSetComplete(m, 4, 1)
		if !errors.Is(err, ErrScoreHydration) {
			t.Fatalf("replayRegularGamesUntilSetComplete = %v, want %v", err, ErrScoreHydration)
		}
		if len(m.CompletedSets) == 0 {
			t.Fatal("expected set to complete before hydration error")
		}
		last := m.CompletedSets[len(m.CompletedSets)-1]
		if last.GamesA == 4 && last.GamesB == 1 {
			t.Fatalf("completed line %d-%d should differ from target 4-1", last.GamesA, last.GamesB)
		}
	})

	t.Run("assert games mismatch", func(t *testing.T) {
		m := NewMatch(A)
		err := assertInProgressSet(m, SetScore{GamesA: 5, GamesB: 4}, format)
		if !errors.Is(err, ErrScoreHydration) {
			t.Fatalf("error = %v, want %v", err, ErrScoreHydration)
		}
	})

	t.Run("assert phase not tiebreak at six all", func(t *testing.T) {
		m := NewMatch(A)
		m.Current.Games = [2]int{6, 6}
		m.Current.Phase = Regular
		err := assertInProgressSet(m, SetScore{GamesA: 6, GamesB: 6}, format)
		if !errors.Is(err, ErrScoreHydration) {
			t.Fatalf("error = %v, want %v", err, ErrScoreHydration)
		}
	})

	t.Run("assert tiebreak points mismatch", func(t *testing.T) {
		m := matchFromScoreForTest(t, format, A, "6-6(3-2)")
		err := assertInProgressSet(m, SetScore{GamesA: 6, GamesB: 6, TiebreakA: intPtr(4), TiebreakB: intPtr(2)}, format)
		if !errors.Is(err, ErrScoreHydration) {
			t.Fatalf("error = %v, want %v", err, ErrScoreHydration)
		}
	})

	t.Run("assert tiebreak points on regular in progress", func(t *testing.T) {
		m := matchFromScoreForTest(t, format, A, "5-4")
		err := assertInProgressSet(m, SetScore{GamesA: 5, GamesB: 4, TiebreakA: intPtr(1), TiebreakB: intPtr(0)}, format)
		if !errors.Is(err, ErrScoreInvalidSet) {
			t.Fatalf("error = %v, want %v", err, ErrScoreInvalidSet)
		}
	})

	t.Run("assert phase not regular", func(t *testing.T) {
		m := NewMatch(A)
		m.Current.Games = [2]int{5, 4}
		m.Current.Phase = Tiebreak
		err := assertInProgressSet(m, SetScore{GamesA: 5, GamesB: 4}, format)
		if !errors.Is(err, ErrScoreHydration) {
			t.Fatalf("error = %v, want %v", err, ErrScoreHydration)
		}
	})
}

func TestMatchFromScore_errors(t *testing.T) {
	format := DefaultFormat()

	t.Run("completed set with tiebreak parens", func(t *testing.T) {
		matchFromScoreExpect(t, format, A, "6-6(7-5)", ErrScoreInvalidSet)
	})

	t.Run("completed 7-6 with parens", func(t *testing.T) {
		matchFromScoreExpect(t, format, A, "7-6(1-0)", ErrScoreInvalidSet)
	})

	t.Run("parse error", func(t *testing.T) {
		matchFromScoreExpect(t, format, A, "bad score", ErrScoreInvalidToken)
	})

	t.Run("validate error", func(t *testing.T) {
		matchFromScoreExpect(t, format, A, "7-4", ErrScoreInvalidSet)
	})

	t.Run("classifySet error in hydration loop", func(t *testing.T) {
		// Production always validates first; skip validation to exercise the
		// duplicate classifySet guard inside MatchFromScore's loop.
		// (Scores like 7-6(1-0) fail at ParseScore before the loop.)
		_, err := callMatchFromScoreWithValidateStub(t, format, A, "6-6(7-5)",
			func(MatchFormat, []SetScore) error { return nil })
		if !errors.Is(err, ErrScoreInvalidSet) {
			t.Fatalf("MatchFromScore = %v, want %v", err, ErrScoreInvalidSet)
		}
	})

	t.Run("completed set not recorded", func(t *testing.T) {
		_, err := callMatchFromScoreWithHydrateStub(t, DefaultFormat(), A, "6-4",
			func(m *Match, s SetScore, format MatchFormat, complete bool) error {
				if complete {
					return nil
				}
				return hydrateSet(m, s, format, complete)
			})
		if !errors.Is(err, ErrScoreHydration) {
			t.Fatalf("MatchFromScore = %v, want %v", err, ErrScoreHydration)
		}
	})

	t.Run("completed set wrong line", func(t *testing.T) {
		_, err := callMatchFromScoreWithHydrateStub(t, DefaultFormat(), A, "6-4",
			func(m *Match, s SetScore, format MatchFormat, complete bool) error {
				if complete {
					m.CompletedSets = append(m.CompletedSets, SetResult{GamesA: 0, GamesB: 6})
					return nil
				}
				return hydrateSet(m, s, format, complete)
			})
		if !errors.Is(err, ErrScoreHydration) {
			t.Fatalf("MatchFromScore = %v, want %v", err, ErrScoreHydration)
		}
	})

	t.Run("hydrateSetFn error", func(t *testing.T) {
		_, err := callMatchFromScoreWithHydrateStub(t, format, A, "5-4",
			func(*Match, SetScore, MatchFormat, bool) error {
				return ErrScoreHydration
			})
		if !errors.Is(err, ErrScoreHydration) {
			t.Fatalf("MatchFromScore = %v, want %v", err, ErrScoreHydration)
		}
	})

	t.Run("assert in progress fails after hydration", func(t *testing.T) {
		_, err := callMatchFromScoreWithHydrateStub(t, format, A, "6-6(3-2)",
			func(m *Match, s SetScore, format MatchFormat, complete bool) error {
				if err := hydrateSet(m, s, format, complete); err != nil {
					return err
				}
				if !complete {
					m.Current.TiebreakPoints = [2]int{9, 9}
				}
				return nil
			})
		if !errors.Is(err, ErrScoreHydration) {
			t.Fatalf("MatchFromScore = %v, want %v", err, ErrScoreHydration)
		}
	})
}
