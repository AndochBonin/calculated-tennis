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

const okRates = `"rates":[{"hold":0.85,"brk":0.3},{"hold":0.75,"brk":0.2}]`

func TestSimulate(t *testing.T) {
	rec := post(t, `{"playerA":"A","playerB":"B","surface":"clay","format":"gs-men","sims":200,`+okRates+`}`)
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
		`{"playerA":"A","playerB":"B","surface":"clay","format":"atp","sims":0,` + okRates + `}`,
		`{"playerA":"A","playerB":"B","surface":"clay","format":"atp","sims":100001,` + okRates + `}`,
		`{"playerA":"A","playerB":"B","surface":"carpet","format":"atp","sims":10,` + okRates + `}`,
		`{"playerA":"A","playerB":"B","surface":"hard","format":"bo7","sims":10,` + okRates + `}`,
		`{"playerA":" ","playerB":"B","surface":"hard","format":"atp","sims":10,` + okRates + `}`,
		`{"playerA":"A","playerB":"B","surface":"hard","format":"atp","sims":10,"rates":[{"hold":1.5,"brk":0.3},{"hold":0.75,"brk":0.2}]}`,
		// Both always hold: tiebreaks would never end.
		`{"playerA":"A","playerB":"B","surface":"hard","format":"atp","sims":10,"rates":[{"hold":1,"brk":0},{"hold":1,"brk":0}]}`,
		`not json`,
	} {
		if rec := post(t, body); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", body, rec.Code)
		}
	}
}

func TestRates(t *testing.T) {
	orig := fetchRate
	t.Cleanup(func() { fetchRate = orig })
	t.Setenv("TENNISABSTRACT_CACHE_TTL", "")
	fetchRate = func(_ context.Context, _ *tennisabstract.Client, name string, _ tennisabstract.MatchSurface) (tennis.PlayerRates, error) {
		if name == "Nobody" {
			return tennis.PlayerRates{}, errors.New("player not found")
		}
		return tennis.PlayerRates{HoldPct: 0.85, BreakPct: 0.3}, nil
	}
	get := func(query string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		newMux(nil).ServeHTTP(rec, httptest.NewRequest("GET", "/api/rates?"+query, nil))
		return rec
	}

	rec := get("player=Jannik+Sinner&surface=clay")
	var r rates
	if err := json.NewDecoder(rec.Body).Decode(&r); err != nil || rec.Code != http.StatusOK || r != (rates{0.85, 0.3}) {
		t.Fatalf("status %d rates %+v err %v", rec.Code, r, err)
	}
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "max-age=21600") {
		t.Fatalf("Cache-Control = %q", cc)
	}

	for _, q := range []string{"player=+&surface=clay", "player=A&surface=carpet"} {
		if rec := get(q); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", q, rec.Code)
		}
	}

	rec = get("player=Nobody&surface=hard")
	if rec.Code != http.StatusBadGateway || !strings.Contains(rec.Body.String(), "Nobody: player not found") || rec.Header().Get("Cache-Control") != "" {
		t.Fatalf("status %d body %s cache %q, want uncached 502", rec.Code, rec.Body, rec.Header().Get("Cache-Control"))
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
