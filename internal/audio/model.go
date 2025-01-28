package audio

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"runtime"
	"sync"
	"time"

	"github.com/hajimehoshi/go-mp3"
	"gonum.org/v1/gonum/dsp/fourier"
)

// Model represents the raw PCM data plus FFT outputs and beat/onset analysis results.
type Model struct {
	RawData    []float64
	SampleRate int

	FFTData   [][]float64
	FreqBands []float64

	BeatData       []float64
	BeatOnsets     []bool
	EstimatedTempo float64

	PeakFrequencies []float64
	RMSEnergy       []float64
	SpectralFlux    []float64

	windowSize int
	hopSize    int
	fftSize    int
}

// NewModel creates a new Model with default analysis parameters.
func NewModel(sampleRate int) *Model {
	return &Model{
		SampleRate: sampleRate,
		windowSize: 2048,
		hopSize:    512,
		fftSize:    2048,
	}
}

// SetParameters updates the FFT window/hop sizes, if needed.
func (m *Model) SetParameters(windowSize, hopSize, fftSize int) {
	m.windowSize = windowSize
	m.hopSize = hopSize
	m.fftSize = fftSize
}

// decodeMP3ToPCM converts MP3 bytes to a mono float64 slice.
func decodeMP3ToPCM(
	mp3Bytes []byte,
	progressFn func(float64),
	cancelChan chan struct{},
) ([]float64, int, error) {

	reader := bytes.NewReader(mp3Bytes)
	dec, err := mp3.NewDecoder(reader)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to init mp3 decoder: %w", err)
	}

	sampleRate := dec.SampleRate() // often 44100 or 48000
	const bytesPerSample = 2
	const channels = 2
	frameSize := bytesPerSample * channels

	var pcm []float64
	totalSize := int64(len(mp3Bytes))
	var totalRead int64

	buf := make([]byte, 8192)
	for {
		select {
		case <-cancelChan:
			return nil, 0, fmt.Errorf("decode cancelled")
		default:
		}

		n, readErr := dec.Read(buf)
		if n > 0 {
			frames := n / frameSize
			for i := 0; i < frames; i++ {
				left := int16(buf[i*4+0]) | (int16(buf[i*4+1]) << 8)
				right := int16(buf[i*4+2]) | (int16(buf[i*4+3]) << 8)
				mono := float64(left+right) * 0.5
				mono /= 32768.0
				pcm = append(pcm, mono)
			}
			totalRead += int64(n)

			if progressFn != nil && totalSize > 0 {
				fraction := float64(totalRead) / float64(totalSize)
				if fraction > 1.0 {
					fraction = 1.0
				}
				progressFn(fraction)
			}
		}

		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, 0, fmt.Errorf("decode mp3 read error: %w", readErr)
		}
	}

	return pcm, sampleRate, nil
}

// AnalyzeWaveform decodes MP3 data into RawData.
func (m *Model) AnalyzeWaveform(
	mp3Bytes []byte,
	progressFn func(float64),
	cancelChan chan struct{},
) error {

	startTime := time.Now()

	pcmSamples, sr, err := decodeMP3ToPCM(mp3Bytes, func(frac float64) {
		if progressFn != nil {
			progressFn(frac * 0.95)
		}
	}, cancelChan)
	if err != nil {
		return fmt.Errorf("decode error: %w", err)
	}
	m.RawData = pcmSamples
	m.SampleRate = sr

	if progressFn != nil {
		progressFn(1.0)
	}

	logDebug("AnalyzeWaveform: decoded PCM has %d samples at sr=%d (%.2f sec)",
		len(pcmSamples), sr, float64(len(pcmSamples))/float64(sr))
	logDebug("Waveform analysis took %v", time.Since(startTime))
	return nil
}

// AnalyzeSpectrum runs a short-time FFT over RawData, populating FFTData + FreqBands.
func (m *Model) AnalyzeSpectrum(
	progressFn func(float64),
	cancelChan chan struct{},
) error {

	if m.SampleRate <= 0 {
		return fmt.Errorf("invalid sample rate (%d)", m.SampleRate)
	}
	if len(m.RawData) < m.windowSize {
		return fmt.Errorf("insufficient data for spectrum analysis")
	}

	numWindows := (len(m.RawData) - m.windowSize) / m.hopSize
	if numWindows < 1 {
		return fmt.Errorf("not enough samples for any FFT window")
	}

	m.initFrequencyBands()

	m.FFTData = make([][]float64, numWindows)
	for i := range m.FFTData {
		m.FFTData[i] = make([]float64, m.fftSize/2)
	}

	realFFT := fourier.NewFFT(m.fftSize)
	numCPU := runtime.NumCPU()
	windowChan := make(chan int, numWindows)
	errChan := make(chan error, numCPU)
	var wg sync.WaitGroup

	logDebug("Starting FFT with numWindows=%d, windowSize=%d, hopSize=%d", numWindows, m.windowSize, m.hopSize)

	// Start parallel workers
	for i := 0; i < numCPU; i++ {
		wg.Add(1)
		go m.fftWorker(realFFT, windowChan, &wg, progressFn, cancelChan, errChan, numWindows)
	}

	// Feed window indices
	go func() {
		defer close(windowChan)
		for w := 0; w < numWindows; w++ {
			select {
			case <-cancelChan:
				return
			default:
				windowChan <- w
			}
		}
	}()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	// Wait for completion or cancellation
	select {
	case <-done:
	case <-cancelChan:
		return fmt.Errorf("analysis cancelled")
	case err := <-errChan:
		return err
	}

	// Compute spectral features
	if err := m.calculateSpectralFeatures(cancelChan, progressFn); err != nil {
		return err
	}

	return nil
}

// initFrequencyBands populates FreqBands up to Nyquist.
func (m *Model) initFrequencyBands() {
	m.FreqBands = make([]float64, m.fftSize/2)
	nyquist := float64(m.SampleRate) / 2.0
	for i := range m.FreqBands {
		m.FreqBands[i] = float64(i) * nyquist / float64(m.fftSize/2)
	}
}

// fftWorker applies a Hanning window, runs FFT, and stores amplitude results for a subset of frames.
func (m *Model) fftWorker(
	realFFT *fourier.FFT,
	windowChan chan int,
	wg *sync.WaitGroup,
	progressFn func(float64),
	cancelChan chan struct{},
	errChan chan error,
	totalWindows int,
) {
	defer wg.Done()

	windowed := make([]float64, m.fftSize)

	logDebug("fftWorker started. totalWindows=%d", totalWindows)

	for windowIdx := range windowChan {
		select {
		case <-cancelChan:
			select {
			case errChan <- fmt.Errorf("cancelled"):
			default:
			}
			return
		default:
		}

		startSample := windowIdx * m.hopSize
		if startSample+m.windowSize > len(m.RawData) {
			select {
			case errChan <- fmt.Errorf("invalid window index"):
			default:
			}
			return
		}

		// Apply Hanning
		for i := 0; i < m.fftSize; i++ {
			if i < m.windowSize {
				w := 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/float64(m.windowSize)))
				windowed[i] = m.RawData[startSample+i] * w
			} else {
				windowed[i] = 0
			}
		}

		spectrum := realFFT.Coefficients(nil, windowed)
		for freq := 0; freq < m.fftSize/2; freq++ {
			re := real(spectrum[freq])
			im := imag(spectrum[freq])
			m.FFTData[windowIdx][freq] = math.Sqrt(re*re + im*im)
		}

		if progressFn != nil && totalWindows > 0 {
			if windowIdx%500 == 0 {
				f := float64(windowIdx) / float64(totalWindows)
				progressFn(f)
			}
		}
	}

	logDebug("fftWorker finished")
}

// calculateSpectralFeatures extracts flux, peak frequencies, and RMS energy from the FFT slices.
func (m *Model) calculateSpectralFeatures(cancelChan chan struct{}, progressFn func(float64)) error {
	logDebug("calculateSpectralFeatures: starting for %d frames", len(m.FFTData))

	numFrames := len(m.FFTData)
	m.SpectralFlux = make([]float64, numFrames)
	m.PeakFrequencies = make([]float64, numFrames)
	m.RMSEnergy = make([]float64, numFrames)

	for i := 0; i < numFrames; i++ {
		select {
		case <-cancelChan:
			return fmt.Errorf("cancelled")
		default:
		}
		if i > 0 {
			m.SpectralFlux[i] = m.calculateFlux(m.FFTData[i], m.FFTData[i-1])
		}
		m.PeakFrequencies[i] = m.findPeakFrequency(m.FFTData[i])
		m.RMSEnergy[i] = m.calculateRMSEnergy(m.FFTData[i])
	}
	logDebug("calculateSpectralFeatures: completed")

	if progressFn != nil {
		progressFn(1.0)
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
	var sumSq float64
	for _, amp := range spectrum {
		sumSq += amp * amp
	}
	return math.Sqrt(sumSq / float64(len(spectrum)))
}

// AnalyzeBeats calls AnalyzeSpectrum if needed, then processes onsets to estimate tempo and refine beat info.
func (m *Model) AnalyzeBeats(
	progressFn func(float64),
	cancelChan chan struct{},
) error {

	if m.SampleRate <= 0 {
		return fmt.Errorf("invalid sample rate: %d", m.SampleRate)
	}
	if len(m.FFTData) == 0 {
		err := m.AnalyzeSpectrum(func(frac float64) {
			if progressFn != nil {
				progressFn(frac * 0.6)
			}
		}, cancelChan)
		if err != nil {
			return err
		}
	}

	numFrames := len(m.FFTData)
	if numFrames < 2 {
		return fmt.Errorf("not enough FFT frames")
	}

	m.BeatData = make([]float64, numFrames)
	m.BeatOnsets = make([]bool, numFrames)

	if err := m.calculateOnsetFunction(progressFn, cancelChan); err != nil {
		return err
	}

	return m.detectBeats(progressFn, cancelChan)
}

// calculateOnsetFunction uses a rolling approach to detect transient energy for the beat envelope.
func (m *Model) calculateOnsetFunction(
	progressFn func(float64),
	cancelChan chan struct{},
) error {

	numFrames := len(m.FFTData)
	numCPU := runtime.NumCPU()
	chunkSize := numFrames / numCPU
	if chunkSize < 1 {
		chunkSize = numFrames
	}

	var wg sync.WaitGroup
	errChan := make(chan error, numCPU)

	for i := 0; i < numCPU; i++ {
		start := i * chunkSize
		end := (i + 1) * chunkSize
		if i == numCPU-1 {
			end = numFrames
		}

		wg.Add(1)
		go func(s, e int) {
			defer wg.Done()

			history := make([]float64, 43)
			hPos := 0

			for idx := s; idx < e; idx++ {
				select {
				case <-cancelChan:
					errChan <- fmt.Errorf("cancelled")
					return
				default:
				}
				var energy float64
				for freq := 0; freq < len(m.FFTData[idx]); freq++ {
					// Accumulate lower-frequency energy (heuristic for onset)
					if freq < m.fftSize/4 {
						energy += m.FFTData[idx][freq] * m.FFTData[idx][freq]
					}
				}
				energy = math.Sqrt(energy)

				m.BeatData[idx] = energy
				history[hPos] = energy
				hPos = (hPos + 1) % len(history)

				var sum, count float64
				for _, eVal := range history {
					if eVal > 0 {
						sum += eVal
						count++
					}
				}
				if count > 0 {
					threshold := (sum / count) * 1.3
					m.BeatOnsets[idx] = energy > threshold
				}
			}

			if progressFn != nil && numFrames > 0 {
				localFrac := float64(e-s) / float64(numFrames)
				progressFn(0.6 + localFrac*0.2)
			}
		}(start, end)
	}

	waitDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitDone)
	}()

	select {
	case <-waitDone:
		select {
		case e := <-errChan:
			return e
		default:
		}
	case <-cancelChan:
		return fmt.Errorf("cancelled")
	case e := <-errChan:
		return e
	}

	return nil
}

// detectBeats uses a basic interval histogram approach to guess BPM, then refines onsets.
func (m *Model) detectBeats(progressFn func(float64), cancelChan chan struct{}) error {
	intervals := make([]float64, 0, len(m.BeatOnsets)/2)
	lastBeat := -1

	for i, isBeat := range m.BeatOnsets {
		if isBeat {
			if lastBeat != -1 {
				intervals = append(intervals, float64(i-lastBeat))
			}
			lastBeat = i
		}
	}

	if len(intervals) == 0 {
		m.EstimatedTempo = 120.0
		if progressFn != nil {
			progressFn(1.0)
		}
		return nil
	}

	hist := make(map[int]int)
	for _, iv := range intervals {
		b := int(math.Round(iv))
		hist[b]++
	}

	bestInterval := 0
	maxCount := 0
	for iv, count := range hist {
		if count > maxCount {
			maxCount = count
			bestInterval = iv
		}
	}

	if bestInterval > 0 {
		secondsPerBeat := float64(bestInterval*m.hopSize) / float64(m.SampleRate)
		if secondsPerBeat > 0 {
			m.EstimatedTempo = 60.0 / secondsPerBeat
		} else {
			m.EstimatedTempo = 120.0
		}
		m.refineBeatDetection(progressFn, cancelChan)
	} else {
		m.EstimatedTempo = 120.0
	}

	if progressFn != nil {
		progressFn(1.0)
	}
	return nil
}

// refineBeatDetection tries to align onsets to a consistent BPM for a more stable “beat” visualization.
func (m *Model) refineBeatDetection(progressFn func(float64), cancelChan chan struct{}) {
	framesPerBeat := (60.0 / m.EstimatedTempo) *
		(float64(m.SampleRate) / float64(m.hopSize))
	if framesPerBeat <= 0 {
		return
	}

	searchWindow := int(framesPerBeat * 0.1)
	refined := make([]bool, len(m.BeatOnsets))

	firstBeat := -1
	for i, isBeat := range m.BeatOnsets {
		if isBeat {
			firstBeat = i
			refined[i] = true
			break
		}
	}
	if firstBeat < 0 {
		return
	}

	expectedPos := float64(firstBeat)
	for expectedPos < float64(len(m.BeatOnsets)) {
		select {
		case <-cancelChan:
			return
		default:
		}

		pos := int(math.Round(expectedPos))
		if pos < 0 || pos >= len(m.BeatOnsets) {
			break
		}
		start := pos - searchWindow
		if start < 0 {
			start = 0
		}
		end := pos + searchWindow
		if end >= len(m.BeatOnsets) {
			end = len(m.BeatOnsets) - 1
		}

		maxE := 0.0
		maxPos := pos
		for i := start; i <= end; i++ {
			if m.BeatData[i] > maxE {
				maxE = m.BeatData[i]
				maxPos = i
			}
		}
		threshold := m.calculateLocalThreshold(maxPos)
		if maxE > threshold {
			refined[maxPos] = true
		}
		expectedPos += framesPerBeat
	}

	m.BeatOnsets = refined
}

// calculateLocalThreshold returns a local average + standard deviation threshold for peak detection.
func (m *Model) calculateLocalThreshold(pos int) float64 {
	windowSize := 43
	start := pos - windowSize/2
	if start < 0 {
		start = 0
	}
	end := pos + windowSize/2
	if end >= len(m.BeatData) {
		end = len(m.BeatData) - 1
	}
	var sum, count float64
	for i := start; i <= end; i++ {
		sum += m.BeatData[i]
		count++
	}
	if count == 0 {
		return 0
	}
	mean := sum / count
	var variance float64
	for i := start; i <= end; i++ {
		diff := m.BeatData[i] - mean
		variance += diff * diff
	}
	variance /= count
	stdDev := math.Sqrt(variance)
	return mean + 1.5*stdDev
}

// Utility: GetBeatTimes returns the times at which each beat occurs, for reference.
func (m *Model) GetBeatTimes() []time.Duration {
	if m.SampleRate <= 0 {
		return nil
	}
	var beats []time.Duration
	frameDur := time.Duration(float64(m.hopSize) / float64(m.SampleRate) * float64(time.Second))
	for i, isBeat := range m.BeatOnsets {
		if isBeat {
			beats = append(beats, time.Duration(i)*frameDur)
		}
	}
	return beats
}

// GetFrequencyResponse returns the FFT frequency bins at a particular time offset.
func (m *Model) GetFrequencyResponse(ts time.Duration) []float64 {
	if m.SampleRate <= 0 || m.hopSize <= 0 {
		return nil
	}
	index := int(ts.Seconds() * float64(m.SampleRate) / float64(m.hopSize))
	if index < 0 || index >= len(m.FFTData) {
		return nil
	}
	return m.FFTData[index]
}

// GetEnvelopeSegment returns a slice of RMS energy between two timestamps, for advanced use.
func (m *Model) GetEnvelopeSegment(start, end time.Duration) []float64 {
	if m.SampleRate <= 0 || m.hopSize <= 0 {
		return nil
	}
	startIndex := int(start.Seconds() * float64(m.SampleRate) / float64(m.hopSize))
	endIndex := int(end.Seconds() * float64(m.SampleRate) / float64(m.hopSize))

	if startIndex < 0 {
		startIndex = 0
	}
	if endIndex >= len(m.RMSEnergy) {
		endIndex = len(m.RMSEnergy) - 1
	}
	if startIndex > endIndex {
		return nil
	}
	return m.RMSEnergy[startIndex:endIndex]
}

// GetSpectralCentroid returns an array of frequency centroids for frames within [start, end].
func (m *Model) GetSpectralCentroid(start, end time.Duration) []float64 {
	if m.SampleRate <= 0 || m.hopSize <= 0 {
		return nil
	}
	startFrame := int(start.Seconds() * float64(m.SampleRate) / float64(m.hopSize))
	endFrame := int(end.Seconds() * float64(m.SampleRate) / float64(m.hopSize))
	if startFrame < 0 {
		startFrame = 0
	}
	if endFrame >= len(m.FFTData) {
		endFrame = len(m.FFTData) - 1
	}
	if startFrame > endFrame {
		return nil
	}

	centroids := make([]float64, endFrame-startFrame)
	for i := range centroids {
		var weightedSum, total float64
		for j, freq := range m.FreqBands {
			val := m.FFTData[startFrame+i][j]
			weightedSum += freq * val
			total += val
		}
		if total > 0 {
			centroids[i] = weightedSum / total
		}
	}
	return centroids
}
