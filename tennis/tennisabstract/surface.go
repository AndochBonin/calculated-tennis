package tennisabstract

import (
	"fmt"
	"strings"
)

// MatchSurface is a normalized court surface.
type MatchSurface string

const (
	SurfaceHard  MatchSurface = "Hard"
	SurfaceClay  MatchSurface = "Clay"
	SurfaceGrass MatchSurface = "Grass"
)

// alphas are the calibrated projection alphas per surface. Lower = more
// lopsided (better player dominates); higher = closer.
var alphas = map[MatchSurface]float64{SurfaceHard: 1.0, SurfaceClay: 1.3, SurfaceGrass: 0.7}

// Alpha returns the projection alpha for the surface (1.0 when unknown).
func Alpha(surface MatchSurface) float64 {
	if a, ok := alphas[surface]; ok {
		return a
	}
	return 1.0
}

// ParseMatchSurface accepts hard, clay, or grass (case-insensitive).
func ParseMatchSurface(raw string) (MatchSurface, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "hard":
		return SurfaceHard, nil
	case "clay":
		return SurfaceClay, nil
	case "grass":
		return SurfaceGrass, nil
	}
	return "", fmt.Errorf("unknown match surface %q (want hard, clay, or grass)", raw)
}
