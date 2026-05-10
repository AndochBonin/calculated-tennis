package clob

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shopspring/decimal"
)

func TestValidateLimitPrice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		price   string
		tick    string
		wantErr string
	}{
		{"multiple", "0.5", "0.01", ""},
		{"equivalent_decimals", "0.50", "0.010", ""},
		{"tick_one", "0.1", "0.1", ""},
		{"zero_tick", "0.5", "0", "tick size must be positive"},
		{"negative_tick", "0.5", "-0.01", "tick size must be positive"},
		{"not_multiple", "0.055", "0.01", "not a multiple"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			price, err := decimal.NewFromString(tt.price)
			if err != nil {
				t.Fatal(err)
			}
			tick, err := decimal.NewFromString(tt.tick)
			if err != nil {
				t.Fatal(err)
			}
			err = ValidateLimitPrice(price, tick)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error: %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateMinOrderSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		size    string
		min     string
		wantErr string
	}{
		{"at_min", "10", "10", ""},
		{"above_min", "5", "0.01", ""},
		{"below_min", "5", "10", "below minimum"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			size, err := decimal.NewFromString(tt.size)
			if err != nil {
				t.Fatal(err)
			}
			min, err := decimal.NewFromString(tt.min)
			if err != nil {
				t.Fatal(err)
			}
			err = ValidateMinOrderSize(size, min)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error: %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateLimitOrderAgainstBook(t *testing.T) {
	bookJSON := `{"market":"m1","asset_id":"tok-a","timestamp":"ts","hash":"h","bids":[],"asks":[],"min_order_size":"1","tick_size":"0.01","neg_risk":true,"last_trade_price":"0.52"}`

	mux := http.NewServeMux()
	mux.HandleFunc("/book", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("token_id"); got != "tok-a" {
			t.Fatalf("token_id query: got %q", got)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(bookJSON))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL))

	book, err := c.ValidateLimitOrderAgainstBook("tok-a", mustDec(t, "0.5"), mustDec(t, "2"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if book == nil || book.AssetID != "tok-a" || !book.NegRisk {
		t.Fatalf("unexpected book: %#v", book)
	}

	_, err = c.ValidateLimitOrderAgainstBook("tok-a", mustDec(t, "0.505"), mustDec(t, "2"))
	if err == nil || !strings.Contains(err.Error(), "not a multiple") {
		t.Fatalf("expected tick error, got: %v", err)
	}

	_, err = c.ValidateLimitOrderAgainstBook("tok-a", mustDec(t, "0.5"), mustDec(t, "0.5"))
	if err == nil || !strings.Contains(err.Error(), "below minimum") {
		t.Fatalf("expected min size error, got: %v", err)
	}
}

func TestValidateLimitOrderAgainstBookGetOrderBookError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/book", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("nope"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL))
	_, err := c.ValidateLimitOrderAgainstBook("tok", mustDec(t, "0.5"), mustDec(t, "1"))
	if err == nil || !strings.Contains(err.Error(), "unexpected status 500") {
		t.Fatalf("expected HTTP error, got: %v", err)
	}
}

func mustDec(t *testing.T, s string) decimal.Decimal {
	t.Helper()
	d, err := decimal.NewFromString(s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}
