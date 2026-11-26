package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/AndochBonin/calculated-tennis/tennis/tennis"
	"github.com/AndochBonin/calculated-tennis/tennis/tennisabstract"
)

func post(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	newMux(nil).ServeHTTP(rec, httptest.NewRequest("POST", "/api/simulate", strings.NewReader(body)))
	return rec
}

func TestSimulate(t *testing.T) {
	orig := fetchRates
	t.Cleanup(func() { fetchRates = orig })
	fetchRates = func(context.Context, *tennisabstract.Client, [2]string, tennisabstract.MatchSurface) ([2]tennis.PlayerRates, error) {
		return [2]tennis.PlayerRates{{HoldPct: 0.85, BreakPct: 0.3}, {HoldPct: 0.75, BreakPct: 0.2}}, nil
	}

	rec := post(t, `{"playerA":"A","playerB":"B","surface":"clay","format":"gs-men","sims":200}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var resp simResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Matches) != 200 || resp.Surface != "clay" || resp.Rates[0].Hold != 0.85 {
		t.Fatalf("unexpected response: %d matches, surface %q, rates %+v", len(resp.Matches), resp.Surface, resp.Rates)
	}
	winsA := 0
	for i, m := range resp.Matches {
		sets := [2]int{}
		for _, s := range m.S {
			if len(s) != 2 && len(s) != 4 {
				t.Fatalf("match %d: bad set %v", i, s)
			}
			if s[0] > s[1] {
				sets[0]++
			} else {
				sets[1]++
			}
		}
		if max(sets[0], sets[1]) != 3 {
			t.Fatalf("match %d: sets %v in best of 5", i, sets)
		}
		if sets[0] == 3 {
			winsA++
		}
	}
	if winsA != resp.WinsA {
		t.Fatalf("winsA = %d, recomputed %d", resp.WinsA, winsA)
	}

	for _, body := range []string{
		`{"playerA":"A","playerB":"B","surface":"clay","format":"atp","sims":0}`,
		`{"playerA":"A","playerB":"B","surface":"clay","format":"atp","sims":100001}`,
		`{"playerA":"A","playerB":"B","surface":"carpet","format":"atp","sims":10}`,
		`{"playerA":"A","playerB":"B","surface":"hard","format":"bo7","sims":10}`,
		`{"playerA":" ","playerB":"B","surface":"hard","format":"atp","sims":10}`,
		`not json`,
	} {
		if rec := post(t, body); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", body, rec.Code)
		}
	}

	fetchRates = func(context.Context, *tennisabstract.Client, [2]string, tennisabstract.MatchSurface) ([2]tennis.PlayerRates, error) {
		return [2]tennis.PlayerRates{}, errors.New("Nobody: player not found")
	}
	rec = post(t, `{"playerA":"Nobody","playerB":"B","surface":"hard","format":"atp","sims":10}`)
	if rec.Code != http.StatusBadGateway || !strings.Contains(rec.Body.String(), "player not found") {
		t.Fatalf("status %d body %s, want 502 with error", rec.Code, rec.Body)
	}
}

func TestConfig(t *testing.T) {
	rec := httptest.NewRecorder()
	newMux(nil).ServeHTTP(rec, httptest.NewRequest("GET", "/api/config", nil))
	var cfg struct {
		Players  []string
		Surfaces []struct{ ID string }
		Formats  []struct{ ID string }
	}
	if err := json.NewDecoder(rec.Body).Decode(&cfg); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Players) < 100 || len(cfg.Surfaces) != 3 || cfg.Surfaces[0].ID != "hard" || len(cfg.Formats) != 2 {
		t.Fatalf("config = %d players, %+v, %+v", len(cfg.Players), cfg.Surfaces, cfg.Formats)
	}
}
