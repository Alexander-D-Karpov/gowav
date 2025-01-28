package commands

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"gowav/internal/audio"
	"gowav/pkg/api"
	"os"
	"path/filepath"
	"strings"
)

type Commander struct {
	player       *audio.Player
	processor    *audio.Processor
	apiClient    *api.Client
	mode         Mode
	loadProgress float64
	currentTrack *Track

	searchResults []SearchResult
}

func NewCommander() *Commander {
	return &Commander{
		player:    audio.NewPlayer(),
		processor: audio.NewProcessor(),
		apiClient: api.NewClient(),
		mode:      ModeNormal,
	}
}

func (c *Commander) IsInTrackMode() bool {
	return c.mode == ModeTrack
}

func (c *Commander) GetProcessor() *audio.Processor {
	return c.processor
}

func (c *Commander) GetCurrentTrack() *Track {
	if c.processor == nil || c.processor.GetMetadata() == nil {
		return nil
	}
	meta := c.processor.GetMetadata()
	return &Track{
		Title:    meta.Title,
		Artist:   meta.Artist,
		Album:    meta.Album,
		Duration: int(meta.Duration.Seconds()),
	}
}

func (c *Commander) GetPlaybackStatus() string {
	if c.player == nil {
		return ""
	}
	state := c.player.GetState()
	position := c.player.GetPosition()
	duration := c.player.GetDuration()

	status := fmt.Sprintf("[%s] %s / %s",
		formatPlaybackState(state),
		FormatDuration(position),
		FormatDuration(duration))

	return status + "\n" + c.player.RenderTrackBar(60)
}

func (c *Commander) GetLoadingProgress() float64 {
	return c.loadProgress
}
func (c *Commander) SetLoadingProgress(progress float64) {
	c.loadProgress = progress
}

func (c *Commander) Execute(input string) (string, error, tea.Cmd) {
	input = strings.TrimSpace(input)
	input = strings.TrimPrefix(input, ":")

	// If user typed a path => load
	if strings.HasPrefix(input, "/") || strings.HasPrefix(input, "./") || strings.HasPrefix(input, "~/") {
		path := strings.TrimSpace(input)
		if strings.HasPrefix(path, "~/") {
			home, err := os.UserHomeDir()
			if err == nil {
				path = filepath.Join(home, path[2:])
			}
		}
		output, err := c.handleLoad(path)
		if err == nil {
			c.mode = ModeTrack
		}
		return output, err, nil
	}

	parts := strings.Fields(input)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty command"), nil
	}

	cmd := strings.ToLower(parts[0])
	args := parts[1:]

	if c.mode == ModeTrack {
		return c.handleTrackCommand(cmd, args)
	}
	return c.handleNormalCommand(cmd, args)
}

func (c *Commander) GetPlayer() *audio.Player {
	return c.player
}
