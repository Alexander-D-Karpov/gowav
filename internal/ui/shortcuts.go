package ui

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"strings"
)

func (m Model) handleShortcut(key string) (string, error, tea.Cmd) {
	if command, ok := m.shortcuts[key]; ok {
		switch command {
		case "toggle-mode":
			if m.uiMode == ModeFull {
				m.uiMode = ModeMini
			} else {
				m.uiMode = ModeFull
			}
			return "UI mode toggled", nil, nil
		default:
			return m.commander.Execute(command)
		}
	}
	return "", nil, nil
}

func (m Model) showShortcuts() string {
	var sb strings.Builder
	sb.WriteString("\nKeyboard Shortcuts:\n")
	for key, command := range m.shortcuts {
		sb.WriteString(fmt.Sprintf("%-12s: %s\n", key, command))
	}
	return sb.String()
}
