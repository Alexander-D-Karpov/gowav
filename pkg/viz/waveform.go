package viz

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"math"
	"strings"
	"time"
)

const waveformMaxHeight = 40

type WaveformViz struct {
	data          []float64
	sampleRate    int
	maxAmp        float64
	totalDuration time.Duration
}

func CreateWaveformViz(data []float64, sampleRate int) Visualization {
	maxAmp := 0.0
	for _, v := range data {
		a := math.Abs(v)
		if a > maxAmp {
			maxAmp = a
		}
	}

	return &WaveformViz{
		data:       data,
		sampleRate: sampleRate,
		maxAmp:     maxAmp,
	}
}

func (w *WaveformViz) Render(state ViewState) string {
	if len(w.data) == 0 {
		return "No data for waveform."
	}

	var sb strings.Builder

	// Always use full width
	availWidth := state.Width
	availHeight := state.Height - 4
	if availHeight < 3 {
		availHeight = 3
	}
	if availHeight > waveformMaxHeight {
		availHeight = waveformMaxHeight
	}

	// Calculate samples per column based on zoom
	samplesPerCol := int(float64(w.sampleRate) / float64(availWidth) / state.Zoom)
	if samplesPerCol < 1 {
		samplesPerCol = 1
	}

	startSample := int(state.Offset.Seconds() * float64(w.sampleRate))
	startSample = clamp(startSample, 0, len(w.data)-1)

	// Render timeline
	sb.WriteString(w.renderTimeAxis(state))
	sb.WriteString("\n")

	// Initialize display buffer
	display := make([][]string, availHeight)
	for i := range display {
		display[i] = make([]string, availWidth)
		for j := range display[i] {
			display[i][j] = " "
		}
	}

	centerY := availHeight / 2
	style := lipgloss.NewStyle().Foreground(state.ColorScheme.Primary)

	// For each column, find min and max amplitude
	for x := 0; x < availWidth; x++ {
		startIdx := startSample + (x * samplesPerCol)
		if startIdx >= len(w.data) {
			break
		}

		// Find min and max values for this column
		var minVal, maxVal float64
		for i := 0; i < samplesPerCol && (startIdx+i) < len(w.data); i++ {
			val := w.data[startIdx+i]
			if val < minVal {
				minVal = val
			}
			if val > maxVal {
				maxVal = val
			}
		}

		// Scale to display height
		minHeight := int((minVal / w.maxAmp) * float64(availHeight/2-1))
		maxHeight := int((maxVal / w.maxAmp) * float64(availHeight/2-1))

		// Draw the waveform
		minY := centerY + minHeight
		maxY := centerY + maxHeight

		// Ensure proper boundaries
		minY = clamp(minY, 0, availHeight-1)
		maxY = clamp(maxY, 0, availHeight-1)

		// Draw vertical line
		for y := minY; y <= maxY; y++ {
			if y == centerY {
				display[y][x] = "─"
			} else if y == minY || y == maxY {
				display[y][x] = "█"
			} else {
				display[y][x] = "│"
			}
		}
	}

	// Render the display buffer
	for y := 0; y < availHeight; y++ {
		for x := 0; x < availWidth; x++ {
			if display[y][x] != " " {
				sb.WriteString(style.Render(display[y][x]))
			} else {
				sb.WriteString(" ")
			}
		}
		sb.WriteString("\n")
	}

	// Add control info
	curTime := formatDuration(state.Offset)
	totalTime := formatDuration(w.totalDuration)
	info := fmt.Sprintf(" Zoom: %.2fx | Position: %s/%s | ←/→: Scroll | +/-: Zoom | 0: Reset ",
		state.Zoom, curTime, totalTime)
	sb.WriteString(lipgloss.NewStyle().Foreground(state.ColorScheme.Text).Render(info))

	return sb.String()
}

func (w *WaveformViz) renderTimeAxis(state ViewState) string {
	var sb strings.Builder
	visibleTime := w.totalDuration.Seconds() / state.Zoom
	startTime := state.Offset.Seconds()
	numMarkers := state.Width / 8
	if numMarkers < 1 {
		numMarkers = 1
	}
	timeStep := visibleTime / float64(numMarkers)

	prevPos := -1
	for i := 0; i < numMarkers; i++ {
		timePos := startTime + (float64(i) * timeStep)
		if timePos > w.totalDuration.Seconds() {
			break
		}

		timeStr := fmt.Sprintf("%02d:%02d",
			int(timePos)/60,
			int(timePos)%60)

		pos := int(float64(i) * float64(state.Width) / float64(numMarkers))
		if pos <= prevPos {
			continue
		}
		prevPos = pos

		if i == 0 {
			sb.WriteString(fmt.Sprintf("%-8s", timeStr))
		} else {
			padding := pos - len(sb.String())
			if padding > 0 {
				sb.WriteString(strings.Repeat(" ", padding))
			}
			sb.WriteString(timeStr)
		}
	}
	return sb.String()
}

func (w *WaveformViz) SetTotalDuration(duration time.Duration) {
	w.totalDuration = duration
}

func (w *WaveformViz) Name() string {
	return "Waveform"
}

func (w *WaveformViz) Description() string {
	return "Audio waveform visualization"
}

func (w *WaveformViz) HandleInput(string, *ViewState) bool {
	return false
}
