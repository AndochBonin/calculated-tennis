package risk

import (
	"context"
	"math/big"
	"testing"

	"github.com/AndochBonin/calculated-tennis/moneymanager/internal/balance"
	"github.com/AndochBonin/calculated-tennis/moneymanager/internal/testutil"
	"github.com/ethereum/go-ethereum/common"
	"github.com/shopspring/decimal"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestAllocator_Allocate_buy(t *testing.T) {
	// 100 USDC available, 5% cap => 5 USDC, price 0.50 => 10 shares
	wallet := common.HexToAddress("0x0000000000000000000000000000000000000001")
	raw := decimal.NewFromInt(100).Mul(decimal.NewFromInt(1_000_000)).BigInt()

	a := NewAllocator(testutil.StubBalance{Total: raw}, Config{
		DepositWallet: wallet,
		MaxPctBalance: decimal.NewFromFloat(0.05),
		MinShareSize:  decimal.NewFromInt(5),
	})

	size, err := a.Allocate(context.Background(), SideBuy, decimal.RequireFromString("0.50"))
	if err != nil {
		t.Fatal(err)
	}
	if !size.Equal(decimal.NewFromInt(10)) {
		t.Fatalf("size: got %s want 10", size)
	}
}

func TestAllocator_Allocate_maxOrderCap(t *testing.T) {
	wallet := common.HexToAddress("0x0000000000000000000000000000000000000001")
	raw := decimal.NewFromInt(1000).Mul(decimal.NewFromInt(1_000_000)).BigInt()

	a := NewAllocator(testutil.StubBalance{Total: raw}, Config{
		DepositWallet: wallet,
		MaxPctBalance: decimal.NewFromFloat(0.50),
		MaxOrderUSDC:  decimal.NewFromInt(10),
		MinShareSize:  decimal.NewFromInt(5),
	})

	size, err := a.Allocate(context.Background(), SideBuy, decimal.RequireFromString("1"))
	if err != nil {
		t.Fatal(err)
	}
	if !size.Equal(decimal.NewFromInt(10)) {
		t.Fatalf("size: got %s want 10", size)
	}
}

func TestAllocator_Allocate_insufficientBalance(t *testing.T) {
	wallet := common.HexToAddress("0x0000000000000000000000000000000000000001")
	a := NewAllocator(testutil.StubBalance{Total: big.NewInt(0)}, Config{
		DepositWallet: wallet,
		MaxPctBalance: decimal.NewFromFloat(0.05),
		MinShareSize:  decimal.NewFromInt(5),
	})

	_, err := a.Allocate(context.Background(), SideBuy, decimal.RequireFromString("0.50"))
	if err == nil {
		t.Fatal("expected error")
	}
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("code: got %v want FailedPrecondition", status.Code(err))
	}
}

func TestAllocator_Allocate_belowMinSize(t *testing.T) {
	wallet := common.HexToAddress("0x0000000000000000000000000000000000000001")
	// 10 USDC, 5% => 0.5 USDC, price 0.50 => 1 share < min 5
	raw := decimal.NewFromInt(10).Mul(decimal.NewFromInt(1_000_000)).BigInt()

	a := NewAllocator(testutil.StubBalance{Total: raw}, Config{
		DepositWallet: wallet,
		MaxPctBalance: decimal.NewFromFloat(0.05),
		MinShareSize:  decimal.NewFromInt(5),
	})

	_, err := a.Allocate(context.Background(), SideBuy, decimal.RequireFromString("0.50"))
	if err == nil {
		t.Fatal("expected error")
	}
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("code: got %v want FailedPrecondition", status.Code(err))
	}
}

func TestAllocator_Allocate_sellRejected(t *testing.T) {
	wallet := common.HexToAddress("0x0000000000000000000000000000000000000001")
	raw := decimal.NewFromInt(100).Mul(decimal.NewFromInt(1_000_000)).BigInt()

	a := NewAllocator(testutil.StubBalance{Total: raw}, Config{
		DepositWallet: wallet,
		MaxPctBalance: decimal.NewFromFloat(0.05),
		MinShareSize:  decimal.NewFromInt(5),
	})

	_, err := a.Allocate(context.Background(), SideSell, decimal.RequireFromString("0.50"))
	if err == nil {
		t.Fatal("expected error")
	}
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("code: got %v want FailedPrecondition", status.Code(err))
	}
}

func TestConfigFromEnv(t *testing.T) {
	wallet := common.HexToAddress("0x0000000000000000000000000000000000000001")
	t.Setenv("MONEYMANAGER_MAX_ORDER_USDC", "25")
	t.Setenv("MONEYMANAGER_MAX_PCT_BALANCE", "0.10")
	t.Setenv("MONEYMANAGER_MIN_SHARE_SIZE", "1")

	cfg, err := ConfigFromEnv(wallet)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.MaxOrderUSDC.Equal(decimal.NewFromInt(25)) {
		t.Fatalf("MaxOrderUSDC: got %s", cfg.MaxOrderUSDC)
	}
	if !cfg.MaxPctBalance.Equal(decimal.NewFromFloat(0.10)) {
		t.Fatalf("MaxPctBalance: got %s", cfg.MaxPctBalance)
	}
	if !cfg.MinShareSize.Equal(decimal.NewFromInt(1)) {
		t.Fatalf("MinShareSize: got %s", cfg.MinShareSize)
	}
}

func TestSignatureTypeFromEnv_default(t *testing.T) {
	t.Setenv("POLYMARKET_CLOB_SIGNATURE_TYPE", "")
	got, err := SignatureTypeFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if got != 3 {
		t.Fatalf("got %d want 3", got)
	}
}

func TestBalanceReaderInterface(t *testing.T) {
	var _ BalanceReader = (*balance.Reader)(nil)
}
