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

	switch strings.ToLower(strings.TrimSpace(os.Getenv("POLYMARKET_CLOB_SERVER_TIME"))) {
	case "1", "true", "yes":
		c.useServerTime = true
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
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
