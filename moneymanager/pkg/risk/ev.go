package risk

import (
	"math"

	"github.com/shopspring/decimal"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ExpectedValueDecimalOdds returns E = winProb*decimalOdds - 1 for $1 staked at decimal odds.
func ExpectedValueDecimalOdds(winProb, decimalOdds float64) float64 {
	return winProb*decimalOdds - 1
}

// ExpectedValueBuyPrice returns E = winProb/price - 1 for a BUY at Polymarket price c (payoff 1/c per $1).
func ExpectedValueBuyPrice(winProb float64, price decimal.Decimal) (decimal.Decimal, error) {
	if !price.IsPositive() {
		return decimal.Zero, status.Error(codes.InvalidArgument, "price must be positive")
	}
	p := decimal.NewFromFloat(winProb)
	return p.Div(price).Sub(decimal.NewFromInt(1)), nil
}

// ValidateWinProbability rejects p outside (0, 1].
func ValidateWinProbability(p float64) error {
	if math.IsNaN(p) || p <= 0 || p > 1 {
		return status.Error(codes.InvalidArgument, "win_probability must be in (0, 1]")
	}
	return nil
}

// ValidatePositiveEV checks win probability and requires strictly positive expected value for BUY.
func ValidatePositiveEV(winProb float64, side Side, price decimal.Decimal) error {
	if err := ValidateWinProbability(winProb); err != nil {
		return err
	}
	switch side {
	case SideSell:
		return status.Error(codes.FailedPrecondition, "sell EV not supported")
	case SideBuy:
		ev, err := ExpectedValueBuyPrice(winProb, price)
		if err != nil {
			return err
		}
		if !ev.IsPositive() {
			return status.Error(codes.FailedPrecondition, "expected value is not positive")
		}
		return nil
	default:
		return status.Errorf(codes.InvalidArgument, "unknown side %q", side)
	}
}
