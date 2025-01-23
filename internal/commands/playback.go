package commands

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"gowav/internal/audio"
	"time"
)

func (c *Commander) handlePlay() (string, error, tea.Cmd) {
	if c.processor == nil || c.processor.GetCurrentFile() == nil {
		return "", fmt.Errorf("no track loaded"), nil
	}
	if err := c.player.Play(c.processor.GetCurrentFile()); err != nil {
		return "", fmt.Errorf("failed to play: %w", err), nil
	}
	return "Playing...", nil, c.startPlaybackUpdates()
}

func (c *Commander) handlePause() (string, error, tea.Cmd) {
	if c.player.GetState() != audio.StatePlaying {
		return "", fmt.Errorf("no track is currently playing"), nil
	}
	if err := c.player.Pause(); err != nil {
		return "", fmt.Errorf("failed to pause: %w", err), nil
	}
	return "Paused", nil, nil
}

func (c *Commander) handleStop() (string, error, tea.Cmd) {
	if err := c.player.Stop(); err != nil {
		return "", fmt.Errorf("failed to stop: %w", err), nil
	}
	return "Stopped", nil, nil
}

func (c *Commander) startPlaybackUpdates() tea.Cmd {
	return tea.Tick(time.Second/10, func(time.Time) tea.Msg {
		return playbackUpdateMsg{}
	})
}

func formatPlaybackState(state audio.PlaybackState) string {
	switch state {
	case audio.StatePlaying:
		return "Playing"
	case audio.StatePaused:
		return "Paused"
	default:
		return "Stopped"
	}
}
