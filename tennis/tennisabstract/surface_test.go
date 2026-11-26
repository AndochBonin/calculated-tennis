package tennisabstract

import (
	"strings"
	"testing"
)

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

func TestAlpha(t *testing.T) {
	if Alpha(SurfaceClay) != 1.3 || Alpha(SurfaceGrass) != 0.7 || Alpha(SurfaceHard) != 1.0 || Alpha("") != 1.0 {
		t.Fatal("unexpected alpha")
	}
}
