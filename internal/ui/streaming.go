package ui

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"time"
)

type streamMsg struct {
	url      string
	progress float64
	error    error
}

func (m Model) handleStreamStart(url string) tea.Cmd {
	return func() tea.Msg {
		return streamMsg{url: url, progress: 0}
	}
}

func (m Model) updateStreamProgress(msg streamMsg) (Model, tea.Cmd) {
	if msg.error != nil {
		m.loading = false
		m.mainOutput = fmt.Sprintf("Streaming error: %v", msg.error)
		return m, nil
	}

	m.loading = true
	m.loadingMsg = fmt.Sprintf("Streaming from %s... %.0f%%", msg.url, msg.progress*100)

	if msg.progress >= 1.0 {
		m.loading = false
		return m, nil
	}

	return m, tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return streamMsg{url: msg.url, progress: msg.progress + 0.1}
	})
}
