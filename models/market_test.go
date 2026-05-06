package models

import (
	"reflect"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestMarketFromGamma(t *testing.T) {
	timestamp := time.Date(2026, time.May, 6, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name  string
		input GammaMarket
		want  Market
	}{
		{
			name: "maps gamma market fields to domain market",
			input: GammaMarket{
				ConditionID:           "cond-123",
				Slug:                  "atp-miami-open",
				Question:              "Will player A win?",
				Category:              "sports",
				EndDate:               timestamp,
				Active:                true,
				Closed:                false,
				AcceptingOrders:       true,
				OrderMinSize:          5,
				OrderPriceMinTickSize: decimal.RequireFromString("0.01"),
				MakerBaseFee:          10,
				TakerBaseFee:          20,
				GameStartTime:         "2026-05-06T11:00:00Z",
			},
			want: Market{
				ConditionID:     "cond-123",
				Slug:            "atp-miami-open",
				Question:        "Will player A win?",
				Category:        "sports",
				EndDate:         timestamp,
				Active:          true,
				Closed:          false,
				AcceptingOrders: true,
				MinOrderSize:    5,
				MinTickSize:     decimal.RequireFromString("0.01"),
				MakerFee:        10,
				TakerFee:        20,
				GameStartTime:   "2026-05-06T11:00:00Z",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MarketFromGamma(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("MarketFromGamma() mismatch:\n got: %+v\nwant: %+v", got, tt.want)
			}
		})
	}
}
