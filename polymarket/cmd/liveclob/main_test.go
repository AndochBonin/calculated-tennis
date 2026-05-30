package main

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/AndochBonin/E3/polymarket/clob"
	"github.com/AndochBonin/E3/polymarket/models"
)

func TestFirstOrderIDs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		data []models.Order
		n    int
		want []string
	}{
		{name: "empty", data: nil, n: 3, want: nil},
		{name: "skip_empty_id", data: []models.Order{{ID: ""}, {ID: "a"}, {ID: "b"}}, n: 5, want: []string{"a", "b"}},
		{name: "cap_n", data: []models.Order{{ID: "1"}, {ID: "2"}, {ID: "3"}}, n: 2, want: []string{"1", "2"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := firstOrderIDs(tt.data, tt.n)
			if len(got) != len(tt.want) {
				t.Fatalf("len got %d want %d (got %#v)", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("got %#v want %#v", got, tt.want)
				}
			}
		})
	}
}

func TestFirstTradeIDs(t *testing.T) {
	t.Parallel()
	t.Run("skips_empty", func(t *testing.T) {
		t.Parallel()
		got := firstTradeIDs([]models.Trade{{ID: ""}, {ID: "t1"}}, 5)
		if len(got) != 1 || got[0] != "t1" {
			t.Fatalf("got %#v", got)
		}
	})
	t.Run("stops_at_n", func(t *testing.T) {
		t.Parallel()
		got := firstTradeIDs([]models.Trade{{ID: "a"}, {ID: "b"}, {ID: "c"}}, 2)
		if len(got) != 2 || got[0] != "a" || got[1] != "b" {
			t.Fatalf("got %#v", got)
		}
	})
}

func TestFirstPositionKeys(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		p    models.Position
		want string
	}{
		{name: "condition_and_asset", p: models.Position{ConditionID: "c", Asset: "a"}, want: "c:a"},
		{name: "condition_only", p: models.Position{ConditionID: "c"}, want: "c"},
		{name: "asset_only", p: models.Position{Asset: "a"}, want: "a"},
		{name: "slug_fallback", p: models.Position{Slug: "s"}, want: "s"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := firstPositionKeys([]models.Position{tt.p}, 5)
			if len(got) != 1 || got[0] != tt.want {
				t.Fatalf("got %#v want %q", got, tt.want)
			}
		})
	}

	t.Run("skips_all_empty", func(t *testing.T) {
		t.Parallel()
		if got := firstPositionKeys([]models.Position{{}}, 5); len(got) != 0 {
			t.Fatalf("got %#v", got)
		}
	})

	t.Run("condition_wins_over_slug", func(t *testing.T) {
		t.Parallel()
		got := firstPositionKeys([]models.Position{{ConditionID: "c", Slug: "s"}}, 5)
		if len(got) != 1 || got[0] != "c" {
			t.Fatalf("got %#v", got)
		}
	})

	t.Run("stops_at_n", func(t *testing.T) {
		t.Parallel()
		got := firstPositionKeys([]models.Position{
			{ConditionID: "c1"},
			{ConditionID: "c2"},
		}, 1)
		if len(got) != 1 || got[0] != "c1" {
			t.Fatalf("got %#v", got)
		}
	})
}

func TestRunProbe_success(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/data/orders", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("orders: %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"limit":25,"next_cursor":"cur","count":2,"data":[{"id":"o1"},{"id":"o2"}]}`))
	})
	mux.HandleFunc("/data/trades", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("trades: %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"limit":10,"next_cursor":"","count":1,"data":[{"id":"t1"}]}`))
	})
	mux.HandleFunc("/positions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("positions: %s", r.Method)
		}
		if r.URL.Query().Get("user") == "" {
			t.Fatalf("positions: missing user query")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"conditionId":"c1","asset":"a1"}]`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	c := clob.NewClient(
		clob.WithBaseURL(srv.URL),
		clob.WithDataAPIBaseURL(srv.URL),
		clob.WithUserAddress("0xuser"),
	)
	if err := runProbe(log, c); err != nil {
		t.Fatalf("runProbe: %v", err)
	}
}

func TestRunProbe_emptyResponses(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	emptyOrders := `{"limit":0,"next_cursor":"","count":0,"data":[]}`
	emptyTrades := `{"limit":0,"next_cursor":"","count":0,"data":[]}`
	for _, path := range []string{"/data/orders", "/data/trades"} {
		p := path
		body := emptyOrders
		if p == "/data/trades" {
			body = emptyTrades
		}
		mux.HandleFunc(p, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(body))
		})
	}
	mux.HandleFunc("/positions", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("user") == "" {
			t.Fatalf("positions: missing user query")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	c := clob.NewClient(
		clob.WithBaseURL(srv.URL),
		clob.WithDataAPIBaseURL(srv.URL),
		clob.WithUserAddress("0xuser"),
	)
	if err := runProbe(log, c); err != nil {
		t.Fatalf("runProbe: %v", err)
	}
}

func TestRunProbe_getOrdersError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	t.Cleanup(srv.Close)

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	c := clob.NewClient(clob.WithBaseURL(srv.URL))
	err := runProbe(log, c)
	if err == nil || !strings.Contains(err.Error(), "get orders") {
		t.Fatalf("expected get orders error, got: %v", err)
	}
}

type stubQuerier struct {
	ordersResp   *models.OrdersResponse
	ordersErr    error
	tradesResp   *models.TradesResponse
	tradesErr    error
	positions    []models.Position
	positionsErr error
}

func (s stubQuerier) GetOrders() (*models.OrdersResponse, error) { return s.ordersResp, s.ordersErr }
func (s stubQuerier) GetTrades() (*models.TradesResponse, error) { return s.tradesResp, s.tradesErr }
func (s stubQuerier) GetPositions() ([]models.Position, error)   { return s.positions, s.positionsErr }

func TestRunProbe_getTradesError(t *testing.T) {
	t.Parallel()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	stub := stubQuerier{
		ordersResp: &models.OrdersResponse{},
		tradesErr:  errors.New("trades failed"),
	}
	err := runProbe(log, stub)
	if err == nil || err.Error() != "trades failed" {
		t.Fatalf("got: %v", err)
	}
}

func TestRunProbe_getPositionsError(t *testing.T) {
	t.Parallel()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	stub := stubQuerier{
		ordersResp:   &models.OrdersResponse{},
		tradesResp:   &models.TradesResponse{},
		positionsErr: errors.New("positions failed"),
	}
	err := runProbe(log, stub)
	if err == nil || err.Error() != "positions failed" {
		t.Fatalf("got: %v", err)
	}
}

func testCLOBMux(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/data/orders", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("orders: %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"limit":0,"next_cursor":"","count":0,"data":[]}`))
	})
	mux.HandleFunc("/data/trades", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("trades: %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"limit":0,"next_cursor":"","count":0,"data":[]}`))
	})
	mux.HandleFunc("/positions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("positions: %s", r.Method)
		}
		if r.URL.Query().Get("user") == "" {
			t.Fatalf("positions: missing user query")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	})
	return httptest.NewServer(mux)
}

func TestExitRun_success(t *testing.T) {
	t.Chdir(t.TempDir())
	srv := testCLOBMux(t)
	t.Cleanup(srv.Close)

	t.Setenv("POLYMARKET_CLOB_BASE_URL", srv.URL)
	t.Setenv("POLYMARKET_API_KEY", "test-key")
	t.Setenv("POLYMARKET_API_SECRET", "")
	t.Setenv("POLYMARKET_PASSPHRASE", "test-pass")
	t.Setenv("POLYMARKET_ADDRESS", "0xtest")

	t.Setenv("POLYMARKET_DATA_API_BASE_URL", srv.URL)
	t.Setenv("POLYMARKET_USER_ADDRESS", "0xtest")

	if code := exitRun(); code != 0 {
		t.Fatalf("exitRun: want 0, got %d", code)
	}
}

func TestExitRun_probeFails(t *testing.T) {
	t.Chdir(t.TempDir())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	t.Cleanup(srv.Close)

	t.Setenv("POLYMARKET_CLOB_BASE_URL", srv.URL)
	t.Setenv("POLYMARKET_API_KEY", "test-key")
	t.Setenv("POLYMARKET_API_SECRET", "")
	t.Setenv("POLYMARKET_PASSPHRASE", "test-pass")
	t.Setenv("POLYMARKET_ADDRESS", "0xtest")

	if code := exitRun(); code != 1 {
		t.Fatalf("exitRun: want 1, got %d", code)
	}
}
