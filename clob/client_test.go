package clob

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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
