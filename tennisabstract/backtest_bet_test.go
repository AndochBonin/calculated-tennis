package tennisabstract

import "testing"

func TestDecideBet(t *testing.T) {
	t.Parallel()

	const avgW, avgL = 1.85, 2.10

	cases := []struct {
		name    string
		winsA   int
		sims    int
		minPick float64
		avgW    float64
		avgL    float64
		want    BetSide
		wantOdd float64
		wantOK  bool
	}{
		{
			name: "bet A above 50%", winsA: 2501, sims: 5000, minPick: 0.5,
			avgW: avgW, avgL: avgL, want: BetSideA, wantOdd: avgW, wantOK: true,
		},
		{
			name: "bet B below 50%", winsA: 2499, sims: 5000, minPick: 0.5,
			avgW: avgW, avgL: avgL, want: BetSideB, wantOdd: avgL, wantOK: true,
		},
		{
			name: "skip exact 50/50 at 0.5", winsA: 2500, sims: 5000, minPick: 0.5,
			avgW: avgW, avgL: avgL, want: BetSideNone, wantOK: true,
		},
		{
			name: "bet A at 60% bar", winsA: 3001, sims: 5000, minPick: 0.6,
			avgW: avgW, avgL: avgL, want: BetSideA, wantOdd: avgW, wantOK: true,
		},
		{
			name: "skip middle band at 60%", winsA: 3000, sims: 5000, minPick: 0.6,
			avgW: avgW, avgL: avgL, want: BetSideNone, wantOK: true,
		},
		{
			name: "bet B at 60% bar", winsA: 1999, sims: 5000, minPick: 0.6,
			avgW: avgW, avgL: avgL, want: BetSideB, wantOdd: avgL, wantOK: true,
		},
		{
			name: "all wins A", winsA: 5000, sims: 5000, minPick: 0.5,
			avgW: avgW, avgL: avgL, want: BetSideA, wantOdd: avgW, wantOK: true,
		},
		{
			name: "zero wins A bets B", winsA: 0, sims: 5000, minPick: 0.5,
			avgW: avgW, avgL: avgL, want: BetSideB, wantOdd: avgL, wantOK: true,
		},
		{
			name: "minPick 1 never bets (strict >)", winsA: 5000, sims: 5000, minPick: 1,
			avgW: avgW, avgL: avgL, want: BetSideNone, wantOK: true,
		},
		{
			name: "invalid minPick zero", winsA: 2501, sims: 5000, minPick: 0,
			avgW: avgW, avgL: avgL, wantOK: false,
		},
		{
			name: "invalid minPick above 1", winsA: 2501, sims: 5000, minPick: 1.01,
			avgW: avgW, avgL: avgL, wantOK: false,
		},
		{
			name: "invalid winsA negative", winsA: -1, sims: 5000, minPick: 0.5,
			avgW: avgW, avgL: avgL, wantOK: false,
		},
		{
			name: "invalid winsA over sims", winsA: 5001, sims: 5000, minPick: 0.5,
			avgW: avgW, avgL: avgL, wantOK: false,
		},
		{
			name: "invalid sims zero", winsA: 100, sims: 0, minPick: 0.5,
			avgW: avgW, avgL: avgL, wantOK: false,
		},
		{
			name: "missing avgW on A bet", winsA: 2501, sims: 5000, minPick: 0.5,
			avgW: 0, avgL: avgL, wantOK: false,
		},
		{
			name: "missing avgL on B bet", winsA: 2499, sims: 5000, minPick: 0.5,
			avgW: avgW, avgL: 0, wantOK: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotSide, gotOdd, gotOK := DecideBet(tc.winsA, tc.sims, tc.minPick, tc.avgW, tc.avgL)
			if gotOK != tc.wantOK {
				t.Fatalf("ok = %v, want %v (side=%v odds=%v)", gotOK, tc.wantOK, gotSide, gotOdd)
			}
			if !tc.wantOK {
				return
			}
			if gotSide != tc.want {
				t.Errorf("side = %v, want %v", gotSide, tc.want)
			}
			if gotOdd != tc.wantOdd {
				t.Errorf("odds = %v, want %v", gotOdd, tc.wantOdd)
			}
		})
	}
}

func TestDecideFavoriteBet(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		avgW    float64
		avgL    float64
		want    BetSide
		wantOdd float64
		wantOK  bool
	}{
		{name: "A favorite", avgW: 1.4, avgL: 2.8, want: BetSideA, wantOdd: 1.4, wantOK: true},
		{name: "B favorite", avgW: 2.5, avgL: 1.6, want: BetSideB, wantOdd: 1.6, wantOK: true},
		{name: "tie picks A", avgW: 1.9, avgL: 1.9, want: BetSideA, wantOdd: 1.9, wantOK: true},
		{name: "invalid avgW", avgW: 0, avgL: 2.0, wantOK: false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotSide, gotOdd, gotOK := DecideFavoriteBet(tc.avgW, tc.avgL)
			if gotOK != tc.wantOK {
				t.Fatalf("ok = %v, want %v", gotOK, tc.wantOK)
			}
			if !tc.wantOK {
				return
			}
			if gotSide != tc.want {
				t.Errorf("side = %v, want %v", gotSide, tc.want)
			}
			if gotOdd != tc.wantOdd {
				t.Errorf("odds = %v, want %v", gotOdd, tc.wantOdd)
			}
		})
	}
}
