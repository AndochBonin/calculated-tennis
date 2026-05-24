package main

import (
	"bufio"
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/AndochBonin/polymarket/internal/prompt"
	"github.com/AndochBonin/polymarket/tennis"
)

func TestResolveInputs_flagsOnly(t *testing.T) {
	t.Parallel()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	in, err := resolveInputs(r,
		"Daniil Medvedev", "Jannik Sinner", "atp", "2.5", "10000",
		prompt.IsInteractive,
	)
	if err != nil {
		t.Fatalf("resolveInputs: %v", err)
	}
	if in.playerA != "Daniil Medvedev" || in.playerB != "Jannik Sinner" {
		t.Fatalf("players: A=%q B=%q", in.playerA, in.playerB)
	}
	if in.formatLabel != "ATP best-of-3" {
		t.Fatalf("format label %q", in.formatLabel)
	}
	if in.alpha != 2.5 || in.sims != 10000 {
		t.Fatalf("alpha=%v sims=%d", in.alpha, in.sims)
	}
}

func TestResolveInputs_formatCaseInsensitive(t *testing.T) {
	t.Parallel()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	in, err := resolveInputs(r, "a", "b", "GS-MEN", "1", "1", prompt.IsInteractive)
	if err != nil {
		t.Fatalf("resolveInputs: %v", err)
	}
	if in.formatLabel != "Grand Slam men best-of-5" {
		t.Fatalf("format label %q", in.formatLabel)
	}
	want := tennis.GrandSlamMenFormat()
	if in.format != want {
		t.Fatalf("format %+v want %+v", in.format, want)
	}
}

func TestResolveInputs_interactivePrompts(t *testing.T) {
	t.Parallel()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close(); w.Close() })

	input := strings.Join([]string{
		"Daniil Medvedev",
		"Jannik Sinner",
		"2",
		"2.5",
		"5000",
	}, "\n") + "\n"
	if _, err := w.WriteString(input); err != nil {
		t.Fatal(err)
	}

	in, err := resolveInputs(r, "", "", "", "", "", func(*os.File) bool { return true })
	if err != nil {
		t.Fatalf("resolveInputs: %v", err)
	}
	if in.playerA != "Daniil Medvedev" || in.playerB != "Jannik Sinner" {
		t.Fatalf("players: A=%q B=%q", in.playerA, in.playerB)
	}
	if in.formatLabel != "Grand Slam men best-of-5" {
		t.Fatalf("format label %q", in.formatLabel)
	}
	if in.alpha != 2.5 || in.sims != 5000 {
		t.Fatalf("alpha=%v sims=%d", in.alpha, in.sims)
	}
}

func TestResolveInputs_nonInteractiveMissing(t *testing.T) {
	t.Parallel()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	_, err = resolveInputs(r, "", "", "", "", "", prompt.IsInteractive)
	if !errors.Is(err, errUsage) {
		t.Fatalf("got err %v want errUsage", err)
	}
}

func TestParseFormatChoice_valid(t *testing.T) {
	t.Parallel()
	cases := []struct {
		raw   string
		label string
		want  tennis.MatchFormat
	}{
		{"1", "ATP best-of-3", tennis.DefaultFormat()},
		{"atp", "ATP best-of-3", tennis.DefaultFormat()},
		{"2", "Grand Slam men best-of-5", tennis.GrandSlamMenFormat()},
		{"gs-men", "Grand Slam men best-of-5", tennis.GrandSlamMenFormat()},
		{"gs_men", "Grand Slam men best-of-5", tennis.GrandSlamMenFormat()},
		{"3", "Grand Slam women best-of-3", tennis.GrandSlamWomenFormat()},
		{"gs-women", "Grand Slam women best-of-3", tennis.GrandSlamWomenFormat()},
		{"gs_women", "Grand Slam women best-of-3", tennis.GrandSlamWomenFormat()},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.raw, func(t *testing.T) {
			t.Parallel()
			format, label, err := parseFormatChoice(tc.raw)
			if err != nil {
				t.Fatalf("parseFormatChoice(%q): %v", tc.raw, err)
			}
			if label != tc.label {
				t.Fatalf("label %q want %q", label, tc.label)
			}
			if format != tc.want {
				t.Fatalf("format %+v want %+v", format, tc.want)
			}
		})
	}
}

func TestResolveFormat_invalid(t *testing.T) {
	t.Parallel()
	_, _, err := parseFormatChoice("wimbledon")
	if !errors.Is(err, errInvalidFormat) {
		t.Fatalf("got err %v want errInvalidFormat", err)
	}
}

func TestResolvePlayerName_flag(t *testing.T) {
	t.Parallel()
	name, err := resolvePlayerName("Daniil Medvedev", "", nil)
	if err != nil {
		t.Fatalf("resolvePlayerName: %v", err)
	}
	if name != "Daniil Medvedev" {
		t.Fatalf("got %q", name)
	}
}

func TestResolvePlayerName_trimsFlag(t *testing.T) {
	t.Parallel()
	name, err := resolvePlayerName("  Jannik Sinner  ", "", nil)
	if err != nil {
		t.Fatalf("resolvePlayerName: %v", err)
	}
	if name != "Jannik Sinner" {
		t.Fatalf("got %q", name)
	}
}

func TestResolvePlayerName_interactivePrompt(t *testing.T) {
	t.Parallel()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close(); w.Close() })

	if _, err := w.WriteString("Carlos Alcaraz\n"); err != nil {
		t.Fatal(err)
	}
	br := bufio.NewReader(r)

	name, err := resolvePlayerName("", "Player A name: ", br)
	if err != nil {
		t.Fatalf("resolvePlayerName: %v", err)
	}
	if name != "Carlos Alcaraz" {
		t.Fatalf("got %q", name)
	}
}

func TestResolvePlayerName_nonInteractiveMissing(t *testing.T) {
	t.Parallel()
	_, err := resolvePlayerName("", "", nil)
	if !errors.Is(err, errUsage) {
		t.Fatalf("got err %v want errUsage", err)
	}
}

func TestResolvePlayerName_interactiveEmptyAfterPrompt(t *testing.T) {
	t.Parallel()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close(); w.Close() })

	if _, err := w.WriteString("\n"); err != nil {
		t.Fatal(err)
	}
	br := bufio.NewReader(r)

	_, err = resolvePlayerName("", "Player A name: ", br)
	if !errors.Is(err, errUsage) {
		t.Fatalf("got err %v want errUsage", err)
	}
}

func TestResolveInputs_trimsFlags(t *testing.T) {
	t.Parallel()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	in, err := resolveInputs(r,
		"  Player A  ", "  Player B  ", "  atp  ", "  2.5  ", "  100  ",
		prompt.IsInteractive,
	)
	if err != nil {
		t.Fatalf("resolveInputs: %v", err)
	}
	if in.playerA != "Player A" || in.playerB != "Player B" {
		t.Fatalf("players: A=%q B=%q", in.playerA, in.playerB)
	}
	if in.alpha != 2.5 || in.sims != 100 {
		t.Fatalf("alpha=%v sims=%d", in.alpha, in.sims)
	}
}

func TestResolveInputs_interactivePartialFlags(t *testing.T) {
	t.Parallel()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close(); w.Close() })

	if _, err := w.WriteString("gs-women\n2500\n"); err != nil {
		t.Fatal(err)
	}

	in, err := resolveInputs(r, "A", "B", "", "3", "", func(*os.File) bool { return true })
	if err != nil {
		t.Fatalf("resolveInputs: %v", err)
	}
	if in.playerA != "A" || in.playerB != "B" {
		t.Fatalf("players: A=%q B=%q", in.playerA, in.playerB)
	}
	if in.formatLabel != "Grand Slam women best-of-3" {
		t.Fatalf("format label %q", in.formatLabel)
	}
	if in.alpha != 3 || in.sims != 2500 {
		t.Fatalf("alpha=%v sims=%d", in.alpha, in.sims)
	}
}

func TestResolveAlpha_invalid(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"0", "-1", "not-a-number"} {
		_, err := resolveAlpha(raw, nil)
		if !errors.Is(err, errInvalidAlpha) {
			t.Fatalf("alpha %q: got err %v want errInvalidAlpha", raw, err)
		}
	}
}

func TestResolveSims_invalid(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"0", "-5", "nope"} {
		_, err := resolveSims(raw, nil)
		if !errors.Is(err, errInvalidSims) {
			t.Fatalf("sims %q: got err %v want errInvalidSims", raw, err)
		}
	}
}

func TestPrintSummary(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	names := [2]string{"Player A", "Player B"}
	rates := [2]tennis.PlayerRates{
		{HoldPct: 0.821, BreakPct: 0.243},
		{HoldPct: 0.852, BreakPct: 0.281},
	}
	in := simInputs{
		formatLabel: "ATP best-of-3",
		alpha:       2.5,
		sims:        10000,
	}
	result := tennis.SimulationResult{Wins: [2]int{4521, 5479}}

	printSummary(&buf, names, rates, in, result)
	out := buf.String()
	for _, want := range []string{
		"Match simulation",
		"Player A: Player A  (hold 82.1%, break 24.3%)",
		"Player B: Player B  (hold 85.2%, break 28.1%)",
		"Format:   ATP best-of-3",
		"Alpha:    2.5",
		"Sims:     10000",
		"Results",
		"Player A:  4521 wins (45.2%)",
		"Player B:  5479 wins (54.8%)",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}
