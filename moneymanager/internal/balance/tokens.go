package balance

import "github.com/ethereum/go-ethereum/common"

const (
	// PolygonChainID is the Polymarket / CLOB signing chain.
	PolygonChainID = 137

	// USDCDecimals is the decimal precision for Polygon USDC (bridged and native).
	USDCDecimals = 6
)

// Polygon USDC collateral tokens (Polymarket uses bridged USDC).
var (
	BridgedUSDC = common.HexToAddress("0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174")
	NativeUSDC  = common.HexToAddress("0x3c499c542cEF5E3811e1192ce70d8cC03d5c3359")
)
