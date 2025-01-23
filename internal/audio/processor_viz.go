package audio

import (
	"fmt"
	"gowav/pkg/viz"
	"time"
)

func (p *Processor) SwitchVisualization(mode viz.ViewMode) (string, error) {
	p.mu.Lock()

	// If analysis is already in progress, return status
	if p.status.State == StateAnalyzing {
		p.mu.Unlock()
		return "", fmt.Errorf("analysis in progress: %s", p.status.Message)
	}

	// Check if track data is available
	if p.audioModel == nil || len(p.audioModel.RawData) == 0 {
		p.mu.Unlock()
		return "", fmt.Errorf("no audio data available")
	}

	// Check if visualization already exists
	if p.vizCache[mode] {
		p.vizManager.SetMode(mode)
		p.mu.Unlock()
		return fmt.Sprintf("Switched to %s visualization", getModeName(mode)), nil
	}

	// Start analysis for this visualization mode
	p.status = ProcessingStatus{
		State:     StateAnalyzing,
		Message:   fmt.Sprintf("Preparing %s visualization...", getModeName(mode)),
		Progress:  0,
		CanCancel: true,
		StartTime: time.Now(),
	}

	p.mu.Unlock()

	// Run analysis in background
	go func() {
		err := p.analyzeForMode(mode)
		if err != nil {
			p.mu.Lock()
			p.status = ProcessingStatus{
				State:   StateIdle,
				Message: fmt.Sprintf("Analysis failed: %v", err),
			}
			p.mu.Unlock()
			return
		}

		// Create visualization
		var v viz.Visualization
		p.mu.Lock()
		defer p.mu.Unlock()

		// Ensure we still have valid data
		if p.audioModel == nil || p.metadata == nil {
			p.status = ProcessingStatus{
				State:   StateIdle,
				Message: "Audio data no longer available",
			}
			return
		}

		switch mode {
		case viz.WaveformMode:
			if len(p.audioModel.RawData) > 0 {
				v = viz.NewWaveformViz(p.audioModel.RawData, p.audioModel.SampleRate)
			}

		case viz.SpectrogramMode:
			if p.audioModel.FFTData != nil && len(p.audioModel.FFTData) > 0 {
				v = viz.NewSpectrogramViz(p.audioModel.FFTData, p.audioModel.FreqBands, p.audioModel.SampleRate)
			}

		case viz.DensityMode:
			if len(p.audioModel.RawData) > 0 {
				v = viz.NewDensityViz(p.audioModel.RawData, p.audioModel.SampleRate)
			}

		case viz.TempoMode:
			if len(p.audioModel.BeatData) > 0 {
				v = viz.NewTempoViz(p.audioModel.BeatData, p.audioModel.RawData, p.audioModel.SampleRate)
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
			p.status = ProcessingStatus{
				State:   StateIdle,
				Message: fmt.Sprintf("Failed to create %s visualization", getModeName(mode)),
			}
			return
		}

		// Set duration and add to manager
		v.SetTotalDuration(p.metadata.Duration)
		p.vizManager.AddVisualization(mode, v)
		p.vizCache[mode] = true
		p.vizManager.SetMode(mode)

		// Update status
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

	// Return visualization
	return p.vizManager.Render()
}

func (p *Processor) HandleVisualizationInput(key string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Ignore input during processing
	if p.status.State == StateLoading || p.status.State == StateAnalyzing {
		return false
	}

	return p.vizManager.HandleInput(key)
}

func getModeName(mode viz.ViewMode) string {
	switch mode {
	case viz.WaveformMode:
		return "waveform"
	case viz.SpectrogramMode:
		return "spectrogram"
	case viz.DensityMode:
		return "density"
	case viz.TempoMode:
		return "tempo"
	case viz.BeatMapMode:
		return "beatmap"
	default:
		return "unknown"
	}
}
