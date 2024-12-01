package ui

import (
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"gowav/internal/commands"
)

type UIMode int

const (
	ModeFull UIMode = iota
	ModeMini
)

type progressMsg float64

type Model struct {
	input          textinput.Model
	viewport       viewport.Model
	commander      *commands.Commander
	progress       progress.Model
	spinner        spinner.Model
	style          lipgloss.Style
	ready          bool
	width          int
	height         int
	mainOutput     string
	tabOutput      string
	lastUpdateTime time.Time
	history        []string
	historyPos     int
	tabState       *TabState
	searchMode     bool
	searchQuery    string
	exitPrompt     bool
	loading        bool
	loadingMsg     string
	uiMode         UIMode
	shortcuts      map[string]string
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		spinner.Tick,
	)
}

func NewModel() Model {
	input := textinput.New()
	input.Placeholder = "Enter command (type 'help' for list)"
	input.Focus()
	input.CharLimit = 256
	input.Width = 80

	p := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(40),
		progress.WithoutPercentage(),
	)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	style := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240"))

	// Define keyboard shortcuts
	shortcuts := map[string]string{
		"ctrl+q":     "quit",
		"ctrl+p":     "play",
		"ctrl+s":     "stop",
		"ctrl+space": "pause",
		"ctrl+l":     "clear",
		"ctrl+m":     "toggle-mode",
		"ctrl+u":     "volume-up",
		"ctrl+d":     "volume-down",
	}

	return Model{
		input:          input,
		commander:      commands.NewCommander(),
		progress:       p,
		spinner:        s,
		style:          style,
		history:        make([]string, 0),
		historyPos:     -1,
		mainOutput:     "Welcome to gowav! Type 'help' for commands.\nPress '?' to show keyboard shortcuts.",
		lastUpdateTime: time.Now(),
		uiMode:         ModeFull,
		shortcuts:      shortcuts,
	}
}
