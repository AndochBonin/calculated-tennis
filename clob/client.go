package clob

import (
	"net/http"
)

const clobBaseURL = "https://clob.polymarket.com"

type Client struct {
	http    *http.Client
	baseURL string
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

func NewClient(opts ...Option) *Client {
	c := &Client{
		http:    &http.Client{},
		baseURL: clobBaseURL,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}
