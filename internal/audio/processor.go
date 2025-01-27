// internal/audio/processor.go

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
	// First check if we can switch
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

	// If already analyzed, just switch
	if p.vizCache[mode] {
		err := p.vizManager.SetMode(mode)
		p.mu.RUnlock()
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Switched to %s visualization", getModeName(mode)), nil
	}
	p.mu.RUnlock()

	// Start analysis in a separate goroutine
	resultChan := make(chan error, 1)
	go func() {
		err := p.analyzeAndCreateVisualization(mode)
		resultChan <- err
	}()

	// Wait for initial analysis to start
	select {
	case err := <-resultChan:
		if err != nil {
			return "", fmt.Errorf("failed to start analysis: %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		// Analysis started successfully
	}

	return fmt.Sprintf("Preparing %s visualization...", getModeName(mode)), nil
}

func (p *Processor) analyzeAndCreateVisualization(mode viz.ViewMode) error {
	p.mu.Lock()
	// Create new audio model if needed
	if p.audioModel == nil {
		p.audioModel = NewModel(p.metadata.SampleRate)
		logDebug("Created new audio model with sample rate: %d", p.metadata.SampleRate)
	}

	// Update status
	p.status = ProcessingStatus{
		State:     StateAnalyzing,
		Message:   fmt.Sprintf("Preparing %s visualization...", getModeName(mode)),
		Progress:  0,
		CanCancel: true,
		StartTime: time.Now(),
	}
	p.mu.Unlock()

	// Do the analysis outside the lock
	err := p.initializeAnalysis(mode)
	if err != nil {
		p.setError(fmt.Sprintf("Analysis failed: %v", err))
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Create visualization
	var v viz.Visualization
	switch mode {
	case viz.WaveformMode:
		if len(p.audioModel.RawData) == 0 {
			return fmt.Errorf("waveform analysis incomplete")
		}
		v = viz.CreateWaveformViz(p.audioModel.RawData, p.audioModel.SampleRate)

	case viz.SpectrogramMode:
		if p.audioModel.FFTData == nil || len(p.audioModel.FFTData) == 0 {
			return fmt.Errorf("spectrum analysis incomplete")
		}
		v = viz.NewSpectrogramViz(
			p.audioModel.FFTData,
			p.audioModel.FreqBands,
			p.audioModel.SampleRate,
		)

	case viz.TempoMode:
		if len(p.audioModel.BeatData) == 0 {
			return fmt.Errorf("tempo analysis incomplete")
		}
		v = viz.NewTempoViz(
			p.audioModel.BeatData,
			p.audioModel.RawData,
			p.audioModel.SampleRate,
		)

	case viz.BeatMapMode:
		if len(p.audioModel.BeatOnsets) == 0 {
			return fmt.Errorf("beat analysis incomplete")
		}
		v = viz.NewBeatViz(
			p.audioModel.BeatData,
			p.audioModel.BeatOnsets,
			p.audioModel.EstimatedTempo,
			p.audioModel.SampleRate,
		)
	}

	if v == nil {
		return fmt.Errorf("failed to create visualization")
	}

	v.SetTotalDuration(p.metadata.Duration)
	p.vizManager.AddVisualization(mode, v)
	p.vizCache[mode] = true

	err = p.vizManager.SetMode(mode)
	if err != nil {
		return fmt.Errorf("failed to set visualization mode: %v", err)
	}

	p.status = ProcessingStatus{
		State:    StateIdle,
		Message:  fmt.Sprintf("%s visualization ready", getModeName(mode)),
		Progress: 1.0,
	}

	return nil
}

func (p *Processor) initializeAnalysis(mode viz.ViewMode) error {
	// Check what analysis we need
	p.mu.Lock()
	if p.audioModel == nil {
		p.audioModel = NewModel(p.metadata.SampleRate)
		logDebug("Created new audio model with sample rate: %d", p.metadata.SampleRate)
	}

	var analysisNeeded bool
	switch mode {
	case viz.WaveformMode:
		analysisNeeded = len(p.audioModel.RawData) == 0
	case viz.SpectrogramMode:
		analysisNeeded = p.audioModel.FFTData == nil || len(p.audioModel.FFTData) == 0
	case viz.TempoMode, viz.BeatMapMode:
		analysisNeeded = len(p.audioModel.BeatData) == 0
	}

	// Get required data while holding lock
	currentFile := p.currentFile
	audioModel := p.audioModel
	cancelChan := p.analysisCancel
	p.mu.Unlock()

	if !analysisNeeded {
		return nil
	}

	// Do analysis outside lock
	var err error
	switch mode {
	case viz.WaveformMode:
		logDebug("Starting waveform analysis...")
		err = audioModel.AnalyzeWaveform(
			currentFile,
			func(progress float64) {
				p.updateAnalysisProgress(progress, "Analyzing waveform...")
			},
			cancelChan,
		)
		if err != nil {
			return fmt.Errorf("waveform analysis failed: %w", err)
		}

	case viz.SpectrogramMode:
		// First ensure we have waveform data
		if len(audioModel.RawData) == 0 {
			logDebug("Spectrum analysis requires waveform data, analyzing waveform first...")
			err = audioModel.AnalyzeWaveform(
				currentFile,
				func(progress float64) {
					p.updateAnalysisProgress(progress*0.5, "Preparing waveform data...")
				},
				cancelChan,
			)
			if err != nil {
				return fmt.Errorf("waveform analysis failed: %w", err)
			}
		}

		logDebug("Starting spectrum analysis...")
		err = audioModel.AnalyzeSpectrum(
			func(progress float64) {
				p.updateAnalysisProgress(0.5+progress*0.5, "Computing frequency analysis...")
			},
			cancelChan,
		)
		if err != nil {
			return fmt.Errorf("spectrum analysis failed: %w", err)
		}

	case viz.TempoMode, viz.BeatMapMode:
		// Tempo analysis requires spectrum data
		if audioModel.FFTData == nil || len(audioModel.FFTData) == 0 {
			// First get waveform if needed
			if len(audioModel.RawData) == 0 {
				logDebug("Tempo analysis requires waveform data, analyzing waveform first...")
				err = audioModel.AnalyzeWaveform(
					currentFile,
					func(progress float64) {
						p.updateAnalysisProgress(progress*0.3, "Preparing waveform data...")
					},
					cancelChan,
				)
				if err != nil {
					return fmt.Errorf("waveform analysis failed: %w", err)
				}
			}

			// Then get spectrum
			logDebug("Tempo analysis requires spectrum data, computing spectrum...")
			err = audioModel.AnalyzeSpectrum(
				func(progress float64) {
					p.updateAnalysisProgress(0.3+progress*0.3, "Computing frequency analysis...")
				},
				cancelChan,
			)
			if err != nil {
				return fmt.Errorf("spectrum analysis failed: %w", err)
			}
		}

		logDebug("Starting tempo/beat analysis...")
		err = audioModel.AnalyzeBeats(
			func(progress float64) {
				p.updateAnalysisProgress(0.6+progress*0.4, "Detecting beats and tempo...")
			},
			cancelChan,
		)
		if err != nil {
			return fmt.Errorf("beat analysis failed: %w", err)
		}

	default:
		return fmt.Errorf("unknown visualization mode: %d", mode)
	}

	p.mu.Lock()
	p.analyzedFor[mode] = true
	p.mu.Unlock()

	logDebug("Analysis completed for mode: %s", getModeName(mode))
	return nil
}

// Helper function for progress updates during analysis
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

func (p *Processor) GetStatus() ProcessingStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.status
}

func (p *Processor) GetMetadata() *Metadata {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.metadata
}

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
	default:
		return "unknown"
	}
}
