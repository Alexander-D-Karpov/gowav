package commands

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"gowav/internal/audio"
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
	case "artwork":
		return c.handleArtwork()
	case "stop":
		return c.handleStop()
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
		output, err := c.handleLoad(path)
		if err == nil {
			c.mode = ModeTrack
		}
		return output, err, nil
	case "search", "s":
		if len(args) == 0 {
			return "", fmt.Errorf("usage: search <query>"), nil
		}
		return c.handleSearch(strings.Join(args, " "))
	case "quit", "q", "exit":
		return "Goodbye!", nil, tea.Quit
	default:
		return "", fmt.Errorf("unknown command: %s (type 'help' for available commands)", cmd), nil
	}
}
