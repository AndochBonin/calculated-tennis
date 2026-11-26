package core

import (
	"context"

	"github.com/AndochBonin/calculated-tennis/polymarket/models"
)

// TradeSignal represents a trading decision emitted by a trader.
type TradeSignal struct {
	TokenID        string
	Side           models.OrderSide
	Price          string  // decimal string from the feed
	NegRisk        bool
	WinProbability float64 // model P(outcome wins), (0,1]; required for ProcessSignal
}

// Trader is the interface for a topic-specific trading strategy.
type Trader interface {
	// Start begins market discovery and feed subscriptions.
	Start(ctx context.Context) error
	// Stop gracefully shuts down the trader.
	Stop()
	// Signals returns the channel the trader emits TradeSignals on.
	Signals() <-chan TradeSignal
}
