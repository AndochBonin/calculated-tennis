// checkbalance prints Polygon USDC balances for the Polymarket deposit wallet.
//
// Run (from repo root):
//
//	make check-balance
//
// Or from moneymanager/:
//
//	go run ./cmd/checkbalance
//
// Env:
//   - POLYGON_RPC_URL — Polygon PoS JSON-RPC (see moneymanager/.env.example)
//   - POLYMARKET_DEPOSIT_WALLET or DEPOSIT_WALLET — wallet to inspect
//   - POLYMARKET_SECRETS_MANAGER_SECRET_ID — optional AWS Secrets Manager JSON secret
package main

import (
	"context"
	"fmt"
	"log/slog"
	"math/big"
	"os"

	"github.com/AndochBonin/calculated-tennis/moneymanager/internal/balance"
	"github.com/AndochBonin/calculated-tennis/moneymanager/internal/env"
	"github.com/AndochBonin/calculated-tennis/moneymanager/internal/secrets"
	"github.com/ethereum/go-ethereum/common"
)

func main() {
	os.Exit(exitRun())
}

func exitRun() int {
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	env.Load()
	secrets.MustLoadFromEnvIfConfigured(context.Background(), log)

	wallet, err := balance.DepositWalletFromEnv()
	if err != nil {
		log.Error("wallet", "err", err)
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

	ctx := context.Background()
	chainID, err := reader.ChainID(ctx)
	if err != nil {
		log.Error("chain id", "err", err)
		return 1
	}

	bridged, err := reader.BalanceOf(ctx, balance.BridgedUSDC, wallet)
	if err != nil {
		log.Error("bridged usdc", "err", err)
		return 1
	}
	native, err := reader.BalanceOf(ctx, balance.NativeUSDC, wallet)
	if err != nil {
		log.Error("native usdc", "err", err)
		return 1
	}

	printReport(wallet, chainID, bridged, native)
	return 0
}

func printReport(wallet common.Address, chainID, bridged, native *big.Int) {
	fmt.Printf("wallet: %s\n", wallet.Hex())
	fmt.Printf("chain_id: %s\n", chainID.String())
	if chainID.Int64() != balance.PolygonChainID {
		fmt.Fprintf(os.Stderr, "warning: expected Polygon chain id %d, RPC reported %s\n",
			balance.PolygonChainID, chainID.String())
	}
	fmt.Printf("usdc_bridged (%s): %s USDC\n", balance.BridgedUSDC.Hex(), balance.FormatUSDC(bridged))
	fmt.Printf("usdc_native (%s): %s USDC\n", balance.NativeUSDC.Hex(), balance.FormatUSDC(native))
	total := new(big.Int).Add(bridged, native)
	fmt.Printf("usdc_total: %s USDC\n", balance.FormatUSDC(total))
}
