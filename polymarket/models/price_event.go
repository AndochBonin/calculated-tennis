package models

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// PriceEvent is the raw WebSocket message received from the CLOB feed.

type ClobMarketPrice struct {
	Price decimal.Decimal `json:"price"`
}

type PriceEvent struct {
	EventType    string        `json:"event_type"`
	Market       string        `json:"market"`
	PriceChanges []PriceChange `json:"price_changes"`
	Timestamp    string        `json:"timestamp"`
}

// PriceChange represents a single asset price update within a PriceEvent.
type PriceChange struct {
	AssetID string    `json:"asset_id"`
	Price   string    `json:"price"` // price comes as a string here
	Size    string    `json:"size"`
	Side    OrderSide `json:"side"`
	Hash    string    `json:"hash"`
}

func ClobPriceToPriceEvent(clobPrice ClobMarketPrice, marketID Market, tokenID string, side OrderSide) *PriceEvent {
	priceChange := PriceChange{
		AssetID: tokenID,
		Price:   fmt.Sprintf("%d", clobPrice.Price),
		Side:    side,
	}
	
	return &PriceEvent{
		EventType: "price_change",
		Market:    fmt.Sprintf("%v", marketID),
		PriceChanges: []PriceChange{priceChange},
		Timestamp: time.Now().String(),
	}
}
