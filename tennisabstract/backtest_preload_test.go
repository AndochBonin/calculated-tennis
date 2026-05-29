package tennisabstract

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPreloadCalibrationMatchRecent_dedupesBySlugAndDate(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/player-classic.cgi":
			_, _ = w.Write(medvedevClassicSnippet(t))
		case "/jsmatches/DaniilMedvedevCareer.js":
			_, _ = w.Write(medvedevCareerJSSnippet(t))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	c := NewClient(WithBaseURL(srv.URL))
	matches := []CalibrationMatch{
		{
			PlayerA: "Daniil Medvedev", PlayerASlug: "DaniilMedvedev",
			PlayerB: "Opponent One", PlayerBSlug: "OpponentOne",
			TourneyDate: 20260502,
		},
		{
			PlayerA: "Daniil Medvedev", PlayerASlug: "DaniilMedvedev",
			PlayerB: "Opponent Two", PlayerBSlug: "OpponentTwo",
			TourneyDate: 20260502,
		},
	}

	preload := PreloadCalibrationMatchRecent(context.Background(), c, matches, 15)
	if len(preload.PerMatch) != 2 {
		t.Fatalf("PerMatch len=%d, want 2", len(preload.PerMatch))
	}
	if len(preload.PerMatch[0].RecentA) == 0 || len(preload.PerMatch[1].RecentA) == 0 {
		t.Fatal("expected Medvedev recent rows for both matches")
	}
	if &preload.PerMatch[0].RecentA[0] != &preload.PerMatch[1].RecentA[0] {
		t.Fatal("same slug and tourney date should share one cached recent slice")
	}
}

func TestMatchPlayerRatesFromPreload_matchesLivePath(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/player-classic.cgi":
			_, _ = w.Write(medvedevClassicSnippet(t))
		case "/jsmatches/DaniilMedvedevCareer.js":
			_, _ = w.Write(medvedevCareerJSSnippet(t))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	c := NewClient(WithBaseURL(srv.URL))
	ctx := context.Background()
	m := CalibrationMatch{
		PlayerA: "Daniil Medvedev", PlayerASlug: "DaniilMedvedev",
		PlayerB: "Daniil Medvedev", PlayerBSlug: "DaniilMedvedev",
		TourneyDate: 20260506,
	}
	rates := PlayerRatesMap{
		"DaniilMedvedev": {Hold2024: 0.801, Break2024: 0.27},
	}
	formOpts := FormOptions{
		HalfLifeMatches: 5,
		FormWeightMax:   0.3,
		FormRatioMin:    0.92,
		FormRatioMax:    1.08,
	}

	preload := PreloadCalibrationMatchRecent(ctx, c, []CalibrationMatch{m}, 15)
	if !preload.PerMatch[0].OK {
		t.Fatal("preload failed")
	}

	fromPreload, ok := MatchPlayerRatesFromPreload(m, rates, preload, 0, formOpts)
	if !ok {
		t.Fatal("MatchPlayerRatesFromPreload returned false")
	}

	live, ok := MatchPlayerRates(ctx, m, rates, c, true, formOpts)
	if !ok {
		t.Fatal("MatchPlayerRates returned false")
	}

	const eps = 1e-12
	if abs(fromPreload[0].HoldPct-live[0].HoldPct) > eps || abs(fromPreload[0].BreakPct-live[0].BreakPct) > eps {
		t.Fatalf("player A preload=%+v live=%+v", fromPreload[0], live[0])
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func TestPreloadCalibrationMatchRecent_nilClient(t *testing.T) {
	t.Parallel()

	preload := PreloadCalibrationMatchRecent(context.Background(), nil, []CalibrationMatch{{TourneyDate: 20250101}}, 0)
	if len(preload.PerMatch) != 1 || preload.PerMatch[0].OK {
		t.Fatalf("nil client: %+v", preload.PerMatch[0])
	}
}

func TestPreloadCalibrationMatchRecent_invalidDate(t *testing.T) {
	t.Parallel()

	preload := PreloadCalibrationMatchRecent(context.Background(), NewClient(), []CalibrationMatch{{TourneyDate: 0}}, 0)
	if preload.PerMatch[0].OK {
		t.Fatal("expected not ok for invalid tourney date")
	}
}
