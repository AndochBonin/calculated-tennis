package clob

import (
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
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

	ctfExchangeV2DomainName    = "Polymarket CTF Exchange"
	ctfExchangeV2DomainVersion = "2"

	// Must match clob-client-v2 exchangeOrderBuilderV2 ORDER_TYPE_STRING / contentsHash encoding.
	orderV2TypeString = "Order(uint256 salt,address maker,address signer,uint256 tokenId,uint256 makerAmount,uint256 takerAmount,uint8 side,uint8 signatureType,uint256 timestamp,bytes32 metadata,bytes32 builder)"
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
//
// The wrapped EIP-1271 TypedDataSign envelope used here is what the CLOB expects for
// signatureType=3 (POLY_1271). Types 0/1/2 need a different (plain EIP-712) path — not implemented.
func (s *Signer) BuildOrder(
	tokenID string,
	side models.OrderSide,
	price decimal.Decimal,
	size decimal.Decimal,
	negRisk bool,
	expiration int64,
	timestampMs int64,
	depositWallet string,
	signatureType uint8,
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

	depositWalletAddr := common.HexToAddress(depositWallet)
	exchangeAddr := common.HexToAddress(contractAddress)

	bytes32Zero := common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000")

	orderStruct := apitypes.Types{
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
	}

	// POLY_1271 (signatureType 3): maker and order "signer" are the funder / deposit wallet.
	// This matches py_clob_client_v2 OrderBuilder._v2_order_signer() (signer=funder for POLY_1271).
	// The EOA still signs the outer TypedDataSign digest (s.privateKey); the CLOB ties the API key to funder.
	// Use *big.Int for ints so apitypes matches viem's numeric EIP-712 encoding (uint8/uint256).
	orderSignerAddr := s.address
	if signatureType == 3 {
		orderSignerAddr = depositWalletAddr
	}
	orderMessage := apitypes.TypedDataMessage{
		"salt":            fmt.Sprintf("%d", salt),
		"maker":           depositWalletAddr.Hex(),
		"signer":          orderSignerAddr.Hex(),
		"tokenId":         tokenID,
		"makerAmount":     makerAmount,
		"takerAmount":     takerAmount,
		"side":            big.NewInt(int64(sideInt)),
		"signatureType":   big.NewInt(int64(signatureType)),
		"timestamp":       fmt.Sprintf("%d", timestampMs),
		"metadata":        bytes32Zero.Hex(),
		"builder":         bytes32Zero.Hex(),
	}

	typedData := apitypes.TypedData{
		Types: apitypes.Types{
			"EIP712Domain": {
				{Name: "name", Type: "string"},
				{Name: "version", Type: "string"},
				{Name: "chainId", Type: "uint256"},
				{Name: "verifyingContract", Type: "address"},
			},
			"TypedDataSign": {
				{Name: "contents", Type: "Order"},
				{Name: "name", Type: "string"},
				{Name: "version", Type: "string"},
				{Name: "chainId", Type: "uint256"},
				{Name: "verifyingContract", Type: "address"},
				{Name: "salt", Type: "bytes32"},
			},
			"Order": orderStruct["Order"],
		},
		PrimaryType: "TypedDataSign",
		Domain: apitypes.TypedDataDomain{
			Name:              ctfExchangeV2DomainName,
			Version:           ctfExchangeV2DomainVersion,
			ChainId:           (*math.HexOrDecimal256)(big.NewInt(ChainID)),
			VerifyingContract: exchangeAddr.Hex(),
		},
		Message: apitypes.TypedDataMessage{
			"contents":          orderMessage,
			"name":              "DepositWallet",
			"version":           "1",
			"chainId":           big.NewInt(int64(ChainID)),
			"verifyingContract": depositWalletAddr.Hex(),
			"salt":              bytes32Zero.Hex(),
		},
	}

	hash, _, err := apitypes.TypedDataAndHash(typedData)
	if err != nil {
		return nil, fmt.Errorf("hash typed data: %w", err)
	}

	innerSig, err := crypto.Sign(hash, s.privateKey)
	if err != nil {
		return nil, fmt.Errorf("sign order: %w", err)
	}

	innerSig[64] += 27

	tokenIDInt, err := parseAssetIDUint256(tokenID)
	if err != nil {
		return nil, err
	}
	makerAmt := new(big.Int)
	if _, ok := makerAmt.SetString(makerAmount, 10); !ok {
		return nil, fmt.Errorf("makerAmount %q", makerAmount)
	}
	takerAmt := new(big.Int)
	if _, ok := takerAmt.SetString(takerAmount, 10); !ok {
		return nil, fmt.Errorf("takerAmount %q", takerAmount)
	}

	contentsPacked, err := packPoly1271ContentsInput(
		crypto.Keccak256([]byte(orderV2TypeString)),
		salt,
		depositWalletAddr,
		orderSignerAddr,
		tokenIDInt,
		makerAmt,
		takerAmt,
		uint8(sideInt),
		signatureType,
		timestampMs,
		bytes32Zero,
		bytes32Zero,
	)
	if err != nil {
		return nil, fmt.Errorf("pack poly1271 contents: %w", err)
	}
	contentsHash := crypto.Keccak256(contentsPacked)

	appPacked, err := packPoly1271AppDomainSep(ChainID, exchangeAddr)
	if err != nil {
		return nil, fmt.Errorf("pack app domain sep: %w", err)
	}
	appDomainSep := crypto.Keccak256(appPacked)

	typeStringBytes := []byte(orderV2TypeString)
	lenHex := fmt.Sprintf("%04x", len(typeStringBytes))

	wireSig := hex.EncodeToString(innerSig) +
		hex.EncodeToString(appDomainSep) +
		hex.EncodeToString(contentsHash) +
		hex.EncodeToString(typeStringBytes) +
		lenHex

	return &models.OrderPayload{
		Maker:         depositWalletAddr.Hex(),
		Signer:        orderSignerAddr.Hex(),
		TokenID:       tokenID,
		MakerAmount:   makerAmount,
		TakerAmount:   takerAmount,
		Side:          side,
		Expiration:    fmt.Sprintf("%d", expiration),
		Timestamp:     fmt.Sprintf("%d", timestampMs),
		Metadata:      bytes32Zero.Hex(),
		Builder:       bytes32Zero.Hex(),
		Signature:     "0x" + wireSig,
		Salt:          salt,
		SignatureType: int(signatureType),
	}, nil
}

func parseAssetIDUint256(s string) (*big.Int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty token id")
	}
	base := 10
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		s = s[2:]
		base = 16
	}
	n := new(big.Int)
	if _, ok := n.SetString(s, base); !ok {
		return nil, fmt.Errorf("parse token id %q", s)
	}
	return n, nil
}

// packPoly1271AppDomainSep matches ExchangeOrderBuilderV2 appDomainSep (keccak256 of abi.encode).
func packPoly1271AppDomainSep(chainID int, verifyingContract common.Address) ([]byte, error) {
	domainTypeHash := crypto.Keccak256([]byte("EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)"))
	nameHash := crypto.Keccak256([]byte(ctfExchangeV2DomainName))
	versionHash := crypto.Keccak256([]byte(ctfExchangeV2DomainVersion))

	tBytes32, err := abi.NewType("bytes32", "", nil)
	if err != nil {
		return nil, err
	}
	tUint256, err := abi.NewType("uint256", "", nil)
	if err != nil {
		return nil, err
	}
	tAddr, err := abi.NewType("address", "", nil)
	if err != nil {
		return nil, err
	}
	args := abi.Arguments{
		{Type: tBytes32},
		{Type: tBytes32},
		{Type: tBytes32},
		{Type: tUint256},
		{Type: tAddr},
	}
	return args.Pack(
		common.BytesToHash(domainTypeHash),
		common.BytesToHash(nameHash),
		common.BytesToHash(versionHash),
		big.NewInt(int64(chainID)),
		verifyingContract,
	)
}

// packPoly1271ContentsInput returns abi.encode(...) input bytes before keccak (contentsHash).
func packPoly1271ContentsInput(
	orderTypeHash []byte,
	salt int64,
	maker, signer common.Address,
	tokenID, makerAmt, takerAmt *big.Int,
	side, sigType uint8,
	timestampMs int64,
	metadata, builder common.Hash,
) ([]byte, error) {
	tBytes32, err := abi.NewType("bytes32", "", nil)
	if err != nil {
		return nil, err
	}
	tUint256, err := abi.NewType("uint256", "", nil)
	if err != nil {
		return nil, err
	}
	tUint8, err := abi.NewType("uint8", "", nil)
	if err != nil {
		return nil, err
	}
	tAddr, err := abi.NewType("address", "", nil)
	if err != nil {
		return nil, err
	}
	args := abi.Arguments{
		{Type: tBytes32},
		{Type: tUint256},
		{Type: tAddr},
		{Type: tAddr},
		{Type: tUint256},
		{Type: tUint256},
		{Type: tUint256},
		{Type: tUint8},
		{Type: tUint8},
		{Type: tUint256},
		{Type: tBytes32},
		{Type: tBytes32},
	}
	return args.Pack(
		common.BytesToHash(orderTypeHash),
		big.NewInt(salt),
		maker,
		signer,
		tokenID,
		makerAmt,
		takerAmt,
		side,
		sigType,
		big.NewInt(timestampMs),
		metadata,
		builder,
	)
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
