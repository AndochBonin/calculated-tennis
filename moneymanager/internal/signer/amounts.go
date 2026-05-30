package signer

import (
	"github.com/AndochBonin/E3/moneymanager/internal/order"
	"github.com/shopspring/decimal"
)

var micro = decimal.NewFromInt(1_000_000)

func usdcToMicro(amount decimal.Decimal) string {
	return amount.Mul(micro).StringFixed(0)
}

func computeAmounts(side order.Side, price, size decimal.Decimal) (string, string) {
	if side == order.SideBuy {
		makerAmount := usdcToMicro(price.Mul(size))
		takerAmount := size.Mul(micro).StringFixed(0)
		return makerAmount, takerAmount
	}
	makerAmount := size.Mul(micro).StringFixed(0)
	takerAmount := usdcToMicro(price.Mul(size))
	return makerAmount, takerAmount
}
