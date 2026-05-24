package tennis

import (
	"errors"
	"math"
)

var (
	ErrInvalidAlpha = errors.New("tennis: alpha must be positive")
	ErrInvalidRate  = errors.New("tennis: hold and break percentages must be in [0,1]")
)

// GameWinProb returns the probability that the server wins the next game or
// tiebreak point. serverHold is the server's hold percentage and opponentBreak
// is the returner's break percentage (fractions in [0,1]). alpha must be
// positive.
func GameWinProb(serverHold, opponentBreak, alpha float64) (float64, error) {
	if alpha <= 0 {
		return 0, ErrInvalidAlpha
	}
	if serverHold < 0 || serverHold > 1 || opponentBreak < 0 || opponentBreak > 1 {
		return 0, ErrInvalidRate
	}
	if serverHold >= opponentBreak {
		return (1 + math.Pow(serverHold-opponentBreak, alpha)) / 2, nil
	}
	return (1 - math.Pow(opponentBreak-serverHold, alpha)) / 2, nil
}
