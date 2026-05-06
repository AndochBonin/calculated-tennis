package main

import (
	"flag"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/AndochBonin/polymarket/core"
)

func TestMain(m *testing.M) {
	origCommandLine := flag.CommandLine
	origArgs := os.Args
	origVerboseIDs := core.VerboseIDs

	isolated := flag.NewFlagSet("main-test", flag.ContinueOnError)
	isolated.SetOutput(io.Discard)
	flag.CommandLine = isolated
	os.Args = []string{"main-test", "-v"}
	core.VerboseIDs = false

	parseVerboseFlag()
	if !core.VerboseIDs {
		flag.CommandLine = origCommandLine
		os.Args = origArgs
		core.VerboseIDs = origVerboseIDs
		os.Exit(1)
	}

	// Keep other tests isolated from global flag and verbose state changes.
	flag.CommandLine = origCommandLine
	os.Args = origArgs
	core.VerboseIDs = origVerboseIDs

	os.Exit(m.Run())
}

func TestLogLevelFromEnv(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     slog.Level
	}{
		{name: "debug", envValue: "debug", want: slog.LevelDebug},
		{name: "info", envValue: "info", want: slog.LevelInfo},
		{name: "warn", envValue: "warn", want: slog.LevelWarn},
		{name: "warning", envValue: "warning", want: slog.LevelWarn},
		{name: "error", envValue: "error", want: slog.LevelError},
		{name: "empty", envValue: "", want: slog.LevelInfo},
		{name: "invalid", envValue: "nope", want: slog.LevelInfo},
		{name: "trim spaces", envValue: "  DEBUG ", want: slog.LevelDebug},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("LOG_LEVEL", tc.envValue)

			got := logLevelFromEnv()
			if got != tc.want {
				t.Fatalf("logLevelFromEnv() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSetupLoggingWarnsOnInvalidLogLevel(t *testing.T) {
	t.Setenv("LOG_LEVEL", "invalid-level")

	origLogger := slog.Default()
	t.Cleanup(func() {
		slog.SetDefault(origLogger)
	})

	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() failed: %v", err)
	}
	os.Stderr = w
	t.Cleanup(func() {
		os.Stderr = origStderr
		_ = r.Close()
		_ = w.Close()
	})

	setupLogging()

	_ = w.Close()

	done := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()

	select {
	case out := <-done:
		if !strings.Contains(out, "invalid LOG_LEVEL, defaulting to info") {
			t.Fatalf("expected warning in output, got: %q", out)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for logger output")
	}
}
