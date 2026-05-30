package models

import "github.com/shopspring/decimal"

type PriceLevel struct {
	Price decimal.Decimal `json:"price"`
	Size  decimal.Decimal `json:"size"`
}

type OrderBook struct {
	Market         string          `json:"market"`
	AssetID        string          `json:"asset_id"`
	Timestamp      string          `json:"timestamp"`
	Hash           string          `json:"hash"`
	Bids           []PriceLevel    `json:"bids"`
	Asks           []PriceLevel    `json:"asks"`
	MinOrderSize   decimal.Decimal `json:"min_order_size"`
	TickSize       decimal.Decimal `json:"tick_size"`
	NegRisk        bool            `json:"neg_risk"`
	LastTradePrice decimal.Decimal `json:"last_trade_price"`
}
