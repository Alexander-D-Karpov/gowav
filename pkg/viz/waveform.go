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

	// Actual duration from number of samples
	actualDuration := time.Duration(float64(len(w.data)) / float64(w.sampleRate) * float64(time.Second))
	if w.totalDuration == 0 || w.totalDuration < actualDuration {
		w.totalDuration = actualDuration
	}

	// Always clamp offset
	if state.Offset < 0 {
		state.Offset = 0
	}
	if state.Offset > w.totalDuration {
		state.Offset = w.totalDuration
	}

	var sb strings.Builder
	availWidth := state.Width
	availHeight := state.Height - 4
	if availHeight < 3 {
		availHeight = 3
	}
	if availHeight > waveformMaxHeight {
		availHeight = waveformMaxHeight
	}

	// If zoom <= 1, we want the entire track visible at once => each col covers length(w.data)/availWidth
	// If zoom > 1, we see a portion of track => samples per col = (len(w.data)/availWidth)*zoom
	// But we invert it so that a bigger zoom means fewer columns per sample => we see less of the track
	var spc float64
	if state.Zoom <= 1.0 {
		// entire track fits
		spc = float64(len(w.data)) / float64(availWidth)
	} else {
		// user is zoomed in => we see only a fraction
		spc = (float64(len(w.data)) / float64(availWidth)) * state.Zoom
	}
	if spc < 1 {
		spc = 1
	}

	// Start sample
	startSample := int(state.Offset.Seconds() * float64(w.sampleRate))
	if startSample < 0 {
		startSample = 0
	}
	if startSample >= len(w.data) {
		startSample = len(w.data) - 1
	}

	// Timeline
	sb.WriteString(w.renderTimeAxis(state, spc))
	sb.WriteString("\n")

	// Prepare buffer
	display := make([][]string, availHeight)
	for i := range display {
		display[i] = make([]string, availWidth)
		for j := range display[i] {
			display[i][j] = " "
		}
	}

	centerY := availHeight / 2
	style := lipgloss.NewStyle().Foreground(state.ColorScheme.Primary)

	for x := 0; x < availWidth; x++ {
		colStart := int(float64(startSample) + float64(x)*spc)
		if colStart >= len(w.data) {
			break
		}
		colEnd := int(float64(colStart) + spc)
		if colEnd > len(w.data) {
			colEnd = len(w.data)
		}
		if colEnd <= colStart {
			continue
		}

		minVal := 0.0
		maxVal := 0.0
		first := true
		for i := colStart; i < colEnd; i++ {
			val := w.data[i]
			if first {
				minVal = val
				maxVal = val
				first = false
			} else {
				if val < minVal {
					minVal = val
				}
				if val > maxVal {
					maxVal = val
				}
			}
		}
		minPix := int((minVal / w.maxAmp) * float64(availHeight/2-1))
		maxPix := int((maxVal / w.maxAmp) * float64(availHeight/2-1))

		pixLow := clamp(centerY+minPix, 0, availHeight-1)
		pixHigh := clamp(centerY+maxPix, 0, availHeight-1)

		for y := pixLow; y <= pixHigh; y++ {
			if y == centerY {
				display[y][x] = "─"
			} else if y == pixLow || y == pixHigh {
				display[y][x] = "█"
			} else {
				display[y][x] = "│"
			}
		}
	}

	// Output buffer
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

	curTime := formatDuration(state.Offset)
	totalTime := formatDuration(w.totalDuration)
	if state.Offset > w.totalDuration {
		curTime = totalTime
	}
	info := fmt.Sprintf(" Zoom: %.2fx | Position: %s/%s | ←/→: Scroll | +/-: Zoom | 0: Reset ",
		state.Zoom, curTime, totalTime)
	sb.WriteString(lipgloss.NewStyle().Foreground(state.ColorScheme.Text).Render(info))

	return sb.String()
}

func (w *WaveformViz) renderTimeAxis(state ViewState, spc float64) string {
	var sb strings.Builder

	// The entire track length
	actualDuration := time.Duration(float64(len(w.data)) / float64(w.sampleRate) * float64(time.Second))

	// If zoom <= 1 => we see entire track from 0..end
	// If zoom > 1 => we show offset.. offset + some portion
	var startSec float64
	var endSec float64

	if state.Zoom <= 1.0 {
		// Full track
		startSec = 0
		endSec = actualDuration.Seconds()
	} else {
		// user is zoomed in => we see offset.. portion
		startSec = state.Offset.Seconds()
		cols := float64(state.Width)
		samplesSpan := cols * spc
		durSpan := samplesSpan / float64(w.sampleRate)
		end := startSec + durSpan
		if end > actualDuration.Seconds() {
			end = actualDuration.Seconds()
		}
		endSec = end
	}

	numMarkers := state.Width / 8
	if numMarkers < 1 {
		numMarkers = 1
	}
	step := (endSec - startSec) / float64(numMarkers)
	prevPos := -1

	for i := 0; i <= numMarkers; i++ {
		t := startSec + float64(i)*step
		if t < 0 {
			t = 0
		}
		if t > actualDuration.Seconds() {
			t = actualDuration.Seconds()
		}
		label := fmt.Sprintf("%02d:%02d", int(t)/60, int(t)%60)
		pos := int(float64(i) * float64(state.Width) / float64(numMarkers))
		if pos <= prevPos {
			continue
		}
		prevPos = pos

		if i == 0 {
			sb.WriteString(fmt.Sprintf("%-8s", label))
		} else {
			padding := pos - len(sb.String())
			if padding > 0 {
				sb.WriteString(strings.Repeat(" ", padding))
				sb.WriteString(label)
			}
		}
	}
	return sb.String()
}

func (w *WaveformViz) SetTotalDuration(duration time.Duration) {
	// Compare with actual wave-based duration
	actualDuration := time.Duration(float64(len(w.data)) / float64(w.sampleRate) * float64(time.Second))
	if actualDuration > duration {
		w.totalDuration = actualDuration
	} else {
		w.totalDuration = duration
	}
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
