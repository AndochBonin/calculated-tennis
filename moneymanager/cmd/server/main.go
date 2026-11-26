// moneymanager server signs Polymarket CLOB orders and applies risk allocation over gRPC.
//
// Run (from repo root):
//
//	make -C moneymanager run
//
// Env:
//   - POLYMARKET_PRIVATE_KEY or METAMASK_KEY — signing key (required)
//   - POLYMARKET_DEPOSIT_WALLET or DEPOSIT_WALLET — funder address (required)
//   - POLYGON_RPC_URL — JSON-RPC for live USDC balance reads (required)
//   - MONEYMANAGER_GRPC_ADDR — listen address (default 127.0.0.1:50051)
//   - MONEYMANAGER_MAX_ORDER_USDC — optional absolute USDC cap per signal
//   - MONEYMANAGER_MAX_PCT_BALANCE — fraction of balance to allocate (default 0.05)
//   - POLYMARKET_CLOB_SIGNATURE_TYPE — signature type (default 3)
//   - POLYMARKET_SECRETS_MANAGER_SECRET_ID — optional AWS Secrets Manager JSON secret
package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	moneymanagerv1 "github.com/AndochBonin/calculated-tennis/moneymanager/gen/moneymanager/v1"
	"github.com/AndochBonin/calculated-tennis/moneymanager/internal/balance"
	"github.com/AndochBonin/calculated-tennis/moneymanager/internal/env"
	"github.com/AndochBonin/calculated-tennis/moneymanager/pkg/risk"
	"github.com/AndochBonin/calculated-tennis/moneymanager/internal/secrets"
	"github.com/AndochBonin/calculated-tennis/moneymanager/internal/server"
	"github.com/AndochBonin/calculated-tennis/moneymanager/internal/signer"
	"google.golang.org/grpc"
)

func main() {
	os.Exit(exitRun())
}

func exitRun() int {
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	env.Load()
	secrets.MustLoadFromEnvIfConfigured(context.Background(), log)

	keyHex, err := server.PrivateKeyFromEnv()
	if err != nil {
		log.Error("private key", "err", err)
		return 1
	}
	s, err := signer.NewSigner(keyHex)
	if err != nil {
		log.Error("signer", "err", err)
		return 1
	}

	depositWallet, err := balance.DepositWalletFromEnv()
	if err != nil {
		log.Error("deposit wallet", "err", err)
		return 1
	}
	rpcURL, err := balance.RPCURLFromEnv()
	if err != nil {
		log.Error("rpc", "err", err)
		return 1
	}

	reader, err := balance.NewReader(rpcURL)
	if err != nil {
		log.Error("connect", "err", err)
		return 1
	}
	defer reader.Close()

	riskCfg, err := risk.ConfigFromEnv(depositWallet)
	if err != nil {
		log.Error("risk config", "err", err)
		return 1
	}
	sigType, err := risk.SignatureTypeFromEnv()
	if err != nil {
		log.Error("signature type", "err", err)
		return 1
	}

	svc := server.New(server.Config{
		Signer:               s,
		Allocator:            risk.NewAllocator(reader, riskCfg),
		DefaultDepositWallet: depositWallet.Hex(),
		DefaultSignatureType: sigType,
	})

	addr := server.ListenAddrFromEnv()
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Error("listen", "addr", addr, "err", err)
		return 1
	}

	grpcServer := grpc.NewServer()
	moneymanagerv1.RegisterMoneyManagerServiceServer(grpcServer, svc)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		log.Info("shutting down")
		grpcServer.GracefulStop()
	}()

	log.Info("listening",
		"addr", addr,
		"signer", s.Address(),
		"deposit_wallet", depositWallet.Hex(),
	)
	if err := grpcServer.Serve(lis); err != nil {
		log.Error("serve", "err", err)
		return 1
	}
	return 0
}
