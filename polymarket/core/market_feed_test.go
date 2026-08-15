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

	"github.com/AndochBonin/calculated-tennis/polymarket/models"
	"github.com/gorilla/websocket"
)

func TestCategoryFeedDispatchPriceChangeBroadcastsPerAsset(t *testing.T) {
	feed := newCategoryFeed(CategoryATP)
	chAssetA := make(chan any, 1)
	chAssetB := make(chan any, 1)
	feed.subscribers["asset-a"] = []tokenMeta{{name: "A", ch: chAssetA}}
	feed.subscribers["asset-b"] = []tokenMeta{{name: "B", ch: chAssetB}}

	feed.dispatch([]byte(`{
		"event_type":"price_change",
		"market":"mkt-1",
		"price_changes":[
			{"asset_id":"asset-a","price":"0.51","size":"20","side":"buy","hash":"h1"},
			{"asset_id":"asset-b","price":"0.49","size":"10","side":"sell","hash":"h2"}
		],
		"timestamp":"2026-01-01T00:00:00Z"
	}`))

	select {
	case got := <-chAssetA:
		event, ok := got.(models.PriceEvent)
		if !ok {
			t.Fatalf("expected models.PriceEvent for asset-a, got %T", got)
		}
		if event.EventType != "price_change" || event.Market != "mkt-1" {
			t.Fatalf("unexpected event fields: %+v", event)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected event for asset-a")
	}

	select {
	case got := <-chAssetB:
		event, ok := got.(models.PriceEvent)
		if !ok {
			t.Fatalf("expected models.PriceEvent for asset-b, got %T", got)
		}
		if event.EventType != "price_change" || event.Market != "mkt-1" {
			t.Fatalf("unexpected event fields: %+v", event)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected event for asset-b")
	}
}

func TestCategoryFeedDispatchBookBroadcastsByAsset(t *testing.T) {
	feed := newCategoryFeed(CategoryATP)
	ch := make(chan any, 1)
	feed.subscribers["asset-book"] = []tokenMeta{{name: "book", ch: ch}}

	feed.dispatch([]byte(`{
		"event_type":"book",
		"asset_id":"asset-book",
		"market":"mkt-book",
		"bids":[{"price":"0.40","size":"100"}],
		"asks":[{"price":"0.60","size":"90"}],
		"timestamp":"2026-01-01T00:00:00Z",
		"hash":"book-hash"
	}`))

	select {
	case got := <-ch:
		event, ok := got.(models.BookEvent)
		if !ok {
			t.Fatalf("expected models.BookEvent, got %T", got)
		}
		if event.AssetID != "asset-book" || event.Market != "mkt-book" {
			t.Fatalf("unexpected event fields: %+v", event)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected book event")
	}
}

func TestCategoryFeedDispatchMarketResolvedBroadcastsAllAssets(t *testing.T) {
	feed := newCategoryFeed(CategoryATP)
	chA := make(chan any, 1)
	chB := make(chan any, 1)
	feed.subscribers["asset-a"] = []tokenMeta{{name: "A", ch: chA}}
	feed.subscribers["asset-b"] = []tokenMeta{{name: "B", ch: chB}}

	feed.dispatch([]byte(`{
		"event_type":"market_resolved",
		"id":"r1",
		"market":"market-1",
		"assets_ids":["asset-a","asset-b"],
		"winning_asset_id":"asset-a",
		"winning_outcome":"yes",
		"timestamp":"2026-01-01T00:00:00Z",
		"tags":["ATP"]
	}`))

	for token, ch := range map[string]chan any{"asset-a": chA, "asset-b": chB} {
		select {
		case got := <-ch:
			event, ok := got.(models.MarketResolvedEvent)
			if !ok {
				t.Fatalf("expected models.MarketResolvedEvent for %s, got %T", token, got)
			}
			if event.Market != "market-1" {
				t.Fatalf("unexpected market for %s: %+v", token, event)
			}
		case <-time.After(200 * time.Millisecond):
			t.Fatalf("expected market_resolved event for %s", token)
		}
	}
}

func TestCategoryFeedDispatchNewMarketInnerUnmarshalError(t *testing.T) {
	feed := newCategoryFeed(CategoryATP)
	called := make(chan struct{}, 1)
	feed.OnNewMarket(func(models.NewMarketEvent) {
		close(called)
	})

	feed.dispatch([]byte(`{"event_type":"new_market","assets_ids":"not-an-array"}`))

	select {
	case <-called:
		t.Fatal("OnNewMarket should not run when inner unmarshal fails")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestCategoryFeedNotifyNewMarketAsyncNilHandlerNoPanic(t *testing.T) {
	feed := newCategoryFeed(CategoryATP)
	feed.notifyNewMarketAsync(models.NewMarketEvent{EventType: "new_market", Slug: "atp-nil-handler"})
}

func TestCategoryFeedBroadcastToDropsWhenChannelFull(t *testing.T) {
	feed := newCategoryFeed(CategoryATP)
	ch := make(chan any, 1)
	ch <- models.PriceEvent{EventType: "price_change"}
	feed.subscribers["asset-full"] = []tokenMeta{{name: "full", ch: ch}}

	feed.broadcastTo("asset-full", models.BookEvent{EventType: "book"})

	select {
	case got := <-ch:
		pe, ok := got.(models.PriceEvent)
		if !ok || pe.EventType != "price_change" {
			t.Fatalf("expected original price event to remain, got %#v", got)
		}
	default:
		t.Fatal("expected buffered event to still be present")
	}
}

func TestCategoryFeedBroadcastErrorDropsWhenChannelFull(t *testing.T) {
	feed := newCategoryFeed(CategoryATP)
	ch := make(chan any, 1)
	first := errors.New("first")
	second := errors.New("second")
	ch <- first
	feed.subscribers["a"] = []tokenMeta{{name: "x", ch: ch}}

	feed.broadcastError(second)

	gotFirst := <-ch
	if gotFirst != first {
		t.Fatalf("expected first error to remain in buffer, got %v", gotFirst)
	}
}

func TestCategoryFeedDispatchNewMarketInvokesListenerAsync(t *testing.T) {
	feed := newCategoryFeed(CategoryATP)
	received := make(chan models.NewMarketEvent, 1)
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	feed.OnNewMarket(func(ev models.NewMarketEvent) {
		select {
		case entered <- struct{}{}:
		default:
		}
		<-release
		received <- ev
	})

	start := time.Now()
	feed.dispatch([]byte(`{
		"event_type":"new_market",
		"id":"new-1",
		"slug":"atp-rome-final",
		"sports_market_type":"moneyline",
		"market":"market-123",
		"condition_id":"cond-123",
		"assets_ids":["a1","a2"],
		"outcomes":["yes","no"],
		"timestamp":"2026-01-01T00:00:00Z"
	}`))

	select {
	case <-entered:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected new_market listener to start")
	}

	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Fatalf("expected dispatch to return without blocking listener, took %v", elapsed)
	}

	close(release)

	select {
	case got := <-received:
		if got.EventType != "new_market" || got.Slug != "atp-rome-final" || got.ConditionID != "cond-123" {
			t.Fatalf("unexpected new_market event fields: %+v", got)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected parsed new_market event")
	}
}

func TestCategoryFeedDispatchInvalidOrUnknownEvents(t *testing.T) {
	feed := newCategoryFeed(CategoryATP)
	ch := make(chan any, 1)
	feed.subscribers["asset-a"] = []tokenMeta{{name: "A", ch: ch}}

	feed.dispatch([]byte(`{not-json`))
	feed.dispatch([]byte(`{"event_type":"unknown","asset_id":"asset-a"}`))
	feed.dispatch([]byte(`{"event_type":"price_change","market":"m","price_changes":"bad-type"}`))
	feed.dispatch([]byte(`{"event_type":"book","asset_id":"asset-a","bids":"bad-type"}`))
	feed.dispatch([]byte(`{"event_type":"market_resolved","assets_ids":"bad-type"}`))

	select {
	case got := <-ch:
		t.Fatalf("did not expect a broadcast for invalid/unknown events, got %T", got)
	default:
	}
}

func TestCategoryFeedBroadcastErrorToAllSubscribers(t *testing.T) {
	feed := newCategoryFeed(CategoryATP)
	ch1 := make(chan any, 1)
	ch2 := make(chan any, 1)
	feed.subscribers["asset-a"] = []tokenMeta{{name: "A", ch: ch1}}
	feed.subscribers["asset-b"] = []tokenMeta{{name: "B", ch: ch2}}

	wantErr := errors.New("connection down")
	feed.broadcastError(wantErr)

	for token, ch := range map[string]chan any{"asset-a": ch1, "asset-b": ch2} {
		select {
		case got := <-ch:
			gotErr, ok := got.(error)
			if !ok {
				t.Fatalf("expected error for %s, got %T", token, got)
			}
			if gotErr != wantErr {
				t.Fatalf("expected same error instance for %s", token)
			}
		case <-time.After(200 * time.Millisecond):
			t.Fatalf("expected error for %s", token)
		}
	}
}

func TestCategoryFeedSleepOrDone(t *testing.T) {
	t.Run("returns false after timer when context active", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		feed := newCategoryFeed(CategoryATP)
		feed.ctx = ctx

		if done := feed.sleepOrDone(5 * time.Millisecond); done {
			t.Fatal("expected false when timer elapses without cancellation")
		}
	})

	t.Run("returns true when context cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		feed := newCategoryFeed(CategoryATP)
		feed.ctx = ctx
		cancel()

		if done := feed.sleepOrDone(50 * time.Millisecond); !done {
			t.Fatal("expected true when context is cancelled")
		}
	})

	t.Run("returns false immediately when non-positive duration and context active", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		feed := newCategoryFeed(CategoryATP)
		feed.ctx = ctx

		if done := feed.sleepOrDone(0); done {
			t.Fatal("expected false for non-positive duration with active context")
		}
	})

	t.Run("returns true immediately when non-positive duration and context cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		feed := newCategoryFeed(CategoryATP)
		feed.ctx = ctx
		cancel()

		if done := feed.sleepOrDone(-1); !done {
			t.Fatal("expected true for non-positive duration with cancelled context")
		}
	})
}

func TestCategoryFeedEnsureDefaults(t *testing.T) {
	assertFeedConfig := func(t *testing.T, feed *CategoryFeed, wantWSURL string, wantDialPtr uintptr, wantReconnectDelay, wantReconnectMaxDelay, wantHeartbeat time.Duration) {
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
		if feed.heartbeatInterval != wantHeartbeat {
			t.Fatalf("expected heartbeatInterval %v, got %v", wantHeartbeat, feed.heartbeatInterval)
		}
	}

	t.Run("fills all missing and invalid defaults", func(t *testing.T) {
		feed := &CategoryFeed{
			category:    CategoryATP,
			subscribers: make(map[string][]tokenMeta),
		}

		feed.ensureDefaults()

		assertFeedConfig(
			t,
			feed,
			wsURL,
			reflect.ValueOf(defaultWSDialContext).Pointer(),
			reconnectDelay,
			reconnectMaxDelay,
			heartbeatInterval,
		)
	})

	t.Run("preserves explicit non-zero custom values", func(t *testing.T) {
		customDial := func(ctx context.Context, url string, header http.Header) (*websocket.Conn, *http.Response, error) {
			return nil, nil, errors.New("expected not to dial in this test")
		}

		feed := &CategoryFeed{
			category:          CategoryATP,
			subscribers:       make(map[string][]tokenMeta),
			wsURL:             "ws://example.test/ws",
			dialContext:       customDial,
			reconnectDelay:    123 * time.Millisecond,
			reconnectMaxDelay: 456 * time.Millisecond,
			heartbeatInterval: 789 * time.Millisecond,
		}

		feed.ensureDefaults()

		assertFeedConfig(
			t,
			feed,
			"ws://example.test/ws",
			reflect.ValueOf(customDial).Pointer(),
			123*time.Millisecond,
			456*time.Millisecond,
			789*time.Millisecond,
		)
	})
}

func TestCategoryFeedSubscribeWithActiveConnSendsSubscribeMessage(t *testing.T) {
	conn, incoming, cleanup := newWebsocketTestConn(t)
	defer cleanup()

	feed := newCategoryFeed(CategoryATP)
	feed.conn = conn
	ch := make(chan any, 1)

	feed.Subscribe("asset-1", "market-1", ch)

	msg := mustReceiveWSMessage(t, incoming)
	if msg["type"] != "market" {
		t.Fatalf("expected market subscribe type, got %#v", msg["type"])
	}
	if enabled, ok := msg["custom_feature_enabled"].(bool); !ok || !enabled {
		t.Fatalf("expected custom_feature_enabled=true, got %#v", msg["custom_feature_enabled"])
	}
	assertAssetsIDsContains(t, msg, "asset-1")
}

func TestCategoryFeedUnsubscribeWithActiveConnOnlyOnLastSubscriber(t *testing.T) {
	conn, incoming, cleanup := newWebsocketTestConn(t)
	defer cleanup()

	feed := newCategoryFeed(CategoryATP)
	feed.conn = conn
	ch1 := make(chan any, 1)
	ch2 := make(chan any, 1)
	feed.subscribers["asset-1"] = []tokenMeta{
		{name: "first", ch: ch1},
		{name: "second", ch: ch2},
	}

	if err := feed.Unsubscribe("asset-1", ch1); err != nil {
		t.Fatalf("unsubscribe first subscriber returned error: %v", err)
	}
	select {
	case msg := <-incoming:
		t.Fatalf("did not expect unsubscribe message before last subscriber removed, got %#v", msg)
	case <-time.After(50 * time.Millisecond):
	}

	if err := feed.Unsubscribe("asset-1", ch2); err != nil {
		t.Fatalf("unsubscribe last subscriber returned error: %v", err)
	}
	msg := mustReceiveWSMessage(t, incoming)
	if msg["operation"] != "unsubscribe" {
		t.Fatalf("expected unsubscribe operation, got %#v", msg["operation"])
	}
	assertAssetsIDsContains(t, msg, "asset-1")
}

func TestCategoryFeedSendSubscribeWriteError(t *testing.T) {
	clientConn, serverConn, cleanup := newWebsocketPair(t)
	defer cleanup()

	feed := newCategoryFeed(CategoryATP)
	feed.conn = clientConn

	_ = clientConn.Close()
	_ = serverConn.Close()

	err := feed.sendSubscribe([]string{"asset-err"})
	if err == nil {
		t.Fatal("expected sendSubscribe to return write error when websocket is closed")
	}
}

func TestCategoryFeedSendSubscribeWithNilConnIsNoOp(t *testing.T) {
	feed := newCategoryFeed(CategoryATP)

	err := feed.sendSubscribe([]string{"asset-1"})
	if err != nil {
		t.Fatalf("expected nil error when connection is nil, got %v", err)
	}
}

func TestCategoryFeedSendUnsubscribeWriteError(t *testing.T) {
	clientConn, serverConn, cleanup := newWebsocketPair(t)
	defer cleanup()

	feed := newCategoryFeed(CategoryATP)
	feed.conn = clientConn

	_ = clientConn.Close()
	_ = serverConn.Close()

	err := feed.sendUnsubscribe([]string{"asset-err"})
	if err == nil {
		t.Fatal("expected sendUnsubscribe to return write error when websocket is closed")
	}
}

func TestCategoryFeedSendUnsubscribeWithNilConnIsNoOp(t *testing.T) {
	feed := newCategoryFeed(CategoryATP)

	err := feed.sendUnsubscribe([]string{"asset-1"})
	if err != nil {
		t.Fatalf("expected nil error when connection is nil, got %v", err)
	}
}

func TestCategoryFeedSubscribeWithActiveConnWriteFailureStillRegistersSubscriber(t *testing.T) {
	clientConn, serverConn, cleanup := newWebsocketPair(t)
	defer cleanup()

	feed := newCategoryFeed(CategoryATP)
	feed.conn = clientConn
	events := make(chan any, 1)

	_ = clientConn.Close()
	_ = serverConn.Close()

	feed.Subscribe("asset-err", "market-err", events)

	metas := feed.subscribers["asset-err"]
	if len(metas) != 1 {
		t.Fatalf("expected subscriber to be registered despite write failure, got %d", len(metas))
	}
}

func TestCategoryFeedSubscribeExistingTokenWithActiveConnSkipsDuplicateSend(t *testing.T) {
	conn, incoming, cleanup := newWebsocketTestConn(t)
	defer cleanup()

	feed := newCategoryFeed(CategoryATP)
	feed.conn = conn
	first := make(chan any, 1)
	second := make(chan any, 1)
	feed.subscribers["asset-1"] = []tokenMeta{{name: "first", ch: first}}

	feed.Subscribe("asset-1", "second", second)

	select {
	case msg := <-incoming:
		t.Fatalf("did not expect subscribe message for existing token, got %#v", msg)
	case <-time.After(50 * time.Millisecond):
	}

	metas := feed.subscribers["asset-1"]
	if len(metas) != 2 {
		t.Fatalf("expected second subscriber to be appended, got %d", len(metas))
	}
}

func newWebsocketTestConn(t *testing.T) (*websocket.Conn, <-chan map[string]any, func()) {
	t.Helper()

	incoming := make(chan map[string]any, 8)
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverConn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer close(incoming)
		defer serverConn.Close()

		for {
			var msg map[string]any
			if err := serverConn.ReadJSON(&msg); err != nil {
				return
			}
			incoming <- msg
		}
	}))

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	clientConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		server.Close()
		t.Fatalf("dial websocket test server: %v", err)
	}

	cleanup := func() {
		_ = clientConn.Close()
		server.Close()
	}
	return clientConn, incoming, cleanup
}

func newDrainReadWebsocketServer(t *testing.T) (string, <-chan *websocket.Conn, func()) {
	t.Helper()

	serverConns := make(chan *websocket.Conn, 4)
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverConn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		serverConns <- serverConn
		defer serverConn.Close()
		for {
			if _, _, err := serverConn.ReadMessage(); err != nil {
				return
			}
		}
	}))

	return "ws" + strings.TrimPrefix(server.URL, "http"), serverConns, server.Close
}

func mustReceiveWSMessage(t *testing.T, incoming <-chan map[string]any) map[string]any {
	t.Helper()
	select {
	case msg, ok := <-incoming:
		if !ok {
			t.Fatal("websocket incoming channel closed before message")
		}
		return msg
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for websocket message")
		return nil
	}
}

func assertAssetsIDsContains(t *testing.T, msg map[string]any, expected string) {
	t.Helper()
	raw, ok := msg["assets_ids"]
	if !ok {
		t.Fatalf("assets_ids missing from message: %#v", msg)
	}
	ids, ok := raw.([]any)
	if !ok {
		t.Fatalf("assets_ids has unexpected type %T", raw)
	}
	for _, id := range ids {
		if id == expected {
			return
		}
	}
	t.Fatalf("assets_ids did not contain %q: %#v", expected, ids)
}

func TestCategoryFeedStopWithNilConn(t *testing.T) {
	feed := newCategoryFeed(CategoryATP)
	feed.Stop()
	feed.Stop()
}

func TestMarketFeedManagerLifecycleAndLookup(t *testing.T) {
	manager := NewMarketFeedManager([]Category{CategoryATP, CategoryNBA})
	if len(manager.marketFeeds) != 2 {
		t.Fatalf("expected 2 feeds, got %d", len(manager.marketFeeds))
	}

	feedATP, err := manager.GetMarketFeed(CategoryATP)
	if err != nil {
		t.Fatalf("expected ATP feed, got error: %v", err)
	}
	if feedATP == nil || feedATP.category != CategoryATP {
		t.Fatalf("unexpected ATP feed: %+v", feedATP)
	}

	_, err = manager.GetMarketFeed(Category("UNKNOWN"))
	if err == nil {
		t.Fatal("expected error for unknown category")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	manager.Start(ctx)
	manager.Stop()
}

func TestCategoryFeedStartWithInjectedWSDialerURLReconnectsAndResubscribes(t *testing.T) {
	serverConns := make(chan *websocket.Conn, 4)
	incoming := make(chan map[string]any, 16)
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverConn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		serverConns <- serverConn
		defer serverConn.Close()
		for {
			var msg map[string]any
			if err := serverConn.ReadJSON(&msg); err != nil {
				return
			}
			incoming <- msg
		}
	}))
	defer server.Close()

	feed := newCategoryFeed(CategoryATP)
	feed.wsURL = "ws" + strings.TrimPrefix(server.URL, "http")
	feed.reconnectDelay = 10 * time.Millisecond
	feed.reconnectMaxDelay = 20 * time.Millisecond
	feed.heartbeatInterval = time.Hour
	var dialCalls atomic.Int32
	feed.dialContext = func(ctx context.Context, url string, header http.Header) (*websocket.Conn, *http.Response, error) {
		dialCalls.Add(1)
		return websocket.DefaultDialer.DialContext(ctx, url, header)
	}

	events := make(chan any, 4)
	feed.Subscribe("asset-1", "asset-one", events)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	feed.Start(ctx)
	defer feed.Stop()

	conn1 := mustReceiveServerConn(t, serverConns)
	first := mustReceiveWSMessage(t, incoming)
	if first["type"] != "market" {
		t.Fatalf("expected initial market subscribe, got %#v", first["type"])
	}
	assertAssetsIDsContains(t, first, "asset-1")

	if err := conn1.WriteMessage(websocket.TextMessage, []byte(`{
		"event_type":"price_change",
		"market":"mkt-live",
		"price_changes":[{"asset_id":"asset-1","price":"0.55","size":"10","side":"buy","hash":"h"}],
		"timestamp":"2026-01-01T00:00:00Z"
	}`)); err != nil {
		t.Fatalf("write server event: %v", err)
	}

	select {
	case got := <-events:
		event, ok := got.(models.PriceEvent)
		if !ok {
			t.Fatalf("expected models.PriceEvent, got %T", got)
		}
		if event.Market != "mkt-live" {
			t.Fatalf("unexpected market: %+v", event)
		}
	case <-time.After(400 * time.Millisecond):
		t.Fatal("expected price event from injected websocket server")
	}

	if err := conn1.Close(); err != nil {
		t.Fatalf("close first websocket connection: %v", err)
	}

	_ = mustReceiveServerConn(t, serverConns)
	second := mustReceiveWSMessage(t, incoming)
	if second["type"] != "market" {
		t.Fatalf("expected market subscribe after reconnect, got %#v", second["type"])
	}
	assertAssetsIDsContains(t, second, "asset-1")

	if dialCalls.Load() < 2 {
		t.Fatalf("expected dialer to be called at least twice, got %d", dialCalls.Load())
	}
}

func TestCategoryFeedStartUsesDefaultWSDialerWhenNil(t *testing.T) {
	serverConns := make(chan *websocket.Conn, 1)
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		serverConns <- conn
		defer conn.Close()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer server.Close()

	feed := newCategoryFeed(CategoryATP)
	feed.wsURL = "ws" + strings.TrimPrefix(server.URL, "http")
	feed.dialContext = nil // force ensureDefaults to wire defaultWSDialContext
	feed.reconnectDelay = 5 * time.Millisecond
	feed.reconnectMaxDelay = 10 * time.Millisecond
	feed.heartbeatInterval = time.Hour

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	feed.Start(ctx)
	defer feed.Stop()

	conn := mustReceiveServerConn(t, serverConns)
	_ = conn.Close()
}

func TestCategoryFeedConnectLoopDialFailureBackoffAndCancel(t *testing.T) {
	feed := newCategoryFeed(CategoryATP)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	feed.ctx = ctx
	feed.reconnectDelay = 2 * time.Millisecond
	feed.reconnectMaxDelay = 4 * time.Millisecond

	events := make(chan any, 8)
	feed.Subscribe("asset-1", "asset-one", events)

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

func TestCategoryFeedConnectLoopReturnsWhenDialHonorsCanceledContext(t *testing.T) {
	feed := newCategoryFeed(CategoryATP)
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

func TestCategoryFeedConnectLoopCancelAfterDialBeforeAssignClosesConnAndReturns(t *testing.T) {
	clientConn, serverConn, cleanup := newWebsocketPair(t)
	defer cleanup()

	feed := newCategoryFeed(CategoryATP)
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

func TestCategoryFeedConnectLoopReconnectResubscribeWriteFailureContinues(t *testing.T) {
	wsURL, serverConns, closeServer := newDrainReadWebsocketServer(t)
	defer closeServer()

	feed := newCategoryFeed(CategoryATP)
	feed.wsURL = wsURL
	feed.reconnectDelay = 10 * time.Millisecond
	feed.reconnectMaxDelay = 20 * time.Millisecond
	feed.heartbeatInterval = time.Hour

	events := make(chan any, 4)
	feed.Subscribe("asset-1", "asset-one", events)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	feed.ctx = ctx

	var dialCalls atomic.Int32
	feed.dialContext = func(ctx context.Context, url string, header http.Header) (*websocket.Conn, *http.Response, error) {
		conn, resp, err := websocket.DefaultDialer.DialContext(ctx, url, header)
		if err != nil {
			return nil, resp, err
		}
		if dialCalls.Add(1) == 2 {
			_ = conn.Close()
		}
		return conn, resp, nil
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		feed.connectLoop()
	}()

	firstConn := mustReceiveServerConn(t, serverConns)
	_ = firstConn.Close() // force readLoop to exit and reconnect

	_ = mustReceiveServerConn(t, serverConns) // second dial attempt happened

	deadline := time.Now().Add(700 * time.Millisecond)
	for time.Now().Before(deadline) {
		if dialCalls.Load() >= 2 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if dialCalls.Load() < 2 {
		t.Fatalf("expected at least 2 dial attempts, got %d", dialCalls.Load())
	}

	cancel()
	select {
	case <-done:
	case <-time.After(700 * time.Millisecond):
		t.Fatal("connectLoop did not stop after cancellation")
	}
}

func TestCategoryFeedConnectLoopCancelDuringBackoffSleepExits(t *testing.T) {
	feed := newCategoryFeed(CategoryATP)
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

	time.Sleep(20 * time.Millisecond) // ensure loop is in sleepOrDone backoff window
	cancel()

	select {
	case <-done:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("connectLoop did not exit after cancel during backoff")
	}
}

func TestCategoryFeedReadLoopContextCancellationDuringReadWait(t *testing.T) {
	clientConn, serverConn, cleanup := newWebsocketPair(t)
	defer cleanup()

	feed := newCategoryFeed(CategoryATP)
	ctx, cancel := context.WithCancel(context.Background())
	feed.ctx = ctx

	errCh := make(chan error, 1)
	go func() {
		errCh <- feed.readLoop(clientConn)
	}()

	time.Sleep(10 * time.Millisecond)
	cancel()
	_ = serverConn.Close()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected readLoop to return read error once blocked read is unblocked")
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatal("readLoop did not return after cancellation and connection close")
	}
}

func TestCategoryFeedReadLoopSkipsEmptyWhitespaceAndInvalidJSON(t *testing.T) {
	clientConn, serverConn, cleanup := newWebsocketPair(t)
	defer cleanup()

	feed := newCategoryFeed(CategoryATP)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	feed.ctx = ctx

	events := make(chan any, 1)
	feed.subscribers["asset-1"] = []tokenMeta{{name: "asset-1", ch: events}}

	errCh := make(chan error, 1)
	go func() {
		errCh <- feed.readLoop(clientConn)
	}()

	payloads := [][]byte{
		[]byte(""),
		[]byte("   \n\t "),
		[]byte("not-json"),
	}
	for _, payload := range payloads {
		if err := serverConn.WriteMessage(websocket.TextMessage, payload); err != nil {
			t.Fatalf("write test payload: %v", err)
		}
	}

	_ = serverConn.Close()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected readLoop to return read error after websocket close")
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatal("readLoop did not exit after websocket close")
	}

	select {
	case got := <-events:
		t.Fatalf("did not expect dispatch/broadcast for skipped payloads, got %T", got)
	default:
	}
}

func TestCategoryFeedReadLoopDispatchInnerUnmarshalErrors(t *testing.T) {
	clientConn, serverConn, cleanup := newWebsocketPair(t)
	defer cleanup()

	feed := newCategoryFeed(CategoryATP)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	feed.ctx = ctx

	events := make(chan any, 1)
	feed.subscribers["asset-1"] = []tokenMeta{{name: "asset-1", ch: events}}

	errCh := make(chan error, 1)
	go func() {
		errCh <- feed.readLoop(clientConn)
	}()

	messages := [][]byte{
		[]byte(`{"event_type":"price_change","market":"m","price_changes":"bad-type"}`),
		[]byte(`{"event_type":"book","asset_id":"asset-1","bids":"bad-type"}`),
		[]byte(`{"event_type":"market_resolved","assets_ids":"bad-type"}`),
	}
	for _, msg := range messages {
		if err := serverConn.WriteMessage(websocket.TextMessage, msg); err != nil {
			t.Fatalf("write test payload: %v", err)
		}
	}

	_ = serverConn.Close()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected readLoop to return read error after websocket close")
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatal("readLoop did not exit after websocket close")
	}

	select {
	case got := <-events:
		t.Fatalf("did not expect broadcast for malformed dispatch payloads, got %T", got)
	default:
	}
}

func TestCategoryFeedReadLoopReturnsNilWhenContextAlreadyCanceled(t *testing.T) {
	clientConn, _, cleanup := newWebsocketPair(t)
	defer cleanup()

	feed := newCategoryFeed(CategoryATP)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	feed.ctx = ctx

	if err := feed.readLoop(clientConn); err != nil {
		t.Fatalf("expected nil when context is already canceled, got %v", err)
	}
}

func TestCategoryFeedRunHeartbeatTickSuccessAndWriteFailure(t *testing.T) {
	clientConn, serverConn, cleanup := newWebsocketPair(t)
	defer cleanup()

	feed := newCategoryFeed(CategoryATP)
	feed.heartbeatInterval = 10 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	received := make(chan struct{}, 1)
	go func() {
		_, _, err := serverConn.ReadMessage()
		if err == nil {
			received <- struct{}{}
		}
		_ = serverConn.Close()
	}()

	done := make(chan struct{})
	go func() {
		defer close(done)
		feed.runHeartbeat(clientConn, ctx)
	}()

	select {
	case <-received:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("expected heartbeat ping message to be written")
	}

	time.Sleep(40 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("runHeartbeat did not stop after context cancellation")
	}
}

func mustReceiveServerConn(t *testing.T, conns <-chan *websocket.Conn) *websocket.Conn {
	t.Helper()
	select {
	case c := <-conns:
		return c
	case <-time.After(400 * time.Millisecond):
		t.Fatal("timed out waiting for server websocket connection")
		return nil
	}
}

func newWebsocketPair(t *testing.T) (*websocket.Conn, *websocket.Conn, func()) {
	t.Helper()

	upgrader := websocket.Upgrader{}
	serverConns := make(chan *websocket.Conn, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		serverConns <- conn
	}))

	clientConn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		server.Close()
		t.Fatalf("dial websocket test server: %v", err)
	}

	var serverConn *websocket.Conn
	select {
	case serverConn = <-serverConns:
	case <-time.After(300 * time.Millisecond):
		_ = clientConn.Close()
		server.Close()
		t.Fatal("timed out waiting for websocket server conn")
	}

	cleanup := func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
		server.Close()
	}

	return clientConn, serverConn, cleanup
}
