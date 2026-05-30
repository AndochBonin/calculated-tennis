package risk

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"

	"github.com/AndochBonin/E3/moneymanager/internal/balance"
	"github.com/AndochBonin/E3/moneymanager/internal/order"
	"github.com/ethereum/go-ethereum/common"
	"github.com/shopspring/decimal"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	defaultMaxPctBalance = 0.05
	defaultMinShareSize    = 5
)

// BalanceReader fetches live USDC collateral for allocation.
type BalanceReader interface {
	USDCTotal(ctx context.Context, wallet common.Address) (*big.Int, error)
}

// Config holds risk limits for signal allocation.
type Config struct {
	DepositWallet common.Address
	MaxOrderUSDC  decimal.Decimal // zero = no absolute cap
	MaxPctBalance decimal.Decimal // fraction of available USDC (e.g. 0.05 = 5%)
	MinShareSize  decimal.Decimal
}

// ConfigFromEnv loads risk limits. depositWallet must already be resolved.
func ConfigFromEnv(depositWallet common.Address) (Config, error) {
	cfg := Config{
		DepositWallet: depositWallet,
		MaxPctBalance: decimal.NewFromFloat(defaultMaxPctBalance),
		MinShareSize:  decimal.NewFromInt(defaultMinShareSize),
	}

	if v := strings.TrimSpace(os.Getenv("MONEYMANAGER_MAX_ORDER_USDC")); v != "" {
		d, err := decimal.NewFromString(v)
		if err != nil {
			return Config{}, fmt.Errorf("risk: MONEYMANAGER_MAX_ORDER_USDC: %w", err)
		}
		if d.IsNegative() {
			return Config{}, fmt.Errorf("risk: MONEYMANAGER_MAX_ORDER_USDC must be non-negative")
		}
		cfg.MaxOrderUSDC = d
	}

	if v := strings.TrimSpace(os.Getenv("MONEYMANAGER_MAX_PCT_BALANCE")); v != "" {
		d, err := decimal.NewFromString(v)
		if err != nil {
			return Config{}, fmt.Errorf("risk: MONEYMANAGER_MAX_PCT_BALANCE: %w", err)
		}
		if d.IsNegative() || d.GreaterThan(decimal.NewFromInt(1)) {
			return Config{}, fmt.Errorf("risk: MONEYMANAGER_MAX_PCT_BALANCE must be between 0 and 1")
		}
		cfg.MaxPctBalance = d
	}

	if v := strings.TrimSpace(os.Getenv("MONEYMANAGER_MIN_SHARE_SIZE")); v != "" {
		d, err := decimal.NewFromString(v)
		if err != nil {
			return Config{}, fmt.Errorf("risk: MONEYMANAGER_MIN_SHARE_SIZE: %w", err)
		}
		if !d.IsPositive() {
			return Config{}, fmt.Errorf("risk: MONEYMANAGER_MIN_SHARE_SIZE must be positive")
		}
		cfg.MinShareSize = d
	}

	return cfg, nil
}

// Allocator applies v1 risk rules and derives order size from live USDC balance.
type Allocator struct {
	bal BalanceReader
	cfg Config
}

func NewAllocator(bal BalanceReader, cfg Config) *Allocator {
	return &Allocator{bal: bal, cfg: cfg}
}

// Allocate returns share size for a trade signal. Only BUY is supported in v1.
func (a *Allocator) Allocate(ctx context.Context, side order.Side, price decimal.Decimal) (decimal.Decimal, error) {
	if side != order.SideBuy {
		return decimal.Zero, status.Errorf(codes.FailedPrecondition, "sell signals are not supported by risk v1")
	}
	if !price.IsPositive() {
		return decimal.Zero, status.Errorf(codes.InvalidArgument, "price must be positive")
	}

	raw, err := a.bal.USDCTotal(ctx, a.cfg.DepositWallet)
	if err != nil {
		return decimal.Zero, status.Errorf(codes.Internal, "read usdc balance: %v", err)
	}

	available := decimal.NewFromBigInt(raw, -balance.USDCDecimals)
	if !available.IsPositive() {
		return decimal.Zero, status.Errorf(codes.FailedPrecondition, "insufficient USDC balance")
	}

	allocated := available.Mul(a.cfg.MaxPctBalance)
	if a.cfg.MaxOrderUSDC.IsPositive() && allocated.GreaterThan(a.cfg.MaxOrderUSDC) {
		allocated = a.cfg.MaxOrderUSDC
	}
	if allocated.GreaterThan(available) {
		allocated = available
	}
	if !allocated.IsPositive() {
		return decimal.Zero, status.Errorf(codes.FailedPrecondition, "allocated USDC is zero")
	}

	size := allocated.Div(price).Truncate(2)
	if size.LessThan(a.cfg.MinShareSize) {
		return decimal.Zero, status.Errorf(codes.FailedPrecondition,
			"allocated size %s is below minimum %s (balance %s USDC, cap %s%%)",
			size, a.cfg.MinShareSize, available.StringFixed(balance.USDCDecimals),
			a.cfg.MaxPctBalance.Mul(decimal.NewFromInt(100)).StringFixed(2))
	}

	return size, nil
}

// SignatureTypeFromEnv returns POLYMARKET_CLOB_SIGNATURE_TYPE (default 3).
func SignatureTypeFromEnv() (uint8, error) {
	v := strings.TrimSpace(os.Getenv("POLYMARKET_CLOB_SIGNATURE_TYPE"))
	if v == "" {
		return 3, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("POLYMARKET_CLOB_SIGNATURE_TYPE: %w", err)
	}
	if n < 0 || n > 3 {
		return 0, fmt.Errorf("POLYMARKET_CLOB_SIGNATURE_TYPE=%d is invalid (must be 0..3)", n)
	}
	return uint8(n), nil
}
