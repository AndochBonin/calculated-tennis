// Live CLOB probe: calls GetOrders, GetTrades, and GetPositions against Polymarket CLOB.
//
// Run (from repo root, with credentials in .env or the environment):
//
//	go run ./cmd/liveclob
//
// Or: make live-clob
//
// Required env (see clob.NewClient): POLYMARKET_API_KEY, POLYMARKET_API_SECRET,
// POLYMARKET_PASSPHRASE. POLYMARKET_ADDRESS must equal the EOA wallet that derived those
// credentials (same key as cmd/python/generate_creds.py uses for derive_api_key).
// Optional: POLYMARKET_USER_ADDRESS (?user= for positions); POLYMARKET_DATA_API_BASE_URL;
// POLYMARKET_CLOB_SERVER_TIME=true; POLYMARKET_CLOB_BASE_URL.
package main

import (
	"log/slog"
	"os"

	"github.com/AndochBonin/polymarket/clob"
	"github.com/AndochBonin/polymarket/models"
	"github.com/joho/godotenv"
)

const previewIDs = 5

// clobQuerier is the subset of *clob.Client used by the probe (for tests).
type clobQuerier interface {
	GetOrders() (*models.OrdersResponse, error)
	GetTrades() (*models.TradesResponse, error)
	GetPositions() ([]models.Position, error)
}

func main() {
	os.Exit(exitRun())
}

func exitRun() int {
	_ = godotenv.Load()

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if err := runProbe(log, clob.NewClient()); err != nil {
		return 1
	}
	return 0
}

func runProbe(log *slog.Logger, c clobQuerier) error {
	orders, err := c.GetOrders()
	if err != nil {
		log.Error("GetOrders", "err", err)
		return err
	}
	nOrders := len(orders.Data)
	log.Info("orders",
		"data_len", nOrders,
		"count_field", orders.Count,
		"limit", orders.Limit,
		"has_next_cursor", orders.NextCursor != "",
	)
	if nOrders > 0 {
		log.Info("orders_first_ids", "ids", firstOrderIDs(orders.Data, previewIDs))
	}

	trades, err := c.GetTrades()
	if err != nil {
		log.Error("GetTrades", "err", err)
		return err
	}
	nTrades := len(trades.Data)
	log.Info("trades",
		"data_len", nTrades,
		"count_field", trades.Count,
		"limit", trades.Limit,
		"has_next_cursor", trades.NextCursor != "",
	)
	if nTrades > 0 {
		log.Info("trades_first_ids", "ids", firstTradeIDs(trades.Data, previewIDs))
	}

	positions, err := c.GetPositions()
	if err != nil {
		log.Error("GetPositions", "err", err)
		return err
	}
	nPos := len(positions)
	log.Info("positions", "len", nPos)
	if nPos > 0 {
		log.Info("positions_first_keys", "keys", firstPositionKeys(positions, previewIDs))
	}
	return nil
}

func firstOrderIDs(data []models.Order, n int) []string {
	out := make([]string, 0, min(n, len(data)))
	for i := range data {
		if len(out) >= n {
			break
		}
		if data[i].ID != "" {
			out = append(out, data[i].ID)
		}
	}
	return out
}

func firstTradeIDs(data []models.Trade, n int) []string {
	out := make([]string, 0, min(n, len(data)))
	for i := range data {
		if len(out) >= n {
			break
		}
		if data[i].ID != "" {
			out = append(out, data[i].ID)
		}
	}
	return out
}

func firstPositionKeys(data []models.Position, n int) []string {
	out := make([]string, 0, min(n, len(data)))
	for i := range data {
		if len(out) >= n {
			break
		}
		p := data[i]
		var key string
		switch {
		case p.ConditionID != "" && p.Asset != "":
			key = p.ConditionID + ":" + p.Asset
		case p.ConditionID != "":
			key = p.ConditionID
		case p.Asset != "":
			key = p.Asset
		default:
			key = p.Slug
		}
		if key != "" {
			out = append(out, key)
		}
	}
	return out
}
