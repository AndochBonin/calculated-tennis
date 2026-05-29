package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AndochBonin/polymarket/tennisabstract"
)

func TestRunBacktest_smoke(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	matchesPath := filepath.Join(dir, "matches.csv")
	if err := os.WriteFile(matchesPath, []byte(smokeMatchesCSV), 0o644); err != nil {
		t.Fatal(err)
	}
	ratesPath := filepath.Join(dir, "rates.json")
	if err := os.WriteFile(ratesPath, []byte(smokeRatesJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := BacktestConfig{
		MatchesPath: matchesPath,
		RatesPath:   ratesPath,
		Sims:        100,
		Alpha:       1,
		MinPick:     0.5,
		Seed:        42,
	}

	res1, err := RunBacktests(cfg)
	if err != nil {
		t.Fatalf("RunBacktests: %v", err)
	}
	res2, err := RunBacktests(cfg)
	if err != nil {
		t.Fatalf("RunBacktests repeat: %v", err)
	}
	if res1.Sim != res2.Sim {
		t.Fatalf("non-deterministic sim: %+v vs %+v", res1.Sim, res2.Sim)
	}
	if res1.Favorite != res2.Favorite {
		t.Fatalf("non-deterministic favorite: %+v vs %+v", res1.Favorite, res2.Favorite)
	}
	if res1.SimForm != res2.SimForm {
		t.Fatalf("non-deterministic sim-form: %+v vs %+v", res1.SimForm, res2.SimForm)
	}

	stats := res1.Sim
	if stats.MatchesWalk != 2 {
		t.Fatalf("MatchesWalk = %d, want 2", stats.MatchesWalk)
	}
	if stats.Bets+stats.Skipped != stats.MatchesWalk {
		t.Fatalf("bets(%d)+skipped(%d) != walked(%d)", stats.Bets, stats.Skipped, stats.MatchesWalk)
	}
	if stats.Wins+stats.Losses != stats.Bets {
		t.Fatalf("wins(%d)+losses(%d) != bets(%d)", stats.Wins, stats.Losses, stats.Bets)
	}
	if stats.GrossProfit-stats.GrossLoss != stats.FinalBalance {
		t.Fatalf("gross_profit(%.2f) - gross_loss(%.2f) != final_balance(%.2f)",
			stats.GrossProfit, stats.GrossLoss, stats.FinalBalance)
	}

	fav := res1.Favorite
	if fav.MatchesWalk != stats.MatchesWalk {
		t.Fatalf("favorite MatchesWalk = %d, sim = %d", fav.MatchesWalk, stats.MatchesWalk)
	}
	if fav.Skipped != stats.Skipped {
		t.Fatalf("favorite skipped = %d, sim skipped = %d (must match)", fav.Skipped, stats.Skipped)
	}
	if fav.Bets != stats.Bets {
		t.Fatalf("favorite bets = %d, sim bets = %d (must match)", fav.Bets, stats.Bets)
	}

	form := res1.SimForm
	if form.MatchesWalk != stats.MatchesWalk {
		t.Fatalf("sim-form MatchesWalk = %d, sim = %d", form.MatchesWalk, stats.MatchesWalk)
	}
	if form.Bets+form.Skipped != form.MatchesWalk {
		t.Fatalf("sim-form bets(%d)+skipped(%d) != walked(%d)", form.Bets, form.Skipped, form.MatchesWalk)
	}
}

func TestSettleBet(t *testing.T) {
	t.Parallel()

	winner := tennisabstract.BetSideA
	tests := []struct {
		picked, actual tennisabstract.BetSide
		stake, odds    float64
		want           float64
	}{
		{picked: winner, actual: winner, stake: 1, odds: 2.0, want: 2.0},
		{picked: winner, actual: winner, stake: 5, odds: 1.8, want: 9.0},
		{picked: tennisabstract.BetSideB, actual: winner, stake: 1, odds: 3.0, want: -1},
		{picked: tennisabstract.BetSideB, actual: winner, stake: 2.5, odds: 3.0, want: -2.5},
	}
	for _, tc := range tests {
		got := settleBet(tc.picked, tc.actual, tc.odds, tc.stake)
		if got != tc.want {
			t.Errorf("settleBet = %v, want %v", got, tc.want)
		}
	}
}

const smokeMatchesCSV = `tourney_id,tourney_name,surface,draw_size,tourney_level,tourney_date,match_num,winner_id,winner_seed,winner_entry,winner_name,winner_hand,winner_ht,winner_ioc,winner_age,loser_id,loser_seed,loser_entry,loser_name,loser_hand,loser_ht,loser_ioc,loser_age,score,best_of,round,minutes,w_ace,w_df,w_svpt,w_1stIn,w_1stWon,w_2ndWon,w_SvGms,w_bpSaved,w_bpFaced,l_ace,l_df,l_svpt,l_1stIn,l_1stWon,l_2ndWon,l_SvGms,l_bpSaved,l_bpFaced,winner_rank,winner_rank_points,loser_rank,loser_rank_points,AvgW,AvgL
2025-001,Test Open,Hard,32,A,20250101,1,1,,,Alice Strong,R,180,USA,25,2,,,Bob Weak,R,180,USA,26,6-3 6-4,3,R32,90,,,,,,,,,,,,,,,,,,,,,,,2.0,3.0
2025-001,Test Open,Hard,32,A,20250102,1,3,,,Carol Ace,R,175,USA,24,4,,,Dan Base,R,175,USA,27,7-6(5) 6-2,3,R32,100,,,,,,,,,,,,,,,,,,,,,,,1.5,2.5
`

const smokeRatesJSON = `{
  "AliceStrong": {"hold_2024": 0.85, "break_2024": 0.15},
  "BobWeak": {"hold_2024": 0.55, "break_2024": 0.45},
  "CarolAce": {"hold_2024": 0.80, "break_2024": 0.20},
  "DanBase": {"hold_2024": 0.70, "break_2024": 0.30}
}
`
