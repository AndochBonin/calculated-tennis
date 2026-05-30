// Package testserver starts an in-process Money Manager gRPC server for integration tests.
package testserver

import (
	"net"

	"github.com/AndochBonin/E3/moneymanager/internal/risk"
	"github.com/AndochBonin/E3/moneymanager/internal/server"
	"github.com/AndochBonin/E3/moneymanager/internal/signer"
	"github.com/AndochBonin/E3/moneymanager/internal/testutil"
	moneymanagerv1 "github.com/AndochBonin/E3/moneymanager/gen/moneymanager/v1"
	"github.com/ethereum/go-ethereum/common"
	"github.com/shopspring/decimal"
	"google.golang.org/grpc"
)

// Config configures a test gRPC server.
type Config struct {
	PrivateKeyHex        string
	DefaultDepositWallet string
	DefaultSignatureType uint8
	// BalanceUSDC is stub collateral for ProcessSignal. When nil, defaults to 100 USDC.
	BalanceUSDC *int64
}

// Start listens on 127.0.0.1:0 and serves MoneyManagerService. Caller must invoke cleanup when done.
func Start(cfg Config) (addr string, cleanup func(), err error) {
	s, err := signer.NewSigner(cfg.PrivateKeyHex)
	if err != nil {
		return "", nil, err
	}
	sigType := cfg.DefaultSignatureType
	if sigType == 0 {
		sigType = 3
	}

	deposit := cfg.DefaultDepositWallet
	balUSDC := int64(100)
	if cfg.BalanceUSDC != nil {
		balUSDC = *cfg.BalanceUSDC
	}
	raw := decimal.NewFromInt(balUSDC).Mul(decimal.NewFromInt(1_000_000)).BigInt()
	alloc := risk.NewAllocator(testutil.StubBalance{Total: raw}, risk.Config{
		DepositWallet: common.HexToAddress(deposit),
		MaxPctBalance: decimal.NewFromFloat(0.05),
		MinShareSize:  decimal.NewFromInt(5),
	})

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, err
	}
	gs := grpc.NewServer()
	moneymanagerv1.RegisterMoneyManagerServiceServer(gs, server.New(server.Config{
		Signer:               s,
		Allocator:            alloc,
		DefaultDepositWallet: deposit,
		DefaultSignatureType: sigType,
	}))
	go gs.Serve(lis)

	return lis.Addr().String(), func() {
		gs.Stop()
		_ = lis.Close()
	}, nil
}
