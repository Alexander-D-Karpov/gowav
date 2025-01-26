package viz

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"strings"
	"time"
)

const beatMaxHeight = 40

type BeatViz struct {
	beatData      []float64
	onsets        []bool
	bpm           float64
	sampleRate    int
	maxBeat       float64
	totalDuration time.Duration
}

func NewBeatViz(beatData []float64, onsets []bool, bpm float64, sampleRate int) *BeatViz {
	maxBeat := 0.0
	for _, b := range beatData {
		if b > maxBeat {
			maxBeat = b
		}
	}

	return &BeatViz{
		beatData:   beatData,
		onsets:     onsets,
		bpm:        bpm,
		sampleRate: sampleRate,
		maxBeat:    maxBeat,
	}
}

func (b *BeatViz) Render(state ViewState) string {
	if len(b.beatData) == 0 {
		return "No beat data available"
	}

	var sb strings.Builder

	// Calculate dimensions and scaling
	height := state.Height - 5
	if height > beatMaxHeight {
		height = beatMaxHeight
	}

	samplesPerCol := int(float64(len(b.beatData)) / float64(state.Width) / state.Zoom)
	if samplesPerCol < 1 {
		samplesPerCol = 1
	}

	startSample := int((state.Offset.Seconds() / b.totalDuration.Seconds()) * float64(len(b.beatData)))
	startSample = clamp(startSample, 0, len(b.beatData)-1)

	// Initialize display buffer
	display := make([][]string, height)
	for i := range display {
		display[i] = make([]string, state.Width)
		for j := range display[i] {
			display[i][j] = " "
		}
	}

	// Show BPM at the top
	sb.WriteString(fmt.Sprintf("Tempo: %.1f BPM\n", b.bpm))

	// Fill display buffer
	for x := 0; x < state.Width; x++ {
		idx := startSample + (x * samplesPerCol)
		if idx >= len(b.beatData) {
			break
		}

		// Average beat intensity and check for onsets in this column
		var beatSum float64
		hasOnset := false
		count := 0

		for i := 0; i < samplesPerCol && (idx+i) < len(b.beatData); i++ {
			beatSum += b.beatData[idx+i]
			if b.onsets[idx+i] {
				hasOnset = true
			}
			count++
		}

		if count > 0 {
			beatVal := beatSum / float64(count)
			beatHeight := int((beatVal / b.maxBeat) * float64(height-1))

			// Draw beat intensity bars
			for y := height - 1; y >= height-beatHeight-1; y-- {
				if y >= 0 {
					style := lipgloss.NewStyle()
					if hasOnset {
						style = style.Foreground(state.ColorScheme.Primary)
					} else {
						style = style.Foreground(state.ColorScheme.Secondary)
					}
					display[y][x] = style.Render("█")
				}
			}

			// Mark beat onsets at the top
			if hasOnset {
				display[0][x] = lipgloss.NewStyle().
					Foreground(state.ColorScheme.Primary).
					Render("▼")
			}
		}
	}

	// Render display buffer
	for y := 0; y < height; y++ {
		sb.WriteString(strings.Join(display[y], ""))
		sb.WriteString("\n")
	}

	// Draw time axis
	sb.WriteString(b.renderTimeAxis(state, startSample, samplesPerCol))

	return sb.String()
}

func (b *BeatViz) renderTimeAxis(state ViewState, startSample, samplesPerCol int) string {
	var sb strings.Builder

	// Calculate time markers
	secPerSample := 1.0 / float64(b.sampleRate)
	numMarkers := state.Width / 10

	// Draw main time markers
	for i := 0; i <= numMarkers; i++ {
		pos := float64(i) * float64(state.Width) / float64(numMarkers)
		timeOffset := time.Duration(float64(startSample+int(pos)*samplesPerCol) * secPerSample * float64(time.Second))
		sb.WriteString(fmt.Sprintf("%-8s", formatDuration(timeOffset)))
	}

	return sb.String()
}

func (b *BeatViz) Name() string {
	return "Beat Pattern"
}

func (b *BeatViz) Description() string {
	return "Beat detection and rhythm analysis"
}

func (b *BeatViz) SetTotalDuration(duration time.Duration) {
	b.totalDuration = duration
}

func (b *BeatViz) HandleInput(string, *ViewState) bool {
	return false
}
