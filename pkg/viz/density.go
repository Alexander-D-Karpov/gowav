package viz

import (
	"github.com/charmbracelet/lipgloss"
	"gonum.org/v1/gonum/dsp/fourier"
	"math"
	"strings"
	"time"
)

type DensityViz struct {
	densityData   []float64
	spectralData  [][]float64
	freqBands     []float64
	sampleRate    int
	maxDensity    float64
	totalDuration time.Duration
}

func NewDensityViz(rawData []float64, sampleRate int) *DensityViz {
	// Initialize with window size for FFT
	windowSize := 2048
	hopSize := 512
	numFrames := (len(rawData) - windowSize) / hopSize

	// Create FFT plan
	fft := fourier.NewFFT(windowSize)

	// Initialize data structures
	densityData := make([]float64, numFrames)
	spectralData := make([][]float64, numFrames)
	maxDensity := 0.0

	// Process frames
	window := make([]float64, windowSize)
	for i := 0; i < numFrames; i++ {
		start := i * hopSize

		// Apply Hanning window
		for j := 0; j < windowSize; j++ {
			sample := rawData[start+j]
			window[j] = sample * 0.5 * (1 - math.Cos(2*math.Pi*float64(j)/float64(windowSize)))
		}

		// Compute FFT
		spectrum := fft.Coefficients(nil, window)

		// Calculate spectral density
		var energy float64
		freqBins := make([]float64, len(spectrum)/2)

		for j := 0; j < len(spectrum)/2; j++ {
			re := real(spectrum[j])
			im := imag(spectrum[j])
			mag := math.Sqrt(re*re + im*im)
			freqBins[j] = mag
			energy += mag
		}

		spectralData[i] = freqBins
		densityData[i] = energy

		if energy > maxDensity {
			maxDensity = energy
		}
	}

	// Create frequency bands
	nyquist := float64(sampleRate) / 2.0
	freqBands := make([]float64, windowSize/2)
	for i := range freqBands {
		freqBands[i] = float64(i) * nyquist / float64(windowSize/2)
	}

	return &DensityViz{
		densityData:  densityData,
		spectralData: spectralData,
		freqBands:    freqBands,
		sampleRate:   sampleRate,
		maxDensity:   maxDensity,
	}
}

func (d *DensityViz) Render(state ViewState) string {
	if len(d.densityData) == 0 {
		return "No density data available"
	}

	var sb strings.Builder

	// Calculate dimensions
	height := state.Height - 4
	if height < 10 {
		height = 10
	}
	if height > 40 {
		height = 40
	}

	// Calculate view parameters
	samplesPerCol := int(float64(len(d.densityData)) / float64(state.Width) / state.Zoom)
	if samplesPerCol < 1 {
		samplesPerCol = 1
	}

	startFrame := int((state.Offset.Seconds() / d.totalDuration.Seconds()) * float64(len(d.densityData)))
	startFrame = clamp(startFrame, 0, len(d.densityData)-1)

	// Draw time axis
	sb.WriteString(d.renderTimeAxis(state, startFrame, samplesPerCol))
	sb.WriteString("\n")

	// Initialize color scale
	intensity := make([]float64, state.Width)
	maxIntensity := 0.0

	// Calculate intensity values
	for x := 0; x < state.Width; x++ {
		frame := startFrame + x*samplesPerCol
		if frame >= len(d.densityData) {
			break
		}

		// Average over the column
		sum := 0.0
		count := 0
		for i := 0; i < samplesPerCol && frame+i < len(d.densityData); i++ {
			sum += d.densityData[frame+i]
			count++
		}

		if count > 0 {
			intensity[x] = sum / float64(count)
			if intensity[x] > maxIntensity {
				maxIntensity = intensity[x]
			}
		}
	}

	// Render density map
	chars := []string{"·", ":", "▪", "▮", "█"}

	for y := 0; y < height; y++ {
		yRatio := float64(height-y-1) / float64(height-1)

		for x := 0; x < state.Width; x++ {
			if x >= len(intensity) {
				sb.WriteString(" ")
				continue
			}

			normalizedIntensity := 0.0
			if maxIntensity > 0 {
				normalizedIntensity = intensity[x] / maxIntensity
			}

			// Add vertical gradient effect
			gradientIntensity := normalizedIntensity * (1.0 - 0.5*math.Abs(yRatio-0.5))

			// Select character and color
			charIdx := int(gradientIntensity * float64(len(chars)-1))
			charIdx = clamp(charIdx, 0, len(chars)-1)

			color := getGradientColor(gradientIntensity, state.ColorScheme)
			sb.WriteString(lipgloss.NewStyle().
				Foreground(color).
				Render(chars[charIdx]))
		}
		sb.WriteString("\n")
	}

	// Add legend
	sb.WriteString("Density: ")
	for _, char := range chars {
		sb.WriteString(" " + char)
	}
	sb.WriteString(" (low → high)\n")

	return sb.String()
}

func (d *DensityViz) renderTimeAxis(state ViewState, startFrame, samplesPerCol int) string {
	var sb strings.Builder

	framesPerSecond := float64(d.sampleRate) / float64(512) // hop size
	numMarkers := 10
	markerStep := state.Width / numMarkers

	for i := 0; i <= numMarkers; i++ {
		pos := i * markerStep
		frame := startFrame + pos*samplesPerCol
		seconds := float64(frame) / framesPerSecond
		timestamp := time.Duration(seconds * float64(time.Second))

		if i == 0 {
			sb.WriteString(formatDuration(timestamp))
		} else {
			padding := pos - sb.Len()
			if padding > 0 {
				sb.WriteString(strings.Repeat(" ", padding))
				sb.WriteString(formatDuration(timestamp))
			}
		}
	}

	return sb.String()
}

func (d *DensityViz) Name() string {
	return "Density Map"
}

func (d *DensityViz) Description() string {
	return "Audio density visualization with spectral energy mapping"
}

func (d *DensityViz) SetTotalDuration(duration time.Duration) {
	d.totalDuration = duration
}

func (d *DensityViz) HandleInput(string, *ViewState) bool {
	return false
}
