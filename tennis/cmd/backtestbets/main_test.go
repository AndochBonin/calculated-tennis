package main

import (
	"bufio"
	"errors"
	"os"
	"testing"

	"github.com/AndochBonin/calculated-tennis/tennis/internal/prompt"
)

func TestResolveBacktestInputs_flagsOnly(t *testing.T) {
	t.Parallel()

	r, w, err := osPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	in, err := resolveBacktestInputs(r, backtestInputFlags{
		stake:   "2.5",
		minPick: "0.6",
		sims:    "1000",
	}, prompt.IsInteractive)
	if err != nil {
		t.Fatal(err)
	}
	if in.stake != 2.5 || in.minPick != 0.6 || in.sims != 1000 {
		t.Fatalf("got stake=%v minPick=%v sims=%v", in.stake, in.minPick, in.sims)
	}
	if in.betMode != BetModeSingles || in.maxParlayMatches != 0 {
		t.Fatalf("got mode=%q maxParlay=%d, want singles/0", in.betMode, in.maxParlayMatches)
	}
}

func TestResolveBacktestInputs_nonInteractiveDefaults(t *testing.T) {
	t.Parallel()

	r, w, err := osPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	in, err := resolveBacktestInputs(r, backtestInputFlags{}, prompt.IsInteractive)
	if err != nil {
		t.Fatal(err)
	}
	if in.stake != defaultStake || in.minPick != defaultMinPick || in.sims != defaultSims {
		t.Fatalf("got stake=%v minPick=%v sims=%v, want defaults", in.stake, in.minPick, in.sims)
	}
}

func TestResolveBacktestInputs_interactivePrompts(t *testing.T) {
	t.Parallel()

	r, w, err := osPipe()
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		_, _ = w.WriteString("5\n")
		_, _ = w.WriteString("0.55\n")
		_, _ = w.WriteString("2000\n")
		_, _ = w.WriteString("\n") // min odds default
		_, _ = w.WriteString("\n") // require positive EV: no
		_, _ = w.WriteString("\n") // bet mode: singles
		w.Close()
	}()

	in, err := resolveBacktestInputs(r, backtestInputFlags{}, func(*os.File) bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	if in.stake != 5 || in.minPick != 0.55 || in.sims != 2000 {
		t.Fatalf("got stake=%v minPick=%v sims=%v", in.stake, in.minPick, in.sims)
	}
}

func TestResolveBacktestInputs_interactiveDefaultsOnEnter(t *testing.T) {
	t.Parallel()

	r, w, err := osPipe()
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		_, _ = w.WriteString("\n\n\n\n\n\n\n")
		w.Close()
	}()

	in, err := resolveBacktestInputs(r, backtestInputFlags{}, func(*os.File) bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	if in.stake != defaultStake || in.minPick != defaultMinPick || in.minOdds != defaultMinOdds || in.sims != defaultSims {
		t.Fatalf("got stake=%v minPick=%v sims=%v, want defaults", in.stake, in.minPick, in.sims)
	}
}

func TestResolveBacktestInputs_interactivePartialFlags(t *testing.T) {
	t.Parallel()

	r, w, err := osPipe()
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		_, _ = w.WriteString("0.7\n")
		_, _ = w.WriteString("\n") // min odds default
		_, _ = w.WriteString("\n") // require positive EV: no
		_, _ = w.WriteString("\n") // bet mode: singles
		w.Close()
	}()

	in, err := resolveBacktestInputs(r, backtestInputFlags{
		stake: "3",
		sims:  "500",
	}, func(*os.File) bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	if in.stake != 3 || in.minPick != 0.7 || in.sims != 500 {
		t.Fatalf("got stake=%v minPick=%v sims=%v", in.stake, in.minPick, in.sims)
	}
}

func TestResolveStake_invalid(t *testing.T) {
	t.Parallel()

	_, err := resolveStake("-1", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveMinPick_invalid(t *testing.T) {
	t.Parallel()

	_, err := resolveMinPick("1.5", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveSims_invalid(t *testing.T) {
	t.Parallel()

	_, err := resolveSims("0", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveRequirePositiveEV_flag(t *testing.T) {
	t.Parallel()

	r, w, err := osPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	in, err := resolveBacktestInputs(r, backtestInputFlags{requirePositiveEV: true}, prompt.IsInteractive)
	if err != nil {
		t.Fatal(err)
	}
	if !in.requirePositiveEV {
		t.Fatal("expected requirePositiveEV true from flag")
	}
}

func TestResolveMinOdds_flagAndDefault(t *testing.T) {
	t.Parallel()

	r, w, err := osPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	in, err := resolveBacktestInputs(r, backtestInputFlags{minOdds: "1.5"}, prompt.IsInteractive)
	if err != nil {
		t.Fatal(err)
	}
	if in.minOdds != 1.5 {
		t.Fatalf("minOdds = %v, want 1.5", in.minOdds)
	}
}

func TestResolveMinOdds_invalid(t *testing.T) {
	t.Parallel()

	_, err := resolveMinOdds("0.5", nil)
	if err == nil {
		t.Fatal("expected error for min-odds < 1")
	}
}

func TestResolveBacktestInputs_parlayFlags(t *testing.T) {
	t.Parallel()

	r, w, err := osPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	in, err := resolveBacktestInputs(r, backtestInputFlags{
		parlay:           true,
		maxParlayMatches: "3",
	}, prompt.IsInteractive)
	if err != nil {
		t.Fatal(err)
	}
	if in.betMode != BetModeParlay || in.maxParlayMatches != 3 {
		t.Fatalf("got mode=%q maxParlay=%d", in.betMode, in.maxParlayMatches)
	}
}

func TestResolveBacktestInputs_maxParlayFlagImpliesParlay(t *testing.T) {
	t.Parallel()

	r, w, err := osPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	in, err := resolveBacktestInputs(r, backtestInputFlags{maxParlayMatches: "4"}, prompt.IsInteractive)
	if err != nil {
		t.Fatal(err)
	}
	if in.betMode != BetModeParlay || in.maxParlayMatches != 4 {
		t.Fatalf("got mode=%q maxParlay=%d", in.betMode, in.maxParlayMatches)
	}
}

func TestResolveBacktestInputs_parlayFlagWithoutMaxNonInteractive(t *testing.T) {
	t.Parallel()

	r, w, err := osPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	_, err = resolveBacktestInputs(r, backtestInputFlags{parlay: true}, prompt.IsInteractive)
	if !errors.Is(err, errInvalidMaxParlayMatches) {
		t.Fatalf("got err %v, want errInvalidMaxParlayMatches", err)
	}
}

func TestResolveBacktestInputs_interactiveParlay(t *testing.T) {
	t.Parallel()

	r, w, err := osPipe()
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		_, _ = w.WriteString("\n\n\n\n\n") // stake, min-pick, sims, min-odds, require-ev defaults
		_, _ = w.WriteString("parlay\n")
		_, _ = w.WriteString("2\n")
		w.Close()
	}()

	in, err := resolveBacktestInputs(r, backtestInputFlags{}, func(*os.File) bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	if in.betMode != BetModeParlay || in.maxParlayMatches != 2 {
		t.Fatalf("got mode=%q maxParlay=%d", in.betMode, in.maxParlayMatches)
	}
}

func TestResolveBacktestInputs_interactiveParlayDefaultMaxLegs(t *testing.T) {
	t.Parallel()

	r, w, err := osPipe()
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		_, _ = w.WriteString("\n\n\n\n\n")
		_, _ = w.WriteString("p\n")
		_, _ = w.WriteString("\n") // max legs default
		w.Close()
	}()

	in, err := resolveBacktestInputs(r, backtestInputFlags{}, func(*os.File) bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	if in.betMode != BetModeParlay || in.maxParlayMatches != defaultMaxParlayMatches {
		t.Fatalf("got mode=%q maxParlay=%d", in.betMode, in.maxParlayMatches)
	}
}

func TestResolveBetMode_invalid(t *testing.T) {
	t.Parallel()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		_, _ = w.WriteString("combo\n")
		w.Close()
	}()

	_, err = resolveBetMode(false, "", bufio.NewReader(r))
	if !errors.Is(err, errInvalidBetMode) {
		t.Fatalf("got err %v, want errInvalidBetMode", err)
	}
}

func TestResolveMaxParlayMatches_invalid(t *testing.T) {
	t.Parallel()

	_, err := resolveMaxParlayMatches(BetModeParlay, "0", nil)
	if !errors.Is(err, errInvalidMaxParlayMatches) {
		t.Fatalf("got err %v, want errInvalidMaxParlayMatches", err)
	}
}

// osPipe returns a pipe reader (non-TTY) and writer for simulated stdin.
func osPipe() (*os.File, *os.File, error) {
	r, w, err := os.Pipe()
	return r, w, err
}
