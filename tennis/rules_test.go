package tennis

import (
	"errors"
	"fmt"
	"testing"
)

func mustWinGames(t *testing.T, m *Match, winner Player, n int) {
	t.Helper()
	for range n {
		if err := m.WinGame(winner); err != nil {
			t.Fatalf("WinGame(%v): %v", winner, err)
		}
	}
}

func mustWinTBPoints(t *testing.T, m *Match, winner Player, n int) {
	t.Helper()
	for range n {
		if err := m.WinTiebreakPoint(winner); err != nil {
			t.Fatalf("WinTiebreakPoint(%v): %v", winner, err)
		}
	}
}

func gamesToSixFive(t *testing.T, m *Match, leader Player) {
	t.Helper()
	for range 5 {
		if err := m.WinGame(A); err != nil {
			t.Fatalf("WinGame(A): %v", err)
		}
		if err := m.WinGame(B); err != nil {
			t.Fatalf("WinGame(B): %v", err)
		}
	}
	if err := m.WinGame(leader); err != nil {
		t.Fatalf("WinGame(%v) to 6-5: %v", leader, err)
	}
}

func gamesToSixSix(t *testing.T, m *Match) {
	t.Helper()
	gamesToSixFive(t, m, A)
	if err := m.WinGame(B); err != nil {
		t.Fatalf("WinGame(B) to 6-6: %v", err)
	}
	if m.Phase() != Tiebreak {
		t.Fatalf("phase = %v, want Tiebreak", m.Phase())
	}
	a, b := m.CurrentSetGames()
	if a != 6 || b != 6 {
		t.Fatalf("games = %d-%d, want 6-6", a, b)
	}
}

func TestWinGame_setCompletion(t *testing.T) {

	tests := []struct {
		name       string
		first      Player
		play       func(t *testing.T, m *Match)
		wantResult SetResult
		wantSets   [2]int
		wantDone   bool
	}{
		{
			name:  "6-4 A",
			first: A,
			play: func(t *testing.T, m *Match) {
				mustWinGames(t, m, B, 4)
				mustWinGames(t, m, A, 6)
			},
			wantResult: SetResult{GamesA: 6, GamesB: 4},
			wantSets:   [2]int{1, 0},
		},
		{
			name:  "6-2 B",
			first: B,
			play: func(t *testing.T, m *Match) {
				mustWinGames(t, m, A, 2)
				mustWinGames(t, m, B, 6)
			},
			wantResult: SetResult{GamesA: 2, GamesB: 6},
			wantSets:   [2]int{0, 1},
		},
		{
			name:  "7-5 from 6-5",
			first: A,
			play: func(t *testing.T, m *Match) {
				gamesToSixFive(t, m, A)
				if err := m.WinGame(A); err != nil {
					t.Fatalf("WinGame clincher: %v", err)
				}
			},
			wantResult: SetResult{GamesA: 7, GamesB: 5},
			wantSets:   [2]int{1, 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMatch(tt.first)
			tt.play(t, m)

			if len(m.CompletedSets) != 1 {
				t.Fatalf("completed sets = %d, want 1", len(m.CompletedSets))
			}
			got := m.CompletedSets[0]
			if got != tt.wantResult {
				t.Fatalf("set result = %+v, want %+v", got, tt.wantResult)
			}
			a, b := m.SetsWon()
			if a != tt.wantSets[0] || b != tt.wantSets[1] {
				t.Fatalf("sets won = %d-%d, want %d-%d", a, b, tt.wantSets[0], tt.wantSets[1])
			}
			if m.Done != tt.wantDone {
				t.Fatalf("Done = %v, want %v", m.Done, tt.wantDone)
			}
			if m.Phase() != Regular {
				t.Fatalf("phase = %v, want Regular for next set", m.Phase())
			}
		})
	}
}

func TestWinGame_sixSix_entersTiebreak(t *testing.T) {

	m := NewMatch(A)
	gamesToSixSix(t, m)

	if m.Done {
		t.Fatal("match should not be done at 6-6 before tiebreak")
	}
	if len(m.CompletedSets) != 0 {
		t.Fatalf("completed sets = %d, want 0", len(m.CompletedSets))
	}
	a, b := m.TiebreakPoints()
	if a != 0 || b != 0 {
		t.Fatalf("tiebreak points = %d-%d, want 0-0", a, b)
	}
}

func TestWinTiebreakPoint_margins(t *testing.T) {

	tests := []struct {
		name       string
		play       func(t *testing.T, m *Match)
		wantTB     [2]int
		wantDone   bool
		wantResult *SetResult
	}{
		{
			name: "7-0 ends immediately",
			play: func(t *testing.T, m *Match) {
				gamesToSixSix(t, m)
				mustWinTBPoints(t, m, A, 7)
			},
			wantDone:   false,
			wantResult: &SetResult{GamesA: 7, GamesB: 6},
		},
		{
			name: "7-5 ends immediately",
			play: func(t *testing.T, m *Match) {
				gamesToSixSix(t, m)
				mustWinTBPoints(t, m, B, 5)
				mustWinTBPoints(t, m, A, 7)
			},
			wantDone:   false,
			wantResult: &SetResult{GamesA: 7, GamesB: 6},
		},
		{
			name: "6-6 in TB continues",
			play: func(t *testing.T, m *Match) {
				gamesToSixSix(t, m)
				mustWinTBPoints(t, m, A, 6)
				mustWinTBPoints(t, m, B, 6)
			},
			wantTB: [2]int{6, 6},
		},
		{
			name: "7-6 in TB continues",
			play: func(t *testing.T, m *Match) {
				gamesToSixSix(t, m)
				mustWinTBPoints(t, m, A, 6)
				mustWinTBPoints(t, m, B, 6)
				if err := m.WinTiebreakPoint(A); err != nil {
					t.Fatalf("WinTiebreakPoint: %v", err)
				}
			},
			wantTB: [2]int{7, 6},
		},
		{
			name: "8-6 ends",
			play: func(t *testing.T, m *Match) {
				gamesToSixSix(t, m)
				mustWinTBPoints(t, m, A, 6)
				mustWinTBPoints(t, m, B, 6)
				if err := m.WinTiebreakPoint(A); err != nil {
					t.Fatalf("WinTiebreakPoint to 7-6: %v", err)
				}
				if err := m.WinTiebreakPoint(A); err != nil {
					t.Fatalf("WinTiebreakPoint to 8-6: %v", err)
				}
			},
			wantResult: &SetResult{GamesA: 7, GamesB: 6},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMatch(A)
			tt.play(t, m)

			if tt.wantResult != nil {
				if len(m.CompletedSets) != 1 {
					t.Fatalf("completed sets = %d, want 1", len(m.CompletedSets))
				}
				if m.CompletedSets[0] != *tt.wantResult {
					t.Fatalf("set result = %+v, want %+v", m.CompletedSets[0], *tt.wantResult)
				}
				if m.Phase() != Regular {
					t.Fatalf("phase = %v, want Regular", m.Phase())
				}
			} else {
				if len(m.CompletedSets) != 0 {
					t.Fatalf("completed sets = %d, want 0", len(m.CompletedSets))
				}
				a, b := m.TiebreakPoints()
				if a != tt.wantTB[0] || b != tt.wantTB[1] {
					t.Fatalf("tiebreak points = %d-%d, want %d-%d", a, b, tt.wantTB[0], tt.wantTB[1])
				}
				if m.Phase() != Tiebreak {
					t.Fatalf("phase = %v, want Tiebreak", m.Phase())
				}
			}
			if m.Done != tt.wantDone {
				t.Fatalf("Done = %v, want %v", m.Done, tt.wantDone)
			}
		})
	}
}

func TestMatch_BO3(t *testing.T) {

	tests := []struct {
		name         string
		play         func(t *testing.T, m *Match)
		wantSets     [2]int
		wantResults  []SetResult
		wantSetCount int
	}{
		{
			name: "6-0 6-0 in twelve games",
			play: func(t *testing.T, m *Match) {
				mustWinGames(t, m, A, 12)
			},
			wantSets:     [2]int{2, 0},
			wantResults:  []SetResult{{6, 0}, {6, 0}},
			wantSetCount: 2,
		},
		{
			name: "7-6 6-7 7-6 three sets",
			play: func(t *testing.T, m *Match) {
				gamesToSixSix(t, m)
				mustWinTBPoints(t, m, A, 7)
				gamesToSixSix(t, m)
				mustWinTBPoints(t, m, B, 7)
				gamesToSixSix(t, m)
				mustWinTBPoints(t, m, A, 7)
			},
			wantSets:     [2]int{2, 1},
			wantResults:  []SetResult{{7, 6}, {6, 7}, {7, 6}},
			wantSetCount: 3,
		},
		{
			name: "no third set after 2-0",
			play: func(t *testing.T, m *Match) {
				mustWinGames(t, m, A, 12)
				if err := m.WinGame(A); err == nil {
					t.Fatal("WinGame after match complete: want error")
				} else if !errors.Is(err, ErrMatchComplete) {
					t.Fatalf("WinGame error = %v, want ErrMatchComplete", err)
				}
			},
			wantSets:     [2]int{2, 0},
			wantResults:  []SetResult{{6, 0}, {6, 0}},
			wantSetCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMatch(A)
			tt.play(t, m)

			if !m.Done {
				t.Fatal("match should be complete")
			}
			if m.Winner == nil || *m.Winner != A {
				t.Fatalf("winner = %v, want A", m.Winner)
			}
			a, b := m.SetsWon()
			if a != tt.wantSets[0] || b != tt.wantSets[1] {
				t.Fatalf("sets won = %d-%d, want %d-%d", a, b, tt.wantSets[0], tt.wantSets[1])
			}
			if len(m.CompletedSets) != tt.wantSetCount {
				t.Fatalf("completed sets = %d, want %d", len(m.CompletedSets), tt.wantSetCount)
			}
			for i, want := range tt.wantResults {
				if m.CompletedSets[i] != want {
					t.Fatalf("set %d result = %+v, want %+v", i+1, m.CompletedSets[i], want)
				}
			}
		})
	}
}

func TestServerForTBPoint_rotation(t *testing.T) {

	first := A
	tests := []struct {
		pointsPlayed int
		want         Player
	}{
		{0, A}, // point 1
		{1, B}, // point 2
		{2, B}, // point 3
		{3, A}, // point 4
		{4, A}, // point 5
		{5, B}, // point 6
		{7, A}, // point 8
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("after_%d_points", tt.pointsPlayed), func(t *testing.T) {
			got := serverForTBPoint(tt.pointsPlayed, first)
			if got != tt.want {
				t.Fatalf("serverForTBPoint(%d) = %v, want %v", tt.pointsPlayed, got, tt.want)
			}
		})
	}
}

func TestMatch_firstServerAfterTiebreakSet(t *testing.T) {

	tests := []struct {
		name  string
		first Player
		endTB func(t *testing.T, m *Match) Player // completes set via tiebreak; returns TB point-1 server
	}{
		{
			name:  "7-0 odd point count",
			first: A,
			endTB: func(t *testing.T, m *Match) Player {
				gamesToSixSix(t, m)
				first := m.Current.FirstServerOfTiebreak
				mustWinTBPoints(t, m, A, 7)
				return first
			},
		},
		{
			name:  "7-5 even point count",
			first: A,
			endTB: func(t *testing.T, m *Match) Player {
				gamesToSixSix(t, m)
				first := m.Current.FirstServerOfTiebreak
				mustWinTBPoints(t, m, B, 5)
				mustWinTBPoints(t, m, A, 7)
				return first
			},
		},
		{
			name:  "8-6 extended tiebreak",
			first: A,
			endTB: func(t *testing.T, m *Match) Player {
				gamesToSixSix(t, m)
				first := m.Current.FirstServerOfTiebreak
				mustWinTBPoints(t, m, A, 6)
				mustWinTBPoints(t, m, B, 6)
				if err := m.WinTiebreakPoint(A); err != nil {
					t.Fatalf("WinTiebreakPoint to 7-6: %v", err)
				}
				if err := m.WinTiebreakPoint(A); err != nil {
					t.Fatalf("WinTiebreakPoint to 8-6: %v", err)
				}
				return first
			},
		},
		{
			name:  "match first server B",
			first: B,
			endTB: func(t *testing.T, m *Match) Player {
				gamesToSixSix(t, m)
				first := m.Current.FirstServerOfTiebreak
				mustWinTBPoints(t, m, A, 7)
				return first
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMatch(tt.first)
			firstTB := tt.endTB(t, m)

			if m.Done {
				t.Fatal("match should continue after one tiebreak set")
			}
			if m.Phase() != Regular {
				t.Fatalf("phase = %v, want Regular", m.Phase())
			}
			if len(m.CompletedSets) != 1 {
				t.Fatalf("completed sets = %d, want 1", len(m.CompletedSets))
			}

			want := firstTB.Opponent() // receiver of tiebreak point 1 serves game 1 of next set
			if m.Server() != want {
				t.Fatalf("Server() = %v, want %v (opponent of FirstServerOfTiebreak %v)", m.Server(), want, firstTB)
			}
		})
	}
}

func TestMatch_tiebreakServeRotation(t *testing.T) {

	m := NewMatch(A)
	gamesToSixSix(t, m)

	if m.Server() != m.Current.FirstServerOfTiebreak {
		t.Fatalf("Server() = %v, FirstServerOfTiebreak = %v", m.Server(), m.Current.FirstServerOfTiebreak)
	}

	tests := []struct {
		pointsPlayed int
	}{
		{0},
		{1},
		{3},
		{5},
		{7},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("after_%d_points", tt.pointsPlayed), func(t *testing.T) {
			m := NewMatch(A)
			gamesToSixSix(t, m)
			first := m.Current.FirstServerOfTiebreak
			wantNext := serverForTBPoint(tt.pointsPlayed, first)
			// Alternate winners so the score stays close; server only depends on point count.
			for i := range tt.pointsPlayed {
				winner := A
				if i%2 == 1 {
					winner = B
				}
				if err := m.WinTiebreakPoint(winner); err != nil {
					t.Fatalf("WinTiebreakPoint %d: %v", i, err)
				}
			}
			if m.Server() != wantNext {
				t.Fatalf("after %d TB points Server() = %v, want %v", tt.pointsPlayed, m.Server(), wantNext)
			}
		})
	}
}

func reachGrandSlamMenDecidingSet(t *testing.T, m *Match) {
	t.Helper()
	mustWinGames(t, m, A, 6)
	mustWinGames(t, m, B, 6)
	mustWinGames(t, m, A, 6)
	mustWinGames(t, m, B, 6)
	a, b := m.SetsWon()
	if a != 2 || b != 2 {
		t.Fatalf("sets won = %d-%d, want 2-2 before deciding set", a, b)
	}
}

func reachGrandSlamWomenDecidingSet(t *testing.T, m *Match) {
	t.Helper()
	mustWinGames(t, m, A, 6)
	mustWinGames(t, m, B, 6)
	a, b := m.SetsWon()
	if a != 1 || b != 1 {
		t.Fatalf("sets won = %d-%d, want 1-1 before deciding set", a, b)
	}
}

func TestGrandSlam_tiebreakPoints(t *testing.T) {

	tests := []struct {
		name       string
		format     MatchFormat
		setup      func(t *testing.T, m *Match)
		playTB     func(t *testing.T, m *Match)
		wantTB     [2]int // if set still in progress
		wantResult *SetResult
	}{
		{
			name:   "non-deciding set uses 7-point tiebreak",
			format: GrandSlamMenFormat(),
			setup:  func(t *testing.T, m *Match) {},
			playTB: func(t *testing.T, m *Match) {
				gamesToSixSix(t, m)
				mustWinTBPoints(t, m, B, 5)
				mustWinTBPoints(t, m, A, 7)
			},
			wantResult: &SetResult{GamesA: 7, GamesB: 6},
		},
		{
			name:   "men deciding set 7-5 does not end",
			format: GrandSlamMenFormat(),
			setup:  reachGrandSlamMenDecidingSet,
			playTB: func(t *testing.T, m *Match) {
				gamesToSixSix(t, m)
				mustWinTBPoints(t, m, B, 5)
				mustWinTBPoints(t, m, A, 7)
			},
			wantTB: [2]int{7, 5},
		},
		{
			name:   "men deciding set 10-8 ends",
			format: GrandSlamMenFormat(),
			setup:  reachGrandSlamMenDecidingSet,
			playTB: func(t *testing.T, m *Match) {
				gamesToSixSix(t, m)
				mustWinTBPoints(t, m, B, 5)
				mustWinTBPoints(t, m, A, 7)
				mustWinTBPoints(t, m, B, 2)
				mustWinTBPoints(t, m, A, 3)
			},
			wantResult: &SetResult{GamesA: 7, GamesB: 6},
		},
		{
			name:   "women deciding set 7-5 does not end",
			format: GrandSlamWomenFormat(),
			setup:  reachGrandSlamWomenDecidingSet,
			playTB: func(t *testing.T, m *Match) {
				gamesToSixSix(t, m)
				mustWinTBPoints(t, m, B, 5)
				mustWinTBPoints(t, m, A, 7)
			},
			wantTB: [2]int{7, 5},
		},
		{
			name:   "women deciding set 10-8 ends",
			format: GrandSlamWomenFormat(),
			setup:  reachGrandSlamWomenDecidingSet,
			playTB: func(t *testing.T, m *Match) {
				gamesToSixSix(t, m)
				mustWinTBPoints(t, m, B, 5)
				mustWinTBPoints(t, m, A, 7)
				mustWinTBPoints(t, m, B, 2)
				mustWinTBPoints(t, m, A, 3)
			},
			wantResult: &SetResult{GamesA: 7, GamesB: 6},
		},
		{
			name:   "default format still uses 7-point tiebreak in any set",
			format: DefaultFormat(),
			setup:  func(t *testing.T, m *Match) {},
			playTB: func(t *testing.T, m *Match) {
				gamesToSixSix(t, m)
				mustWinTBPoints(t, m, B, 5)
				mustWinTBPoints(t, m, A, 7)
			},
			wantResult: &SetResult{GamesA: 7, GamesB: 6},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMatch(A, tt.format)
			tt.setup(t, m)
			tt.playTB(t, m)

			if tt.wantResult != nil {
				if len(m.CompletedSets) == 0 {
					t.Fatal("expected set to complete")
				}
				got := m.CompletedSets[len(m.CompletedSets)-1]
				if got != *tt.wantResult {
					t.Fatalf("set result = %+v, want %+v", got, *tt.wantResult)
				}
				if m.Phase() != Regular && !m.Done {
					t.Fatalf("phase = %v, want Regular or match done", m.Phase())
				}
			} else {
				if m.Phase() != Tiebreak {
					t.Fatalf("phase = %v, want Tiebreak", m.Phase())
				}
				a, b := m.TiebreakPoints()
				if a != tt.wantTB[0] || b != tt.wantTB[1] {
					t.Fatalf("tiebreak points = %d-%d, want %d-%d", a, b, tt.wantTB[0], tt.wantTB[1])
				}
			}
		})
	}
}

func TestIllegalTransitions(t *testing.T) {

	tests := []struct {
		name string
		run  func(t *testing.T) error
		want error
	}{
		{
			name: "WinGame during tiebreak",
			run: func(t *testing.T) error {
				m := NewMatch(A)
				gamesToSixSix(t, m)
				return m.WinGame(A)
			},
			want: ErrWrongPhase,
		},
		{
			name: "WinTiebreakPoint during regular games",
			run: func(t *testing.T) error {
				m := NewMatch(A)
				return m.WinTiebreakPoint(A)
			},
			want: ErrWrongPhase,
		},
		{
			name: "WinTiebreakPoint after match complete",
			run: func(t *testing.T) error {
				m := NewMatch(A)
				mustWinGames(t, m, A, 12)
				return m.WinTiebreakPoint(A)
			},
			want: ErrMatchComplete,
		},
		{
			name: "WinGame after match complete",
			run: func(t *testing.T) error {
				m := NewMatch(A)
				mustWinGames(t, m, A, 12)
				return m.WinGame(A)
			},
			want: ErrMatchComplete,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run(t)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
		})
	}
}
