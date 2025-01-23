package ui

import tea "github.com/charmbracelet/bubbletea"

// TUI wraps our Bubble Tea program.
type TUI struct {
	program *tea.Program
}

// New returns a new TUI handle
func New() *TUI {
	return &TUI{}
}

// Start runs the TUI main loop
func (t *TUI) Start() error {
	p := tea.NewProgram(NewModel(), tea.WithAltScreen())
	t.program = p
	_, err := p.Run()
	return err
}
