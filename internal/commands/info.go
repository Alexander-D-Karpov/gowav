package commands

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
)

func (c *Commander) handleInfo() (string, error, tea.Cmd) {
	metadata := c.processor.GetMetadata()
	if metadata == nil {
		return "", fmt.Errorf("no track loaded"), nil
	}

	info := metadata.String()

	if c.player != nil {
		state := c.player.GetState()
		position := c.player.GetPosition()
		duration := c.player.GetDuration()

		info += "\n\nPlayback Status:\n"
		info += fmt.Sprintf("State: %s\n", formatPlaybackState(state))
		info += fmt.Sprintf("Position: %s\n", formatDuration(position))
		info += fmt.Sprintf("Duration: %s\n", formatDuration(duration))
		info += "\n" + c.player.RenderTrackBar(60)
	}

	return info, nil, nil
}
