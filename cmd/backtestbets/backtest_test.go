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

	stats1, err := RunBacktest(cfg)
	if err != nil {
		t.Fatalf("RunBacktest: %v", err)
	}
	stats2, err := RunBacktest(cfg)
	if err != nil {
		t.Fatalf("RunBacktest repeat: %v", err)
	}
	if stats1 != stats2 {
		t.Fatalf("non-deterministic: %+v vs %+v", stats1, stats2)
	}
	if stats1.MatchesWalk != 2 {
		t.Fatalf("MatchesWalk = %d, want 2", stats1.MatchesWalk)
	}
	if stats1.Bets+stats1.Skipped != stats1.MatchesWalk {
		t.Fatalf("bets(%d)+skipped(%d) != walked(%d)", stats1.Bets, stats1.Skipped, stats1.MatchesWalk)
	}
	if stats1.Wins+stats1.Losses != stats1.Bets {
		t.Fatalf("wins(%d)+losses(%d) != bets(%d)", stats1.Wins, stats1.Losses, stats1.Bets)
	}
	if stats1.GrossProfit-stats1.GrossLoss != stats1.FinalBalance {
		t.Fatalf("gross_profit(%.2f) - gross_loss(%.2f) != final_balance(%.2f)",
			stats1.GrossProfit, stats1.GrossLoss, stats1.FinalBalance)
	}
}

func TestSettleHistoricalBet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		side, stake, odds float64
		want              float64
	}{
		{side: 1, stake: 1, odds: 2.0, want: 2.0},
		{side: 1, stake: 5, odds: 1.8, want: 9.0},
		{side: 2, stake: 1, odds: 3.0, want: -1},
		{side: 2, stake: 2.5, odds: 3.0, want: -2.5},
	}
	for _, tc := range tests {
		got := settleHistoricalBet(tennisabstract.BetSide(tc.side), tc.odds, tc.stake)
		if got != tc.want {
			t.Errorf("settleHistoricalBet(side=%v, odds=%v, stake=%v) = %v, want %v",
				tc.side, tc.odds, tc.stake, got, tc.want)
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
