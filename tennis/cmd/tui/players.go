package main

import (
	_ "embed"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// playerNames is the list of supported player display names (2025 ATP tour),
// generated from tennisabstract/testdata/atp_matches_2025.csv via
// UniquePlayerNamesFromMatchesCSV. Embedded so name autocomplete works from any
// working directory.
//
//go:embed players.txt
var playersRaw string

// playerNamesMsg carries the loaded supported names back to the Update loop.
type playerNamesMsg struct{ names []string }

// loadPlayerNamesCmd parses the embedded name list off the UI goroutine. It runs
// when leaving the surface page, just before the player-selection screen.
func loadPlayerNamesCmd() tea.Cmd {
	return func() tea.Msg {
		lines := strings.Split(strings.TrimSpace(playersRaw), "\n")
		names := make([]string, 0, len(lines))
		for _, l := range lines {
			if l = strings.TrimSpace(l); l != "" {
				names = append(names, l)
			}
		}
		return playerNamesMsg{names}
	}
}
