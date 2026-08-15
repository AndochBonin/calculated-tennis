# Calculated Tennis

Tennis simulations and Polymarket gambling based on *Calculated Bets* by Steven Skiena and [Tennis Abstract](https://www.tennisabstract.com) by Jeff Sackmann.

## Tennis simulation TUI

The tennis match simulator ships as a terminal UI (built with [Bubble Tea](https://github.com/charmbracelet/bubbletea)). Run it with `go run ./cmd/tui` from the `tennis/` module, or `make -C tennis tui`. Step through the form (players, format, surface, sims); it fetches hold/break rates and runs the Monte Carlo projection. The chosen court surface sets the alpha (essentially the weight of serve dominance on the outcome) and sets the theme — **hard** (blue), **clay** (terracotta), or **grass** (green) — recoloring the interface to match.
