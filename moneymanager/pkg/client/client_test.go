package client

import (
	"context"
	"strings"
	"testing"

	"github.com/AndochBonin/E3/moneymanager/pkg/order"
	"github.com/AndochBonin/E3/moneymanager/pkg/testserver"
	"github.com/shopspring/decimal"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const testPrivKeyHex = "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
const testDepositWallet = "0x70997970C51812dc3A010C7d01b50e0d17dc79C8"

func dialTestServer(t *testing.T, balanceUSDC *int64) *Client {
	t.Helper()
	addr, cleanup, err := testserver.Start(testserver.Config{
		PrivateKeyHex:        testPrivKeyHex,
		DefaultDepositWallet: testDepositWallet,
		DefaultSignatureType: 3,
		BalanceUSDC:          balanceUSDC,
	})
	if err != nil {
		t.Fatalf("testserver.Start: %v", err)
	}
	t.Cleanup(cleanup)

	c, err := Dial(context.Background(), addr)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestSignLimitOrder_grpcIntegration(t *testing.T) {
	c := dialTestServer(t, nil)

	payload, err := c.SignLimitOrder(context.Background(), SignLimitOrderParams{
		TokenID:       "12345",
		Side:          order.SideBuy,
		Price:         decimal.RequireFromString("0.50"),
		Size:          decimal.RequireFromString("10"),
		TimestampMs:   1_700_000_123,
		DepositWallet: testDepositWallet,
		SignatureType: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if payload.Maker != testDepositWallet {
		t.Fatalf("maker: got %q want %q", payload.Maker, testDepositWallet)
	}
	if payload.MakerAmount != "5000000" || payload.TakerAmount != "10000000" {
		t.Fatalf("amounts: maker=%q taker=%q", payload.MakerAmount, payload.TakerAmount)
	}
	if !strings.HasPrefix(payload.Signature, "0x") {
		t.Fatalf("signature: got %q", payload.Signature)
	}
}

func TestProcessSignal_grpcIntegration(t *testing.T) {
	c := dialTestServer(t, nil)

	payload, err := c.ProcessSignal(context.Background(), ProcessSignalParams{
		TokenID:     "12345",
		Side:        order.SideBuy,
		Price:       "0.50",
		TimestampMs: 1_700_000_123,
	})
	if err != nil {
		t.Fatal(err)
	}
	if payload.TokenID != "12345" || payload.Side != order.SideBuy {
		t.Fatalf("payload: %+v", payload)
	}
	if payload.MakerAmount == "" || payload.Signature == "" {
		t.Fatalf("incomplete payload: %+v", payload)
	}
}

func TestProcessSignal_sellRejected(t *testing.T) {
	c := dialTestServer(t, nil)

	_, err := c.ProcessSignal(context.Background(), ProcessSignalParams{
		TokenID:     "12345",
		Side:        order.SideSell,
		Price:       "0.50",
		TimestampMs: 1_700_000_123,
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("code: got %v want FailedPrecondition", status.Code(err))
	}
}

func TestProcessSignal_insufficientBalance(t *testing.T) {
	zero := int64(0)
	c := dialTestServer(t, &zero)

	_, err := c.ProcessSignal(context.Background(), ProcessSignalParams{
		TokenID:     "12345",
		Side:        order.SideBuy,
		Price:       "0.50",
		TimestampMs: 1_700_000_123,
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("code: got %v want FailedPrecondition", status.Code(err))
	}
}

func TestSideToProtoAndFromProto(t *testing.T) {
	if SideFromProto(SideToProto(order.SideBuy)) != order.SideBuy {
		t.Fatal("buy round-trip")
	}
	if SideFromProto(SideToProto(order.SideSell)) != order.SideSell {
		t.Fatal("sell round-trip")
	}
}
