package tennisabstract

import "testing"

func TestPlayerKeyFromSackmannName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		in        string
		wantLast  string
		wantInitial string
	}{
		{"simple", "Daniil Medvedev", "medvedev", "d"},
		{"particle", "Alex De Minaur", "deminaur", "a"},
		{"oconnell", "Christopher Oconnell", "oconnell", "c"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := playerKeyFromSackmannName(tc.in)
			if got.last != tc.wantLast || got.initial != tc.wantInitial {
				t.Fatalf("got %+v, want last=%q initial=%q", got, tc.wantLast, tc.wantInitial)
			}
		})
	}
}

func TestPlayerKeyFromOddsName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		in        string
		wantLast  string
		wantInitial string
	}{
		{"simple", "Medvedev D.", "medvedev", "d"},
		{"particle", "De Minaur A.", "deminaur", "a"},
		{"oconnell", "O Connell C.", "oconnell", "c"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := playerKeyFromOddsName(tc.in)
			if got.last != tc.wantLast || got.initial != tc.wantInitial {
				t.Fatalf("got %+v, want last=%q initial=%q", got, tc.wantLast, tc.wantInitial)
			}
		})
	}
}

func TestMatchPlayerKeys(t *testing.T) {
	t.Parallel()

	a := playerKeyFromSackmannName("Daniil Medvedev")
	b := playerKeyFromOddsName("Medvedev D.")
	if !matchPlayerKeys(a, b) {
		t.Fatalf("expected match between %+v and %+v", a, b)
	}
}
