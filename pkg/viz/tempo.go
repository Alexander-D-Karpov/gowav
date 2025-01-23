package viz

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"strings"
	"time"
)

const tempoMaxHeight = 40

type TempoViz struct {
	tempoData     []float64
	energyData    []float64
	sampleRate    int
	maxTempo      float64
	maxEnergy     float64
	style         lipgloss.Style
	totalDuration time.Duration
}

func NewTempoViz(tempoData, energyData []float64, sampleRate int) *TempoViz {
	maxTempo := 0.0
	for _, t := range tempoData {
		if t > maxTempo {
			maxTempo = t
		}
	}
	maxEnergy := 0.0
	for _, e := range energyData {
		if e > maxEnergy {
			maxEnergy = e
		}
	}
	return &TempoViz{
		tempoData:  tempoData,
		energyData: energyData,
		sampleRate: sampleRate,
		maxTempo:   maxTempo,
		maxEnergy:  maxEnergy,
		style:      lipgloss.NewStyle(),
	}
}

func (t *TempoViz) Name() string {
	return "Tempo & Energy"
}

func (t *TempoViz) Description() string {
	return "Displays tempo variations and energy levels over time"
}

func (t *TempoViz) Render(state ViewState) string {
	if len(t.tempoData) == 0 {
		return "No tempo data"
	}

	var sb strings.Builder

	height := state.Height - 4
	if height < 2 {
		height = 2
	}
	if height > tempoMaxHeight {
		height = tempoMaxHeight
	}
	half := height / 2

	sb.WriteString("Tempo (top half) vs Energy (bottom half)\n")

	offset := int(state.Offset.Seconds() * float64(t.sampleRate) / 1024.0)
	if offset < 0 {
		offset = 0
	}
	if offset >= len(t.tempoData) {
		offset = len(t.tempoData) - 1
	}

	width := state.Width
	samplesPerCol := int(float64(len(t.tempoData)) / float64(width) / state.Zoom)
	if samplesPerCol < 1 {
		samplesPerCol = 1
	}

	topBuf := make([][]string, half)
	botBuf := make([][]string, half)

	for r := 0; r < half; r++ {
		topBuf[r] = make([]string, width)
		botBuf[r] = make([]string, width)
		for c := 0; c < width; c++ {
			topBuf[r][c] = " "
			botBuf[r][c] = " "
		}
	}

	// Tempo
	for col := 0; col < width; col++ {
		idx := offset + col*samplesPerCol
		if idx >= len(t.tempoData) {
			break
		}
		sum := 0.0
		count := 0
		for i := idx; i < idx+samplesPerCol && i < len(t.tempoData); i++ {
			sum += t.tempoData[i]
			count++
		}
		tempo := sum / float64(count)

		scaled := int((tempo / t.maxTempo) * float64(half-1))
		for row := half - 1; row >= 0; row-- {
			if row > half-1-scaled {
				continue
			}
			color := getGradientColor(float64(row)/float64(half-1), state.ColorScheme)
			topBuf[row][col] = lipgloss.NewStyle().Foreground(color).Render("█")
		}
	}

	// Energy
	for col := 0; col < width; col++ {
		idx := offset + col*samplesPerCol
		if idx >= len(t.energyData) {
			break
		}
		sum := 0.0
		count := 0
		for i := idx; i < idx+samplesPerCol && i < len(t.energyData); i++ {
			sum += t.energyData[i]
			count++
		}
		energy := sum / float64(count)

		scaled := int((energy / t.maxEnergy) * float64(half-1))
		for row := half - 1; row >= 0; row-- {
			if row > half-1-scaled {
				continue
			}
			intensity := float64(row) / float64(half-1)
			color := getSpectrogramColor(intensity, state.ColorScheme)
			botBuf[row][col] = lipgloss.NewStyle().Foreground(color).Render("█")
		}
	}

	// Render top half
	for r := 0; r < half; r++ {
		sb.WriteString(strings.Join(topBuf[r], ""))
		sb.WriteString("\n")
	}
	// Render bottom half
	for r := 0; r < half; r++ {
		sb.WriteString(strings.Join(botBuf[r], ""))
		sb.WriteString("\n")
	}

	sb.WriteString(t.renderTimeAxis(state, offset, samplesPerCol))
	return sb.String()
}

func (t *TempoViz) renderTimeAxis(state ViewState, offset, samplesPerCol int) string {
	var sb strings.Builder
	totalSteps := len(t.tempoData)
	secondsPerStep := 1024.0 / float64(t.sampleRate)

	markers := 10
	stepCol := state.Width / markers
	if stepCol < 1 {
		stepCol = 1
	}
	for c := 0; c < state.Width; c += stepCol {
		idx := offset + c*samplesPerCol
		if idx >= totalSteps {
			break
		}
		tSec := float64(idx) * secondsPerStep
		tstamp := time.Duration(tSec * float64(time.Second))
		timeStr := fmt.Sprintf("%02d:%02d", int(tstamp.Minutes()), int(tstamp.Seconds())%60)
		sb.WriteString(fmt.Sprintf("%-7s", timeStr))
	}
	return sb.String()
}

func (t *TempoViz) HandleInput(key string, state *ViewState) bool {
	return false
}

func (t *TempoViz) SetTotalDuration(duration time.Duration) {
	t.totalDuration = duration
}
