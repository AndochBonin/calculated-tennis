package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	mmclient "github.com/AndochBonin/calculated-tennis/moneymanager/pkg/client"
	"github.com/AndochBonin/calculated-tennis/moneymanager/pkg/order"
	"github.com/AndochBonin/calculated-tennis/moneymanager/pkg/testserver"
	"github.com/AndochBonin/calculated-tennis/polymarket/clob"
	"github.com/AndochBonin/calculated-tennis/polymarket/core"
	"github.com/AndochBonin/calculated-tennis/polymarket/gamma"
	"github.com/AndochBonin/calculated-tennis/polymarket/models"
)

func TestMain(m *testing.M) {
	origCommandLine := flag.CommandLine
	origArgs := os.Args
	origVerboseIDs := core.VerboseIDs

	isolated := flag.NewFlagSet("main-test", flag.ContinueOnError)
	isolated.SetOutput(io.Discard)
	flag.CommandLine = isolated
	os.Args = []string{"main-test", "-v"}
	core.VerboseIDs = false

	parseVerboseFlag()
	if !core.VerboseIDs {
		flag.CommandLine = origCommandLine
		os.Args = origArgs
		core.VerboseIDs = origVerboseIDs
		os.Exit(1)
	}

	// Keep other tests isolated from global flag and verbose state changes.
	flag.CommandLine = origCommandLine
	os.Args = origArgs
	core.VerboseIDs = origVerboseIDs

	os.Exit(m.Run())
}

func TestLogLevelFromEnv(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     slog.Level
	}{
		{name: "debug", envValue: "debug", want: slog.LevelDebug},
		{name: "info", envValue: "info", want: slog.LevelInfo},
		{name: "warn", envValue: "warn", want: slog.LevelWarn},
		{name: "warning", envValue: "warning", want: slog.LevelWarn},
		{name: "error", envValue: "error", want: slog.LevelError},
		{name: "empty", envValue: "", want: slog.LevelInfo},
		{name: "invalid", envValue: "nope", want: slog.LevelInfo},
		{name: "trim spaces", envValue: "  DEBUG ", want: slog.LevelDebug},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("LOG_LEVEL", tc.envValue)

			got := logLevelFromEnv()
			if got != tc.want {
				t.Fatalf("logLevelFromEnv() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSetupLoggingWarnsOnInvalidLogLevel(t *testing.T) {
	t.Setenv("LOG_LEVEL", "invalid-level")

	origLogger := slog.Default()
	t.Cleanup(func() {
		slog.SetDefault(origLogger)
	})

	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() failed: %v", err)
	}
	os.Stderr = w
	t.Cleanup(func() {
		os.Stderr = origStderr
		_ = r.Close()
		_ = w.Close()
	})

	setupLogging()

	_ = w.Close()

	done := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()

	select {
	case out := <-done:
		if !strings.Contains(out, "invalid LOG_LEVEL, defaulting to info") {
			t.Fatalf("expected warning in output, got: %q", out)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for logger output")
	}
}

type fakeFeedManager struct {
	marketFeed *core.MarketFeed
	feedErr    error
	started    bool
	stopped    bool
}

func (f *fakeFeedManager) Start(context.Context) { f.started = true }
func (f *fakeFeedManager) Stop()                 { f.stopped = true }
func (f *fakeFeedManager) GetMarketFeed(core.Category) (*core.MarketFeed, error) {
	if f.feedErr != nil {
		return nil, f.feedErr
	}
	return f.marketFeed, nil
}

func TestStartATPStackSuccess(t *testing.T) {
	origNewMarketFeedManager := newMarketFeedManager
	t.Cleanup(func() { newMarketFeedManager = origNewMarketFeedManager })

	wantMarketFeed := &core.MarketFeed{}
	fm := &fakeFeedManager{marketFeed: wantMarketFeed}
	newMarketFeedManager = func([]core.Category) feedManagerRunner { return fm }

	gotManager, gotMarketFeed, err := startATPStack(context.Background())
	if err != nil {
		t.Fatalf("startATPStack() error = %v", err)
	}
	if gotManager != fm {
		t.Fatalf("startATPStack() manager mismatch")
	}
	if gotMarketFeed != wantMarketFeed {
		t.Fatalf("startATPStack() feed mismatch")
	}
	if !fm.started {
		t.Fatal("expected feed manager Start to be called")
	}
	if fm.stopped {
		t.Fatal("did not expect feed manager Stop on success")
	}
}

func TestStartATPStackFeedErrorStopsManager(t *testing.T) {
	origNewMarketFeedManager := newMarketFeedManager
	t.Cleanup(func() { newMarketFeedManager = origNewMarketFeedManager })

	wantErr := errors.New("feed unavailable")
	fm := &fakeFeedManager{feedErr: wantErr}
	newMarketFeedManager = func([]core.Category) feedManagerRunner { return fm }

	gotManager, gotMarketFeed, err := startATPStack(context.Background())
	if !errors.Is(err, wantErr) {
		t.Fatalf("startATPStack() error = %v, want %v", err, wantErr)
	}
	if gotManager != nil || gotMarketFeed != nil {
		t.Fatalf("startATPStack() = (%v, %v), want nil,nil on error", gotManager, gotMarketFeed)
	}
	if !fm.started || !fm.stopped {
		t.Fatalf("expected started and stopped to be true, got started=%v stopped=%v", fm.started, fm.stopped)
	}
}

func TestNewClientsUsesFactories(t *testing.T) {
	origGammaFactory := newGammaClient
	origClobFactory := newClobClient
	t.Cleanup(func() {
		newGammaClient = origGammaFactory
		newClobClient = origClobFactory
	})

	var gammaCalls int
	var clobCalls int
	gammaPtr := &gamma.Client{}
	clobPtr := &clob.Client{}

	newGammaClient = func(_ ...gamma.Option) *gamma.Client {
		gammaCalls++
		return gammaPtr
	}
	newClobClient = func(_ ...clob.Option) *clob.Client {
		clobCalls++
		return clobPtr
	}

	gotGamma, gotClob := newClients()
	if gotGamma != gammaPtr || gotClob != clobPtr {
		t.Fatal("newClients() did not use injected factories")
	}
	if gammaCalls != 1 || clobCalls != 1 {
		t.Fatalf("factory calls = gamma:%d clob:%d, want 1 each", gammaCalls, clobCalls)
	}
}

type fakeGammaFetcher struct {
	result []models.GammaMarket
	err    error
	params gamma.MarketsParams
	called bool
}

func (f *fakeGammaFetcher) GetMarkets(_ context.Context, params gamma.MarketsParams) ([]models.GammaMarket, error) {
	f.called = true
	f.params = params
	return f.result, f.err
}

func TestFetchATPMarkets(t *testing.T) {
	wantMarkets := []models.GammaMarket{{ConditionID: "cond-1"}}
	fetcher := &fakeGammaFetcher{result: wantMarkets}
	got, err := fetchATPMarkets(context.Background(), fetcher)
	if err != nil {
		t.Fatalf("fetchATPMarkets() error = %v", err)
	}
	if len(got) != 1 || got[0].ConditionID != "cond-1" {
		t.Fatalf("fetchATPMarkets() = %#v, want %#v", got, wantMarkets)
	}
	if !fetcher.called {
		t.Fatal("expected GetMarkets to be called")
	}
	if fetcher.params.TagID != int(core.TagATP) {
		t.Fatalf("TagID = %d, want %d", fetcher.params.TagID, int(core.TagATP))
	}
	if fetcher.params.Closed == nil || *fetcher.params.Closed {
		t.Fatal("expected Closed to be non-nil false")
	}
	if len(fetcher.params.SportsMarketTypes) != 1 || fetcher.params.SportsMarketTypes[0] != "moneyline" {
		t.Fatalf("SportsMarketTypes = %#v, want [moneyline]", fetcher.params.SportsMarketTypes)
	}
}

type fakeTrader struct {
	startErr   error
	startCalls int
	stopCalls  int
	signalCh   chan core.TradeSignal
}

func (f *fakeTrader) Start(context.Context) error {
	f.startCalls++
	return f.startErr
}
func (f *fakeTrader) Stop()                            { f.stopCalls++ }
func (f *fakeTrader) Signals() <-chan core.TradeSignal { return f.signalCh }

func TestStartATPTradersSkipsFailedStarts(t *testing.T) {
	origFactory := newATPTrader
	t.Cleanup(func() { newATPTrader = origFactory })

	okTrader := &fakeTrader{signalCh: make(chan core.TradeSignal)}
	failTrader := &fakeTrader{startErr: errors.New("boom"), signalCh: make(chan core.TradeSignal)}
	created := map[string]*fakeTrader{}

	newATPTrader = func(_ *gamma.Client, _ *clob.Client, _ *core.MarketFeed, _ *core.SportsFeed, market models.GammaMarket) traderRunner {
		if market.ConditionID == "bad" {
			created[market.ConditionID] = failTrader
			return failTrader
		}
		created[market.ConditionID] = okTrader
		return okTrader
	}

	tradersOnly, startedIDs := startATPTraders(context.Background(), &gamma.Client{}, &clob.Client{}, &core.MarketFeed{}, nil, []models.GammaMarket{
		{ConditionID: "ok"},
		{ConditionID: "bad"},
	})

	if len(tradersOnly) != 1 {
		t.Fatalf("len(startATPTraders(...)) = %d, want 1", len(tradersOnly))
	}
	if tradersOnly[0] != okTrader {
		t.Fatal("expected only successful trader in result")
	}
	if len(startedIDs) != 1 || startedIDs[0] != "ok" {
		t.Fatalf("started IDs = %#v, want [ok]", startedIDs)
	}
	if created["ok"].startCalls != 1 || created["bad"].startCalls != 1 {
		t.Fatalf("unexpected start calls: ok=%d bad=%d", created["ok"].startCalls, created["bad"].startCalls)
	}
}

func TestTraderRegistryForwardsSignalsFromRegisteredTraders(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t1 := &fakeTrader{signalCh: make(chan core.TradeSignal, 1)}
	t2 := &fakeTrader{signalCh: make(chan core.TradeSignal, 1)}

	t1.signalCh <- core.TradeSignal{TokenID: "t1"}
	t2.signalCh <- core.TradeSignal{TokenID: "t2"}
	close(t1.signalCh)
	close(t2.signalCh)

	registry := newTraderRegistry(ctx, 2)
	registry.Register(t1)
	registry.Register(t2)

	got := map[string]bool{}
	timeout := time.After(2 * time.Second)
	for len(got) < 2 {
		select {
		case sig := <-registry.Signals():
			got[sig.TokenID] = true
		case <-timeout:
			t.Fatalf("timed out waiting for forwarded signals: %#v", got)
		}
	}

	registry.WaitForForwarders()
	cancel()
}

func TestTraderRegistryStopsForwardersOnContextCancelWhileWaitingForSignal(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	tr := &fakeTrader{signalCh: make(chan core.TradeSignal)}
	registry := newTraderRegistry(ctx, 1)
	registry.Register(tr)

	cancel()

	done := make(chan struct{})
	go func() {
		registry.WaitForForwarders()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for forwarder to stop after context cancellation")
	}
}

func TestTraderRegistryStopsForwardersOnContextCancelWhileForwarding(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	registry := newTraderRegistry(ctx, 100)
	for i := 0; i < 101; i++ {
		ch := make(chan core.TradeSignal, 1)
		ch <- core.TradeSignal{TokenID: "tok"}
		close(ch)
		registry.Register(&fakeTrader{signalCh: ch})
	}

	cancel()

	done := make(chan struct{})
	go func() {
		registry.WaitForForwarders()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for blocked forwarder to stop after context cancellation")
	}
}

func TestRunSignalLoggerWithHandler(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signalCh := make(chan core.TradeSignal, 1)
	signalCh <- core.TradeSignal{TokenID: "tok-1"}
	close(signalCh)

	var calls atomic.Int32
	done := make(chan struct{})
	runSignalLoggerWithHandler(ctx, signalCh, func(sig core.TradeSignal) {
		if sig.TokenID == "tok-1" {
			calls.Add(1)
		}
		if calls.Load() == 1 {
			close(done)
		}
	})

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for signal handler call")
	}
}

func TestRunSignalLoggerWithHandlerContextDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	signalCh := make(chan core.TradeSignal)
	var calls atomic.Int32

	runSignalLoggerWithHandler(ctx, signalCh, func(core.TradeSignal) {
		calls.Add(1)
	})

	time.Sleep(25 * time.Millisecond)
	if calls.Load() != 0 {
		t.Fatalf("handler called %d times, want 0", calls.Load())
	}
}

func TestRunSignalLoggerWithHandlerNilHandler(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	signalCh := make(chan core.TradeSignal, 1)
	signalCh <- core.TradeSignal{TokenID: "tok-1"}
	close(signalCh)

	// Should not panic when onSignal is nil; noopSignalHandler should be used.
	runSignalLoggerWithHandler(ctx, signalCh, nil)
	time.Sleep(25 * time.Millisecond)
}

func TestRunSignalLoggerWrapper(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	signalCh := make(chan core.TradeSignal, 1)
	signalCh <- core.TradeSignal{TokenID: "tok-1"}
	close(signalCh)

	runSignalLogger(ctx, signalCh)
	time.Sleep(25 * time.Millisecond)
}

func TestShutdownStopsAndCancels(t *testing.T) {
	ctx, ctxCancel := context.WithCancel(context.Background())
	t.Cleanup(ctxCancel)

	t1 := &fakeTrader{signalCh: make(chan core.TradeSignal)}
	t2 := &fakeTrader{signalCh: make(chan core.TradeSignal)}
	fm := &fakeFeedManager{}

	var canceled atomic.Bool
	cancel := func() {
		canceled.Store(true)
		ctxCancel()
	}

	registry := newTraderRegistry(ctx, 10)
	registry.Register(t1)
	registry.Register(t2)

	shutdown(registry, cancel, fm, nil)

	if t1.stopCalls != 1 || t2.stopCalls != 1 {
		t.Fatalf("stop calls = t1:%d t2:%d, want 1 each", t1.stopCalls, t2.stopCalls)
	}
	if !canceled.Load() {
		t.Fatal("expected cancel function to be called")
	}
	if !fm.stopped {
		t.Fatal("expected feed manager Stop to be called")
	}
}

const testSignalExecutorPrivKeyHex = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
const testSignalExecutorDeposit = "0x70997970C51812dc3A010C7d01b50e0d17dc79C8"

func startTestMoneyManagerForSignals(t *testing.T) *mmclient.Client {
	t.Helper()
	addr, cleanup, err := testserver.Start(testserver.Config{
		PrivateKeyHex:        testSignalExecutorPrivKeyHex,
		DefaultDepositWallet: testSignalExecutorDeposit,
		DefaultSignatureType: 3,
	})
	if err != nil {
		t.Fatalf("testserver.Start: %v", err)
	}
	t.Cleanup(cleanup)

	c, err := mmclient.Dial(context.Background(), addr)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestRunSignalExecutorProcessAndPlace(t *testing.T) {
	mm := startTestMoneyManagerForSignals(t)

	var placeCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/order" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		placeCalls.Add(1)
		var req models.PlaceOrderRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if req.Order.TokenID == "" || req.Order.Signature == "" {
			t.Fatalf("order missing token_id or signature: %+v", req.Order)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(models.PlaceOrderResponse{
			Success: true,
			OrderID: "order-123",
			Status:  "live",
		})
	}))
	t.Cleanup(srv.Close)

	t.Setenv("POLYMARKET_API_KEY", "550e8400-e29b-41d4-a716-446655440000")
	t.Setenv("POLYMARKET_API_SECRET", "AQIDBA==")
	t.Setenv("POLYMARKET_PASSPHRASE", "p")
	t.Setenv("POLYMARKET_ADDRESS", "0xabc")
	clobClient := clob.NewClient(clob.WithBaseURL(srv.URL))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signalCh := make(chan core.TradeSignal, 1)
	signalCh <- core.TradeSignal{
		TokenID:        "12345",
		Side:           models.OrderSideBuy,
		Price:          "0.50",
		WinProbability: 0.55,
	}
	close(signalCh)

	done := make(chan struct{})
	runSignalExecutorWithHandler(ctx, signalCh, clobClient, mm, func(ctx context.Context, c signalClobClient, m signalMoneyManager, sig core.TradeSignal) {
		executeTradeSignal(ctx, c, m, sig)
		close(done)
	})

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for signal executor")
	}
	if placeCalls.Load() != 1 {
		t.Fatalf("PlaceOrder calls = %d, want 1", placeCalls.Load())
	}
}

func TestRunSignalExecutorRejectsSellWithoutPlace(t *testing.T) {
	mm := startTestMoneyManagerForSignals(t)

	var placeCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		placeCalls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	t.Setenv("POLYMARKET_API_KEY", "550e8400-e29b-41d4-a716-446655440000")
	t.Setenv("POLYMARKET_API_SECRET", "AQIDBA==")
	t.Setenv("POLYMARKET_PASSPHRASE", "p")
	t.Setenv("POLYMARKET_ADDRESS", "0xabc")
	clobClient := clob.NewClient(clob.WithBaseURL(srv.URL))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signalCh := make(chan core.TradeSignal, 1)
	signalCh <- core.TradeSignal{
		TokenID:        "12345",
		Side:           models.OrderSideSell,
		Price:          "0.50",
		WinProbability: 0.55,
	}
	close(signalCh)

	done := make(chan struct{})
	runSignalExecutorWithHandler(ctx, signalCh, clobClient, mm, func(ctx context.Context, c signalClobClient, m signalMoneyManager, sig core.TradeSignal) {
		executeTradeSignal(ctx, c, m, sig)
		close(done)
	})

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for signal executor")
	}
	if placeCalls.Load() != 0 {
		t.Fatalf("PlaceOrder calls = %d, want 0", placeCalls.Load())
	}
}

func TestRunSignalExecutorWrapper(t *testing.T) {
	mm := startTestMoneyManagerForSignals(t)

	var placeCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/time":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("1730000999"))
		case "/order":
			placeCalls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(models.PlaceOrderResponse{Success: true, OrderID: "oid"})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	t.Setenv("POLYMARKET_API_KEY", "550e8400-e29b-41d4-a716-446655440000")
	t.Setenv("POLYMARKET_API_SECRET", "AQIDBA==")
	t.Setenv("POLYMARKET_PASSPHRASE", "p")
	t.Setenv("POLYMARKET_ADDRESS", "0xabc")
	clobClient := clob.NewClient(clob.WithBaseURL(srv.URL), clob.WithServerSignedTime(true))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signalCh := make(chan core.TradeSignal, 1)
	signalCh <- core.TradeSignal{
		TokenID:        "12345",
		Side:           models.OrderSideBuy,
		Price:          "0.50",
		WinProbability: 0.55,
	}
	close(signalCh)

	runSignalExecutor(ctx, signalCh, clobClient, mm)

	deadline := time.After(3 * time.Second)
	for placeCalls.Load() < 1 {
		select {
		case <-deadline:
			t.Fatalf("PlaceOrder calls = %d, want 1", placeCalls.Load())
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestExecuteTradeSignalSkipsIncomplete(t *testing.T) {
	var processCalls atomic.Int32
	fakeMM := &fakeSignalMoneyManager{onProcess: func() { processCalls.Add(1) }}
	fakeClob := &fakeSignalClobClient{}

	executeTradeSignal(context.Background(), fakeClob, fakeMM, core.TradeSignal{TokenID: "tok"})
	if processCalls.Load() != 0 {
		t.Fatalf("ProcessSignal calls = %d, want 0", processCalls.Load())
	}

	executeTradeSignal(context.Background(), fakeClob, fakeMM, core.TradeSignal{
		TokenID: "tok",
		Side:    models.OrderSideBuy,
		Price:   "0.50",
	})
	if processCalls.Load() != 0 {
		t.Fatalf("ProcessSignal calls with missing win_probability = %d, want 0", processCalls.Load())
	}
}

type fakeSignalMoneyManager struct {
	onProcess func()
}

func (f *fakeSignalMoneyManager) ProcessSignal(ctx context.Context, p mmclient.ProcessSignalParams) (*order.Payload, error) {
	if f.onProcess != nil {
		f.onProcess()
	}
	return &order.Payload{Signature: "0xsig", TokenID: p.TokenID}, nil
}

type fakeSignalClobClient struct {
	ts int64
}

func (f *fakeSignalClobClient) OrderMessageTimestampMillis() int64 {
	if f.ts != 0 {
		return f.ts
	}
	return 1_700_000_123
}

func (f *fakeSignalClobClient) PlaceOrder(payload *models.OrderPayload, owner string, orderType models.OrderType) (*models.PlaceOrderResponse, error) {
	return &models.PlaceOrderResponse{Success: true, OrderID: "oid"}, nil
}
