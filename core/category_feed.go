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

// CategoryFeed manages a single WebSocket connection for a category.
type CategoryFeed struct {
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

func newCategoryFeed(category Category) *CategoryFeed {
	return &CategoryFeed{
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

func (f *CategoryFeed) ensureDefaults() {
	if f.wsURL == "" {
		f.wsURL = wsURL
	}
	if f.dialContext == nil {
		f.dialContext = defaultWSDialContext
	}
	if f.reconnectDelay <= 0 {
		f.reconnectDelay = reconnectDelay
	}
	if f.reconnectMaxDelay <= 0 {
		f.reconnectMaxDelay = reconnectMaxDelay
	}
	if f.heartbeatInterval <= 0 {
		f.heartbeatInterval = heartbeatInterval
	}
}

// Start connects and begins the read loop with automatic reconnection.
// ctx must be non-nil; cancellation stops reconnects and dial attempts.
func (f *CategoryFeed) Start(ctx context.Context) {
	f.ensureDefaults()
	f.ctx = ctx
	go f.connectLoop()
}

// Stop closes the active WebSocket (unblocking ReadMessage) and clears f.conn.
// Safe to call more than once.
func (f *CategoryFeed) Stop() {
	f.stopOnce.Do(func() {
		f.mu.Lock()
		if c := f.conn; c != nil {
			deadline := time.Now().Add(time.Second)
			_ = c.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), deadline)
			_ = c.Close()
			f.conn = nil
		}
		f.mu.Unlock()
	})
}

func (f *CategoryFeed) connectLoop() {
	delay := f.reconnectDelay
	for {
		select {
		case <-f.ctx.Done():
			return
		default:
		}

		conn, _, err := f.dialContext(f.ctx, f.wsURL, nil)
		if err != nil {
			if f.ctx.Err() != nil {
				return
			}
			slog.Warn("connection failed, retrying",
				"category", f.category,
				"err", err,
				"retry_in", delay,
			)
			f.broadcastError(fmt.Errorf("disconnected: %w", err))
			if f.sleepOrDone(delay) {
				return
			}
			delay = min(delay*2, f.reconnectMaxDelay)
			continue
		}

		if f.ctx.Err() != nil {
			_ = conn.Close()
			return
		}

		delay = f.reconnectDelay
		f.mu.Lock()
		f.conn = conn
		f.mu.Unlock()

		// resubscribe all active token IDs
		f.mu.RLock()
		tokenIDs := make([]string, 0, len(f.subscribers))
		for id := range f.subscribers {
			tokenIDs = append(tokenIDs, id)
		}
		f.mu.RUnlock()

		if len(tokenIDs) > 0 {
			if err := f.sendSubscribe(tokenIDs); err != nil {
				slog.Warn("subscribe failed",
					"category", f.category,
					"err", err,
				)
			} else {
				f.mu.RLock()
				for _, id := range tokenIDs {
					name := ""
					if metas := f.subscribers[id]; len(metas) > 0 {
						name = metas[0].name
					}
					slog.Info("subscribed token",
						append([]any{"category", f.category, "name", name}, AppendVerboseIDs("token_id", id)...)...,
					)
				}
				f.mu.RUnlock()
			}
		}

		heartbeatCtx, cancelHeartbeat := context.WithCancel(f.ctx)
		go f.runHeartbeat(conn, heartbeatCtx)

		if err := f.readLoop(conn); err != nil {
			slog.Warn("read loop ended, reconnecting",
				"category", f.category,
				"err", err,
				"retry_in", delay,
			)
			f.broadcastError(fmt.Errorf("disconnected: %w", err))
		}
		cancelHeartbeat()

		f.mu.Lock()
		if f.conn == conn {
			f.conn = nil
		}
		f.mu.Unlock()

		if f.ctx.Err() != nil {
			return
		}
		if f.sleepOrDone(delay) {
			return
		}
		delay = min(delay*2, f.reconnectMaxDelay)
	}
}

// sleepOrDone waits for d or until f.ctx is cancelled. Returns true if ctx was cancelled.
func (f *CategoryFeed) sleepOrDone(d time.Duration) bool {
	if d <= 0 {
		select {
		case <-f.ctx.Done():
			return true
		default:
			return false
		}
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-f.ctx.Done():
		return true
	case <-t.C:
		return false
	}
}

func (f *CategoryFeed) readLoop(conn *websocket.Conn) error {
	for {
		select {
		case <-f.ctx.Done():
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
			f.dispatch(msg)
		}
	}
}

func (f *CategoryFeed) dispatch(msg []byte) {
	var base struct {
		EventType string `json:"event_type"`
		AssetID   string `json:"asset_id"`
		Market    string `json:"market"`
	}

	if err := json.Unmarshal(msg, &base); err != nil {
		slog.Debug("error unmarshalling message",
			"category", f.category,
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
			f.broadcastTo(change.AssetID, e)
		}
	case "sport_event":
		var e models.SportEvent
		if err := json.Unmarshal(msg, &e); err != nil {
			return
		}
	case "book":
		var e models.BookEvent
		if err := json.Unmarshal(msg, &e); err != nil {
			return
		}
		f.broadcastTo(e.AssetID, e)
	case "market_resolved":
		var e models.MarketResolvedEvent
		if err := json.Unmarshal(msg, &e); err != nil {
			return
		}
		for _, assetID := range e.AssetIDs {
			f.broadcastTo(assetID, e)
		}
	}

}

func (f *CategoryFeed) broadcastTo(tokenID string, event any) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	for _, meta := range f.subscribers[tokenID] {
		select {
		case meta.ch <- event:
		default:
			// drop if channel is full
		}
	}
}

func (f *CategoryFeed) broadcastError(err error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	for _, metas := range f.subscribers {
		for _, meta := range metas {
			select {
			case meta.ch <- err:
			default:
			}
		}
	}
}

func (f *CategoryFeed) runHeartbeat(conn *websocket.Conn, ctx context.Context) {
	ticker := time.NewTicker(f.heartbeatInterval)
	defer ticker.Stop()
	ping := map[string]any{}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			f.connWriteMu.Lock()
			err := conn.WriteJSON(ping)
			f.connWriteMu.Unlock()
			if err != nil {
				slog.Warn("heartbeat write failed",
					"category", f.category,
					"err", err,
				)
			} else {
				//log.Printf("[%s] heartbeat ping", f.category)
			}
		}
	}
}

func (f *CategoryFeed) sendSubscribe(tokenIDs []string) error {
	f.connWriteMu.Lock()
	defer f.connWriteMu.Unlock()
	if f.conn == nil {
		return nil
	}
	msg := map[string]any{
		"assets_ids":             tokenIDs,
		"type":                   "market",
		"initial_dump":           false,
		"custom_feature_enabled": true,
	}
	return f.conn.WriteJSON(msg)
}

// Subscribe registers a channel to receive events for a token ID.
// If the token ID is not yet subscribed on the connection, it is added.
// Network subscribe sends are best effort; failures are logged and retried on reconnect.
func (f *CategoryFeed) Subscribe(tokenID string, name string, ch chan<- any) {
	f.mu.Lock()
	defer f.mu.Unlock()

	_, exists := f.subscribers[tokenID]
	f.subscribers[tokenID] = append(f.subscribers[tokenID], tokenMeta{name: name, ch: ch})

	if !exists && f.conn != nil {
		if err := f.sendSubscribe([]string{tokenID}); err != nil {
			slog.Warn("subscribe failed",
				append([]any{"category", f.category, "name", name, "err", err}, AppendVerboseIDs("token_id", tokenID)...)...,
			)
		} else {
			slog.Info("subscribed token",
				append([]any{"category", f.category, "name", name}, AppendVerboseIDs("token_id", tokenID)...)...,
			)
		}
	}

}

func (f *CategoryFeed) sendUnsubscribe(tokenIDs []string) error {
	f.connWriteMu.Lock()
	defer f.connWriteMu.Unlock()
	if f.conn == nil {
		return nil
	}
	msg := map[string]any{
		"operation":  "unsubscribe",
		"assets_ids": tokenIDs,
	}
	return f.conn.WriteJSON(msg)
}

// Unsubscribe removes a channel from a token ID.
// If no listeners remain, the token ID is unsubscribed from the connection.
func (f *CategoryFeed) Unsubscribe(tokenID string, ch chan<- any) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	metas := f.subscribers[tokenID]
	for i, meta := range metas {
		if meta.ch == ch {
			f.subscribers[tokenID] = append(metas[:i], metas[i+1:]...)
			break
		}
	}

	if len(f.subscribers[tokenID]) == 0 {
		delete(f.subscribers, tokenID)
		if f.conn != nil {
			return f.sendUnsubscribe([]string{tokenID})
		}
	}

	return nil
}

// FeedManager holds all category feeds and starts them at program boot.
type FeedManager struct {
	feeds map[Category]*CategoryFeed
}

func NewFeedManager(categories []Category) *FeedManager {
	m := &FeedManager{feeds: make(map[Category]*CategoryFeed)}
	for _, cat := range categories {
		m.feeds[cat] = newCategoryFeed(cat)
	}
	return m
}

func (m *FeedManager) Start(ctx context.Context) {
	for _, f := range m.feeds {
		f.Start(ctx)
	}
}

func (m *FeedManager) Stop() {
	for _, f := range m.feeds {
		f.Stop()
	}
}

func (m *FeedManager) Feed(cat Category) (*CategoryFeed, error) {
	f, ok := m.feeds[cat]
	if !ok {
		return nil, fmt.Errorf("no feed for category %s", cat)
	}
	return f, nil
}
