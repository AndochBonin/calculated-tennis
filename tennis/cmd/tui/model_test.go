package main

import (
	"context"
	"testing"

	"github.com/AndochBonin/calculated-tennis/tennis/tennis"
	tea "github.com/charmbracelet/bubbletea"
)

// send applies one message to the model and returns the updated model plus the command.
func send(t *testing.T, m model, msg tea.Msg) (model, tea.Cmd) {
	t.Helper()
	next, cmd := m.Update(msg)
	return next.(model), cmd
}

// typeInto types text into the focused field without submitting.
func typeInto(t *testing.T, m model, s string) model {
	t.Helper()
	m, _ = send(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
	return m
}

func enter(t *testing.T, m model) model {
	t.Helper()
	m, _ = send(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	return m
}

func tab(t *testing.T, m model) model {
	t.Helper()
	m, _ = send(t, m, tea.KeyMsg{Type: tea.KeyTab})
	return m
}

func down(t *testing.T, m model) model {
	t.Helper()
	m, _ = send(t, m, tea.KeyMsg{Type: tea.KeyDown})
	return m
}

// TestFormToResults walks the whole page flow: pick Grass on the surface page
// (which must swap the theme), fill players+format, alpha+sims, skip the score
// (coin toss), then push it through the rates + simulate pipeline and assert the
// results state is coherent.
func TestFormToResults(t *testing.T) {
	m := initialModel(context.Background(), nil)

	// Page 1: surface — Hard -> Clay -> Grass.
	m = down(t, m)
	m = down(t, m)
	if got := surfaceLabels[m.surfaceIdx]; got != "Grass" {
		t.Fatalf("surface cursor on %q, want Grass", got)
	}
	m = enter(t, m) // select Grass, advance
	if m.theme.Name != "Grass" {
		t.Fatalf("theme = %q after selecting Grass, want Grass", m.theme.Name)
	}
	if m.page != pagePlayers {
		t.Fatalf("page = %d after surface, want pagePlayers (%d)", m.page, pagePlayers)
	}

	// Page 2: players + format (default Best of 3).
	m = typeInto(t, m, "Jannik Sinner")
	m = tab(t, m)
	m = typeInto(t, m, "Carlos Alcaraz")
	m = enter(t, m)
	if m.playerA != "Jannik Sinner" || m.playerB != "Carlos Alcaraz" {
		t.Fatalf("players = %q / %q", m.playerA, m.playerB)
	}
	if m.page != pageMetrics {
		t.Fatalf("page = %d after players, want pageMetrics (%d)", m.page, pageMetrics)
	}

	// Page 3: number of simulations (alpha is surface-derived, not entered).
	m = typeInto(t, m, "1000")
	m = enter(t, m) // -> startSim

	if m.sims != 1000 {
		t.Fatalf("sims = %d, want 1000", m.sims)
	}
	if m.state != stateLoading {
		t.Fatalf("state = %d after form, want stateLoading (%d)", m.state, stateLoading)
	}
	// Alpha comes from tennisabstract.AlphaFromEnv; with no env set it is the
	// code default.
	if m.alpha != 1.0 {
		t.Fatalf("alpha = %v, want surface-derived default 1.0", m.alpha)
	}

	// Inject fetched rates, then run the real (pure) simulate command it returns.
	rates := [2]tennis.PlayerRates{{HoldPct: 0.82, BreakPct: 0.22}, {HoldPct: 0.80, BreakPct: 0.20}}
	m, cmd := send(t, m, ratesMsg{rates})
	if cmd == nil {
		t.Fatal("expected a simulate command after ratesMsg")
	}
	msg := cmd()
	res, ok := msg.(resultMsg)
	if !ok {
		t.Fatalf("simulate produced %T, want resultMsg (err: %+v)", msg, msg)
	}
	m, _ = send(t, m, res)

	if m.state != stateResults {
		t.Fatalf("state = %d after result, want stateResults (%d)", m.state, stateResults)
	}
	total := m.result.WinCount(tennis.A) + m.result.WinCount(tennis.B)
	if total != m.sims {
		t.Fatalf("wins sum to %d, want sims=%d", total, m.sims)
	}
}

// TestPlayerNamesAutocomplete verifies the embedded supported-names list parses
// and populates type-ahead suggestions on both player-name inputs.
func TestPlayerNamesAutocomplete(t *testing.T) {
	m := initialModel(context.Background(), nil)

	msg := loadPlayerNamesCmd()()
	names, ok := msg.(playerNamesMsg)
	if !ok {
		t.Fatalf("loadPlayerNamesCmd produced %T, want playerNamesMsg", msg)
	}
	if len(names.names) == 0 {
		t.Fatal("expected embedded player names, got none")
	}

	m, _ = send(t, m, names)
	if !m.namesLoaded {
		t.Fatal("namesLoaded should be true after playerNamesMsg")
	}
	if got := len(m.inPlayerA.AvailableSuggestions()); got != len(names.names) {
		t.Fatalf("player A suggestions = %d, want %d", got, len(names.names))
	}
	if len(m.inPlayerB.AvailableSuggestions()) == 0 {
		t.Fatal("player B suggestions were not set")
	}
}

