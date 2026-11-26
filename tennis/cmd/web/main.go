// Web front end for the Monte Carlo tennis match projection.
//
// Run (from the tennis module, after `cd web && npm run build`):
//
//	go run ./cmd/web            # http://localhost:8080
//	make web                    # builds the SPA, then runs this
//
// For UI development run this alongside `npm run dev` in web/ (Vite proxies /api here).
//
// Optional env: REDIS_ADDR or REDIS_URL enables the Redis rate cache;
// TENNISABSTRACT_CACHE_TTL sets its TTL. Alpha resolves per surface from
// TENNISABSTRACT_ALPHA_*; form tuning from TENNISABSTRACT_FORM_*_{HARD|CLAY|GRASS}.
package main

import (
	"context"
	cryptorand "crypto/rand"
	_ "embed"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"math/rand/v2"
	"net/http"
	"os"
	"strings"

	"github.com/AndochBonin/calculated-tennis/tennis/tennis"
	"github.com/AndochBonin/calculated-tennis/tennis/tennisabstract"
	"github.com/AndochBonin/calculated-tennis/tennis/web"
	"github.com/joho/godotenv"
)

const maxSims = 100_000

// playersRaw is the supported player display names (2025 ATP tour), generated
// from tennisabstract/testdata/atp_matches_2025.csv via UniquePlayerNamesFromMatchesCSV.
//
//go:embed players.txt
var playersRaw string

type format struct {
	ID     string             `json:"id"`
	Label  string             `json:"label"`
	format tennis.MatchFormat // unexported: not sent to the client
}

var formats = []format{
	{"atp", "Best of 3", tennis.DefaultFormat()},
	{"gs-men", "Best of 5", tennis.GrandSlamMenFormat()},
}

var surfaces = []tennisabstract.MatchSurface{
	tennisabstract.SurfaceHard, tennisabstract.SurfaceClay, tennisabstract.SurfaceGrass,
}

// fetchRates returns both players' form-adjusted hold/break rates for the
// surface. A variable so tests can stub out Tennis Abstract.
var fetchRates = func(ctx context.Context, client *tennisabstract.Client, names [2]string, surface tennisabstract.MatchSurface) ([2]tennis.PlayerRates, error) {
	formOpts := tennisabstract.FormOptionsFromEnv(surface)
	var rates [2]tennis.PlayerRates
	for i, name := range names {
		stats, err := client.GetPlayerStats(ctx, name)
		if err != nil {
			return rates, fmt.Errorf("%s: %w", name, err)
		}
		adj, err := tennisabstract.AdjustedHoldBreak(stats, formOpts)
		if err != nil {
			return rates, fmt.Errorf("%s: %w", name, err)
		}
		rates[i] = tennis.PlayerRates{HoldPct: adj.HoldPct, BreakPct: adj.BreakPct}
	}
	return rates, nil
}

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	flag.Parse()
	_ = godotenv.Load()

	opts := tennisabstract.HTTPClientOptionsFromEnv()
	if cacheConfigured() {
		cache, err := tennisabstract.NewRedisCacheFromEnv()
		if err != nil {
			log.Fatalf("redis cache: %v", err)
		}
		opts = append(opts,
			tennisabstract.WithCache(cache),
			tennisabstract.WithCacheTTL(tennisabstract.CacheTTLFromEnv()),
		)
	}

	log.Printf("listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, newMux(tennisabstract.NewClient(opts...))))
}

func newMux(client *tennisabstract.Client) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/config", handleConfig)
	mux.HandleFunc("POST /api/simulate", func(w http.ResponseWriter, r *http.Request) { handleSimulate(w, r, client) })
	dist, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		panic(err)
	}
	mux.Handle("/", http.FileServerFS(dist))
	return mux
}

func handleConfig(w http.ResponseWriter, _ *http.Request) {
	type surfaceCfg struct {
		ID    string  `json:"id"`
		Alpha float64 `json:"alpha"`
	}
	sc := make([]surfaceCfg, len(surfaces))
	for i, s := range surfaces {
		sc[i] = surfaceCfg{strings.ToLower(string(s)), tennisabstract.AlphaFromEnv(s)}
	}
	var players []string
	for _, l := range strings.Split(playersRaw, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			players = append(players, l)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"players": players, "surfaces": sc, "formats": formats})
}

type simRequest struct {
	PlayerA string `json:"playerA"`
	PlayerB string `json:"playerB"`
	Surface string `json:"surface"`
	Format  string `json:"format"`
	Sims    int    `json:"sims"`
}

type rates struct {
	Hold float64 `json:"hold"`
	Brk  float64 `json:"brk"`
}

// simMatch is one simulated match: f = first server (0 = A, 1 = B), s = sets as
// [gamesA, gamesB] or [gamesA, gamesB, tbA, tbB] when the set went to a tiebreak.
type simMatch struct {
	F int     `json:"f"`
	S [][]int `json:"s"`
}

type simResponse struct {
	PlayerA string     `json:"playerA"`
	PlayerB string     `json:"playerB"`
	Surface string     `json:"surface"`
	Format  string     `json:"format"`
	Alpha   float64    `json:"alpha"`
	Sims    int        `json:"sims"`
	WinsA   int        `json:"winsA"`
	Rates   [2]rates   `json:"rates"`
	Matches []simMatch `json:"matches"`
}

func handleSimulate(w http.ResponseWriter, r *http.Request, client *tennisabstract.Client) {
	var req simRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.PlayerA, req.PlayerB = strings.TrimSpace(req.PlayerA), strings.TrimSpace(req.PlayerB)
	if req.PlayerA == "" || req.PlayerB == "" {
		writeErr(w, http.StatusBadRequest, "both player names are required")
		return
	}
	if req.Sims < 1 || req.Sims > maxSims {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("sims must be between 1 and %d", maxSims))
		return
	}
	surface, err := tennisabstract.ParseMatchSurface(req.Surface)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	var f *format
	for i := range formats {
		if formats[i].ID == req.Format {
			f = &formats[i]
		}
	}
	if f == nil {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("unknown format %q", req.Format))
		return
	}

	pr, err := fetchRates(r.Context(), client, [2]string{req.PlayerA, req.PlayerB}, surface)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	alpha := tennisabstract.AlphaFromEnv(surface)
	rng, err := newSeededRNG()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	matches, err := tennis.SimulateFreshMatches(f.format, pr, alpha, req.Sims, rng)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	// ponytail: full per-match payload (~4MB JSON at 100k best-of-5 sims, ungzipped); compact/gzip if served remotely
	resp := simResponse{
		PlayerA: req.PlayerA, PlayerB: req.PlayerB,
		Surface: strings.ToLower(string(surface)), Format: f.ID,
		Alpha: alpha, Sims: req.Sims,
		Rates:   [2]rates{{pr[0].HoldPct, pr[0].BreakPct}, {pr[1].HoldPct, pr[1].BreakPct}},
		Matches: make([]simMatch, len(matches)),
	}
	for i, m := range matches {
		if *m.Winner == tennis.A {
			resp.WinsA++
		}
		sets := make([][]int, len(m.CompletedSets))
		for j, s := range m.CompletedSets {
			sets[j] = []int{s.GamesA, s.GamesB}
			if tb := m.SetTiebreaks[j]; tb != [2]int{} {
				sets[j] = append(sets[j], tb[0], tb[1])
			}
		}
		resp.Matches[i] = simMatch{F: int(m.FirstServer), S: sets}
	}
	writeJSON(w, http.StatusOK, resp)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write response: %v", err)
	}
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// newSeededRNG returns a PCG RNG seeded from crypto/rand.
func newSeededRNG() (*rand.Rand, error) {
	var buf [16]byte
	if _, err := cryptorand.Read(buf[:]); err != nil {
		return nil, err
	}
	return rand.New(rand.NewPCG(binary.LittleEndian.Uint64(buf[0:8]), binary.LittleEndian.Uint64(buf[8:16]))), nil
}

func cacheConfigured() bool {
	return strings.TrimSpace(os.Getenv("REDIS_URL")) != "" ||
		strings.TrimSpace(os.Getenv("REDIS_ADDR")) != ""
}
