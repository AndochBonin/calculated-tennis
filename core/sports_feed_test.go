package core

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AndochBonin/polymarket/models"
	"github.com/gorilla/websocket"
)

func TestSportsFeedEnsureDefaults(t *testing.T) {
	assertFeedConfig := func(t *testing.T, feed *SportsFeed, wantWSURL string, wantDialPtr uintptr, wantReconnectDelay, wantReconnectMaxDelay time.Duration) {
		t.Helper()
		if feed.wsURL != wantWSURL {
			t.Fatalf("expected wsURL %q, got %q", wantWSURL, feed.wsURL)
		}
		if reflect.ValueOf(feed.dialContext).Pointer() != wantDialPtr {
			t.Fatal("unexpected dialContext function pointer")
		}
		if feed.reconnectDelay != wantReconnectDelay {
			t.Fatalf("expected reconnectDelay %v, got %v", wantReconnectDelay, feed.reconnectDelay)
		}
		if feed.reconnectMaxDelay != wantReconnectMaxDelay {
			t.Fatalf("expected reconnectMaxDelay %v, got %v", wantReconnectMaxDelay, feed.reconnectMaxDelay)
		}
	}

	t.Run("fills all missing and invalid defaults", func(t *testing.T) {
		feed := &SportsFeed{
			subscribers: make(map[int64][]sportsSubscriberMeta),
		}
		feed.ensureDefaults()
		assertFeedConfig(
			t,
			feed,
			sportsWSURL,
			reflect.ValueOf(defaultWSDialContext).Pointer(),
			sportsReconnectDelay,
			sportsReconnectMaxDelay,
		)
	})

	t.Run("preserves explicit non-zero custom values", func(t *testing.T) {
		customDial := func(ctx context.Context, url string, header http.Header) (*websocket.Conn, *http.Response, error) {
			return nil, nil, errors.New("expected not to dial in this test")
		}
		feed := &SportsFeed{
			subscribers:       make(map[int64][]sportsSubscriberMeta),
			wsURL:             "ws://example.test/ws",
			dialContext:       customDial,
			reconnectDelay:    123 * time.Millisecond,
			reconnectMaxDelay: 456 * time.Millisecond,
		}
		feed.ensureDefaults()
		assertFeedConfig(
			t,
			feed,
			"ws://example.test/ws",
			reflect.ValueOf(customDial).Pointer(),
			123*time.Millisecond,
			456*time.Millisecond,
		)
	})
}

func TestSportsFeedSleepOrDone(t *testing.T) {
	t.Run("returns false after timer when context active", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		feed := NewSportsFeed()
		feed.ctx = ctx

		if done := feed.sleepOrDone(5 * time.Millisecond); done {
			t.Fatal("expected false when timer elapses without cancellation")
		}
	})

	t.Run("returns true when context cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		feed := NewSportsFeed()
		feed.ctx = ctx
		cancel()

		if done := feed.sleepOrDone(50 * time.Millisecond); !done {
			t.Fatal("expected true when context is cancelled")
		}
	})

	t.Run("returns false immediately when non-positive duration and context active", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		feed := NewSportsFeed()
		feed.ctx = ctx

		if done := feed.sleepOrDone(0); done {
			t.Fatal("expected false for non-positive duration with active context")
		}
	})

	t.Run("returns true immediately when non-positive duration and context cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		feed := NewSportsFeed()
		feed.ctx = ctx
		cancel()

		if done := feed.sleepOrDone(-1); !done {
			t.Fatal("expected true for non-positive duration with cancelled context")
		}
	})
}

func TestSportsFeedDispatchRoutesByGameID(t *testing.T) {
	feed := NewSportsFeed()
	match := make(chan any, 1)
	other := make(chan any, 1)
	feed.Subscribe(5428186, "match", match)
	feed.Subscribe(9999999, "other", other)

	feed.dispatch([]byte(`{
		"gameId":5428186,
		"leagueAbbreviation":"atp",
		"homeTeam":"Sinner",
		"awayTeam":"Alcaraz",
		"status":"inprogress",
		"score":"1-0",
		"period":"set2",
		"live":true,
		"ended":false,
		"eventState":{
			"type":"tennis",
			"startTime":"2026-05-01T10:00:00Z",
			"lastUpdate":"2026-05-01T10:10:00Z",
			"score":"1-0",
			"period":"set2",
			"live":true,
			"ended":false,
			"tournamentName":"Rome Masters",
			"tennisRound":"QF"
		}
	}`))

	select {
	case got := <-match:
		event, ok := got.(models.SportsEvent)
		if !ok {
			t.Fatalf("expected models.SportsEvent, got %T", got)
		}
		if event.GameID != 5428186 || event.LeagueAbbreviation != "atp" {
			t.Fatalf("unexpected event payload: %+v", event)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected event for matching game id")
	}

	select {
	case got := <-other:
		t.Fatalf("did not expect event for non-matching game id, got %T", got)
	default:
	}
}

func TestSportsFeedReadLoopPingPong(t *testing.T) {
	clientConn, serverConn, cleanup := newWebsocketPair(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	feed := NewSportsFeed()
	feed.ctx = ctx

	errCh := make(chan error, 1)
	go func() {
		errCh <- feed.readLoop(clientConn)
	}()

	if err := serverConn.WriteMessage(websocket.TextMessage, []byte("ping")); err != nil {
		t.Fatalf("write ping: %v", err)
	}

	msgType, msg, err := serverConn.ReadMessage()
	if err != nil {
		t.Fatalf("read pong: %v", err)
	}
	if msgType != websocket.TextMessage || string(msg) != "pong" {
		t.Fatalf("expected pong text message, got type=%d payload=%q", msgType, string(msg))
	}

	_ = serverConn.Close()
	select {
	case <-errCh:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("readLoop did not exit after socket close")
	}
}

func TestSportsFeedReadLoopPingWriteFailureReturnsError(t *testing.T) {
	clientConn, serverConn, cleanup := newWebsocketPair(t)
	defer cleanup()

	feed := NewSportsFeed()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	feed.ctx = ctx

	errCh := make(chan error, 1)
	go func() {
		errCh <- feed.readLoop(clientConn)
	}()

	if err := clientConn.SetWriteDeadline(time.Now().Add(-1 * time.Millisecond)); err != nil {
		t.Fatalf("set write deadline: %v", err)
	}
	if err := serverConn.WriteMessage(websocket.TextMessage, []byte("ping")); err != nil {
		t.Fatalf("write ping: %v", err)
	}

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected write/read error after ping when server closes connection")
		}
	case <-time.After(400 * time.Millisecond):
		t.Fatal("readLoop did not return after server close")
	}
}

func TestSportsFeedConnectLoopReturnsImmediatelyWhenContextAlreadyCanceled(t *testing.T) {
	feed := NewSportsFeed()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	feed.ctx = ctx

	feed.connectLoop()
}

func TestSportsFeedStartReconnectsAndReceivesEvents(t *testing.T) {
	serverConns := make(chan *websocket.Conn, 4)
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		serverConns <- conn
	}))
	defer server.Close()

	feed := NewSportsFeed()
	feed.wsURL = "ws" + strings.TrimPrefix(server.URL, "http")
	feed.reconnectDelay = 10 * time.Millisecond
	feed.reconnectMaxDelay = 20 * time.Millisecond

	var dialCalls atomic.Int32
	feed.dialContext = func(ctx context.Context, url string, header http.Header) (*websocket.Conn, *http.Response, error) {
		dialCalls.Add(1)
		return websocket.DefaultDialer.DialContext(ctx, url, header)
	}

	events := make(chan any, 1)
	feed.Subscribe(5428186, "match", events)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	feed.Start(ctx)
	defer feed.Stop()

	conn1 := mustReceiveServerConn(t, serverConns)
	_ = conn1.Close()

	conn2 := mustReceiveServerConn(t, serverConns)
	if err := conn2.WriteMessage(websocket.TextMessage, []byte(`{
		"gameId":5428186,
		"leagueAbbreviation":"atp",
		"homeTeam":"Home",
		"awayTeam":"Away",
		"status":"inprogress",
		"score":"0-0",
		"period":"set1",
		"live":true,
		"ended":false,
		"eventState":{"type":"tennis"}
	}`)); err != nil {
		t.Fatalf("write event on reconnected socket: %v", err)
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	gotEvent := false
	for time.Now().Before(deadline) {
		select {
		case got := <-events:
			if _, ok := got.(error); ok {
				continue
			}
			event, ok := got.(models.SportsEvent)
			if !ok {
				t.Fatalf("expected models.SportsEvent or error, got %T", got)
			}
			if event.GameID != 5428186 {
				t.Fatalf("expected gameId 5428186, got %d", event.GameID)
			}
			gotEvent = true
		case <-time.After(30 * time.Millisecond):
		}
		if gotEvent {
			break
		}
	}
	if !gotEvent {
		t.Fatal("expected event after reconnect")
	}

	if dialCalls.Load() < 2 {
		t.Fatalf("expected at least 2 dial attempts, got %d", dialCalls.Load())
	}
}

func TestSportsFeedUnsubscribeRemovesListener(t *testing.T) {
	feed := NewSportsFeed()
	ch := make(chan any, 1)
	feed.Subscribe(5428186, "match", ch)
	feed.Unsubscribe(5428186, ch)

	feed.dispatch([]byte(`{"gameId":5428186,"leagueAbbreviation":"atp","eventState":{"type":"tennis"}}`))

	select {
	case got := <-ch:
		t.Fatalf("did not expect event after unsubscribe, got %T", got)
	default:
	}
}

func TestSportsFeedSubscribeZeroGameIDNoOp(t *testing.T) {
	feed := NewSportsFeed()
	ch := make(chan any, 1)

	feed.Subscribe(0, "zero", ch)
	if len(feed.subscribers) != 0 {
		t.Fatalf("expected no subscribers for gameID 0, got %#v", feed.subscribers)
	}
}

func TestSportsFeedDispatchDropsInvalidAndZeroGameID(t *testing.T) {
	feed := NewSportsFeed()
	ch := make(chan any, 1)
	feed.Subscribe(5428186, "match", ch)

	feed.dispatch([]byte(`{`))
	feed.dispatch([]byte(`{}`))
	feed.dispatch([]byte(`{"gameId":0,"leagueAbbreviation":"atp"}`))

	select {
	case got := <-ch:
		t.Fatalf("did not expect broadcast for invalid/zero game payloads, got %T", got)
	default:
	}
}

func TestSportsFeedReadLoopSkipsNonTextWhitespaceAndInvalidJSON(t *testing.T) {
	clientConn, serverConn, cleanup := newWebsocketPair(t)
	defer cleanup()

	feed := NewSportsFeed()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	feed.ctx = ctx

	events := make(chan any, 1)
	feed.Subscribe(5428186, "match", events)

	errCh := make(chan error, 1)
	go func() {
		errCh <- feed.readLoop(clientConn)
	}()

	if err := serverConn.WriteMessage(websocket.BinaryMessage, []byte(`{"gameId":5428186}`)); err != nil {
		t.Fatalf("write binary frame: %v", err)
	}
	if err := serverConn.WriteMessage(websocket.TextMessage, []byte(" \n\t ")); err != nil {
		t.Fatalf("write whitespace payload: %v", err)
	}
	if err := serverConn.WriteMessage(websocket.TextMessage, []byte("not-json")); err != nil {
		t.Fatalf("write invalid json payload: %v", err)
	}

	_ = serverConn.Close()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected readLoop to return read error after websocket close")
		}
	case <-time.After(400 * time.Millisecond):
		t.Fatal("readLoop did not exit after websocket close")
	}

	select {
	case got := <-events:
		t.Fatalf("did not expect broadcast for skipped payloads, got %T", got)
	default:
	}
}

func TestSportsFeedReadLoopReturnsNilWhenContextAlreadyCanceled(t *testing.T) {
	clientConn, _, cleanup := newWebsocketPair(t)
	defer cleanup()

	feed := NewSportsFeed()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	feed.ctx = ctx

	if err := feed.readLoop(clientConn); err != nil {
		t.Fatalf("expected nil when context is already canceled, got %v", err)
	}
}

func TestSportsFeedReadLoopDispatchesValidJSONMessage(t *testing.T) {
	clientConn, serverConn, cleanup := newWebsocketPair(t)
	defer cleanup()

	feed := NewSportsFeed()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	feed.ctx = ctx

	events := make(chan any, 1)
	feed.Subscribe(5428186, "match", events)

	errCh := make(chan error, 1)
	go func() {
		errCh <- feed.readLoop(clientConn)
	}()

	if err := serverConn.WriteMessage(websocket.TextMessage, []byte(`{"gameId":5428186,"leagueAbbreviation":"atp","eventState":{"type":"tennis"}}`)); err != nil {
		t.Fatalf("write valid sports payload: %v", err)
	}

	select {
	case got := <-events:
		if _, ok := got.(models.SportsEvent); !ok {
			t.Fatalf("expected models.SportsEvent from readLoop dispatch, got %T", got)
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatal("expected dispatched sports event")
	}

	_ = serverConn.Close()
	select {
	case <-errCh:
	case <-time.After(400 * time.Millisecond):
		t.Fatal("readLoop did not exit after websocket close")
	}
}

func TestSportsFeedConnectLoopDialFailureBackoffAndCancel(t *testing.T) {
	feed := NewSportsFeed()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	feed.ctx = ctx
	feed.reconnectDelay = 2 * time.Millisecond
	feed.reconnectMaxDelay = 4 * time.Millisecond

	events := make(chan any, 8)
	feed.Subscribe(5428186, "match", events)

	var attempts atomic.Int32
	feed.dialContext = func(ctx context.Context, url string, header http.Header) (*websocket.Conn, *http.Response, error) {
		n := attempts.Add(1)
		if n >= 3 {
			cancel()
		}
		return nil, nil, errors.New("dial fail")
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		feed.connectLoop()
	}()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("connectLoop did not stop in time")
	}

	if attempts.Load() < 2 {
		t.Fatalf("expected multiple dial attempts, got %d", attempts.Load())
	}

	select {
	case got := <-events:
		if _, ok := got.(error); !ok {
			t.Fatalf("expected broadcasted error on dial failure, got %T", got)
		}
	default:
		t.Fatal("expected at least one broadcasted dial error")
	}
}

func TestSportsFeedConnectLoopReturnsWhenDialHonorsCanceledContext(t *testing.T) {
	feed := NewSportsFeed()
	ctx, cancel := context.WithCancel(context.Background())
	feed.ctx = ctx
	feed.reconnectDelay = 5 * time.Millisecond
	feed.reconnectMaxDelay = 10 * time.Millisecond

	dialEntered := make(chan struct{})
	feed.dialContext = func(ctx context.Context, url string, header http.Header) (*websocket.Conn, *http.Response, error) {
		close(dialEntered)
		<-ctx.Done()
		return nil, nil, ctx.Err()
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		feed.connectLoop()
	}()

	select {
	case <-dialEntered:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("dialContext was not entered")
	}
	cancel()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("connectLoop did not exit after context cancellation")
	}
}

func TestSportsFeedConnectLoopCancelAfterDialBeforeAssignClosesConnAndReturns(t *testing.T) {
	clientConn, serverConn, cleanup := newWebsocketPair(t)
	defer cleanup()

	feed := NewSportsFeed()
	ctx, cancel := context.WithCancel(context.Background())
	feed.ctx = ctx
	feed.reconnectDelay = 5 * time.Millisecond
	feed.reconnectMaxDelay = 10 * time.Millisecond

	dialReady := make(chan struct{})
	dialRelease := make(chan struct{})
	feed.dialContext = func(ctx context.Context, url string, header http.Header) (*websocket.Conn, *http.Response, error) {
		close(dialReady)
		<-dialRelease
		return clientConn, nil, nil
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		feed.connectLoop()
	}()

	select {
	case <-dialReady:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("dialContext was not entered")
	}

	cancel()
	close(dialRelease)

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("connectLoop did not exit after cancel before assignment")
	}

	_, _, err := serverConn.ReadMessage()
	if err == nil {
		t.Fatal("expected server websocket to be closed when ctx canceled after dial")
	}

	feed.mu.RLock()
	defer feed.mu.RUnlock()
	if feed.conn != nil {
		t.Fatal("expected feed conn to remain nil after early cancel")
	}
}

func TestSportsFeedConnectLoopCancelDuringBackoffSleepExits(t *testing.T) {
	feed := NewSportsFeed()
	ctx, cancel := context.WithCancel(context.Background())
	feed.ctx = ctx
	feed.reconnectDelay = 150 * time.Millisecond
	feed.reconnectMaxDelay = 300 * time.Millisecond

	firstDialAttempt := make(chan struct{})
	feed.dialContext = func(ctx context.Context, url string, header http.Header) (*websocket.Conn, *http.Response, error) {
		select {
		case <-firstDialAttempt:
		default:
			close(firstDialAttempt)
		}
		return nil, nil, errors.New("dial fail")
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		feed.connectLoop()
	}()

	select {
	case <-firstDialAttempt:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("dialContext was not entered")
	}

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("connectLoop did not exit after cancel during backoff")
	}
}

func TestSportsFeedConnectLoopBroadcastsErrorAfterReadLoopDisconnect(t *testing.T) {
	clientConn, serverConn, cleanup := newWebsocketPair(t)
	defer cleanup()

	feed := NewSportsFeed()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	feed.ctx = ctx
	feed.reconnectDelay = 5 * time.Millisecond
	feed.reconnectMaxDelay = 10 * time.Millisecond

	events := make(chan any, 2)
	feed.Subscribe(5428186, "match", events)

	var dialCount atomic.Int32
	feed.dialContext = func(ctx context.Context, url string, header http.Header) (*websocket.Conn, *http.Response, error) {
		if dialCount.Add(1) == 1 {
			return clientConn, nil, nil
		}
		<-ctx.Done()
		return nil, nil, ctx.Err()
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		feed.connectLoop()
	}()

	_ = serverConn.Close()

	select {
	case got := <-events:
		if _, ok := got.(error); !ok {
			t.Fatalf("expected disconnect error broadcast, got %T", got)
		}
	case <-time.After(400 * time.Millisecond):
		t.Fatal("expected error broadcast after read loop disconnect")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("connectLoop did not stop after cancellation")
	}
}

func TestSportsFeedConnectLoopReadLoopReturnsNilOnCancel(t *testing.T) {
	clientConn, serverConn, cleanup := newWebsocketPair(t)
	defer cleanup()

	feed := NewSportsFeed()
	ctx, cancel := context.WithCancel(context.Background())
	feed.ctx = ctx
	feed.reconnectDelay = 5 * time.Millisecond
	feed.reconnectMaxDelay = 10 * time.Millisecond

	dialCalls := make(chan struct{}, 1)
	feed.dialContext = func(ctx context.Context, url string, header http.Header) (*websocket.Conn, *http.Response, error) {
		select {
		case dialCalls <- struct{}{}:
		default:
		}
		return clientConn, nil, nil
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		feed.connectLoop()
	}()

	select {
	case <-dialCalls:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("dialContext was not called")
	}

	feed.mu.RLock()
	assignedConn := feed.conn
	feed.mu.RUnlock()
	if assignedConn == nil {
		t.Fatal("expected connection to be assigned")
	}

	cancel()
	_ = serverConn.Close()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("connectLoop did not stop after cancellation")
	}
}

func TestSportsFeedConnectLoopKeepsConnWhenReplacedDuringReadLoop(t *testing.T) {
	clientConn, serverConn, cleanup := newWebsocketPair(t)
	defer cleanup()

	feed := NewSportsFeed()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	feed.ctx = ctx
	feed.reconnectDelay = 5 * time.Millisecond
	feed.reconnectMaxDelay = 10 * time.Millisecond

	dialEntered := make(chan struct{}, 1)
	feed.dialContext = func(ctx context.Context, url string, header http.Header) (*websocket.Conn, *http.Response, error) {
		select {
		case dialEntered <- struct{}{}:
		default:
		}
		return clientConn, nil, nil
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		feed.connectLoop()
	}()

	select {
	case <-dialEntered:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("dialContext was not called")
	}

	feed.mu.Lock()
	feed.conn = &websocket.Conn{}
	feed.mu.Unlock()

	_ = serverConn.Close()
	cancel()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("connectLoop did not stop after cancellation")
	}

	feed.mu.RLock()
	defer feed.mu.RUnlock()
	if feed.conn == nil {
		t.Fatal("expected replaced conn value to remain when cleanup sees different conn")
	}
}

func TestSportsFeedConnectLoopCancelDuringBackoffAfterReadErrorExits(t *testing.T) {
	clientConn, serverConn, cleanup := newWebsocketPair(t)
	defer cleanup()

	feed := NewSportsFeed()
	ctx, cancel := context.WithCancel(context.Background())
	feed.ctx = ctx
	feed.reconnectDelay = 150 * time.Millisecond
	feed.reconnectMaxDelay = 300 * time.Millisecond

	var dialCalls atomic.Int32
	feed.dialContext = func(ctx context.Context, url string, header http.Header) (*websocket.Conn, *http.Response, error) {
		dialCalls.Add(1)
		return clientConn, nil, nil
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		feed.connectLoop()
	}()

	deadline := time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(deadline) {
		feed.mu.RLock()
		connected := feed.conn != nil
		feed.mu.RUnlock()
		if connected {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	_ = serverConn.Close()
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("connectLoop did not exit after cancel during post-read backoff")
	}
}
