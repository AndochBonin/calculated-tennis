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

func startATPStack(ctx context.Context) (*core.FeedManager, *core.CategoryFeed, error) {
	feedManager := core.NewFeedManager([]core.Category{core.CategoryATP})
	feedManager.Start(ctx)
	atpFeed, err := feedManager.Feed(core.CategoryATP)
	if err != nil {
		feedManager.Stop()
		return nil, nil, err
	}
	return feedManager, atpFeed, nil
}

func newClients() (*gamma.Client, *clob.Client) {
	return gamma.NewClient(), clob.NewClient()
}

func fetchATPMarkets(ctx context.Context, gammaClient *gamma.Client) ([]models.GammaMarket, error) {
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
	atpFeed *core.CategoryFeed,
	markets []models.GammaMarket,
) []*core.ATPTrader {
	var atpTraders []*core.ATPTrader
	for _, m := range markets {
		tr := core.NewATPTrader(gammaClient, clobClient, atpFeed, m)
		if err := tr.Start(ctx); err != nil {
			slog.Warn("skip market",
				append([]any{"err", err}, core.AppendVerboseIDs("condition_id", m.ConditionID)...)...)
			continue
		}
		atpTraders = append(atpTraders, tr)
	}
	return atpTraders
}

func forwardAllTraderSignals(ctx context.Context, traders []*core.ATPTrader) (<-chan core.TradeSignal, *sync.WaitGroup) {
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
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case sig, ok := <-signalCh:
				if !ok {
					return
				}
				slog.Info("signal received",
					append([]any{"side", sig.Side}, core.AppendVerboseIDs("token_id", sig.TokenID)...)...)
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
	traders []*core.ATPTrader,
	forwardWg *sync.WaitGroup,
	cancel context.CancelFunc,
	feedManager *core.FeedManager,
) {
	for _, tr := range traders {
		tr.Stop()
	}
	forwardWg.Wait()
	cancel()
	feedManager.Stop()
	slog.Info("shutting down")
}

func main() {
	_ = godotenv.Load()
	parseVerboseFlag()
	setupLogging()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	feedManager, atpFeed, err := startATPStack(ctx)
	if err != nil {
		slog.Error("failed to get ATP feed", "err", err)
		os.Exit(1)
	}

	gammaClient, clobClient := newClients()

	markets, err := fetchATPMarkets(ctx, gammaClient)
	if err != nil {
		feedManager.Stop()
		slog.Error("failed to fetch ATP markets", "err", err)
		os.Exit(1)
	}
	filtered := core.FilterATPMarkets(markets)
	slog.Info("ATP markets after filter", "filtered", len(filtered), "total", len(markets))

	atpTraders := startATPTraders(ctx, gammaClient, clobClient, atpFeed, filtered)

	signalCh, forwardWg := forwardAllTraderSignals(ctx, atpTraders)
	runSignalLogger(ctx, signalCh)

	waitInterrupt()
	shutdown(atpTraders, forwardWg, cancel, feedManager)
}
