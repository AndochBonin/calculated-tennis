package tennisabstract

import (
	"strings"
	"testing"
)

func clearFormEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		formMinSeasonMatchesEnv,
		formRecentMatchLimitEnv,
		formHalfLifeMatchesEnv,
		formWeightMaxEnv,
		formRatioMinEnv,
		formRatioMaxEnv,
		formChallengerWeightEnv,
		"TENNISABSTRACT_FORM_HALF_LIFE_MATCHES_HARD",
		"TENNISABSTRACT_FORM_WEIGHT_MAX_HARD",
		"TENNISABSTRACT_FORM_RATIO_MIN_HARD",
		"TENNISABSTRACT_FORM_RATIO_MAX_HARD",
		"TENNISABSTRACT_FORM_HALF_LIFE_MATCHES_CLAY",
		"TENNISABSTRACT_FORM_WEIGHT_MAX_CLAY",
		"TENNISABSTRACT_FORM_RATIO_MIN_CLAY",
		"TENNISABSTRACT_FORM_RATIO_MAX_CLAY",
		"TENNISABSTRACT_FORM_HALF_LIFE_MATCHES_GRASS",
		"TENNISABSTRACT_FORM_WEIGHT_MAX_GRASS",
		"TENNISABSTRACT_FORM_RATIO_MIN_GRASS",
		"TENNISABSTRACT_FORM_RATIO_MAX_GRASS",
	} {
		t.Setenv(key, "")
	}
}

func TestFormOptionsFromEnv_clayOverridesGlobal(t *testing.T) {
	clearFormEnv(t)

	t.Setenv(formHalfLifeMatchesEnv, "5")
	t.Setenv(formWeightMaxEnv, "0.30")
	t.Setenv(formRatioMinEnv, "0.84")
	t.Setenv(formRatioMaxEnv, "1.12")

	t.Setenv("TENNISABSTRACT_FORM_HALF_LIFE_MATCHES_CLAY", "7")
	t.Setenv("TENNISABSTRACT_FORM_WEIGHT_MAX_CLAY", "0.70")
	t.Setenv("TENNISABSTRACT_FORM_RATIO_MIN_CLAY", "0.92")
	t.Setenv("TENNISABSTRACT_FORM_RATIO_MAX_CLAY", "1.20")

	got := FormOptionsFromEnv(SurfaceClay)
	if got.HalfLifeMatches != 7 || got.FormWeightMax != 0.70 ||
		got.FormRatioMin != 0.92 || got.FormRatioMax != 1.20 {
		t.Fatalf("clay overrides: %+v", got)
	}
	if got.MinSeasonMatches != defaultMinSeasonMatches ||
		got.RecentMatchLimit != defaultRecentMatchLimit {
		t.Fatalf("globals from withDefaults: %+v", got)
	}
}

func TestFormOptionsFromEnv_globalOnly(t *testing.T) {
	clearFormEnv(t)

	t.Setenv(formHalfLifeMatchesEnv, "4")
	t.Setenv(formWeightMaxEnv, "0.25")
	t.Setenv(formRatioMinEnv, "0.90")
	t.Setenv(formRatioMaxEnv, "1.10")
	t.Setenv(formMinSeasonMatchesEnv, "22")

	got := FormOptionsFromEnv(SurfaceClay)
	if got.HalfLifeMatches != 4 || got.FormWeightMax != 0.25 ||
		got.FormRatioMin != 0.90 || got.FormRatioMax != 1.10 {
		t.Fatalf("global tuned fields: %+v", got)
	}
	if got.MinSeasonMatches != 22 {
		t.Fatalf("global min season: %d", got.MinSeasonMatches)
	}
}

func TestFormOptionsFromEnv_emptySurfaceUsesGlobal(t *testing.T) {
	clearFormEnv(t)

	t.Setenv(formHalfLifeMatchesEnv, "6")
	t.Setenv(formWeightMaxEnv, "0.40")

	got := FormOptionsFromEnv("")
	if got.HalfLifeMatches != 6 || got.FormWeightMax != 0.40 {
		t.Fatalf("empty surface globals: %+v", got)
	}
}

func TestFormOptionsFromEnv_invalidSurfaceOverrideFallsBack(t *testing.T) {
	clearFormEnv(t)

	t.Setenv(formWeightMaxEnv, "0.30")
	t.Setenv("TENNISABSTRACT_FORM_WEIGHT_MAX_CLAY", "not-a-number")

	got := FormOptionsFromEnv(SurfaceClay)
	if got.FormWeightMax != 0.30 {
		t.Fatalf("invalid clay override → global: got %v", got.FormWeightMax)
	}
}

func TestParseMatchSurface(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in    string
		want  MatchSurface
		isErr bool
	}{
		{"hard", SurfaceHard, false},
		{"HARD", SurfaceHard, false},
		{" clay ", SurfaceClay, false},
		{"grass", SurfaceGrass, false},
		{"Hard", SurfaceHard, false},
		{"Clay", SurfaceClay, false},
		{"Grass", SurfaceGrass, false},
		{"", "", true},
		{"carpet", "", true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			got, err := ParseMatchSurface(tc.in)
			if tc.isErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParseMatchSurface_errorMessage(t *testing.T) {
	_, err := ParseMatchSurface("indoor")
	if err == nil || !strings.Contains(err.Error(), "indoor") {
		t.Fatalf("error = %v", err)
	}
}
