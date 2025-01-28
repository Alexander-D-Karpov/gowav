package audio

import (
	"fmt"
	"gowav/pkg/viz"
	"strings"
	"sync"
	"time"
)

type ProcessingState int

const (
	StateIdle ProcessingState = iota
	StateLoading
	StateAnalyzing
)

type ProcessingStatus struct {
	State       ProcessingState
	Message     string
	Progress    float64
	CanCancel   bool
	StartTime   time.Time
	BytesLoaded int64
	TotalBytes  int64
}

type Processor struct {
	mu sync.RWMutex

	currentFile []byte
	metadata    *Metadata
	audioModel  *Model
	vizManager  *viz.Manager

	status         ProcessingStatus
	analysisCancel chan struct{}
	analysisDone   bool

	analyzedFor map[viz.ViewMode]bool
	vizCache    map[viz.ViewMode]bool
}

func NewProcessor() *Processor {
	return &Processor{
		vizManager:     viz.NewManager(),
		analyzedFor:    make(map[viz.ViewMode]bool),
		vizCache:       make(map[viz.ViewMode]bool),
		analysisCancel: make(chan struct{}),
	}
}

func (p *Processor) LoadFile(path string) error {
	logDebug("Starting to load file: %s", path)
	p.CancelProcessing()

	p.mu.Lock()
	p.currentFile = nil
	p.metadata = nil
	p.audioModel = nil
	p.analyzedFor = make(map[viz.ViewMode]bool)
	p.vizCache = make(map[viz.ViewMode]bool)

	p.status = ProcessingStatus{
		State:     StateLoading,
		Message:   "Loading file...",
		Progress:  0,
		CanCancel: true,
		StartTime: time.Now(),
	}
	cancelChan := p.analysisCancel
	p.mu.Unlock()

	go func() {
		var fileData []byte
		var err error

		if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
			fileData, err = p.loadFromURL(path, cancelChan)
		} else {
			fileData, err = p.loadFromFile(path, cancelChan)
		}

		if err != nil {
			p.setLoadError(fmt.Sprintf("Load failed: %v", err))
			return
		}

		md, err := ExtractMetadata(fileData)
		if err != nil {
			p.setLoadError(fmt.Sprintf("Metadata extraction failed: %v", err))
			return
		}

		p.mu.Lock()
		p.currentFile = fileData
		p.metadata = md
		p.audioModel = nil
		p.analysisDone = false
		p.status = ProcessingStatus{
			State:    StateIdle,
			Message:  "File loaded successfully",
			Progress: 1.0,
		}
		p.mu.Unlock()
	}()

	return nil
}

func (p *Processor) SwitchVisualization(mode viz.ViewMode) (string, error) {
	p.mu.RLock()
	if p.status.State == StateAnalyzing {
		msg := p.status.Message
		p.mu.RUnlock()
		return "", fmt.Errorf("analysis in progress: %s", msg)
	}
	if len(p.currentFile) == 0 {
		p.mu.RUnlock()
		return "", fmt.Errorf("no audio data available")
	}
	if p.vizCache[mode] {
		err := p.vizManager.SetMode(mode)
		p.mu.RUnlock()
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Switched to %s visualization", getModeName(mode)), nil
	}
	p.mu.RUnlock()

	resultChan := make(chan error, 1)
	go func() {
		err := p.analyzeAndCreateVisualization(mode)
		resultChan <- err
	}()

	select {
	case err := <-resultChan:
		if err != nil {
			return "", fmt.Errorf("failed to start analysis: %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		// Analysis started
	}

	return fmt.Sprintf("Preparing %s visualization...", getModeName(mode)), nil
}

func (p *Processor) analyzeAndCreateVisualization(mode viz.ViewMode) error {
	p.mu.Lock()
	if p.audioModel == nil {
		p.audioModel = NewModel(p.metadata.SampleRate)
		logDebug("Created new audio model with sample rate: %d", p.metadata.SampleRate)
	}

	p.status = ProcessingStatus{
		State:     StateAnalyzing,
		Message:   fmt.Sprintf("Preparing %s visualization...", getModeName(mode)),
		Progress:  0,
		CanCancel: true,
		StartTime: time.Now(),
	}
	currentFile := p.currentFile
	cancelChan := p.analysisCancel
	p.mu.Unlock()

	errChan := make(chan error, 3)
	doneChan := make(chan struct{})
	var wg sync.WaitGroup
	cleanupOnce := sync.Once{}

	cleanup := func() {
		cleanupOnce.Do(func() {
			select {
			case <-errChan:
			default:
				close(errChan)
			}
			select {
			case <-doneChan:
			default:
				close(doneChan)
			}
		})
	}
	defer cleanup()

	runAnalysisStage := func(name string, fn func() error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			start := time.Now()
			err := fn()
			if err != nil {
				select {
				case errChan <- fmt.Errorf("%s failed: %w", name, err):
				default:
				}
				return
			}
			logDebug("%s completed in %v", name, time.Since(start))
		}()
	}

	// Choose analysis steps
	switch mode {
	case viz.WaveformMode:
		if len(p.audioModel.RawData) == 0 {
			runAnalysisStage("waveform", func() error {
				return p.audioModel.AnalyzeWaveform(
					currentFile,
					func(progress float64) {
						p.updateAnalysisProgress(progress, "Analyzing waveform...")
					},
					cancelChan,
				)
			})
		}

	case viz.SpectrogramMode:
		if len(p.audioModel.RawData) == 0 {
			runAnalysisStage("waveform", func() error {
				return p.audioModel.AnalyzeWaveform(
					currentFile,
					func(progress float64) {
						p.updateAnalysisProgress(progress*0.4, "Analyzing waveform...")
					},
					cancelChan,
				)
			})
		}
		if p.audioModel.FFTData == nil {
			runAnalysisStage("spectrum", func() error {
				return p.audioModel.AnalyzeSpectrum(
					func(progress float64) {
						p.updateAnalysisProgress(0.4+progress*0.6, "Computing frequency analysis...")
					},
					cancelChan,
				)
			})
		}

	case viz.TempoMode:
		if len(p.audioModel.RawData) == 0 {
			runAnalysisStage("waveform", func() error {
				return p.audioModel.AnalyzeWaveform(
					currentFile,
					func(progress float64) {
						p.updateAnalysisProgress(progress*0.3, "Analyzing waveform...")
					},
					cancelChan,
				)
			})
		}
		if p.audioModel.FFTData == nil {
			runAnalysisStage("spectrum", func() error {
				return p.audioModel.AnalyzeSpectrum(
					func(progress float64) {
						p.updateAnalysisProgress(0.3+progress*0.3, "Computing frequency analysis...")
					},
					cancelChan,
				)
			})
		}
		if len(p.audioModel.BeatData) == 0 {
			runAnalysisStage("beats", func() error {
				return p.audioModel.AnalyzeBeats(
					func(progress float64) {
						p.updateAnalysisProgress(0.6+progress*0.4, "Detecting beats...")
					},
					cancelChan,
				)
			})
		}

	case viz.BeatMapMode:
		if len(p.audioModel.RawData) == 0 {
			runAnalysisStage("waveform", func() error {
				return p.audioModel.AnalyzeWaveform(
					currentFile,
					func(progress float64) {
						p.updateAnalysisProgress(progress*0.3, "Analyzing waveform...")
					},
					cancelChan,
				)
			})
		}
		if p.audioModel.FFTData == nil {
			runAnalysisStage("spectrum", func() error {
				return p.audioModel.AnalyzeSpectrum(
					func(progress float64) {
						p.updateAnalysisProgress(0.3+progress*0.3, "Computing frequency analysis...")
					},
					cancelChan,
				)
			})
		}
		if len(p.audioModel.BeatData) == 0 {
			runAnalysisStage("beats", func() error {
				return p.audioModel.AnalyzeBeats(
					func(progress float64) {
						p.updateAnalysisProgress(0.6+progress*0.4, "Detecting beats...")
					},
					cancelChan,
				)
			})
		}

	case viz.DensityMode:
		if len(p.audioModel.RawData) == 0 {
			runAnalysisStage("waveform", func() error {
				return p.audioModel.AnalyzeWaveform(
					currentFile,
					func(progress float64) {
						p.updateAnalysisProgress(progress, "Analyzing waveform...")
					},
					cancelChan,
				)
			})
		}

	default:
		return fmt.Errorf("unsupported visualization mode: %v", mode)
	}

	// Wait for completion or error
	completed := make(chan struct{})
	go func() {
		wg.Wait()
		close(completed)
	}()

	select {
	case err := <-errChan:
		return err
	case <-completed:
	case <-cancelChan:
		return fmt.Errorf("analysis cancelled")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Correct actual duration from samples if needed
	actualDuration := time.Duration(float64(len(p.audioModel.RawData)) / float64(p.audioModel.SampleRate) * float64(time.Second))
	if actualDuration > p.metadata.Duration {
		logDebug("Actual duration (%v) differs from metadata (%v), using actual", actualDuration, p.metadata.Duration)
		p.metadata.Duration = actualDuration
	}

	var v viz.Visualization
	switch mode {
	case viz.WaveformMode:
		v = viz.CreateWaveformViz(p.audioModel.RawData, p.audioModel.SampleRate)
	case viz.SpectrogramMode:
		v = viz.NewSpectrogramViz(p.audioModel.FFTData, p.audioModel.FreqBands, p.audioModel.SampleRate)
	case viz.TempoMode:
		v = viz.NewTempoViz(p.audioModel.BeatData, p.audioModel.RawData, p.audioModel.SampleRate)
	case viz.BeatMapMode:
		v = viz.NewBeatViz(p.audioModel.BeatData, p.audioModel.BeatOnsets, p.audioModel.EstimatedTempo, p.audioModel.SampleRate)
	case viz.DensityMode:
		v = viz.NewDensityViz(p.audioModel.RawData, p.audioModel.SampleRate)
	}

	if v == nil {
		return fmt.Errorf("failed to create visualization")
	}

	// Set total duration in Visualization
	p.vizManager.SetTotalDuration(p.metadata.Duration)
	v.SetTotalDuration(p.metadata.Duration)

	// Add and switch to mode
	p.vizManager.AddVisualization(mode, v)
	p.vizCache[mode] = true

	if err := p.vizManager.SetMode(mode); err != nil {
		return fmt.Errorf("failed to set visualization mode: %v", err)
	}

	// Done
	p.status = ProcessingStatus{
		State:    StateIdle,
		Message:  fmt.Sprintf("%s visualization ready", getModeName(mode)),
		Progress: 1.0,
	}

	logDebug("Created %s visualization with duration %v", getModeName(mode), p.metadata.Duration)
	return nil
}

// CancelProcessing cancels any ongoing analysis or load.
func (p *Processor) CancelProcessing() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.analysisCancel != nil {
		close(p.analysisCancel)
	}
	p.analysisCancel = make(chan struct{})

	p.status = ProcessingStatus{
		State:    StateIdle,
		Message:  "Processing cancelled",
		Progress: 0,
	}
}

// GetVisualization returns the current visualization output.
func (p *Processor) GetVisualization() string {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.status.State == StateAnalyzing {
		return fmt.Sprintf("%s\nProgress: %d%%\n",
			p.status.Message,
			int(p.status.Progress*100))
	}

	if p.status.State == StateLoading {
		if p.status.TotalBytes > 0 {
			percent := float64(p.status.BytesLoaded) / float64(p.status.TotalBytes) * 100
			return fmt.Sprintf("Loading file... %.1f%%\n", percent)
		}
		return "Loading file...\n"
	}

	return p.vizManager.Render()
}

// HandleVisualizationInput processes key commands for visualization.
func (p *Processor) HandleVisualizationInput(key string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.status.State == StateLoading || p.status.State == StateAnalyzing {
		return false
	}

	if strings.HasPrefix(key, "resize:") {
		parts := strings.Split(strings.TrimPrefix(key, "resize:"), "x")
		if len(parts) == 2 {
			var width, height int
			fmt.Sscanf(parts[0], "%d", &width)
			fmt.Sscanf(parts[1], "%d", &height)
			if p.vizManager != nil {
				p.vizManager.SetDimensions(width, height)
				return true
			}
		}
		return false
	}

	if p.vizManager != nil {
		switch key {
		case "next":
			m, _ := p.vizManager.CycleMode(1)
			p.status.Message = fmt.Sprintf("Switched to %s visualization", m)
			return true
		case "prev":
			m, _ := p.vizManager.CycleMode(-1)
			p.status.Message = fmt.Sprintf("Switched to %s visualization", m)
			return true
		case "zoom-in":
			p.vizManager.UpdateZoom(1.2)
			return true
		case "zoom-out":
			p.vizManager.UpdateZoom(0.8)
			return true
		case "left":
			p.vizManager.UpdateOffset(-time.Second)
			return true
		case "right":
			p.vizManager.UpdateOffset(time.Second)
			return true
		case "reset":
			p.vizManager.Reset()
			return true
		}
	}
	return false
}

// GetStatus returns current loading/analysis state.
func (p *Processor) GetStatus() ProcessingStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.status
}

// GetMetadata returns the extracted metadata (if any).
func (p *Processor) GetMetadata() *Metadata {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.metadata
}

// GetCurrentFile returns the raw audio data.
func (p *Processor) GetCurrentFile() []byte {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.currentFile
}

func getModeName(mode viz.ViewMode) string {
	switch mode {
	case viz.WaveformMode:
		return "waveform"
	case viz.SpectrogramMode:
		return "spectrogram"
	case viz.TempoMode:
		return "tempo"
	case viz.BeatMapMode:
		return "beatmap"
	case viz.DensityMode:
		return "density"
	default:
		return "unknown"
	}
}

func (p *Processor) updateAnalysisProgress(progress float64, message string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}

	elapsed := time.Since(p.status.StartTime)
	var etaMsg string
	if progress > 0 && progress < 1 {
		totalEstimate := elapsed.Seconds() / progress
		remaining := time.Duration((totalEstimate - elapsed.Seconds()) * float64(time.Second))
		etaMsg = formatETA(remaining)
	} else {
		etaMsg = "calculating..."
	}

	p.status = ProcessingStatus{
		State:     StateAnalyzing,
		Message:   fmt.Sprintf("%s (ETA: %s)", message, etaMsg),
		Progress:  progress,
		CanCancel: true,
		StartTime: p.status.StartTime,
	}
}
