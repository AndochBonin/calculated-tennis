package balance

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// USDCTotal returns bridged + native USDC balance (6 decimals) for wallet.
func (r *Reader) USDCTotal(ctx context.Context, wallet common.Address) (*big.Int, error) {
	bridged, err := r.BalanceOf(ctx, BridgedUSDC, wallet)
	if err != nil {
		return nil, fmt.Errorf("balance: bridged usdc: %w", err)
	}
	native, err := r.BalanceOf(ctx, NativeUSDC, wallet)
	if err != nil {
		return nil, fmt.Errorf("balance: native usdc: %w", err)
	}
	return new(big.Int).Add(bridged, native), nil
}
