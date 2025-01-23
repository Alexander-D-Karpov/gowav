package commands

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"
	"strings"
)

func (c *Commander) handleArtwork() (string, error, tea.Cmd) {
	if c.processor == nil {
		return "", fmt.Errorf("no track loaded"), nil
	}

	metadata := c.processor.GetMetadata()
	if metadata == nil {
		return "", fmt.Errorf("no track metadata available"), nil
	}

	// Check for artwork presence and create debug info
	if !metadata.HasArtwork || metadata.Artwork == nil {
		return "", fmt.Errorf("no artwork available"), nil
	}

	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("87")).
		Render(fmt.Sprintf("%s - %s", metadata.Artist, metadata.Title))

	width, height, _ := term.GetSize(0)
	if width == 0 || height == 0 {
		width = 80
		height = 24
	}

	bounds := metadata.Artwork.Bounds()
	origWidth := bounds.Dx()
	origHeight := bounds.Dy()

	targetWidth := width - 4
	targetHeight := height - 8
	aspect := float64(origWidth) / float64(origHeight) * 2

	if float64(targetWidth)/float64(targetHeight) > aspect {
		targetWidth = int(float64(targetHeight) * aspect)
	} else {
		targetHeight = int(float64(targetWidth) / aspect)
	}

	if targetWidth < 2 {
		targetWidth = 2
	}
	if targetHeight < 2 {
		targetHeight = 2
	}

	var sb strings.Builder
	for y := 0; y < targetHeight; y++ {
		for x := 0; x < targetWidth; x++ {
			imgX := int(float64(x) * float64(origWidth) / float64(targetWidth))
			imgY := int(float64(y) * float64(origHeight) / float64(targetHeight))
			r, g, b, _ := metadata.Artwork.At(imgX, imgY).RGBA()
			r >>= 8
			g >>= 8
			b >>= 8
			colorCode := fmt.Sprintf("#%02x%02x%02x", r, g, b)
			sb.WriteString(lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorCode)).
				Render("█"))
		}
		sb.WriteString("\n")
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(0)

	output := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		style.Render(sb.String()),
	)

	return output, nil, nil
}
