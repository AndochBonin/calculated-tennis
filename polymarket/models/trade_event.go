package models

// TradeEventStatus represents the status of a trade in a user channel WebSocket event.
type TradeEventStatus string

const (
	TradeEventStatusMatched   TradeEventStatus = "MATCHED"
	TradeEventStatusMined     TradeEventStatus = "MINED"
	TradeEventStatusConfirmed TradeEventStatus = "CONFIRMED"
	TradeEventStatusRetrying  TradeEventStatus = "RETRYING"
	TradeEventStatusFailed    TradeEventStatus = "FAILED"
)

// TradeEventMakerOrder represents a maker order involved in a trade match.
type TradeEventMakerOrder struct {
	OrderID       string    `json:"order_id"`
	Owner         string    `json:"owner"`
	MakerAddress  string    `json:"maker_address"`
	MatchedAmount string    `json:"matched_amount"`
	Price         string    `json:"price"`
	FeeRateBps    string    `json:"fee_rate_bps"`
	AssetID       string    `json:"asset_id"`
	Outcome       string    `json:"outcome"`
	Side          OrderSide `json:"side"`
}

// TradeEvent is a user channel WebSocket message for a matched trade.
type TradeEvent struct {
	EventType       string                 `json:"event_type"`
	Type            string                 `json:"type"`
	ID              string                 `json:"id"`
	TakerOrderID    string                 `json:"taker_order_id"`
	Market          string                 `json:"market"`
	AssetID         string                 `json:"asset_id"`
	Side            OrderSide              `json:"side"`
	Size            string                 `json:"size"`
	Price           string                 `json:"price"`
	FeeRateBps      string                 `json:"fee_rate_bps"`
	Status          TradeEventStatus       `json:"status"`
	MatchTime       string                 `json:"matchtime"`
	LastUpdate      string                 `json:"last_update"`
	Outcome         string                 `json:"outcome"`
	Owner           string                 `json:"owner"`
	TradeOwner      string                 `json:"trade_owner"`
	MakerAddress    string                 `json:"maker_address"`
	TransactionHash string                 `json:"transaction_hash"`
	BucketIndex     int                    `json:"bucket_index"`
	MakerOrders     []TradeEventMakerOrder `json:"maker_orders"`
	TraderSide      TraderSide             `json:"trader_side"`
	Timestamp       string                 `json:"timestamp"`
}
