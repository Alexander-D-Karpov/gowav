package viz

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"strings"
	"time"
)

const spectrogramMaxHeight = 40

type SpectrogramViz struct {
	fftData    [][]float64 // timeSteps x freqBands
	freqBands  []float64
	sampleRate int

	maxEnergy     float64
	style         lipgloss.Style
	totalDuration time.Duration
}

func NewSpectrogramViz(fftData [][]float64, freqBands []float64, sampleRate int) *SpectrogramViz {
	maxEnergy := 0.0
	for _, timeSlice := range fftData {
		for _, energy := range timeSlice {
			if energy > maxEnergy {
				maxEnergy = energy
			}
		}
	}

	return &SpectrogramViz{
		fftData:    fftData,
		freqBands:  freqBands,
		sampleRate: sampleRate,
		maxEnergy:  maxEnergy,
		style:      lipgloss.NewStyle(),
	}
}

func (s *SpectrogramViz) Name() string {
	return "Spectrogram"
}

func (s *SpectrogramViz) Description() string {
	return "Displays frequency content over time with color intensity"
}

func (s *SpectrogramViz) Render(state ViewState) string {
	if len(s.fftData) == 0 || len(s.fftData[0]) == 0 {
		return "No spectrogram data"
	}

	var sb strings.Builder

	offsetTimeSteps := int(state.Offset.Seconds() * float64(s.sampleRate) / 1024.0)
	if offsetTimeSteps < 0 {
		offsetTimeSteps = 0
	}
	if offsetTimeSteps >= len(s.fftData) {
		offsetTimeSteps = len(s.fftData) - 1
	}

	numTimeSteps := len(s.fftData)
	width := state.Width
	height := state.Height - 4
	if height < 1 {
		height = 1
	}
	if height > spectrogramMaxHeight {
		height = spectrogramMaxHeight
	}

	timeStepsPerCol := int(float64(numTimeSteps) / float64(width) / state.Zoom)
	if timeStepsPerCol < 1 {
		timeStepsPerCol = 1
	}

	numFreqBands := len(s.freqBands)
	if height > numFreqBands {
		height = numFreqBands
	}

	sb.WriteString(s.renderFrequencyAxis(height))
	sb.WriteString("\n")

	for row := 0; row < height; row++ {
		freqIdx := numFreqBands - 1 - row

		for col := 0; col < width; col++ {
			timeIdx := offsetTimeSteps + col*timeStepsPerCol
			if timeIdx >= numTimeSteps {
				break
			}
			sum := 0.0
			count := 0
			for i := 0; i < timeStepsPerCol && timeIdx+i < numTimeSteps; i++ {
				sum += s.fftData[timeIdx+i][freqIdx]
				count++
			}
			energy := sum / float64(count)
			intensity := 0.0
			if s.maxEnergy > 0 {
				intensity = energy / s.maxEnergy
			}
			color := getSpectrogramColor(intensity, state.ColorScheme)
			sb.WriteString(lipgloss.NewStyle().Background(color).Render(" "))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(s.renderTimeAxis(state, offsetTimeSteps, timeStepsPerCol))
	return sb.String()
}

func (s *SpectrogramViz) renderFrequencyAxis(rows int) string {
	var sb strings.Builder
	step := len(s.freqBands) / rows
	if step < 1 {
		step = 1
	}
	for i := 0; i < rows; i++ {
		idx := len(s.freqBands) - 1 - i*step
		if idx < 0 {
			break
		}
		freq := s.freqBands[idx]
		label := ""
		if freq >= 1000 {
			label = fmt.Sprintf("%3.1fkHz", freq/1000)
		} else {
			label = fmt.Sprintf("%4.0fHz", freq)
		}
		sb.WriteString(fmt.Sprintf("%-7s ", label))
	}
	return sb.String()
}

func (s *SpectrogramViz) renderTimeAxis(state ViewState, offsetTimeSteps, timeStepsPerCol int) string {
	var sb strings.Builder

	totalTimeSteps := len(s.fftData)
	secPerStep := 1024.0 / float64(s.sampleRate)

	markers := 10
	stepCol := state.Width / markers
	if stepCol < 1 {
		stepCol = 1
	}
	for c := 0; c < state.Width; c += stepCol {
		timeIdx := offsetTimeSteps + c*timeStepsPerCol
		if timeIdx >= totalTimeSteps {
			break
		}
		tSec := float64(timeIdx) * secPerStep
		tstamp := time.Duration(tSec * float64(time.Second))
		timeStr := fmt.Sprintf("%02d:%02d", int(tstamp.Minutes()), int(tstamp.Seconds())%60)
		sb.WriteString(fmt.Sprintf("%-7s", timeStr))
	}
	return sb.String()
}

func (s *SpectrogramViz) HandleInput(key string, state *ViewState) bool {
	return false
}

func (s *SpectrogramViz) SetTotalDuration(duration time.Duration) {
	s.totalDuration = duration
}

func getSpectrogramColor(intensity float64, scheme ColorScheme) lipgloss.Color {
	if intensity < 0 {
		intensity = 0
	}
	if intensity > 1 {
		intensity = 1
	}
	if intensity < 0.5 {
		t := intensity * 2
		r1, g1, b1 := hexToRGB(string(scheme.Background))
		r2, g2, b2 := hexToRGB(string(scheme.Primary))
		r := int(float64(r1) + t*float64(r2-r1))
		g := int(float64(g1) + t*float64(g2-g1))
		b := int(float64(b1) + t*float64(b2-b1))
		return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", r, g, b))
	}
	t := (intensity - 0.5) * 2
	r1, g1, b1 := hexToRGB(string(scheme.Primary))
	r2, g2, b2 := hexToRGB(string(scheme.Accent))
	r := int(float64(r1) + t*float64(r2-r1))
	g := int(float64(g1) + t*float64(g2-g1))
	b := int(float64(b1) + t*float64(b2-b1))
	return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", r, g, b))
}
