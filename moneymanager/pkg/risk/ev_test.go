package risk

import (
	"math"
	"testing"

	"github.com/shopspring/decimal"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestExpectedValueDecimalOdds(t *testing.T) {
	tests := []struct {
		name   string
		p, odds float64
		want   float64
	}{
		{"positive EV", 0.6, 2.0, 0.2},
		{"break even", 0.5, 2.0, 0},
		{"negative EV", 0.5, 1.9, -0.05},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExpectedValueDecimalOdds(tt.p, tt.odds)
			if math.Abs(got-tt.want) > 1e-12 {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}

func TestExpectedValueBuyPrice(t *testing.T) {
	ev, err := ExpectedValueBuyPrice(0.6, decimal.RequireFromString("0.50"))
	if err != nil {
		t.Fatal(err)
	}
	// 0.6/0.5 - 1 = 0.2
	want := decimal.RequireFromString("0.2")
	if !ev.Equal(want) {
		t.Fatalf("got %s want %s", ev, want)
	}

	_, err = ExpectedValueBuyPrice(0.6, decimal.Zero)
	if err == nil {
		t.Fatal("expected error for non-positive price")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code: got %v want InvalidArgument", status.Code(err))
	}
}

func TestValidateWinProbability(t *testing.T) {
	tests := []struct {
		name    string
		p       float64
		wantErr codes.Code
	}{
		{"interior", 0.55, codes.OK},
		{"one", 1, codes.OK},
		{"zero", 0, codes.InvalidArgument},
		{"negative", -0.1, codes.InvalidArgument},
		{"above one", 1.01, codes.InvalidArgument},
		{"nan", math.NaN(), codes.InvalidArgument},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateWinProbability(tt.p)
			if tt.wantErr == codes.OK {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error")
			}
			if status.Code(err) != tt.wantErr {
				t.Fatalf("code: got %v want %v", status.Code(err), tt.wantErr)
			}
		})
	}
}

func TestValidatePositiveEV(t *testing.T) {
	tests := []struct {
		name    string
		p       float64
		side    Side
		price   string
		wantErr codes.Code
	}{
		{"buy positive EV", 0.6, SideBuy, "0.50", codes.OK},
		{"buy zero EV", 0.5, SideBuy, "0.50", codes.FailedPrecondition},
		{"buy negative EV", 0.4, SideBuy, "0.50", codes.FailedPrecondition},
		{"sell", 0.6, SideSell, "0.50", codes.FailedPrecondition},
		{"invalid probability", 0, SideBuy, "0.50", codes.InvalidArgument},
		{"invalid price", 0.6, SideBuy, "0", codes.InvalidArgument},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			price, err := decimal.NewFromString(tt.price)
			if err != nil {
				t.Fatal(err)
			}
			err = ValidatePositiveEV(tt.p, tt.side, price)
			if tt.wantErr == codes.OK {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error")
			}
			if status.Code(err) != tt.wantErr {
				t.Fatalf("code: got %v want %v", status.Code(err), tt.wantErr)
			}
		})
	}
}

func TestExpectedValueDecimalOdds_acceptReject(t *testing.T) {
	if ExpectedValueDecimalOdds(0.6, 2.0) <= 0 {
		t.Fatal("expected positive EV for p=0.6, odds=2.0")
	}
	if ExpectedValueDecimalOdds(0.5, 1.9) > 0 {
		t.Fatal("expected non-positive EV for p=0.5, odds=1.9")
	}
}
