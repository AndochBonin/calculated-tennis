package models

import "github.com/shopspring/decimal"

// Position represents a single open position from GET /positions.
type Position struct {
	ProxyWallet        string          `json:"proxyWallet"`
	Asset              string          `json:"asset"`
	ConditionID        string          `json:"conditionId"`
	Size               decimal.Decimal `json:"size"`
	AvgPrice           decimal.Decimal `json:"avgPrice"`
	InitialValue       decimal.Decimal `json:"initialValue"`
	CurrentValue       decimal.Decimal `json:"currentValue"`
	CashPnl            decimal.Decimal `json:"cashPnl"`
	PercentPnl         decimal.Decimal `json:"percentPnl"`
	TotalBought        decimal.Decimal `json:"totalBought"`
	RealizedPnl        decimal.Decimal `json:"realizedPnl"`
	PercentRealizedPnl decimal.Decimal `json:"percentRealizedPnl"`
	CurPrice           decimal.Decimal `json:"curPrice"`
	Redeemable         bool            `json:"redeemable"`
	Mergeable          bool            `json:"mergeable"`
	Title              string          `json:"title"`
	Slug               string          `json:"slug"`
	Icon               string          `json:"icon"`
	EventSlug          string          `json:"eventSlug"`
	Outcome            string          `json:"outcome"`
	OutcomeIndex       int             `json:"outcomeIndex"`
	OppositeOutcome    string          `json:"oppositeOutcome"`
	OppositeAsset      string          `json:"oppositeAsset"`
	EndDate            string          `json:"endDate"`
	NegativeRisk       bool            `json:"negativeRisk"`
}
