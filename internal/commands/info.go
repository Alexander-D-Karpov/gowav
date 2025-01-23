package commands

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"time"
)

func (c *Commander) handleInfo() (string, error, tea.Cmd) {
	meta := c.processor.GetMetadata()
	if meta == nil {
		return "", fmt.Errorf("no track loaded"), nil
	}

	info := meta.String()

	if c.player != nil {
		state := c.player.GetState()
		position := c.player.GetPosition()
		duration := c.player.GetDuration()

		info += "\n\nPlayback Status:\n"
		info += fmt.Sprintf("State: %s\n", formatPlaybackState(state))
		info += fmt.Sprintf("Position: %s\n", localFormatDuration(position))
		info += fmt.Sprintf("Duration: %s\n", localFormatDuration(duration))
		info += "\n" + c.player.RenderTrackBar(60)
	}

	return info, nil, nil
}

func localFormatDuration(d time.Duration) string {
	min := int(d.Minutes())
	sec := int(d.Seconds()) % 60
	return fmt.Sprintf("%d:%02d", min, sec)
}
