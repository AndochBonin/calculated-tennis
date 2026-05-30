package clob

import (
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultClobBaseURL = "https://clob.polymarket.com"
const defaultDataAPIBaseURL = "https://data-api.polymarket.com"

type Client struct {
	http       *http.Client
	baseURL    string
	apiKey     string
	secret     string
	passphrase string
	address    string
	// POLYMARKET_USER_ADDRESS — query param user for GET /positions (data API).
	userAddress string
	// POLYMARKET_DATA_API_BASE_URL — host for GET /positions (default defaultDataAPIBaseURL).
	dataAPIBaseURL string
	// When true, L2 timestamps use GET /time (same idea as TS ClobClient useServerTime).
	useServerTime bool
	// POLYMARKET_DEPOSIT_WALLET (fallback DEPOSIT_WALLET) — Safe / deposit address for EIP-712 TypedDataSign.
	depositWallet string
}

type Option func(*Client)

func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		if baseURL != "" {
			c.baseURL = baseURL
		}
	}
}

func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		if httpClient != nil {
			c.http = httpClient
		}
	}
}

func WithTransport(transport http.RoundTripper) Option {
	return func(c *Client) {
		if transport != nil {
			c.http.Transport = transport
		}
	}
}

// WithAddress sets POLY_ADDRESS for L2 auth. Prefer the signer EOA address that owns
// the apiKey/secret/passphrase (matches the wallet used to derive-api-key).
func WithAddress(address string) Option {
	return func(c *Client) {
		if trimmed := strings.TrimSpace(address); trimmed != "" {
			c.address = trimmed
		}
	}
}

// WithServerSignedTime uses CLOB GET /time for POLY_TIMESTAMP (TS client useServerTime).
func WithServerSignedTime(enabled bool) Option {
	return func(c *Client) {
		c.useServerTime = enabled
	}
}

// WithDataAPIBaseURL sets the base URL for GET /positions (POLYMARKET_DATA_API_BASE_URL).
func WithDataAPIBaseURL(baseURL string) Option {
	return func(c *Client) {
		if trimmed := strings.TrimSpace(baseURL); trimmed != "" {
			c.dataAPIBaseURL = trimmed
		}
	}
}

// WithUserAddress sets the data API positions ?user= query (POLYMARKET_USER_ADDRESS).
func WithUserAddress(addr string) Option {
	return func(c *Client) {
		if trimmed := strings.TrimSpace(addr); trimmed != "" {
			c.userAddress = trimmed
		}
	}
}

// WithDepositWallet sets the deposit / Safe address used for order signing (POLYMARKET_DEPOSIT_WALLET).
func WithDepositWallet(addr string) Option {
	return func(c *Client) {
		if trimmed := strings.TrimSpace(addr); trimmed != "" {
			c.depositWallet = trimmed
		}
	}
}

func NewClient(opts ...Option) *Client {
	c := &Client{
		http: &http.Client{},
	}

	baseURL := strings.TrimSpace(os.Getenv("POLYMARKET_CLOB_BASE_URL"))
	if baseURL == "" {
		baseURL = defaultClobBaseURL
	}
	c.baseURL = baseURL
	c.apiKey = strings.TrimSpace(os.Getenv("POLYMARKET_API_KEY"))
	c.secret = strings.TrimSpace(os.Getenv("POLYMARKET_API_SECRET"))
	c.passphrase = strings.TrimSpace(os.Getenv("POLYMARKET_PASSPHRASE"))
	c.address = strings.TrimSpace(os.Getenv("POLYMARKET_ADDRESS"))
	c.userAddress = strings.TrimSpace(os.Getenv("POLYMARKET_USER_ADDRESS"))
	c.dataAPIBaseURL = strings.TrimSpace(os.Getenv("POLYMARKET_DATA_API_BASE_URL"))
	if dw := strings.TrimSpace(os.Getenv("POLYMARKET_DEPOSIT_WALLET")); dw != "" {
		c.depositWallet = dw
	} else if dw := strings.TrimSpace(os.Getenv("DEPOSIT_WALLET")); dw != "" {
		c.depositWallet = dw
	}

	switch strings.ToLower(strings.TrimSpace(os.Getenv("POLYMARKET_CLOB_SERVER_TIME"))) {
	case "1", "true", "yes":
		c.useServerTime = true
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// AuthAddress returns the address sent as POLY_ADDRESS on L2 requests (expected API key owner).
func (c *Client) AuthAddress() string {
	return c.address
}

// DepositWallet returns the configured deposit / Safe address used for order maker and EIP-712 domain.
func (c *Client) DepositWallet() string {
	return c.depositWallet
}

func (c *Client) fetchClobServerUnix() int64 {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/time", nil)
	if err != nil {
		return time.Now().Unix()
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return time.Now().Unix()
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return time.Now().Unix()
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 64))
	if err != nil {
		return time.Now().Unix()
	}
	ts, err := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
	if err != nil {
		return time.Now().Unix()
	}
	return ts
}

// orderMessageTimestampMillis returns milliseconds for EIP-712 order timestamps.
// When useServerTime is true, it uses GET /time (seconds) scaled to ms; /time is
// second resolution so the value is coarse but valid for uniqueness alongside salt.
func (c *Client) orderMessageTimestampMillis() int64 {
	if c.useServerTime {
		return c.fetchClobServerUnix() * 1000
	}
	return time.Now().UnixMilli()
}

func (c *Client) addAuthHeaders(req *http.Request, body string) {
	ts := time.Now().Unix()
	if c.useServerTime {
		ts = c.fetchClobServerUnix()
	}
	timestamp := strconv.FormatInt(ts, 10)
	sig := c.hmacSignature(timestamp, req.Method, req.URL.Path, body)

	req.Header.Set("POLY_ADDRESS", c.address)
	req.Header.Set("POLY_API_KEY", c.apiKey)
	req.Header.Set("POLY_PASSPHRASE", c.passphrase)
	req.Header.Set("POLY_SIGNATURE", sig)
	req.Header.Set("POLY_TIMESTAMP", timestamp)
}
