package models

// PriceEvent is the raw WebSocket message received from the CLOB feed.
type PriceEvent struct {
	EventType    string        `json:"event_type"`
	Market       string        `json:"market"`
	PriceChanges []PriceChange `json:"price_changes"`
	Timestamp    string        `json:"timestamp"`
}

// PriceChange represents a single asset price update within a PriceEvent.
type PriceChange struct {
	AssetID string    `json:"asset_id"`
	Price   string    `json:"price"`
	Size    string    `json:"size"`
	Side    OrderSide `json:"side"`
	Hash    string    `json:"hash"`
	BestBid string    `json:"best_bid"`
	BestAsk string    `json:"best_ask"`
}
