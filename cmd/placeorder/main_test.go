package main

import (
	"errors"
	"os"
	"testing"

	"github.com/AndochBonin/polymarket/internal/prompt"
)

func TestResolveInputs_bothProvided(t *testing.T) {
	t.Parallel()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	tokenID, priceStr, err := resolveInputs(r, []string{"tok123"}, "0.50", prompt.IsInteractive)
	if err != nil {
		t.Fatalf("resolveInputs: %v", err)
	}
	if tokenID != "tok123" || priceStr != "0.50" {
		t.Fatalf("got tokenID=%q priceStr=%q", tokenID, priceStr)
	}
}

func TestResolveInputs_priceFlagOnly(t *testing.T) {
	t.Parallel()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	_, _, err = resolveInputs(r, nil, "0.25", prompt.IsInteractive)
	if !errors.Is(err, errUsage) {
		t.Fatalf("got err %v want errUsage", err)
	}
}

func TestResolveInputs_tokenOnly(t *testing.T) {
	t.Parallel()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	_, _, err = resolveInputs(r, []string{"tok"}, "", prompt.IsInteractive)
	if !errors.Is(err, errUsage) {
		t.Fatalf("got err %v want errUsage", err)
	}
}

func TestResolveInputs_tooManyArgs(t *testing.T) {
	t.Parallel()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	_, _, err = resolveInputs(r, []string{"a", "b"}, "0.5", prompt.IsInteractive)
	if !errors.Is(err, errUsage) {
		t.Fatalf("got err %v want errUsage", err)
	}
}

func TestResolveInputs_interactivePrompts(t *testing.T) {
	t.Parallel()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close(); w.Close() })

	if _, err := w.WriteString("0.42\nabc-token\n"); err != nil {
		t.Fatal(err)
	}

	tokenID, priceStr, err := resolveInputs(r, nil, "", func(*os.File) bool { return true })
	if err != nil {
		t.Fatalf("resolveInputs: %v", err)
	}
	if priceStr != "0.42" || tokenID != "abc-token" {
		t.Fatalf("got tokenID=%q priceStr=%q", tokenID, priceStr)
	}
}

func TestResolveInputs_trimsInputs(t *testing.T) {
	t.Parallel()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	tokenID, priceStr, err := resolveInputs(r, []string{"  tok  "}, "  0.99  ", prompt.IsInteractive)
	if err != nil {
		t.Fatalf("resolveInputs: %v", err)
	}
	if tokenID != "tok" || priceStr != "0.99" {
		t.Fatalf("got tokenID=%q priceStr=%q", tokenID, priceStr)
	}
}

func TestResolveInputs_emptyTokenArg(t *testing.T) {
	t.Parallel()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	_, _, err = resolveInputs(r, []string{"   "}, "0.5", prompt.IsInteractive)
	if !errors.Is(err, errUsage) {
		t.Fatalf("got err %v want errUsage", err)
	}
}

func TestResolveInputs_interactivePriceOnly(t *testing.T) {
	t.Parallel()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close(); w.Close() })

	if _, err := w.WriteString("0.33\n"); err != nil {
		t.Fatal(err)
	}

	tokenID, priceStr, err := resolveInputs(r, []string{"existing-tok"}, "", func(*os.File) bool { return true })
	if err != nil {
		t.Fatalf("resolveInputs: %v", err)
	}
	if tokenID != "existing-tok" || priceStr != "0.33" {
		t.Fatalf("got tokenID=%q priceStr=%q", tokenID, priceStr)
	}
}

func TestResolveInputs_interactiveStillMissingAfterPrompt(t *testing.T) {
	t.Parallel()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close(); w.Close() })

	if _, err := w.WriteString("\n\n"); err != nil {
		t.Fatal(err)
	}

	_, _, err = resolveInputs(r, nil, "", func(*os.File) bool { return true })
	if !errors.Is(err, errUsage) {
		t.Fatalf("got err %v want errUsage", err)
	}
}
