package ui

import (
	"gowav/internal/commands"
	"gowav/internal/types"
	"gowav/pkg/viz"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// UIMode distinguishes different display modes in our TUI.
type UIMode int

const (
	// ModeFull is the default mode, showing main output + input + progress bar, etc.
	ModeFull UIMode = iota

	// ModeMini is a compact mode that primarily shows playback info and an input line.
	ModeMini

	// ModeViz is a mode dedicated to visualizations (waveform, spectrogram, etc.).
	ModeViz
)

// Model is the main Bubble Tea model for our TUI.
type AudioModel struct {
	// Input and main display
	input    textinput.Model
	viewport viewport.Model

	// Commander for executing all user commands
	commander *commands.Commander

	// Progress/spinner for loading/analysis
	progress progress.Model
	spinner  spinner.Model

	// Layout & style
	style  lipgloss.Style
	ready  bool
	width  int
	height int

	// Main outputs
	mainOutput string
	tabOutput  string

	// History
	history    []string
	historyPos int

	// Key states
	exitPrompt  bool
	uiMode      UIMode
	searchMode  bool
	searchQuery string // used if we want to store the typed query

	// Visualization
	vizEnabled     bool
	currentVizMode viz.ViewMode

	// Timestamp for measuring intervals if needed
	lastUpdateTime time.Time

	// Loading/analysis state with progress
	loadingState *types.LoadingState

	// Tab-completion
	tabState *TabState

	// Keyboard shortcuts map
	shortcuts map[string]string
}

// NewModel creates a new TUI model with defaults.
func NewModel() AudioModel {
	// Text input field
	input := textinput.New()
	input.Placeholder = "Enter command (type 'help' for list)"
	input.Focus()
	input.CharLimit = 256
	input.Width = 80

	// Progress bar
	p := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(40),
		progress.WithoutPercentage(),
	)

	// Spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	// Style
	style := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240"))

	// Define default keyboard shortcuts
	defaultShortcuts := map[string]string{
		"ctrl+q":     "quit",
		"ctrl+p":     "play",
		"ctrl+s":     "stop",
		"ctrl+space": "pause",
		"ctrl+m":     "toggle-mode",
		"ctrl+l":     "clear",
		"ctrl+u":     "volume-up",
		"ctrl+d":     "volume-down",
		"v":          "toggle-viz",
		"tab":        "next-viz",
		"shift+tab":  "prev-viz",
		"+":          "zoom-in",
		"-":          "zoom-out",
		"left":       "scroll-left",
		"right":      "scroll-right",
		"0":          "reset-viz",
		"esc":        "exit-viz",
	}

	return AudioModel{
		input:          input,
		commander:      commands.NewCommander(),
		progress:       p,
		spinner:        s,
		style:          style,
		history:        make([]string, 0),
		historyPos:     -1,
		mainOutput:     "Welcome to gowav! Type 'help' for commands.\nPress '?' to show shortcuts.",
		lastUpdateTime: time.Now(),
		uiMode:         ModeFull,
		loadingState:   &types.LoadingState{},
		shortcuts:      defaultShortcuts,
	}
}

// Init is part of the Bubble Tea interface. It can return a command to run upon start.
func (m AudioModel) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		spinner.Tick,
	)
}
