package balance

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/shopspring/decimal"
)

// erc20BalanceOfABI is the minimal IERC20 balanceOf selector.
const erc20BalanceOfABI = `[{"constant":true,"inputs":[{"name":"account","type":"address"}],"name":"balanceOf","outputs":[{"name":"","type":"uint256"}],"payable":false,"stateMutability":"view","type":"function"}]`

var balanceOfABI abi.ABI

func init() {
	parsed, err := abi.JSON(strings.NewReader(erc20BalanceOfABI))
	if err != nil {
		panic("balance: parse ERC20 ABI: " + err.Error())
	}
	balanceOfABI = parsed
}

// contractCaller is the subset of ethclient used for balance reads (mockable in tests).
type contractCaller interface {
	CallContract(ctx context.Context, call ethereum.CallMsg, blockNumber *big.Int) ([]byte, error)
	ChainID(ctx context.Context) (*big.Int, error)
}

// Reader fetches on-chain ERC-20 balances over JSON-RPC.
type Reader struct {
	client contractCaller
}

// NewReader dials rpcURL and returns a Reader. The caller must call Close when done.
func NewReader(rpcURL string) (*Reader, error) {
	rpcURL = strings.TrimSpace(rpcURL)
	if rpcURL == "" {
		return nil, fmt.Errorf("balance: empty RPC URL")
	}
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("balance: dial RPC: %w", err)
	}
	return &Reader{client: client}, nil
}

// NewReaderWithClient is for tests and custom RPC wiring.
func NewReaderWithClient(client contractCaller) *Reader {
	return &Reader{client: client}
}

// Close releases the underlying RPC connection when backed by *ethclient.Client.
func (r *Reader) Close() {
	if c, ok := r.client.(*ethclient.Client); ok {
		c.Close()
	}
}

// ChainID returns the chain ID reported by the RPC endpoint.
func (r *Reader) ChainID(ctx context.Context) (*big.Int, error) {
	id, err := r.client.ChainID(ctx)
	if err != nil {
		return nil, fmt.Errorf("balance: chain id: %w", err)
	}
	return id, nil
}

// BalanceOf returns the raw token balance (smallest units) for wallet at the latest block.
func (r *Reader) BalanceOf(ctx context.Context, token, wallet common.Address) (*big.Int, error) {
	data, err := balanceOfABI.Pack("balanceOf", wallet)
	if err != nil {
		return nil, fmt.Errorf("balance: pack balanceOf: %w", err)
	}
	out, err := r.client.CallContract(ctx, ethereum.CallMsg{
		To:   &token,
		Data: data,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("balance: call balanceOf: %w", err)
	}
	vals, err := balanceOfABI.Unpack("balanceOf", out)
	if err != nil {
		return nil, fmt.Errorf("balance: unpack balanceOf: %w", err)
	}
	if len(vals) != 1 {
		return nil, fmt.Errorf("balance: unexpected balanceOf return count %d", len(vals))
	}
	amount, ok := vals[0].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("balance: balanceOf return type %T", vals[0])
	}
	return amount, nil
}

// FormatUSDC formats a 6-decimal USDC amount for display.
func FormatUSDC(raw *big.Int) string {
	if raw == nil {
		return "0"
	}
	return decimal.NewFromBigInt(raw, -USDCDecimals).StringFixed(USDCDecimals)
}

// DepositWalletFromEnv resolves POLYMARKET_DEPOSIT_WALLET with DEPOSIT_WALLET fallback.
func DepositWalletFromEnv() (common.Address, error) {
	wallet := strings.TrimSpace(os.Getenv("POLYMARKET_DEPOSIT_WALLET"))
	if wallet == "" {
		wallet = strings.TrimSpace(os.Getenv("DEPOSIT_WALLET"))
	}
	if wallet == "" {
		return common.Address{}, fmt.Errorf("deposit wallet not set (POLYMARKET_DEPOSIT_WALLET or DEPOSIT_WALLET)")
	}
	if !common.IsHexAddress(wallet) {
		return common.Address{}, fmt.Errorf("invalid deposit wallet address %q", wallet)
	}
	return common.HexToAddress(wallet), nil
}

// DefaultPolygonRPCURL is used when POLYGON_RPC_URL is unset (e.g. missing .env).
// Override via moneymanager/.env — see .env.example.
const DefaultPolygonRPCURL = "https://polygon.drpc.org"

// RPCURLFromEnv returns POLYGON_RPC_URL from the environment, or DefaultPolygonRPCURL.
func RPCURLFromEnv() (string, error) {
	if url := strings.TrimSpace(os.Getenv("POLYGON_RPC_URL")); url != "" {
		return url, nil
	}
	return DefaultPolygonRPCURL, nil
}
