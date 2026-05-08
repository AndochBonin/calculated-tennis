package models

// MarketResolvedEvent is a market channel WebSocket message for a resolved market.
type MarketResolvedEvent struct {
	EventType      string   `json:"event_type"`
	ID             string   `json:"id"`
	Market         string   `json:"market"`
	AssetIDs       []string `json:"assets_ids"`
	WinningAssetID string   `json:"winning_asset_id"`
	WinningOutcome string   `json:"winning_outcome"`
	Timestamp      string   `json:"timestamp"`
	Tags           []string `json:"tags"`
}

// NewMarketEvent is a market channel WebSocket message for newly created markets.
// It is deserialization-only and mirrors the wire payload shape.
type NewMarketEvent struct {
	EventType        string   `json:"event_type"`
	ID               string   `json:"id"`
	Slug             string   `json:"slug"`
	SportsMarketType string   `json:"sports_market_type"`
	Market           string   `json:"market"`
	ConditionID      string   `json:"condition_id"`
	AssetIDs         []string `json:"assets_ids"`
	Outcomes         []string `json:"outcomes"`
	Timestamp        string   `json:"timestamp"`
}

// BookEvent is a market channel WebSocket message containing a full order book snapshot.
type BookEvent struct {
	EventType string       `json:"event_type"`
	AssetID   string       `json:"asset_id"`
	Market    string       `json:"market"`
	Bids      []PriceLevel `json:"bids"`
	Asks      []PriceLevel `json:"asks"`
	Timestamp string       `json:"timestamp"`
	Hash      string       `json:"hash"`
}
