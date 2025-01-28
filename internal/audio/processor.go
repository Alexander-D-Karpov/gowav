package audio

import (
	"fmt"
	"gowav/pkg/viz"
	"strings"
	"sync"
	"time"
)

// ProcessingState enumerates the high-level states of the audio Processor (idle, loading, or analyzing).
type ProcessingState int

const (
	StateIdle ProcessingState = iota
	StateLoading
	StateAnalyzing
)

// ProcessingStatus provides progress or error messages for the current stage.
type ProcessingStatus struct {
	State       ProcessingState
	Message     string
	Progress    float64
	CanCancel   bool
	StartTime   time.Time
	BytesLoaded int64
	TotalBytes  int64
}

// Processor is responsible for loading audio data, extracting metadata, running analysis, and managing visualizations.
type Processor struct {
	mu sync.RWMutex

	currentFile []byte
	metadata    *Metadata
	audioModel  *Model

	vizManager     *viz.Manager
	analysisDone   bool
	analysisCancel chan struct{}

	status      ProcessingStatus
	analyzedFor map[viz.ViewMode]bool
	vizCache    map[viz.ViewMode]bool
}

// NewProcessor creates a Processor with a fresh Viz Manager and no current track loaded.
func NewProcessor() *Processor {
	return &Processor{
		vizManager:     viz.NewManager(),
		analyzedFor:    make(map[viz.ViewMode]bool),
		vizCache:       make(map[viz.ViewMode]bool),
		analysisCancel: make(chan struct{}),
	}
}

// LoadFile asynchronously loads (and decodes) an audio file or URL.
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

// SwitchVisualization either returns a cached visualization or triggers analysis creation in a background goroutine.
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
		// Already have that visualization
		err := p.vizManager.SetMode(mode)
		p.mu.RUnlock()
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Switched to %s visualization", getModeName(mode)), nil
	}
	p.mu.RUnlock()

	// Otherwise, run analysis + build the visualization in background
	go func() {
		if err := p.analyzeAndCreateVisualization(mode); err != nil {
			logDebug("analyzeAndCreateVisualization failed for %v: %v", mode, err)
		}
	}()

	return fmt.Sprintf("Preparing %s visualization...", getModeName(mode)), nil
}

// analyzeAndCreateVisualization runs the needed analysis steps and attaches a Visualization to the Manager.
func (p *Processor) analyzeAndCreateVisualization(mode viz.ViewMode) error {
	p.mu.Lock()
	if p.audioModel == nil {
		p.audioModel = NewModel(p.metadata.SampleRate)
		logDebug("Created new audio model with sample rate: %d", p.metadata.SampleRate)
	}

	p.status = ProcessingStatus{
		State:     StateAnalyzing,
		Message:   fmt.Sprintf("Analyzing for %s visualization...", getModeName(mode)),
		Progress:  0,
		CanCancel: true,
		StartTime: time.Now(),
	}
	currentFile := p.currentFile
	cancelChan := p.analysisCancel
	p.mu.Unlock()

	startAll := time.Now()
	err := p.runRequiredAnalysis(mode, currentFile, cancelChan)
	if err != nil {
		p.setError(fmt.Sprintf("analysis failed: %v", err))
		return err
	}
	logDebug("%s completed in %v", getModeName(mode), time.Since(startAll))

	p.mu.Lock()
	defer p.mu.Unlock()

	var visualization viz.Visualization
	switch mode {
	case viz.WaveformMode:
		visualization = viz.CreateWaveformViz(p.audioModel.RawData, p.audioModel.SampleRate)
	case viz.SpectrogramMode:
		visualization = viz.NewSpectrogramViz(p.audioModel.FFTData, p.audioModel.FreqBands, p.audioModel.SampleRate)
	case viz.TempoMode:
		visualization = viz.NewTempoViz(
			p.audioModel.BeatData,
			p.audioModel.RMSEnergy,
			p.audioModel.SampleRate,
		)
	case viz.BeatMapMode:
		visualization = viz.NewBeatViz(p.audioModel.BeatData, p.audioModel.BeatOnsets, p.audioModel.EstimatedTempo, p.audioModel.SampleRate)
	case viz.DensityMode:
		visualization = viz.NewDensityViz(p.audioModel.RawData, p.audioModel.SampleRate)
	default:
		err := fmt.Errorf("unknown visualization mode: %v", mode)
		p.setError(err.Error())
		return err
	}

	// Update the actual track duration, in case our analysis discovered more accurate info
	actualDur := time.Duration(float64(len(p.audioModel.RawData)) / float64(p.audioModel.SampleRate) * float64(time.Second))
	if actualDur > p.metadata.Duration {
		p.metadata.Duration = actualDur
	}

	p.vizManager.SetTotalDuration(p.metadata.Duration)
	visualization.SetTotalDuration(p.metadata.Duration)
	p.vizManager.AddVisualization(mode, visualization)
	p.vizCache[mode] = true

	if err := p.vizManager.SetMode(mode); err != nil {
		p.setError(fmt.Sprintf("SetMode failed: %v", err))
		return err
	}

	p.status = ProcessingStatus{
		State:    StateIdle,
		Message:  fmt.Sprintf("%s visualization ready", getModeName(mode)),
		Progress: 1.0,
	}
	logDebug("Created %s visualization with duration %v", getModeName(mode), p.metadata.Duration)
	return nil
}

// runRequiredAnalysis checks which analysis steps are necessary for the requested visualization.
func (p *Processor) runRequiredAnalysis(mode viz.ViewMode, file []byte, cancelChan chan struct{}) error {
	progressFn := func(progress float64, msg string) {
		p.updateAnalysisProgress(progress, msg)
	}

	switch mode {
	case viz.WaveformMode:
		if len(p.audioModel.RawData) == 0 {
			if err := p.audioModel.AnalyzeWaveform(file, func(f float64) {
				progressFn(f, "Analyzing waveform...")
			}, cancelChan); err != nil {
				return err
			}
		}
	case viz.SpectrogramMode:
		if len(p.audioModel.RawData) == 0 {
			if err := p.audioModel.AnalyzeWaveform(file, func(f float64) {
				progressFn(0.3*f, "Analyzing waveform...")
			}, cancelChan); err != nil {
				return err
			}
		}
		if p.audioModel.FFTData == nil {
			if err := p.audioModel.AnalyzeSpectrum(func(f float64) {
				progressFn(0.3+0.7*f, "Computing frequency analysis...")
			}, cancelChan); err != nil {
				return err
			}
		}
	case viz.TempoMode:
		if len(p.audioModel.RawData) == 0 {
			if err := p.audioModel.AnalyzeWaveform(file, func(f float64) {
				progressFn(0.3*f, "Analyzing waveform...")
			}, cancelChan); err != nil {
				return err
			}
		}
		if p.audioModel.FFTData == nil {
			if err := p.audioModel.AnalyzeSpectrum(func(f float64) {
				progressFn(0.3+0.3*f, "Computing frequency analysis...")
			}, cancelChan); err != nil {
				return err
			}
		}
		if len(p.audioModel.BeatData) == 0 {
			if err := p.audioModel.AnalyzeBeats(func(f float64) {
				progressFn(0.6+0.4*f, "Detecting beats...")
			}, cancelChan); err != nil {
				return err
			}
		}
	case viz.BeatMapMode:
		if len(p.audioModel.RawData) == 0 {
			if err := p.audioModel.AnalyzeWaveform(file, func(f float64) {
				progressFn(0.3*f, "Analyzing waveform...")
			}, cancelChan); err != nil {
				return err
			}
		}
		if p.audioModel.FFTData == nil {
			if err := p.audioModel.AnalyzeSpectrum(func(f float64) {
				progressFn(0.3+0.3*f, "Computing frequency analysis...")
			}, cancelChan); err != nil {
				return err
			}
		}
		if len(p.audioModel.BeatData) == 0 {
			if err := p.audioModel.AnalyzeBeats(func(f float64) {
				progressFn(0.6+0.4*f, "Detecting beats...")
			}, cancelChan); err != nil {
				return err
			}
		}
	case viz.DensityMode:
		if len(p.audioModel.RawData) == 0 {
			if err := p.audioModel.AnalyzeWaveform(file, func(f float64) {
				progressFn(f, "Analyzing waveform...")
			}, cancelChan); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unsupported mode: %v", mode)
	}
	return nil
}

// updateAnalysisProgress modifies the processor status to show progress in the UI.
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
	var eta string
	if progress > 0 && progress < 1 {
		totalEstimate := elapsed.Seconds() / progress
		remaining := time.Duration((totalEstimate - elapsed.Seconds()) * float64(time.Second))
		eta = formatETA(remaining)
	} else {
		eta = "calculating..."
	}

	p.status = ProcessingStatus{
		State:     StateAnalyzing,
		Message:   fmt.Sprintf("%s (ETA: %s)", message, eta),
		Progress:  progress,
		CanCancel: true,
		StartTime: p.status.StartTime,
	}
}

// GetVisualization renders the current visualization or shows loading progress if not ready.
func (p *Processor) GetVisualization() string {
	p.mu.Lock()
	defer p.mu.Unlock()

	switch p.status.State {
	case StateAnalyzing:
		return fmt.Sprintf("%s\nProgress: %d%%\n", p.status.Message, int(p.status.Progress*100))
	case StateLoading:
		if p.status.TotalBytes > 0 {
			perc := float64(p.status.BytesLoaded) / float64(p.status.TotalBytes) * 100
			return fmt.Sprintf("Loading file... %.1f%%\n", perc)
		}
		return "Loading file...\n"
	}

	return p.vizManager.Render()
}

// HandleVisualizationInput processes keys for panning or zooming the current visualization, etc.
func (p *Processor) HandleVisualizationInput(key string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.status.State == StateLoading || p.status.State == StateAnalyzing {
		return false
	}

	if strings.HasPrefix(key, "resize:") {
		var w, h int
		fmt.Sscanf(strings.TrimPrefix(key, "resize:"), "%dx%d", &w, &h)
		p.vizManager.SetDimensions(w, h)
		return true
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

// GetStatus retrieves the current ProcessingStatus for the UI.
func (p *Processor) GetStatus() ProcessingStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.status
}

// GetMetadata returns the extracted Metadata for the loaded track (if any).
func (p *Processor) GetMetadata() *Metadata {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.metadata
}

// GetCurrentFile returns the raw bytes of the loaded audio file.
func (p *Processor) GetCurrentFile() []byte {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.currentFile
}

// CancelProcessing stops any ongoing analysis or file loading by closing the cancel channel.
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

// getModeName returns a string for each known ViewMode to display in UI messages.
func getModeName(mode viz.ViewMode) string {
	switch mode {
	case viz.WaveformMode:
		return "waveform"
	case viz.SpectrogramMode:
		return "spectrogram"
	case viz.TempoMode:
		return "tempo"
	case viz.BeatMapMode:
		return "beat"
	case viz.DensityMode:
		return "density"
	default:
		return "unknown"
	}
}
