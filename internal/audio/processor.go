package audio

import (
	"fmt"
	"gowav/pkg/viz"
	"io"
	"net/http"
	"os"
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

// Processor handles audio file loading and analysis
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
		analysisCancel: make(chan struct{}),
	}
}

// LoadFile loads an audio file and starts metadata extraction
func (p *Processor) LoadFile(path string) error {
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
		var _ int64

		if isURL(path) {
			fileData, _, err = p.loadFromURL(path, cancelChan)
		} else {
			fileData, _, err = p.loadFromFile(path, cancelChan)
		}

		if err != nil {
			p.setLoadError(fmt.Sprintf("Load failed: %v", err))
			return
		}

		// Extract metadata - this runs in parallel internally
		md, err := ExtractMetadata(fileData)
		if err != nil {
			p.setLoadError(fmt.Sprintf("Metadata extraction failed: %v", err))
			return
		}

		// Update processor state with loaded data
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
	}()

	return nil
}

func (p *Processor) setLoadError(msg string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.status = ProcessingStatus{
		State:   StateIdle,
		Message: msg,
	}
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

func isURL(path string) bool {
	return strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://")
}

// loadFromFile reads a local audio file with progress tracking
func (p *Processor) loadFromFile(path string, cancelChan chan struct{}) ([]byte, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("open error: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, 0, fmt.Errorf("stat error: %w", err)
	}

	data := make([]byte, 0, info.Size())
	buf := make([]byte, 32*1024)
	var totalRead int64
	readStart := time.Now()

	for {
		select {
		case <-cancelChan:
			return nil, 0, fmt.Errorf("cancelled")
		default:
		}

		n, err := file.Read(buf)
		if n > 0 {
			data = append(data, buf[:n]...)
			totalRead += int64(n)

			// Update progress with ETA
			elapsed := time.Since(readStart)
			if elapsed > 0 {
				bytesPerSec := float64(totalRead) / elapsed.Seconds()
				remainingBytes := info.Size() - totalRead
				eta := time.Duration(float64(remainingBytes)/bytesPerSec) * time.Second

				p.updateLoadProgress(totalRead, info.Size(), eta)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, 0, fmt.Errorf("read error: %w", err)
		}
	}

	return data, info.Size(), nil
}

// loadFromURL downloads an audio file with progress tracking
func (p *Processor) loadFromURL(url string, cancelChan chan struct{}) ([]byte, int64, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, 0, fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	data := make([]byte, 0)
	buf := make([]byte, 32*1024)
	var totalRead int64
	readStart := time.Now()

	for {
		select {
		case <-cancelChan:
			return nil, 0, fmt.Errorf("cancelled")
		default:
		}

		n, err := resp.Body.Read(buf)
		if n > 0 {
			data = append(data, buf[:n]...)
			totalRead += int64(n)

			// Update progress with ETA if content length known
			if resp.ContentLength > 0 {
				elapsed := time.Since(readStart)
				if elapsed > 0 {
					bytesPerSec := float64(totalRead) / elapsed.Seconds()
					remainingBytes := resp.ContentLength - totalRead
					eta := time.Duration(float64(remainingBytes)/bytesPerSec) * time.Second

					p.updateLoadProgress(totalRead, resp.ContentLength, eta)
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, 0, fmt.Errorf("download error: %w", err)
		}
	}

	return data, resp.ContentLength, nil
}

func (p *Processor) updateLoadProgress(bytesRead, total int64, eta time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	var progress float64
	if total > 0 {
		progress = float64(bytesRead) / float64(total)
	}

	etaMsg := "calculating..."
	if eta > 0 {
		if eta > 1*time.Hour {
			etaMsg = fmt.Sprintf("%.1f hours", eta.Hours())
		} else if eta > 1*time.Minute {
			etaMsg = fmt.Sprintf("%.1f minutes", eta.Minutes())
		} else {
			etaMsg = fmt.Sprintf("%.0f seconds", eta.Seconds())
		}
	}

	p.status = ProcessingStatus{
		State:       StateLoading,
		Message:     fmt.Sprintf("Loading file... (ETA: %s)", etaMsg),
		Progress:    progress,
		CanCancel:   true,
		StartTime:   p.status.StartTime,
		BytesLoaded: bytesRead,
		TotalBytes:  total,
	}
}

// analyzeForMode ensures analysis is done for a specific visualization mode
func (p *Processor) analyzeForMode(mode viz.ViewMode) error {
	p.mu.Lock()
	if p.analyzedFor[mode] {
		p.mu.Unlock()
		return nil // Already analyzed for this mode
	}

	// Start analysis if not already running
	if p.status.State != StateAnalyzing {
		p.status = ProcessingStatus{
			State:     StateAnalyzing,
			Message:   fmt.Sprintf("Analyzing for %s visualization...", getModeName(mode)),
			Progress:  0,
			CanCancel: true,
			StartTime: time.Now(),
		}
		cancelChan := p.analysisCancel
		p.mu.Unlock()

		// Create model if needed
		if p.audioModel == nil {
			p.audioModel = NewModel(p.metadata.SampleRate)
		}

		// Run appropriate analysis based on mode
		var err error
		switch mode {
		case viz.WaveformMode:
			err = p.audioModel.AnalyzeWaveform(
				p.currentFile,
				func(progress float64) { p.updateAnalysisProgress(progress) },
				cancelChan,
			)

		case viz.SpectrogramMode:
			err = p.audioModel.AnalyzeSpectrum(
				func(progress float64) { p.updateAnalysisProgress(progress) },
				cancelChan,
			)

		case viz.TempoMode, viz.BeatMapMode:
			err = p.audioModel.AnalyzeBeats(
				func(progress float64) { p.updateAnalysisProgress(progress) },
				cancelChan,
			)

		default:
			err = fmt.Errorf("unsupported visualization mode")
		}

		if err != nil {
			p.setAnalysisComplete(err)
			return err
		}

		p.mu.Lock()
		p.analyzedFor[mode] = true
		p.status = ProcessingStatus{
			State:    StateIdle,
			Message:  "Analysis complete",
			Progress: 1.0,
		}
	}
	p.mu.Unlock()
	return nil
}

func (p *Processor) updateAnalysisProgress(frac float64) {
	if frac < 0 {
		frac = 0
	} else if frac > 1 {
		frac = 1
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	elapsed := time.Since(p.status.StartTime)
	var etaMsg string

	if frac > 0 && frac < 1 {
		totalEstimate := elapsed.Seconds() / frac
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
		Message:   fmt.Sprintf("Analyzing... (ETA: %s)", etaMsg),
		Progress:  frac,
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
