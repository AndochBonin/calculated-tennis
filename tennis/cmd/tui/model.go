package main

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/AndochBonin/calculated-tennis/tennis/tennis"
	"github.com/AndochBonin/calculated-tennis/tennis/tennisabstract"
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

// step enumerates the form fields in order. stepDone marks completion and
// doubles as the total input-step count for the progress indicator.
type step int

const (
	stepPlayerA step = iota
	stepPlayerB
	stepFormat
	stepSurface
	stepAlpha
	stepSims
	stepScore
	stepFirstServer
	stepDone
)

type labeledFormat struct {
	label  string
	format tennis.MatchFormat
}

func formatChoices() []labeledFormat {
	return []labeledFormat{
		{"ATP best-of-3", tennis.DefaultFormat()},
		{"Grand Slam men best-of-5", tennis.GrandSlamMenFormat()},
		{"Grand Slam women best-of-3", tennis.GrandSlamWomenFormat()},
	}
}

var surfaceChoices = []tennisabstract.MatchSurface{
	tennisabstract.SurfaceHard,
	tennisabstract.SurfaceClay,
	tennisabstract.SurfaceGrass,
}

type model struct {
	client *tennisabstract.Client
	ctx    context.Context

	state state
	step  step
	theme Theme

	width  int // terminal size, from tea.WindowSizeMsg; centers the view
	height int

	ti      textinput.Model
	spinner spinner.Model
	cursor  int // selection index for choice steps

	// collected inputs
	playerA           string
	playerB           string
	format            tennis.MatchFormat
	formatLabel       string
	surface           tennisabstract.MatchSurface
	alpha             float64
	sims              int
	score             string
	firstServer       tennis.Player
	firstServerChosen bool
	useCoinToss       bool

	// results
	rates  [2]tennis.PlayerRates
	result tennis.SimulationResult

	err    error
	errMsg string // inline validation message on the form
}

func initialModel(ctx context.Context, client *tennisabstract.Client) model {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.CharLimit = 120

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(hardTheme.Accent)

	m := model{
		client: client,
		ctx:    ctx,
		state:  stateForm,
		step:   stepPlayerA,
		theme:  hardTheme,
		ti:     ti,
		spinner: sp,
	}
	m.configureStep()
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

// --- form ---

func (m *model) isChoiceStep() bool {
	return m.step == stepFormat || m.step == stepSurface || m.step == stepFirstServer
}

func (m model) choiceLabels() []string {
	switch m.step {
	case stepFormat:
		out := make([]string, 0, 3)
		for _, c := range formatChoices() {
			out = append(out, c.label)
		}
		return out
	case stepSurface:
		return []string{"Hard", "Clay", "Grass"}
	case stepFirstServer:
		labels := []string{m.playerA, m.playerB}
		if m.score == "" {
			labels = append(labels, "Coin toss (per simulation)")
		}
		return labels
	}
	return nil
}

// currentChoiceIndex restores the prior selection when (re)entering a choice step.
func (m model) currentChoiceIndex() int {
	switch m.step {
	case stepFormat:
		for i, c := range formatChoices() {
			if c.label == m.formatLabel {
				return i
			}
		}
	case stepSurface:
		for i, s := range surfaceChoices {
			if s == m.surface {
				return i
			}
		}
	case stepFirstServer:
		if m.firstServerChosen {
			if m.firstServer == tennis.B {
				return 1
			}
			return 0
		}
		return 2 // coin toss (clamped away when a score is present)
	}
	return 0
}

// configureStep prepares the widget/cursor for the current step, prefilling any
// previously entered value.
func (m *model) configureStep() {
	m.errMsg = ""
	if m.isChoiceStep() {
		idx := m.currentChoiceIndex()
		if n := len(m.choiceLabels()); idx >= n {
			idx = n - 1
		}
		if idx < 0 {
			idx = 0
		}
		m.cursor = idx
		return
	}
	m.ti.Reset()
	m.ti.Placeholder = stepPlaceholder(m.step)
	m.ti.SetValue(m.storedText(m.step))
	m.ti.CursorEnd()
	m.ti.Focus()
}

func (m model) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		var cmd tea.Cmd
		m.ti, cmd = m.ti.Update(msg)
		return m, cmd
	}
	if key.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}
	if m.isChoiceStep() {
		return m.updateChoice(key)
	}
	switch key.Type {
	case tea.KeyEnter:
		return m.submitText()
	case tea.KeyEsc:
		return m.goBack()
	}
	var cmd tea.Cmd
	m.ti, cmd = m.ti.Update(key)
	return m, cmd
}

func (m model) updateChoice(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	n := len(m.choiceLabels())
	switch key.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < n-1 {
			m.cursor++
		}
	case "enter":
		return m.selectChoice()
	case "esc":
		return m.goBack()
	}
	return m, nil
}

func (m model) selectChoice() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepFormat:
		fc := formatChoices()[m.cursor]
		m.format, m.formatLabel = fc.format, fc.label
	case stepSurface:
		m.surface = surfaceChoices[m.cursor]
		m.theme = themeForSurface(m.surface)
		m.spinner.Style = lipgloss.NewStyle().Foreground(m.theme.Accent)
	case stepFirstServer:
		switch m.cursor {
		case 0:
			m.firstServer, m.firstServerChosen, m.useCoinToss = tennis.A, true, false
		case 1:
			m.firstServer, m.firstServerChosen, m.useCoinToss = tennis.B, true, false
		default:
			m.firstServerChosen, m.useCoinToss = false, true
		}
	}
	return m.advance()
}

func (m model) submitText() (tea.Model, tea.Cmd) {
	val := strings.TrimSpace(m.ti.Value())
	switch m.step {
	case stepPlayerA:
		if val == "" {
			m.errMsg = "player name is required"
			return m, nil
		}
		m.playerA = val
	case stepPlayerB:
		if val == "" {
			m.errMsg = "player name is required"
			return m, nil
		}
		m.playerB = val
	case stepAlpha:
		f, err := strconv.ParseFloat(val, 64)
		if err != nil || f <= 0 || math.IsNaN(f) || math.IsInf(f, 0) {
			m.errMsg = "alpha must be a number > 0"
			return m, nil
		}
		m.alpha = f
	case stepSims:
		n, err := strconv.Atoi(val)
		if err != nil || n <= 0 {
			m.errMsg = "sims must be a whole number > 0"
			return m, nil
		}
		m.sims = n
	case stepScore:
		m.score = val // optional; validated when the match is built
	}
	return m.advance()
}

func (m model) advance() (tea.Model, tea.Cmd) {
	m.step++
	if m.step == stepDone {
		return m.startSim()
	}
	m.configureStep()
	if m.isChoiceStep() {
		return m, nil
	}
	return m, textinput.Blink
}

func (m model) goBack() (tea.Model, tea.Cmd) {
	if m.step == stepPlayerA {
		return m, nil
	}
	m.step--
	m.configureStep()
	if m.isChoiceStep() {
		return m, nil
	}
	return m, textinput.Blink
}

func (m model) startSim() (tea.Model, tea.Cmd) {
	m.state = stateLoading
	names := [2]string{m.playerA, m.playerB}
	return m, tea.Batch(m.spinner.Tick, fetchRatesCmd(m.ctx, m.client, names, m.surface))
}

func (m model) simInputs() simInputs {
	return simInputs{
		format:            m.format,
		surface:           m.surface,
		alpha:             m.alpha,
		sims:              m.sims,
		score:             m.score,
		firstServer:       m.firstServer,
		firstServerChosen: m.firstServerChosen,
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
	m.step = stepPlayerA
	m.err = nil
	m.result = tennis.SimulationResult{}
	m.configureStep()
	return m, textinput.Blink
}

// --- step metadata ---

func (m model) storedText(s step) string {
	switch s {
	case stepPlayerA:
		return m.playerA
	case stepPlayerB:
		return m.playerB
	case stepAlpha:
		if m.alpha > 0 {
			return strconv.FormatFloat(m.alpha, 'g', -1, 64)
		}
	case stepSims:
		if m.sims > 0 {
			return strconv.Itoa(m.sims)
		}
	case stepScore:
		return m.score
	}
	return ""
}

func stepPlaceholder(s step) string {
	switch s {
	case stepPlayerA, stepPlayerB:
		return "e.g. Jannik Sinner"
	case stepAlpha:
		return "e.g. 2.5"
	case stepSims:
		return "e.g. 10000"
	case stepScore:
		return "e.g. 7-5 4-6 2-3  (optional — Enter to skip)"
	}
	return ""
}

func stepTitle(s step) string {
	switch s {
	case stepPlayerA:
		return "Player A name"
	case stepPlayerB:
		return "Player B name"
	case stepFormat:
		return "Match format"
	case stepSurface:
		return "Court surface"
	case stepAlpha:
		return "Alpha (sensitivity, > 0)"
	case stepSims:
		return "Number of simulations"
	case stepScore:
		return "Match score so far (optional)"
	case stepFirstServer:
		return "First server"
	}
	return ""
}

func pct(v float64) string {
	return fmt.Sprintf("%.1f%%", 100*v)
}
