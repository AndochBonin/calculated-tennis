package core

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/AndochBonin/polymarket/models"
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

func TestCategoryFeedDispatchSportEventDoesNotBroadcast(t *testing.T) {
	feed := newCategoryFeed(CategoryATP)
	ch := make(chan any, 1)
	feed.subscribers["asset-a"] = []tokenMeta{{name: "A", ch: ch}}

	feed.dispatch([]byte(`{
		"event_type":"sport_event",
		"asset_id":"asset-a",
		"slug":"atp-match",
		"live":true,
		"ended":false,
		"score":"1-0",
		"period":"set1",
		"elapsed":"10",
		"last_update":"now"
	}`))

	select {
	case got := <-ch:
		t.Fatalf("did not expect sport_event to be broadcast, got %T", got)
	default:
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

func TestCategoryFeedDispatchInvalidOrUnknownEvents(t *testing.T) {
	feed := newCategoryFeed(CategoryATP)
	ch := make(chan any, 1)
	feed.subscribers["asset-a"] = []tokenMeta{{name: "A", ch: ch}}

	feed.dispatch([]byte(`{not-json`))
	feed.dispatch([]byte(`{"event_type":"unknown","asset_id":"asset-a"}`))
	feed.dispatch([]byte(`{"event_type":"price_change","market":"m","price_changes":"bad-type"}`))
	feed.dispatch([]byte(`{"event_type":"sport_event","slug":1}`))
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

func TestCategoryFeedSubscribeWithActiveConnSendsSubscribeMessage(t *testing.T) {
	conn, incoming, cleanup := newWebsocketTestConn(t)
	defer cleanup()

	feed := newCategoryFeed(CategoryATP)
	feed.conn = conn
	ch := make(chan any, 1)

	if err := feed.Subscribe("asset-1", "market-1", ch); err != nil {
		t.Fatalf("subscribe returned error: %v", err)
	}

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

func TestFeedManagerLifecycleAndLookup(t *testing.T) {
	manager := NewFeedManager([]Category{CategoryATP, CategoryNBA})
	if len(manager.feeds) != 2 {
		t.Fatalf("expected 2 feeds, got %d", len(manager.feeds))
	}

	feedATP, err := manager.Feed(CategoryATP)
	if err != nil {
		t.Fatalf("expected ATP feed, got error: %v", err)
	}
	if feedATP == nil || feedATP.category != CategoryATP {
		t.Fatalf("unexpected ATP feed: %+v", feedATP)
	}

	_, err = manager.Feed(Category("UNKNOWN"))
	if err == nil {
		t.Fatal("expected error for unknown category")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	manager.Start(ctx)
	manager.Stop()
}
