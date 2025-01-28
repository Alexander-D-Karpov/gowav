package audio

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"time"
)

func (p *Processor) loadFromFile(path string, cancelChan chan struct{}) ([]byte, error) {
	startTime := time.Now()
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open error: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat error: %w", err)
	}

	// If file is empty, just read it all at once.
	if info.Size() == 0 {
		data, err := io.ReadAll(file)
		if err != nil {
			return nil, fmt.Errorf("read error (empty file): %w", err)
		}
		return data, nil
	}

	data := make([]byte, 0, info.Size())
	buf := make([]byte, 64*1024)
	var totalRead int64
	readStart := time.Now()
	var lastUpdate time.Time

	// We'll only show a real ETA after at least 512 KB read.
	const minBytesForETA = 512 * 1024

	for {
		select {
		case <-cancelChan:
			return nil, fmt.Errorf("cancelled")
		default:
		}

		n, err := file.Read(buf)
		if n > 0 {
			data = append(data, buf[:n]...)
			totalRead += int64(n)
		}

		now := time.Now()
		// Update UI every 100ms or upon EOF
		if now.Sub(lastUpdate) > 100*time.Millisecond || (err == io.EOF && n > 0) {
			elapsed := now.Sub(readStart)

			var progress float64
			if info.Size() > 0 {
				progress = float64(totalRead) / float64(info.Size())
				if progress > 1 {
					progress = 1
				}
			}

			var etaStr = "calculating..."
			if elapsed > 0 && totalRead > minBytesForETA {
				bytesPerSec := float64(totalRead) / elapsed.Seconds()
				remaining := float64(info.Size()-totalRead) / bytesPerSec
				if remaining < 0 {
					remaining = 0
				}
				etaStr = formatETA(time.Duration(remaining) * time.Second)
			}

			p.mu.Lock()
			p.status = ProcessingStatus{
				State:       StateLoading,
				Message:     fmt.Sprintf("Loading file... (ETA: %s)", etaStr),
				Progress:    progress,
				CanCancel:   true,
				StartTime:   readStart,
				BytesLoaded: totalRead,
				TotalBytes:  info.Size(),
			}
			p.mu.Unlock()

			lastUpdate = now
			// Allow the TUI to process this status update
			runtime.Gosched()
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read error: %w", err)
		}
	}

	totalLoadTime := time.Since(startTime)
	logDebug("File %s loaded in %v (size=%d bytes)", path, totalLoadTime, totalRead)
	return data, nil
}

func (p *Processor) loadFromURL(url string, cancelChan chan struct{}) ([]byte, error) {
	startTime := time.Now()
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %d", resp.StatusCode)
	}

	contentLength := resp.ContentLength
	data := make([]byte, 0, 32*1024)
	buf := make([]byte, 64*1024)
	var totalRead int64
	readStart := time.Now()
	var lastUpdate time.Time

	const minBytesForETA = 512 * 1024

	for {
		select {
		case <-cancelChan:
			return nil, fmt.Errorf("cancelled")
		default:
		}

		n, err := resp.Body.Read(buf)
		if n > 0 {
			data = append(data, buf[:n]...)
			totalRead += int64(n)
		}

		now := time.Now()
		if now.Sub(lastUpdate) > 100*time.Millisecond || (err == io.EOF && n > 0) {
			var progress float64
			var etaStr = "calculating..."
			elapsed := now.Sub(readStart)

			if contentLength > 0 {
				progress = float64(totalRead) / float64(contentLength)
				if progress > 1 {
					progress = 1
				}
			}

			if elapsed > 0 && totalRead > minBytesForETA {
				bytesPerSec := float64(totalRead) / elapsed.Seconds()
				remaining := float64(contentLength-totalRead) / bytesPerSec
				if remaining < 0 {
					remaining = 0
				}
				etaStr = formatETA(time.Duration(remaining) * time.Second)
			}

			p.mu.Lock()
			p.status = ProcessingStatus{
				State:       StateLoading,
				Message:     fmt.Sprintf("Downloading... (ETA: %s)", etaStr),
				Progress:    progress,
				CanCancel:   true,
				StartTime:   readStart,
				BytesLoaded: totalRead,
				TotalBytes:  contentLength,
			}
			p.mu.Unlock()

			lastUpdate = now
			runtime.Gosched()
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("download error: %w", err)
		}
	}

	totalLoadTime := time.Since(startTime)
	logDebug("URL %s downloaded in %v (size=%d bytes)", url, totalLoadTime, totalRead)
	return data, nil
}

func formatETA(d time.Duration) string {
	if d > 1*time.Hour {
		return fmt.Sprintf("%.1f hours", d.Hours())
	} else if d > 1*time.Minute {
		return fmt.Sprintf("%.1f minutes", d.Minutes())
	}
	if d < 0 {
		d = 0
	}
	sec := d.Seconds()
	return fmt.Sprintf("%.0f seconds", sec)
}
