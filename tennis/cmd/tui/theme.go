package main

import (
	"github.com/AndochBonin/calculated-tennis/tennis/tennisabstract"
	"github.com/charmbracelet/lipgloss"
)

// Theme is a court-surface-derived color palette. Selecting the surface in the
// form swaps the active theme, so the whole UI recolors to match the court.
type Theme struct {
	Name    string
	Accent  lipgloss.Color // borders, selection, emphasis
	Title   lipgloss.Color // title-bar background
	OnTitle lipgloss.Color // title-bar foreground
	Text    lipgloss.Color // body values
	Muted   lipgloss.Color // labels, help, empty bar
	WinA    lipgloss.Color // player A bar
	WinB    lipgloss.Color // player B bar
}

// Hard = cool blue, Clay = terracotta/orange, Grass = green.
var (
	hardTheme = Theme{
		Name:    "Hard",
		Accent:  lipgloss.Color("39"),
		Title:   lipgloss.Color("25"),
		OnTitle: lipgloss.Color("231"),
		Text:    lipgloss.Color("252"),
		Muted:   lipgloss.Color("245"),
		WinA:    lipgloss.Color("81"),
		WinB:    lipgloss.Color("208"),
	}
	clayTheme = Theme{
		Name:    "Clay",
		Accent:  lipgloss.Color("166"),
		Title:   lipgloss.Color("130"),
		OnTitle: lipgloss.Color("231"),
		Text:    lipgloss.Color("223"),
		Muted:   lipgloss.Color("180"),
		WinA:    lipgloss.Color("214"),
		WinB:    lipgloss.Color("172"),
	}
	grassTheme = Theme{
		Name:    "Grass",
		Accent:  lipgloss.Color("34"),
		Title:   lipgloss.Color("22"),
		OnTitle: lipgloss.Color("231"),
		Text:    lipgloss.Color("194"),
		Muted:   lipgloss.Color("108"),
		WinA:    lipgloss.Color("120"),
		WinB:    lipgloss.Color("228"),
	}
)

// themeForSurface maps a court surface to its palette; unknown surfaces use Hard.
func themeForSurface(s tennisabstract.MatchSurface) Theme {
	switch s {
	case tennisabstract.SurfaceClay:
		return clayTheme
	case tennisabstract.SurfaceGrass:
		return grassTheme
	default:
		return hardTheme
	}
}

func (t Theme) titleStyle() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Padding(0, 1).
		Background(t.Title).Foreground(t.OnTitle)
}

func (t Theme) panelStyle() lipgloss.Style {
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Accent).Padding(1, 2)
}

func (t Theme) labelStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(t.Muted)
}

func (t Theme) valueStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(t.Text).Bold(true)
}

func (t Theme) accentStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(t.Accent).Bold(true)
}

func (t Theme) helpStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(t.Muted).Italic(true)
}
