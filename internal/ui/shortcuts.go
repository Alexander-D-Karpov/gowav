package ui

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"strings"
)

// handleShortcut checks if the pressed key is in our shortcuts map and executes the associated command string.
func (m AudioModel) handleShortcut(key string) (string, error, tea.Cmd) {
	if command, ok := m.shortcuts[key]; ok {
		switch command {
		case "toggle-mode":
			if m.uiMode == ModeViz {
				return "Switched to Full Mode", nil, func() tea.Msg { return nil }
			} else if m.uiMode == ModeFull {
				return "Switched to Mini Mode", nil, func() tea.Msg { return nil }
			} else {
				return "Switched to Full Mode", nil, func() tea.Msg { return nil }
			}

		case "toggle-viz":
			if !m.commander.IsInTrackMode() {
				return "", fmt.Errorf("no track loaded"), nil
			}
			if m.uiMode == ModeViz {
				return "Exiting Viz Mode", nil, func() tea.Msg { return nil }
			} else {
				return "Entering Viz Mode", nil, func() tea.Msg { return nil }
			}

		default:
			// For other shortcuts, treat them as typed commands (like "play", "stop", etc.).
			return m.commander.Execute(command)
		}
	}
	return "", nil, nil
}

// showShortcuts prints a set of known key bindings for the user.
func (m AudioModel) showShortcuts() string {
	if m.uiMode == ModeViz {
		return m.showVisualizationShortcuts()
	}

	var sb strings.Builder
	sb.WriteString("\nKeyboard Shortcuts:\n")

	generalShortcuts := make(map[string]string)
	playbackShortcuts := make(map[string]string)

	for key, cmd := range m.shortcuts {
		if strings.Contains(cmd, "play") || strings.Contains(cmd, "pause") ||
			strings.Contains(cmd, "stop") || strings.Contains(cmd, "volume") {
			playbackShortcuts[key] = cmd
		} else {
			generalShortcuts[key] = cmd
		}
	}

	sb.WriteString("\nGeneral Controls:\n")
	for key, cmd := range generalShortcuts {
		sb.WriteString(fmt.Sprintf("  %-12s: %s\n", key, cmd))
	}

	sb.WriteString("\nPlayback Controls:\n")
	for key, cmd := range playbackShortcuts {
		sb.WriteString(fmt.Sprintf("  %-12s: %s\n", key, cmd))
	}

	return sb.String()
}

// showVisualizationShortcuts is displayed when in Viz mode.
func (m AudioModel) showVisualizationShortcuts() string {
	var sb strings.Builder
	sb.WriteString("\nVisualization Controls:\n\n")

	sb.WriteString("Navigation:\n")
	sb.WriteString("  left/h       : Move backward in time\n")
	sb.WriteString("  right/l      : Move forward in time\n")
	sb.WriteString("  +/=          : Zoom in\n")
	sb.WriteString("  -/_          : Zoom out\n")
	sb.WriteString("  0            : Reset view\n")

	sb.WriteString("\nView Controls:\n")
	sb.WriteString("  tab          : Next visualization type\n")
	sb.WriteString("  shift+tab    : Previous visualization type\n")
	sb.WriteString("  q/esc        : Exit visualization mode\n")

	sb.WriteString("\nAvailable Commands:\n")
	sb.WriteString("  viz wave     : Waveform\n")
	sb.WriteString("  viz spectrum : Spectrogram\n")
	sb.WriteString("  viz tempo    : Tempo/energy\n")
	sb.WriteString("  viz density  : Audio density map\n")
	sb.WriteString("  viz beat     : Beat & rhythm patterns\n")

	return sb.String()
}
