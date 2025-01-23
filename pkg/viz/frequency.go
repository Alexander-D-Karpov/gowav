package viz

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"sort"
	"strings"
	"time"
)

const freqMaxHeight = 40

type FrequencyViz struct {
	freqBands     []float64
	freqData      [][]float64
	sampleRate    int
	maxAmplitude  float64
	style         lipgloss.Style
	totalDuration time.Duration
}

func NewFrequencyViz(freqBands []float64, freqData [][]float64, sampleRate int) *FrequencyViz {
	maxAmp := 0.0
	for _, slice := range freqData {
		for _, amp := range slice {
			if amp > maxAmp {
				maxAmp = amp
			}
		}
	}

	return &FrequencyViz{
		freqBands:    freqBands,
		freqData:     freqData,
		sampleRate:   sampleRate,
		maxAmplitude: maxAmp,
		style:        lipgloss.NewStyle(),
	}
}

func (f *FrequencyViz) Name() string {
	return "Frequency Analysis"
}

func (f *FrequencyViz) Description() string {
	return "Shows frequency distribution and peaks over time"
}

func (f *FrequencyViz) Render(state ViewState) string {
	if len(f.freqData) == 0 || len(f.freqData[0]) == 0 {
		return "No frequency data"
	}

	var sb strings.Builder

	offsetTimeSteps := int(state.Offset.Seconds() * float64(f.sampleRate) / 1024.0)
	if offsetTimeSteps < 0 {
		offsetTimeSteps = 0
	}
	if offsetTimeSteps >= len(f.freqData) {
		offsetTimeSteps = len(f.freqData) - 1
	}

	width := state.Width
	height := state.Height - 4
	if height < 1 {
		height = 1
	}
	if height > freqMaxHeight {
		height = freqMaxHeight
	}

	numTimeSteps := len(f.freqData)
	samplesPerCol := int(float64(numTimeSteps) / float64(width) / state.Zoom)
	if samplesPerCol < 1 {
		samplesPerCol = 1
	}

	bands := len(f.freqBands)
	if height > bands {
		height = bands
	}

	// Time axis
	sb.WriteString(f.renderTimeAxis(state, offsetTimeSteps, samplesPerCol))
	sb.WriteString("\n")

	// from top freq to bottom freq
	for row := 0; row < height; row++ {
		freqIdx := bands - 1 - row
		freq := f.freqBands[freqIdx]
		var label string
		if freq >= 1000 {
			label = fmt.Sprintf("%4.1fk ", freq/1000)
		} else {
			label = fmt.Sprintf("%4.0f  ", freq)
		}
		sb.WriteString(label + "│")

		for col := 0; col < width; col++ {
			timeIdx := offsetTimeSteps + col*samplesPerCol
			if timeIdx >= numTimeSteps {
				break
			}
			sum := 0.0
			count := 0
			for i := timeIdx; i < timeIdx+samplesPerCol && i < numTimeSteps; i++ {
				sum += f.freqData[i][freqIdx]
				count++
			}
			amp := 0.0
			if count > 0 {
				amp = sum / float64(count)
			}
			intensity := 0.0
			if f.maxAmplitude > 0 {
				intensity = amp / f.maxAmplitude
			}
			char, color := f.getFrequencyChar(intensity, state.ColorScheme)
			sb.WriteString(lipgloss.NewStyle().Foreground(color).Render(char))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(f.renderPeakFrequencies(offsetTimeSteps, samplesPerCol))
	return sb.String()
}

func (f *FrequencyViz) getFrequencyChar(intensity float64, scheme ColorScheme) (string, lipgloss.Color) {
	chars := []string{"·", ":", "⋮", "‖", "█"}

	if intensity < 0 {
		intensity = 0
	}
	if intensity > 1 {
		intensity = 1
	}
	idx := int(intensity * float64(len(chars)-1))
	if idx < 0 {
		idx = 0
	}
	if idx >= len(chars) {
		idx = len(chars) - 1
	}

	char := chars[idx]
	color := getGradientColor(intensity, scheme)
	return char, color
}

func (f *FrequencyViz) renderTimeAxis(state ViewState, offsetTimeSteps, samplesPerCol int) string {
	var sb strings.Builder
	totalSteps := len(f.freqData)
	step := state.Width / 10
	if step < 1 {
		step = 1
	}

	secondsPerStep := 1024.0 / float64(f.sampleRate)
	for c := 0; c < state.Width; c += step {
		timeIdx := offsetTimeSteps + c*samplesPerCol
		if timeIdx >= totalSteps {
			break
		}
		tSec := float64(timeIdx) * secondsPerStep
		tstamp := time.Duration(tSec * float64(time.Second))
		timeStr := fmt.Sprintf("%02d:%02d", int(tstamp.Minutes()), int(tstamp.Seconds())%60)
		sb.WriteString(fmt.Sprintf("%-7s", timeStr))
	}
	return sb.String()
}

func (f *FrequencyViz) renderPeakFrequencies(offsetTimeSteps, samplesPerCol int) string {
	var sb strings.Builder
	sb.WriteString("\nPeak frequencies: ")

	timeIdxEnd := offsetTimeSteps + samplesPerCol
	if timeIdxEnd > len(f.freqData) {
		timeIdxEnd = len(f.freqData)
	}

	peakFreqs := make(map[int]float64)
	for i := offsetTimeSteps; i < timeIdxEnd; i++ {
		if i >= len(f.freqData) {
			break
		}
		maxAmp := 0.0
		peakIdx := 0
		for j, amp := range f.freqData[i] {
			if amp > maxAmp {
				maxAmp = amp
				peakIdx = j
			}
		}
		peakFreqs[peakIdx] += maxAmp
	}

	type freqPeak struct {
		freq  float64
		power float64
	}
	var peaks []freqPeak
	for idx, power := range peakFreqs {
		peaks = append(peaks, freqPeak{f.freqBands[idx], power})
	}
	sort.Slice(peaks, func(i, j int) bool {
		return peaks[i].power > peaks[j].power
	})

	top := 3
	if len(peaks) < 3 {
		top = len(peaks)
	}
	for i := 0; i < top; i++ {
		if i >= len(peaks) {
			break
		}
		if peaks[i].freq >= 1000 {
			sb.WriteString(fmt.Sprintf("%.1fkHz ", peaks[i].freq/1000))
		} else {
			sb.WriteString(fmt.Sprintf("%.0fHz ", peaks[i].freq))
		}
	}
	sb.WriteString("\n")
	return sb.String()
}

func (f *FrequencyViz) HandleInput(key string, state *ViewState) bool {
	return false
}

func (f *FrequencyViz) SetTotalDuration(duration time.Duration) {
	f.totalDuration = duration
}
