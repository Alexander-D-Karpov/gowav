package ui

import (
	"fmt"
	"strings"
	"time"
)

func (m AudioModel) View() string {
	if !m.ready {
		return "\nInitializing..."
	}

	// If we have a loading/analysis in progress, show that first
	if m.loadingState.IsLoading {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("\n%s %s\n",
			m.spinner.View(),
			m.loadingState.Message,
		))

		// Only show progress bar if we have actual progress
		if m.loadingState.FileSize > 0 && m.loadingState.BytesLoaded > 0 {
			progress := float64(m.loadingState.BytesLoaded) / float64(m.loadingState.FileSize)
			sb.WriteString(m.progress.ViewAs(progress))

			// Only show ETA if we have enough data to calculate it
			if m.loadingState.BytesLoaded > 0 && time.Since(m.loadingState.StartTime).Seconds() > 0.5 {
				eta := m.loadingState.GetETA()
				if eta != "" {
					sb.WriteString(fmt.Sprintf("\nETA: %s", eta))
				}
			}
		}

		// Show cancel option if available
		if m.loadingState.CanCancel {
			sb.WriteString("\n(Press Ctrl+C to cancel)")
		}
		return sb.String()
	}

	// Switch view based on mode
	switch m.uiMode {
	case ModeMini:
		return m.miniView()
	case ModeViz:
		return m.vizView()
	default:
		return m.fullView()
	}
}

// miniView is a compact UI.
func (m AudioModel) miniView() string {
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

// fullView is the default UI with a main viewport + input line.
func (m AudioModel) fullView() string {
	var sb strings.Builder

	// Show main content in viewport
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

// vizView is for visualization mode.
func (m AudioModel) vizView() string {
	var sb strings.Builder

	if m.commander.IsInTrackMode() {
		track := m.commander.GetCurrentTrack()
		sb.WriteString(fmt.Sprintf("\n%s - %s\n", track.Artist, track.Title))

		// Render the visualization
		vizContent := m.commander.GetProcessor().GetVisualization()
		sb.WriteString(vizContent)

		sb.WriteString("\n" + m.commander.GetPlaybackStatus())
	} else {
		sb.WriteString("\nNo track loaded for visualization")
	}

	// Input line at bottom
	sb.WriteString(fmt.Sprintf("\n%s%s", m.getPrompt(), m.input.View()))
	return sb.String()
}

func (m AudioModel) getPrompt() string {
	if m.searchMode {
		return "search> "
	}
	if m.uiMode == ModeViz {
		return "viz> "
	}
	return "> "
}
