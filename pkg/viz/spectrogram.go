package viz

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"math"
	"strings"
	"time"
)

type SpectrogramViz struct {
	fftData       [][]float64
	freqBands     []float64
	sampleRate    int
	totalDuration time.Duration
}

func NewSpectrogramViz(fftData [][]float64, freqs []float64, rate int) *SpectrogramViz {
	return &SpectrogramViz{
		fftData:    fftData,
		freqBands:  freqs,
		sampleRate: rate,
	}
}

func (s *SpectrogramViz) Render(st ViewState) string {
	if len(s.fftData) == 0 || len(s.freqBands) == 0 {
		return "No spectrogram data"
	}
	var sb strings.Builder

	// Terminal bounds
	graphHeight := st.Height - 6
	if graphHeight < 4 {
		graphHeight = 4
	}
	if graphHeight > 50 {
		graphHeight = 50
	}

	// Fixed frequency label margin
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
	framesPerCol := int(float64(numFrames) / float64(graphWidth) / st.Zoom)
	if framesPerCol < 1 {
		framesPerCol = 1
	}

	startFrame := int((st.Offset.Seconds() / s.totalDuration.Seconds()) * float64(numFrames))
	startFrame = clamp(startFrame, 0, numFrames-1)

	rows := graphHeight
	if rows > len(s.freqBands) {
		rows = len(s.freqBands)
	}

	sb.WriteString("Spectrogram (dB):\n")

	// Pre-allocate and initialize all lines
	lines := make([]strings.Builder, rows)

	// Process each row
	for row := 0; row < rows; row++ {
		freqIdx := len(s.freqBands) - 1 - int((float64(row)/float64(rows))*float64(len(s.freqBands)))
		if freqIdx < 0 {
			freqIdx = 0
		}

		// Format frequency label with proper alignment
		freqVal := s.freqBands[freqIdx]
		var freqLabel string
		if freqVal >= 1000 {
			freqLabel = fmt.Sprintf("%5.1fk", freqVal/1000)
		} else {
			freqLabel = fmt.Sprintf("%6.0f", freqVal)
		}
		lines[row].WriteString(freqLabel + " ")

		// Build row content
		for col := 0; col < graphWidth; col++ {
			frame := startFrame + col*framesPerCol
			if frame >= numFrames {
				lines[row].WriteByte(' ')
				continue
			}

			amp := s.fftData[frame][freqIdx]
			if amp < 1e-12 {
				amp = 1e-12
			}

			// Convert to dB
			dbVal := 20 * math.Log10(amp)
			if dbVal < dbMin {
				dbVal = dbMin
			}
			if dbVal > dbMax {
				dbVal = dbMax
			}

			// Map to color
			ratio := (dbVal - dbMin) / (dbMax - dbMin)
			cIndex := int(ratio * float64(len(colors)-1))
			cIndex = clamp(cIndex, 0, len(colors)-1)

			// Render colored block
			style := lipgloss.NewStyle().
				Background(colors[cIndex]).
				Foreground(colors[cIndex])
			lines[row].WriteString(style.Render("█"))
		}
	}

	// Combine all lines
	for i := 0; i < rows; i++ {
		sb.WriteString(lines[i].String())
		sb.WriteString("\n")
	}

	// Add time axis
	sb.WriteString(s.renderTimeAxis(graphWidth, framesPerCol, startFrame))
	sb.WriteString("\n")

	// Add color legend
	sb.WriteString(s.renderLegend(colors))

	return sb.String()
}

func (s *SpectrogramViz) renderTimeAxis(width, framesPerCol, startFrame int) string {
	var b strings.Builder
	numFrames := len(s.fftData)
	secPerFrame := s.totalDuration.Seconds() / float64(numFrames)

	b.WriteString("Time: ")
	markers := 8
	for i := 0; i <= markers; i++ {
		x := float64(i) * float64(width) / float64(markers)
		frame := startFrame + int(x)*framesPerCol
		if frame >= numFrames {
			frame = numFrames - 1
		}

		tSec := float64(frame) * secPerFrame
		if tSec < 0 {
			tSec = 0
		}
		if tSec > s.totalDuration.Seconds() {
			tSec = s.totalDuration.Seconds()
		}

		minu := int(tSec) / 60
		seco := int(tSec) % 60
		label := fmt.Sprintf("%02d:%02d", minu, seco)

		if i == 0 {
			b.WriteString(label)
		} else {
			pad := int(x) - (len(b.String()) - 6)
			if pad > 0 {
				b.WriteString(strings.Repeat(" ", pad))
			}
			b.WriteString(label)
		}
	}
	return b.String()
}

func (s *SpectrogramViz) renderLegend(cols []lipgloss.Color) string {
	var b strings.Builder
	b.WriteString("dB: ")
	for _, c := range cols {
		sty := lipgloss.NewStyle().Background(c).Foreground(c)
		b.WriteString(sty.Render(" "))
	}
	b.WriteString(" (quiet → loud)")
	return b.String()
}

func (s *SpectrogramViz) renderColorScale(colors []lipgloss.Color) string {
	var sb strings.Builder
	sb.WriteString("dB: ")

	for _, color := range colors {
		style := lipgloss.NewStyle().
			Background(color).
			Foreground(color)
		sb.WriteString(style.Render(" "))
	}
	sb.WriteString(" (quiet → loud)")
	return sb.String()
}

func (s *SpectrogramViz) Name() string {
	return "Spectrogram"
}

func (s *SpectrogramViz) Description() string {
	return "Frequency content over time, dB scale"
}

func (s *SpectrogramViz) SetTotalDuration(d time.Duration) {
	s.totalDuration = d
}

func (s *SpectrogramViz) HandleInput(string, *ViewState) bool {
	return false
}
