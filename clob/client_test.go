package clob

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/AndochBonin/polymarket/models"
)

func TestNewClientPicksUpBaseURLFromEnv(t *testing.T) {
	t.Setenv("POLYMARKET_CLOB_BASE_URL", "  https://env.example.com  ")

	c := NewClient()
	if c.baseURL != "https://env.example.com" {
		t.Fatalf("expected baseURL from env (trimmed), got: %q", c.baseURL)
	}
}

func TestNewClientFallsBackToDefaultBaseURLWhenEnvUnset(t *testing.T) {
	t.Setenv("POLYMARKET_CLOB_BASE_URL", "")

	c := NewClient()
	if c.baseURL != defaultClobBaseURL {
		t.Fatalf("expected default baseURL %q, got: %q", defaultClobBaseURL, c.baseURL)
	}
}

func TestWithBaseURLOverridesEnv(t *testing.T) {
	t.Setenv("POLYMARKET_CLOB_BASE_URL", "https://env.example.com")

	c := NewClient(WithBaseURL("https://override.example.com"))
	if c.baseURL != "https://override.example.com" {
		t.Fatalf("expected WithBaseURL to override env, got: %q", c.baseURL)
	}
}

func TestFetchClobServerUnixParsesPlainBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/time" {
			t.Fatalf("path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("1730000999\n"))
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL))
	if ts := c.fetchClobServerUnix(); ts != 1730000999 {
		t.Fatalf("fetchClobServerUnix: got %d", ts)
	}
}

func TestOrderMessageTimestampMillisUsesServerTimeWhenEnabled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/time" {
			t.Fatalf("path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("1730000999"))
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL), WithServerSignedTime(true))
	if got := c.orderMessageTimestampMillis(); got != 1730000999000 {
		t.Fatalf("orderMessageTimestampMillis: got %d want 1730000999000", got)
	}
}

func TestOrderMessageTimestampMillisUsesLocalClockWhenServerTimeDisabled(t *testing.T) {
	// Server errors if hit; WithServerSignedTime(false) must not call GET /time.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request to %s", r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL), WithServerSignedTime(false))
	before := time.Now().UnixMilli()
	got := c.orderMessageTimestampMillis()
	after := time.Now().UnixMilli()
	if got < before || got > after {
		t.Fatalf("orderMessageTimestampMillis: got %d not in [%d,%d]", got, before, after)
	}
}

func TestWithAddressTrimsAndSets(t *testing.T) {
	c := NewClient(WithAddress("  0xabc  "))
	if c.address != "0xabc" {
		t.Fatalf("address: got %q", c.address)
	}
}

func TestWithAddressEmptySkips(t *testing.T) {
	t.Setenv("POLYMARKET_ADDRESS", "0xenv")
	c := NewClient(WithAddress("   "))
	if c.address != "0xenv" {
		t.Fatalf("expected env address unchanged, got %q", c.address)
	}
}

func TestWithDataAPIBaseURLTrimsAndSets(t *testing.T) {
	c := NewClient(WithDataAPIBaseURL("  https://data.example  "))
	if c.dataAPIBaseURL != "https://data.example" {
		t.Fatalf("dataAPIBaseURL: got %q", c.dataAPIBaseURL)
	}
}

func TestWithDataAPIBaseURLEmptySkips(t *testing.T) {
	t.Setenv("POLYMARKET_DATA_API_BASE_URL", "https://from.env")
	c := NewClient(WithDataAPIBaseURL("  "))
	if c.dataAPIBaseURL != "https://from.env" {
		t.Fatalf("expected env data API base unchanged, got %q", c.dataAPIBaseURL)
	}
}

func TestWithServerSignedTime(t *testing.T) {
	t.Setenv("POLYMARKET_CLOB_SERVER_TIME", "")
	c := NewClient(WithServerSignedTime(true))
	if !c.useServerTime {
		t.Fatal("expected useServerTime true")
	}
	c = NewClient(WithServerSignedTime(false))
	if c.useServerTime {
		t.Fatal("expected useServerTime false")
	}
}

func TestNewClientServerTimeEnvTruthValues(t *testing.T) {
	for _, v := range []string{"1", "true", "yes", "TRUE", " Yes "} {
		t.Run(v, func(t *testing.T) {
			t.Setenv("POLYMARKET_CLOB_SERVER_TIME", v)
			if c := NewClient(); !c.useServerTime {
				t.Fatalf("expected useServerTime for env %q", v)
			}
		})
	}
}

func TestNewClientServerTimeEnvOtherValuesFalse(t *testing.T) {
	t.Setenv("POLYMARKET_CLOB_SERVER_TIME", "0")
	if c := NewClient(); c.useServerTime {
		t.Fatal("expected useServerTime false for 0")
	}
}

func TestWithBaseURLHTTPClientTransportEmptySkips(t *testing.T) {
	t.Setenv("POLYMARKET_CLOB_BASE_URL", "https://env.example.com")
	c := NewClient(
		WithBaseURL(""),
		WithHTTPClient(nil),
		WithTransport(nil),
	)
	if c.baseURL != "https://env.example.com" {
		t.Fatalf("baseURL: got %q", c.baseURL)
	}
	if c.http == nil {
		t.Fatal("expected default http client")
	}
}

func nearNowUnix(ts int64) bool {
	now := time.Now().Unix()
	return ts >= now-2 && ts <= now+2
}

func TestFetchClobServerUnixNewRequestErrorFallsBackToLocalTime(t *testing.T) {
	c := NewClient(WithBaseURL(":"))
	if ts := c.fetchClobServerUnix(); !nearNowUnix(ts) {
		t.Fatalf("expected local unix time, got %d", ts)
	}
}

func TestFetchClobServerUnixDoErrorFallsBackToLocalTime(t *testing.T) {
	c := NewClient(
		WithBaseURL("http://example.com"),
		WithTransport(roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("transport error")
		})),
	)
	if ts := c.fetchClobServerUnix(); !nearNowUnix(ts) {
		t.Fatalf("expected local unix time, got %d", ts)
	}
}

func TestFetchClobServerUnixNonOKFallsBackToLocalTime(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	defer srv.Close()
	c := NewClient(WithBaseURL(srv.URL))
	if ts := c.fetchClobServerUnix(); !nearNowUnix(ts) {
		t.Fatalf("expected local unix time, got %d", ts)
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

func TestFetchClobServerUnixReadBodyErrorFallsBackToLocalTime(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	c := NewClient(
		WithBaseURL(srv.URL),
		WithTransport(roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if !strings.HasSuffix(req.URL.Path, "/time") {
				t.Fatalf("path: %s", req.URL.Path)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(errReader{}),
			}, nil
		})),
	)
	if ts := c.fetchClobServerUnix(); !nearNowUnix(ts) {
		t.Fatalf("expected local unix time, got %d", ts)
	}
}

func TestFetchClobServerUnixInvalidIntegerBodyFallsBackToLocalTime(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not-a-unix-ts"))
	}))
	defer srv.Close()
	c := NewClient(WithBaseURL(srv.URL))
	if ts := c.fetchClobServerUnix(); !nearNowUnix(ts) {
		t.Fatalf("expected local unix time, got %d", ts)
	}
}

func TestAddAuthHeadersUsesServerTimeFromCLOB(t *testing.T) {
	t.Setenv("POLYMARKET_API_KEY", "k")
	t.Setenv("POLYMARKET_API_SECRET", "AQIDBA==")
	t.Setenv("POLYMARKET_PASSPHRASE", "p")
	t.Setenv("POLYMARKET_ADDRESS", "0x1")

	ref := NewClient()

	mux := http.NewServeMux()
	mux.HandleFunc("/time", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("1730000999"))
	})
	mux.HandleFunc("/data/orders", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("POLY_TIMESTAMP"); got != "1730000999" {
			t.Errorf("POLY_TIMESTAMP: got %q want 1730000999", got)
		}
		wantSig := ref.hmacSignature("1730000999", http.MethodGet, "/data/orders", "")
		if got := r.Header.Get("POLY_SIGNATURE"); got != wantSig {
			t.Errorf("POLY_SIGNATURE: got %q want %q", got, wantSig)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"limit":0,"next_cursor":"","count":0,"data":[]}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL), WithServerSignedTime(true))
	if _, err := c.GetOrders(); err != nil {
		t.Fatalf("GetOrders: %v", err)
	}
}

func TestPlaceOrder(t *testing.T) {
	t.Setenv("POLYMARKET_API_KEY", "k")
	t.Setenv("POLYMARKET_API_SECRET", "AQIDBA==")
	t.Setenv("POLYMARKET_PASSPHRASE", "p")
	t.Setenv("POLYMARKET_ADDRESS", "0x1")

	ref := NewClient()
	payload := &models.OrderPayload{
		Maker:         "0xmaker",
		Signer:        "0xsigner",
		TokenID:       "token1",
		MakerAmount:   "1",
		TakerAmount:   "2",
		Side:          models.OrderSideBuy,
		Expiration:    "0",
		Timestamp:     "123",
		Signature:     "sig",
		Salt:          1,
		SignatureType: 0,
	}
	wantReq := models.PlaceOrderRequest{
		Order:     *payload,
		Owner:     "0xowner",
		OrderType: models.OrderTypeGTC,
		PostOnly:  false,
		DeferExec: false,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/time", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("1730000999"))
	})
	mux.HandleFunc("/order", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("PlaceOrder: want POST, got %s", r.Method)
		}
		if r.URL.Path != "/order" {
			t.Fatalf("PlaceOrder: path %q", r.URL.Path)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		var got models.PlaceOrderRequest
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("unmarshal body: %v", err)
		}
		if got != wantReq {
			t.Fatalf("request round-trip: got %+v want %+v", got, wantReq)
		}
		ts := r.Header.Get("POLY_TIMESTAMP")
		if ts != "1730000999" {
			t.Errorf("POLY_TIMESTAMP: got %q want 1730000999", ts)
		}
		wantSig := ref.hmacSignature(ts, http.MethodPost, r.URL.Path, string(body))
		if gotSig := r.Header.Get("POLY_SIGNATURE"); gotSig != wantSig {
			t.Errorf("POLY_SIGNATURE: got %q want %q", gotSig, wantSig)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true,"orderID":"ord-1","status":"live"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL), WithServerSignedTime(true))
	out, err := c.PlaceOrder(payload, "0xowner", models.OrderTypeGTC)
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}
	if out.OrderID != "ord-1" || out.Status != "live" || !out.Success {
		t.Fatalf("response: %+v", out)
	}
}

func TestPlaceOrder_DefaultOwnerFromAPIKey(t *testing.T) {
	const apiKey = "550e8400-e29b-41d4-a716-446655440000"
	t.Setenv("POLYMARKET_API_KEY", apiKey)
	t.Setenv("POLYMARKET_API_SECRET", "AQIDBA==")
	t.Setenv("POLYMARKET_PASSPHRASE", "p")
	t.Setenv("POLYMARKET_ADDRESS", "0x1")

	ref := NewClient()
	payload := &models.OrderPayload{
		Maker:         "0xmaker",
		Signer:        "0xsigner",
		TokenID:       "token1",
		MakerAmount:   "1",
		TakerAmount:   "2",
		Side:          models.OrderSideBuy,
		Expiration:    "0",
		Timestamp:     "123",
		Signature:     "sig",
		Salt:          1,
		SignatureType: 0,
	}
	wantReq := models.PlaceOrderRequest{
		Order:     *payload,
		Owner:     apiKey,
		OrderType: models.OrderTypeGTC,
		PostOnly:  false,
		DeferExec: false,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/time", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("1730000999"))
	})
	mux.HandleFunc("/order", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		var got models.PlaceOrderRequest
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("unmarshal body: %v", err)
		}
		if got != wantReq {
			t.Fatalf("request round-trip: got %+v want %+v", got, wantReq)
		}
		ts := r.Header.Get("POLY_TIMESTAMP")
		wantSig := ref.hmacSignature(ts, http.MethodPost, r.URL.Path, string(body))
		if gotSig := r.Header.Get("POLY_SIGNATURE"); gotSig != wantSig {
			t.Errorf("POLY_SIGNATURE: got %q want %q", gotSig, wantSig)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true,"orderID":"ord-1","status":"live"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL), WithServerSignedTime(true))

	for _, name := range []string{"empty_string", "whitespace_only"} {
		t.Run(name, func(t *testing.T) {
			owner := ""
			if name == "whitespace_only" {
				owner = "  \t  "
			}
			out, err := c.PlaceOrder(payload, owner, models.OrderTypeGTC)
			if err != nil {
				t.Fatalf("PlaceOrder: %v", err)
			}
			if out.OrderID != "ord-1" {
				t.Fatalf("response: %+v", out)
			}
		})
	}
}

func TestPlaceOrder_ErrWhenOwnerEmptyAndNoAPIKey(t *testing.T) {
	t.Setenv("POLYMARKET_API_KEY", "")
	payload := &models.OrderPayload{Maker: "0x1"}
	c := NewClient()
	_, err := c.PlaceOrder(payload, "", models.OrderTypeGTC)
	if err == nil {
		t.Fatal("expected error when owner and API key are empty")
	}
}

func TestCancelOrder(t *testing.T) {
	t.Setenv("POLYMARKET_API_KEY", "k")
	t.Setenv("POLYMARKET_API_SECRET", "AQIDBA==")
	t.Setenv("POLYMARKET_PASSPHRASE", "p")
	t.Setenv("POLYMARKET_ADDRESS", "0x1")

	ref := NewClient()
	orderID := "cancel-me-1"

	mux := http.NewServeMux()
	mux.HandleFunc("/time", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("1730000999"))
	})
	mux.HandleFunc("/order", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("CancelOrder: want DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/order" {
			t.Fatalf("CancelOrder: path %q", r.URL.Path)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		var req models.CancelOrderRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("unmarshal body: %v", err)
		}
		if req.OrderID != orderID {
			t.Fatalf("orderID: got %q want %q", req.OrderID, orderID)
		}
		if !strings.Contains(string(body), orderID) {
			t.Fatalf("body should contain order id: %s", body)
		}
		ts := r.Header.Get("POLY_TIMESTAMP")
		wantSig := ref.hmacSignature(ts, http.MethodDelete, r.URL.Path, string(body))
		if gotSig := r.Header.Get("POLY_SIGNATURE"); gotSig != wantSig {
			t.Errorf("POLY_SIGNATURE: got %q want %q", gotSig, wantSig)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"canceled":["` + orderID + `"],"not_canceled":{}}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL), WithServerSignedTime(true))
	if err := c.CancelOrder(orderID); err != nil {
		t.Fatalf("CancelOrder: %v", err)
	}
}
