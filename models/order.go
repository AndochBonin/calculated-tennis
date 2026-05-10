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

// OrderPayload is the signed order struct sent inside POST /order.
type OrderPayload struct {
	Maker         string    `json:"maker"`
	Signer        string    `json:"signer"`
	TokenID       string    `json:"tokenId"`
	MakerAmount   string    `json:"makerAmount"`
	TakerAmount   string    `json:"takerAmount"`
	Side          OrderSide `json:"side"`
	Expiration    string    `json:"expiration"`
	Timestamp     string    `json:"timestamp"`
	Metadata      string    `json:"metadata"`
	Builder       string    `json:"builder"`
	Signature     string    `json:"signature"`
	Salt          int64     `json:"salt"`
	SignatureType int       `json:"signatureType"`
}

// PlaceOrderRequest is the full request body for POST /order.
type PlaceOrderRequest struct {
	Order     OrderPayload `json:"order"`
	Owner     string       `json:"owner"`
	OrderType OrderType    `json:"orderType"`
	DeferExec bool         `json:"deferExec"`
}

// PlaceOrderResponse is the response body returned from POST /order.
type PlaceOrderResponse struct {
	Success            bool     `json:"success"`
	OrderID            string   `json:"orderID"`
	Status             string   `json:"status"`
	MakingAmount       string   `json:"makingAmount"`
	TakingAmount       string   `json:"takingAmount"`
	TransactionsHashes []string `json:"transactionsHashes"`
	TradeIDs           []string `json:"tradeIDs"`
	ErrorMsg           string   `json:"errorMsg"`
}

// CancelOrderRequest is the request body for DELETE /order.
type CancelOrderRequest struct {
	OrderID string `json:"orderID"`
}

// CancelOrderResponse is the response body returned from DELETE /order.
type CancelOrderResponse struct {
	Canceled    []string          `json:"canceled"`
	NotCanceled map[string]string `json:"not_canceled"`
}
