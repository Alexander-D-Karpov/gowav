package types

import (
	"fmt"
	"sync"
	"time"
)

type LoadingState struct {
	IsLoading   bool
	Message     string
	Progress    float64
	StartTime   time.Time
	FileSize    int64
	BytesLoaded int64
	CanCancel   bool
	mu          sync.RWMutex
}

func (s *LoadingState) UpdateProgress(loaded int64, total int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Only update if we have valid values
	if loaded >= 0 && total > 0 {
		s.BytesLoaded = loaded
		s.FileSize = total
		s.Progress = float64(loaded) / float64(total)
	}
}

func (s *LoadingState) GetETA() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return empty string if we don't have enough data
	if s.BytesLoaded == 0 || s.FileSize == 0 {
		return ""
	}

	elapsed := time.Since(s.StartTime)
	if elapsed <= 0 {
		return ""
	}

	rate := float64(s.BytesLoaded) / elapsed.Seconds()
	if rate <= 0 {
		return ""
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
