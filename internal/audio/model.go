// internal/audio/model.go
package audio

import (
	"fmt"
	"math"
	"runtime"
	"sync"
	"time"
)

type Model struct {
	// Raw audio data
	RawData    []float64
	SampleRate int

	// Spectrum analysis
	FFTData   [][]float64 // Time x Frequency bins
	FreqBands []float64   // Frequency values for each bin

	// Beat/tempo analysis
	BeatData       []float64 // Energy envelope
	BeatOnsets     []bool    // Beat detection flags
	EstimatedTempo float64   // Estimated BPM

	// Additional analysis data
	PeakFrequencies []float64 // Most prominent frequencies
	RMSEnergy       []float64 // RMS energy over time
	SpectralFlux    []float64 // Spectral change over time

	// Configuration
	windowSize int
	hopSize    int
	fftSize    int
}

// NewModel creates a new audio model with default settings
func NewModel(sampleRate int) *Model {
	return &Model{
		SampleRate: sampleRate,
		windowSize: 2048,
		hopSize:    512,
		fftSize:    2048,
	}
}

// SetParameters allows customizing analysis parameters
func (m *Model) SetParameters(windowSize, hopSize, fftSize int) {
	m.windowSize = windowSize
	m.hopSize = hopSize
	m.fftSize = fftSize
}

// AnalyzeWaveform converts raw bytes to float64 samples and performs basic analysis
func (m *Model) AnalyzeWaveform(rawBytes []byte, progressFn func(float64), cancelChan chan struct{}) error {
	dataLen := len(rawBytes)
	if dataLen < 2 {
		return fmt.Errorf("data too short")
	}

	// Process in parallel chunks
	numCPU := runtime.NumCPU()
	chunkSize := dataLen / (2 * numCPU)
	if chunkSize < 1024 {
		chunkSize = 1024
	}

	m.RawData = make([]float64, dataLen/2)
	var wg sync.WaitGroup
	errChan := make(chan error, numCPU)

	// Process each chunk in parallel
	for i := 0; i < numCPU; i++ {
		wg.Add(1)
		start := i * chunkSize * 2
		end := (i + 1) * chunkSize * 2
		if i == numCPU-1 {
			end = dataLen - (dataLen % 2)
		}

		go func(start, end int) {
			defer wg.Done()
			if err := m.processWaveformChunk(rawBytes, start, end, progressFn, cancelChan); err != nil {
				errChan <- err
			}
		}(start, end)
	}

	wg.Wait()

	// Check for errors
	select {
	case err := <-errChan:
		return fmt.Errorf("waveform analysis error: %w", err)
	default:
		return nil
	}
}

func (m *Model) processWaveformChunk(rawBytes []byte, start, end int, progressFn func(float64), cancelChan chan struct{}) error {
	for i := start; i < end-1; i += 2 {
		select {
		case <-cancelChan:
			return fmt.Errorf("cancelled")
		default:
			// Convert bytes to float64 sample
			val := int16(rawBytes[i]) | int16(rawBytes[i+1])<<8
			m.RawData[i/2] = float64(val) / 32768.0

			// Report progress periodically
			if i%8192 == 0 {
				progress := float64(i-start) / float64(end-start)
				progressFn(progress)
			}
		}
	}
	return nil
}

// AnalyzeSpectrum performs FFT analysis
func (m *Model) AnalyzeSpectrum(progressFn func(float64), cancelChan chan struct{}) error {
	if len(m.RawData) < m.windowSize {
		return fmt.Errorf("insufficient data for spectrum analysis")
	}

	numWindows := (len(m.RawData) - m.windowSize) / m.hopSize
	if numWindows < 1 {
		return fmt.Errorf("insufficient data for spectrum analysis")
	}

	// Initialize frequency bands
	m.initFrequencyBands()

	// Initialize FFT data
	m.FFTData = make([][]float64, numWindows)
	for i := range m.FFTData {
		m.FFTData[i] = make([]float64, m.fftSize/2)
	}

	// Process in parallel
	numCPU := runtime.NumCPU()
	windowChan := make(chan int, numWindows)
	errChan := make(chan error, numCPU)
	var wg sync.WaitGroup

	// Start worker goroutines
	for i := 0; i < numCPU; i++ {
		wg.Add(1)
		go m.fftWorker(windowChan, &wg, progressFn, cancelChan, errChan)
	}

	// Feed windows to workers
	go func() {
		for i := 0; i < numWindows; i++ {
			select {
			case <-cancelChan:
				close(windowChan)
				return
			default:
				windowChan <- i
			}
		}
		close(windowChan)
	}()

	wg.Wait()

	// Check for errors
	select {
	case err := <-errChan:
		return fmt.Errorf("spectrum analysis error: %w", err)
	default:
		// Calculate additional spectral features
		return m.calculateSpectralFeatures(cancelChan)
	}
}

func (m *Model) initFrequencyBands() {
	m.FreqBands = make([]float64, m.fftSize/2)
	nyquist := float64(m.SampleRate) / 2
	for i := range m.FreqBands {
		m.FreqBands[i] = float64(i) * nyquist / float64(m.fftSize/2)
	}
}

func (m *Model) fftWorker(windowChan chan int, wg *sync.WaitGroup, progressFn func(float64),
	cancelChan chan struct{}, errChan chan error) {
	defer wg.Done()

	windowed := make([]float64, m.windowSize)
	for windowIdx := range windowChan {
		select {
		case <-cancelChan:
			errChan <- fmt.Errorf("cancelled")
			return
		default:
			if err := m.processFFTWindow(windowIdx, windowed); err != nil {
				errChan <- err
				return
			}

			if windowIdx%(len(m.FFTData)/100+1) == 0 {
				progressFn(float64(windowIdx) / float64(len(m.FFTData)))
			}
		}
	}
}

func (m *Model) processFFTWindow(windowIdx int, windowed []float64) error {
	startSample := windowIdx * m.hopSize
	if startSample+m.windowSize > len(m.RawData) {
		return fmt.Errorf("invalid window index")
	}

	// Apply Hanning window
	for i := 0; i < m.windowSize; i++ {
		win := 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/float64(m.windowSize)))
		windowed[i] = m.RawData[startSample+i] * win
	}

	// Compute FFT
	m.computeFFT(windowIdx, windowed)

	return nil
}

func (m *Model) computeFFT(windowIdx int, windowed []float64) {
	for freq := 0; freq < m.fftSize/2; freq++ {
		var re, im float64
		for t := 0; t < m.windowSize; t++ {
			angle := 2 * math.Pi * float64(freq*t) / float64(m.fftSize)
			re += windowed[t] * math.Cos(angle)
			im -= windowed[t] * math.Sin(angle)
		}
		m.FFTData[windowIdx][freq] = math.Sqrt(re*re + im*im)
	}
}

func (m *Model) calculateSpectralFeatures(cancelChan chan struct{}) error {
	numFrames := len(m.FFTData)
	m.SpectralFlux = make([]float64, numFrames)
	m.PeakFrequencies = make([]float64, numFrames)
	m.RMSEnergy = make([]float64, numFrames)

	for i := 0; i < numFrames; i++ {
		select {
		case <-cancelChan:
			return fmt.Errorf("cancelled")
		default:
			// Calculate spectral flux
			if i > 0 {
				m.SpectralFlux[i] = m.calculateFlux(m.FFTData[i], m.FFTData[i-1])
			}

			// Find peak frequency
			m.PeakFrequencies[i] = m.findPeakFrequency(m.FFTData[i])

			// Calculate RMS energy
			m.RMSEnergy[i] = m.calculateRMSEnergy(m.FFTData[i])
		}
	}

	return nil
}

func (m *Model) calculateFlux(current, previous []float64) float64 {
	var flux float64
	for i := range current {
		diff := current[i] - previous[i]
		if diff > 0 {
			flux += diff
		}
	}
	return flux
}

func (m *Model) findPeakFrequency(spectrum []float64) float64 {
	maxAmp := 0.0
	peakIdx := 0
	for i, amp := range spectrum {
		if amp > maxAmp {
			maxAmp = amp
			peakIdx = i
		}
	}
	return m.FreqBands[peakIdx]
}

func (m *Model) calculateRMSEnergy(spectrum []float64) float64 {
	var sumSquares float64
	for _, amp := range spectrum {
		sumSquares += amp * amp
	}
	return math.Sqrt(sumSquares / float64(len(spectrum)))
}

// AnalyzeBeats performs beat and tempo analysis
func (m *Model) AnalyzeBeats(progressFn func(float64), cancelChan chan struct{}) error {
	// Ensure we have spectrum data
	if len(m.FFTData) == 0 {
		if err := m.AnalyzeSpectrum(func(p float64) {
			progressFn(p * 0.5) // First half for spectrum
		}, cancelChan); err != nil {
			return err
		}
	}

	numFrames := len(m.FFTData)
	m.BeatData = make([]float64, numFrames)
	m.BeatOnsets = make([]bool, numFrames)

	// Calculate onset detection function in parallel
	if err := m.calculateOnsetFunction(progressFn, cancelChan); err != nil {
		return err
	}

	// Detect beats and estimate tempo
	return m.detectBeats(progressFn, cancelChan)
}

func (m *Model) calculateOnsetFunction(progressFn func(float64), cancelChan chan struct{}) error {
	numCPU := runtime.NumCPU()
	chunkSize := len(m.FFTData) / numCPU
	if chunkSize < 1 {
		chunkSize = 1
	}

	var wg sync.WaitGroup
	errChan := make(chan error, numCPU)

	for i := 0; i < numCPU; i++ {
		wg.Add(1)
		start := i * chunkSize
		end := (i + 1) * chunkSize
		if i == numCPU-1 {
			end = len(m.FFTData)
		}

		go func(start, end int) {
			defer wg.Done()

			// Local history buffer
			history := make([]float64, 43) // ~1 second
			historyPos := 0

			for idx := start; idx < end; idx++ {
				select {
				case <-cancelChan:
					errChan <- fmt.Errorf("cancelled")
					return
				default:
					// Calculate energy focusing on low frequencies
					var energy float64
					for freq := 0; freq < len(m.FFTData[idx]); freq++ {
						if freq < m.fftSize/4 {
							energy += m.FFTData[idx][freq] * m.FFTData[idx][freq]
						}
					}
					energy = math.Sqrt(energy)

					m.BeatData[idx] = energy

					// Update history and detect onsets
					history[historyPos] = energy
					historyPos = (historyPos + 1) % len(history)

					// Calculate adaptive threshold
					var sum, count float64
					for _, e := range history {
						if e > 0 {
							sum += e
							count++
						}
					}
					threshold := (sum / count) * 1.3

					m.BeatOnsets[idx] = energy > threshold

					// Update progress
					if (idx-start)%(chunkSize/100+1) == 0 {
						progress := 0.5 + 0.5*float64(idx-start)/float64(end-start)
						progressFn(progress)
					}
				}
			}
		}(start, end)
	}

	wg.Wait()

	select {
	case err := <-errChan:
		return err
	default:
		return nil
	}
}

func (m *Model) detectBeats(progressFn func(float64), cancelChan chan struct{}) error {
	// Collect inter-onset intervals
	intervals := make([]float64, 0, len(m.BeatOnsets)/2)
	lastBeat := -1

	for i, isBeat := range m.BeatOnsets {
		if isBeat {
			if lastBeat != -1 {
				interval := float64(i - lastBeat)
				intervals = append(intervals, interval)
			}
			lastBeat = i
		}
	}

	if len(intervals) == 0 {
		m.EstimatedTempo = 120.0 // Default fallback
		return nil
	}

	// Create histogram of intervals
	hist := make(map[int]int)
	for _, interval := range intervals {
		bucket := int(math.Round(interval))
		hist[bucket]++
	}

	// Find most common interval
	maxCount := 0
	bestInterval := 0
	for interval, count := range hist {
		if count > maxCount {
			maxCount = count
			bestInterval = interval
		}
	}

	// Convert to BPM
	if bestInterval > 0 {
		secondsPerBeat := float64(bestInterval*m.hopSize) / float64(m.SampleRate)
		m.EstimatedTempo = 60.0 / secondsPerBeat

		// Refine beats using estimated tempo
		m.refineBeatDetection(progressFn, cancelChan)
	} else {
		m.EstimatedTempo = 120.0 // Default fallback
	}

	return nil
}

// refineBeatDetection improves beat detection using tempo information
func (m *Model) refineBeatDetection(progressFn func(float64), cancelChan chan struct{}) {
	// Calculate expected beat interval in frames
	framesPerBeat := (60.0 / m.EstimatedTempo) * float64(m.SampleRate) / float64(m.hopSize)

	// Window for searching around expected beat positions
	searchWindow := int(framesPerBeat * 0.1) // 10% of beat interval

	// Temporary storage for refined beats
	refinedBeats := make([]bool, len(m.BeatOnsets))

	// Start from first detected beat
	var firstBeat int
	for i, isBeat := range m.BeatOnsets {
		if isBeat {
			firstBeat = i
			refinedBeats[i] = true
			break
		}
	}

	// Predict and refine subsequent beats
	expectedPos := float64(firstBeat)
	for expectedPos < float64(len(m.BeatOnsets)) {
		select {
		case <-cancelChan:
			return
		default:
			// Convert expected position to integer
			pos := int(math.Round(expectedPos))

			// Define search window boundaries
			start := pos - searchWindow
			if start < 0 {
				start = 0
			}
			end := pos + searchWindow
			if end >= len(m.BeatOnsets) {
				end = len(m.BeatOnsets) - 1
			}

			// Find local maximum in beat detection function
			maxEnergy := 0.0
			maxPos := pos
			for i := start; i <= end; i++ {
				if m.BeatData[i] > maxEnergy {
					maxEnergy = m.BeatData[i]
					maxPos = i
				}
			}

			// Mark as beat if energy is significant
			threshold := m.calculateLocalThreshold(maxPos)
			if maxEnergy > threshold {
				refinedBeats[maxPos] = true
			}

			// Move to next expected beat position
			expectedPos += framesPerBeat
		}
	}

	// Update beat detections
	m.BeatOnsets = refinedBeats
}

// calculateLocalThreshold computes adaptive threshold for beat detection
func (m *Model) calculateLocalThreshold(pos int) float64 {
	// Window size for local energy calculation
	windowSize := 43 // ~1 second

	start := pos - windowSize/2
	if start < 0 {
		start = 0
	}
	end := pos + windowSize/2
	if end >= len(m.BeatData) {
		end = len(m.BeatData) - 1
	}

	// Calculate mean energy in window
	var sum, count float64
	for i := start; i <= end; i++ {
		sum += m.BeatData[i]
		count++
	}
	mean := sum / count

	// Calculate standard deviation
	var variance float64
	for i := start; i <= end; i++ {
		diff := m.BeatData[i] - mean
		variance += diff * diff
	}
	variance /= count
	stdDev := math.Sqrt(variance)

	// Return threshold based on mean and standard deviation
	return mean + 1.5*stdDev
}

// GetBeatTimes returns the timestamps of detected beats
func (m *Model) GetBeatTimes() []time.Duration {
	var beatTimes []time.Duration
	frameDuration := time.Duration(float64(m.hopSize) / float64(m.SampleRate) * float64(time.Second))

	for i, isBeat := range m.BeatOnsets {
		if isBeat {
			beatTime := time.Duration(i) * frameDuration
			beatTimes = append(beatTimes, beatTime)
		}
	}
	return beatTimes
}

// GetFrequencyResponse returns the frequency response at a specific time
func (m *Model) GetFrequencyResponse(timestamp time.Duration) []float64 {
	frameIndex := int(timestamp.Seconds() * float64(m.SampleRate) / float64(m.hopSize))
	if frameIndex < 0 || frameIndex >= len(m.FFTData) {
		return nil
	}
	return m.FFTData[frameIndex]
}

// GetEnvelopeSegment returns the amplitude envelope for a time range
func (m *Model) GetEnvelopeSegment(start, end time.Duration) []float64 {
	startFrame := int(start.Seconds() * float64(m.SampleRate) / float64(m.hopSize))
	endFrame := int(end.Seconds() * float64(m.SampleRate) / float64(m.hopSize))

	if startFrame < 0 {
		startFrame = 0
	}
	if endFrame >= len(m.RMSEnergy) {
		endFrame = len(m.RMSEnergy) - 1
	}

	return m.RMSEnergy[startFrame:endFrame]
}

// GetSpectralCentroid calculates the spectral centroid over time
func (m *Model) GetSpectralCentroid(start, end time.Duration) []float64 {
	startFrame := int(start.Seconds() * float64(m.SampleRate) / float64(m.hopSize))
	endFrame := int(end.Seconds() * float64(m.SampleRate) / float64(m.hopSize))

	if startFrame < 0 {
		startFrame = 0
	}
	if endFrame >= len(m.FFTData) {
		endFrame = len(m.FFTData) - 1
	}

	centroids := make([]float64, endFrame-startFrame)

	for i := range centroids {
		var weightedSum, totalEnergy float64
		for j, freq := range m.FreqBands {
			energy := m.FFTData[startFrame+i][j]
			weightedSum += freq * energy
			totalEnergy += energy
		}
		if totalEnergy > 0 {
			centroids[i] = weightedSum / totalEnergy
		}
	}

	return centroids
}
