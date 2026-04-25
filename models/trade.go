package models

// TradeStatus represents the current state of a trade.
type TradeStatus string

const (
	TradeStatusConfirmed TradeStatus = "TRADE_STATUS_CONFIRMED"
	TradeStatusFailed    TradeStatus = "TRADE_STATUS_FAILED"
	TradeStatusRetrying  TradeStatus = "TRADE_STATUS_RETRYING"
	TradeStatusMatched   TradeStatus = "TRADE_STATUS_MATCHED"
	TradeStatusMined     TradeStatus = "TRADE_STATUS_MINED"
)

// TraderSide represents whether we were the maker or taker in the trade.
type TraderSide string

const (
	TraderSideTaker TraderSide = "TAKER"
	TraderSideMaker TraderSide = "MAKER"
)

// Trade represents a single executed trade from GET /trades.
type Trade struct {
	ID              string      `json:"id"`
	TakerOrderID    string      `json:"taker_order_id"`
	Market          string      `json:"market"`
	AssetID         string      `json:"asset_id"`
	Side            OrderSide   `json:"side"`
	Size            string      `json:"size"`
	FeeRateBps      string      `json:"fee_rate_bps"`
	Price           string      `json:"price"`
	Status          TradeStatus `json:"status"`
	MatchTime       string      `json:"match_time"`
	LastUpdate      string      `json:"last_update"`
	Outcome         string      `json:"outcome"`
	BucketIndex     int         `json:"bucket_index"`
	Owner           string      `json:"owner"`
	MakerAddress    string      `json:"maker_address"`
	TransactionHash string      `json:"transaction_hash"`
	TraderSide      TraderSide  `json:"trader_side"`
	MakerOrders     []string    `json:"maker_orders"`
}

// TradesResponse is the paginated wrapper from GET /trades.
type TradesResponse struct {
	Limit      int     `json:"limit"`
	NextCursor string  `json:"next_cursor"`
	Count      int     `json:"count"`
	Data       []Trade `json:"data"`
}
