package tennis

import "testing"

func TestNewMatch_defaultFormat(t *testing.T) {

	m := NewMatch(A)
	if m.Format != DefaultFormat() {
		t.Fatalf("Format = %+v, want default %+v", m.Format, DefaultFormat())
	}
	if m.FirstServer != A || m.Server() != A {
		t.Fatalf("FirstServer/Server = %v/%v, want A", m.FirstServer, m.Server())
	}
	if m.Done || m.Winner != nil {
		t.Fatal("new match should not be complete")
	}
}

func TestNewMatch_customFormat(t *testing.T) {

	custom := MatchFormat{
		SetsToWin:           3,
		GamesPerSet:         4,
		GameMargin:          2,
		TiebreakAtGamesEach: 3,
		TiebreakPointsToWin: 5,
		TiebreakPointMargin: 2,
	}
	m := NewMatch(B, custom)
	if m.Format != custom {
		t.Fatalf("Format = %+v, want %+v", m.Format, custom)
	}
	if m.FirstServer != B || m.Server() != B {
		t.Fatalf("FirstServer/Server = %v/%v, want B", m.FirstServer, m.Server())
	}
}

func TestMatch_Clone_partialMatch(t *testing.T) {

	m := NewMatch(A)
	for range 5 {
		if err := m.WinGame(A); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.WinGame(B); err != nil {
		t.Fatal(err)
	}

	c := m.Clone()
	if c.Server() != m.Server() {
		t.Fatalf("Server = %v, want %v", c.Server(), m.Server())
	}
	if c.Phase() != m.Phase() {
		t.Fatalf("Phase = %v, want %v", c.Phase(), m.Phase())
	}
	ga, gb := c.CurrentSetGames()
	wantA, wantB := m.CurrentSetGames()
	if ga != wantA || gb != wantB {
		t.Fatalf("CurrentSetGames = %d-%d, want %d-%d", ga, gb, wantA, wantB)
	}
}

func TestMatch_Clone_isolatedFromMutations(t *testing.T) {

	m := NewMatch(A)
	c := m.Clone()

	if err := c.WinGame(A); err != nil {
		t.Fatal(err)
	}
	a, b := m.CurrentSetGames()
	if a != 0 || b != 0 {
		t.Fatalf("original games = %d-%d after clone mutation, want 0-0", a, b)
	}
}

func TestMatch_Clone_copiesWinner(t *testing.T) {

	fmt := DefaultFormat()
	fmt.SetsToWin = 1
	m := NewMatch(A, fmt)
	mustWinGames(t, m, B, 4)
	mustWinGames(t, m, A, 6)
	if !m.Done || m.Winner == nil || *m.Winner != A {
		t.Fatalf("setup: Done=%v Winner=%v", m.Done, m.Winner)
	}

	c := m.Clone()
	if !c.Done || c.Winner == nil || *c.Winner != A {
		t.Fatalf("clone Done=%v Winner=%v", c.Done, c.Winner)
	}
	if c.Winner == m.Winner {
		t.Fatal("Winner pointer should not be shared")
	}
}

func TestMatch_Clone_completedSetsAndWinner(t *testing.T) {

	fmt := DefaultFormat()
	fmt.SetsToWin = 3

	m := NewMatch(A, fmt)
	mustWinGames(t, m, B, 4)
	mustWinGames(t, m, A, 6)

	c := m.Clone()
	if len(c.CompletedSets) != 1 || len(m.CompletedSets) != 1 {
		t.Fatalf("CompletedSets len = %d/%d, want 1/1", len(c.CompletedSets), len(m.CompletedSets))
	}
	if &c.CompletedSets[0] == &m.CompletedSets[0] {
		t.Fatal("CompletedSets should not share backing array")
	}
	if c.CompletedSets[0] != m.CompletedSets[0] {
		t.Fatalf("CompletedSets[0] = %+v, want %+v", c.CompletedSets[0], m.CompletedSets[0])
	}

	for range 2 {
		mustWinGames(t, &c, B, 4)
		mustWinGames(t, &c, A, 6)
	}
	if !c.Done || c.Winner == nil || *c.Winner != A {
		t.Fatalf("clone Done=%v Winner=%v, want complete with A", c.Done, c.Winner)
	}
	if m.Done || m.Winner != nil {
		t.Fatal("original match should still be in progress")
	}
	if c.Winner == m.Winner {
		t.Fatal("Winner pointer should not be shared")
	}
}
