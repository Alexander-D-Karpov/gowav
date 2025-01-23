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
	data       []float64
	sampleRate int

	maxAmp        float64
	style         lipgloss.Style
	totalDuration time.Duration
}

func NewWaveformViz(data []float64, sampleRate int) *WaveformViz {
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
		style:      lipgloss.NewStyle(),
	}
}

func (w *WaveformViz) Name() string {
	return "Waveform"
}

func (w *WaveformViz) Description() string {
	return "Displays the audio waveform with amplitude over time"
}

func (w *WaveformViz) Render(state ViewState) string {
	if len(w.data) == 0 {
		return "No data for waveform."
	}

	var sb strings.Builder

	offsetSamples := int(state.Offset.Seconds() * float64(w.sampleRate))
	if offsetSamples < 0 {
		offsetSamples = 0
	}
	if offsetSamples >= len(w.data) {
		offsetSamples = len(w.data) - 1
	}

	totalSamples := len(w.data)
	samplesPerCol := int(float64(totalSamples) / float64(state.Width) / state.Zoom)
	if samplesPerCol < 1 {
		samplesPerCol = 1
	}

	height := state.Height - 4
	if height < 1 {
		height = 1
	}
	if height > waveformMaxHeight {
		height = waveformMaxHeight
	}

	sb.WriteString(w.renderTimeAxis(state, offsetSamples, samplesPerCol))
	sb.WriteString("\n")

	waveform := make([][]string, height)
	for i := range waveform {
		waveform[i] = make([]string, state.Width)
		for j := range waveform[i] {
			waveform[i][j] = " "
		}
	}

	halfHeight := height / 2

	for col := 0; col < state.Width; col++ {
		idx := offsetSamples + col*samplesPerCol
		if idx >= totalSamples {
			break
		}

		minAmp := 0.0
		maxAmp := 0.0
		end := idx + samplesPerCol
		if end > totalSamples {
			end = totalSamples
		}
		for i := idx; i < end; i++ {
			val := w.data[i]
			if val < minAmp {
				minAmp = val
			}
			if val > maxAmp {
				maxAmp = val
			}
		}
		minPos := int((minAmp / w.maxAmp) * float64(halfHeight-1))
		maxPos := int((maxAmp / w.maxAmp) * float64(halfHeight-1))

		center := halfHeight
		top := center + maxPos
		bottom := center + minPos

		for row := 0; row < height; row++ {
			char := " "
			intensity := 0.0
			if row == center {
				char = "─"
				intensity = 0.3
			} else if row == top || row == bottom {
				char = "█"
				intensity = 0.8
			} else if (row > center && row < top) || (row < center && row > bottom) {
				char = "│"
				intensity = 0.5
			}

			if char != " " {
				color := getGradientColor(intensity, state.ColorScheme)
				waveform[row][col] = lipgloss.NewStyle().Foreground(color).Render(char)
			}
		}
	}

	for row := 0; row < height; row++ {
		sb.WriteString(strings.Join(waveform[row], ""))
		sb.WriteString("\n")
	}

	info := fmt.Sprintf(" Zoom: %.2fx | Offset: %s | ←/→: Scroll | +/-: Zoom | 0: Reset ",
		state.Zoom, state.Offset.String())
	sb.WriteString(lipgloss.NewStyle().Foreground(state.ColorScheme.Text).Render(info))

	return sb.String()
}

func (w *WaveformViz) renderTimeAxis(state ViewState, offsetSamples, samplesPerCol int) string {
	var sb strings.Builder

	markers := 10
	stepCol := state.Width / markers
	if stepCol < 1 {
		stepCol = 1
	}

	for i := 0; i < state.Width; i += stepCol {
		sampleIndex := offsetSamples + i*samplesPerCol
		if sampleIndex >= len(w.data) {
			break
		}
		sec := float64(sampleIndex) / float64(w.sampleRate)
		tstamp := time.Duration(sec * float64(time.Second))
		timeStr := fmt.Sprintf("%02d:%02d", int(tstamp.Minutes()), int(tstamp.Seconds())%60)
		sb.WriteString(fmt.Sprintf("%-7s", timeStr))
	}

	return sb.String()
}

func (w *WaveformViz) HandleInput(key string, state *ViewState) bool {
	return false
}

func (w *WaveformViz) SetTotalDuration(duration time.Duration) {
	w.totalDuration = duration
}
