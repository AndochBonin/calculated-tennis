package signer

import (
	"crypto/ecdsa"
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/AndochBonin/E3/moneymanager/pkg/order"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/shopspring/decimal"
)

const testPrivKeyHexValid = "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"

const testEOAAddressExpected = "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"

const testDepositWallet = "0x70997970C51812dc3A010C7d01b50e0d17dc79C8"

func TestNewSigner_invalidHex(t *testing.T) {
	_, err := NewSigner("not-hex")
	if err == nil || !strings.Contains(err.Error(), "parse private key") {
		t.Fatalf("NewSigner: got err=%v", err)
	}
}

func TestNewSigner_invalidKeyLength(t *testing.T) {
	_, err := NewSigner("0x01")
	if err == nil || !strings.Contains(err.Error(), "parse private key") {
		t.Fatalf("NewSigner short key: got err=%v", err)
	}
}

func TestNewSigner_validWithAndWithoutPrefix(t *testing.T) {
	keyNoPrefix := strings.TrimPrefix(testPrivKeyHexValid, "0x")
	s0x, err := NewSigner(testPrivKeyHexValid)
	if err != nil {
		t.Fatalf("NewSigner with 0x: %v", err)
	}
	sPlain, err := NewSigner(keyNoPrefix)
	if err != nil {
		t.Fatalf("NewSigner without 0x: %v", err)
	}
	if !strings.EqualFold(s0x.Address(), sPlain.Address()) {
		t.Fatalf("address mismatch: %q vs %q", s0x.Address(), sPlain.Address())
	}
	if !strings.EqualFold(s0x.Address(), testEOAAddressExpected) {
		t.Fatalf("Address: got %q want %q", s0x.Address(), testEOAAddressExpected)
	}
}

func TestSigner_Address_hexChecksum(t *testing.T) {
	s, err := NewSigner(testPrivKeyHexValid)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	got := s.Address()
	want := common.HexToAddress(testEOAAddressExpected).Hex()
	if got != want {
		t.Fatalf("Address: got %q want %q", got, want)
	}
}

func TestBuildOrder_buyAndSell_makerTakerAmounts(t *testing.T) {
	s, err := NewSigner(testPrivKeyHexValid)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}

	price := decimal.RequireFromString("0.5")
	size := decimal.RequireFromString("10")
	expiration := int64(1_700_000_000)
	ts := int64(1_700_000_123)
	deposit := common.HexToAddress(testDepositWallet).Hex()

	t.Run("buy", func(t *testing.T) {
		p, err := s.BuildOrder(
			"12345",
			order.SideBuy,
			price,
			size,
			false,
			expiration,
			ts,
			deposit,
			3,
		)
		if err != nil {
			t.Fatalf("BuildOrder: %v", err)
		}
		if p.Side != order.SideBuy {
			t.Fatalf("Side: got %q", p.Side)
		}
		if p.MakerAmount != "5000000" || p.TakerAmount != "10000000" {
			t.Fatalf("amounts: maker=%q taker=%q", p.MakerAmount, p.TakerAmount)
		}
	})

	t.Run("sell", func(t *testing.T) {
		p, err := s.BuildOrder(
			"12345",
			order.SideSell,
			price,
			size,
			false,
			expiration,
			ts,
			deposit,
			3,
		)
		if err != nil {
			t.Fatalf("BuildOrder: %v", err)
		}
		if p.Side != order.SideSell {
			t.Fatalf("Side: got %q", p.Side)
		}
		if p.MakerAmount != "10000000" || p.TakerAmount != "5000000" {
			t.Fatalf("amounts: maker=%q taker=%q", p.MakerAmount, p.TakerAmount)
		}
	})
}

func TestBuildOrder_negRisk_usesNegRiskExchange(t *testing.T) {
	s, err := NewSigner(testPrivKeyHexValid)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	price := decimal.RequireFromString("0.5")
	size := decimal.RequireFromString("10")
	deposit := common.HexToAddress(testDepositWallet).Hex()

	pRiskOff, err := s.BuildOrder("999", order.SideBuy, price, size, false, 1, 2, deposit, 3)
	if err != nil {
		t.Fatalf("BuildOrder negRisk=false: %v", err)
	}
	pRiskOn, err := s.BuildOrder("999", order.SideBuy, price, size, true, 1, 2, deposit, 3)
	if err != nil {
		t.Fatalf("BuildOrder negRisk=true: %v", err)
	}
	if pRiskOff.Signature == pRiskOn.Signature {
		t.Fatal("expected different signatures for standard vs neg-risk exchange domain")
	}
}

func TestBuildOrder_signatureType_signerField(t *testing.T) {
	s, err := NewSigner(testPrivKeyHexValid)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	price := decimal.RequireFromString("0.5")
	size := decimal.RequireFromString("2")
	deposit := common.HexToAddress(testDepositWallet).Hex()
	eoa := common.HexToAddress(s.Address()).Hex()

	t.Run("type3_signerIsDeposit", func(t *testing.T) {
		p, err := s.BuildOrder("42", order.SideBuy, price, size, false, 1, 2, deposit, 3)
		if err != nil {
			t.Fatalf("BuildOrder: %v", err)
		}
		if p.Signer != deposit {
			t.Fatalf("Signer: got %q want deposit %q", p.Signer, deposit)
		}
		if p.SignatureType != 3 {
			t.Fatalf("SignatureType: got %d", p.SignatureType)
		}
	})

	t.Run("type0_signerIsEOA", func(t *testing.T) {
		p, err := s.BuildOrder("42", order.SideBuy, price, size, false, 1, 2, deposit, 0)
		if err != nil {
			t.Fatalf("BuildOrder: %v", err)
		}
		if p.Signer != eoa {
			t.Fatalf("Signer: got %q want EOA %q", p.Signer, eoa)
		}
		if p.SignatureType != 0 {
			t.Fatalf("SignatureType: got %d", p.SignatureType)
		}
	})
}

func TestBuildOrder_tokenID_parseErrors(t *testing.T) {
	s, err := NewSigner(testPrivKeyHexValid)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	deposit := common.HexToAddress(testDepositWallet).Hex()
	price := decimal.RequireFromString("0.5")
	size := decimal.RequireFromString("1")

	tests := []struct {
		name    string
		tokenID string
		wantSub string
	}{
		{"empty_after_signing_path", "", "empty token id"},
		{"whitespace_only", "  \t ", "hash typed data"},
		{"non_numeric_decimal", "not-digits", "hash typed data"},
		{"invalid_hex", "0xgg", "hash typed data"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.BuildOrder(tt.tokenID, order.SideBuy, price, size, false, 1, 2, deposit, 3)
			if err == nil || !strings.Contains(err.Error(), tt.wantSub) {
				t.Fatalf("tokenID=%q: got err=%v want substring %q", tt.tokenID, err, tt.wantSub)
			}
		})
	}
}

func TestBuildOrder_tokenID_decimalAndHex_ok(t *testing.T) {
	s, err := NewSigner(testPrivKeyHexValid)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	deposit := common.HexToAddress(testDepositWallet).Hex()
	price := decimal.RequireFromString("1")
	size := decimal.RequireFromString("1")

	for _, tokenID := range []string{"123456789", "99", "0xff"} {
		t.Run(tokenID, func(t *testing.T) {
			p, err := s.BuildOrder(tokenID, order.SideBuy, price, size, false, 9, 9, deposit, 3)
			if err != nil {
				t.Fatalf("BuildOrder: %v", err)
			}
			if p.TokenID != tokenID {
				t.Fatalf("TokenID echo: got %q", p.TokenID)
			}
			if !strings.HasPrefix(p.Signature, "0x") {
				prefixLen := 4
				if len(p.Signature) < prefixLen {
					prefixLen = len(p.Signature)
				}
				t.Fatalf("Signature prefix: got %q", p.Signature[:prefixLen])
			}
		})
	}
}

func TestBuildOrder_randomSalt_randReadFails(t *testing.T) {
	s, err := NewSigner(testPrivKeyHexValid)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	deposit := common.HexToAddress(testDepositWallet).Hex()
	prev := randReadFn
	randReadFn = func([]byte) (int, error) {
		return 0, errors.New("boom")
	}
	t.Cleanup(func() {
		randReadFn = prev
	})

	_, err = s.BuildOrder(
		"1",
		order.SideBuy,
		decimal.RequireFromString("1"),
		decimal.RequireFromString("1"),
		false,
		1,
		2,
		deposit,
		3,
	)
	if err == nil || !strings.Contains(err.Error(), "random salt") {
		t.Fatalf("BuildOrder with failing rand.Read: got err=%v", err)
	}
}

func TestBuildOrder_payload_fields(t *testing.T) {
	s, err := NewSigner(testPrivKeyHexValid)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	deposit := common.HexToAddress(testDepositWallet).Hex()
	expiration := int64(999888777)
	ts := int64(888777666)

	p, err := s.BuildOrder(
		"7",
		order.SideBuy,
		decimal.RequireFromString("1"),
		decimal.RequireFromString("1"),
		false,
		expiration,
		ts,
		deposit,
		3,
	)
	if err != nil {
		t.Fatalf("BuildOrder: %v", err)
	}
	if p.Maker != deposit {
		t.Fatalf("Maker: got %q", p.Maker)
	}
	if p.Expiration != "999888777" || p.Timestamp != "888777666" {
		t.Fatalf("Expiration/Timestamp: %#v", p)
	}
	meta := "0x0000000000000000000000000000000000000000000000000000000000000000"
	if p.Metadata != meta || p.Builder != meta {
		t.Fatalf("metadata/builder: %#v", p)
	}
	if p.Salt == 0 {
		t.Fatal("expected non-zero salt")
	}
}

func TestParseAssetIDUint256_uppercaseHexPrefix(t *testing.T) {
	n, err := parseAssetIDUint256("0Xff")
	if err != nil {
		t.Fatalf("parseAssetIDUint256: %v", err)
	}
	if n.Cmp(big.NewInt(255)) != 0 {
		t.Fatalf("value: got %s", n.String())
	}
}

func TestParseAssetIDUint256_invalidDigits(t *testing.T) {
	_, err := parseAssetIDUint256("12abc")
	if err == nil || !strings.Contains(err.Error(), "parse token id") {
		t.Fatalf("want parse token id error, got %v", err)
	}
}

func TestParseOrderAmountMicroDefault_invalidMaker(t *testing.T) {
	_, _, err := parseOrderAmountMicroDefault("not-base10", "1")
	if err == nil || !strings.Contains(err.Error(), `makerAmount "not-base10"`) {
		t.Fatalf("got err=%v", err)
	}
}

func TestParseOrderAmountMicroDefault_invalidTaker(t *testing.T) {
	_, _, err := parseOrderAmountMicroDefault("1", "bad")
	if err == nil || !strings.Contains(err.Error(), `takerAmount "bad"`) {
		t.Fatalf("got err=%v", err)
	}
}

func TestBuildOrder_cryptoSignFails(t *testing.T) {
	s, err := NewSigner(testPrivKeyHexValid)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	deposit := common.HexToAddress(testDepositWallet).Hex()

	prev := cryptoSignFn
	cryptoSignFn = func(hash []byte, prv *ecdsa.PrivateKey) ([]byte, error) {
		return nil, errors.New("sign boom")
	}
	t.Cleanup(func() { cryptoSignFn = prev })

	_, err = s.BuildOrder(
		"5",
		order.SideBuy,
		decimal.RequireFromString("1"),
		decimal.RequireFromString("1"),
		false,
		1,
		2,
		deposit,
		3,
	)
	if err == nil || !strings.Contains(err.Error(), "sign order") || !strings.Contains(err.Error(), "sign boom") {
		t.Fatalf("BuildOrder: got err=%v", err)
	}
}

func TestBuildOrder_packPoly1271ContentsFails(t *testing.T) {
	s, err := NewSigner(testPrivKeyHexValid)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	deposit := common.HexToAddress(testDepositWallet).Hex()

	prev := packPoly1271ContentsInputFn
	packPoly1271ContentsInputFn = func(
		orderTypeHash []byte,
		salt int64,
		maker, signer common.Address,
		tokenID, makerAmt, takerAmt *big.Int,
		side, sigType uint8,
		timestampMs int64,
		metadata, builder common.Hash,
	) ([]byte, error) {
		return nil, errors.New("contents pack boom")
	}
	t.Cleanup(func() { packPoly1271ContentsInputFn = prev })

	_, err = s.BuildOrder(
		"6",
		order.SideBuy,
		decimal.RequireFromString("1"),
		decimal.RequireFromString("1"),
		false,
		1,
		2,
		deposit,
		3,
	)
	if err == nil || !strings.Contains(err.Error(), "pack poly1271 contents") {
		t.Fatalf("BuildOrder: got err=%v", err)
	}
}

func TestBuildOrder_packPoly1271AppDomainSepFails(t *testing.T) {
	s, err := NewSigner(testPrivKeyHexValid)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	deposit := common.HexToAddress(testDepositWallet).Hex()

	prev := packPoly1271AppDomainSepFn
	packPoly1271AppDomainSepFn = func(chainID int, verifyingContract common.Address) ([]byte, error) {
		return nil, errors.New("app sep boom")
	}
	t.Cleanup(func() { packPoly1271AppDomainSepFn = prev })

	_, err = s.BuildOrder(
		"8",
		order.SideBuy,
		decimal.RequireFromString("1"),
		decimal.RequireFromString("1"),
		false,
		1,
		2,
		deposit,
		3,
	)
	if err == nil || !strings.Contains(err.Error(), "pack app domain sep") {
		t.Fatalf("BuildOrder: got err=%v", err)
	}
}

func TestPackPoly1271AppDomainSepDefault_abiNewTypeFailures(t *testing.T) {
	prevHook := abiNewTypeHook
	t.Cleanup(func() { abiNewTypeHook = prevHook })

	addr := common.HexToAddress("0x1111111111111111111111111111111111111111")

	for failOn := 1; failOn <= 3; failOn++ {
		call := 0
		abiNewTypeHook = func(typ string, internal string, components []abi.ArgumentMarshaling) (abi.Type, error) {
			call++
			if call == failOn {
				return abi.Type{}, errors.New("abi boom")
			}
			return prevHook(typ, internal, components)
		}
		_, err := packPoly1271AppDomainSepDefault(137, addr)
		if err == nil || err.Error() != "abi boom" {
			t.Fatalf("failOn=%d: got err=%v", failOn, err)
		}
	}
}

func TestPackPoly1271ContentsInputDefault_abiNewTypeFailures(t *testing.T) {
	prevHook := abiNewTypeHook
	t.Cleanup(func() { abiNewTypeHook = prevHook })

	hash := make([]byte, 32)
	addr := common.HexToAddress("0x2222222222222222222222222222222222222222")
	tok := big.NewInt(1)

	for failOn := 1; failOn <= 4; failOn++ {
		call := 0
		abiNewTypeHook = func(typ string, internal string, components []abi.ArgumentMarshaling) (abi.Type, error) {
			call++
			if call == failOn {
				return abi.Type{}, errors.New("abi boom")
			}
			return prevHook(typ, internal, components)
		}
		_, err := packPoly1271ContentsInputDefault(hash, 1, addr, addr, tok, tok, tok, 0, 3, 9, common.Hash{}, common.Hash{})
		if err == nil || err.Error() != "abi boom" {
			t.Fatalf("failOn=%d: got err=%v", failOn, err)
		}
	}
}

func TestBuildOrder_parseOrderAmountMicroStub_makerFails(t *testing.T) {
	s, err := NewSigner(testPrivKeyHexValid)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	deposit := common.HexToAddress(testDepositWallet).Hex()

	prev := parseOrderAmountMicroFn
	parseOrderAmountMicroFn = func(_, _ string) (*big.Int, *big.Int, error) {
		return nil, nil, errors.New(`makerAmount "bad"`)
	}
	t.Cleanup(func() { parseOrderAmountMicroFn = prev })

	_, err = s.BuildOrder(
		"4",
		order.SideBuy,
		decimal.RequireFromString("1"),
		decimal.RequireFromString("1"),
		false,
		1,
		2,
		deposit,
		3,
	)
	if err == nil || !strings.Contains(err.Error(), `makerAmount "bad"`) {
		t.Fatalf("BuildOrder: got err=%v", err)
	}
}

func TestBuildOrder_parseOrderAmountMicroStub_takerFails(t *testing.T) {
	s, err := NewSigner(testPrivKeyHexValid)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	deposit := common.HexToAddress(testDepositWallet).Hex()

	prev := parseOrderAmountMicroFn
	parseOrderAmountMicroFn = func(_, _ string) (*big.Int, *big.Int, error) {
		return nil, nil, errors.New(`takerAmount "bad"`)
	}
	t.Cleanup(func() { parseOrderAmountMicroFn = prev })

	_, err = s.BuildOrder(
		"4",
		order.SideBuy,
		decimal.RequireFromString("1"),
		decimal.RequireFromString("1"),
		false,
		1,
		2,
		deposit,
		3,
	)
	if err == nil || !strings.Contains(err.Error(), `takerAmount "bad"`) {
		t.Fatalf("BuildOrder: got err=%v", err)
	}
}
