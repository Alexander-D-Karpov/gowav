package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"time"
)

// streamMsg is a message that increments streaming progress or signals an error.
type streamMsg struct {
	url      string
	progress float64
	error    error
}

func (m AudioModel) handleStreamStart(url string) tea.Cmd {
	// Mark loading
	m.loadingState.IsLoading = true
	m.loadingState.Message = "Streaming..."
	m.loadingState.CanCancel = true
	m.loadingState.StartTime = time.Now()
	m.loadingState.Progress = 0

	// Return a command that sends a `streamMsg` to increment progress
	return func() tea.Msg {
		return streamMsg{url: url, progress: 0}
	}
}
