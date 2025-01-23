package viz

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"strings"
	"time"
)

type ViewMode int

const (
	WaveformMode ViewMode = iota
	SpectrogramMode
	DensityMode
	TempoMode
	FrequencyMode
	BeatMapMode
)

// ViewState maintains common visualization state
type ViewState struct {
	Mode          ViewMode
	Zoom          float64
	Offset        time.Duration
	Width         int
	Height        int
	ColorScheme   ColorScheme
	IsInteractive bool

	// Navigation properties
	ScrollSpeed   time.Duration
	WindowSize    time.Duration
	TotalDuration time.Duration
}

// Visualization interface
type Visualization interface {
	Render(state ViewState) string
	HandleInput(key string, state *ViewState) bool
	Name() string
	Description() string
	SetTotalDuration(duration time.Duration)
}

// Helper for color gradient
func hexToRGB(hex string) (int, int, int) {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) == 3 {
		hex = string(hex[0]) + string(hex[0]) +
			string(hex[1]) + string(hex[1]) +
			string(hex[2]) + string(hex[2])
	}

	var r, g, b int
	fmt.Sscanf(hex, "%02x%02x%02x", &r, &g, &b)
	return r, g, b
}

func getGradientColor(intensity float64, scheme ColorScheme) lipgloss.Color {
	r1, g1, b1 := hexToRGB(string(scheme.Primary))
	r2, g2, b2 := hexToRGB(string(scheme.Accent))

	r := int(float64(r1) + intensity*float64(r2-r1))
	g := int(float64(g1) + intensity*float64(g2-g1))
	b := int(float64(b1) + intensity*float64(b2-b1))

	return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", r, g, b))
}
