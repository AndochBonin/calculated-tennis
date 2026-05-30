package main

import (
	"os"
	"testing"

	"github.com/AndochBonin/E3/tennis/internal/prompt"
)

func TestResolveBacktestInputs_flagsOnly(t *testing.T) {
	t.Parallel()

	r, w, err := osPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	in, err := resolveBacktestInputs(r, "2.5", "0.6", "1000", prompt.IsInteractive)
	if err != nil {
		t.Fatal(err)
	}
	if in.stake != 2.5 || in.minPick != 0.6 || in.sims != 1000 {
		t.Fatalf("got stake=%v minPick=%v sims=%v", in.stake, in.minPick, in.sims)
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

	in, err := resolveBacktestInputs(r, "", "", "", prompt.IsInteractive)
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
		w.Close()
	}()

	in, err := resolveBacktestInputs(r, "", "", "", func(*os.File) bool { return true })
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
		_, _ = w.WriteString("\n\n\n")
		w.Close()
	}()

	in, err := resolveBacktestInputs(r, "", "", "", func(*os.File) bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	if in.stake != defaultStake || in.minPick != defaultMinPick || in.sims != defaultSims {
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
		w.Close()
	}()

	in, err := resolveBacktestInputs(r, "3", "", "500", func(*os.File) bool { return true })
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

// osPipe returns a pipe reader (non-TTY) and writer for simulated stdin.
func osPipe() (*os.File, *os.File, error) {
	r, w, err := os.Pipe()
	return r, w, err
}
