package utils

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

var MusicExtensions = map[string]bool{
	".mp3":  true,
	".flac": true,
	".m4a":  true,
	".wav":  true,
	".ogg":  true,
	".opus": true,
	".aac":  true,
	".wma":  true,
}

// Magic numbers for common audio formats
var MagicNumbers = map[string][]byte{
	"mp3":  {0x49, 0x44, 0x33},       // ID3
	"flac": {0x66, 0x4C, 0x61, 0x43}, // fLaC
	"wav":  {0x52, 0x49, 0x46, 0x46}, // RIFF
	"ogg":  {0x4F, 0x67, 0x67, 0x53}, // OggS
	"m4a":  {0x66, 0x74, 0x79, 0x70}, // ftyp
}

func IsMusicFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if !MusicExtensions[ext] {
		return false
	}

	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	header := make([]byte, 8)
	n, err := file.Read(header)
	if err != nil || n < 8 {
		return false
	}

	mimeType := http.DetectContentType(header)
	if strings.HasPrefix(mimeType, "audio/") {
		return true
	}

	for _, magic := range MagicNumbers {
		if len(magic) <= len(header) {
			matches := true
			for i, b := range magic {
				if header[i] != b {
					matches = false
					break
				}
			}
			if matches {
				return true
			}
		}
	}

	return false
}

func GetCompletions(partialPath string) []string {
	dir := filepath.Dir(partialPath)
	if dir == "." {
		dir = "."
	}

	prefix := filepath.Base(partialPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var completions []string
	for _, entry := range entries {
		name := entry.Name()
		fullPath := filepath.Join(dir, name)

		if !strings.HasPrefix(strings.ToLower(name), strings.ToLower(prefix)) {
			continue
		}

		if entry.IsDir() {
			completions = append(completions, fullPath+string(os.PathSeparator))
			continue
		}

		if IsMusicFile(fullPath) {
			completions = append(completions, fullPath)
		}
	}

	return completions
}
