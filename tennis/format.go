package tennis

// MatchFormat describes set and tiebreak rules. v1 defaults are ATP best-of-3
// with a 7-point tiebreak at 6–6.
type MatchFormat struct {
	SetsToWin           int
	GamesPerSet         int
	GameMargin          int
	TiebreakAtGamesEach int
	TiebreakPointsToWin int
	TiebreakPointMargin int
}

// DefaultFormat returns the v1 rules: best-of-3 sets, first to 6 games by 2,
// 7-point tiebreak at 6–6 (win by 2).
func DefaultFormat() MatchFormat {
	return MatchFormat{
		SetsToWin:           2,
		GamesPerSet:         6,
		GameMargin:          2,
		TiebreakAtGamesEach: 6,
		TiebreakPointsToWin: 7,
		TiebreakPointMargin: 2,
	}
}
