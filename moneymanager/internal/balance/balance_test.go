package balance

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
)

type stubCaller struct {
	chainID *big.Int
	callFn  func(ctx context.Context, call ethereum.CallMsg, blockNumber *big.Int) ([]byte, error)
}

func (s *stubCaller) CallContract(ctx context.Context, call ethereum.CallMsg, blockNumber *big.Int) ([]byte, error) {
	return s.callFn(ctx, call, blockNumber)
}

func (s *stubCaller) ChainID(ctx context.Context) (*big.Int, error) {
	return s.chainID, nil
}

func TestBalanceOf_unpacksReturn(t *testing.T) {
	wallet := common.HexToAddress("0x0000000000000000000000000000000000000001")
	want := big.NewInt(1_500_000) // 1.5 USDC

	data, err := balanceOfABI.Pack("balanceOf", wallet)
	if err != nil {
		t.Fatal(err)
	}
	ret, err := balanceOfABI.Methods["balanceOf"].Outputs.Pack(want)
	if err != nil {
		t.Fatal(err)
	}

	r := NewReaderWithClient(&stubCaller{
		chainID: big.NewInt(PolygonChainID),
		callFn: func(ctx context.Context, call ethereum.CallMsg, blockNumber *big.Int) ([]byte, error) {
			if string(call.Data) != string(data) {
				t.Fatalf("unexpected calldata")
			}
			return ret, nil
		},
	})

	got, err := r.BalanceOf(context.Background(), BridgedUSDC, wallet)
	if err != nil {
		t.Fatal(err)
	}
	if got.Cmp(want) != 0 {
		t.Fatalf("balance: got %s want %s", got, want)
	}
}

func TestUSDCTotal_sumsBridgedAndNative(t *testing.T) {
	wallet := common.HexToAddress("0x0000000000000000000000000000000000000001")
	bridged := big.NewInt(1_000_000)
	native := big.NewInt(500_000)

	bridgedData, err := balanceOfABI.Pack("balanceOf", wallet)
	if err != nil {
		t.Fatal(err)
	}
	nativeData, err := balanceOfABI.Pack("balanceOf", wallet)
	if err != nil {
		t.Fatal(err)
	}
	bridgedRet, err := balanceOfABI.Methods["balanceOf"].Outputs.Pack(bridged)
	if err != nil {
		t.Fatal(err)
	}
	nativeRet, err := balanceOfABI.Methods["balanceOf"].Outputs.Pack(native)
	if err != nil {
		t.Fatal(err)
	}

	r := NewReaderWithClient(&stubCaller{
		chainID: big.NewInt(PolygonChainID),
		callFn: func(ctx context.Context, call ethereum.CallMsg, blockNumber *big.Int) ([]byte, error) {
			switch {
			case call.To != nil && *call.To == BridgedUSDC && string(call.Data) == string(bridgedData):
				return bridgedRet, nil
			case call.To != nil && *call.To == NativeUSDC && string(call.Data) == string(nativeData):
				return nativeRet, nil
			default:
				t.Fatalf("unexpected CallContract to=%v", call.To)
				return nil, nil
			}
		},
	})

	got, err := r.USDCTotal(context.Background(), wallet)
	if err != nil {
		t.Fatal(err)
	}
	want := big.NewInt(1_500_000)
	if got.Cmp(want) != 0 {
		t.Fatalf("USDCTotal: got %s want %s", got, want)
	}
}

func TestFormatUSDC(t *testing.T) {
	tests := []struct {
		raw  int64
		want string
	}{
		{0, "0.000000"},
		{1_000_000, "1.000000"},
		{1_500_000, "1.500000"},
	}
	for _, tc := range tests {
		got := FormatUSDC(big.NewInt(tc.raw))
		if got != tc.want {
			t.Errorf("FormatUSDC(%d) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

func TestDepositWalletFromEnv(t *testing.T) {
	t.Setenv("POLYMARKET_DEPOSIT_WALLET", "0xAbCdefabcdefABCDEFabcdefABCDEFabcdefABcD")
	t.Setenv("DEPOSIT_WALLET", "0x0000000000000000000000000000000000000002")
	addr, err := DepositWalletFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if addr != common.HexToAddress("0xAbCdefabcdefABCDEFabcdefABCDEFabcdefABcD") {
		t.Fatalf("got %s", addr)
	}
}

func TestDepositWalletFromEnv_fallback(t *testing.T) {
	t.Setenv("POLYMARKET_DEPOSIT_WALLET", "")
	t.Setenv("DEPOSIT_WALLET", "0x0000000000000000000000000000000000000002")
	addr, err := DepositWalletFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if addr != common.HexToAddress("0x0000000000000000000000000000000000000002") {
		t.Fatalf("got %s", addr)
	}
}

func TestDepositWalletFromEnv_missing(t *testing.T) {
	t.Setenv("POLYMARKET_DEPOSIT_WALLET", "")
	t.Setenv("DEPOSIT_WALLET", "")
	_, err := DepositWalletFromEnv()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRPCURLFromEnv(t *testing.T) {
	t.Setenv("POLYGON_RPC_URL", "https://polygon.example")
	url, err := RPCURLFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if url != "https://polygon.example" {
		t.Fatalf("got %q", url)
	}
}

func TestRPCURLFromEnv_defaultWhenUnset(t *testing.T) {
	t.Setenv("POLYGON_RPC_URL", "")
	url, err := RPCURLFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if url != DefaultPolygonRPCURL {
		t.Fatalf("got %q, want %q", url, DefaultPolygonRPCURL)
	}
}
