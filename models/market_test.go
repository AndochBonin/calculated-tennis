package models

import (
	"encoding/json"
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

func TestGammaMarketUnmarshalEventsGameID(t *testing.T) {
	t.Parallel()

	const rawMarket = `{
		"id":"2144525",
		"slug":"atp-sample-market",
		"conditionId":"0xabc123",
		"question":"Will player A win?",
		"category":"sports",
		"active":true,
		"closed":false,
		"archived":false,
		"acceptingOrders":true,
		"enableOrderBook":true,
		"clobTokenIds":"[\"yes-token\",\"no-token\"]",
		"outcomes":"[\"YES\",\"NO\"]",
		"liquidityNum":"0",
		"volumeNum":"0",
		"lastTradePrice":"0",
		"bestBid":"0",
		"bestAsk":"0",
		"orderMinSize":1,
		"orderPriceMinTickSize":"0.01",
		"makerBaseFee":0,
		"takerBaseFee":0,
		"endDate":"2026-05-06T10:00:00Z",
		"events":[
			{
				"gameId":5428186
			}
		]
	}`

	var got GammaMarket
	if err := json.Unmarshal([]byte(rawMarket), &got); err != nil {
		t.Fatalf("unmarshal raw gamma market: %v", err)
	}

	if len(got.Events) != 1 {
		t.Fatalf("expected one event, got %d", len(got.Events))
	}
	if got.Events[0].GameID != 5428186 {
		t.Fatalf("expected events[0].gameId=5428186, got %d", got.Events[0].GameID)
	}
}
