package tennis

import "testing"

func TestNewMatch_defaultFormat(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
