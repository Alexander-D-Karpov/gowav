package audio

import (
	"bytes"
	"fmt"
	"image"
	"io"
	"strings"
	"time"

	"github.com/dhowden/tag"
	"github.com/hajimehoshi/go-mp3"
)

type Metadata struct {
	Title       string
	Artist      string
	Album       string
	Year        int
	Genre       string
	Track       string
	TrackTotal  string
	Disc        string
	DiscTotal   string
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
	BPM         string
	ReleaseDate string

	Duration        time.Duration
	BitRate         int
	SampleRate      int
	Channels        int
	FileSize        int64
	Format          string
	Encoder         string
	EncoderSettings string

	HasArtwork  bool
	ArtworkMIME string
	ArtworkSize image.Point
	Artwork     image.Image
}

func ExtractMetadata(data []byte) (*Metadata, error) {
	// First pass: read tags.
	reader := bytes.NewReader(data)
	meta, err := tag.ReadFrom(reader)
	if err != nil {
		return nil, fmt.Errorf("failed reading tags: %w", err)
	}

	// Collect basic tag info.
	trackNum, trackTotal := meta.Track()
	discNum, discTotal := meta.Disc()
	m := &Metadata{
		Title:       meta.Title(),
		Artist:      meta.Artist(),
		Album:       meta.Album(),
		AlbumArtist: meta.AlbumArtist(),
		Year:        meta.Year(),
		Genre:       meta.Genre(),
		Track:       fmt.Sprintf("%d", trackNum),
		TrackTotal:  fmt.Sprintf("%d", trackTotal),
		Disc:        fmt.Sprintf("%d", discNum),
		DiscTotal:   fmt.Sprintf("%d", discTotal),
		FileSize:    int64(len(data)),
		Channels:    2, // By default. We'll confirm when decoding.
		SampleRate:  44100,
	}

	// Handle attached artwork if available.
	if pic := meta.Picture(); pic != nil && len(pic.Data) > 0 {
		img, _, imgErr := image.Decode(bytes.NewReader(pic.Data))
		if imgErr == nil {
			m.Artwork = img
			m.HasArtwork = true
			m.ArtworkSize = img.Bounds().Size()
			m.ArtworkMIME = pic.MIMEType
		}
	} else if raw := meta.Raw(); raw != nil {
		if apicData, ok := raw["APIC"]; ok {
			switch pic := apicData.(type) {
			case *tag.Picture:
				if pic != nil && len(pic.Data) > 0 {
					img, _, err := image.Decode(bytes.NewReader(pic.Data))
					if err == nil {
						m.Artwork = img
						m.HasArtwork = true
						m.ArtworkSize = img.Bounds().Size()
						m.ArtworkMIME = pic.MIMEType
					}
				}
			case []byte:
				if len(pic) > 0 {
					img, _, err := image.Decode(bytes.NewReader(pic))
					if err == nil {
						m.Artwork = img
						m.HasArtwork = true
						m.ArtworkSize = img.Bounds().Size()
						m.ArtworkMIME = "image/jpeg"
					}
				}
			}
		}
	}

	// Second pass: decode the MP3 fully to get correct duration and sample rate.
	// (If it's not an MP3, we fall back to minimal defaults.)
	m.detectAudioProperties(data)

	// Compute bitrate from the final duration.
	if m.Duration > 0 {
		m.BitRate = int(float64(m.FileSize*8) / m.Duration.Seconds() / 1000.0)
	} else {
		m.BitRate = 0
	}

	// Fill in defaults if some fields missing.
	m.ensureDefaults()
	return m, nil
}

// decodeMP3Fully reads the MP3 data from start to finish, counting frames to get an accurate duration.
func (m *Metadata) detectAudioProperties(raw []byte) {
	r := bytes.NewReader(raw)
	dec, err := mp3.NewDecoder(r)
	if err != nil {
		// If not MP3 or any error: leave defaults (e.g. WAV, FLAC can be handled in a future expansion).
		return
	}
	m.SampleRate = dec.SampleRate()
	m.Channels = 2

	// Count total samples by reading all PCM data. Each frame is 4 bytes for 16-bit stereo (2 channels).
	var totalPCMFrames int64
	buf := make([]byte, 8192)
	for {
		n, readErr := dec.Read(buf)
		if n > 0 {
			// Each sample is 4 bytes for stereo 16-bit. So sampleCount = n / 4.
			totalPCMFrames += int64(n / 4)
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return
		}
	}
	sec := float64(totalPCMFrames) / float64(m.SampleRate)
	m.Duration = time.Duration(sec * float64(time.Second))
}

// Provide fallbacks if missing.
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
