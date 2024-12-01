package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"gowav/internal/audio"
	"gowav/pkg/api"
)

type Mode int

const (
	ModeNormal Mode = iota
	ModeTrack
)

type Track struct {
	Title    string
	Artist   string
	Album    string
	Duration time.Duration
}

type Commander struct {
	player        *audio.Player
	processor     *audio.Processor
	apiClient     *api.Client
	searchResults []SearchResult
	mode          Mode
	currentTrack  *Track
	loadProgress  float64
}

func (c *Commander) GetCurrentTrack() *Track {
	if c.processor == nil || c.processor.GetMetadata() == nil {
		return nil
	}

	metadata := c.processor.GetMetadata()
	return &Track{
		Title:    metadata.Title,
		Artist:   metadata.Artist,
		Album:    metadata.Album,
		Duration: metadata.Duration,
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
		formatDuration(position),
		formatDuration(duration))

	return status + "\n" + c.player.RenderTrackBar(60)
}

func (c *Commander) GetLoadingProgress() float64 {
	return c.loadProgress
}

func (c *Commander) SetLoadingProgress(progress float64) {
	c.loadProgress = progress
}

func NewCommander() *Commander {
	return &Commander{
		player:    audio.NewPlayer(),
		processor: audio.NewProcessor(),
		apiClient: api.NewClient(),
	}
}

func (c *Commander) IsInTrackMode() bool {
	return c.mode == ModeTrack
}

func (c *Commander) Execute(input string) (string, error, tea.Cmd) {
	input = strings.TrimSpace(input)
	input = strings.TrimPrefix(input, ":")

	// Check if input is a direct file path
	if strings.HasPrefix(input, "/") || strings.HasPrefix(input, "./") || strings.HasPrefix(input, "~/") {
		path := strings.TrimSpace(input)
		if strings.HasPrefix(path, "~/") {
			homeDir, err := os.UserHomeDir()
			if err == nil {
				path = filepath.Join(homeDir, path[2:])
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

	return c.handleCommand(cmd, args, c.mode)
}