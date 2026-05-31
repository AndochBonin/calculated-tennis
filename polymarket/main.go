package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	mmclient "github.com/AndochBonin/E3/moneymanager/pkg/client"
	"github.com/AndochBonin/E3/moneymanager/pkg/order"
	"github.com/AndochBonin/E3/moneymanager/pkg/risk"
	"github.com/AndochBonin/E3/polymarket/clob"
	"github.com/AndochBonin/E3/polymarket/core"
	"github.com/AndochBonin/E3/polymarket/gamma"
	"github.com/AndochBonin/E3/polymarket/models"
	"github.com/AndochBonin/E3/polymarket/secrets"
	"github.com/joho/godotenv"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type gammaMarketsFetcher interface {
	GetMarkets(context.Context, gamma.MarketsParams) ([]models.GammaMarket, error)
}

type traderRunner interface {
	Start(context.Context) error
	Stop()
	Signals() <-chan core.TradeSignal
}

type traderRegistry struct {
	ctx       context.Context
	signalCh  chan core.TradeSignal
	forwardWg sync.WaitGroup

	mu      sync.Mutex
	traders []traderRunner
}

func newTraderRegistry(ctx context.Context, bufferSize int) *traderRegistry {
	if bufferSize <= 0 {
		bufferSize = 1
	}
	return &traderRegistry{
		ctx:      ctx,
		signalCh: make(chan core.TradeSignal, bufferSize),
	}
}

func (r *traderRegistry) Register(tr traderRunner) {
	if r == nil || tr == nil {
		return
	}

	r.mu.Lock()
	r.traders = append(r.traders, tr)
	r.mu.Unlock()

	r.forwardWg.Add(1)
	go func(t traderRunner) {
		defer r.forwardWg.Done()
		for {
			select {
			case <-r.ctx.Done():
				return
			case sig, ok := <-t.Signals():
				if !ok {
					return
				}
				select {
				case r.signalCh <- sig:
				case <-r.ctx.Done():
					return
				}
			}
		}
	}(tr)
}

func (r *traderRegistry) Signals() <-chan core.TradeSignal {
	if r == nil {
		return nil
	}
	return r.signalCh
}

func (r *traderRegistry) StopAll() {
	if r == nil {
		return
	}
	r.mu.Lock()
	traders := append([]traderRunner(nil), r.traders...)
	r.mu.Unlock()
	for _, tr := range traders {
		tr.Stop()
	}
}

func (r *traderRegistry) WaitForForwarders() {
	if r == nil {
		return
	}
	r.forwardWg.Wait()
}

type feedManagerRunner interface {
	Start(context.Context)
	Stop()
	GetMarketFeed(core.Category) (*core.MarketFeed, error)
}

var (
	newMarketFeedManager = func(categories []core.Category) feedManagerRunner {
		return core.NewMarketFeedManager(categories)
	}
	newGammaClient = gamma.NewClient
	newClobClient      = clob.NewClient
	dialMoneyManager   = mmclient.DialFromEnv
	newSportsFeed      = core.NewSportsFeed
	newATPTrader   = func(gammaClient *gamma.Client, clobClient *clob.Client, marketFeed *core.MarketFeed, sportsFeed *core.SportsFeed, market models.GammaMarket) traderRunner {
		return core.NewATPTrader(gammaClient, clobClient, marketFeed, sportsFeed, market)
	}
)

func noopSignalHandler(core.TradeSignal) {
	// Explicit no-op for tests that only assert lifecycle behavior.
}

func logLevelFromEnv() slog.Level {
	levelStr := strings.TrimSpace(os.Getenv("LOG_LEVEL"))
	if levelStr == "" {
		return slog.LevelInfo
	}

	switch strings.ToLower(levelStr) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func parseVerboseFlag() {
	verboseIDs := flag.Bool("v", false, "enable verbose token/condition ID logging")
	flag.Parse()
	core.VerboseIDs = *verboseIDs
}

func setupLogging() {
	logLevel := new(slog.LevelVar)
	lvl := logLevelFromEnv()
	logLevel.Set(lvl)

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
	})))

	if raw := strings.TrimSpace(os.Getenv("LOG_LEVEL")); raw != "" {
		if lvl == slog.LevelInfo && strings.ToLower(raw) != "info" {
			slog.Warn("invalid LOG_LEVEL, defaulting to info", "value", raw)
		}
	}
}

func startATPStack(ctx context.Context) (feedManagerRunner, *core.MarketFeed, error) {
	feedManager := newMarketFeedManager([]core.Category{core.CategoryATP})
	feedManager.Start(ctx)
	marketFeed, err := feedManager.GetMarketFeed(core.CategoryATP)
	if err != nil {
		feedManager.Stop()
		return nil, nil, err
	}
	return feedManager, marketFeed, nil
}

func newClients() (*gamma.Client, *clob.Client) {
	return newGammaClient(), newClobClient()
}

func fetchATPMarkets(ctx context.Context, gammaClient gammaMarketsFetcher) ([]models.GammaMarket, error) {
	closed := false
	return gammaClient.GetMarkets(ctx, gamma.MarketsParams{
		Limit:             50,
		TagID:             int(core.TagATP),
		Closed:            &closed,
		SportsMarketTypes: []string{"moneyline"},
	})
}

func startATPTraders(
	ctx context.Context,
	gammaClient *gamma.Client,
	clobClient *clob.Client,
	marketFeed *core.MarketFeed,
	sportsFeed *core.SportsFeed,
	markets []models.GammaMarket,
) ([]traderRunner, []string) {
	var atpTraders []traderRunner
	var startedConditionIDs []string
	for _, m := range markets {
		tr := newATPTrader(gammaClient, clobClient, marketFeed, sportsFeed, m)
		if err := tr.Start(ctx); err != nil {
			slog.Error("skip market",
				append([]any{"err", err}, core.AppendVerboseIDs("condition_id", m.ConditionID)...)...)
			continue
		}
		atpTraders = append(atpTraders, tr)
		startedConditionIDs = append(startedConditionIDs, m.ConditionID)
	}
	return atpTraders, startedConditionIDs
}

type signalMoneyManager interface {
	ProcessSignal(ctx context.Context, p mmclient.ProcessSignalParams) (*order.Payload, error)
}

type signalClobClient interface {
	OrderMessageTimestampMillis() int64
	PlaceOrder(payload *models.OrderPayload, owner string, orderType models.OrderType) (*models.PlaceOrderResponse, error)
}

// runSignalExecutor drains trade signals: ProcessSignal via Money Manager, then PlaceOrder on CLOB.
func runSignalExecutor(ctx context.Context, signalCh <-chan core.TradeSignal, clobClient signalClobClient, mm signalMoneyManager) {
	runSignalExecutorWithHandler(ctx, signalCh, clobClient, mm, executeTradeSignal)
}

func runSignalExecutorWithHandler(
	ctx context.Context,
	signalCh <-chan core.TradeSignal,
	clobClient signalClobClient,
	mm signalMoneyManager,
	onSignal func(context.Context, signalClobClient, signalMoneyManager, core.TradeSignal),
) {
	if onSignal == nil {
		onSignal = executeTradeSignal
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case sig, ok := <-signalCh:
				if !ok {
					return
				}
				onSignal(ctx, clobClient, mm, sig)
			}
		}
	}()
}

func executeTradeSignal(ctx context.Context, clobClient signalClobClient, mm signalMoneyManager, sig core.TradeSignal) {
	logFields := append([]any{"side", sig.Side, "price", sig.Price, "neg_risk", sig.NegRisk},
		core.AppendVerboseIDs("token_id", sig.TokenID)...)

	if strings.TrimSpace(sig.TokenID) == "" {
		slog.Warn("trade signal missing token_id", logFields...)
		return
	}
	if strings.TrimSpace(sig.Price) == "" {
		slog.Warn("trade signal missing price", logFields...)
		return
	}
	if sig.Side != models.OrderSideBuy && sig.Side != models.OrderSideSell {
		slog.Warn("trade signal invalid side", logFields...)
		return
	}
	if err := risk.ValidateWinProbability(sig.WinProbability); err != nil {
		slog.Warn("trade signal invalid win_probability", append([]any{"err", err}, logFields...)...)
		return
	}
	if clobClient == nil {
		slog.Error("trade signal: clob client is nil", logFields...)
		return
	}
	if mm == nil {
		slog.Error("trade signal: money manager client is nil", logFields...)
		return
	}

	payload, err := mm.ProcessSignal(ctx, mmclient.ProcessSignalParams{
		TokenID:         sig.TokenID,
		Side:            order.Side(sig.Side),
		Price:           sig.Price,
		NegRisk:         sig.NegRisk,
		Expiration:      0,
		TimestampMs:     clobClient.OrderMessageTimestampMillis(),
		WinProbability:  sig.WinProbability,
	})
	if err != nil {
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			case codes.FailedPrecondition, codes.InvalidArgument:
				slog.Info("trade signal rejected", append([]any{"reason", st.Message()}, logFields...)...)
				return
			}
		}
		slog.Error("trade signal process failed", append([]any{"err", err}, logFields...)...)
		return
	}

	orderPayload := clob.OrderPayloadFromMoneyManager(payload)
	if orderPayload == nil {
		slog.Error("trade signal: empty order payload", logFields...)
		return
	}

	resp, err := clobClient.PlaceOrder(orderPayload, "", models.OrderTypeGTC)
	if err != nil {
		slog.Error("place order failed", append([]any{"err", err}, logFields...)...)
		return
	}

	slog.Info("order placed",
		append([]any{
			"order_id", resp.OrderID,
			"status", resp.Status,
		}, logFields...)...,
	)
}

// runSignalLogger drains signals until shutdown (logging only; used in tests).
func runSignalLogger(ctx context.Context, signalCh <-chan core.TradeSignal) {
	runSignalLoggerWithHandler(ctx, signalCh, func(sig core.TradeSignal) {
		slog.Info("signal received",
			append([]any{"side", sig.Side, "price", sig.Price},
				core.AppendVerboseIDs("token_id", sig.TokenID)...)...)
	})
}

func runSignalLoggerWithHandler(ctx context.Context, signalCh <-chan core.TradeSignal, onSignal func(core.TradeSignal)) {
	if onSignal == nil {
		onSignal = noopSignalHandler
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case sig, ok := <-signalCh:
				if !ok {
					return
				}
				onSignal(sig)
			}
		}
	}()
}

func waitInterrupt() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
}

func shutdown(
	registry *traderRegistry,
	cancel context.CancelFunc,
	feedManager feedManagerRunner,
	sportsFeed *core.SportsFeed,
) {
	if registry != nil {
		registry.StopAll()
	}
	cancel()
	if registry != nil {
		registry.WaitForForwarders()
	}
	if feedManager != nil {
		feedManager.Stop()
	}
	if sportsFeed != nil {
		sportsFeed.Stop()
	}
	slog.Info("shutting down")
}

func main() {
	_ = godotenv.Load()
	parseVerboseFlag()
	setupLogging()
	secrets.MustLoadFromEnvIfConfigured(context.Background(), slog.Default())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	feedManager, marketFeed, err := startATPStack(ctx)
	if err != nil {
		slog.Error("failed to get ATP market feed", "err", err)
		os.Exit(1)
	}
	sportsFeed := newSportsFeed()
	sportsFeed.Start(ctx)

	gammaClient, clobClient := newClients()

	markets, err := fetchATPMarkets(ctx, gammaClient)
	if err != nil {
		feedManager.Stop()
		sportsFeed.Stop()
		slog.Error("failed to fetch ATP markets", "err", err)
		os.Exit(1)
	}
	filtered := core.FilterATPMarkets(markets)
	slog.Info("ATP markets after filter", "filtered", len(filtered), "total", len(markets))

	atpTraders, startedConditionIDs := startATPTraders(ctx, gammaClient, clobClient, marketFeed, sportsFeed, filtered)

	mmClient, err := dialMoneyManager(ctx)
	if err != nil {
		feedManager.Stop()
		sportsFeed.Stop()
		slog.Error("failed to connect to money manager", "err", err, "addr", mmclient.AddrFromEnv())
		os.Exit(1)
	}
	defer func() { _ = mmClient.Close() }()

	registry := newTraderRegistry(ctx, 100)
	for _, tr := range atpTraders {
		registry.Register(tr)
	}
	runSignalExecutor(ctx, registry.Signals(), clobClient, mmClient)

	discovery := core.NewATPMarketDiscovery(gammaClient, func(ctx context.Context, market models.GammaMarket) error {
		tr := newATPTrader(gammaClient, clobClient, marketFeed, sportsFeed, market)
		if err := tr.Start(ctx); err != nil {
			return err
		}
		registry.Register(tr)
		return nil
	}, startedConditionIDs)
	marketFeed.OnNewMarket(func(ev models.NewMarketEvent) {
		discovery.HandleNewMarket(ctx, ev)
	})

	waitInterrupt()
	shutdown(registry, cancel, feedManager, sportsFeed)
}
