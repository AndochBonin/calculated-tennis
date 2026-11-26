package tennisabstract

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Surface-specific form tuning reads TENNISABSTRACT_FORM_{field}_{SURFACE} when the
// match surface is Hard, Clay, or Grass (suffixes _HARD, _CLAY, _GRASS). Each field
// falls back to the global key, then to code defaults in form.go:
//
//   - HALF_LIFE_MATCHES  → TENNISABSTRACT_FORM_HALF_LIFE_MATCHES
//   - WEIGHT_MAX         → TENNISABSTRACT_FORM_WEIGHT_MAX
//   - RATIO_MIN / MAX    → TENNISABSTRACT_FORM_RATIO_MIN / _MAX
//
// Global keys still govern MIN_SEASON_MATCHES, RECENT_MATCH_LIMIT, and CHALLENGER_WEIGHT
// via FormOptions.withDefaults(). Recent-match selection remains all surfaces; only
// the four tuned fields vary by surface.
//
// Unknown or empty MatchSurface uses global env + defaults only (no surface override).

// FormOptionsFromEnv returns form tuning for hold/break adjustment on the given surface.
// HalfLifeMatches, FormWeightMax, FormRatioMin, and FormRatioMax resolve from
// surface-specific env vars when set; MinSeasonMatches, RecentMatchLimit, and
// ChallengerWeight come from global env via withDefaults.
func FormOptionsFromEnv(surface MatchSurface) FormOptions {
	opts := FormOptions{
		HalfLifeMatches: floatFormEnv(surface, "HALF_LIFE_MATCHES", formHalfLifeMatchesEnv, defaultHalfLifeMatches),
		FormWeightMax:   floatFormEnv(surface, "WEIGHT_MAX", formWeightMaxEnv, defaultFormWeightMax),
		FormRatioMin:    floatFormEnv(surface, "RATIO_MIN", formRatioMinEnv, defaultFormRatioMin),
		FormRatioMax:    floatFormEnv(surface, "RATIO_MAX", formRatioMaxEnv, defaultFormRatioMax),
	}
	return opts.withDefaults()
}

// ParseMatchSurface accepts hard, clay, or grass (case-insensitive) and canonical
// CSV values Hard, Clay, Grass.
func ParseMatchSurface(raw string) (MatchSurface, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "hard":
		return SurfaceHard, nil
	case "clay":
		return SurfaceClay, nil
	case "grass":
		return SurfaceGrass, nil
	}
	if s, ok := normalizeMatchSurface(raw); ok {
		return s, nil
	}
	return "", fmt.Errorf("unknown match surface %q (want hard, clay, or grass)", raw)
}

func floatFormEnv(surface MatchSurface, fieldSuffix, globalKey string, fallback float64) float64 {
	if key, ok := surfaceFormEnvKey(fieldSuffix, surface); ok {
		if v, ok := positiveFloatFromEnvKey(key); ok {
			return v
		}
	}
	return positiveFloatFromEnv(globalKey, fallback)
}

func surfaceFormEnvKey(fieldSuffix string, surface MatchSurface) (string, bool) {
	surfSuffix, ok := surfaceEnvSuffix(surface)
	if !ok {
		return "", false
	}
	return "TENNISABSTRACT_FORM_" + fieldSuffix + surfSuffix, true
}

func surfaceEnvSuffix(surface MatchSurface) (string, bool) {
	switch surface {
	case SurfaceHard:
		return "_HARD", true
	case SurfaceClay:
		return "_CLAY", true
	case SurfaceGrass:
		return "_GRASS", true
	default:
		return "", false
	}
}

func positiveFloatFromEnvKey(key string) (float64, bool) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(raw, 64)
	if err != nil || f <= 0 {
		return 0, false
	}
	return f, true
}
