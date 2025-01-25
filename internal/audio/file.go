package audio

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

func getStringTag(tags map[string]interface{}, key string) string {
	if val, ok := tags[key]; ok {
		switch v := val.(type) {
		case string:
			return v
		case []string:
			if len(v) > 0 {
				return v[0]
			}
		case []interface{}:
			if len(v) > 0 {
				if str, ok := v[0].(string); ok {
					return str
				}
			}
		}
	}
	return ""
}

func (p *Processor) loadFromFile(path string, cancelChan chan struct{}) ([]byte, error) {
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

			// Update progress with ETA
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

		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read error: %w", err)
		}
	}

	return data, nil
}

func (p *Processor) loadFromURL(url string, cancelChan chan struct{}) ([]byte, error) {
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

			// Update progress if content length is known
			if resp.ContentLength > 0 {
				elapsed := time.Since(readStart)
				if elapsed > 0 {
					bytesPerSec := float64(totalRead) / elapsed.Seconds()
					remainingBytes := resp.ContentLength - totalRead
					eta := time.Duration(float64(remainingBytes)/bytesPerSec) * time.Second

					p.mu.Lock()
					p.status = ProcessingStatus{
						State:       StateLoading,
						Message:     fmt.Sprintf("Downloading... (ETA: %s)", formatETA(eta)),
						Progress:    float64(totalRead) / float64(resp.ContentLength),
						CanCancel:   true,
						StartTime:   readStart,
						BytesLoaded: totalRead,
						TotalBytes:  resp.ContentLength,
					}
					p.mu.Unlock()
				}
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("download error: %w", err)
		}
	}

	return data, nil
}

func formatETA(d time.Duration) string {
	if d > 1*time.Hour {
		return fmt.Sprintf("%.1f hours", d.Hours())
	} else if d > 1*time.Minute {
		return fmt.Sprintf("%.1f minutes", d.Minutes())
	}
	return fmt.Sprintf("%.0f seconds", d.Seconds())
}
