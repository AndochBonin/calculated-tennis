package clob

import (
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
	"github.com/shopspring/decimal"

	"github.com/AndochBonin/polymarket/models"
	"github.com/AndochBonin/polymarket/utils"
)

const (
	// V2 contract addresses on Polygon
	ExchangeAddress        = "0xE111180000d2663C0091e4f400237545B87B996B"
	NegRiskExchangeAddress = "0xe2222d279d744050d28e00520010520000310F59"

	ChainID = 137
)

// Signer signs Polymarket orders using EIP-712.
type Signer struct {
	privateKey *ecdsa.PrivateKey
	address    common.Address
}

func NewSigner(privateKeyHex string) (*Signer, error) {
	key, err := crypto.HexToECDSA(stripHexPrefix(privateKeyHex))
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	address := crypto.PubkeyToAddress(key.PublicKey)
	return &Signer{privateKey: key, address: address}, nil
}

func (s *Signer) Address() string {
	return s.address.Hex()
}

// BuildOrder constructs and signs an OrderPayload ready for POST /order.
// Pass timestampMs from Client.orderMessageTimestampMillis (or local time) so the
// EIP-712 timestamp matches POLY_TIMESTAMP policy when using server time.
func (s *Signer) BuildOrder(
	tokenID string,
	side models.OrderSide,
	price decimal.Decimal,
	size decimal.Decimal,
	negRisk bool,
	expiration int64,
	timestampMs int64,
) (*models.OrderPayload, error) {
	makerAmount, takerAmount := computeAmounts(side, price, size)

	salt, err := randomSalt()
	if err != nil {
		return nil, err
	}

	contractAddress := ExchangeAddress
	if negRisk {
		contractAddress = NegRiskExchangeAddress
	}

	sideInt := 0
	if side == models.OrderSideSell {
		sideInt = 1
	}

	typedData := apitypes.TypedData{
		Types: apitypes.Types{
			"EIP712Domain": {
				{Name: "name", Type: "string"},
				{Name: "version", Type: "string"},
				{Name: "chainId", Type: "uint256"},
				{Name: "verifyingContract", Type: "address"},
			},
			"Order": {
				{Name: "salt", Type: "uint256"},
				{Name: "maker", Type: "address"},
				{Name: "signer", Type: "address"},
				{Name: "tokenId", Type: "uint256"},
				{Name: "makerAmount", Type: "uint256"},
				{Name: "takerAmount", Type: "uint256"},
				{Name: "side", Type: "uint8"},
				{Name: "signatureType", Type: "uint8"},
				{Name: "timestamp", Type: "uint256"},
				{Name: "metadata", Type: "bytes32"},
				{Name: "builder", Type: "bytes32"},
			},
		},
		PrimaryType: "Order",
		Domain: apitypes.TypedDataDomain{
			Name:              "Polymarket CTF Exchange",
			Version:           "2",
			ChainId:           (*math.HexOrDecimal256)(big.NewInt(ChainID)),
			VerifyingContract: contractAddress,
		},
		Message: apitypes.TypedDataMessage{
			"salt":          fmt.Sprintf("%d", salt),
			"maker":         s.address.Hex(),
			"signer":        s.address.Hex(),
			"tokenId":       tokenID,
			"makerAmount":   makerAmount,
			"takerAmount":   takerAmount,
			"side":          fmt.Sprintf("%d", sideInt),
			"signatureType": "0", // EOA
			"timestamp":     fmt.Sprintf("%d", timestampMs),
			"metadata":      "0x0000000000000000000000000000000000000000000000000000000000000000",
			"builder":       "0x0000000000000000000000000000000000000000000000000000000000000000",
		},
	}

	hash, _, err := apitypes.TypedDataAndHash(typedData)
	if err != nil {
		return nil, fmt.Errorf("hash typed data: %w", err)
	}

	sig, err := crypto.Sign(hash, s.privateKey)
	if err != nil {
		return nil, fmt.Errorf("sign order: %w", err)
	}

	// adjust recovery id for ethereum
	sig[64] += 27

	return &models.OrderPayload{
		Maker:         s.address.Hex(),
		Signer:        s.address.Hex(),
		TokenID:       tokenID,
		MakerAmount:   makerAmount,
		TakerAmount:   takerAmount,
		Side:          side,
		Expiration:    fmt.Sprintf("%d", expiration),
		Timestamp:     fmt.Sprintf("%d", timestampMs),
		Metadata:      "0x0000000000000000000000000000000000000000000000000000000000000000",
		Builder:       "0x0000000000000000000000000000000000000000000000000000000000000000",
		Signature:     fmt.Sprintf("0x%x", sig),
		Salt:          salt,
		SignatureType: 0,
	}, nil
}

func computeAmounts(side models.OrderSide, price, size decimal.Decimal) (string, string) {
	// Outcome tokens use the same 6-decimal scale as USDC; USDC notionals use utils.USDCToMicro.
	micro := decimal.NewFromInt(1_000_000)
	if side == models.OrderSideBuy {
		// makerAmount = USDC you spend, takerAmount = shares you get
		makerAmount := utils.USDCToMicro(price.Mul(size))
		takerAmount := size.Mul(micro).StringFixed(0)
		return makerAmount, takerAmount
	}
	// makerAmount = shares you give, takerAmount = USDC you get
	makerAmount := size.Mul(micro).StringFixed(0)
	takerAmount := utils.USDCToMicro(price.Mul(size))
	return makerAmount, takerAmount
}

func stripHexPrefix(s string) string {
	if len(s) >= 2 && s[:2] == "0x" {
		return s[2:]
	}
	return s
}

func randomSalt() (int64, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, fmt.Errorf("random salt: %w", err)
	}
	u := binary.BigEndian.Uint64(b[:])
	// Non-negative int63 for API/JSON compatibility.
	return int64(u & 0x7FFFFFFFFFFFFFFF), nil
}
