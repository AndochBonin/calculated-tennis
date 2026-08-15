// Interactive TUI front-end for the Monte Carlo tennis match projection.
//
// Run (from the tennis module):
//
//	go run ./cmd/tui
//	make tui
//
// It collects the same inputs as cmd/simmatch (players, format, surface, alpha,
// sims, optional score / first server), fetches Tennis Abstract hold/break rates,
// runs the projection, and renders the result. The chosen court surface sets the
// color theme: hard (blue), clay (terracotta), or grass (green).
//
// Optional env (same as cmd/simmatch): REDIS_ADDR or REDIS_URL enables the Redis
// cache; TENNISABSTRACT_CACHE_TTL sets its TTL. Surface-specific form tuning uses
// TENNISABSTRACT_FORM_*_{HARD|CLAY|GRASS} from .env when set.
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/AndochBonin/calculated-tennis/tennis/tennisabstract"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/joho/godotenv"
)

func main() {
	os.Exit(run())
}

func run() int {
	_ = godotenv.Load()

	opts := tennisabstract.HTTPClientOptionsFromEnv()
	if cacheConfigured() {
		cache, err := tennisabstract.NewRedisCacheFromEnv()
		if err != nil {
			fmt.Fprintf(os.Stderr, "redis cache: %v\n", err)
			return 1
		}
		opts = append(opts,
			tennisabstract.WithCache(cache),
			tennisabstract.WithCacheTTL(tennisabstract.CacheTTLFromEnv()),
		)
	}

	client := tennisabstract.NewClient(opts...)
	p := tea.NewProgram(initialModel(context.Background(), client), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "tui: %v\n", err)
		return 1
	}
	return 0
}

func cacheConfigured() bool {
	return strings.TrimSpace(os.Getenv("REDIS_URL")) != "" ||
		strings.TrimSpace(os.Getenv("REDIS_ADDR")) != ""
}
