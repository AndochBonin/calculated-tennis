package order

// Side is the direction of a signed CLOB order.
type Side string

const (
	SideBuy  Side = "BUY"
	SideSell Side = "SELL"
)

// Payload is a signed order ready for POST /order.
type Payload struct {
	Maker         string
	Signer        string
	TokenID       string
	MakerAmount   string
	TakerAmount   string
	Side          Side
	Expiration    string
	Timestamp     string
	Metadata      string
	Builder       string
	Signature     string
	Salt          int64
	SignatureType int
}
