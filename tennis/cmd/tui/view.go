package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/AndochBonin/calculated-tennis/tennis/tennis"
	"github.com/charmbracelet/lipgloss"
)

const winBarWidth = 30

var warnStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true)

func (m model) View() string {
	content := m.stateView()
	if m.width == 0 || m.height == 0 {
		return content // pre-size / non-tty fallback
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m model) stateView() string {
	switch m.state {
	case stateLoading:
		return m.viewLoading()
	case stateResults:
		return m.viewResults()
	case stateError:
		return m.viewError()
	default:
		return m.viewForm()
	}
}

func (m model) header() string {
	t := m.theme
	return t.titleStyle().Render("🎾 Calculated Tennis") +
		t.labelStyle().Render("  "+t.Name+" court")
}

func (m model) panel(content string) string {
	return m.theme.panelStyle().Render(content)
}

func (m model) viewForm() string {
	t := m.theme
	var b strings.Builder
	b.WriteString(m.header())
	b.WriteString("\n\n")
	b.WriteString(t.accentStyle().Render(fmt.Sprintf("Step %d/%d", int(m.step)+1, int(stepDone))))
	b.WriteString("  ")
	b.WriteString(t.valueStyle().Render(stepTitle(m.step)))
	b.WriteString("\n\n")

	if m.isChoiceStep() {
		for i, label := range m.choiceLabels() {
			if i == m.cursor {
				b.WriteString(t.accentStyle().Render("▸ ") + t.valueStyle().Render(label) + "\n")
			} else {
				b.WriteString("  " + t.labelStyle().Render(label) + "\n")
			}
		}
	} else {
		b.WriteString(m.ti.View())
		b.WriteString("\n")
	}

	if m.errMsg != "" {
		b.WriteString("\n" + warnStyle.Render("⚠ "+m.errMsg) + "\n")
	}
	b.WriteString("\n")
	b.WriteString(t.helpStyle().Render(m.formHelp()))
	return m.panel(b.String())
}

func (m model) formHelp() string {
	if m.isChoiceStep() {
		return "↑/↓ move • enter select • esc back • ctrl+c quit"
	}
	return "enter continue • esc back • ctrl+c quit"
}

func (m model) viewLoading() string {
	t := m.theme
	content := m.header() + "\n\n" +
		m.spinner.View() + " " +
		t.valueStyle().Render("Fetching rates & simulating…") + "\n\n" +
		t.helpStyle().Render(fmt.Sprintf("%s vs %s on %s", m.playerA, m.playerB, m.surface))
	return m.panel(content)
}

func (m model) kv(label, value string) string {
	t := m.theme
	return t.labelStyle().Width(15).Render(label) + t.valueStyle().Render(value) + "\n"
}

func (m model) viewResults() string {
	t := m.theme
	var b strings.Builder
	b.WriteString(m.header())
	b.WriteString("\n\n")
	b.WriteString(t.accentStyle().Render("Match Projection"))
	b.WriteString("\n\n")

	b.WriteString(m.kv("Player A", fmt.Sprintf("%s  (hold %s, break %s)", m.playerA, pct(m.rates[0].HoldPct), pct(m.rates[0].BreakPct))))
	b.WriteString(m.kv("Player B", fmt.Sprintf("%s  (hold %s, break %s)", m.playerB, pct(m.rates[1].HoldPct), pct(m.rates[1].BreakPct))))
	b.WriteString(m.kv("Format", m.formatLabel))
	b.WriteString(m.kv("Surface", string(m.surface)))
	if m.score != "" {
		b.WriteString(m.kv("Score", m.score))
	}
	switch {
	case m.useCoinToss:
		b.WriteString(m.kv("First server", "coin toss per simulation"))
	case m.firstServerChosen:
		server := m.playerA
		if m.firstServer == tennis.B {
			server = m.playerB
		}
		b.WriteString(m.kv("First server", server))
	}
	b.WriteString(m.kv("Alpha", strconv.FormatFloat(m.alpha, 'g', -1, 64)))
	b.WriteString(m.kv("Sims", strconv.Itoa(m.sims)))

	b.WriteString("\n")
	b.WriteString(t.accentStyle().Render("Results"))
	b.WriteString("\n")
	names := [2]string{m.playerA, m.playerB}
	colors := [2]lipgloss.Color{t.WinA, t.WinB}
	for i, p := range []tennis.Player{tennis.A, tennis.B} {
		wins := m.result.WinCount(p)
		frac := 0.0
		if m.sims > 0 {
			frac = float64(wins) / float64(m.sims)
		}
		b.WriteString(m.winRow(names[i], wins, frac, colors[i]))
	}

	b.WriteString("\n")
	b.WriteString(t.helpStyle().Render("r run again • q quit"))
	return m.panel(b.String())
}

func (m model) winRow(name string, wins int, frac float64, c lipgloss.Color) string {
	filled := int(frac*winBarWidth + 0.5)
	if filled > winBarWidth {
		filled = winBarWidth
	}
	bar := lipgloss.NewStyle().Foreground(c).Render(strings.Repeat("█", filled)) +
		m.theme.labelStyle().Render(strings.Repeat("░", winBarWidth-filled))
	label := m.theme.valueStyle().Render(fmt.Sprintf("%-18s", trunc(name, 18)))
	stat := lipgloss.NewStyle().Foreground(c).Bold(true).Render(fmt.Sprintf(" %d (%.1f%%)", wins, 100*frac))
	return label + " " + bar + stat + "\n"
}

func (m model) viewError() string {
	t := m.theme
	msg := "unknown error"
	if m.err != nil {
		msg = m.err.Error()
	}
	content := m.header() + "\n\n" +
		warnStyle.Render("Something went wrong") + "\n\n" +
		t.valueStyle().Render(msg) + "\n\n" +
		t.helpStyle().Render("r edit inputs • q quit")
	return m.panel(content)
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}
