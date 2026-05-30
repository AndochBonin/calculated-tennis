package server

import (
	"context"
	"testing"

	"github.com/AndochBonin/E3/moneymanager/internal/order"
	"github.com/AndochBonin/E3/moneymanager/internal/risk"
	"github.com/AndochBonin/E3/moneymanager/internal/signer"
	"github.com/AndochBonin/E3/moneymanager/internal/testutil"
	moneymanagerv1 "github.com/AndochBonin/E3/moneymanager/gen/moneymanager/v1"
	"github.com/ethereum/go-ethereum/common"
	"github.com/shopspring/decimal"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const testPrivKeyHex = "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
const testDepositWallet = "0x70997970C51812dc3A010C7d01b50e0d17dc79C8"

func testServer(t *testing.T) *Server {
	t.Helper()
	s, err := signer.NewSigner(testPrivKeyHex)
	if err != nil {
		t.Fatal(err)
	}
	raw := decimal.NewFromInt(100).Mul(decimal.NewFromInt(1_000_000)).BigInt()
	alloc := risk.NewAllocator(testutil.StubBalance{Total: raw}, risk.Config{
		DepositWallet: common.HexToAddress(testDepositWallet),
		MaxPctBalance: decimal.NewFromFloat(0.05),
		MinShareSize:  decimal.NewFromInt(5),
	})
	return New(Config{
		Signer:               s,
		Allocator:            alloc,
		DefaultDepositWallet: testDepositWallet,
		DefaultSignatureType: 3,
	})
}

func TestSignOrder_success(t *testing.T) {
	svc := testServer(t)
	resp, err := svc.SignOrder(context.Background(), &moneymanagerv1.SignOrderRequest{
		TokenId:     "12345",
		Side:        moneymanagerv1.OrderSide_ORDER_SIDE_BUY,
		Price:       "0.50",
		Size:        "10",
		TimestampMs: 1_700_000_123,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetOrder().GetSignature() == "" {
		t.Fatal("expected signature")
	}
	if resp.GetOrder().GetMaker() != testDepositWallet {
		t.Fatalf("maker: got %q", resp.GetOrder().GetMaker())
	}
}

func TestSignOrder_missingSide(t *testing.T) {
	svc := testServer(t)
	_, err := svc.SignOrder(context.Background(), &moneymanagerv1.SignOrderRequest{
		TokenId: "12345",
		Price:   "0.50",
		Size:    "10",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code: got %v", status.Code(err))
	}
}

func TestProcessSignal_success(t *testing.T) {
	svc := testServer(t)
	resp, err := svc.ProcessSignal(context.Background(), &moneymanagerv1.ProcessSignalRequest{
		TokenId:     "12345",
		Side:        moneymanagerv1.OrderSide_ORDER_SIDE_BUY,
		Price:       "0.50",
		TimestampMs: 1_700_000_123,
	})
	if err != nil {
		t.Fatal(err)
	}
	order := resp.GetOrder()
	if order.GetSignature() == "" {
		t.Fatal("expected signature")
	}
	// 100 USDC * 5% / 0.50 = 10 shares
	if order.GetSide() != moneymanagerv1.OrderSide_ORDER_SIDE_BUY {
		t.Fatalf("side: got %v", order.GetSide())
	}
}

func TestProcessSignal_sellRejected(t *testing.T) {
	svc := testServer(t)
	_, err := svc.ProcessSignal(context.Background(), &moneymanagerv1.ProcessSignalRequest{
		TokenId:     "12345",
		Side:        moneymanagerv1.OrderSide_ORDER_SIDE_SELL,
		Price:       "0.50",
		TimestampMs: 1_700_000_123,
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("code: got %v want FailedPrecondition", status.Code(err))
	}
}

func TestSideToProto(t *testing.T) {
	if sideToProto(order.SideBuy) != moneymanagerv1.OrderSide_ORDER_SIDE_BUY {
		t.Fatal("buy mapping")
	}
	if sideToProto(order.SideSell) != moneymanagerv1.OrderSide_ORDER_SIDE_SELL {
		t.Fatal("sell mapping")
	}
}
