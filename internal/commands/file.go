package commands

import (
	"fmt"
)

func (c *Commander) handleLoad(path string) (string, error) {
	c.loadProgress = 0

	// Update progress as we go
	c.SetLoadingProgress(0.2) // Started loading

	if err := c.processor.LoadFile(path); err != nil {
		return "", fmt.Errorf("failed to load file: %w", err)
	}

	c.SetLoadingProgress(0.6) // File loaded

	metadata := c.processor.GetMetadata()
	c.currentTrack = &Track{
		Title:    metadata.Title,
		Artist:   metadata.Artist,
		Album:    metadata.Album,
		Duration: metadata.Duration,
	}

	c.SetLoadingProgress(1.0) // Complete
	c.mode = ModeTrack

	return fmt.Sprintf("Loaded file: %s\n%s", path, metadata), nil
}
