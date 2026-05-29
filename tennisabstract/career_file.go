package tennisabstract

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AndochBonin/polymarket/models"
)

const (
	defaultCareerCacheDir = "tennisabstract/testdata/career"
	careerCacheDirEnv     = "TENNISABSTRACT_CAREER_DIR"
)

// jsonMarshalIndent is swappable in tests to cover marshal error handling.
var jsonMarshalIndent = func(v any) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}

// DefaultCareerCacheDir returns the default on-disk career matches directory.
func DefaultCareerCacheDir() string {
	return defaultCareerCacheDir
}

// CareerCacheDirFromEnv returns TENNISABSTRACT_CAREER_DIR if set, else DefaultCareerCacheDir.
func CareerCacheDirFromEnv() string {
	if v := strings.TrimSpace(os.Getenv(careerCacheDirEnv)); v != "" {
		return v
	}
	return DefaultCareerCacheDir()
}

// CareerMatchesFilePath returns {dir}/{lowercase_slug}.json.
func CareerMatchesFilePath(dir, slug string) string {
	base := strings.ToLower(strings.TrimSpace(slug))
	return filepath.Join(dir, base+".json")
}

// ReadCareerMatchesFile loads one player's career matches from disk.
// ok is false when the file is missing (not an error).
func ReadCareerMatchesFile(dir, slug string) (models.CareerMatches, bool, error) {
	path := CareerMatchesFilePath(dir, slug)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return models.CareerMatches{}, false, nil
		}
		return models.CareerMatches{}, false, fmt.Errorf("read career matches %q: %w", path, err)
	}
	var career models.CareerMatches
	if err := json.Unmarshal(data, &career); err != nil {
		return models.CareerMatches{}, false, fmt.Errorf("decode career matches %q: %w", path, err)
	}
	return career, true, nil
}

// WriteCareerMatchesFile writes career matches as indented JSON, creating dir if needed.
func WriteCareerMatchesFile(dir, slug string, career models.CareerMatches) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create career cache dir %q: %w", dir, err)
	}
	raw, err := jsonMarshalIndent(career)
	if err != nil {
		return fmt.Errorf("marshal career matches: %w", err)
	}
	path := CareerMatchesFilePath(dir, slug)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return fmt.Errorf("write career matches %q: %w", path, err)
	}
	return nil
}
