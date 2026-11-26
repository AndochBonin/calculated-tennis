package models

// OrderEventType represents the type of order lifecycle event.
type OrderEventType string

const (
	OrderEventTypePlacement    OrderEventType = "PLACEMENT"
	OrderEventTypeUpdate       OrderEventType = "UPDATE"
	OrderEventTypeCancellation OrderEventType = "CANCELLATION"
)

// OrderEvent is a user channel WebSocket message for order lifecycle changes.
type OrderEvent struct {
	EventType       string         `json:"event_type"`
	ID              string         `json:"id"`
	Owner           string         `json:"owner"`
	Market          string         `json:"market"`
	AssetID         string         `json:"asset_id"`
	Side            OrderSide      `json:"side"`
	OrderOwner      string         `json:"order_owner"`
	OriginalSize    string         `json:"original_size"`
	SizeMatched     string         `json:"size_matched"`
	Price           string         `json:"price"`
	AssociateTrades []string       `json:"associate_trades"`
	Outcome         string         `json:"outcome"`
	Type            OrderEventType `json:"type"`
	CreatedAt       string         `json:"created_at"`
	Expiration      string         `json:"expiration"`
	OrderType       OrderType      `json:"order_type"`
	Status          string         `json:"status"`
	MakerAddress    string         `json:"maker_address"`
	Timestamp       string         `json:"timestamp"`
}
