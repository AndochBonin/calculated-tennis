package tennisabstract

import (
	"testing"
)

func TestTitleWord_empty(t *testing.T) {
	t.Parallel()
	if got := titleWord(""); got != "" {
		t.Fatalf("titleWord(\"\") = %q, want empty", got)
	}
}

func TestPlayerSlug(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "display name",
			in:   "Daniil Medvedev",
			want: "DaniilMedvedev",
		},
		{
			name: "existing slug unchanged",
			in:   "DaniilMedvedev",
			want: "DaniilMedvedev",
		},
		{
			name: "lowercase display name",
			in:   "daniil medvedev",
			want: "DaniilMedvedev",
		},
		{
			name: "extra whitespace",
			in:   "  Daniil   Medvedev  ",
			want: "DaniilMedvedev",
		},
		{
			name: "three word name",
			in:   "Carlos Alcaraz Garfia",
			want: "CarlosAlcarazGarfia",
		},
		{
			name: "particle de",
			in:   "Alex de Minaur",
			want: "AlexDeMinaur",
		},
		{
			name: "slug with different casing preserved",
			in:   "novakdjokovic",
			want: "novakdjokovic",
		},
		{
			name: "empty",
			in:   "",
			want: "",
		},
		{
			name: "whitespace only",
			in:   "   ",
			want: "",
		},
		{
			name: "hyphenated display name title-cases whole token",
			in:   "Jean-Luc Paglieri",
			want: "Jean-lucPaglieri",
		},
		{
			name: "hyphenated single token not treated as slug",
			in:   "Jean-Luc",
			want: "Jean-luc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := PlayerSlug(tt.in); got != tt.want {
				t.Fatalf("PlayerSlug(%q): got %q want %q", tt.in, got, tt.want)
			}
		})
	}
}
