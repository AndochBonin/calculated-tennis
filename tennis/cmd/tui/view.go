package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/AndochBonin/calculated-tennis/tennis/tennis"
	"github.com/charmbracelet/bubbles/textinput"
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
	b.WriteString(t.accentStyle().Render(fmt.Sprintf("Step %d/%d", int(m.page)+1, int(pageDone))))
	b.WriteString("  ")
	b.WriteString(t.valueStyle().Render(pageTitle(m.page)))
	b.WriteString("\n\n")

	b.WriteString(m.pageBody())

	if m.errMsg != "" {
		b.WriteString("\n" + warnStyle.Render("⚠ "+m.errMsg) + "\n")
	}
	b.WriteString("\n")
	b.WriteString(t.helpStyle().Render(m.formHelp()))
	return m.panel(b.String())
}

// pageBody renders the fields for the current page.
func (m model) pageBody() string {
	switch m.page {
	case pageSurface:
		return m.viewSurfacePage()
	case pagePlayers:
		var b strings.Builder
		b.WriteString(m.inputRow("Player A", m.inPlayerA, m.focus == 0))
		b.WriteString(m.inputRow("Player B", m.inPlayerB, m.focus == 1))
		b.WriteString(m.selectorRow("Format", formatLabels(), m.formatIdx, m.focus == 2))
		return b.String()
	case pageMetrics:
		return m.inputRow("Simulations", m.inSims, m.focus == 0)
	}
	return ""
}

func (m model) viewSurfacePage() string {
	t := m.theme
	var b strings.Builder
	for i, label := range surfaceLabels {
		if i == m.surfaceIdx {
			b.WriteString(t.accentStyle().Render("▸ ") + t.valueStyle().Render(label) + "\n")
		} else {
			b.WriteString("  " + t.labelStyle().Render(label) + "\n")
		}
	}
	return b.String()
}

func formatLabels() []string {
	fc := formatChoices()
	out := make([]string, 0, len(fc))
	for _, c := range fc {
		out = append(out, c.label)
	}
	return out
}

// inputRow renders a labeled text input; the focused row gets an accent marker.
func (m model) inputRow(label string, ti textinput.Model, focused bool) string {
	t := m.theme
	marker := "  "
	labelStyle := t.labelStyle()
	if focused {
		marker = t.accentStyle().Render("▸ ")
		labelStyle = t.accentStyle()
	}
	return marker + labelStyle.Render(label) + "\n  " + ti.View() + "\n\n"
}

// selectorRow renders a labeled ◄ value ► selector; accent when focused.
func (m model) selectorRow(label string, opts []string, idx int, focused bool) string {
	t := m.theme
	if idx < 0 || idx >= len(opts) {
		idx = 0
	}
	val := ""
	if len(opts) > 0 {
		val = opts[idx]
	}
	marker := "  "
	labelStyle := t.labelStyle()
	valStyle := t.valueStyle()
	if focused {
		marker = t.accentStyle().Render("▸ ")
		labelStyle = t.accentStyle()
		val = "◄ " + val + " ►"
		valStyle = t.accentStyle()
	}
	return marker + labelStyle.Render(label) + "\n  " + valStyle.Render(val) + "\n\n"
}

func (m model) formHelp() string {
	switch m.page {
	case pageSurface:
		return "↑/↓ move • enter select • ctrl+c quit"
	case pagePlayers:
		return "tab move field • → accept name • ←/→ format • enter continue • esc back • ctrl+c quit"
	default: // metrics
		return "enter run • esc back • ctrl+c quit"
	}
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
	b.WriteString(m.kv("First server", "coin toss per simulation"))
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
