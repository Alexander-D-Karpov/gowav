package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// CompletionType determines which category of completions (commands, file paths, etc.) we’re handling.
type CompletionType int

const (
	CompletionNone CompletionType = iota
	CompletionCommand
	CompletionFile
	CompletionVisualization
	CompletionPlayback
)

// TabState holds the current state of partial completions in progress, such as which suggestion index we’re on.
type TabState struct {
	Completions   []string
	CurrentIndex  int
	OriginalInput string
	CurrentPath   string
	Command       string
	HasTabbed     bool
	Type          CompletionType
}

// CompletionDef records info about a particular command, including aliases and subCommands (e.g., viz wave).
type CompletionDef struct {
	Command     string
	Aliases     []string
	Type        CompletionType
	SubCommands []string
	Description string
}

// completionDefs is the table of known commands we can suggest when the user presses <Tab>.
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

// handleTabCompletion decides how to autocomplete the user’s input, depending on whether it’s a command or subcommand.
func (m *AudioModel) handleTabCompletion() {
	input := m.input.Value()

	// If the user typed nothing, suggest all commands.
	if input == "" {
		m.handleCommandCompletion("")
		return
	}

	// Split into first token (potential command) plus remainder.
	parts := strings.Fields(input)
	cmd := strings.ToLower(parts[0])

	// Check if this token matches a known command or alias from completionDefs.
	var matchingDef *CompletionDef
	for _, def := range completionDefs {
		if cmd == def.Command || contains(def.Aliases, cmd) {
			matchingDef = &def
			break
		}
	}

	// If we did not find a known command, we try to complete the command itself.
	if matchingDef == nil {
		m.handleCommandCompletion(cmd)
		return
	}

	// Depending on the type (File, Visualization, etc.), handle completions.
	switch matchingDef.Type {
	case CompletionFile:
		m.handleFileCompletion(matchingDef, parts)
	case CompletionVisualization:
		m.handleVizCompletion(matchingDef, parts)
	case CompletionCommand:
		if len(parts) == 1 {
			// Possibly complete subcommands or just show commands.
			m.handleCommandCompletion(cmd)
		}
	case CompletionPlayback:
		if len(parts) == 1 {
			// Typically no subcommand for simple playback, so we clear.
			m.clearTabCompletion()
		}
	}
}

// handleCommandCompletion tries to complete the first token as a known command (including aliases).
func (m *AudioModel) handleCommandCompletion(partial string) {
	var completions []string

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

// handleVizCompletion autocompletes subcommands like "viz wave", "viz spectrum", etc.
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

	// Check if we’re starting fresh or cycling through the same set of suggestions again.
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
		// Cycle to the next suggestion if user pressed Tab repeatedly.
		m.tabState.CurrentIndex = (m.tabState.CurrentIndex + 1) % len(completions)
		m.tabState.HasTabbed = true
	}

	m.updateInputWithCompletion()
	m.formatCompletionsDisplay()
}

// handleFileCompletion attempts to tab-complete a file path for commands like “load <file>”.
func (m *AudioModel) handleFileCompletion(def *CompletionDef, parts []string) {
	path := "."
	if len(parts) > 1 {
		path = strings.TrimSpace(strings.Join(parts[1:], " "))
		path = strings.Trim(path, `"'`)
	}

	// Expand tilde (~) if present.
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

// updateTabState either initializes or advances the current TabState with a new list of completions.
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
		// If the user hit Tab again, move to the next suggestion.
		m.tabState.CurrentIndex = (m.tabState.CurrentIndex + 1) % len(completions)
	}

	m.tabState.HasTabbed = true
	m.updateInputWithCompletion()
	m.formatCompletionsDisplay()
}

// updateInputWithCompletion replaces the current input text with the selected suggestion.
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
	default:
		// For other types (e.g. no completions), do nothing special.
	}
	m.input.CursorEnd()
}

// formatCompletionsDisplay builds a multi-column display of the current list of completions for the user.
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

	// Decide how many columns we can fit on one line.
	itemWidth := maxWidth + 4
	columns := (m.width - 4) / itemWidth
	if columns < 1 {
		columns = 1
	}

	for i, completion := range m.tabState.Completions {
		// For file completion, just show the basename plus a trailing slash if it’s a directory.
		name := completion
		if m.tabState.Type == CompletionFile {
			base := filepath.Base(completion)
			if strings.HasSuffix(completion, string(os.PathSeparator)) {
				base += "/"
			}
			name = base
		}

		if i == m.tabState.CurrentIndex {
			sb.WriteString("> ")
		} else {
			sb.WriteString("  ")
		}
		sb.WriteString(name)

		// If this is a command completion, we can also show a brief description to the right.
		if m.tabState.Type == CompletionCommand {
			for _, def := range completionDefs {
				if def.Command == name || contains(def.Aliases, name) {
					padding := strings.Repeat(" ", maxWidth-len(name)+2)
					sb.WriteString(padding + "- " + def.Description)
					break
				}
			}
			sb.WriteString("\n")
		} else {
			// Try a multi-column layout
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

// clearTabCompletion resets our TabState and hides the completions display.
func (m *AudioModel) clearTabCompletion() {
	m.tabState = nil
	m.tabOutput = ""
}

// contains checks if a slice of strings has a particular string.
func contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

// getFilesystemCompletions scans a local directory for matches that start with the user’s partial path.
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

// isAudioFile does a quick extension check for recognized audio formats.
func isAudioFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".mp3", ".wav", ".flac", ".ogg", ".m4a", ".aac":
		return true
	default:
		return false
	}
}
