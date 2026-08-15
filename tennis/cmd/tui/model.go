package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/AndochBonin/calculated-tennis/tennis/tennis"
	"github.com/AndochBonin/calculated-tennis/tennis/tennisabstract"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type state int

const (
	stateForm state = iota
	stateLoading
	stateResults
	stateError
)

// page enumerates the grouped form screens in order. pageDone marks completion
// and doubles as the total page count for the progress indicator.
type page int

const (
	pageSurface page = iota // Hard / Clay / Grass
	pagePlayers             // player A + player B + format
	pageMetrics             // number of simulations
	pageDone
)

type labeledFormat struct {
	label  string
	format tennis.MatchFormat
}

func formatChoices() []labeledFormat {
	return []labeledFormat{
		{"Best of 3", tennis.DefaultFormat()},
		{"Best of 5 (grand slam)", tennis.GrandSlamMenFormat()},
	}
}

var surfaceChoices = []tennisabstract.MatchSurface{
	tennisabstract.SurfaceHard,
	tennisabstract.SurfaceClay,
	tennisabstract.SurfaceGrass,
}

var surfaceLabels = []string{"Hard", "Clay", "Grass"}

type model struct {
	client *tennisabstract.Client
	ctx    context.Context

	state state
	page  page
	focus int // focused field within the current page
	theme Theme

	width  int // terminal size, from tea.WindowSizeMsg; centers the view
	height int

	// text inputs, one per text field, persistent across back/forward.
	inPlayerA textinput.Model
	inPlayerB textinput.Model
	inSims    textinput.Model

	spinner spinner.Model

	// selector indices
	surfaceIdx int
	formatIdx  int

	namesLoaded bool // supported player names fetched into the name inputs

	// collected/resolved inputs (populated on each page's submit)
	playerA     string
	playerB     string
	format      tennis.MatchFormat
	formatLabel string
	surface     tennisabstract.MatchSurface
	alpha       float64 // surface-derived, from tennisabstract.AlphaFromEnv
	sims        int

	// results
	rates  [2]tennis.PlayerRates
	result tennis.SimulationResult

	err    error
	errMsg string // inline validation message on the form
}

func newInput(placeholder string) textinput.Model {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.CharLimit = 120
	ti.Placeholder = placeholder
	return ti
}

// newNameInput is a text input with type-ahead suggestions enabled, for the
// player-name fields. Suggestions are populated once the supported-names list
// loads (see loadPlayerNamesCmd). AcceptSuggestion is rebound to → (right)
// because the default (tab) is consumed by our field navigation; → accepts the
// ghost completion when at end-of-input and otherwise moves the cursor. Cycle
// suggestions with ctrl+n / ctrl+p.
func newNameInput(placeholder string) textinput.Model {
	ti := newInput(placeholder)
	ti.ShowSuggestions = true
	ti.KeyMap.AcceptSuggestion = key.NewBinding(key.WithKeys("right"))
	return ti
}

func initialModel(ctx context.Context, client *tennisabstract.Client) model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(hardTheme.Accent)

	m := model{
		client:     client,
		ctx:        ctx,
		state:      stateForm,
		page:       pageSurface,
		theme:      hardTheme,
		inPlayerA:  newNameInput("e.g. Jannik Sinner"),
		inPlayerB:  newNameInput("e.g. Carlos Alcaraz"),
		inSims:     newInput("e.g. 10000"),
		spinner:    sp,
		surfaceIdx: 0,
		formatIdx:  0,
	}
	m.syncFocus()
	return m
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		m.width, m.height = ws.Width, ws.Height
		return m, nil
	}
	switch m.state {
	case stateForm:
		return m.updateForm(msg)
	case stateLoading:
		return m.updateLoading(msg)
	default: // results / error
		return m.updateDone(msg)
	}
}

// --- form navigation helpers ---

// pageInputs returns the focusable text inputs for the current page, in order.
// Selector fields (format) are not text inputs and are handled separately; nil
// entries mark a selector's focus slot.
func (m *model) pageInputs() []*textinput.Model {
	switch m.page {
	case pagePlayers:
		return []*textinput.Model{&m.inPlayerA, &m.inPlayerB, nil} // nil = format selector
	case pageMetrics:
		return []*textinput.Model{&m.inSims}
	}
	return nil // surface page is a single vertical list
}

// focusCount is the number of focusable fields on the current page.
func (m *model) focusCount() int {
	switch m.page {
	case pagePlayers:
		return 3
	case pageMetrics:
		return 1
	default: // surface
		return 1
	}
}

// focusedIsSelector reports whether the focused field is a selector (not a text
// input) on the current page.
func (m *model) focusedIsSelector() bool {
	if m.page == pagePlayers {
		return m.focus == 2 // format
	}
	return false
}

// syncFocus focuses the active text input and blurs the rest.
func (m *model) syncFocus() {
	all := []*textinput.Model{&m.inPlayerA, &m.inPlayerB, &m.inSims}
	for _, ti := range all {
		ti.Blur()
	}
	inputs := m.pageInputs()
	if m.focus >= 0 && m.focus < len(inputs) && inputs[m.focus] != nil {
		inputs[m.focus].Focus()
	}
}

func (m model) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	if names, ok := msg.(playerNamesMsg); ok {
		m.namesLoaded = true
		m.inPlayerA.SetSuggestions(names.names)
		m.inPlayerB.SetSuggestions(names.names)
		return m, nil
	}
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m.forwardToFocused(msg)
	}
	if key.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}
	switch key.Type {
	case tea.KeyEnter:
		return m.submitPage()
	case tea.KeyEsc:
		return m.prevPage()
	}

	if m.page == pageSurface {
		return m.updateSurface(key)
	}

	// Multi-field pages: tab/shift+tab and up/down move focus between fields.
	switch key.String() {
	case "tab", "down":
		m.focus = (m.focus + 1) % m.focusCount()
		m.errMsg = ""
		m.syncFocus()
		return m, textinput.Blink
	case "shift+tab", "up":
		m.focus = (m.focus - 1 + m.focusCount()) % m.focusCount()
		m.errMsg = ""
		m.syncFocus()
		return m, textinput.Blink
	case "left", "right":
		if m.focusedIsSelector() {
			return m.changeSelector(key.String() == "right"), nil
		}
	}
	return m.forwardToFocused(key)
}

func (m model) updateSurface(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "up", "k":
		if m.surfaceIdx > 0 {
			m.surfaceIdx--
		}
	case "down", "j":
		if m.surfaceIdx < len(surfaceChoices)-1 {
			m.surfaceIdx++
		}
	}
	// Live-recolor the whole UI to the highlighted surface.
	m.surface = surfaceChoices[m.surfaceIdx]
	m.theme = themeForSurface(m.surface)
	m.spinner.Style = lipgloss.NewStyle().Foreground(m.theme.Accent)
	return m, nil
}

// changeSelector advances (or retreats) the focused selector's index.
func (m model) changeSelector(forward bool) model {
	if m.page != pagePlayers {
		return m
	}
	n := len(formatChoices())
	if forward {
		m.formatIdx = (m.formatIdx + 1) % n
	} else {
		m.formatIdx = (m.formatIdx - 1 + n) % n
	}
	return m
}

// forwardToFocused passes a message to the focused text input (if any).
func (m model) forwardToFocused(msg tea.Msg) (tea.Model, tea.Cmd) {
	inputs := m.pageInputs()
	if m.focus < 0 || m.focus >= len(inputs) || inputs[m.focus] == nil {
		return m, nil
	}
	var cmd tea.Cmd
	*inputs[m.focus], cmd = inputs[m.focus].Update(msg)
	return m, cmd
}

func (m model) submitPage() (tea.Model, tea.Cmd) {
	switch m.page {
	case pageSurface:
		m.surface = surfaceChoices[m.surfaceIdx]
		m.theme = themeForSurface(m.surface)
		m.spinner.Style = lipgloss.NewStyle().Foreground(m.theme.Accent)
		// Fetch supported names just before the player-selection page.
		next, cmd := m.nextPage()
		if !m.namesLoaded {
			return next, tea.Batch(cmd, loadPlayerNamesCmd())
		}
		return next, cmd
	case pagePlayers:
		if strings.TrimSpace(m.inPlayerA.Value()) == "" || strings.TrimSpace(m.inPlayerB.Value()) == "" {
			m.errMsg = "player name is required"
			return m, nil
		}
		m.playerA = strings.TrimSpace(m.inPlayerA.Value())
		m.playerB = strings.TrimSpace(m.inPlayerB.Value())
		fc := formatChoices()[m.formatIdx]
		m.format, m.formatLabel = fc.format, fc.label
	case pageMetrics:
		n, err := strconv.Atoi(strings.TrimSpace(m.inSims.Value()))
		if err != nil || n <= 0 {
			m.errMsg = "sims must be a whole number > 0"
			return m, nil
		}
		m.sims = n
		return m.startSim()
	}
	return m.nextPage()
}

func (m model) nextPage() (tea.Model, tea.Cmd) {
	m.page++
	m.focus = 0
	m.errMsg = ""
	m.syncFocus()
	return m, textinput.Blink
}

func (m model) prevPage() (tea.Model, tea.Cmd) {
	if m.page == pageSurface {
		return m, nil
	}
	m.page--
	m.focus = 0
	m.errMsg = ""
	m.syncFocus()
	return m, textinput.Blink
}

func (m model) startSim() (tea.Model, tea.Cmd) {
	// Alpha is surface-derived (env override or code default), not user input.
	m.alpha = tennisabstract.AlphaFromEnv(m.surface)
	m.state = stateLoading
	names := [2]string{m.playerA, m.playerB}
	return m, tea.Batch(m.spinner.Tick, fetchRatesCmd(m.ctx, m.client, names, m.surface))
}

func (m model) simInputs() simInputs {
	// Matches always simulate fresh from 0-0 with a coin-toss serve each run;
	// score and explicit first server are no longer collected in the TUI.
	return simInputs{
		format:  m.format,
		surface: m.surface,
		alpha:   m.alpha,
		sims:    m.sims,
	}
}

// --- loading ---

func (m model) updateLoading(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case ratesMsg:
		m.rates = msg.rates
		return m, runSimCmd(m.simInputs(), m.rates)
	case resultMsg:
		m.result = msg.result
		m.state = stateResults
		return m, nil
	case errMsg:
		m.err = msg.err
		m.state = stateError
		return m, nil
	}
	return m, nil
}

// --- results / error ---

func (m model) updateDone(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "q", "ctrl+c", "esc":
		return m, tea.Quit
	case "r":
		return m.restart()
	}
	return m, nil
}

func (m model) restart() (tea.Model, tea.Cmd) {
	m.state = stateForm
	m.page = pageSurface
	m.focus = 0
	m.err = nil
	m.errMsg = ""
	m.result = tennis.SimulationResult{}
	m.syncFocus()
	return m, textinput.Blink
}

// --- metadata ---

func pageTitle(p page) string {
	switch p {
	case pageSurface:
		return "Court surface"
	case pagePlayers:
		return "Players & format"
	case pageMetrics:
		return "Number of simulations"
	}
	return ""
}

func pct(v float64) string {
	return fmt.Sprintf("%.1f%%", 100*v)
}
