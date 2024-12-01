package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"gowav/internal/ui"
)

func main() {
	p := tea.NewProgram(ui.NewModel())
	if err := p.Start(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
