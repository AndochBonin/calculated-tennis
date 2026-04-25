package models

import (
	"time"

	"github.com/shopspring/decimal"
)

// GammaMarket is the raw API response from Gamma /markets.
// Used only for deserialization — map to Market for domain use.
type GammaMarket struct {
	ConditionID           string          `json:"conditionId"`
	Question              string          `json:"question"`
	Category              string          `json:"category"`
	EndDate               time.Time       `json:"endDate"`
	Active                bool            `json:"active"`
	Closed                bool            `json:"closed"`
	Archived              bool            `json:"archived"`
	AcceptingOrders       bool            `json:"acceptingOrders"`
	EnableOrderBook       bool            `json:"enableOrderBook"`
	ClobTokenIds          string          `json:"clobTokenIds"`
	OutcomePrices         string          `json:"outcomePrices"`
	Outcomes              string          `json:"outcomes"`
	LiquidityNum          decimal.Decimal `json:"liquidityNum"`
	VolumeNum             decimal.Decimal `json:"volumeNum"`
	LastTradePrice        decimal.Decimal `json:"lastTradePrice"`
	BestBid               decimal.Decimal `json:"bestBid"`
	BestAsk               decimal.Decimal `json:"bestAsk"`
	OrderMinSize          int             `json:"orderMinSize"`
	OrderPriceMinTickSize decimal.Decimal `json:"orderPriceMinTickSize"`
	MakerBaseFee          int             `json:"makerBaseFee"`
	TakerBaseFee          int             `json:"takerBaseFee"`
	GameStartTime         string          `json:"gameStartTime"`
	Tags                  []struct {
		ID    string `json:"id"`
		Label string `json:"label"`
		Slug  string `json:"slug"`
	} `json:"tags"`
}

// Market is the domain model used throughout the engine.
type Market struct {
	ConditionID     string
	Question        string
	Category        string
	Tags            []string
	EndDate         time.Time
	Active          bool
	Closed          bool
	AcceptingOrders bool
	Outcomes        []Outcome
	MinOrderSize    int
	MinTickSize     decimal.Decimal
	MakerFee        int
	TakerFee        int
	GameStartTime   string
}

// Outcome represents a single Yes/No token within a market.
type Outcome struct {
	TokenID string
	Name    string
	Price   decimal.Decimal
}

func MarketFromGamma(g GammaMarket) Market {
	// parse outcomes, token IDs and prices from JSON strings here
	return Market{
		ConditionID:     g.ConditionID,
		Question:        g.Question,
		Category:        g.Category,
		EndDate:         g.EndDate,
		Active:          g.Active,
		Closed:          g.Closed,
		AcceptingOrders: g.AcceptingOrders,
		MinOrderSize:    g.OrderMinSize,
		MinTickSize:     g.OrderPriceMinTickSize,
		MakerFee:        g.MakerBaseFee,
		TakerFee:        g.TakerBaseFee,
		GameStartTime:   g.GameStartTime,
	}
}
