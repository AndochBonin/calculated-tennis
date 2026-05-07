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

	"github.com/AndochBonin/polymarket/clob"
	"github.com/AndochBonin/polymarket/core"
	"github.com/AndochBonin/polymarket/gamma"
	"github.com/AndochBonin/polymarket/models"
	"github.com/joho/godotenv"
)

type gammaMarketsFetcher interface {
	GetMarkets(context.Context, gamma.MarketsParams) ([]models.GammaMarket, error)
}

type traderRunner interface {
	Start(context.Context) error
	Stop()
	Signals() <-chan core.TradeSignal
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
	newClobClient  = clob.NewClient
	newSportsFeed  = core.NewSportsFeed
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
) []traderRunner {
	var atpTraders []traderRunner
	for _, m := range markets {
		tr := newATPTrader(gammaClient, clobClient, marketFeed, sportsFeed, m)
		if err := tr.Start(ctx); err != nil {
			slog.Warn("skip market",
				append([]any{"err", err}, core.AppendVerboseIDs("condition_id", m.ConditionID)...)...)
			continue
		}
		atpTraders = append(atpTraders, tr)
	}
	return atpTraders
}

func forwardAllTraderSignals(ctx context.Context, traders []traderRunner) (<-chan core.TradeSignal, *sync.WaitGroup) {
	signalCh := make(chan core.TradeSignal, 100)
	var forwardWg sync.WaitGroup
	for _, tr := range traders {
		forwardWg.Go(func() {
			for {
				select {
				case <-ctx.Done():
					return
				case sig, ok := <-tr.Signals():
					if !ok {
						return
					}
					select {
					case signalCh <- sig:
					case <-ctx.Done():
						return
					}
				}
			}
		})
	}
	return signalCh, &forwardWg
}

// runSignalLogger drains signals until shutdown: when ctx is cancelled (root cancel runs before
// explicit Stop) or when the signals channel is closed. Money Manager will handle this later.
func runSignalLogger(ctx context.Context, signalCh <-chan core.TradeSignal) {
	runSignalLoggerWithHandler(ctx, signalCh, func(sig core.TradeSignal) {
		slog.Info("signal received",
			append([]any{"side", sig.Side}, core.AppendVerboseIDs("token_id", sig.TokenID)...)...)
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
	traders []traderRunner,
	forwardWg *sync.WaitGroup,
	cancel context.CancelFunc,
	feedManager feedManagerRunner,
	sportsFeed *core.SportsFeed,
) {
	for _, tr := range traders {
		tr.Stop()
	}
	forwardWg.Wait()
	cancel()
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

	atpTraders := startATPTraders(ctx, gammaClient, clobClient, marketFeed, sportsFeed, filtered)

	signalCh, forwardWg := forwardAllTraderSignals(ctx, atpTraders)
	runSignalLogger(ctx, signalCh)

	waitInterrupt()
	shutdown(atpTraders, forwardWg, cancel, feedManager, sportsFeed)
}
