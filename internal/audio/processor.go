package audio

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"strings"
)

type Processor struct {
	currentFile []byte
	metadata    *Metadata
}

func NewProcessor() *Processor {
	return &Processor{}
}

func (p *Processor) LoadFile(path string) error {
	var data []byte
	var err error

	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		// Load from URL
		resp, err := http.Get(path)
		if err != nil {
			return fmt.Errorf("failed to download file: %w", err)
		}
		defer resp.Body.Close()

		data, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}
	} else {
		// Load from local file
		data, err = os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}
	}

	// Extract metadata
	metadata, err := ExtractMetadata(data)
	if err != nil {
		return fmt.Errorf("failed to extract metadata: %w", err)
	}

	p.currentFile = data
	p.metadata = metadata
	return nil
}

func (p *Processor) GetCurrentFile() []byte {
	return p.currentFile
}

func (p *Processor) GetMetadata() *Metadata {
	return p.metadata
}

// GetArtwork renders artwork as colored ASCII art for terminal display
func (p *Processor) GetArtwork() (string, error) {
	if !p.metadata.HasArtwork || p.metadata.Artwork == nil {
		return "", fmt.Errorf("no artwork available")
	}

	// Get artwork dimensions
	bounds := p.metadata.Artwork.Bounds()
	origWidth := bounds.Dx()
	origHeight := bounds.Dy()

	// Get terminal dimensions and calculate target size
	width, height, err := term.GetSize(0)
	if err != nil {
		width = 80  // fallback width
		height = 24 // fallback height
	}

	// Calculate target dimensions, leaving room for borders and padding
	targetWidth := width - 4   // Account for borders and minimal padding
	targetHeight := height - 8 // Account for borders, title, and padding

	// Adjust for terminal character aspect ratio (approximately 2:1)
	aspectRatio := float64(origWidth) / float64(origHeight) * 2
	if float64(targetWidth)/float64(targetHeight) > aspectRatio {
		targetWidth = int(float64(targetHeight) * aspectRatio)
	} else {
		targetHeight = int(float64(targetWidth) / aspectRatio)
	}

	var sb strings.Builder
	for y := 0; y < targetHeight; y++ {
		for x := 0; x < targetWidth; x++ {
			// Map terminal position back to image position
			imgX := int(float64(x) * float64(origWidth) / float64(targetWidth))
			imgY := int(float64(y) * float64(origHeight) / float64(targetHeight))

			// Get pixel color
			r, g, b, _ := p.metadata.Artwork.At(imgX, imgY).RGBA()

			// Convert from 0-65535 to 0-255 range
			r = r >> 8
			g = g >> 8
			b = b >> 8

			// Create color string for the block character
			colorCode := fmt.Sprintf("#%02x%02x%02x", r, g, b)

			// Use block character with the color
			colored := lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorCode)).
				Render("█")

			sb.WriteString(colored)
		}
		sb.WriteRune('\n')
	}

	// Create border style
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(0)

	return style.Render(sb.String()), nil
}
