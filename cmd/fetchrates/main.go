// Build player_rates_2024.json from unique names in a matches CSV via Tennis Abstract.
//
// Run (from repo root):
//
//	go run ./cmd/fetchrates
//	go run ./cmd/fetchrates -matches=tennisabstract/testdata/atp_matches_2025.csv -out=tennisabstract/testdata/player_rates_2024.json
//
// Or: make fetch-rates
//
// Optional env: REDIS_ADDR or REDIS_URL (enables Redis cache when set; otherwise no cache),
// TENNISABSTRACT_CACHE_TTL (e.g. "6h", default 6h when cache is enabled).
//
// Use -merge to keep existing slugs in -out and only fetch missing players.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/AndochBonin/polymarket/tennisabstract"
	"github.com/joho/godotenv"
)

func main() {
	os.Exit(exitRun())
}

func exitRun() int {
	_ = godotenv.Load()

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	matchesFlag := flag.String("matches", "tennisabstract/testdata/atp_matches_2025.csv", "ATP matches CSV path")
	outFlag := flag.String("out", "tennisabstract/testdata/player_rates_2024.json", "output JSON path (slug → hold_2024/break_2024)")
	yearFlag := flag.Int("year", 2024, "calendar year for SeasonHoldBreak")
	mergeFlag := flag.Bool("merge", false, "keep existing entries in -out and only fetch missing slugs")
	flag.Parse()

	if *yearFlag <= 0 {
		log.Error("invalid year", "year", *yearFlag)
		return 2
	}

	names, err := readUniqueNames(*matchesFlag)
	if err != nil {
		log.Error("read matches csv", "path", *matchesFlag, "err", err)
		return 1
	}
	log.Info("players in csv", "unique_names", len(names))

	rates := tennisabstract.PlayerRatesMap{}
	if *mergeFlag {
		existing, err := tennisabstract.ReadPlayerRatesFile(*outFlag)
		if err != nil {
			if !os.IsNotExist(err) {
				log.Error("read existing rates", "path", *outFlag, "err", err)
				return 1
			}
		} else {
			rates = existing
			log.Info("merged existing rates", "slugs", len(rates))
		}
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
	ctx := context.Background()

	var fetchErrs int
	var skipped int
	for i, name := range names {
		slug := tennisabstract.PlayerSlug(name)
		if slug == "" {
			log.Warn("skip empty slug", "name", name)
			skipped++
			continue
		}
		if _, ok := rates[slug]; ok {
			continue
		}

		log.Info("fetching player stats", "progress", fmt.Sprintf("%d/%d", i+1, len(names)), "name", name, "slug", slug)
		stats, err := client.GetPlayerStats(ctx, name)
		if err != nil {
			log.Error("get player stats", "name", name, "slug", slug, "err", err)
			fetchErrs++
			continue
		}
		hold, brk, err := tennisabstract.SeasonHoldBreak(stats, *yearFlag)
		if err != nil {
			log.Error("season hold/break", "name", name, "slug", slug, "year", *yearFlag, "err", err)
			fetchErrs++
			continue
		}
		rates[slug] = tennisabstract.PlayerRates2024{
			Hold2024:  hold,
			Break2024: brk,
		}
	}

	if err := tennisabstract.WritePlayerRatesFile(*outFlag, rates); err != nil {
		log.Error("write rates", "path", *outFlag, "err", err)
		return 1
	}

	log.Info("done",
		"out", *outFlag,
		"slugs_written", len(rates),
		"fetch_errors", fetchErrs,
		"skipped_names", skipped,
	)
	if fetchErrs > 0 {
		return 1
	}
	return 0
}

func readUniqueNames(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return tennisabstract.UniquePlayerNamesFromMatchesCSV(f)
}

func cacheConfigured() bool {
	return strings.TrimSpace(os.Getenv("REDIS_URL")) != "" ||
		strings.TrimSpace(os.Getenv("REDIS_ADDR")) != ""
}
