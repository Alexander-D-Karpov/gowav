package audio

import (
	"bytes"
	"fmt"
	"github.com/hajimehoshi/go-mp3"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dhowden/tag"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
)

type Metadata struct {
	Title       string
	Artist      string
	Album       string
	Year        int
	Genre       string
	Track       string
	Disc        string
	AlbumArtist string

	// Additional metadata fields
	Encoder     string
	Comment     string
	Copyright   string
	TSRC        string
	EncodedBy   string
	ReleaseDate string

	// Audio properties
	Duration   time.Duration
	BitRate    int
	SampleRate int
	Channels   int
	FileSize   int64

	// Artwork
	HasArtwork  bool
	ArtworkMIME string
	ArtworkSize image.Point
	Artwork     image.Image
	BPM         string
	Lyrics      string

	// Debug info
	RawTags map[string]interface{}
}

func ExtractMetadata(data []byte) (*Metadata, error) {
	reader := bytes.NewReader(data)

	// First try standard reading
	m, err := tag.ReadFrom(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	// Create metadata struct with basic info...
	metadata := &Metadata{
		Title:       tryDecode(m.Title()),
		Artist:      tryDecode(m.Artist()),
		Album:       tryDecode(m.Album()),
		Year:        m.Year(),
		Genre:       tryDecode(m.Genre()),
		FileSize:    int64(len(data)),
		AlbumArtist: tryDecode(m.AlbumArtist()),
	}

	// Get audio properties
	reader.Seek(0, io.SeekStart)
	decoder, err := mp3.NewDecoder(reader)
	if err == nil {
		var totalSamples int64
		buf := make([]byte, 8192)
		for {
			n, err := decoder.Read(buf)
			if n > 0 {
				totalSamples += int64(n / 4) // 4 bytes per stereo frame
			}
			if err == io.EOF {
				break
			}
			if err != nil {
				break
			}
		}

		sampleRate := decoder.SampleRate()
		if totalSamples > 0 && sampleRate > 0 {
			metadata.Duration = time.Duration(float64(totalSamples) / float64(sampleRate) * float64(time.Second))
			metadata.SampleRate = sampleRate
			metadata.Channels = 2
			metadata.BitRate = int(float64(len(data)*8) / metadata.Duration.Seconds() / 1000)
		}
	}

	// Get audio properties...
	if rawTags := m.Raw(); rawTags != nil {
		metadata.RawTags = rawTags

		// Handle basic tags...
		metadata.Track = getStringTag(rawTags, "TRCK")
		metadata.Disc = getStringTag(rawTags, "TPOS")
		metadata.EncodedBy = getStringTag(rawTags, "TENC")
		metadata.Comment = getStringTag(rawTags, "COMM")
		metadata.Copyright = getStringTag(rawTags, "TCOP")
		metadata.TSRC = getStringTag(rawTags, "TSRC")
		metadata.Encoder = getStringTag(rawTags, "TSSE")

		// ARTWORK EXTRACTION WITH DETAILED LOGGING
		logDebug("Starting artwork extraction...")
		if apicData, ok := rawTags["APIC"]; ok {
			logDebug("Found APIC tag, type: %T", apicData)

			switch pic := apicData.(type) {
			case tag.Picture:
				logDebug("Processing tag.Picture: MIMEType=%s, Type=%d, Description=%s, DataLen=%d",
					pic.MIMEType, pic.Type, pic.Description, len(pic.Data))
				if len(pic.Data) > 0 {
					if err := extractAndSetArtwork(metadata, pic.Data, pic.MIMEType); err != nil {
						logDebug("Failed to extract artwork from tag.Picture: %v", err)
					}
				}

			case *tag.Picture:
				if pic != nil {
					logDebug("Processing *tag.Picture: MIMEType=%s, Type=%d, Description=%s, DataLen=%d",
						pic.MIMEType, pic.Type, pic.Description, len(pic.Data))
					if len(pic.Data) > 0 {
						if err := extractAndSetArtwork(metadata, pic.Data, pic.MIMEType); err != nil {
							logDebug("Failed to extract artwork from *tag.Picture: %v", err)
						}
					}
				} else {
					logDebug("*tag.Picture is nil")
				}

			case []byte:
				logDebug("Processing raw []byte APIC data, length: %d", len(pic))
				if len(pic) > 0 {
					if err := extractAndSetArtwork(metadata, pic, ""); err != nil {
						logDebug("Failed to extract artwork from []byte: %v", err)
					}
				}

			case map[string]interface{}:
				logDebug("Processing map[string]interface{}: %v", pic)
				if picData, ok := pic["Data"].([]byte); ok && len(picData) > 0 {
					if err := extractAndSetArtwork(metadata, picData, ""); err != nil {
						logDebug("Failed to extract artwork from map data: %v", err)
					}
				}

			default:
				logDebug("Unknown APIC type: %T, trying to convert to string for inspection: %v",
					apicData, fmt.Sprintf("%v", apicData))

				// Last resort: try to extract as raw bytes
				if rawBytes, ok := getRawBytes(apicData); ok {
					logDebug("Attempting extraction from raw bytes, length: %d", len(rawBytes))
					if err := extractAndSetArtwork(metadata, rawBytes, ""); err != nil {
						logDebug("Failed to extract artwork from raw bytes: %v", err)
					}
				}
			}

			if !metadata.HasArtwork {
				logDebug("Failed to extract artwork after all attempts")
			}
		} else {
			logDebug("No APIC tag found in metadata")
		}
	}

	return metadata, nil
}

// Helper function to extract artwork
func extractAndSetArtwork(metadata *Metadata, data []byte, mimeType string) error {
	// Debug first few bytes to help identify format
	logDebug("Image data starts with bytes: % x", data[:min(16, len(data))])

	// Check for common image headers
	if len(data) < 12 {
		return fmt.Errorf("data too short for image")
	}

	// Try to identify format and strip any ID3v2/metadata prefix
	var imgData []byte
	switch {
	case bytes.HasPrefix(data, []byte{0xff, 0xd8, 0xff}):
		logDebug("Detected JPEG header")
		imgData = data
	case bytes.HasPrefix(data, []byte{0x89, 0x50, 0x4e, 0x47}):
		logDebug("Detected PNG header")
		imgData = data
	default:
		// Search for JPEG/PNG markers in case there's a prefix
		jpegIndex := bytes.Index(data, []byte{0xff, 0xd8, 0xff})
		pngIndex := bytes.Index(data, []byte{0x89, 0x50, 0x4e, 0x47})

		if jpegIndex >= 0 {
			logDebug("Found JPEG marker at offset %d", jpegIndex)
			imgData = data[jpegIndex:]
		} else if pngIndex >= 0 {
			logDebug("Found PNG marker at offset %d", pngIndex)
			imgData = data[pngIndex:]
		} else {
			// Try the whole data as-is
			logDebug("No standard image markers found, trying raw data")
			imgData = data
		}
	}

	// Try decoding with explicit formats first
	var img image.Image
	var format string
	var err error

	// Try JPEG decoder first
	if img, err = jpeg.Decode(bytes.NewReader(imgData)); err == nil {
		format = "jpeg"
		logDebug("Successfully decoded as JPEG")
	} else {
		logDebug("JPEG decode failed: %v", err)
		// Reset reader and try PNG
		if img, err = png.Decode(bytes.NewReader(imgData)); err == nil {
			format = "png"
			logDebug("Successfully decoded as PNG")
		} else {
			logDebug("PNG decode failed: %v", err)
			// Last resort: try generic decoder
			if img, format, err = image.Decode(bytes.NewReader(imgData)); err != nil {
				return fmt.Errorf("failed to decode image (all attempts): %w", err)
			}
		}
	}

	metadata.Artwork = img
	metadata.HasArtwork = true
	if mimeType != "" {
		metadata.ArtworkMIME = mimeType
	} else {
		metadata.ArtworkMIME = "image/" + format
	}
	bounds := img.Bounds()
	metadata.ArtworkSize = bounds.Size()
	logDebug("Successfully extracted artwork: format=%s size=%dx%d",
		format, bounds.Dx(), bounds.Dy())
	return nil
}

// Helper to try extracting raw bytes from unknown types
func getRawBytes(data interface{}) ([]byte, bool) {
	if str, ok := data.(string); ok {
		return []byte(str), true
	}
	if byt, ok := data.([]byte); ok {
		return byt, true
	}
	// Add more type conversions if needed
	return nil, false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (m *Metadata) String() string {
	var b strings.Builder

	const totalWidth = 80
	const headerWidth = totalWidth - 2 // Accounting for borders

	// Write top border
	b.WriteString("┌" + strings.Repeat("─", headerWidth) + "┐\n")

	// Title line with padding
	title := "TRACK INFORMATION"
	padding := (headerWidth - len(title)) / 2
	b.WriteString("│" + strings.Repeat(" ", padding) + title + strings.Repeat(" ", headerWidth-padding-len(title)) + "│\n")

	// Separator
	b.WriteString("├" + strings.Repeat("─", headerWidth) + "┤\n")

	// Write info sections with consistent formatting
	writeInfoSection(&b, "Title", m.Title, headerWidth)
	writeInfoSection(&b, "Artist", m.Artist, headerWidth)
	writeInfoSection(&b, "Album", m.Album, headerWidth)
	if m.Track != "" {
		writeInfoSection(&b, "Track", m.Track, headerWidth)
	}
	if m.Disc != "" {
		writeInfoSection(&b, "Disc", m.Disc, headerWidth)
	}
	writeInfoSection(&b, "Year", fmt.Sprintf("%d", m.Year), headerWidth)
	writeInfoSection(&b, "Genre", m.Genre, headerWidth)

	// Additional metadata
	if m.Comment != "" {
		writeInfoSection(&b, "Comment", m.Comment, headerWidth)
	}
	if m.Copyright != "" {
		writeInfoSection(&b, "Copyright", m.Copyright, headerWidth)
	}
	if m.TSRC != "" {
		writeInfoSection(&b, "TSRC", m.TSRC, headerWidth)
	}
	if m.EncodedBy != "" {
		writeInfoSection(&b, "Encoded By", m.EncodedBy, headerWidth)
	}

	// Technical details section
	b.WriteString("├" + strings.Repeat("─", headerWidth) + "┤\n")
	b.WriteString("│" + strings.Repeat(" ", padding) + "TECH DETAILS" + strings.Repeat(" ", headerWidth-padding-len("TECH DETAILS")) + "│\n")
	b.WriteString("├" + strings.Repeat("─", headerWidth) + "┤\n")

	writeInfoSection(&b, "Duration", formatDuration(m.Duration), headerWidth)
	writeInfoSection(&b, "Bit Rate", fmt.Sprintf("%d kb/s", m.BitRate), headerWidth)
	writeInfoSection(&b, "Sample Rate", fmt.Sprintf("%d Hz", m.SampleRate), headerWidth)
	writeInfoSection(&b, "Channels", fmt.Sprintf("%d", m.Channels), headerWidth)
	writeInfoSection(&b, "File Size", formatFileSize(m.FileSize), headerWidth)

	// Artwork section
	if m.HasArtwork {
		b.WriteString("├" + strings.Repeat("─", headerWidth) + "┤\n")
		b.WriteString("│" + strings.Repeat(" ", padding) + "ARTWORK" + strings.Repeat(" ", headerWidth-padding-len("ARTWORK")) + "│\n")
		b.WriteString("├" + strings.Repeat("─", headerWidth) + "┤\n")
		writeInfoSection(&b, "MIME Type", m.ArtworkMIME, headerWidth)
		writeInfoSection(&b, "Dimensions", fmt.Sprintf("%dx%d", m.ArtworkSize.X, m.ArtworkSize.Y), headerWidth)
	}

	// Raw tags debug section
	if len(m.RawTags) > 0 {
		b.WriteString("├" + strings.Repeat("─", headerWidth) + "┤\n")
		b.WriteString("│" + strings.Repeat(" ", padding) + "RAW TAGS" + strings.Repeat(" ", headerWidth-padding-len("RAW TAGS")) + "│\n")
		b.WriteString("├" + strings.Repeat("─", headerWidth) + "┤\n")

		// Sort tags for consistent display
		var keys []string
		for k := range m.RawTags {
			keys = append(keys, k)
		}
		for _, k := range keys {
			v := m.RawTags[k]
			writeInfoSection(&b, k, fmt.Sprintf("%v", v), headerWidth)
		}
	}

	// Bottom border
	b.WriteString("└" + strings.Repeat("─", headerWidth) + "┘\n")

	return b.String()
}

// Keep existing helper functions
func tryDecode(s string) string {
	if s == "" {
		return s
	}
	// Already valid UTF-8
	if utf8.ValidString(s) {
		return s
	}

	// Try common encodings
	encodings := []encoding.Encoding{
		charmap.ISO8859_1,
		charmap.Windows1252,
		japanese.EUCJP,
		korean.EUCKR,
		simplifiedchinese.GBK,
	}

	for _, enc := range encodings {
		if decoded, err := enc.NewDecoder().String(s); err == nil {
			return decoded
		}
	}
	return s
}

func writeInfoSection(b *strings.Builder, label, value string, width int) {
	if value == "" {
		return
	}
	const labelWidth = 15
	valueWidth := width - labelWidth - 5 // 5 for "│ " and " │" borders and separator

	// Truncate value if too long
	if len(value) > valueWidth {
		value = value[:valueWidth-3] + "..."
	}

	b.WriteString(fmt.Sprintf("│ %-*s│ %-*s │\n", labelWidth, label, valueWidth, value))
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
