package tennisabstract

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/AndochBonin/E3/tennis/models"
)

func TestDefaultCareerCacheDir(t *testing.T) {
	t.Parallel()
	if got := DefaultCareerCacheDir(); got != "tennisabstract/testdata/career" {
		t.Fatalf("DefaultCareerCacheDir() = %q", got)
	}
}

func TestCareerCacheDirFromEnv(t *testing.T) {
	t.Setenv(careerCacheDirEnv, "")
	if got := CareerCacheDirFromEnv(); got != DefaultCareerCacheDir() {
		t.Fatalf("empty env: got %q", got)
	}

	t.Setenv(careerCacheDirEnv, "/tmp/career-cache")
	if got := CareerCacheDirFromEnv(); got != "/tmp/career-cache" {
		t.Fatalf("custom env: got %q", got)
	}

	t.Setenv(careerCacheDirEnv, "  /custom/path  ")
	if got := CareerCacheDirFromEnv(); got != "/custom/path" {
		t.Fatalf("trimmed env: got %q", got)
	}
}

func TestCareerMatchesFilePath(t *testing.T) {
	t.Parallel()

	dir := "tennisabstract/testdata/career"
	got := CareerMatchesFilePath(dir, "  DaniilMedvedev  ")
	want := filepath.Join(dir, "daniilmedvedev.json")
	if got != want {
		t.Fatalf("CareerMatchesFilePath() = %q, want %q", got, want)
	}
}

func TestWriteReadCareerMatchesFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	want := models.CareerMatches{
		PlayerSlug: "DaniilMedvedev",
		FetchedAt:  time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC),
		Matches: []models.RecentResult{
			{Tournament: "Rome", Surface: "Clay"},
		},
	}
	if err := WriteCareerMatchesFile(dir, "DaniilMedvedev", want); err != nil {
		t.Fatalf("WriteCareerMatchesFile: %v", err)
	}

	got, ok, err := ReadCareerMatchesFile(dir, "daniilmedvedev")
	if err != nil {
		t.Fatalf("ReadCareerMatchesFile: %v", err)
	}
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got.PlayerSlug != want.PlayerSlug {
		t.Fatalf("PlayerSlug = %q, want %q", got.PlayerSlug, want.PlayerSlug)
	}
	if !got.FetchedAt.Equal(want.FetchedAt) {
		t.Fatalf("FetchedAt = %v, want %v", got.FetchedAt, want.FetchedAt)
	}
	if len(got.Matches) != 1 || got.Matches[0].Tournament != "Rome" {
		t.Fatalf("Matches = %+v", got.Matches)
	}

	data, err := os.ReadFile(CareerMatchesFilePath(dir, "DaniilMedvedev"))
	if err != nil {
		t.Fatal(err)
	}
	const prefix = "{\n  \"PlayerSlug\""
	if string(data[:len(prefix)]) != prefix {
		t.Fatalf("expected indented JSON, file start:\n%s", data)
	}
}

func TestReadCareerMatchesFile_missing(t *testing.T) {
	t.Parallel()

	_, ok, err := ReadCareerMatchesFile(t.TempDir(), "nobody")
	if err != nil {
		t.Fatalf("ReadCareerMatchesFile: %v", err)
	}
	if ok {
		t.Fatal("expected ok=false on missing file")
	}
}

func TestReadCareerMatchesFile_invalidJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := CareerMatchesFilePath(dir, "bad")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, ok, err := ReadCareerMatchesFile(dir, "bad")
	if err == nil {
		t.Fatal("expected decode error")
	}
	if ok {
		t.Fatal("ok should be false on corrupt file")
	}
}

func TestWriteCareerMatchesFile_marshalError(t *testing.T) {
	old := jsonMarshalIndent
	t.Cleanup(func() { jsonMarshalIndent = old })
	jsonMarshalIndent = func(any) ([]byte, error) {
		return nil, errors.New("marshal failed")
	}

	err := WriteCareerMatchesFile(t.TempDir(), "slug", models.CareerMatches{})
	if err == nil {
		t.Fatal("expected marshal error")
	}
}
