package clob

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strings"
)

func decodeAPISecret(secret string) ([]byte, error) {
	s := strings.TrimSpace(secret)
	if s == "" {
		return nil, base64.CorruptInputError(0)
	}
	// Polymarket issues URL-safe secrets; Python client uses urlsafe_b64decode.
	if b, err := base64.URLEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	// Fallback: RFC 4648 standard base64 (or mixed forms after normalizing URL-safe chars).
	normalized := strings.ReplaceAll(s, "-", "+")
	normalized = strings.ReplaceAll(normalized, "_", "/")
	padding := (4 - (len(normalized) % 4)) % 4
	normalized += strings.Repeat("=", padding)
	return base64.StdEncoding.DecodeString(normalized)
}

func (c *Client) hmacSignature(timestamp, method, path, body string) string {
	message := timestamp + method + path
	if body != "" {
		message += body
	}

	secret, err := decodeAPISecret(c.secret)
	if err != nil {
		return ""
	}

	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(message))
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	// must be url-safe base64 but keep "=" suffix
	sig = strings.ReplaceAll(sig, "+", "-")
	sig = strings.ReplaceAll(sig, "/", "_")

	return sig
}
