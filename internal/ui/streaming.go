package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"time"
)

// streamMsg updates any streaming or partial download progress in the UI.
type streamMsg struct {
	url      string
	progress float64
	error    error
}

// handleStreamStart triggers a background routine to update the progress of a file/URL stream (if that were implemented).
func (m AudioModel) handleStreamStart(url string) tea.Cmd {
	m.loadingState.IsLoading = true
	m.loadingState.Message = "Streaming..."
	m.loadingState.CanCancel = true
	m.loadingState.StartTime = time.Now()
	m.loadingState.Progress = 0

	return func() tea.Msg {
		return streamMsg{url: url, progress: 0}
	}
}
