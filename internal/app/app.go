package app

import (
	"gowav/internal/ui"
)

// App represents the top-level application structure, tying the TUI to the main Run method.
type App struct {
	ui *ui.TUI
}

// New initializes and returns a new App instance with its own TUI.
func New() *App {
	return &App{
		ui: ui.New(),
	}
}

// Run starts the TUI main loop.
func (a *App) Run() error {
	return a.ui.Start()
}
