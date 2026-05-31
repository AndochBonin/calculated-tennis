// Package testutil provides shared fakes for moneymanager unit and integration tests.
package testutil

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// StubBalance implements pkg/risk.BalanceReader for tests.
type StubBalance struct {
	Total *big.Int
	Err   error
}

func (s StubBalance) USDCTotal(ctx context.Context, wallet common.Address) (*big.Int, error) {
	return s.Total, s.Err
}
