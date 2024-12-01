package commands

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (c *Commander) handleArtwork() (string, error, tea.Cmd) {
	if c.processor == nil {
		return "", fmt.Errorf("no track loaded"), nil
	}

	metadata := c.processor.GetMetadata()
	if metadata == nil {
		return "", fmt.Errorf("no track metadata available"), nil
	}

	if !metadata.HasArtwork {
		return "", fmt.Errorf("no artwork available for current track"), nil
	}

	// Create header
	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("87")).
		Render(fmt.Sprintf("%s - %s", metadata.Artist, metadata.Title))

	// Get artwork
	art, err := c.processor.GetArtwork()
	if err != nil {
		return "", fmt.Errorf("failed to render artwork: %w", err), nil
	}

	// Combine output
	output := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		art,
	)

	return output, nil, nil
}
