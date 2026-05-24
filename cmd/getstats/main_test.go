package main

import (
	"errors"
	"os"
	"testing"

	"github.com/AndochBonin/polymarket/internal/prompt"
)

func TestResolvePlayer_flag(t *testing.T) {
	t.Parallel()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	player, err := resolvePlayer(r, "Daniil Medvedev", nil, prompt.IsInteractive)
	if err != nil {
		t.Fatalf("resolvePlayer: %v", err)
	}
	if player != "Daniil Medvedev" {
		t.Fatalf("got %q", player)
	}
}

func TestResolvePlayer_positional(t *testing.T) {
	t.Parallel()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	player, err := resolvePlayer(r, "", []string{"jannik", "sinner"}, prompt.IsInteractive)
	if err != nil {
		t.Fatalf("resolvePlayer: %v", err)
	}
	if player != "jannik sinner" {
		t.Fatalf("got %q", player)
	}
}

func TestResolvePlayer_flagOverridesPositional(t *testing.T) {
	t.Parallel()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	player, err := resolvePlayer(r, "flag-name", []string{"positional"}, prompt.IsInteractive)
	if err != nil {
		t.Fatalf("resolvePlayer: %v", err)
	}
	if player != "flag-name" {
		t.Fatalf("got %q", player)
	}
}

func TestResolvePlayer_nonInteractiveMissing(t *testing.T) {
	t.Parallel()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	_, err = resolvePlayer(r, "", nil, prompt.IsInteractive)
	if !errors.Is(err, errUsage) {
		t.Fatalf("got err %v want errUsage", err)
	}
}

func TestResolvePlayer_interactivePrompt(t *testing.T) {
	t.Parallel()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close(); w.Close() })

	if _, err := w.WriteString("jannik sinner\n"); err != nil {
		t.Fatal(err)
	}

	player, err := resolvePlayer(r, "", nil, func(*os.File) bool { return true })
	if err != nil {
		t.Fatalf("resolvePlayer: %v", err)
	}
	if player != "jannik sinner" {
		t.Fatalf("got %q", player)
	}
}

func TestResolvePlayer_trimsInputs(t *testing.T) {
	t.Parallel()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	player, err := resolvePlayer(r, "  Daniil Medvedev  ", nil, prompt.IsInteractive)
	if err != nil {
		t.Fatalf("resolvePlayer: %v", err)
	}
	if player != "Daniil Medvedev" {
		t.Fatalf("got %q", player)
	}
}

func TestResolvePlayer_interactiveEmptyAfterPrompt(t *testing.T) {
	t.Parallel()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close(); w.Close() })

	if _, err := w.WriteString("\n"); err != nil {
		t.Fatal(err)
	}

	_, err = resolvePlayer(r, "", nil, func(*os.File) bool { return true })
	if !errors.Is(err, errUsage) {
		t.Fatalf("got err %v want errUsage", err)
	}
}
