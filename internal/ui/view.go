package ui

import (
	"fmt"
	"strings"
)

func (m Model) View() string {
	if !m.ready {
		return "\nInitializing..."
	}

	if m.uiMode == ModeMini {
		return m.miniView()
	}
	return m.fullView()
}

func (m Model) miniView() string {
	var sb strings.Builder

	if m.commander.IsInTrackMode() {
		track := m.commander.GetCurrentTrack()
		sb.WriteString(fmt.Sprintf("\n%s - %s\n", track.Artist, track.Title))
		sb.WriteString(m.commander.GetPlaybackStatus())
	}

	inputPrefix := "> "
	if m.searchMode {
		inputPrefix = "search> "
	}
	sb.WriteString(fmt.Sprintf("\n%s%s", inputPrefix, m.input.View()))

	return sb.String()
}

func (m Model) fullView() string {
	var sb strings.Builder

	if m.loading {
		sb.WriteString(fmt.Sprintf("\n%s %s\n", m.spinner.View(), m.loadingMsg))
		sb.WriteString(m.progress.ViewAs(m.commander.GetLoadingProgress()))
		sb.WriteString("\n\n")
	}

	content := m.mainOutput
	if m.tabOutput != "" {
		content += "\n" + m.tabOutput
	}
	m.viewport.SetContent(content)
	sb.WriteString(m.viewport.View())

	inputPrefix := "> "
	if m.searchMode {
		inputPrefix = "search> "
	}
	sb.WriteString(fmt.Sprintf("\n%s%s", inputPrefix, m.input.View()))

	if m.exitPrompt {
		sb.WriteString("\nPress Ctrl+C again to exit or any other key to continue...")
	}

	return sb.String()
}
