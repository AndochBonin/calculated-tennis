package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

const okRates = `"rates":[{"hold":0.85,"brk":0.3},{"hold":0.75,"brk":0.2}]`

func TestSimulate(t *testing.T) {
	resp, err := simulate(`{"playerA":"A","playerB":"B","surface":"clay","format":"gs-men","sims":200,` + okRates + `}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Matches) != 200 || resp.Surface != "clay" || resp.Alpha != 1.3 || resp.Rates[0].Hold != 0.85 {
		t.Fatalf("unexpected response: %d matches, surface %q, alpha %v, rates %+v", len(resp.Matches), resp.Surface, resp.Alpha, resp.Rates)
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
		if _, err := simulate(body); err == nil {
			t.Errorf("%s: want error", body)
		}
	}
}

func TestRatesFromHTML(t *testing.T) {
	html, err := os.ReadFile("../../tennisabstract/testdata/player_medvedev.html")
	if err != nil {
		t.Fatal(err)
	}
	r, err := ratesFromHTML("Daniil Medvedev", string(html), time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	if err != nil || r.Hold <= 0 || r.Hold >= 1 || r.Brk <= 0 || r.Brk >= 1 {
		t.Fatalf("rates %+v err %v", r, err)
	}

	// No season for the evaluation year.
	_, err = ratesFromHTML("Retired", string(html), time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC))
	if err == nil || err.Error() != "No data found for Retired" {
		t.Fatalf("err = %v, want no data", err)
	}

	if _, err := ratesFromHTML("Empty", "<p>nothing</p>", time.Now()); err == nil || !strings.HasPrefix(err.Error(), "Empty: ") {
		t.Fatalf("err = %v, want parse error", err)
	}
}

func TestConfig(t *testing.T) {
	var cfg struct {
		Players  []string
		Surfaces []struct {
			ID    string
			Alpha float64
		}
		Formats []struct{ ID string }
	}
	if err := json.Unmarshal([]byte(toJSON(config(), nil)), &cfg); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Players) < 100 || len(cfg.Surfaces) != 3 || cfg.Surfaces[0].ID != "hard" || cfg.Surfaces[2].Alpha != 0.7 || len(cfg.Formats) != 2 {
		t.Fatalf("config = %d players, %+v, %+v", len(cfg.Players), cfg.Surfaces, cfg.Formats)
	}
	if got := toJSON(nil, os.ErrNotExist); got != `{"error":"file does not exist"}` {
		t.Fatalf("toJSON error = %s", got)
	}
}
