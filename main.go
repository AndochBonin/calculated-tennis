package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/AndochBonin/polymarket/clob"
	"github.com/AndochBonin/polymarket/core"
	"github.com/AndochBonin/polymarket/gamma"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// feed manager
	feedManager := core.NewFeedManager([]core.Category{core.CategoryATP})
	feedManager.Start(ctx)

	// gamma client
	gammaClient := gamma.NewClient()

	// clob client
	clobClient := clob.NewClient()

	// atp trader
	atpFeed, err := feedManager.Feed(core.CategoryATP)
	if err != nil {
		feedManager.Stop()
		log.Fatalf("failed to get ATP feed: %v", err)
	}

	atpTrader := core.NewATPTrader(gammaClient, clobClient, atpFeed)
	if err := atpTrader.Start(ctx); err != nil {
		feedManager.Stop()
		log.Fatalf("failed to start ATP trader: %v", err)
	}

	// Drain signals until shutdown. Exits when ctx is cancelled (root cancel runs before
	// explicit Stop) or when the signals channel is closed. Money Manager will handle this later.
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case sig, ok := <-atpTrader.Signals():
				if !ok {
					return
				}
				log.Printf("[main] signal received | token=%s side=%s", sig.TokenID, sig.Side)
			}
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit

	cancel()
	atpTrader.Stop()
	feedManager.Stop()

	log.Println("shutting down")
}
