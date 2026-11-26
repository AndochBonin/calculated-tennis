package main

import (
	"context"
	"math"
	"testing"

	"github.com/AndochBonin/calculated-tennis/moneymanager/pkg/risk"
	"github.com/AndochBonin/calculated-tennis/tennis/tennisabstract"
)

func TestCombinedParlayProbOdds(t *testing.T) {
	t.Parallel()
	legs := []parlayLeg{
		{winProb: 0.6, odds: 2.0},
		{winProb: 0.5, odds: 1.8},
	}
	p, odds := combinedParlayProbOdds(legs)
	wantP := 0.3
	wantOdds := 3.6
	if math.Abs(p-wantP) > 1e-12 {
		t.Fatalf("p = %v, want %v", p, wantP)
	}
	if math.Abs(odds-wantOdds) > 1e-12 {
		t.Fatalf("odds = %v, want %v", odds, wantOdds)
	}
	if math.Abs(combinedParlayEV(legs)-risk.ExpectedValueDecimalOdds(wantP, wantOdds)) > 1e-12 {
		t.Fatal("combinedParlayEV mismatch")
	}
}

func TestCombinedParlayProbOdds_empty(t *testing.T) {
	t.Parallel()
	p, odds := combinedParlayProbOdds(nil)
	if p != 0 || odds != 0 {
		t.Fatalf("got p=%v odds=%v, want 0,0", p, odds)
	}
}

func TestParlayEVAllowed(t *testing.T) {
	t.Parallel()
	cfg := BacktestConfig{RequirePositiveEV: true}
	if !parlayEVAllowed(cfg, 0.6, 2.0) {
		t.Fatal("expected positive combined EV to pass")
	}
	if parlayEVAllowed(cfg, 0.5, 1.9) {
		t.Fatal("expected negative combined EV to fail")
	}
	cfg.RequirePositiveEV = false
	if !parlayEVAllowed(cfg, 0.5, 1.9) {
		t.Fatal("expected EV filter off to pass")
	}
}

func TestSettleParlay(t *testing.T) {
	t.Parallel()
	winner := tennisabstract.BetSideA
	legs := []parlayLeg{
		{pick: tennisabstract.BetSideA, odds: 2.0, winProb: 0.6, winner: winner},
		{pick: tennisabstract.BetSideA, odds: 1.5, winProb: 0.7, winner: winner},
	}
	stake := 10.0
	got := settleParlay(legs, stake)
	want := stake * (3.0 - 1) // 2.0 * 1.5
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("win pnl = %v, want %v", got, want)
	}
	legs[1].pick = tennisabstract.BetSideB
	if settleParlay(legs, stake) != -stake {
		t.Fatal("expected full stake loss when any leg loses")
	}
}

func TestSelectParlayWindow(t *testing.T) {
	t.Parallel()

	eligible := []tennisabstract.MatchWithOdds{
		{TourneyDate: 20240101, MatchNum: 1, PlayerA: "a1", PlayerB: "b1"},
		{TourneyDate: 20240101, MatchNum: 2, PlayerA: "a2", PlayerB: "b2"},
		{TourneyDate: 20240101, MatchNum: 3, PlayerA: "a3", PlayerB: "b3"},
		{TourneyDate: 20240101, MatchNum: 4, PlayerA: "a4", PlayerB: "b4"},
	}

	leg := func(m tennisabstract.MatchWithOdds, prob, odds float64) parlayLeg {
		return parlayLeg{
			match:   m,
			pick:    tennisabstract.BetSideA,
			odds:    odds,
			winProb: prob,
			winner:  tennisabstract.BetSideA,
		}
	}

	tests := []struct {
		name      string
		anchor    int
		maxParlay int
		requireEV bool
		eval      func(m tennisabstract.MatchWithOdds) (parlayLeg, bool, error)
		wantK     int
		wantOK    bool
	}{
		{
			name:      "longest window when EV off",
			anchor:    0,
			maxParlay: 3,
			eval: func(m tennisabstract.MatchWithOdds) (parlayLeg, bool, error) {
				return leg(m, 0.9, 1.2), true, nil
			},
			wantK:  3,
			wantOK: true,
		},
		{
			name:      "shrink when middle leg missing",
			anchor:    0,
			maxParlay: 3,
			eval: func(m tennisabstract.MatchWithOdds) (parlayLeg, bool, error) {
				if m.MatchNum == 2 {
					return parlayLeg{}, false, nil
				}
				return leg(m, 0.9, 1.2), true, nil
			},
			wantK:  1,
			wantOK: true,
		},
		{
			name:      "reject when combined EV fails",
			anchor:    0,
			maxParlay: 2,
			requireEV: true,
			eval: func(m tennisabstract.MatchWithOdds) (parlayLeg, bool, error) {
				return leg(m, 0.4, 2.0), true, nil
			},
			wantOK: false,
		},
		{
			name:      "accept smaller k when larger fails EV",
			anchor:    0,
			maxParlay: 2,
			requireEV: true,
			eval: func(m tennisabstract.MatchWithOdds) (parlayLeg, bool, error) {
				if m.MatchNum == 2 {
					return leg(m, 0.4, 2.0), true, nil
				}
				return leg(m, 0.7, 1.5), true, nil
			},
			wantK:  1,
			wantOK: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			deps := &parlayEvalDeps{
				cache:       make(map[parlayMatchKey]parlayLeg),
				testEvalLeg: tc.eval,
			}
			cfg := BacktestConfig{RequirePositiveEV: tc.requireEV}
			gotLegs, k, ok, err := selectParlayWindow(tc.anchor, eligible, cfg, tc.maxParlay, deps)
			if err != nil {
				t.Fatalf("selectParlayWindow: %v", err)
			}
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !tc.wantOK {
				return
			}
			if k != tc.wantK {
				t.Fatalf("k = %d, want %d", k, tc.wantK)
			}
			if len(gotLegs) != tc.wantK {
				t.Fatalf("len(legs) = %d, want %d", len(gotLegs), tc.wantK)
			}
		})
	}
}

func TestParlayEvalDeps_cacheDistinctByTourneyID(t *testing.T) {
	t.Parallel()

	brisbane := tennisabstract.MatchWithOdds{TourneyID: "2025-0339", TourneyDate: 20241230, MatchNum: 365, PlayerA: "Jordan Thompson", PlayerB: "Alex Michelsen"}
	hongKong := tennisabstract.MatchWithOdds{TourneyID: "2025-0336", TourneyDate: 20241230, MatchNum: 365, PlayerA: "Miomir Kecmanovic", PlayerB: "Luciano Darderi"}
	if parlayMatchKeyOf(brisbane) == parlayMatchKeyOf(hongKong) {
		t.Fatal("cache keys must differ across tournaments on same date and match_num")
	}

	calls := 0
	deps := &parlayEvalDeps{
		cache: make(map[parlayMatchKey]parlayLeg),
		testEvalLeg: func(m tennisabstract.MatchWithOdds) (parlayLeg, bool, error) {
			calls++
			return parlayLeg{match: m, pick: tennisabstract.BetSideA, odds: 2, winProb: 0.5}, true, nil
		},
	}
	if _, ok, err := deps.evalLeg(brisbane); err != nil || !ok {
		t.Fatalf("eval brisbane: ok=%v err=%v", ok, err)
	}
	if _, ok, err := deps.evalLeg(hongKong); err != nil || !ok {
		t.Fatalf("eval hong kong: ok=%v err=%v", ok, err)
	}
	if calls != 2 {
		t.Fatalf("testEvalLeg calls = %d, want 2 (no cross-tournament cache hit)", calls)
	}
}

func TestBuildParlayLegs_usesCache(t *testing.T) {
	t.Parallel()
	m0 := tennisabstract.MatchWithOdds{TourneyID: "A", TourneyDate: 20240101, MatchNum: 1}
	m1 := tennisabstract.MatchWithOdds{TourneyID: "A", TourneyDate: 20240101, MatchNum: 2}
	eligible := []tennisabstract.MatchWithOdds{m0, m1}
	calls := 0
	deps := &parlayEvalDeps{
		cache: map[parlayMatchKey]parlayLeg{
			parlayMatchKeyOf(m0): {match: m0, pick: tennisabstract.BetSideA, odds: 1.5, winProb: 0.8},
			parlayMatchKeyOf(m1): {match: m1, pick: tennisabstract.BetSideA, odds: 1.5, winProb: 0.8},
		},
		testEvalLeg: func(m tennisabstract.MatchWithOdds) (parlayLeg, bool, error) {
			calls++
			return parlayLeg{}, false, nil
		},
	}
	legs, ok, err := buildParlayLegs(0, 2, eligible, deps)
	if err != nil || !ok || len(legs) != 2 {
		t.Fatalf("buildParlayLegs: ok=%v len=%d err=%v", ok, len(legs), err)
	}
	if calls != 0 {
		t.Fatalf("testEvalLeg calls = %d, want 0 (served from cache)", calls)
	}
}

func TestPlaceParlay_updatesStats(t *testing.T) {
	t.Parallel()
	legs := []parlayLeg{
		{
			match:   tennisabstract.MatchWithOdds{Surface: tennisabstract.SurfaceHard},
			pick:    tennisabstract.BetSideA,
			odds:    2.0,
			winProb: 0.6,
			winner:  tennisabstract.BetSideA,
		},
		{
			match:   tennisabstract.MatchWithOdds{Surface: tennisabstract.SurfaceHard},
			pick:    tennisabstract.BetSideA,
			odds:    1.5,
			winProb: 0.7,
			winner:  tennisabstract.BetSideA,
		},
	}
	var stats BacktestStats
	placeParlay(context.Background(), BacktestConfig{}, nil, nil, legs, &stats, 5)
	if stats.Bets != 1 {
		t.Fatalf("Bets = %d, want 1", stats.Bets)
	}
	if stats.ParlayLegs != 2 {
		t.Fatalf("ParlayLegs = %d, want 2", stats.ParlayLegs)
	}
	if stats.Wins != 1 {
		t.Fatalf("Wins = %d, want 1", stats.Wins)
	}
	wantPnL := 5 * (3.0 - 1)
	if math.Abs(stats.FinalBalance-wantPnL) > 1e-9 {
		t.Fatalf("FinalBalance = %v, want %v", stats.FinalBalance, wantPnL)
	}
}
