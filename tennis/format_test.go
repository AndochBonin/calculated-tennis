package tennis

import "testing"

func TestDefaultFormat_finalSetTiebreakZero(t *testing.T) {
	t.Parallel()
	f := DefaultFormat()
	if f.FinalSetTiebreakPointsToWin != 0 {
		t.Fatalf("FinalSetTiebreakPointsToWin = %d, want 0", f.FinalSetTiebreakPointsToWin)
	}
	if f.SetsToWin != 2 {
		t.Fatalf("SetsToWin = %d, want 2", f.SetsToWin)
	}
	if f.TiebreakPointsToWin != 7 {
		t.Fatalf("TiebreakPointsToWin = %d, want 7", f.TiebreakPointsToWin)
	}
}

func TestGrandSlamMenFormat(t *testing.T) {
	t.Parallel()
	f := GrandSlamMenFormat()
	if f.SetsToWin != 3 {
		t.Fatalf("SetsToWin = %d, want 3", f.SetsToWin)
	}
	if f.TiebreakPointsToWin != 7 {
		t.Fatalf("TiebreakPointsToWin = %d, want 7", f.TiebreakPointsToWin)
	}
	if f.FinalSetTiebreakPointsToWin != 10 {
		t.Fatalf("FinalSetTiebreakPointsToWin = %d, want 10", f.FinalSetTiebreakPointsToWin)
	}
}

func TestGrandSlamWomenFormat(t *testing.T) {
	t.Parallel()
	f := GrandSlamWomenFormat()
	if f.SetsToWin != 2 {
		t.Fatalf("SetsToWin = %d, want 2", f.SetsToWin)
	}
	if f.TiebreakPointsToWin != 7 {
		t.Fatalf("TiebreakPointsToWin = %d, want 7", f.TiebreakPointsToWin)
	}
	if f.FinalSetTiebreakPointsToWin != 10 {
		t.Fatalf("FinalSetTiebreakPointsToWin = %d, want 10", f.FinalSetTiebreakPointsToWin)
	}
}
