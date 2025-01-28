package viz

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

const waveformMaxHeight = 40

type WaveformViz struct {
	data          []float64
	sampleRate    int
	maxAmp        float64
	totalDuration time.Duration
}

func CreateWaveformViz(data []float64, sampleRate int) Visualization {
	// Find peak amplitude
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

	// Compute actual audio length from sample count
	actualDuration := time.Duration(float64(len(w.data)) / float64(w.sampleRate) * float64(time.Second))
	if w.totalDuration == 0 || w.totalDuration < actualDuration {
		w.totalDuration = actualDuration
	}

	// Clamp offset in [0..w.totalDuration]
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

	// Number of total samples in track
	totalSamples := len(w.data)

	// 1) Calculate how many samples we can display in the current zoom level
	//    If zoom = 1.0 => entire track fits in the screen
	//    If zoom > 1.0 => we see a smaller portion
	//    If zoom < 1.0 => see entire track (like “zoom out”).
	//
	// We'll define "samplesPerScreen" as totalSamples / zoom, but clamp it to at least availWidth.
	var samplesPerScreen float64
	if state.Zoom <= 1.0 {
		samplesPerScreen = float64(totalSamples)
	} else {
		// user is zoomed in
		samplesPerScreen = float64(totalSamples) / state.Zoom
		// never show fewer columns than screen width. If you want infinite zoom, you can remove this:
		if samplesPerScreen < float64(availWidth) {
			samplesPerScreen = float64(availWidth)
		}
	}
	if samplesPerScreen > float64(totalSamples) {
		samplesPerScreen = float64(totalSamples)
	}

	// 2) Convert offset from time -> sample index
	//    offsetSamples is how far in the track we've scrolled.
	offsetSamples := int(state.Offset.Seconds() * float64(w.sampleRate))
	if offsetSamples < 0 {
		offsetSamples = 0
	}
	if offsetSamples >= totalSamples {
		offsetSamples = totalSamples - 1
	}

	// 3) The end sample is offsetSamples + samplesPerScreen
	endSampleF := float64(offsetSamples) + samplesPerScreen
	if endSampleF > float64(totalSamples) {
		endSampleF = float64(totalSamples)
	}
	// how many samples we are actually displaying
	displayedSamples := endSampleF - float64(offsetSamples)

	// 4) “spc” = how many actual audio samples per 1 column
	spc := displayedSamples / float64(availWidth)
	if spc < 1.0 {
		spc = 1.0
	}

	// Render the top timeline for the portion [offset..offset+displayedDuration]
	sb.WriteString(w.renderTimeAxis(state, offsetSamples, displayedSamples, spc))
	sb.WriteString("\n")

	// Prepare a 2D text buffer
	display := make([][]string, availHeight)
	for i := 0; i < availHeight; i++ {
		display[i] = make([]string, availWidth)
		for j := 0; j < availWidth; j++ {
			display[i][j] = " "
		}
	}

	centerY := availHeight / 2
	style := lipgloss.NewStyle().Foreground(state.ColorScheme.Primary)

	// For each column in terminal
	for x := 0; x < availWidth; x++ {
		// colStart is the first sample for this column
		colStart := int(float64(offsetSamples) + float64(x)*spc)
		if colStart >= totalSamples {
			break
		}
		colEnd := int(float64(colStart) + spc)
		if colEnd > totalSamples {
			colEnd = totalSamples
		}
		if colEnd <= colStart {
			continue
		}

		// find min & max in that slice
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

		// scale to vertical
		half := availHeight / 2
		minPix := int((minVal / w.maxAmp) * float64(half-1))
		maxPix := int((maxVal / w.maxAmp) * float64(half-1))

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

	// Write out the buffer
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

	// Show info line with offset/total
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

// renderTimeAxis displays timeline markers from the *current offset in samples*.
func (w *WaveformViz) renderTimeAxis(state ViewState, offsetSamples int, displayedSamples float64, spc float64) string {
	var sb strings.Builder

	// total track length in seconds
	trackSec := w.totalDuration.Seconds()

	// how many seconds are we displaying on screen?
	displayedSec := displayedSamples / float64(w.sampleRate)

	// the start time in seconds
	startSec := float64(offsetSamples) / float64(w.sampleRate)
	// the end time in seconds
	endSec := startSec + displayedSec
	if endSec > trackSec {
		endSec = trackSec
	}
	if startSec < 0 {
		startSec = 0
	}
	if startSec > trackSec {
		startSec = trackSec
	}

	// Number of timeline markers
	numMarkers := state.Width / 8
	if numMarkers < 1 {
		numMarkers = 1
	}
	span := endSec - startSec
	step := span / float64(numMarkers)

	prevPos := -1
	for i := 0; i <= numMarkers; i++ {
		t := startSec + float64(i)*step
		if t < 0 {
			t = 0
		}
		if t > trackSec {
			t = trackSec
		}
		mm := int(t) / 60
		ss := int(t) % 60
		label := fmt.Sprintf("%02d:%02d", mm, ss)

		// approximate the column position for this time
		// i from 0..numMarkers => x from 0..width
		// x = ( (t-startSec)/span ) * width
		if span == 0 {
			// entire track is presumably zero-length or corner case
			break
		}
		fraction := (t - startSec) / span
		pos := int(fraction * float64(state.Width))
		if pos <= prevPos {
			continue
		}
		prevPos = pos

		if i == 0 {
			sb.WriteString(fmt.Sprintf("%-8s", label))
		} else {
			pad := pos - len(sb.String())
			if pad > 0 {
				sb.WriteString(strings.Repeat(" ", pad))
			}
			sb.WriteString(label)
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
