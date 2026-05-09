package clob

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func TestDecodeAPISecretEmptyString(t *testing.T) {
	_, err := decodeAPISecret("")
	if err == nil {
		t.Fatal("expected error for empty secret")
	}
	var corrupt base64.CorruptInputError
	if !errors.As(err, &corrupt) {
		t.Fatalf("expected base64.CorruptInputError, got %T %v", err, err)
	}
}

func TestDecodeAPISecretStandardBase64WithPlus(t *testing.T) {
	key := []byte{0xfb, 0xef, 0xbe}
	stdSecret := base64.StdEncoding.EncodeToString(key)
	if !strings.ContainsAny(stdSecret, "+/") {
		t.Fatalf("fixture should contain + or /, got %q", stdSecret)
	}
	got, err := decodeAPISecret(stdSecret)
	if err != nil {
		t.Fatalf("decodeAPISecret: %v", err)
	}
	want, err := base64.StdEncoding.DecodeString(stdSecret)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("got %x want %x", got, want)
	}
}

func TestDecodeAPISecretURLSafeEquivalentToStd(t *testing.T) {
	key := []byte{0xfe, 0xff, 0xe0, 0x10, 0x20}
	std := base64.StdEncoding.EncodeToString(key)
	urlLike := strings.ReplaceAll(strings.ReplaceAll(std, "+", "-"), "/", "_")
	urlLike = strings.TrimRight(urlLike, "=")

	got, err := decodeAPISecret(urlLike)
	if err != nil {
		t.Fatalf("decodeAPISecret: %v", err)
	}
	want, err := base64.StdEncoding.DecodeString(std)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("got %x want %x", got, want)
	}
}

func TestHMACSignatureURLSafeSecretMatchesStd(t *testing.T) {
	key := []byte{1, 2, 3, 4}
	stdSecret := base64.StdEncoding.EncodeToString(key)
	urlSecret := strings.ReplaceAll(strings.ReplaceAll(stdSecret, "+", "-"), "/", "_")
	urlSecret = strings.TrimRight(urlSecret, "=")

	t.Setenv("POLYMARKET_API_SECRET", stdSecret)
	cStd := NewClient()
	t.Setenv("POLYMARKET_API_SECRET", urlSecret)
	cURL := NewClient()

	const ts, method, path = "1000000000", "GET", "/orders"
	if sigStd, sigURL := cStd.hmacSignature(ts, method, path, ""), cURL.hmacSignature(ts, method, path, ""); sigStd != sigURL {
		t.Fatalf("signature mismatch url-safe vs std: %q vs %q", sigURL, sigStd)
	}
}

func TestHMACSignatureGoldenVector(t *testing.T) {
	t.Setenv("POLYMARKET_API_SECRET", "AQIDBA==")
	c := NewClient()
	got := c.hmacSignature("1000000000", "GET", "/orders", "")
	want := "seLRFCh0NkX9mzKLnlAlujkr2Sap63TDoGK9EHhUXG4="
	if got != want {
		t.Fatalf("signature mismatch: got %q want %q", got, want)
	}
}

func TestHMACSignatureWithBodyGoldenVector(t *testing.T) {
	t.Setenv("POLYMARKET_API_SECRET", "AQIDBA==")
	c := NewClient()
	got := c.hmacSignature("1000000000", "POST", "/orders", `{"a":1}`)
	want := "gzJjSHkcAI90MqvzM7BwgWnvtptLsbhypfp79KVjKBk="
	if got != want {
		t.Fatalf("signature mismatch: got %q want %q", got, want)
	}
}

func TestHMACSignatureInvalidBase64Secret(t *testing.T) {
	t.Setenv("POLYMARKET_API_SECRET", "not-valid-base64!!!")
	c := NewClient()
	if got := c.hmacSignature("1", "GET", "/", ""); got != "" {
		t.Fatalf("expected empty signature for invalid secret, got %q", got)
	}
}
