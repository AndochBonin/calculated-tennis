package utils

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestUSDCToMicro(t *testing.T) {
	tests := []struct {
		name   string
		input  decimal.Decimal
		output string
	}{
		{
			name:   "whole number converts to micros",
			input:  decimal.RequireFromString("2"),
			output: "2000000",
		},
		{
			name:   "fractional converts to micros",
			input:  decimal.RequireFromString("1.234567"),
			output: "1234567",
		},
		{
			name:   "small fractional rounds with fixed 0 precision",
			input:  decimal.RequireFromString("0.000001"),
			output: "1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := USDCToMicro(tt.input)
			if got != tt.output {
				t.Fatalf("USDCToMicro(%s): got %q want %q", tt.input, got, tt.output)
			}
		})
	}
}

func TestMicroToUSDC(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      decimal.Decimal
		expectErr bool
	}{
		{
			name:  "whole micros convert to usdc",
			input: "2000000",
			want:  decimal.RequireFromString("2"),
		},
		{
			name:  "fractional micros convert to usdc",
			input: "1234567",
			want:  decimal.RequireFromString("1.234567"),
		},
		{
			name:      "invalid string returns error",
			input:     "not-a-number",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MicroToUSDC(tt.input)
			if tt.expectErr {
				if err == nil {
					t.Fatalf("MicroToUSDC(%q): expected error", tt.input)
				}
				return
			}

			if err != nil {
				t.Fatalf("MicroToUSDC(%q): unexpected error: %v", tt.input, err)
			}
			if !got.Equal(tt.want) {
				t.Fatalf("MicroToUSDC(%q): got %s want %s", tt.input, got, tt.want)
			}
		})
	}
}
