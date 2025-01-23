package viz

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"math"
	"strings"
	"time"
)

const beatMaxHeight = 40

type BeatViz struct {
	beatData      []float64
	onsets        []bool
	tempo         float64
	sampleRate    int
	maxBeat       float64
	style         lipgloss.Style
	totalDuration time.Duration
}

func NewBeatViz(beatData []float64, onsets []bool, tempo float64, sampleRate int) *BeatViz {
	maxB := 0.0
	for _, b := range beatData {
		if b > maxB {
			maxB = b
		}
	}
	return &BeatViz{
		beatData:   beatData,
		onsets:     onsets,
		tempo:      tempo,
		sampleRate: sampleRate,
		maxBeat:    maxB,
		style:      lipgloss.NewStyle(),
	}
}

func (b *BeatViz) Name() string {
	return "Beat & Rhythm"
}

func (b *BeatViz) Description() string {
	return "Displays beat patterns and rhythm structure"
}

func (b *BeatViz) Render(state ViewState) string {
	if len(b.beatData) == 0 {
		return "No beat data"
	}

	var sb strings.Builder

	height := state.Height - 6
	if height < 2 {
		height = 2
	}
	if height > beatMaxHeight {
		height = beatMaxHeight
	}

	sb.WriteString(fmt.Sprintf("Estimated Tempo: %.1f BPM\n", b.tempo))

	offset := int(state.Offset.Seconds() * float64(b.sampleRate) / 1024.0)
	if offset < 0 {
		offset = 0
	}
	if offset >= len(b.beatData) {
		offset = len(b.beatData) - 1
	}

	width := state.Width
	samplesPerCol := int(float64(len(b.beatData)) / float64(width) / state.Zoom)
	if samplesPerCol < 1 {
		samplesPerCol = 1
	}
	if offset >= len(b.beatData) {
		offset = len(b.beatData) - 1
	}

	confHeight := height / 2
	if confHeight < 1 {
		confHeight = 1
	}
	beatGraph := make([][]string, confHeight)
	for r := 0; r < confHeight; r++ {
		beatGraph[r] = make([]string, width)
		for c := 0; c < width; c++ {
			beatGraph[r][c] = " "
		}
	}

	for col := 0; col < width; col++ {
		idx := offset + col*samplesPerCol
		if idx >= len(b.beatData) {
			break
		}
		sum := 0.0
		hasOnset := false
		count := 0
		for i := 0; i < samplesPerCol && idx+i < len(b.beatData); i++ {
			sum += b.beatData[idx+i]
			if b.onsets[idx+i] {
				hasOnset = true
			}
			count++
		}
		beatConf := 0.0
		if count > 0 {
			beatConf = sum / float64(count)
		}
		scaled := int((beatConf / b.maxBeat) * float64(confHeight-1))

		for row := confHeight - 1; row >= 0; row-- {
			if row > confHeight-1-scaled {
				continue
			}
			clr := state.ColorScheme.Primary
			if hasOnset {
				clr = state.ColorScheme.Accent
			}
			beatGraph[row][col] = lipgloss.NewStyle().Foreground(clr).Render("█")
		}
	}

	rhythmHeight := height - confHeight
	if rhythmHeight < 1 {
		rhythmHeight = 1
	}
	rhythmGraph := make([][]string, rhythmHeight)
	for r := 0; r < rhythmHeight; r++ {
		rhythmGraph[r] = make([]string, width)
		for c := 0; c < width; c++ {
			rhythmGraph[r][c] = " "
		}
	}

	beatDuration := float64(b.sampleRate) * 60.0 / b.tempo
	if beatDuration <= 0 {
		beatDuration = 1
	}
	for col := 0; col < width; col++ {
		idx := offset + col*samplesPerCol
		if idx >= len(b.beatData) {
			break
		}
		phase := float64(idx%int(beatDuration)) / beatDuration
		y := math.Sin(phase * 2 * math.Pi)
		rowPos := int(((y + 1) / 2.0) * float64(rhythmHeight-1))
		rhythmGraph[rhythmHeight-1-rowPos][col] = lipgloss.NewStyle().Foreground(state.ColorScheme.Secondary).Render("·")
	}

	sb.WriteString("Beat Confidence:\n")
	for r := 0; r < confHeight; r++ {
		sb.WriteString(strings.Join(beatGraph[r], ""))
		sb.WriteString("\n")
	}
	sb.WriteString("Rhythm Pattern:\n")
	for r := 0; r < rhythmHeight; r++ {
		sb.WriteString(strings.Join(rhythmGraph[r], ""))
		sb.WriteString("\n")
	}

	sb.WriteString(b.renderTimeAxis(state, offset, samplesPerCol))
	return sb.String()
}

func (b *BeatViz) renderTimeAxis(state ViewState, offset, samplesPerCol int) string {
	var sb strings.Builder
	totalSteps := len(b.beatData)
	secPerStep := 1024.0 / float64(b.sampleRate)

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
		tSec := float64(idx) * secPerStep
		tstamp := time.Duration(tSec * float64(time.Second))
		timeStr := fmt.Sprintf("%02d:%02d", int(tstamp.Minutes()), int(tstamp.Seconds())%60)
		sb.WriteString(fmt.Sprintf("%-8s", timeStr))
	}
	return sb.String()
}

func (b *BeatViz) HandleInput(key string, state *ViewState) bool {
	return false
}

func (b *BeatViz) SetTotalDuration(duration time.Duration) {
	b.totalDuration = duration
}
