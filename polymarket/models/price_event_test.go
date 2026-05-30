package models

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/shopspring/decimal"
)

func TestClobPriceToPriceEvent(t *testing.T) {
	tests := []struct {
		name    string
		price   decimal.Decimal
		tokenID string
		side    OrderSide
	}{
		{
			name:    "builds price event for buy side",
			price:   decimal.RequireFromString("0.41"),
			tokenID: "token-yes",
			side:    OrderSideBuy,
		},
		{
			name:    "builds price event for sell side",
			price:   decimal.RequireFromString("0.59"),
			tokenID: "token-no",
			side:    OrderSideSell,
		},
	}

	market := Market{ConditionID: "cond-001"}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := ClobPriceToPriceEvent(
				ClobMarketPrice{Price: tt.price},
				market,
				tt.tokenID,
				tt.side,
			)

			if event == nil {
				t.Fatal("event should not be nil")
			}
			if event.Timestamp == "" {
				t.Fatal("Timestamp should not be empty")
			}

			want := &PriceEvent{
				EventType: "price_change",
				Market:    fmt.Sprintf("%v", market),
				PriceChanges: []PriceChange{
					{
						AssetID: tt.tokenID,
						Price:   fmt.Sprintf("%d", tt.price), // lock current behavior
						Side:    tt.side,
					},
				},
				Timestamp: event.Timestamp,
			}

			if !reflect.DeepEqual(event, want) {
				t.Fatalf("ClobPriceToPriceEvent() mismatch:\n got: %+v\nwant: %+v", event, want)
			}
		})
	}
}
