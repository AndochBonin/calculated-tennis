package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AndochBonin/E3/polymarket/clob"
	"github.com/AndochBonin/E3/polymarket/core"
	"github.com/AndochBonin/E3/polymarket/gamma"
	"github.com/AndochBonin/E3/polymarket/models"
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
