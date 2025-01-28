package ui

import (
	"fmt"
	"gowav/internal/audio"
	"gowav/internal/commands"
	"gowav/internal/types"
	"gowav/pkg/viz"
	"strings"
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

type enterVizMsg struct {
	mode viz.ViewMode
}

const (
	// ModeFull is the default mode, showing main output + input + progress bar, etc.
	ModeFull UIMode = iota

	// ModeMini is a compact mode that primarily shows playback info and an input line.
	ModeMini

	// ModeViz is a mode dedicated to visualizations (waveform, spectrogram, etc.).
	ModeViz
)

// AudioModel is the main Bubble Tea model for our TUI.
type AudioModel struct {
	// Input and main display
	input    textinput.Model
	viewport viewport.Model

	// Commander for executing user commands
	commander *commands.Commander

	// Progress/spinner for loading or analysis
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
	searchQuery string // used if we want to store typed query

	// Visualization
	vizEnabled     bool
	currentVizMode viz.ViewMode

	// Timestamp for intervals if needed
	lastUpdateTime time.Time

	// Loading/analysis state
	loadingState *types.LoadingState

	// Tab-completion
	tabState *TabState

	// Keyboard shortcuts map
	shortcuts map[string]string

	// Whether to show the "full info" (raw tags, no artwork) vs. partial
	showFullInfo bool
}

// NewModel creates a new TUI model with defaults.
func NewModel() AudioModel {
	// Text input
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

// Init returns any initial commands to run.
func (m AudioModel) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		spinner.Tick,
	)
}

// BuildMetadataOutput chooses partial or full table based on m.showFullInfo.
// If full, we show raw tags (no artwork). If partial, we show artwork if there's space.
func (m *AudioModel) BuildMetadataOutput(meta *audio.Metadata) string {
	if m.showFullInfo {
		// Full table with raw tags, no artwork
		out := meta.AdaptiveStringWithRaw(m.width, m.height)
		out += m.buildPlaybackStatus()
		return out
	} else {
		// Partial table with optional side-by-side artwork
		out := meta.BuildLoadInfo(m.width, m.height)
		out += m.buildPlaybackStatus()
		return out
	}
}

// buildPlaybackStatus appends current playback info from the Commander’s player.
func (m *AudioModel) buildPlaybackStatus() string {
	player := m.commander.GetPlayer()
	if player == nil {
		return ""
	}
	state := player.GetState()
	position := player.GetPosition()
	duration := player.GetDuration()

	var sb strings.Builder
	sb.WriteString("\n\nPlayback Status:\n")
	sb.WriteString(fmt.Sprintf("State: %s\n", formatPlaybackState(state)))
	sb.WriteString(fmt.Sprintf("Position: %s\n", localFormatDuration(position)))
	sb.WriteString(fmt.Sprintf("Duration: %s\n", localFormatDuration(duration)))
	sb.WriteString("\n" + player.RenderTrackBar(60))
	return sb.String()
}

// formatPlaybackState is a small helper for showing playback state text.
func formatPlaybackState(st audio.PlaybackState) string {
	switch st {
	case audio.StatePlaying:
		return "Playing"
	case audio.StatePaused:
		return "Paused"
	default:
		return "Stopped"
	}
}

// localFormatDuration shows mm:ss
func localFormatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	min := int(d.Minutes())
	sec := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d", min, sec)
}
