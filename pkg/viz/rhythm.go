package viz

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"strings"
	"time"
)

const beatMaxHeight = 40

type BeatViz struct {
	beatData      []float64 // Energy envelope
	onsets        []bool    // Beat onset markers
	beatStrength  []float64 // Beat intensity values
	bpm           float64   // Estimated tempo
	sampleRate    int
	maxStrength   float64
	totalDuration time.Duration
}

func NewBeatViz(beatData []float64, onsets []bool, bpm float64, sampleRate int) *BeatViz {
	if len(beatData) == 0 {
		return &BeatViz{}
	}
	// Find max beat strength
	maxStrength := 0.0
	for _, b := range beatData {
		if b > maxStrength {
			maxStrength = b
		}
	}

	beatStrength := make([]float64, len(beatData))
	if maxStrength > 0 {
		for i, b := range beatData {
			beatStrength[i] = b / maxStrength
		}
	}

	return &BeatViz{
		beatData:     beatData,
		onsets:       onsets,
		beatStrength: beatStrength,
		bpm:          bpm,
		sampleRate:   sampleRate,
		maxStrength:  maxStrength,
	}
}

func (b *BeatViz) Render(state ViewState) string {
	if len(b.beatData) == 0 || b.sampleRate <= 0 {
		return "No beat data available"
	}

	var sb strings.Builder

	// Show BPM and basic info
	sb.WriteString(fmt.Sprintf("Tempo: %.1f BPM\n", b.bpm))

	// Calculate dimensions
	height := state.Height - 5
	if height < 2 {
		height = 2
	}
	if height > beatMaxHeight {
		height = beatMaxHeight
	}

	// Calculate view parameters
	samplesPerCol := int(float64(len(b.beatData)) / float64(state.Width) / state.Zoom)
	if samplesPerCol < 1 {
		samplesPerCol = 1
	}

	// Convert offset time → sample
	if b.totalDuration > 0 {
		startSample := int((state.Offset.Seconds() / b.totalDuration.Seconds()) * float64(len(b.beatData)))
		if startSample < 0 {
			startSample = 0
		}
		if startSample >= len(b.beatData) {
			startSample = len(b.beatData) - 1
		}

		// Prepare the display
		display := make([][]string, height)
		for i := range display {
			display[i] = make([]string, state.Width)
			for j := range display[i] {
				display[i][j] = " "
			}
		}

		// Fill display buffer
		for x := 0; x < state.Width; x++ {
			idx := startSample + (x * samplesPerCol)
			if idx >= len(b.beatData) {
				break
			}

			// Average beat strength and check for onsets
			var strengthSum float64
			hasOnset := false
			count := 0

			end := idx + samplesPerCol
			if end > len(b.beatData) {
				end = len(b.beatData)
			}

			for i := idx; i < end; i++ {
				strengthSum += b.beatStrength[i]
				if i < len(b.onsets) && b.onsets[i] {
					hasOnset = true
				}
				count++
			}

			if count > 0 {
				avgStrength := strengthSum / float64(count)
				barHeight := int(avgStrength * float64(height-1))

				// Draw beat strength bars
				for y := height - 1; y >= height-barHeight-1; y-- {
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
		sb.WriteString("\nBeats: ")
		sb.WriteString(lipgloss.NewStyle().
			Foreground(state.ColorScheme.Primary).
			Render("▼ "))
		sb.WriteString("Onset  ")
		sb.WriteString(lipgloss.NewStyle().
			Foreground(state.ColorScheme.Secondary).
			Render("█ "))
		sb.WriteString("Energy")
	} else {
		// If we have zero totalDuration, just say we have data but can’t place it
		sb.WriteString("(No track duration set, cannot place timeline)\n")
	}
	return sb.String()
}

func (b *BeatViz) renderTimeAxis(state ViewState, startSample, samplesPerCol int) string {
	var sb strings.Builder

	secPerSample := 1.0 / float64(b.sampleRate)
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
		if samplePos >= len(b.beatData) {
			samplePos = len(b.beatData) - 1
		}
		timeOffset := time.Duration(float64(samplePos) * secPerSample * float64(time.Second))

		if i == 0 {
			sb.WriteString(formatDuration(timeOffset))
		} else {
			padding := int(pos) - len(sb.String())
			if padding > 0 {
				sb.WriteString(strings.Repeat(" ", padding))
				sb.WriteString(formatDuration(timeOffset))
			}
		}
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
