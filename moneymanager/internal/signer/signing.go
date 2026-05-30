package signer

import (
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/AndochBonin/E3/moneymanager/internal/order"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
	"github.com/shopspring/decimal"
)

const (
	// V2 contract addresses on Polygon.
	ExchangeAddress        = "0xE111180000d2663C0091e4f400237545B87B996B"
	NegRiskExchangeAddress = "0xe2222d279d744050d28e00520010520000310F59"

	ChainID = 137

	ctfExchangeV2DomainName      = "Polymarket CTF Exchange"
	ctfExchangeV2DomainVersion   = "2"
	orderV2TypeString            = "Order(uint256 salt,address maker,address signer,uint256 tokenId,uint256 makerAmount,uint256 takerAmount,uint8 side,uint8 signatureType,uint256 timestamp,bytes32 metadata,bytes32 builder)"
)

var randReadFn = rand.Read

var cryptoSignFn = crypto.Sign

var parseOrderAmountMicroFn = parseOrderAmountMicroDefault

func parseOrderAmountMicroDefault(makerAmount, takerAmount string) (makerAmt, takerAmt *big.Int, err error) {
	makerAmt = new(big.Int)
	if _, ok := makerAmt.SetString(makerAmount, 10); !ok {
		return nil, nil, fmt.Errorf("makerAmount %q", makerAmount)
	}
	takerAmt = new(big.Int)
	if _, ok := takerAmt.SetString(takerAmount, 10); !ok {
		return nil, nil, fmt.Errorf("takerAmount %q", takerAmount)
	}
	return makerAmt, takerAmt, nil
}

var (
	packPoly1271ContentsInputFn = packPoly1271ContentsInputDefault
	packPoly1271AppDomainSepFn  = packPoly1271AppDomainSepDefault
)

var abiNewTypeHook = abi.NewType

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

// BuildOrder constructs and signs an order.Payload ready for POST /order.
// Pass timestampMs from the CLOB client's clock policy so the EIP-712 timestamp
// matches POLY_TIMESTAMP when using server time.
//
// The wrapped EIP-1271 TypedDataSign envelope used here is what the CLOB expects for
// signatureType=3 (POLY_1271). Types 0/1/2 need a different (plain EIP-712) path — not implemented.
func (s *Signer) BuildOrder(
	tokenID string,
	side order.Side,
	price decimal.Decimal,
	size decimal.Decimal,
	negRisk bool,
	expiration int64,
	timestampMs int64,
	depositWallet string,
	signatureType uint8,
) (*order.Payload, error) {
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
	if side == order.SideSell {
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

	innerSig, err := cryptoSignFn(hash, s.privateKey)
	if err != nil {
		return nil, fmt.Errorf("sign order: %w", err)
	}

	innerSig[64] += 27

	tokenIDInt, err := parseAssetIDUint256(tokenID)
	if err != nil {
		return nil, err
	}
	makerAmt, takerAmt, err := parseOrderAmountMicroFn(makerAmount, takerAmount)
	if err != nil {
		return nil, err
	}

	contentsPacked, err := packPoly1271ContentsInputFn(
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

	appPacked, err := packPoly1271AppDomainSepFn(ChainID, exchangeAddr)
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

	return &order.Payload{
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

func packPoly1271AppDomainSepDefault(chainID int, verifyingContract common.Address) ([]byte, error) {
	domainTypeHash := crypto.Keccak256([]byte("EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)"))
	nameHash := crypto.Keccak256([]byte(ctfExchangeV2DomainName))
	versionHash := crypto.Keccak256([]byte(ctfExchangeV2DomainVersion))

	tBytes32, err := abiNewTypeHook("bytes32", "", nil)
	if err != nil {
		return nil, err
	}
	tUint256, err := abiNewTypeHook("uint256", "", nil)
	if err != nil {
		return nil, err
	}
	tAddr, err := abiNewTypeHook("address", "", nil)
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

func packPoly1271ContentsInputDefault(
	orderTypeHash []byte,
	salt int64,
	maker, signer common.Address,
	tokenID, makerAmt, takerAmt *big.Int,
	side, sigType uint8,
	timestampMs int64,
	metadata, builder common.Hash,
) ([]byte, error) {
	tBytes32, err := abiNewTypeHook("bytes32", "", nil)
	if err != nil {
		return nil, err
	}
	tUint256, err := abiNewTypeHook("uint256", "", nil)
	if err != nil {
		return nil, err
	}
	tUint8, err := abiNewTypeHook("uint8", "", nil)
	if err != nil {
		return nil, err
	}
	tAddr, err := abiNewTypeHook("address", "", nil)
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

func stripHexPrefix(s string) string {
	if len(s) >= 2 && s[:2] == "0x" {
		return s[2:]
	}
	return s
}

func randomSalt() (int64, error) {
	var b [8]byte
	if _, err := randReadFn(b[:]); err != nil {
		return 0, fmt.Errorf("random salt: %w", err)
	}
	u := binary.BigEndian.Uint64(b[:])
	return int64(u & 0x7FFFFFFFFFFFFFFF), nil
}
