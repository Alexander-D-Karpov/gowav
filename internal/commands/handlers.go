package commands

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"gowav/internal/audio"
	"gowav/internal/types"
	"gowav/pkg/viz"
	"path/filepath"
	"strings"
)

func (c *Commander) handleCommand(cmd string, args []string, mode Mode) (string, error, tea.Cmd) {
	if mode == ModeTrack {
		return c.handleTrackCommand(cmd, args)
	}
	return c.handleNormalCommand(cmd, args)
}

func (c *Commander) handleTrackCommand(cmd string, args []string) (string, error, tea.Cmd) {
	switch cmd {
	case "help", "h":
		return c.handleTrackHelp()
	case "unload":
		c.mode = ModeNormal
		c.processor = audio.NewProcessor()
		return "Track unloaded. Returning to normal mode.", nil, nil
	case "info", "i":
		return c.handleInfo()
	case "play", "p":
		return c.handlePlay()
	case "pause":
		return c.handlePause()
	case "stop":
		return c.handleStop()
	case "artwork", "art":
		return c.handleArtwork()
	case "viz", "v":
		if len(args) == 0 {
			return c.handleVisualization([]string{"wave"})
		}
		return c.handleVisualization(args)
	default:
		return "", fmt.Errorf("unknown track command: %s (type 'help' for available commands)", cmd), nil
	}
}

func (c *Commander) handleNormalCommand(cmd string, args []string) (string, error, tea.Cmd) {
	switch cmd {
	case "help", "h":
		return c.handleHelp()
	case "load", "l":
		if len(args) == 0 {
			return "", fmt.Errorf("usage: load <path/url>"), nil
		}
		path := strings.Join(args, " ")
		path = strings.Trim(path, `"'`)
		path = filepath.Clean(path)
		out, err := c.handleLoad(path)
		if err == nil {
			c.mode = ModeTrack
		}
		return out, err, nil
	case "search", "s":
		if len(args) == 0 {
			return "", fmt.Errorf("usage: search <query>"), nil
		}
		output, err := c.handleSearch(strings.Join(args, " "))
		return output, err, nil
	case "quit", "q", "exit":
		return "Goodbye!", nil, tea.Quit
	default:
		return "", fmt.Errorf("unknown command: %s (type 'help' for available commands)", cmd), nil
	}
}

func (c *Commander) handleVisualization(args []string) (string, error, tea.Cmd) {
	if len(args) == 0 {
		return "", fmt.Errorf("visualization type required"), nil
	}

	// Check if processor is initialized
	if c.processor == nil || !c.IsInTrackMode() {
		return "", fmt.Errorf("no track loaded"), nil
	}

	vizMap := map[string]viz.ViewMode{
		"wave":     viz.WaveformMode,
		"spectrum": viz.SpectrogramMode,
		"tempo":    viz.TempoMode,
		"density":  viz.DensityMode,
		"beat":     viz.BeatMapMode,
	}

	vizType := strings.ToLower(args[0])
	vMode, ok := vizMap[vizType]
	if !ok {
		return "", fmt.Errorf("unknown visualization: %s", vizType), nil
	}

	// Always try to switch visualization, which will initiate analysis if needed
	output, err := c.processor.SwitchVisualization(vMode)
	if err != nil {
		// Check if it's just "preparing visualization" message
		if strings.Contains(err.Error(), "preparing visualization") {
			return output, nil, func() tea.Msg {
				return types.EnterVizMsg{Mode: vMode}
			}
		}

		// Otherwise it's a real error
		return "", fmt.Errorf("failed to switch visualization: %w", err), nil
	}

	// If we got here, either visualization was cached or instant
	return output, nil, func() tea.Msg {
		return types.EnterVizMsg{Mode: vMode}
	}
}
