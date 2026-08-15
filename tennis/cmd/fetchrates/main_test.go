package main

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/AndochBonin/calculated-tennis/tennis/tennisabstract"
)

func TestFetchRates_integration(t *testing.T) {
	t.Parallel()

	fixture, err := os.ReadFile("../../tennisabstract/testdata/player_medvedev.html")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/jsfrags/DaniilMedvedev.js" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/javascript")
		_, _ = w.Write([]byte("var player_frag = `"))
		_, _ = w.Write(fixture)
		_, _ = w.Write([]byte("`;"))
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	csvPath := filepath.Join(dir, "matches.csv")
	if err := os.WriteFile(csvPath, []byte("winner_name,loser_name\nDaniil Medvedev,Daniil Medvedev\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(dir, "rates.json")

	client := tennisabstract.NewClient(
		tennisabstract.WithBaseURL(srv.URL),
		tennisabstract.WithHTTPClient(srv.Client()),
	)

	names, err := readUniqueNames(csvPath)
	if err != nil {
		t.Fatalf("readUniqueNames: %v", err)
	}
	rates := tennisabstract.PlayerRatesMap{}
	ctx := context.Background()
	for _, name := range names {
		stats, err := client.GetPlayerStats(ctx, name)
		if err != nil {
			t.Fatalf("GetPlayerStats(%q): %v", name, err)
		}
		hold, brk, dr, err := tennisabstract.SeasonBaseline(stats, 2024)
		if err != nil {
			t.Fatalf("SeasonBaseline(%q): %v", name, err)
		}
		rates[stats.PlayerSlug] = tennisabstract.PlayerRates2024{
			Hold2024:  hold,
			Break2024: brk,
			DR2024:    dr,
		}
	}
	if err := tennisabstract.WritePlayerRatesFile(outPath, rates); err != nil {
		t.Fatalf("WritePlayerRatesFile: %v", err)
	}

	got, err := tennisabstract.ReadPlayerRatesFile(outPath)
	if err != nil {
		t.Fatalf("ReadPlayerRatesFile: %v", err)
	}
	med, ok := got["DaniilMedvedev"]
	if !ok {
		t.Fatalf("missing DaniilMedvedev in %v", got)
	}
	if math.Abs(med.Hold2024-0.801) > 1e-9 || math.Abs(med.Break2024-0.27) > 1e-9 {
		t.Fatalf("DaniilMedvedev rates = %+v, want hold 0.801 break 0.27", med)
	}
	if math.Abs(med.DR2024-1.10) > 1e-9 {
		t.Fatalf("DaniilMedvedev DR2024 = %v, want 1.10", med.DR2024)
	}
}
