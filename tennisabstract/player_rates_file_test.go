package tennisabstract

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteReadPlayerRatesFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "player_rates_2024.json")

	want := PlayerRatesMap{
		"ZedPlayer": {Hold2024: 0.75, Break2024: 0.25},
		"AmyPlayer": {Hold2024: 0.80, Break2024: 0.27},
	}
	if err := WritePlayerRatesFile(path, want); err != nil {
		t.Fatalf("WritePlayerRatesFile: %v", err)
	}

	got, err := ReadPlayerRatesFile(path)
	if err != nil {
		t.Fatalf("ReadPlayerRatesFile: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for slug, rates := range want {
		g, ok := got[slug]
		if !ok {
			t.Fatalf("missing slug %q", slug)
		}
		if g != rates {
			t.Fatalf("%q: got %+v want %+v", slug, g, rates)
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Stable key order: Amy before Zed.
	const prefix = "{\n  \"AmyPlayer\""
	if string(data[:len(prefix)]) != prefix {
		t.Fatalf("unexpected file start:\n%s", data)
	}
}
