// Monte Carlo tennis match win projection from Tennis Abstract hold/break rates.
//
// Run (from repo root):
//
//	go run ./cmd/simmatch
//	go run ./cmd/simmatch -player-a="Daniil Medvedev" -player-b="Jannik Sinner" -format=atp -alpha=2.5 -sims=10000
//	go run ./cmd/simmatch ... -score="7-5 4-6 2-3" -first-server=1
//
// Or: make sim-match
// Or: make sim-match PLAYER_A="..." PLAYER_B="..." FORMAT=atp ALPHA=2.5 SIMS=10000
//
// Optional env: REDIS_ADDR or REDIS_URL (enables Redis cache when set; otherwise no cache),
// TENNISABSTRACT_CACHE_TTL (e.g. "6h", default 6h when cache is enabled).
package main

import (
	"bufio"
	"context"
	cryptorand "crypto/rand"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"math"
	"math/rand/v2"
	"os"
	"strconv"
	"strings"

	"github.com/AndochBonin/polymarket/internal/prompt"
	"github.com/AndochBonin/polymarket/tennis"
	"github.com/AndochBonin/polymarket/tennisabstract"
	"github.com/joho/godotenv"
)

var (
	errUsage          = errors.New("usage")
	errInvalidFormat  = errors.New("invalid match format")
	errInvalidAlpha   = errors.New("alpha must be positive")
	errInvalidSims           = errors.New("number of simulations must be positive")
	errFirstServerRequired   = errors.New("first server is required when score is provided")
	errInvalidFirstServer    = errors.New("invalid first server")
)

func main() {
	os.Exit(exitRun())
}

func exitRun() int {
	_ = godotenv.Load()

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	playerAFlag := flag.String("player-a", "", "player A display name or slug")
	playerBFlag := flag.String("player-b", "", "player B display name or slug")
	formatFlag := flag.String("format", "", "match format: atp, gs-men, gs-women")
	alphaFlag := flag.String("alpha", "", "sensitivity parameter (must be > 0)")
	simsFlag := flag.String("sims", "", "number of Monte Carlo simulations")
	scoreFlag := flag.String("score", "", "optional match score (A-B per set, space-separated)")
	firstServerFlag := flag.String("first-server", "", "first server: 1 or a = player A, 2 or b = player B (required with -score)")
	flag.Parse()

	in, err := resolveInputs(os.Stdin, *playerAFlag, *playerBFlag, *formatFlag, *alphaFlag, *simsFlag,
		*scoreFlag, *firstServerFlag, prompt.IsInteractive)
	if err != nil {
		if errors.Is(err, errUsage) {
			log.Error("usage", "msg", "player-a, player-b, format, alpha, and sims are required")
			fmt.Fprintf(os.Stderr, "usage: %s -player-a=<name> -player-b=<name> -format=<atp|gs-men|gs-women> -alpha=<float> -sims=<n>\n", os.Args[0])
			return 2
		}
		log.Error("input", "err", err)
		return 1
	}

	opts := tennisabstract.HTTPClientOptionsFromEnv()
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

	var rates [2]tennis.PlayerRates
	names := [2]string{in.playerA, in.playerB}
	for i, name := range names {
		log.Info("fetching player stats", "player", name)
		stats, err := client.GetPlayerStats(ctx, name)
		if err != nil {
			log.Error("get player stats", "player", name, "err", err)
			return 1
		}
		adj, err := tennisabstract.AdjustedHoldBreak(stats, tennisabstract.FormOptions{})
		if err != nil {
			log.Error("adjusted hold/break", "player", name, "err", err)
			return 1
		}
		rates[i] = tennis.PlayerRates{HoldPct: adj.HoldPct, BreakPct: adj.BreakPct}
	}

	rng, seed1, seed2, err := newSeededRNG()
	if err != nil {
		log.Error("rng seed", "err", err)
		return 1
	}
	log.Info("rng seed", "seed1", seed1, "seed2", seed2)

	log.Info("simulating", "sims", in.sims)
	var result tennis.SimulationResult
	if in.score != "" {
		initial, err := tennis.MatchFromScore(in.format, in.firstServer, in.score)
		if err != nil {
			log.Error("match from score", "err", err)
			return 1
		}
		result, err = tennis.Simulate(initial, rates, in.alpha, in.sims, rng)
	} else if in.firstServerChosen {
		initial := tennis.NewMatch(in.firstServer, in.format)
		result, err = tennis.Simulate(initial, rates, in.alpha, in.sims, rng)
	} else {
		result, err = tennis.SimulateFresh(in.format, rates, in.alpha, in.sims, rng)
	}
	if err != nil {
		log.Error("simulate", "err", err)
		return 1
	}

	printSummary(os.Stdout, names, rates, in, result)
	return 0
}

type simInputs struct {
	playerA           string
	playerB           string
	format            tennis.MatchFormat
	formatLabel       string
	alpha             float64
	sims              int
	score             string
	firstServer       tennis.Player
	firstServerChosen bool
	useCoinToss       bool
}

func resolveInputs(stdin *os.File, playerAFlag, playerBFlag, formatFlag, alphaFlag, simsFlag, scoreFlag, firstServerFlag string, interactive func(*os.File) bool) (simInputs, error) {
	var br *bufio.Reader
	if interactive(stdin) {
		br = bufio.NewReader(stdin)
	}

	playerA, err := resolvePlayerName(playerAFlag, "Player A name: ", br)
	if err != nil {
		return simInputs{}, err
	}
	playerB, err := resolvePlayerName(playerBFlag, "Player B name: ", br)
	if err != nil {
		return simInputs{}, err
	}
	format, formatLabel, err := resolveFormat(formatFlag, br)
	if err != nil {
		return simInputs{}, err
	}
	alpha, err := resolveAlpha(alphaFlag, br)
	if err != nil {
		return simInputs{}, err
	}
	sims, err := resolveSims(simsFlag, br)
	if err != nil {
		return simInputs{}, err
	}
	score, err := resolveScore(scoreFlag, br)
	if err != nil {
		return simInputs{}, err
	}
	firstServer, firstServerChosen, useCoinToss, err := resolveFirstServer(firstServerFlag, score, playerA, playerB, br)
	if err != nil {
		return simInputs{}, err
	}
	return simInputs{
		playerA:           playerA,
		playerB:           playerB,
		format:            format,
		formatLabel:       formatLabel,
		alpha:             alpha,
		sims:              sims,
		score:             score,
		firstServer:       firstServer,
		firstServerChosen: firstServerChosen,
		useCoinToss:       useCoinToss,
	}, nil
}

func resolvePlayerName(flagVal, label string, br *bufio.Reader) (string, error) {
	name := strings.TrimSpace(flagVal)
	if name == "" && br != nil {
		var err error
		name, err = prompt.ReadLineFrom(os.Stderr, br, label)
		if err != nil {
			return "", err
		}
	}
	if name == "" {
		return "", errUsage
	}
	return name, nil
}

func resolveFormat(formatFlag string, br *bufio.Reader) (tennis.MatchFormat, string, error) {
	raw := strings.TrimSpace(formatFlag)
	if raw == "" && br != nil {
		var err error
		raw, err = prompt.ReadLineFrom(os.Stderr, br, formatMenuLabel())
		if err != nil {
			return tennis.MatchFormat{}, "", err
		}
	}
	if raw == "" {
		return tennis.MatchFormat{}, "", errUsage
	}
	return parseFormatChoice(raw)
}

func formatMenuLabel() string {
	return "Match format:\n" +
		"  1) ATP best-of-3\n" +
		"  2) Grand Slam men best-of-5\n" +
		"  3) Grand Slam women best-of-3\n" +
		"Choice (1-3 or atp/gs-men/gs-women): "
}

func parseFormatChoice(raw string) (tennis.MatchFormat, string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "atp":
		return tennis.DefaultFormat(), "ATP best-of-3", nil
	case "2", "gs-men", "gs_men":
		return tennis.GrandSlamMenFormat(), "Grand Slam men best-of-5", nil
	case "3", "gs-women", "gs_women":
		return tennis.GrandSlamWomenFormat(), "Grand Slam women best-of-3", nil
	default:
		return tennis.MatchFormat{}, "", fmt.Errorf("%w: %q", errInvalidFormat, raw)
	}
}

func resolveAlpha(alphaFlag string, br *bufio.Reader) (float64, error) {
	raw := strings.TrimSpace(alphaFlag)
	if raw == "" && br != nil {
		var err error
		raw, err = prompt.ReadLineFrom(os.Stderr, br, "Alpha (> 0): ")
		if err != nil {
			return 0, err
		}
	}
	if raw == "" {
		return 0, errUsage
	}
	alpha, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", errInvalidAlpha, err)
	}
	if alpha <= 0 || math.IsNaN(alpha) || math.IsInf(alpha, 0) {
		return 0, errInvalidAlpha
	}
	return alpha, nil
}

func resolveSims(simsFlag string, br *bufio.Reader) (int, error) {
	raw := strings.TrimSpace(simsFlag)
	if raw == "" && br != nil {
		var err error
		raw, err = prompt.ReadLineFrom(os.Stderr, br, "Number of simulations: ")
		if err != nil {
			return 0, err
		}
	}
	if raw == "" {
		return 0, errUsage
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", errInvalidSims, err)
	}
	if n <= 0 {
		return 0, errInvalidSims
	}
	return n, nil
}

func resolveScore(scoreFlag string, br *bufio.Reader) (string, error) {
	raw := strings.TrimSpace(scoreFlag)
	if raw == "" && br != nil {
		var err error
		raw, err = prompt.ReadLineFrom(os.Stderr, br, "Match score (A-B per set, space-separated, Enter to skip): ")
		if err != nil {
			return "", err
		}
	}
	return raw, nil
}

func firstServerMenuLabel(playerA, playerB string) string {
	return "First server:\n" +
		"  1) " + playerA + "\n" +
		"  2) " + playerB + "\n" +
		"  Enter for coin toss each simulation\n" +
		"Choice (1-2 or a/b, Enter to skip): "
}

func resolveFirstServer(firstServerFlag, score, playerA, playerB string, br *bufio.Reader) (tennis.Player, bool, bool, error) {
	raw := strings.TrimSpace(firstServerFlag)
	if raw == "" && br != nil {
		var err error
		raw, err = prompt.ReadLineFrom(os.Stderr, br, firstServerMenuLabel(playerA, playerB))
		if err != nil {
			return 0, false, false, err
		}
	}
	if raw == "" {
		if score != "" {
			return 0, false, false, errFirstServerRequired
		}
		return 0, false, true, nil
	}
	p, err := parseFirstServerChoice(raw)
	if err != nil {
		return 0, false, false, err
	}
	return p, true, false, nil
}

func parseFirstServerChoice(raw string) (tennis.Player, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "a":
		return tennis.A, nil
	case "2", "b":
		return tennis.B, nil
	default:
		return 0, fmt.Errorf("%w: %q", errInvalidFirstServer, raw)
	}
}

func newSeededRNG() (*rand.Rand, uint64, uint64, error) {
	var buf [16]byte
	if _, err := cryptorand.Read(buf[:]); err != nil {
		return nil, 0, 0, err
	}
	seed1 := binary.LittleEndian.Uint64(buf[0:8])
	seed2 := binary.LittleEndian.Uint64(buf[8:16])
	return rand.New(rand.NewPCG(seed1, seed2)), seed1, seed2, nil
}

func printSummary(w io.Writer, names [2]string, rates [2]tennis.PlayerRates, in simInputs, result tennis.SimulationResult) {
	fmt.Fprintln(w, "Match simulation")
	fmt.Fprintf(w, "  Player A: %s  (hold %s, break %s)\n", names[0], pct(rates[0].HoldPct), pct(rates[0].BreakPct))
	fmt.Fprintf(w, "  Player B: %s  (hold %s, break %s)\n", names[1], pct(rates[1].HoldPct), pct(rates[1].BreakPct))
	fmt.Fprintf(w, "  Format:   %s\n", in.formatLabel)
	if in.score != "" {
		fmt.Fprintf(w, "  Score:    %s\n", in.score)
	}
	if in.useCoinToss {
		fmt.Fprintln(w, "  First server: coin toss per simulation")
	} else if in.firstServerChosen {
		fmt.Fprintf(w, "  First server: %s\n", names[int(in.firstServer)])
	}
	fmt.Fprintf(w, "  Alpha:    %g\n", in.alpha)
	fmt.Fprintf(w, "  Sims:     %d\n", in.sims)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Results")
	for i, p := range []tennis.Player{tennis.A, tennis.B} {
		wins := result.WinCount(p)
		fmt.Fprintf(w, "  %s:  %d wins (%.1f%%)\n", names[i], wins, 100*float64(wins)/float64(in.sims))
	}
}

func pct(v float64) string {
	return fmt.Sprintf("%.1f%%", 100*v)
}

func cacheConfigured() bool {
	return strings.TrimSpace(os.Getenv("REDIS_URL")) != "" ||
		strings.TrimSpace(os.Getenv("REDIS_ADDR")) != ""
}
