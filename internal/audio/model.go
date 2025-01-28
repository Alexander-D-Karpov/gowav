package audio

import (
	"fmt"
	"math"
	"runtime"
	"sync"
	"time"

	"gonum.org/v1/gonum/dsp/fourier"
	"gonum.org/v1/gonum/floats"
)

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

func NewModel(sampleRate int) *Model {
	return &Model{
		SampleRate: sampleRate,
		// Default values
		windowSize: 2048,
		hopSize:    512,
		fftSize:    2048,
	}
}

func (m *Model) SetParameters(windowSize, hopSize, fftSize int) {
	m.windowSize = windowSize
	m.hopSize = hopSize
	m.fftSize = fftSize
}

// AnalyzeWaveform converts raw bytes to float64 samples and performs basic analysis
func (m *Model) AnalyzeWaveform(rawBytes []byte, progressFn func(float64), cancelChan chan struct{}) error {
	startTotal := time.Now()

	dataLen := len(rawBytes)
	if dataLen < 2 {
		return fmt.Errorf("data too short")
	}

	numCPU := runtime.NumCPU()
	chunkSize := dataLen / (2 * numCPU)
	if chunkSize < 1024 {
		chunkSize = 1024
	}

	m.RawData = make([]float64, dataLen/2)

	var wg sync.WaitGroup
	errChan := make(chan error, numCPU)

	// We'll process in chunkSize blocks of *samples* (so chunkSize is for floats)
	chunks := (dataLen / 2) / chunkSize
	if (dataLen/2)%chunkSize != 0 {
		chunks++
	}

	for c := 0; c < chunks; c++ {
		wg.Add(1)
		start := c * chunkSize
		end := (c + 1) * chunkSize
		if end > (dataLen / 2) {
			end = dataLen / 2
		}

		go func(start, end int) {
			defer wg.Done()
			select {
			case <-cancelChan:
				errChan <- fmt.Errorf("cancelled")
				return
			default:
			}

			chunkStartTime := time.Now()
			sliceLen := end - start
			tmpBuf := make([]float64, sliceLen)

			rawOffset := start * 2
			for i := 0; i < sliceLen; i++ {
				val := int16(rawBytes[rawOffset]) | int16(rawBytes[rawOffset+1])<<8
				tmpBuf[i] = float64(val)
				rawOffset += 2
			}

			// Scale using gonum
			floats.Scale(1.0/32768.0, tmpBuf)
			copy(m.RawData[start:end], tmpBuf)

			elapsed := time.Since(chunkStartTime)
			logDebug("Chunk %d samples processed in %v", sliceLen, elapsed)

			progress := float64(end) / float64(dataLen/2)
			progressFn(progress)
		}(start, end)
	}

	wg.Wait()

	select {
	case err := <-errChan:
		return fmt.Errorf("waveform analysis error: %w", err)
	default:
		totalElapsed := time.Since(startTotal)
		logDebug("Total waveform analysis time: %v", totalElapsed)
		return nil
	}
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

	for i := 0; i < numCPU; i++ {
		wg.Add(1)
		go m.fftWorker(realFFT, windowChan, &wg, progressFn, cancelChan, errChan)
	}

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

	select {
	case err := <-errChan:
		return fmt.Errorf("spectrum analysis error: %w", err)
	default:
		return m.calculateSpectralFeatures(cancelChan)
	}
}

func (m *Model) initFrequencyBands() {
	m.FreqBands = make([]float64, m.fftSize/2)
	nyquist := float64(m.SampleRate) / 2.0
	for i := range m.FreqBands {
		m.FreqBands[i] = float64(i) * nyquist / float64(m.fftSize/2)
	}
}

func (m *Model) fftWorker(
	realFFT *fourier.FFT,
	windowChan chan int,
	wg *sync.WaitGroup,
	progressFn func(float64),
	cancelChan chan struct{},
	errChan chan error,
) {
	defer wg.Done()
	windowed := make([]float64, m.fftSize)

	for windowIdx := range windowChan {
		select {
		case <-cancelChan:
			errChan <- fmt.Errorf("cancelled")
			return
		default:
		}

		startSample := windowIdx * m.hopSize
		if startSample+m.windowSize > len(m.RawData) {
			errChan <- fmt.Errorf("invalid window index")
			return
		}

		for i := 0; i < m.fftSize; i++ {
			if i < m.windowSize {
				w := 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/float64(m.windowSize)))
				windowed[i] = m.RawData[startSample+i] * w
			} else {
				windowed[i] = 0.0
			}
		}

		spectrum := realFFT.Coefficients(nil, windowed)
		for freq := 0; freq < m.fftSize/2; freq++ {
			re := real(spectrum[freq])
			im := imag(spectrum[freq])
			m.FFTData[windowIdx][freq] = math.Sqrt(re*re + im*im)
		}

		if windowIdx%(len(m.FFTData)/100+1) == 0 {
			progressFn(float64(windowIdx) / float64(len(m.FFTData)))
		}
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
		}
		if i > 0 {
			m.SpectralFlux[i] = m.calculateFlux(m.FFTData[i], m.FFTData[i-1])
		}
		m.PeakFrequencies[i] = m.findPeakFrequency(m.FFTData[i])
		m.RMSEnergy[i] = m.calculateRMSEnergy(m.FFTData[i])
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

// AnalyzeBeats performs beat and tempo analysis
func (m *Model) AnalyzeBeats(progressFn func(float64), cancelChan chan struct{}) error {
	if len(m.FFTData) == 0 {
		err := m.AnalyzeSpectrum(func(p float64) {
			progressFn(p * 0.5)
		}, cancelChan)
		if err != nil {
			return err
		}
	}

	numFrames := len(m.FFTData)
	m.BeatData = make([]float64, numFrames)
	m.BeatOnsets = make([]bool, numFrames)

	if err := m.calculateOnsetFunction(progressFn, cancelChan); err != nil {
		return err
	}
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
			history := make([]float64, 43)
			histPos := 0

			for idx := start; idx < end; idx++ {
				select {
				case <-cancelChan:
					errChan <- fmt.Errorf("cancelled")
					return
				default:
				}
				var energy float64
				for freq := 0; freq < len(m.FFTData[idx]); freq++ {
					if freq < m.fftSize/4 {
						energy += m.FFTData[idx][freq] * m.FFTData[idx][freq]
					}
				}
				energy = math.Sqrt(energy)
				m.BeatData[idx] = energy
				history[histPos] = energy
				histPos = (histPos + 1) % len(history)

				var sum, count float64
				for _, e := range history {
					if e > 0 {
						sum += e
						count++
					}
				}
				threshold := (sum / count) * 1.3
				m.BeatOnsets[idx] = energy > threshold
			}

			localProgress := float64(end-start) / float64(len(m.FFTData))
			progressFn(0.5 + localProgress*0.5)
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
		return nil
	}

	hist := make(map[int]int)
	for _, interval := range intervals {
		bucket := int(math.Round(interval))
		hist[bucket]++
	}

	maxCount := 0
	bestInterval := 0
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
	return nil
}

func (m *Model) refineBeatDetection(progressFn func(float64), cancelChan chan struct{}) {
	framesPerBeat := (60.0 / m.EstimatedTempo) * (float64(m.SampleRate) / float64(m.hopSize))
	searchWindow := int(framesPerBeat * 0.1)
	refined := make([]bool, len(m.BeatOnsets))

	var firstBeat int
	for i, isBeat := range m.BeatOnsets {
		if isBeat {
			firstBeat = i
			refined[i] = true
			break
		}
	}

	expectedPos := float64(firstBeat)
	for expectedPos < float64(len(m.BeatOnsets)) {
		select {
		case <-cancelChan:
			return
		default:
		}
		pos := int(math.Round(expectedPos))
		start := pos - searchWindow
		if start < 0 {
			start = 0
		}
		end := pos + searchWindow
		if end >= len(m.BeatOnsets) {
			end = len(m.BeatOnsets) - 1
		}

		maxEnergy := 0.0
		maxPos := pos
		for i := start; i <= end; i++ {
			if m.BeatData[i] > maxEnergy {
				maxEnergy = m.BeatData[i]
				maxPos = i
			}
		}
		threshold := m.calculateLocalThreshold(maxPos)
		if maxEnergy > threshold {
			refined[maxPos] = true
		}
		expectedPos += framesPerBeat
	}
	m.BeatOnsets = refined
}

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

func (m *Model) GetBeatTimes() []time.Duration {
	var result []time.Duration
	frameDur := time.Duration(float64(m.hopSize) / float64(m.SampleRate) * float64(time.Second))
	for i, isBeat := range m.BeatOnsets {
		if isBeat {
			result = append(result, time.Duration(i)*frameDur)
		}
	}
	return result
}

func (m *Model) GetFrequencyResponse(timestamp time.Duration) []float64 {
	frameIndex := int(timestamp.Seconds() * float64(m.SampleRate) / float64(m.hopSize))
	if frameIndex < 0 || frameIndex >= len(m.FFTData) {
		return nil
	}
	return m.FFTData[frameIndex]
}

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
		var weightedSum, total float64
		for j, freq := range m.FreqBands {
			e := m.FFTData[startFrame+i][j]
			weightedSum += freq * e
			total += e
		}
		if total > 0 {
			centroids[i] = weightedSum / total
		}
	}
	return centroids
}
