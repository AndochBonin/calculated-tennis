// Fetch Tennis Abstract player stats; print season baseline and form-adjusted hold/break JSON.
//
// Run (from repo root):
//
//	go run ./cmd/getstats
//	go run ./cmd/getstats -player="Daniil Medvedev"
//
// Or: make get-stats (prompts for player name on a TTY)
// Or: make get-stats PLAYER="jannik sinner"
//
// Optional env: REDIS_ADDR or REDIS_URL (enables Redis cache when set; otherwise no cache),
// TENNISABSTRACT_CACHE_TTL (e.g. "6h", default 6h when cache is enabled).
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/AndochBonin/polymarket/internal/prompt"
	"github.com/AndochBonin/polymarket/tennisabstract"
	"github.com/joho/godotenv"
)

var errUsage = errors.New("usage")

func main() {
	os.Exit(exitRun())
}

func exitRun() int {
	_ = godotenv.Load()

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	playerFlag := flag.String("player", "", "player display name or slug")
	flag.Parse()

	player, err := resolvePlayer(os.Stdin, *playerFlag, flag.Args(), prompt.IsInteractive)
	if err != nil {
		if errors.Is(err, errUsage) {
			log.Error("usage", "msg", "-player is required")
			fmt.Fprintf(os.Stderr, "usage: %s -player=<name>\n", os.Args[0])
			return 2
		}
		log.Error("input", "err", err)
		return 1
	}

	opts := []tennisabstract.Option{}
	if cacheConfigured() {
		cache, err := tennisabstract.NewRedisCacheFromEnv()
		if err != nil {
			log.Error("redis cache", "err", err)
			return 1
		}
		opts = append(opts,
			tennisabstract.WithCache(cache),
			tennisabstract.WithCacheTTL(tennisabstract.CacheTTLFromEnv()),
		)
		log.Info("cache enabled", "ttl", tennisabstract.CacheTTLFromEnv().String())
	}

	client := tennisabstract.NewClient(opts...)
	stats, err := client.GetPlayerStats(context.Background(), player)
	if err != nil {
		log.Error("get player stats", "err", err)
		return 1
	}

	rates, err := tennisabstract.AdjustedHoldBreak(stats, tennisabstract.FormOptions{})
	if err != nil {
		log.Error("adjusted hold/break", "err", err)
		return 1
	}

	out := statsOutput{
		PlayerSlug: stats.PlayerSlug,
		Base: holdBreakRates{
			HoldPct:  rates.SeasonHold,
			BreakPct: rates.SeasonBreak,
		},
		Adjusted: adjustedHoldBreak{
			HoldPct:    rates.HoldPct,
			BreakPct:   rates.BreakPct,
			FormWeight: rates.FormWeight,
		},
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		log.Error("encode json", "err", err)
		return 1
	}
	return 0
}

func resolvePlayer(stdin *os.File, playerFlag string, args []string, interactive func(*os.File) bool) (string, error) {
	player := strings.TrimSpace(playerFlag)
	if player == "" {
		player = strings.TrimSpace(strings.Join(args, " "))
	}
	if player == "" && interactive(stdin) {
		br := bufio.NewReader(stdin)
		var err error
		player, err = prompt.ReadLineFrom(os.Stderr, br, "Player name: ")
		if err != nil {
			return "", err
		}
	}
	if player == "" {
		return "", errUsage
	}
	return player, nil
}

type statsOutput struct {
	PlayerSlug string             `json:"player_slug"`
	Base       holdBreakRates     `json:"base"`
	Adjusted   adjustedHoldBreak  `json:"adjusted"`
}

type holdBreakRates struct {
	HoldPct  float64 `json:"hold_pct"`
	BreakPct float64 `json:"break_pct"`
}

type adjustedHoldBreak struct {
	HoldPct    float64 `json:"hold_pct"`
	BreakPct   float64 `json:"break_pct"`
	FormWeight float64 `json:"form_weight"`
}

func cacheConfigured() bool {
	return strings.TrimSpace(os.Getenv("REDIS_URL")) != "" ||
		strings.TrimSpace(os.Getenv("REDIS_ADDR")) != ""
}
