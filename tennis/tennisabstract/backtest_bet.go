package tennisabstract

// BetSide is which player (if any) to back in a backtest bet.
type BetSide int

const (
	BetSideNone BetSide = iota
	BetSideA
	BetSideB
)

// DecideBet applies the min-pick threshold to simulation win counts.
// Player A is the match winner column; player B is the loser.
//
// With minPick T in (0,1]: bet A when winPctA > T (odds AvgW), bet B when
// winPctB > T i.e. winPctA < 1-T (odds AvgL), otherwise skip.
//
// ok is false when inputs are invalid (sims, winsA, minPick, or odds).
func DecideBet(winsA, sims int, minPick float64, avgW, avgL float64) (side BetSide, odds float64, ok bool) {
	if sims <= 0 || winsA < 0 || winsA > sims {
		return BetSideNone, 0, false
	}
	if minPick <= 0 || minPick > 1 {
		return BetSideNone, 0, false
	}

	winPctA := float64(winsA) / float64(sims)
	if winPctA > minPick {
		if avgW <= 0 {
			return BetSideNone, 0, false
		}
		return BetSideA, avgW, true
	}
	if 1-winPctA > minPick {
		if avgL <= 0 {
			return BetSideNone, 0, false
		}
		return BetSideB, avgL, true
	}
	return BetSideNone, 0, true
}

// DecideFavoriteBet backs the pre-match favorite (lower decimal odds).
// Ties on avgW == avgL resolve to player A. ok is false when either odd is non-positive.
func DecideFavoriteBet(avgW, avgL float64) (side BetSide, odds float64, ok bool) {
	if avgW <= 0 || avgL <= 0 {
		return BetSideNone, 0, false
	}
	if avgL < avgW {
		return BetSideB, avgL, true
	}
	return BetSideA, avgW, true
}
