package viz

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"strings"
	"time"
)

const tempoMaxHeight = 40

type TempoViz struct {
	beatData      []float64
	energy        []float64
	sampleRate    int
	maxBeat       float64
	maxEnergy     float64
	totalDuration time.Duration
}

func NewTempoViz(beatData, energy []float64, sampleRate int) *TempoViz {
	maxBeat := 0.0
	for _, b := range beatData {
		if b > maxBeat {
			maxBeat = b
		}
	}

	maxEnergy := 0.0
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
	if len(t.beatData) == 0 {
		return "No tempo data available"
	}

	var sb strings.Builder

	// Calculate display dimensions
	height := state.Height - 4
	if height > tempoMaxHeight {
		height = tempoMaxHeight
	}
	halfHeight := height / 2

	// Calculate samples per column and start position
	samplesPerCol := int(float64(len(t.beatData)) / float64(state.Width) / state.Zoom)
	if samplesPerCol < 1 {
		samplesPerCol = 1
	}

	startSample := int((state.Offset.Seconds() / t.totalDuration.Seconds()) * float64(len(t.beatData)))
	startSample = clamp(startSample, 0, len(t.beatData)-1)

	// Initialize display buffers for tempo and energy
	tempoBuf := make([][]string, halfHeight)
	energyBuf := make([][]string, halfHeight)
	for i := range tempoBuf {
		tempoBuf[i] = make([]string, state.Width)
		energyBuf[i] = make([]string, state.Width)
		for j := range tempoBuf[i] {
			tempoBuf[i][j] = " "
			energyBuf[i][j] = " "
		}
	}

	// Fill buffers
	for x := 0; x < state.Width; x++ {
		idx := startSample + (x * samplesPerCol)
		if idx >= len(t.beatData) {
			break
		}

		// Average values for this column
		var beatSum, energySum float64
		count := 0

		for i := 0; i < samplesPerCol && (idx+i) < len(t.beatData); i++ {
			beatSum += t.beatData[idx+i]
			energySum += t.energy[idx+i]
			count++
		}

		if count > 0 {
			beatVal := beatSum / float64(count)
			energyVal := energySum / float64(count)

			// Map to display heights
			beatHeight := int((beatVal / t.maxBeat) * float64(halfHeight-1))
			energyHeight := int((energyVal / t.maxEnergy) * float64(halfHeight-1))

			// Fill tempo buffer
			for y := halfHeight - 1; y >= halfHeight-beatHeight-1; y-- {
				if y >= 0 {
					tempoBuf[y][x] = lipgloss.NewStyle().
						Foreground(state.ColorScheme.Primary).
						Render("█")
				}
			}

			// Fill energy buffer
			for y := halfHeight - 1; y >= halfHeight-energyHeight-1; y-- {
				if y >= 0 {
					energyBuf[y][x] = lipgloss.NewStyle().
						Foreground(state.ColorScheme.Secondary).
						Render("█")
				}
			}
		}
	}

	// Render buffers
	sb.WriteString("Tempo:\n")
	for y := 0; y < halfHeight; y++ {
		sb.WriteString(strings.Join(tempoBuf[y], ""))
		sb.WriteString("\n")
	}

	sb.WriteString("Energy:\n")
	for y := 0; y < halfHeight; y++ {
		sb.WriteString(strings.Join(energyBuf[y], ""))
		sb.WriteString("\n")
	}

	// Render time axis
	sb.WriteString(t.renderTimeAxis(state, startSample, samplesPerCol))
	return sb.String()
}

func (t *TempoViz) renderTimeAxis(state ViewState, startSample, samplesPerCol int) string {
	var sb strings.Builder

	secPerSample := 1.0 / float64(t.sampleRate)
	numMarkers := state.Width / 10

	for i := 0; i <= numMarkers; i++ {
		pos := float64(i) * float64(state.Width) / float64(numMarkers)
		timeOffset := time.Duration(float64(startSample+int(pos)*samplesPerCol) * secPerSample * float64(time.Second))
		sb.WriteString(fmt.Sprintf("%-8s", formatDuration(timeOffset)))
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
