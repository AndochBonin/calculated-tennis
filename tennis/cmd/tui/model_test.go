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

func typeText(t *testing.T, m model, s string) model {
	t.Helper()
	m, _ = send(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
	m, _ = send(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	return m
}

func enter(t *testing.T, m model) model {
	t.Helper()
	m, _ = send(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	return m
}

func down(t *testing.T, m model) model {
	t.Helper()
	m, _ = send(t, m, tea.KeyMsg{Type: tea.KeyDown})
	return m
}

// TestFormToResults walks the whole flow: fill the form, pick Grass (which must
// swap the theme), then push it through the rates + simulate pipeline and assert
// the results state is coherent.
func TestFormToResults(t *testing.T) {
	m := initialModel(context.Background(), nil)

	m = typeText(t, m, "Jannik Sinner")             // player A
	m = typeText(t, m, "Carlos Alcaraz")            // player B
	m = enter(t, m)                                 // format: default ATP
	m = down(t, m)                                  // surface: Hard -> Clay
	m = down(t, m)                                  //          Clay -> Grass
	if got := m.choiceLabels()[m.cursor]; got != "Grass" {
		t.Fatalf("surface cursor on %q, want Grass", got)
	}
	m = enter(t, m) // select Grass
	if m.theme.Name != "Grass" {
		t.Fatalf("theme = %q after selecting Grass, want Grass", m.theme.Name)
	}
	m = typeText(t, m, "2.5")  // alpha
	m = typeText(t, m, "1000") // sims
	m = enter(t, m)            // score: skip (optional)
	m = enter(t, m)            // first server: default coin toss (no score)

	if m.state != stateLoading {
		t.Fatalf("state = %d after form, want stateLoading (%d)", m.state, stateLoading)
	}
	if !m.useCoinToss {
		t.Fatal("expected coin toss when score is empty and default chosen")
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

// TestScoreRequiresFirstServer confirms that entering a score removes the coin-toss
// option, forcing an explicit first-server pick.
func TestScoreRequiresFirstServer(t *testing.T) {
	m := initialModel(context.Background(), nil)
	m = typeText(t, m, "A")
	m = typeText(t, m, "B")
	m = enter(t, m)            // format
	m = enter(t, m)            // surface: Hard
	m = typeText(t, m, "2.5")  // alpha
	m = typeText(t, m, "1000") // sims
	m = typeText(t, m, "7-5 4-6 2-3")

	if !m.isChoiceStep() || m.step != stepFirstServer {
		t.Fatalf("expected first-server choice step, got step %d", m.step)
	}
	if labels := m.choiceLabels(); len(labels) != 2 {
		t.Fatalf("first-server options = %v, want 2 (no coin toss with a score)", labels)
	}
}
