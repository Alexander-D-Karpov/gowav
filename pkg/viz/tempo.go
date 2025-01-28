package viz

import (
	"github.com/charmbracelet/lipgloss"
	"strings"
	"time"
)

const tempoMaxHeight = 40

type TempoViz struct {
	// beatData   => an array of frame-based “beat intensities” per FFT frame
	// energy     => a parallel array (same length) for “energy” (e.g. RMS)
	beatData      []float64
	energy        []float64
	sampleRate    int
	maxBeat       float64
	maxEnergy     float64
	totalDuration time.Duration
}

// NewTempoViz expects beatData and energy to be the same length,
// e.g. both have “numFrames” from your FFT-based analysis.
func NewTempoViz(beatData, energy []float64, sampleRate int) *TempoViz {
	var maxBeat float64
	for _, b := range beatData {
		if b > maxBeat {
			maxBeat = b
		}
	}
	var maxEnergy float64
	for _, e := range energy {
		if e > maxEnergy {
			maxEnergy = e
		}
	}
	return &TempoViz{
		beatData:   beatData,
		energy:     energy,
		sampleRate: sampleRate,
		maxBeat:    maxBeat,
		maxEnergy:  maxEnergy,
	}
}

func (t *TempoViz) Render(state ViewState) string {
	// If either array is empty or sampleRate is invalid, bail.
	if len(t.beatData) == 0 || len(t.energy) == 0 || t.sampleRate <= 0 {
		return "No tempo data available"
	}

	var sb strings.Builder

	// Determine how much vertical space we have.
	height := state.Height - 4
	if height < 2 {
		height = 2
	}
	if height > tempoMaxHeight {
		height = tempoMaxHeight
	}
	halfHeight := height / 2
	if halfHeight < 1 {
		halfHeight = 1
	}

	// Samples (frames) per column, based on the number of beat/energy frames.
	samplesPerCol := int(float64(len(t.beatData)) / float64(state.Width) / state.Zoom)
	if samplesPerCol < 1 {
		samplesPerCol = 1
	}

	// Convert offset time → the first sample (frame) to display.
	startSample := 0
	if t.totalDuration > 0 {
		startSample = int((state.Offset.Seconds() / t.totalDuration.Seconds()) * float64(len(t.beatData)))
		if startSample < 0 {
			startSample = 0
		}
		if startSample >= len(t.beatData) {
			startSample = len(t.beatData) - 1
		}
	}

	// Buffers for “Tempo” in the top half, “Energy” in the bottom half.
	tempoBuf := make([][]string, halfHeight)
	energyBuf := make([][]string, halfHeight)
	for i := 0; i < halfHeight; i++ {
		tempoBuf[i] = make([]string, state.Width)
		energyBuf[i] = make([]string, state.Width)
		for j := 0; j < state.Width; j++ {
			tempoBuf[i][j] = " "
			energyBuf[i][j] = " "
		}
	}

	// Loop over each screen column
	for x := 0; x < state.Width; x++ {
		idx := startSample + x*samplesPerCol
		if idx >= len(t.beatData) {
			break
		}

		// Average frames in this column so each column is a slice of samples.
		var beatSum, energySum float64
		count := 0
		end := idx + samplesPerCol
		if end > len(t.beatData) {
			end = len(t.beatData)
		}
		for i := idx; i < end; i++ {
			beatSum += t.beatData[i]
			energySum += t.energy[i]
			count++
		}
		if count == 0 {
			continue
		}
		beatVal := beatSum / float64(count)
		energyVal := energySum / float64(count)

		// Scale each to the available half-height
		beatHeight := 0
		if t.maxBeat > 0 {
			beatHeight = int((beatVal / t.maxBeat) * float64(halfHeight-1))
		}
		energyHeight := 0
		if t.maxEnergy > 0 {
			energyHeight = int((energyVal / t.maxEnergy) * float64(halfHeight-1))
		}

		// Fill tempo buffer from the center downward
		for y := halfHeight - 1; y >= halfHeight-beatHeight-1; y-- {
			if y >= 0 {
				tempoBuf[y][x] = lipgloss.NewStyle().
					Foreground(state.ColorScheme.Primary).
					Render("█")
			}
		}

		// Fill energy buffer similarly
		for y := halfHeight - 1; y >= halfHeight-energyHeight-1; y-- {
			if y >= 0 {
				energyBuf[y][x] = lipgloss.NewStyle().
					Foreground(state.ColorScheme.Secondary).
					Render("█")
			}
		}
	}

	// Render “Tempo:”
	sb.WriteString("Tempo:\n")
	for y := 0; y < halfHeight; y++ {
		sb.WriteString(strings.Join(tempoBuf[y], ""))
		sb.WriteString("\n")
	}

	// Render “Energy:”
	sb.WriteString("Energy:\n")
	for y := 0; y < halfHeight; y++ {
		sb.WriteString(strings.Join(energyBuf[y], ""))
		sb.WriteString("\n")
	}

	// Finally, show a time axis across the bottom
	sb.WriteString(t.renderTimeAxis(state, startSample, samplesPerCol))
	return sb.String()
}

// renderTimeAxis draws small timestamp markers along the bottom of the tempo visualization.
func (t *TempoViz) renderTimeAxis(state ViewState, startSample, samplesPerCol int) string {
	var sb strings.Builder

	if t.sampleRate <= 0 {
		return ""
	}
	secPerSample := 1.0 / float64(t.sampleRate)
	numMarkers := state.Width / 10
	if numMarkers < 1 {
		numMarkers = 1
	}

	for i := 0; i <= numMarkers; i++ {
		pos := float64(i) * float64(state.Width) / float64(numMarkers)
		samplePos := startSample + int(pos)*samplesPerCol
		if samplePos < 0 {
			samplePos = 0
		}
		if samplePos >= len(t.beatData) {
			samplePos = len(t.beatData) - 1
		}
		timeOffset := time.Duration(float64(samplePos) * secPerSample * float64(time.Second))

		label := formatDuration(timeOffset)
		if i == 0 {
			sb.WriteString(label)
		} else {
			padding := int(pos) - len(sb.String())
			if padding > 0 {
				sb.WriteString(strings.Repeat(" ", padding))
				sb.WriteString(label)
			}
		}
	}
	return sb.String()
}

func (t *TempoViz) Name() string {
	return "Tempo Analysis"
}
func (t *TempoViz) Description() string {
	return "Tempo and energy patterns"
}
func (t *TempoViz) SetTotalDuration(duration time.Duration) {
	t.totalDuration = duration
}
func (t *TempoViz) HandleInput(string, *ViewState) bool {
	return false
}
