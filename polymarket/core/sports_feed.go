package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/AndochBonin/calculated-tennis/polymarket/models"
	"github.com/gorilla/websocket"
)

const (
	// https://docs.polymarket.com/api-reference/wss/sports — text "ping" every ~5s; reply "pong" within 10s.
	sportsWSURL             = "wss://sports-api.polymarket.com/ws"
	sportsReconnectDelay    = 5 * time.Second
	sportsReconnectMaxDelay = 60 * time.Second
)

type sportsSubscriberMeta struct {
	name string
	ch   chan<- any
}

// SportsFeed manages the sports websocket connection and routes updates by game id.
type SportsFeed struct {
	mu                sync.RWMutex
	connWriteMu       sync.Mutex
	subscribers       map[int64][]sportsSubscriberMeta
	conn              *websocket.Conn
	ctx               context.Context
	stopOnce          sync.Once
	wsURL             string
	dialContext       func(context.Context, string, http.Header) (*websocket.Conn, *http.Response, error)
	reconnectDelay    time.Duration
	reconnectMaxDelay time.Duration
}

func NewSportsFeed() *SportsFeed {
	return &SportsFeed{
		subscribers:       make(map[int64][]sportsSubscriberMeta),
		wsURL:             sportsWSURL,
		dialContext:       defaultWSDialContext,
		reconnectDelay:    sportsReconnectDelay,
		reconnectMaxDelay: sportsReconnectMaxDelay,
	}
}

func (sportsFeed *SportsFeed) ensureDefaults() {
	if sportsFeed.wsURL == "" {
		sportsFeed.wsURL = sportsWSURL
	}
	if sportsFeed.dialContext == nil {
		sportsFeed.dialContext = defaultWSDialContext
	}
	if sportsFeed.reconnectDelay <= 0 {
		sportsFeed.reconnectDelay = sportsReconnectDelay
	}
	if sportsFeed.reconnectMaxDelay <= 0 {
		sportsFeed.reconnectMaxDelay = sportsReconnectMaxDelay
	}
}

func (sportsFeed *SportsFeed) Start(ctx context.Context) {
	sportsFeed.ensureDefaults()
	sportsFeed.ctx = ctx
	go sportsFeed.connectLoop()
}

func (sportsFeed *SportsFeed) Stop() {
	sportsFeed.stopOnce.Do(func() {
		sportsFeed.mu.Lock()
		if c := sportsFeed.conn; c != nil {
			deadline := time.Now().Add(time.Second)
			_ = c.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), deadline)
			_ = c.Close()
			sportsFeed.conn = nil
		}
		sportsFeed.mu.Unlock()
	})
}

func (sportsFeed *SportsFeed) connectLoop() {
	delay := sportsFeed.reconnectDelay
	for {
		select {
		case <-sportsFeed.ctx.Done():
			return
		default:
		}

		conn, _, err := sportsFeed.dialContext(sportsFeed.ctx, sportsFeed.wsURL, nil)
		if err != nil {
			if sportsFeed.ctx.Err() != nil {
				return
			}
			slog.Warn("sports connection failed, retrying", "err", err, "retry_in", delay)
			sportsFeed.broadcastError(fmt.Errorf("disconnected: %w", err))
			if sportsFeed.sleepOrDone(delay) {
				return
			}
			delay = min(delay*2, sportsFeed.reconnectMaxDelay)
			continue
		}

		if sportsFeed.ctx.Err() != nil {
			_ = conn.Close()
			return
		}

		delay = sportsFeed.reconnectDelay
		sportsFeed.mu.Lock()
		sportsFeed.conn = conn
		sportsFeed.mu.Unlock()

		if err := sportsFeed.readLoop(conn); err != nil {
			slog.Warn("sports read loop ended, reconnecting", "err", err, "retry_in", delay)
			sportsFeed.broadcastError(fmt.Errorf("disconnected: %w", err))
		}

		sportsFeed.mu.Lock()
		if sportsFeed.conn == conn {
			sportsFeed.conn = nil
		}
		sportsFeed.mu.Unlock()

		if sportsFeed.ctx.Err() != nil {
			return
		}
		if sportsFeed.sleepOrDone(delay) {
			return
		}
		delay = min(delay*2, sportsFeed.reconnectMaxDelay)
	}
}

func (sportsFeed *SportsFeed) sleepOrDone(d time.Duration) bool {
	if d <= 0 {
		select {
		case <-sportsFeed.ctx.Done():
			return true
		default:
			return false
		}
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-sportsFeed.ctx.Done():
		return true
	case <-t.C:
		return false
	}
}

func (sportsFeed *SportsFeed) readLoop(conn *websocket.Conn) error {
	for {
		select {
		case <-sportsFeed.ctx.Done():
			return nil
		default:
		}

		msgType, msg, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		if msgType != websocket.TextMessage {
			continue
		}

		msg = bytes.TrimSpace(msg)
		if len(msg) == 0 {
			continue
		}

		// Application-level keepalive (not WebSocket RFC ping frames). See sports channel docs.
		if bytes.EqualFold(msg, []byte("ping")) {
			sportsFeed.connWriteMu.Lock()
			writeErr := conn.WriteMessage(websocket.TextMessage, []byte("pong"))
			sportsFeed.connWriteMu.Unlock()
			if writeErr != nil {
				return writeErr
			}
			continue
		}

		if !json.Valid(msg) {
			continue
		}
		sportsFeed.dispatch(msg)
	}
}

func (sportsFeed *SportsFeed) dispatch(msg []byte) {
	var event models.SportsEvent
	if err := json.Unmarshal(msg, &event); err != nil {
		slog.Warn("unmarshal failure", "message", string(msg), "err", err)
		return
	}
	if event.GameID == 0 {
		return
	}
	sportsFeed.broadcastToGame(event.GameID, event)
}

func (sportsFeed *SportsFeed) broadcastToGame(gameID int64, event any) {
	sportsFeed.mu.RLock()
	defer sportsFeed.mu.RUnlock()
	for _, meta := range sportsFeed.subscribers[gameID] {
		select {
		case meta.ch <- event:
		default:
		}
	}
}

func (sportsFeed *SportsFeed) broadcastError(err error) {
	sportsFeed.mu.RLock()
	defer sportsFeed.mu.RUnlock()
	for _, metas := range sportsFeed.subscribers {
		for _, meta := range metas {
			select {
			case meta.ch <- err:
			default:
			}
		}
	}
}

func (sportsFeed *SportsFeed) Subscribe(gameID int64, name string, ch chan<- any) {
	if gameID == 0 {
		return
	}
	sportsFeed.mu.Lock()
	defer sportsFeed.mu.Unlock()
	sportsFeed.subscribers[gameID] = append(sportsFeed.subscribers[gameID], sportsSubscriberMeta{name: name, ch: ch})
	slog.Info("sports subscribed", "name", name, "game_id", strconv.FormatInt(gameID, 10))
}

func (sportsFeed *SportsFeed) Unsubscribe(gameID int64, ch chan<- any) {
	sportsFeed.mu.Lock()
	defer sportsFeed.mu.Unlock()
	metas := sportsFeed.subscribers[gameID]
	for i, meta := range metas {
		if meta.ch == ch {
			sportsFeed.subscribers[gameID] = append(metas[:i], metas[i+1:]...)
			break
		}
	}
	if len(sportsFeed.subscribers[gameID]) == 0 {
		delete(sportsFeed.subscribers, gameID)
	}
}
