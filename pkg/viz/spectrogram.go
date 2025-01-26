package viz

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// SpectrogramViz holds data for the spectrogram view.
type SpectrogramViz struct {
	fftData       [][]float64
	freqBands     []float64
	sampleRate    int
	totalDuration time.Duration
}

// NewSpectrogramViz prepares the spectrogram with raw FFT data.
func NewSpectrogramViz(fftData [][]float64, freqs []float64, rate int) *SpectrogramViz {
	return &SpectrogramViz{
		fftData:    fftData,
		freqBands:  freqs,
		sampleRate: rate,
	}
}

// Name of the visualization.
func (s *SpectrogramViz) Name() string {
	return "Spectrogram"
}

// Description is a short tagline.
func (s *SpectrogramViz) Description() string {
	return "Frequency content over time, dB scale"
}

// SetTotalDuration sets how long the track is, for time axis.
func (s *SpectrogramViz) SetTotalDuration(d time.Duration) {
	s.totalDuration = d
}

// HandleInput is unused here.
func (s *SpectrogramViz) HandleInput(_ string, _ *ViewState) bool {
	return false
}

// Render draws the spectrogram with vertical freq labels, time axis, color map, and dB amplitude.
func (s *SpectrogramViz) Render(st ViewState) string {
	if len(s.fftData) == 0 || len(s.freqBands) == 0 {
		return "No spectrogram data"
	}
	var sb strings.Builder

	// Terminal bounds, leaving space for bottom/time axis + legend
	// Also limit max spectrogram height
	graphHeight := st.Height - 6
	if graphHeight < 4 {
		graphHeight = 4
	}
	if graphHeight > 50 {
		graphHeight = 50
	}
	// Reserve some columns for freq axis
	freqMargin := 8
	graphWidth := st.Width - freqMargin
	if graphWidth < 8 {
		graphWidth = 8
	}

	// DB scale range
	dbMin := -90.0
	dbMax := 0.0

	// Color gradient from quiet (blue/purple) to loud (bright)
	colors := []lipgloss.Color{
		"#000040", "#000080", "#0000c0", "#0000ff", "#4000ff", "#8000ff", "#c000ff",
		"#ff00c0", "#ff0080", "#ff0040", "#ff0000", "#ff4000", "#ff8000", "#ffbf00",
		"#ffff00", "#ffffff",
	}

	numFrames := len(s.fftData)
	// Horizontal pixel => frames
	framesPerCol := int(float64(numFrames) / float64(graphWidth) / st.Zoom)
	if framesPerCol < 1 {
		framesPerCol = 1
	}

	// Time offset
	startFrame := int((st.Offset.Seconds() / s.totalDuration.Seconds()) * float64(numFrames))
	if startFrame < 0 {
		startFrame = 0
	}
	if startFrame >= numFrames {
		startFrame = numFrames - 1
	}

	// Each row => one freq index, top freq first
	rows := graphHeight
	if rows > len(s.freqBands) {
		rows = len(s.freqBands)
	}

	sb.WriteString("Spectrogram (dB):\n")

	lines := make([]string, rows)
	for row := 0; row < rows; row++ {
		// Map row to freq index linearly
		freqIdx := len(s.freqBands) - 1 - int((float64(row)/float64(rows))*float64(len(s.freqBands)))
		if freqIdx < 0 {
			freqIdx = 0
		}
		freqVal := s.freqBands[freqIdx]
		var freqLabel string
		if freqVal >= 1000 {
			freqLabel = fmt.Sprintf("%4.1fk", freqVal/1000)
		} else {
			freqLabel = fmt.Sprintf("%5.0f", freqVal)
		}
		// Start line with freq label
		lineBuilder := strings.Builder{}
		fmt.Fprintf(&lineBuilder, "%6s ", freqLabel)

		// Build row of colored blocks
		for col := 0; col < graphWidth; col++ {
			frame := startFrame + col*framesPerCol
			if frame >= numFrames {
				lineBuilder.WriteByte(' ')
				continue
			}
			amp := s.fftData[frame][freqIdx]
			if amp < 1e-12 {
				amp = 1e-12
			}
			// Convert amplitude -> dB
			dbVal := 20 * math.Log10(amp)
			if dbVal < dbMin {
				dbVal = dbMin
			}
			if dbVal > dbMax {
				dbVal = dbMax
			}
			ratio := (dbVal - dbMin) / (dbMax - dbMin)
			cIndex := int(ratio * float64(len(colors)-1))
			if cIndex < 0 {
				cIndex = 0
			}
			if cIndex >= len(colors) {
				cIndex = len(colors) - 1
			}
			style := lipgloss.NewStyle().Foreground(colors[cIndex]).Background(colors[cIndex])
			lineBuilder.WriteString(style.Render("█"))
		}
		lines[row] = lineBuilder.String()
	}
	sb.WriteString(strings.Join(lines, "\n"))
	sb.WriteString("\n")

	// Add time axis
	sb.WriteString(s.renderTimeAxis(graphWidth, framesPerCol, startFrame))
	sb.WriteString("\n")
	// Add color legend
	sb.WriteString(s.renderLegend(colors))
	return sb.String()
}

func (s *SpectrogramViz) renderTimeAxis(cols, framesPerCol, startFrame int) string {
	var b strings.Builder
	numFrames := len(s.fftData)
	secPerFrame := s.totalDuration.Seconds() / float64(numFrames)

	b.WriteString("Time: ")
	markers := 8
	for i := 0; i <= markers; i++ {
		x := float64(i) * float64(cols) / float64(markers)
		frame := startFrame + int(x)*framesPerCol
		if frame >= numFrames {
			frame = numFrames - 1
		}
		tSec := float64(frame) * secPerFrame
		if tSec < 0 {
			tSec = 0
		}
		minu := int(tSec) / 60
		seco := int(tSec) % 60
		label := fmt.Sprintf("%02d:%02d", minu, seco)
		pad := int(x) - (len(b.String()) - 6)
		if pad > 0 {
			b.WriteString(strings.Repeat(" ", pad))
		}
		b.WriteString(label)
	}
	return b.String()
}

func (s *SpectrogramViz) renderLegend(cols []lipgloss.Color) string {
	var b strings.Builder
	b.WriteString("dB scale: ")
	for _, c := range cols {
		sty := lipgloss.NewStyle().Background(c).Foreground(c)
		b.WriteString(sty.Render(" "))
	}
	b.WriteString(" (quiet → loud)")
	return b.String()
}
