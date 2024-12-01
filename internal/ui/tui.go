package ui

import tea "github.com/charmbracelet/bubbletea"

type TUI struct {
	program *tea.Program
}

func New() *TUI {
	return &TUI{}
}

func (t *TUI) Start() error {
	p := tea.NewProgram(NewModel(), tea.WithAltScreen())
	t.program = p
	return p.Start()
}
