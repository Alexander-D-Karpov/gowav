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

	// Track which visualizations we've analyzed for
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
	p.CancelProcessing() // Stop any existing processing

	// Reset state
	p.mu.Lock()
	p.currentFile = nil
	p.metadata = nil
	p.audioModel = nil
	p.analyzedFor = make(map[viz.ViewMode]bool)
	p.vizCache = make(map[viz.ViewMode]bool)

	// Initialize loading state
	p.status = ProcessingStatus{
		State:       StateLoading,
		Message:     "Loading file...",
		Progress:    0,
		CanCancel:   true,
		StartTime:   time.Now(),
		BytesLoaded: 0,
		TotalBytes:  0,
	}
	cancelChan := p.analysisCancel
	p.mu.Unlock()

	// Start loading in goroutine
	go func() {
		var fileData []byte
		var err error

		if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
			logDebug("Loading from URL")
			fileData, err = p.loadFromURL(path, cancelChan)
		} else {
			logDebug("Loading from file")
			fileData, err = p.loadFromFile(path, cancelChan)
		}

		if err != nil {
			logDebug("Load failed: %v", err)
			p.setLoadError(fmt.Sprintf("Load failed: %v", err))
			return
		}

		logDebug("Successfully loaded %d bytes", len(fileData))

		// Extract metadata
		md, err := ExtractMetadata(fileData)
		if err != nil {
			logDebug("Metadata extraction failed: %v", err)
			p.setLoadError(fmt.Sprintf("Metadata extraction failed: %v", err))
			return
		}

		logDebug("Metadata extracted successfully")

		// Update processor state
		p.mu.Lock()
		p.currentFile = fileData
		p.metadata = md
		p.audioModel = nil // Will be created when needed
		p.analysisDone = false
		p.analyzedFor = make(map[viz.ViewMode]bool)
		p.status = ProcessingStatus{
			State:    StateIdle,
			Message:  "File loaded successfully",
			Progress: 1.0,
		}
		p.mu.Unlock()

		logDebug("File load complete: %s - %s", md.Artist, md.Title)
	}()

	return nil
}

func (p *Processor) SwitchVisualization(mode viz.ViewMode) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// If analysis is in progress, return status
	if p.status.State == StateAnalyzing {
		return "", fmt.Errorf("analysis in progress: %s", p.status.Message)
	}

	// Check if track data is available
	if len(p.currentFile) == 0 {
		return "", fmt.Errorf("no audio data available")
	}

	// Check if visualization already exists
	if p.vizCache[mode] {
		err := p.vizManager.SetMode(mode)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Switched to %s visualization", getModeName(mode)), nil
	}

	startTime := time.Now()
	logDebug("Starting analysis for visualization mode: %s", getModeName(mode))

	// Start analysis for this visualization mode
	p.status = ProcessingStatus{
		State:     StateAnalyzing,
		Message:   fmt.Sprintf("Preparing %s visualization...", getModeName(mode)),
		Progress:  0,
		CanCancel: true,
		StartTime: time.Now(),
	}

	// Run analysis in background
	go func() {
		err := p.analyzeForMode(mode)
		if err != nil {
			p.setError(fmt.Sprintf("Analysis failed: %v", err))
			return
		}

		// Create visualization
		var v viz.Visualization
		p.mu.Lock()
		defer p.mu.Unlock()

		// Ensure we still have valid data
		if p.audioModel == nil || p.metadata == nil {
			p.setError("Audio data no longer available")
			return
		}

		switch mode {
		case viz.WaveformMode:
			if len(p.audioModel.RawData) > 0 {
				v = viz.NewWaveformViz(p.audioModel.RawData, p.audioModel.SampleRate)
			}

		case viz.SpectrogramMode:
			if p.audioModel.FFTData != nil && len(p.audioModel.FFTData) > 0 {
				v = viz.NewSpectrogramViz(
					p.audioModel.FFTData,
					p.audioModel.FreqBands,
					p.audioModel.SampleRate,
				)
			}

		case viz.TempoMode:
			if len(p.audioModel.BeatData) > 0 {
				v = viz.NewTempoViz(
					p.audioModel.BeatData,
					p.audioModel.RawData,
					p.audioModel.SampleRate,
				)
			}

		case viz.BeatMapMode:
			if len(p.audioModel.BeatOnsets) > 0 {
				v = viz.NewBeatViz(
					p.audioModel.BeatData,
					p.audioModel.BeatOnsets,
					p.audioModel.EstimatedTempo,
					p.audioModel.SampleRate,
				)
			}
		}

		if v == nil {
			p.setError(fmt.Sprintf("Failed to create %s visualization", getModeName(mode)))
			return
		}

		// Set duration and add to manager
		v.SetTotalDuration(p.metadata.Duration)
		p.vizManager.AddVisualization(mode, v)
		p.vizCache[mode] = true

		elapsedTime := time.Since(startTime)
		logDebug("Completed %s visualization analysis in %v", getModeName(mode), elapsedTime)

		err = p.vizManager.SetMode(mode)
		if err != nil {
			p.setError(fmt.Sprintf("Failed to set visualization mode: %v", err))
			return
		}

		p.status = ProcessingStatus{
			State:    StateIdle,
			Message:  fmt.Sprintf("%s visualization ready", getModeName(mode)),
			Progress: 1.0,
		}
	}()

	return "", fmt.Errorf("preparing visualization")
}

func (p *Processor) GetVisualization(width, height int) string {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Show progress if analyzing
	if p.status.State == StateAnalyzing {
		return fmt.Sprintf("%s\nProgress: %d%%\n",
			p.status.Message,
			int(p.status.Progress*100))
	}

	// Show loading progress
	if p.status.State == StateLoading {
		if p.status.TotalBytes > 0 {
			percent := float64(p.status.BytesLoaded) / float64(p.status.TotalBytes) * 100
			return fmt.Sprintf("Loading file... %.1f%%\n", percent)
		}
		return "Loading file...\n"
	}

	return p.vizManager.Render()
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

func (p *Processor) analyzeForMode(mode viz.ViewMode) error {
	p.mu.Lock()
	startTime := time.Now()
	cancelChan := p.analysisCancel
	p.mu.Unlock()

	logDebug("Starting analysis for mode: %s", getModeName(mode))

	// Create model if needed
	if p.audioModel == nil {
		p.audioModel = NewModel(p.metadata.SampleRate)
		logDebug("Created new audio model with sample rate: %d", p.metadata.SampleRate)
	}

	// Run analysis based on mode
	var err error
	switch mode {
	case viz.WaveformMode:
		logDebug("Starting waveform analysis")
		err = p.audioModel.AnalyzeWaveform(
			p.currentFile,
			func(progress float64) {
				p.updateAnalysisProgress(progress, "Analyzing waveform...")
			},
			cancelChan,
		)

	case viz.SpectrogramMode:
		logDebug("Starting spectrogram analysis")
		err = p.audioModel.AnalyzeSpectrum(
			func(progress float64) {
				p.updateAnalysisProgress(progress, "Computing frequency analysis...")
			},
			cancelChan,
		)

	case viz.TempoMode, viz.BeatMapMode:
		logDebug("Starting tempo/beat analysis")
		err = p.audioModel.AnalyzeBeats(
			func(progress float64) {
				p.updateAnalysisProgress(progress, "Detecting beats and tempo...")
			},
			cancelChan,
		)
	}

	if err != nil {
		logDebug("Analysis failed for %s: %v", getModeName(mode), err)
		return fmt.Errorf("analysis failed: %v", err)
	}

	elapsedTime := time.Since(startTime)
	logDebug("Completed analysis for %s in %v", getModeName(mode), elapsedTime)

	return nil
}

func (p *Processor) updateAnalysisProgress(progress float64, message string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	elapsed := time.Since(p.status.StartTime)
	var etaMsg string

	if progress > 0 && progress < 1 {
		totalEstimate := elapsed.Seconds() / progress
		remaining := time.Duration((totalEstimate - elapsed.Seconds()) * float64(time.Second))

		if remaining > 1*time.Hour {
			etaMsg = fmt.Sprintf("%.1f hours", remaining.Hours())
		} else if remaining > 1*time.Minute {
			etaMsg = fmt.Sprintf("%.1f minutes", remaining.Minutes())
		} else {
			etaMsg = fmt.Sprintf("%.0f seconds", remaining.Seconds())
		}
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

func (p *Processor) setAnalysisComplete(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err != nil {
		p.status = ProcessingStatus{
			State:   StateIdle,
			Message: fmt.Sprintf("Analysis failed: %v", err),
		}
	} else {
		p.status = ProcessingStatus{
			State:    StateIdle,
			Message:  "Analysis complete",
			Progress: 1.0,
		}
	}
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

// GetMetadata returns the current track's metadata
func (p *Processor) GetMetadata() *Metadata {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.metadata
}

// GetCurrentFile returns the loaded audio file data
func (p *Processor) GetCurrentFile() []byte {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.currentFile
}

// GetStatus returns the current processing status
func (p *Processor) GetStatus() ProcessingStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.status
}

func (p *Processor) HandleVisualizationInput(key string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Ignore input during processing
	if p.status.State == StateLoading || p.status.State == StateAnalyzing {
		return false
	}

	// Handle resize events specially
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

	// Pass input to visualization manager
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
