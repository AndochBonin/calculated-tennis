package clob

import (
	"net/http"
)

const clobBaseURL = "https://clob.polymarket.com"

type Client struct {
	http *http.Client
}

func NewClient() *Client {
	return &Client{http: &http.Client{}}
}
