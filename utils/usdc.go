package utils 

import "github.com/shopspring/decimal"

var micro = decimal.NewFromInt(1_000_000)

func USDCToMicro(amount decimal.Decimal) string {
	return amount.Mul(micro).StringFixed(0)
}

func MicroToUSDC(amount string) (decimal.Decimal, error) {
	d, err := decimal.NewFromString(amount)
	if err != nil {
		return decimal.Zero, err
	}
	return d.Div(micro), nil
}
