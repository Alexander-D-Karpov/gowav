package audio

import (
	"bytes"
	"fmt"
	"github.com/dhowden/tag"
	"github.com/hajimehoshi/go-mp3"
	"image"
	"io"
	"strings"
	"time"
)

type Metadata struct {
	// Basic tags
	Title      string
	Artist     string
	Album      string
	Year       int
	Genre      string
	Track      string
	TrackTotal string
	Disc       string
	DiscTotal  string

	// Extended tags
	AlbumArtist string
	Composer    string
	Conductor   string
	Copyright   string
	EncodedBy   string
	Publisher   string
	ISRC        string
	Language    string
	Comment     string
	Lyrics      string

	// Additional fields
	BPM         string
	ReleaseDate string

	// Technical details
	Duration        time.Duration
	BitRate         int
	SampleRate      int
	Channels        int
	FileSize        int64
	Format          string
	Encoder         string
	EncoderSettings string

	// Artwork
	HasArtwork  bool
	ArtworkMIME string
	ArtworkSize image.Point
	Artwork     image.Image
}

func ExtractMetadata(data []byte) (*Metadata, error) {
	reader := bytes.NewReader(data)

	// First get tag info
	m, err := tag.ReadFrom(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	// Reset reader position for MP3 decoder
	reader.Seek(0, io.SeekStart)
	decoder, err := mp3.NewDecoder(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to create MP3 decoder: %w", err)
	}

	// Get duration in samples and convert to time
	samples := decoder.Length()
	sampleRate := decoder.SampleRate()
	duration := time.Duration(float64(samples) / float64(sampleRate) * float64(time.Second))
	bitRate := int(float64(len(data)) * 8 / duration.Seconds() / 1000)

	trackNum, trackTotal := m.Track()
	discNum, discTotal := m.Disc()

	metadata := &Metadata{
		Title:       m.Title(),
		Artist:      m.Artist(),
		Album:       m.Album(),
		AlbumArtist: m.AlbumArtist(),
		Year:        m.Year(),
		Genre:       m.Genre(),
		Track:       fmt.Sprintf("%d", trackNum),
		TrackTotal:  fmt.Sprintf("%d", trackTotal),
		Disc:        fmt.Sprintf("%d", discNum),
		DiscTotal:   fmt.Sprintf("%d", discTotal),
		Duration:    duration,
		BitRate:     bitRate,
		SampleRate:  sampleRate,
		Channels:    2,
		FileSize:    int64(len(data)),
	}

	// Get artwork - check both direct Picture() and raw APIC tag
	if pic := m.Picture(); pic != nil && len(pic.Data) > 0 {
		if img, _, err := image.Decode(bytes.NewReader(pic.Data)); err == nil {
			bounds := img.Bounds()
			metadata.Artwork = img
			metadata.HasArtwork = true
			metadata.ArtworkSize = bounds.Size()
			metadata.ArtworkMIME = pic.MIMEType
		}
	} else if raw := m.Raw(); raw != nil {
		if apicData, ok := raw["APIC"]; ok {
			switch pic := apicData.(type) {
			case *tag.Picture:
				if pic != nil && len(pic.Data) > 0 {
					if img, _, err := image.Decode(bytes.NewReader(pic.Data)); err == nil {
						bounds := img.Bounds()
						metadata.Artwork = img
						metadata.HasArtwork = true
						metadata.ArtworkSize = bounds.Size()
						metadata.ArtworkMIME = pic.MIMEType
					}
				}
			case []byte:
				if len(pic) > 0 {
					if img, _, err := image.Decode(bytes.NewReader(pic)); err == nil {
						bounds := img.Bounds()
						metadata.Artwork = img
						metadata.HasArtwork = true
						metadata.ArtworkSize = bounds.Size()
						metadata.ArtworkMIME = "image/jpeg" // Assume JPEG as fallback
					}
				}
			}
		}
	}
	return metadata, nil
}

func (m *Metadata) extractAdditionalMetadata(raw map[string]interface{}) {
	// Lyrics
	if lyrics := getStringTag(raw, "USLT"); lyrics != "" {
		m.Lyrics = lyrics
	}

	// Comments
	if comment := getStringTag(raw, "COMM"); comment != "" {
		m.Comment = comment
	}

	// Other ID3v2 tags
	if bpm := getStringTag(raw, "TBPM"); bpm != "" {
		m.BPM = bpm
	}
	if date := getStringTag(raw, "TDOR"); date != "" {
		m.ReleaseDate = date
	}
	if copyright := getStringTag(raw, "TCOP"); copyright != "" {
		m.Copyright = copyright
	}
	if encoded := getStringTag(raw, "TENC"); encoded != "" {
		m.EncodedBy = encoded
	}
	if isrc := getStringTag(raw, "TSRC"); isrc != "" {
		m.ISRC = isrc
	}
}

func (m *Metadata) ensureDefaults() {
	if m.Title == "" {
		m.Title = "Unknown Title"
	}
	if m.Artist == "" {
		m.Artist = "Unknown Artist"
	}
	if m.Album == "" {
		m.Album = "Unknown Album"
	}
}

func getStringTag(raw map[string]interface{}, key string) string {
	if val, ok := raw[key]; ok {
		switch v := val.(type) {
		case string:
			return v
		case []string:
			if len(v) > 0 {
				return v[0]
			}
		case []interface{}:
			if len(v) > 0 {
				if s, ok := v[0].(string); ok {
					return s
				}
			}
		}
	}
	return ""
}

// String formatting for metadata display
func (m *Metadata) String() string {
	var b strings.Builder

	// Make sure all rows have consistent width
	const totalWidth = 80

	b.WriteString("┌" + strings.Repeat("─", totalWidth-2) + "┐\n")

	// Title line
	title := "TRACK INFORMATION"
	padding := (totalWidth - len(title) - 2) / 2
	b.WriteString("│" + strings.Repeat(" ", padding) + title + strings.Repeat(" ", totalWidth-padding-len(title)-2) + "│\n")

	b.WriteString("├" + strings.Repeat("─", totalWidth-2) + "┤\n")

	// Calculate fixed widths for columns
	const labelWidth = 20
	const valueWidth = totalWidth - labelWidth - 5 // 5 for "│ " and " │" borders

	// Basic info
	writeRow(&b, "Title", m.Title, labelWidth, valueWidth)
	writeRow(&b, "Artist", m.Artist, labelWidth, valueWidth)
	writeRow(&b, "Album", m.Album, labelWidth, valueWidth)
	if m.Track != "" {
		track := m.Track
		if m.TrackTotal != "" {
			track += "/" + m.TrackTotal
		}
		writeRow(&b, "Track", track, labelWidth, valueWidth)
	}
	if m.Disc != "" {
		disc := m.Disc
		if m.DiscTotal != "" {
			disc += "/" + m.DiscTotal
		}
		writeRow(&b, "Disc", disc, labelWidth, valueWidth)
	}
	writeRow(&b, "Year", fmt.Sprintf("%d", m.Year), labelWidth, valueWidth)
	writeRow(&b, "Genre", m.Genre, labelWidth, valueWidth)

	// Tech details section
	b.WriteString("├" + strings.Repeat("─", totalWidth-2) + "┤\n")
	b.WriteString("│" + strings.Repeat(" ", padding) + "TECH DETAILS" + strings.Repeat(" ", totalWidth-padding-len("TECH DETAILS")-2) + "│\n")
	b.WriteString("├" + strings.Repeat("─", totalWidth-2) + "┤\n")

	writeRow(&b, "Duration", formatDuration(m.Duration), labelWidth, valueWidth)
	writeRow(&b, "Bit Rate", fmt.Sprintf("%d kb/s", m.BitRate), labelWidth, valueWidth)
	writeRow(&b, "Sample Rate", fmt.Sprintf("%d Hz", m.SampleRate), labelWidth, valueWidth)
	writeRow(&b, "Channels", fmt.Sprintf("%d", m.Channels), labelWidth, valueWidth)
	writeRow(&b, "File Size", formatFileSize(m.FileSize), labelWidth, valueWidth)

	b.WriteString("└" + strings.Repeat("─", totalWidth-2) + "┘\n")
	return b.String()
}

func writeRow(b *strings.Builder, label, value string, labelWidth, valueWidth int) {
	if value == "" {
		return
	}
	// Pad label
	labelStr := fmt.Sprintf("%-*s", labelWidth, label)

	// Truncate value if needed
	if len(value) > valueWidth {
		value = value[:valueWidth-3] + "..."
	}
	// Pad value
	valueStr := fmt.Sprintf("%-*s", valueWidth, value)

	b.WriteString(fmt.Sprintf("│ %s│ %s │\n", labelStr, valueStr))
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

func formatFileSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}
