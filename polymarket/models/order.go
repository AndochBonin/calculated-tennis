package models

// OrderStatus represents the current state of an order.
type OrderStatus string

const (
	OrderStatusLive                   OrderStatus = "ORDER_STATUS_LIVE"
	OrderStatusInvalid                OrderStatus = "ORDER_STATUS_INVALID"
	OrderStatusCanceledMarketResolved OrderStatus = "ORDER_STATUS_CANCELED_MARKET_RESOLVED"
	OrderStatusCanceled               OrderStatus = "ORDER_STATUS_CANCELED"
	OrderStatusMatched                OrderStatus = "ORDER_STATUS_MATCHED"
)

// OrderSide represents the direction of an order.
type OrderSide string

const (
	OrderSideBuy  OrderSide = "BUY"
	OrderSideSell OrderSide = "SELL"
)

// OrderType represents the order execution strategy.
type OrderType string

const (
	OrderTypeGTC OrderType = "GTC" // Good till cancelled
	OrderTypeFOK OrderType = "FOK" // Fill or kill
	OrderTypeFAK OrderType = "FAK" // Fill and kill
	OrderTypeGTD OrderType = "GTD" // Good till date
)

// Order is the domain model returned from GET /orders.
type Order struct {
	ID              string      `json:"id"`
	Status          OrderStatus `json:"status"`
	Owner           string      `json:"owner"`
	MakerAddress    string      `json:"maker_address"`
	Market          string      `json:"market"`
	AssetID         string      `json:"asset_id"`
	Side            OrderSide   `json:"side"`
	OriginalSize    string      `json:"original_size"`
	SizeMatched     string      `json:"size_matched"`
	Price           string      `json:"price"`
	Outcome         string      `json:"outcome"`
	Expiration      string      `json:"expiration"`
	OrderType       OrderType   `json:"order_type"`
	AssociateTrades []string    `json:"associate_trades"`
	CreatedAt       int64       `json:"created_at"`
}

// OrdersResponse is the paginated wrapper from GET /orders.
type OrdersResponse struct {
	Limit      int     `json:"limit"`
	NextCursor string  `json:"next_cursor"`
	Count      int     `json:"count"`
	Data       []Order `json:"data"`
}
