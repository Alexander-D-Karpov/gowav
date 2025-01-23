package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// CompletionType enumerates the kind of completions we're doing
type CompletionType int

const (
	CompletionNone CompletionType = iota
	CompletionCommand
	CompletionFile
	CompletionVisualization
	CompletionPlayback
)

// TabState holds info about completions in-progress
type TabState struct {
	Completions   []string
	CurrentIndex  int
	OriginalInput string
	CurrentPath   string
	Command       string
	HasTabbed     bool
	Type          CompletionType
}

// CompletionDef defines a command and its completion behavior
type CompletionDef struct {
	Command     string
	Aliases     []string
	Type        CompletionType
	SubCommands []string
	Description string
}

var completionDefs = []CompletionDef{
	{
		Command:     "load",
		Aliases:     []string{"l"},
		Type:        CompletionFile,
		Description: "Load audio file",
	},
	{
		Command:     "play",
		Aliases:     []string{"p"},
		Type:        CompletionPlayback,
		Description: "Play current track",
	},
	{
		Command:     "viz",
		Aliases:     []string{"v"},
		Type:        CompletionVisualization,
		SubCommands: []string{"wave", "spectrum", "tempo", "density", "beat"},
		Description: "Visualization controls",
	},
	{
		Command:     "help",
		Aliases:     []string{"h"},
		Type:        CompletionCommand,
		Description: "Show help",
	},
	{
		Command:     "quit",
		Aliases:     []string{"q", "exit"},
		Type:        CompletionCommand,
		Description: "Exit application",
	},
	{
		Command:     "pause",
		Aliases:     []string{},
		Type:        CompletionPlayback,
		Description: "Pause playback",
	},
	{
		Command:     "stop",
		Aliases:     []string{"s"},
		Type:        CompletionPlayback,
		Description: "Stop playback",
	},
	{
		Command:     "artwork",
		Aliases:     []string{"art"},
		Type:        CompletionCommand,
		Description: "Show album artwork",
	},
}

func (m *AudioModel) handleTabCompletion() {
	input := m.input.Value()

	// If empty, list everything
	if input == "" {
		m.handleCommandCompletion("")
		return
	}

	// Split into first token (command) plus the rest
	parts := strings.Fields(input)
	cmd := strings.ToLower(parts[0])

	// Identify if this is a known command or alias
	var matchingDef *CompletionDef
	for _, def := range completionDefs {
		if cmd == def.Command || contains(def.Aliases, cmd) {
			matchingDef = &def
			break
		}
	}

	if matchingDef == nil {
		// No known command => try to complete the command itself
		m.handleCommandCompletion(cmd)
		return
	}

	// If we found a known command, do completion by type
	switch matchingDef.Type {
	case CompletionFile:
		m.handleFileCompletion(matchingDef, parts)
	case CompletionVisualization:
		m.handleVizCompletion(matchingDef, parts)
	case CompletionCommand:
		if len(parts) == 1 {
			m.handleCommandCompletion(cmd)
		}
	case CompletionPlayback:
		if len(parts) == 1 {
			m.clearTabCompletion()
		}
	}
}

func (m *AudioModel) handleCommandCompletion(partial string) {
	var completions []string

	// For each known command definition
	for _, def := range completionDefs {
		if strings.HasPrefix(def.Command, partial) {
			completions = append(completions, def.Command)
		}
		for _, alias := range def.Aliases {
			if strings.HasPrefix(alias, partial) {
				completions = append(completions, alias)
			}
		}
	}

	if len(completions) == 0 {
		m.clearTabCompletion()
		return
	}

	sort.Strings(completions)
	m.updateTabState(completions, CompletionCommand, "", "")
}

func (m *AudioModel) handleVizCompletion(def *CompletionDef, parts []string) {
	var partial string
	if len(parts) > 1 {
		partial = strings.ToLower(parts[1])
	}

	var completions []string
	for _, subCmd := range def.SubCommands {
		if strings.HasPrefix(subCmd, partial) {
			completions = append(completions, subCmd)
		}
	}

	if len(completions) == 0 {
		m.clearTabCompletion()
		return
	}

	isNew := m.tabState == nil ||
		m.tabState.Command != parts[0] ||
		m.tabState.Type != CompletionVisualization

	if isNew {
		m.tabState = &TabState{
			Completions:   completions,
			CurrentIndex:  0,
			OriginalInput: partial,
			Command:       parts[0],
			HasTabbed:     false,
			Type:          CompletionVisualization,
		}
	} else {
		// Cycle through existing completions
		m.tabState.CurrentIndex = (m.tabState.CurrentIndex + 1) % len(completions)
		m.tabState.HasTabbed = true
	}

	m.updateInputWithCompletion()
	m.formatCompletionsDisplay()
}

func (m *AudioModel) handleFileCompletion(def *CompletionDef, parts []string) {
	path := "."
	if len(parts) > 1 {
		path = strings.TrimSpace(strings.Join(parts[1:], " "))
		path = strings.Trim(path, `"'`)
	}

	// Expand ~
	if strings.HasPrefix(path, "~") {
		if homeDir, err := os.UserHomeDir(); err == nil {
			path = strings.Replace(path, "~", homeDir, 1)
		}
	}

	completions := getFilesystemCompletions(path)
	if len(completions) == 0 {
		m.clearTabCompletion()
		return
	}
	m.updateTabState(completions, CompletionFile, path, parts[0])
}

func (m *AudioModel) updateTabState(completions []string, compType CompletionType, path, cmd string) {
	isNew := m.tabState == nil ||
		path != m.tabState.CurrentPath ||
		cmd != m.tabState.Command ||
		compType != m.tabState.Type

	if isNew {
		m.tabState = &TabState{
			Completions:   completions,
			CurrentIndex:  0,
			OriginalInput: path,
			CurrentPath:   path,
			Command:       cmd,
			HasTabbed:     false,
			Type:          compType,
		}
	} else if m.tabState.HasTabbed {
		m.tabState.CurrentIndex = (m.tabState.CurrentIndex + 1) % len(completions)
	}

	m.tabState.HasTabbed = true
	m.updateInputWithCompletion()
	m.formatCompletionsDisplay()
}

func (m *AudioModel) updateInputWithCompletion() {
	if m.tabState == nil || len(m.tabState.Completions) == 0 {
		return
	}
	current := m.tabState.Completions[m.tabState.CurrentIndex]

	switch m.tabState.Type {
	case CompletionCommand:
		m.input.SetValue(current)
	case CompletionFile:
		if strings.Contains(current, " ") {
			current = `"` + current + `"`
		}
		m.input.SetValue(fmt.Sprintf("%s %s", m.tabState.Command, current))
	case CompletionVisualization:
		m.input.SetValue(fmt.Sprintf("%s %s", m.tabState.Command, current))
	}
	m.input.CursorEnd()
}

func (m *AudioModel) formatCompletionsDisplay() {
	if m.tabState == nil || len(m.tabState.Completions) == 0 {
		m.tabOutput = ""
		return
	}

	var sb strings.Builder
	switch m.tabState.Type {
	case CompletionCommand:
		sb.WriteString("\nAvailable Commands:\n")
	case CompletionFile:
		sb.WriteString("\nFiles:\n")
	case CompletionVisualization:
		sb.WriteString("\nVisualization Types:\n")
	default:
		sb.WriteString("\nCompletions:\n")
	}

	maxWidth := 0
	for _, c := range m.tabState.Completions {
		name := filepath.Base(c)
		if len(name) > maxWidth {
			maxWidth = len(name)
		}
	}

	// We will attempt a multi-column display
	itemWidth := maxWidth + 4
	columns := (m.width - 4) / itemWidth
	if columns < 1 {
		columns = 1
	}

	for i, completion := range m.tabState.Completions {
		name := completion
		if m.tabState.Type == CompletionFile {
			name = filepath.Base(completion)
			if strings.HasSuffix(completion, "/") {
				name += "/"
			}
		}
		if i == m.tabState.CurrentIndex {
			sb.WriteString("> ")
		} else {
			sb.WriteString("  ")
		}
		sb.WriteString(name)

		if m.tabState.Type == CompletionCommand {
			// If command, show short description
			for _, def := range completionDefs {
				if def.Command == name || contains(def.Aliases, name) {
					padding := strings.Repeat(" ", maxWidth-len(name)+2)
					sb.WriteString(padding + "- " + def.Description)
					break
				}
			}
			sb.WriteString("\n")
		} else {
			if (i+1)%columns != 0 && i < len(m.tabState.Completions)-1 {
				padding := strings.Repeat(" ", itemWidth-len(name)-2)
				sb.WriteString(padding)
			} else {
				sb.WriteString("\n")
			}
		}
	}
	m.tabOutput = sb.String()
}

func (m *AudioModel) clearTabCompletion() {
	m.tabState = nil
	m.tabOutput = ""
}

// Helper to see if a string is in a slice
func contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

// getFilesystemCompletions returns possible file completions.
func getFilesystemCompletions(path string) []string {
	dir := filepath.Dir(path)
	if dir == "." {
		dir = "."
	}
	base := filepath.Base(path)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var completions []string
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(strings.ToLower(name), strings.ToLower(base)) {
			continue
		}
		fullPath := filepath.Join(dir, name)
		if entry.IsDir() {
			completions = append(completions, fullPath+string(os.PathSeparator))
		} else if isAudioFile(name) {
			completions = append(completions, fullPath)
		}
	}
	sort.Strings(completions)
	return completions
}

// isAudioFile is a minimal extension check
func isAudioFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".mp3", ".wav", ".flac", ".ogg", ".m4a", ".aac":
		return true
	default:
		return false
	}
}
