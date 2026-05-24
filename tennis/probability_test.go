package tennis

import (
	"errors"
	"math"
	"testing"
)

func TestGameWinProb_equalRates(t *testing.T) {
	t.Parallel()
	const alpha = 2
	for _, hold := range []float64{0, 0.5, 0.6, 1} {
		p, err := GameWinProb(hold, hold, alpha)
		if err != nil {
			t.Fatalf("GameWinProb(%v, %v, %v): %v", hold, hold, alpha, err)
		}
		if p != 0.5 {
			t.Fatalf("GameWinProb(%v, %v, %v) = %v, want 0.5", hold, hold, alpha, p)
		}
	}
}

func TestGameWinProb_symmetry(t *testing.T) {
	t.Parallel()
	const alpha = 2
	pHigh, err := GameWinProb(0.8, 0.2, alpha)
	if err != nil {
		t.Fatalf("GameWinProb(0.8, 0.2): %v", err)
	}
	pLow, err := GameWinProb(0.2, 0.8, alpha)
	if err != nil {
		t.Fatalf("GameWinProb(0.2, 0.8): %v", err)
	}
	if math.Abs(pHigh+pLow-1) > 1e-12 {
		t.Fatalf("symmetric pair sums to %v, want 1 (high=%v low=%v)", pHigh+pLow, pHigh, pLow)
	}
	wantHigh := (1 + math.Pow(0.6, alpha)) / 2
	if math.Abs(pHigh-wantHigh) > 1e-12 {
		t.Fatalf("GameWinProb(0.8, 0.2) = %v, want %v", pHigh, wantHigh)
	}
}

func TestGameWinProb_extremes(t *testing.T) {
	t.Parallel()
	const alpha = 2
	p, err := GameWinProb(1, 0, alpha)
	if err != nil {
		t.Fatalf("GameWinProb(1, 0): %v", err)
	}
	if p != 1 {
		t.Fatalf("GameWinProb(1, 0) = %v, want 1", p)
	}
	p, err = GameWinProb(0, 1, alpha)
	if err != nil {
		t.Fatalf("GameWinProb(0, 1): %v", err)
	}
	if p != 0 {
		t.Fatalf("GameWinProb(0, 1) = %v, want 0", p)
	}
}

func TestGameWinProb_validation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		serverHold    float64
		opponentBreak float64
		alpha         float64
		wantErr       error
	}{
		{name: "alpha zero", serverHold: 0.5, opponentBreak: 0.5, alpha: 0, wantErr: ErrInvalidAlpha},
		{name: "alpha negative", serverHold: 0.5, opponentBreak: 0.5, alpha: -1, wantErr: ErrInvalidAlpha},
		{name: "hold below zero", serverHold: -0.1, opponentBreak: 0.5, alpha: 2, wantErr: ErrInvalidRate},
		{name: "hold above one", serverHold: 1.1, opponentBreak: 0.5, alpha: 2, wantErr: ErrInvalidRate},
		{name: "break below zero", serverHold: 0.5, opponentBreak: -0.1, alpha: 2, wantErr: ErrInvalidRate},
		{name: "break above one", serverHold: 0.5, opponentBreak: 1.1, alpha: 2, wantErr: ErrInvalidRate},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := GameWinProb(tt.serverHold, tt.opponentBreak, tt.alpha)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("GameWinProb() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
