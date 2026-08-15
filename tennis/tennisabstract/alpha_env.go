package tennisabstract

// Alpha (projection sensitivity) resolves per surface from
// TENNISABSTRACT_ALPHA_{HARD|CLAY|GRASS}, then the global TENNISABSTRACT_ALPHA,
// then the code default. This mirrors the surface-specific form-tuning pattern
// in form_env.go, letting callers (e.g. the TUI) drop alpha as a manual input.

const (
	alphaEnv     = "TENNISABSTRACT_ALPHA"
	defaultAlpha = 1.0
)

// AlphaFromEnv returns the projection alpha for the given surface. A positive
// surface-specific override wins; otherwise the global override; otherwise
// defaultAlpha.
func AlphaFromEnv(surface MatchSurface) float64 {
	if surfSuffix, ok := surfaceEnvSuffix(surface); ok {
		if v, ok := positiveFloatFromEnvKey(alphaEnv + surfSuffix); ok {
			return v
		}
	}
	return positiveFloatFromEnv(alphaEnv, defaultAlpha)
}
