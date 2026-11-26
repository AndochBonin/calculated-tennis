// Browser (wasm) engine for the Monte Carlo tennis match projection. main.go
// exposes config, rates and simulate to the web worker (web/public/worker.js);
// each returns a JSON string, or {"error": msg}.
//
// Build: make -C tennis wasm (writes web/public/tennis.wasm).
package main

import (
	cryptorand "crypto/rand"
	_ "embed"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/AndochBonin/calculated-tennis/tennis/tennis"
	"github.com/AndochBonin/calculated-tennis/tennis/tennisabstract"
)

const maxSims = 100_000

// playersRaw is the supported player display names (2025 ATP tour).
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

// toJSON encodes v, or {"error": err} when err is set.
func toJSON(v any, err error) string {
	if err != nil {
		v = map[string]string{"error": err.Error()}
	}
	b, err := json.Marshal(v)
	if err != nil {
		b, _ = json.Marshal(map[string]string{"error": err.Error()})
	}
	return string(b)
}

func config() any {
	type surfaceCfg struct {
		ID    string  `json:"id"`
		Alpha float64 `json:"alpha"`
	}
	sc := make([]surfaceCfg, len(surfaces))
	for i, s := range surfaces {
		sc[i] = surfaceCfg{strings.ToLower(string(s)), tennisabstract.Alpha(s)}
	}
	var players []string
	for _, l := range strings.Split(playersRaw, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			players = append(players, l)
		}
	}
	return map[string]any{"players": players, "surfaces": sc, "formats": formats}
}

type rates struct {
	Hold float64 `json:"hold"`
	Brk  float64 `json:"brk"`
}

func noData(name string) error { return errors.New("No data found for " + name) }

// ratesFromHTML turns a Tennis Abstract player fragment (jsfrags/{slug}.js's
// player_frag) into form-adjusted hold/break as of asOf. Form tuning is the
// same on every surface; the surface only changes alpha in simulate.
func ratesFromHTML(name, html string, asOf time.Time) (rates, error) {
	stats, err := tennisabstract.ParsePlayerHTML(strings.NewReader(html), tennisabstract.PlayerSlug(name))
	if err != nil {
		return rates{}, fmt.Errorf("%s: %w", name, err)
	}
	adj, err := tennisabstract.AdjustedHoldBreak(stats, tennisabstract.FormOptions{AsOf: asOf})
	if errors.Is(err, tennisabstract.ErrNoSeasonData) {
		return rates{}, noData(name)
	}
	if err != nil {
		return rates{}, fmt.Errorf("%s: %w", name, err)
	}
	return rates{adj.HoldPct, adj.BreakPct}, nil
}

type simRequest struct {
	PlayerA string   `json:"playerA"`
	PlayerB string   `json:"playerB"`
	Surface string   `json:"surface"`
	Format  string   `json:"format"`
	Sims    int      `json:"sims"`
	Rates   [2]rates `json:"rates"`
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

func simulate(body string) (simResponse, error) {
	var req simRequest
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		return simResponse{}, errors.New("invalid request body")
	}
	req.PlayerA, req.PlayerB = strings.TrimSpace(req.PlayerA), strings.TrimSpace(req.PlayerB)
	if req.PlayerA == "" || req.PlayerB == "" {
		return simResponse{}, errors.New("both player names are required")
	}
	if req.Sims < 1 || req.Sims > maxSims {
		return simResponse{}, fmt.Errorf("sims must be between 1 and %d", maxSims)
	}
	surface, err := tennisabstract.ParseMatchSurface(req.Surface)
	if err != nil {
		return simResponse{}, err
	}
	var f *format
	for i := range formats {
		if formats[i].ID == req.Format {
			f = &formats[i]
		}
	}
	if f == nil {
		return simResponse{}, fmt.Errorf("unknown format %q", req.Format)
	}

	alpha := tennisabstract.Alpha(surface)
	pr := [2]tennis.PlayerRates{{HoldPct: req.Rates[0].Hold, BreakPct: req.Rates[0].Brk}, {HoldPct: req.Rates[1].Hold, BreakPct: req.Rates[1].Brk}}
	// Bound each server's game-win chance: at 0 or 1 for both players,
	// tiebreaks never reach a 2-point margin and the simulation never ends.
	for i := range pr {
		p, err := tennis.GameWinProb(pr[i].HoldPct, pr[1-i].BreakPct, alpha)
		if err != nil || p < 0.01 || p > 0.99 {
			return simResponse{}, errors.New("rates out of range")
		}
	}
	rng, err := newSeededRNG()
	if err != nil {
		return simResponse{}, err
	}
	matches, err := tennis.SimulateFreshMatches(f.format, pr, alpha, req.Sims, rng)
	if err != nil {
		return simResponse{}, err
	}

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
	return resp, nil
}

// newSeededRNG returns a PCG RNG seeded from crypto/rand.
func newSeededRNG() (*rand.Rand, error) {
	var buf [16]byte
	if _, err := cryptorand.Read(buf[:]); err != nil {
		return nil, err
	}
	return rand.New(rand.NewPCG(binary.LittleEndian.Uint64(buf[0:8]), binary.LittleEndian.Uint64(buf[8:16]))), nil
}
