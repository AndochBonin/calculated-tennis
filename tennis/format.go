package tennis

// MatchFormat describes set and tiebreak rules. v1 defaults are ATP best-of-3
// with a 7-point tiebreak at 6–6. Grand Slam presets use a longer deciding-set
// tiebreak via FinalSetTiebreakPointsToWin.
type MatchFormat struct {
	SetsToWin           int
	GamesPerSet         int
	GameMargin          int
	TiebreakAtGamesEach int
	TiebreakPointsToWin int
	TiebreakPointMargin int
	// FinalSetTiebreakPointsToWin is the tiebreak target in the deciding set when
	// > 0; otherwise TiebreakPointsToWin applies to every set.
	FinalSetTiebreakPointsToWin int
}

// DefaultFormat returns the v1 rules: best-of-3 sets, first to 6 games by 2,
// 7-point tiebreak at 6–6 (win by 2).
func DefaultFormat() MatchFormat {
	return MatchFormat{
		SetsToWin:                   2,
		GamesPerSet:                 6,
		GameMargin:                  2,
		TiebreakAtGamesEach:         6,
		TiebreakPointsToWin:         7,
		TiebreakPointMargin:         2,
		FinalSetTiebreakPointsToWin: 0,
	}
}

func grandSlamFormat(setsToWin int) MatchFormat {
	return MatchFormat{
		SetsToWin:                   setsToWin,
		GamesPerSet:                 6,
		GameMargin:                  2,
		TiebreakAtGamesEach:         6,
		TiebreakPointsToWin:         7,
		TiebreakPointMargin:         2,
		FinalSetTiebreakPointsToWin: 10,
	}
}

// GrandSlamMenFormat returns best-of-5 Grand Slam rules (2022+): 7-point
// tiebreak at 6–6 in non-deciding sets, 10-point tiebreak in the deciding set.
func GrandSlamMenFormat() MatchFormat { return grandSlamFormat(3) }

// GrandSlamWomenFormat returns best-of-3 Grand Slam rules (2022+): 7-point
// tiebreak at 6–6 in non-deciding sets, 10-point tiebreak in the deciding set.
func GrandSlamWomenFormat() MatchFormat { return grandSlamFormat(2) }
