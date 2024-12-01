package ui

import (
	"fmt"
	"gowav/pkg/utils"
	"os"
	"path/filepath"
	"strings"
)

type TabState struct {
	Completions   []string
	CurrentIndex  int
	OriginalInput string
	CurrentPath   string
	Command       string
	HasTabbed     bool // Track if we've already tabbed for current input
}

func (m *Model) handleTabCompletion() {
	input := m.input.Value()

	// Handle empty input case
	if input == "" || input == "l " || input == "load " {
		path := ""
		if cwd, err := os.Getwd(); err == nil {
			path = cwd
		}
		m.updateTabCompletions("l", path, false)
		return
	}

	// Check for load command
	if !strings.HasPrefix(input, "load ") && !strings.HasPrefix(input, "l ") {
		return
	}

	parts := strings.SplitN(input, " ", 2)
	cmd := parts[0]
	path := "."

	if len(parts) > 1 {
		path = strings.TrimSpace(parts[1])
		path = strings.Trim(path, `"'`)
	}

	// Expand home directory
	if strings.HasPrefix(path, "~") {
		if homeDir, err := os.UserHomeDir(); err == nil {
			path = strings.Replace(path, "~", homeDir, 1)
		}
	}

	// Determine if we should cycle or get new completions
	isNewPath := m.tabState == nil || path != m.tabState.CurrentPath
	shouldCycle := m.tabState != nil && m.tabState.HasTabbed && !isNewPath

	// Update completions
	m.updateTabCompletions(cmd, path, shouldCycle)
}

func (m *Model) updateTabCompletions(cmd, path string, shouldCycle bool) {
	// Get completions
	completions := utils.GetCompletions(path)
	if len(completions) == 0 {
		m.tabOutput = ""
		return
	}

	// Initialize or update tab state
	if m.tabState == nil || path != m.tabState.CurrentPath {
		m.tabState = &TabState{
			Completions:   completions,
			CurrentIndex:  0,
			OriginalInput: path,
			CurrentPath:   path,
			Command:       cmd,
			HasTabbed:     false,
		}
	} else if shouldCycle {
		// Cycle through completions only on subsequent tabs
		m.tabState.CurrentIndex = (m.tabState.CurrentIndex + 1) % len(completions)
	}

	// Mark that we've tabbed for this input
	m.tabState.HasTabbed = true

	// Update input with current selection
	current := completions[m.tabState.CurrentIndex]
	if strings.Contains(current, " ") {
		current = `"` + current + `"`
	}

	// Set cursor to end of input
	m.input.SetValue(fmt.Sprintf("%s %s", cmd, current))
	m.input.CursorEnd()

	// Format completions display
	m.formatCompletionsDisplay()
}

func (m *Model) formatCompletionsDisplay() {
	if m.tabState == nil || len(m.tabState.Completions) == 0 {
		m.tabOutput = ""
		return
	}

	var sb strings.Builder
	sb.WriteString("\nCompletions:\n")

	// Calculate max width for items
	maxWidth := 0
	for _, c := range m.tabState.Completions {
		name := filepath.Base(c)
		if len(name) > maxWidth {
			maxWidth = len(name)
		}
	}

	// Add padding and space for selection indicator
	itemWidth := maxWidth + 4
	columns := (m.width - 4) / itemWidth
	if columns < 1 {
		columns = 1
	}

	// Format in columns
	for i, completion := range m.tabState.Completions {
		name := filepath.Base(completion)
		isDir := strings.HasSuffix(completion, "/")

		// Add selection indicator
		if i == m.tabState.CurrentIndex {
			sb.WriteString("> ")
		} else {
			sb.WriteString("  ")
		}

		// Add name with directory/file indicator
		if isDir {
			name += "/"
		}
		sb.WriteString(name)

		// Padding and newlines
		if (i+1)%columns != 0 && i < len(m.tabState.Completions)-1 {
			padding := strings.Repeat(" ", itemWidth-len(name)-2)
			sb.WriteString(padding)
		} else {
			sb.WriteString("\n")
		}
	}

	m.tabOutput = sb.String()
}

func (m *Model) clearTabCompletion() {
	m.tabState = nil
	m.tabOutput = ""
}

// Add key handler to handle forward slash
func (m *Model) handleKeyForward(r rune) {
	if r == '/' && m.tabState != nil {
		m.handleTabCompletion()
	}
}
