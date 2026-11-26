package risk

import pkgrisk "github.com/AndochBonin/calculated-tennis/moneymanager/pkg/risk"

type (
	// BalanceReader fetches live USDC collateral for allocation.
	BalanceReader = pkgrisk.BalanceReader
	// Config holds risk limits for signal allocation.
	Config = pkgrisk.Config
	// Allocator applies v1 risk rules and derives order size from live USDC balance.
	Allocator = pkgrisk.Allocator
	// Side is the direction of a trade signal for allocation.
	Side = pkgrisk.Side
)

const (
	SideBuy  = pkgrisk.SideBuy
	SideSell = pkgrisk.SideSell
)

var (
	NewAllocator         = pkgrisk.NewAllocator
	ConfigFromEnv        = pkgrisk.ConfigFromEnv
	SignatureTypeFromEnv = pkgrisk.SignatureTypeFromEnv

	ExpectedValueDecimalOdds = pkgrisk.ExpectedValueDecimalOdds
	ExpectedValueBuyPrice    = pkgrisk.ExpectedValueBuyPrice
	ValidateWinProbability   = pkgrisk.ValidateWinProbability
	ValidatePositiveEV       = pkgrisk.ValidatePositiveEV
)
