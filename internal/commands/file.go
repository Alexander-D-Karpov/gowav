package commands

import (
	"fmt"
	"sync"
	"time"
)

type LoadingState struct {
	Progress    float64
	Message     string
	StartTime   time.Time
	FileSize    int64
	BytesLoaded int64
	IsLoading   bool
	CanCancel   bool
	mu          sync.RWMutex
}

func (s *LoadingState) UpdateProgress(loaded int64, total int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.BytesLoaded = loaded
	s.FileSize = total
	if total > 0 {
		s.Progress = float64(loaded) / float64(total)
	}
}

func (s *LoadingState) GetETA() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.Progress <= 0 || s.Progress >= 1 {
		return "calculating..."
	}

	elapsed := time.Since(s.StartTime)
	if elapsed <= 0 {
		return "calculating..."
	}

	rate := float64(s.BytesLoaded) / elapsed.Seconds()
	if rate <= 0 {
		return "calculating..."
	}

	remaining := float64(s.FileSize-s.BytesLoaded) / rate
	eta := time.Duration(remaining) * time.Second

	if eta > 1*time.Hour {
		return fmt.Sprintf("%.1f hours", eta.Hours())
	} else if eta > 1*time.Minute {
		return fmt.Sprintf("%.1f minutes", eta.Minutes())
	}
	return fmt.Sprintf("%.0f seconds", eta.Seconds())
}

// handleLoad starts the asynchronous load of a local file or URL.
func (c *Commander) handleLoad(path string) (string, error) {
	err := c.processor.LoadFile(path)
	if err != nil {
		return "", err
	}
	// We only confirm that loading started. The UI will show the spinner/progress/ETA while loading.
	return fmt.Sprintf("Started loading file: %s\nPress Ctrl+C to cancel...", path), nil
}
