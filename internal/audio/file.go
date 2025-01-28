package audio

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// loadFromFile reads bytes from a local file, updating progress in the Processor status.
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

	data := make([]byte, 0, info.Size())
	buf := make([]byte, 32*1024)
	var totalRead int64
	readStart := time.Now()

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

			elapsed := time.Since(readStart)
			if elapsed > 0 {
				bytesPerSec := float64(totalRead) / elapsed.Seconds()
				remainingBytes := info.Size() - totalRead
				eta := time.Duration(float64(remainingBytes)/bytesPerSec) * time.Second

				p.mu.Lock()
				p.status = ProcessingStatus{
					State:       StateLoading,
					Message:     fmt.Sprintf("Loading file... (ETA: %s)", formatETA(eta)),
					Progress:    float64(totalRead) / float64(info.Size()),
					CanCancel:   true,
					StartTime:   readStart,
					BytesLoaded: totalRead,
					TotalBytes:  info.Size(),
				}
				p.mu.Unlock()
			}
		}

		// Brief sleep to let the UI update progress
		time.Sleep(40 * time.Millisecond)

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

// loadFromURL downloads bytes from a given URL, updating progress in the Processor status.
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

	data := make([]byte, 0)
	buf := make([]byte, 32*1024)
	var totalRead int64
	readStart := time.Now()

	contentLength := resp.ContentLength

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

			if contentLength > 0 {
				elapsed := time.Since(readStart)
				if elapsed > 0 {
					bytesPerSec := float64(totalRead) / elapsed.Seconds()
					remainingBytes := contentLength - totalRead
					eta := time.Duration(float64(remainingBytes)/bytesPerSec) * time.Second

					p.mu.Lock()
					p.status = ProcessingStatus{
						State:       StateLoading,
						Message:     fmt.Sprintf("Downloading... (ETA: %s)", formatETA(eta)),
						Progress:    float64(totalRead) / float64(contentLength),
						CanCancel:   true,
						StartTime:   readStart,
						BytesLoaded: totalRead,
						TotalBytes:  contentLength,
					}
					p.mu.Unlock()
				}
			}
		}

		// Let the UI update in between chunk reads
		time.Sleep(40 * time.Millisecond)

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

// formatETA is a helper that turns a duration into a human-friendly ETA string (e.g. "10 seconds").
func formatETA(d time.Duration) string {
	if d > 1*time.Hour {
		return fmt.Sprintf("%.1f hours", d.Hours())
	} else if d > 1*time.Minute {
		return fmt.Sprintf("%.1f minutes", d.Minutes())
	}
	sec := d.Seconds()
	if sec < 0 {
		sec = 0
	}
	return fmt.Sprintf("%.0f seconds", sec)
}
