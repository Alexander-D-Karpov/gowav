package viz

import (
	"fmt"
	"strings"
)

// Command represents a visualization command
type Command struct {
	Name        string
	Description string
	Handler     func(*Manager, []string) error
}

var Commands = map[string]Command{
	"viz": {
		Name:        "viz",
		Description: "Change visualization mode (viz [mode])",
		Handler:     handleVizMode,
	},
	"zoom": {
		Name:        "zoom",
		Description: "Set zoom level (zoom <level>)",
		Handler:     handleZoom,
	},
	"color": {
		Name:        "color",
		Description: "Change color scheme (color [scheme])",
		Handler:     handleColorScheme,
	},
	"reset": {
		Name:        "reset",
		Description: "Reset visualization state",
		Handler:     handleReset,
	},
}

// GetVizCommands returns a formatted list of visualization commands
func GetVizCommands() string {
	var sb strings.Builder
	sb.WriteString("Visualization Commands:\n\n")

	for _, cmd := range Commands {
		sb.WriteString(fmt.Sprintf("%-12s %s\n", cmd.Name, cmd.Description))
	}

	return sb.String()
}

func handleVizMode(m *Manager, args []string) error {
	if len(args) == 0 {
		// Show available modes
		var sb strings.Builder
		sb.WriteString("Available visualization modes:\n")
		for mode, viz := range m.visualizations {
			sb.WriteString(fmt.Sprintf("  %d: %s - %s\n", mode, viz.Name(), viz.Description()))
		}
		return fmt.Errorf(sb.String())
	}

	// Parse mode
	var mode ViewMode
	if _, err := fmt.Sscanf(args[0], "%d", &mode); err != nil {
		return fmt.Errorf("invalid mode: %s", args[0])
	}

	return m.SetMode(mode)
}

func handleZoom(m *Manager, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("current zoom: %.1fx", m.state.Zoom)
	}

	var zoom float64
	if _, err := fmt.Sscanf(args[0], "%f", &zoom); err != nil {
		return fmt.Errorf("invalid zoom level: %s", args[0])
	}

	if zoom < 1.0 {
		return fmt.Errorf("zoom must be >= 1.0")
	}

	m.state.Zoom = zoom
	return nil
}

func handleColorScheme(m *Manager, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("color scheme selection not implemented")
	}

	// TODO: Add support for selecting specific color schemes by name
	return fmt.Errorf("color scheme selection by name not yet implemented")
}

func handleReset(m *Manager, args []string) error {
	m.state.Zoom = 1.0
	m.state.Offset = 0
	return nil
}

// AutocompleteViz provides command autocompletion for visualization commands
func AutocompleteViz(input string) []string {
	if input == "" {
		return nil
	}

	var completions []string
	prefix := strings.ToLower(input)

	// Complete commands
	for cmdName := range Commands {
		if strings.HasPrefix(cmdName, prefix) {
			completions = append(completions, cmdName)
		}
	}

	// If input starts with "viz ", complete with mode numbers
	if strings.HasPrefix(input, "viz ") {
		for i := 0; i < 8; i++ { // Number of ViewMode values
			modeStr := fmt.Sprintf("viz %d", i)
			if strings.HasPrefix(modeStr, input) {
				completions = append(completions, modeStr)
			}
		}
	}

	return completions
}
