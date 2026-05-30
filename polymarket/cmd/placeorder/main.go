// Place a small test limit order on the live Polymarket CLOB (real funds / real API).
//
// Run (from repo root, with credentials in .env or the environment):
//
//	go run ./cmd/placeorder
//	go run ./cmd/placeorder -price=0.50 <token_id>
//
// Or: make place-order
// Or: make place-order PRICE=0.50 TOKEN=<token_id>
//
// Arguments:
//   - token_id: CLOB outcome token id (asset id); positional, or prompted when omitted on a TTY.
//   - -price: limit price as a decimal string (must match book tick size); flag or prompted when omitted on a TTY.
//
// Fixed order shape: 5 shares (typical minimum), BUY, GTC, expiration 0.
//
// Required env (see clob.NewClient): POLYMARKET_API_KEY, POLYMARKET_API_SECRET,
// POLYMARKET_PASSPHRASE, POLYMARKET_ADDRESS (EOA that derived the API credentials).
// Deposit / Safe address for order EIP-712 TypedDataSign (must be set for BuildLimitOrder):
// POLYMARKET_DEPOSIT_WALLET, or DEPOSIT_WALLET (e.g. from cmd/python/deploy_wallet.py output).
// This is not necessarily the same as POLYMARKET_ADDRESS.
// Private key for EIP-712 signing: POLYMARKET_PRIVATE_KEY, or METAMASK_KEY (same as
// cmd/python/generate_creds.py).
// Optional: POLYMARKET_CLOB_SERVER_TIME=true; POLYMARKET_CLOB_BASE_URL; POLYMARKET_USER_ADDRESS;
// POLYMARKET_DATA_API_BASE_URL.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/AndochBonin/E3/polymarket/clob"
	"github.com/AndochBonin/E3/polymarket/internal/prompt"
	"github.com/AndochBonin/E3/polymarket/models"
	"github.com/AndochBonin/E3/polymarket/secrets"
	"github.com/joho/godotenv"
	"github.com/shopspring/decimal"
)

var errUsage = errors.New("usage")

func main() {
	os.Exit(exitRun())
}

func exitRun() int {
	_ = godotenv.Load()

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	secrets.MustLoadFromEnvIfConfigured(context.Background(), log)

	priceFlag := flag.String("price", "", "limit price (decimal string)")
	flag.Parse()

	tokenID, priceStr, err := resolveInputs(os.Stdin, flag.Args(), *priceFlag, prompt.IsInteractive)
	if err != nil {
		if errors.Is(err, errUsage) {
			log.Error("usage", "msg", "price and token_id required")
			fmt.Fprintf(os.Stderr, "usage: %s -price=<decimal> <token_id>\n", os.Args[0])
			return 2
		}
		log.Error("input", "err", err)
		return 1
	}

	price, err := decimal.NewFromString(priceStr)
	if err != nil {
		log.Error("parse price", "err", err)
		return 1
	}

	keyHex, err := privateKeyFromEnv()
	if err != nil {
		log.Error("private key", "err", err)
		return 1
	}

	signer, err := clob.NewSigner(keyHex)
	if err != nil {
		log.Error("signer", "err", err)
		return 1
	}

	client := clob.NewClient()
	polyAddr := client.AuthAddress()
	depo := client.DepositWallet()
	log.Info("addresses",
		"eoa_from_private_key", signer.Address(),
		"poly_address_header", polyAddr,
		"deposit_wallet", depo,
		"eoa_matches_poly_address", strings.EqualFold(signer.Address(), polyAddr),
	)
	size := decimal.NewFromInt(5)

	book, err := client.ValidateLimitOrderAgainstBook(tokenID, price, size)
	if err != nil {
		log.Error("validate order", "err", err)
		return 1
	}

	payload, err := client.BuildLimitOrder(signer, tokenID, models.OrderSideBuy, price, size, book.NegRisk, 0)
	if err != nil {
		log.Error("build order", "err", err)
		return 1
	}

	log.Info("signed_order_payload",
		"maker", payload.Maker,
		"signer", payload.Signer,
		"maker_is_deposit_wallet", strings.EqualFold(payload.Maker, depo),
		"signer_is_eoa_key", strings.EqualFold(payload.Signer, signer.Address()),
		"signer_matches_poly_address", strings.EqualFold(payload.Signer, polyAddr),
		"packed_signature_byte_len", (len(payload.Signature)-2)/2,
	)

	resp, err := client.PlaceOrder(payload, "", models.OrderTypeGTC)
	if err != nil {
		log.Error("place order", "err", err)
		return 1
	}

	log.Info("placed",
		"orderID", resp.OrderID,
		"status", resp.Status,
		"signer", signer.Address(),
	)
	return 0
}

func resolveInputs(stdin *os.File, args []string, priceFlag string, interactive func(*os.File) bool) (tokenID, priceStr string, err error) {
	if len(args) > 1 {
		return "", "", errUsage
	}
	if len(args) == 1 {
		tokenID = strings.TrimSpace(args[0])
	}
	priceStr = strings.TrimSpace(priceFlag)

	if (tokenID == "" || priceStr == "") && interactive(stdin) {
		br := bufio.NewReader(stdin)
		if priceStr == "" {
			priceStr, err = prompt.ReadLineFrom(os.Stderr, br, "Limit price (decimal): ")
			if err != nil {
				return "", "", err
			}
		}
		if tokenID == "" {
			tokenID, err = prompt.ReadLineFrom(os.Stderr, br, "Token ID: ")
			if err != nil {
				return "", "", err
			}
		}
	}

	if tokenID == "" || priceStr == "" {
		return "", "", errUsage
	}
	return tokenID, priceStr, nil
}

func privateKeyFromEnv() (string, error) {
	for _, k := range []string{"POLYMARKET_PRIVATE_KEY", "METAMASK_KEY"} {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v, nil
		}
	}
	return "", fmt.Errorf("set POLYMARKET_PRIVATE_KEY or METAMASK_KEY")
}
