package clob

import (
	"fmt"

	"github.com/AndochBonin/E3/polymarket/models"
	"github.com/shopspring/decimal"
)

// ValidateLimitPrice checks that price lies on the tick grid: price must be an exact
// multiple of tickSize in decimal space (e.g. 0.5 on 0.01 ticks).
func ValidateLimitPrice(price, tickSize decimal.Decimal) error {
	if !tickSize.IsPositive() {
		return fmt.Errorf("tick size must be positive")
	}
	if price.Mod(tickSize).IsZero() {
		return nil
	}
	return fmt.Errorf("limit price is not a multiple of tick size %s", tickSize)
}

// ValidateMinOrderSize checks size >= minOrderSize.
func ValidateMinOrderSize(size, minOrderSize decimal.Decimal) error {
	if size.Cmp(minOrderSize) < 0 {
		return fmt.Errorf("order size %s is below minimum %s", size, minOrderSize)
	}
	return nil
}

// ValidateLimitOrderAgainstBook fetches the order book for tokenID and ensures price
// matches TickSize and size meets MinOrderSize. On success it returns the book so
// callers can pass book.NegRisk into signing; if negRisk does not match the book, the
// exchange rejects the order.
func (c *Client) ValidateLimitOrderAgainstBook(tokenID string, price, size decimal.Decimal) (*models.OrderBook, error) {
	book, err := c.GetOrderBook(tokenID)
	if err != nil {
		return nil, err
	}
	if err := ValidateLimitPrice(price, book.TickSize); err != nil {
		return nil, err
	}
	if err := ValidateMinOrderSize(size, book.MinOrderSize); err != nil {
		return nil, err
	}
	return book, nil
}
