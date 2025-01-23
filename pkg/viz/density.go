package viz

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"math"
	"strings"
	"time"
)

const densityMaxHeight = 40

type DensityViz struct {
	data       []float64
	sampleRate int

	windowSize    int
	style         lipgloss.Style
	maxHeight     int
	totalDuration time.Duration
}

func NewDensityViz(data []float64, sampleRate int) *DensityViz {
	return &DensityViz{
		data:       data,
		sampleRate: sampleRate,
		windowSize: 1024,
		style:      lipgloss.NewStyle(),
		maxHeight:  densityMaxHeight,
	}
}

func (d *DensityViz) Name() string {
	return "Density Map"
}

func (d *DensityViz) Description() string {
	return "Shows audio density over time with intensity mapping"
}

func (d *DensityViz) Render(state ViewState) string {
	if len(d.data) == 0 {
		return "No data for density map."
	}

	var sb strings.Builder

	offsetSamples := int(state.Offset.Seconds() * float64(d.sampleRate))
	if offsetSamples < 0 {
		offsetSamples = 0
	}
	if offsetSamples >= len(d.data) {
		offsetSamples = len(d.data) - 1
	}

	width := state.Width
	height := state.Height - 3
	if height < 1 {
		height = 1
	}
	if height > d.maxHeight {
		height = d.maxHeight
	}

	sb.WriteString(d.renderTimeAxis(state, offsetSamples))
	sb.WriteString("\n")

	samplesPerCol := int(float64(len(d.data)) / float64(width) / state.Zoom)
	if samplesPerCol < 1 {
		samplesPerCol = 1
	}

	energyMap := make([]float64, width)
	maxEnergy := 0.0

	for col := 0; col < width; col++ {
		idx := offsetSamples + col*samplesPerCol
		if idx >= len(d.data) {
			break
		}
		sum := 0.0
		count := 0
		end := idx + samplesPerCol
		if end > len(d.data) {
			end = len(d.data)
		}
		for i := idx; i < end; i++ {
			val := d.data[i]
			sum += val * val
			count++
		}
		if count > 0 {
			rms := math.Sqrt(sum / float64(count))
			energyMap[col] = rms
			if rms > maxEnergy {
				maxEnergy = rms
			}
		}
	}

	for row := 0; row < height; row++ {
		for col := 0; col < width; col++ {
			e := energyMap[col]
			intensity := 0.0
			if maxEnergy > 0 {
				intensity = e / maxEnergy
			}
			char, color := d.getDensityChar(intensity, state.ColorScheme)
			sb.WriteString(lipgloss.NewStyle().Foreground(color).Render(char))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(d.renderIntensityScale(state))
	return sb.String()
}

func (d *DensityViz) getDensityChar(intensity float64, scheme ColorScheme) (string, lipgloss.Color) {
	chars := []string{"·", "▪", "■", "█"}

	if intensity < 0 {
		intensity = 0
	}
	if intensity > 1 {
		intensity = 1
	}
	idx := int(intensity * float64(len(chars)-1))
	char := chars[idx]

	color := getGradientColor(intensity, scheme)
	return char, color
}

func (d *DensityViz) renderTimeAxis(state ViewState, offsetSamples int) string {
	var sb strings.Builder

	totalSamples := len(d.data)
	markers := 10
	stepCol := state.Width / markers
	if stepCol < 1 {
		stepCol = 1
	}

	samplesPerCol := int(float64(totalSamples) / float64(state.Width) / state.Zoom)
	for c := 0; c < state.Width; c += stepCol {
		sampleIndex := offsetSamples + c*samplesPerCol
		if sampleIndex >= totalSamples {
			break
		}
		tSec := float64(sampleIndex) / float64(d.sampleRate)
		tstamp := time.Duration(tSec * float64(time.Second))
		timeStr := fmt.Sprintf("%02d:%02d", int(tstamp.Minutes()), int(tstamp.Seconds())%60)
		sb.WriteString(fmt.Sprintf("%-8s", timeStr))
	}
	return sb.String()
}

func (d *DensityViz) renderIntensityScale(state ViewState) string {
	var sb strings.Builder
	chars := []string{"·", "▪", "■", "█"}

	sb.WriteString("Intensity: ")
	for i, char := range chars {
		intensity := float64(i) / float64(len(chars)-1)
		color := getGradientColor(intensity, state.ColorScheme)
		sb.WriteString(lipgloss.NewStyle().Foreground(color).Render(char))
		sb.WriteString(" ")
	}
	return sb.String()
}

func (d *DensityViz) HandleInput(key string, state *ViewState) bool {
	return false
}

func (d *DensityViz) SetTotalDuration(duration time.Duration) {
	d.totalDuration = duration
}
