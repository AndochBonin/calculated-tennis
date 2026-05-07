package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/AndochBonin/polymarket/models"
	"github.com/gorilla/websocket"
)

const (
	wsURL             = "wss://ws-subscriptions-clob.polymarket.com/ws/market"
	reconnectDelay    = 5 * time.Second
	reconnectMaxDelay = 60 * time.Second
	heartbeatInterval = 10 * time.Second
)

// Category represents a supported trading category.
type Category string

const (
	CategoryNBA Category = "NBA"
	CategoryATP Category = "ATP"
)

// Tag is the unique identifier used to filter markets
type Tag int

const (
	TagNBA Tag = 745
	TagATP Tag = 864
)

type tokenMeta struct {
	name string
	ch   chan<- any
}

// MarketFeed manages a single WebSocket connection for a category.
type MarketFeed struct {
	category          Category
	mu                sync.RWMutex
	connWriteMu       sync.Mutex
	subscribers       map[string][]tokenMeta
	conn              *websocket.Conn
	ctx               context.Context
	stopOnce          sync.Once
	wsURL             string
	dialContext       func(context.Context, string, http.Header) (*websocket.Conn, *http.Response, error)
	reconnectDelay    time.Duration
	reconnectMaxDelay time.Duration
	heartbeatInterval time.Duration
}

func newMarketFeed(category Category) *MarketFeed {
	return &MarketFeed{
		category:          category,
		subscribers:       make(map[string][]tokenMeta),
		wsURL:             wsURL,
		dialContext:       defaultWSDialContext,
		reconnectDelay:    reconnectDelay,
		reconnectMaxDelay: reconnectMaxDelay,
		heartbeatInterval: heartbeatInterval,
	}
}

func defaultWSDialContext(ctx context.Context, url string, header http.Header) (*websocket.Conn, *http.Response, error) {
	var dialer websocket.Dialer
	return dialer.DialContext(ctx, url, header)
}

func (marketFeed *MarketFeed) ensureDefaults() {
	if marketFeed.wsURL == "" {
		marketFeed.wsURL = wsURL
	}
	if marketFeed.dialContext == nil {
		marketFeed.dialContext = defaultWSDialContext
	}
	if marketFeed.reconnectDelay <= 0 {
		marketFeed.reconnectDelay = reconnectDelay
	}
	if marketFeed.reconnectMaxDelay <= 0 {
		marketFeed.reconnectMaxDelay = reconnectMaxDelay
	}
	if marketFeed.heartbeatInterval <= 0 {
		marketFeed.heartbeatInterval = heartbeatInterval
	}
}

// Start connects and begins the read loop with automatic reconnection.
// ctx must be non-nil; cancellation stops reconnects and dial attempts.
func (marketFeed *MarketFeed) Start(ctx context.Context) {
	marketFeed.ensureDefaults()
	marketFeed.ctx = ctx
	go marketFeed.connectLoop()
}

// Stop closes the active WebSocket (unblocking ReadMessage) and clears f.conn.
// Safe to call more than once.
func (marketFeed *MarketFeed) Stop() {
	marketFeed.stopOnce.Do(func() {
		marketFeed.mu.Lock()
		if c := marketFeed.conn; c != nil {
			deadline := time.Now().Add(time.Second)
			_ = c.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), deadline)
			_ = c.Close()
			marketFeed.conn = nil
		}
		marketFeed.mu.Unlock()
	})
}

func (marketFeed *MarketFeed) connectLoop() {
	delay := marketFeed.reconnectDelay
	for {
		select {
		case <-marketFeed.ctx.Done():
			return
		default:
		}

		conn, _, err := marketFeed.dialContext(marketFeed.ctx, marketFeed.wsURL, nil)
		if err != nil {
			if marketFeed.ctx.Err() != nil {
				return
			}
			slog.Warn("connection failed, retrying",
				"category", marketFeed.category,
				"err", err,
				"retry_in", delay,
			)
			marketFeed.broadcastError(fmt.Errorf("disconnected: %w", err))
			if marketFeed.sleepOrDone(delay) {
				return
			}
			delay = min(delay*2, marketFeed.reconnectMaxDelay)
			continue
		}

		if marketFeed.ctx.Err() != nil {
			_ = conn.Close()
			return
		}

		delay = marketFeed.reconnectDelay
		marketFeed.mu.Lock()
		marketFeed.conn = conn
		marketFeed.mu.Unlock()

		// resubscribe all active token IDs
		marketFeed.mu.RLock()
		tokenIDs := make([]string, 0, len(marketFeed.subscribers))
		for id := range marketFeed.subscribers {
			tokenIDs = append(tokenIDs, id)
		}
		marketFeed.mu.RUnlock()

		if len(tokenIDs) > 0 {
			if err := marketFeed.sendSubscribe(tokenIDs); err != nil {
				slog.Warn("subscribe failed",
					"category", marketFeed.category,
					"err", err,
				)
			} else {
				marketFeed.mu.RLock()
				for _, id := range tokenIDs {
					name := ""
					if metas := marketFeed.subscribers[id]; len(metas) > 0 {
						name = metas[0].name
					}
					slog.Info("subscribed token",
						append([]any{"category", marketFeed.category, "name", name}, AppendVerboseIDs("token_id", id)...)...,
					)
				}
				marketFeed.mu.RUnlock()
			}
		}

		heartbeatCtx, cancelHeartbeat := context.WithCancel(marketFeed.ctx)
		go marketFeed.runHeartbeat(conn, heartbeatCtx)

		if err := marketFeed.readLoop(conn); err != nil {
			slog.Warn("read loop ended, reconnecting",
				"category", marketFeed.category,
				"err", err,
				"retry_in", delay,
			)
			marketFeed.broadcastError(fmt.Errorf("disconnected: %w", err))
		}
		cancelHeartbeat()

		marketFeed.mu.Lock()
		if marketFeed.conn == conn {
			marketFeed.conn = nil
		}
		marketFeed.mu.Unlock()

		if marketFeed.ctx.Err() != nil {
			return
		}
		if marketFeed.sleepOrDone(delay) {
			return
		}
		delay = min(delay*2, marketFeed.reconnectMaxDelay)
	}
}

// sleepOrDone waits for d or until f.ctx is cancelled. Returns true if ctx was cancelled.
func (marketFeed *MarketFeed) sleepOrDone(d time.Duration) bool {
	if d <= 0 {
		select {
		case <-marketFeed.ctx.Done():
			return true
		default:
			return false
		}
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-marketFeed.ctx.Done():
		return true
	case <-t.C:
		return false
	}
}

func (marketFeed *MarketFeed) readLoop(conn *websocket.Conn) error {
	for {
		select {
		case <-marketFeed.ctx.Done():
			return nil
		default:
		}

		_, msg, err := conn.ReadMessage()
		if err != nil {
			return err
		}

		msg = bytes.TrimSpace(msg)
		if len(msg) == 0 || !json.Valid(msg) {
			//log.Printf("[%s] heartbeat pong", f.category)
		} else {
			marketFeed.dispatch(msg)
		}
	}
}

func (marketFeed *MarketFeed) dispatch(msg []byte) {
	var base struct {
		EventType string `json:"event_type"`
		AssetID   string `json:"asset_id"`
		Market    string `json:"market"`
	}

	if err := json.Unmarshal(msg, &base); err != nil {
		slog.Debug("error unmarshalling message",
			"category", marketFeed.category,
			"message", msg,
			"err", err,
		)
		return
	}

	switch base.EventType {
	case "price_change":
		var e models.PriceEvent
		if err := json.Unmarshal(msg, &e); err != nil {
			return
		}
		for _, change := range e.PriceChanges {
			marketFeed.broadcastTo(change.AssetID, e)
		}
	case "book":
		var e models.BookEvent
		if err := json.Unmarshal(msg, &e); err != nil {
			return
		}
		marketFeed.broadcastTo(e.AssetID, e)
	case "market_resolved":
		var e models.MarketResolvedEvent
		if err := json.Unmarshal(msg, &e); err != nil {
			return
		}
		for _, assetID := range e.AssetIDs {
			marketFeed.broadcastTo(assetID, e)
		}
	}

}

func (marketFeed *MarketFeed) broadcastTo(tokenID string, event any) {
	marketFeed.mu.RLock()
	defer marketFeed.mu.RUnlock()
	for _, meta := range marketFeed.subscribers[tokenID] {
		select {
		case meta.ch <- event:
		default:
			// drop if channel is full
		}
	}
}

func (marketFeed *MarketFeed) broadcastError(err error) {
	marketFeed.mu.RLock()
	defer marketFeed.mu.RUnlock()
	for _, metas := range marketFeed.subscribers {
		for _, meta := range metas {
			select {
			case meta.ch <- err:
			default:
			}
		}
	}
}

func (marketFeed *MarketFeed) runHeartbeat(conn *websocket.Conn, ctx context.Context) {
	ticker := time.NewTicker(marketFeed.heartbeatInterval)
	defer ticker.Stop()
	ping := map[string]any{}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			marketFeed.connWriteMu.Lock()
			err := conn.WriteJSON(ping)
			marketFeed.connWriteMu.Unlock()
			if err != nil {
				slog.Warn("heartbeat write failed",
					"category", marketFeed.category,
					"err", err,
				)
			} else {
				//log.Printf("[%s] heartbeat ping", f.category)
			}
		}
	}
}

func (marketFeed *MarketFeed) sendSubscribe(tokenIDs []string) error {
	marketFeed.connWriteMu.Lock()
	defer marketFeed.connWriteMu.Unlock()
	if marketFeed.conn == nil {
		return nil
	}
	msg := map[string]any{
		"assets_ids":             tokenIDs,
		"type":                   "market",
		"initial_dump":           false,
		"custom_feature_enabled": true,
	}
	return marketFeed.conn.WriteJSON(msg)
}

// Subscribe registers a channel to receive events for a token ID.
// If the token ID is not yet subscribed on the connection, it is added.
// Network subscribe sends are best effort; failures are logged and retried on reconnect.
func (marketFeed *MarketFeed) Subscribe(tokenID string, name string, ch chan<- any) {
	marketFeed.mu.Lock()
	defer marketFeed.mu.Unlock()

	_, exists := marketFeed.subscribers[tokenID]
	marketFeed.subscribers[tokenID] = append(marketFeed.subscribers[tokenID], tokenMeta{name: name, ch: ch})

	if !exists && marketFeed.conn != nil {
		if err := marketFeed.sendSubscribe([]string{tokenID}); err != nil {
			slog.Warn("subscribe failed",
				append([]any{"category", marketFeed.category, "name", name, "err", err}, AppendVerboseIDs("token_id", tokenID)...)...,
			)
		} else {
			slog.Info("subscribed token",
				append([]any{"category", marketFeed.category, "name", name}, AppendVerboseIDs("token_id", tokenID)...)...,
			)
		}
	}

}

func (marketFeed *MarketFeed) sendUnsubscribe(tokenIDs []string) error {
	marketFeed.connWriteMu.Lock()
	defer marketFeed.connWriteMu.Unlock()
	if marketFeed.conn == nil {
		return nil
	}
	msg := map[string]any{
		"operation":  "unsubscribe",
		"assets_ids": tokenIDs,
	}
	return marketFeed.conn.WriteJSON(msg)
}

// Unsubscribe removes a channel from a token ID.
// If no listeners remain, the token ID is unsubscribed from the connection.
func (marketFeed *MarketFeed) Unsubscribe(tokenID string, ch chan<- any) error {
	marketFeed.mu.Lock()
	defer marketFeed.mu.Unlock()

	metas := marketFeed.subscribers[tokenID]
	for i, meta := range metas {
		if meta.ch == ch {
			marketFeed.subscribers[tokenID] = append(metas[:i], metas[i+1:]...)
			break
		}
	}

	if len(marketFeed.subscribers[tokenID]) == 0 {
		delete(marketFeed.subscribers, tokenID)
		if marketFeed.conn != nil {
			return marketFeed.sendUnsubscribe([]string{tokenID})
		}
	}

	return nil
}

// MarketFeedManager holds market feeds per trading category and starts them at program boot.
type MarketFeedManager struct {
	marketFeeds map[Category]*MarketFeed
}

func NewMarketFeedManager(categories []Category) *MarketFeedManager {
	m := &MarketFeedManager{marketFeeds: make(map[Category]*MarketFeed)}
	for _, cat := range categories {
		m.marketFeeds[cat] = newMarketFeed(cat)
	}
	return m
}

func (m *MarketFeedManager) Start(ctx context.Context) {
	for _, marketFeed := range m.marketFeeds {
		marketFeed.Start(ctx)
	}
}

func (m *MarketFeedManager) Stop() {
	for _, marketFeed := range m.marketFeeds {
		marketFeed.Stop()
	}
}

func (m *MarketFeedManager) GetMarketFeed(cat Category) (*MarketFeed, error) {
	marketFeed, ok := m.marketFeeds[cat]
	if !ok {
		return nil, fmt.Errorf("no market feed for category %s", cat)
	}
	return marketFeed, nil
}

// Backward-compatible aliases for in-flight downstream refactors.
type CategoryFeed = MarketFeed

func newCategoryFeed(category Category) *MarketFeed { return newMarketFeed(category) }
