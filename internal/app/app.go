package app

import (
	"gowav/internal/ui"
)

type App struct {
	ui *ui.TUI
}

func New() *App {
	return &App{
		ui: ui.New(),
	}
}

func (a *App) Run() error {
	return a.ui.Start()
}
