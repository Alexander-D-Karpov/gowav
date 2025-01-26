package viz

import (
	"time"
)

type ViewMode int

const (
	WaveformMode ViewMode = iota
	SpectrogramMode
	TempoMode
	DensityMode
	BeatMapMode
)

type ViewState struct {
	Mode          ViewMode
	Zoom          float64
	Offset        time.Duration
	Width         int
	Height        int
	ColorScheme   ColorScheme
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

// Core utility functions used across all visualizations
func clamp(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
