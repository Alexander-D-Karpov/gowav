package audio

import (
	"bytes"
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"github.com/dhowden/tag"
	"github.com/hajimehoshi/go-mp3"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"strings"
	"time"
	"unicode/utf8"

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
	Encoder     string
	Comment     string
	Copyright   string
	TSRC        string
	EncodedBy   string
	ReleaseDate string
	Duration    time.Duration
	BitRate     int
	SampleRate  int
	Channels    int
	FileSize    int64
	HasArtwork  bool
	ArtworkMIME string
	ArtworkSize image.Point
	Artwork     image.Image
	BPM         string
	Lyrics      string
	RawTags     map[string]interface{}
}

func ExtractMetadata(data []byte) (*Metadata, error) {
	reader := bytes.NewReader(data)
	m, err := tag.ReadFrom(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	metadata := &Metadata{
		Title:       tryDecode(m.Title()),
		Artist:      tryDecode(m.Artist()),
		Album:       tryDecode(m.Album()),
		Year:        m.Year(),
		Genre:       tryDecode(m.Genre()),
		FileSize:    int64(len(data)),
		AlbumArtist: tryDecode(m.AlbumArtist()),
	}

	reader.Seek(0, io.SeekStart)
	decoder, err := mp3.NewDecoder(reader)
	if err == nil {
		var totalPCMFrames int64
		buf := make([]byte, 8192)
		for {
			n, readErr := decoder.Read(buf)
			if n > 0 {
				totalPCMFrames += int64(n / 4) // 4 bytes per stereo frame
			}
			if readErr == io.EOF {
				break
			}
			if readErr != nil {
				break
			}
		}
		sampleRate := decoder.SampleRate()
		if totalPCMFrames > 0 && sampleRate > 0 {
			metadata.Duration = time.Duration(float64(totalPCMFrames) / float64(sampleRate) * float64(time.Second))
			metadata.SampleRate = sampleRate
			metadata.Channels = 2
			metadata.BitRate = int(float64(len(data)*8) / metadata.Duration.Seconds() / 1000)
		}
	}

	if rawTags := m.Raw(); rawTags != nil {
		metadata.RawTags = rawTags
		metadata.Track = getStringTag(rawTags, "TRCK")
		metadata.Disc = getStringTag(rawTags, "TPOS")
		metadata.EncodedBy = getStringTag(rawTags, "TENC")
		metadata.Comment = getStringTag(rawTags, "COMM")
		metadata.Copyright = getStringTag(rawTags, "TCOP")
		metadata.TSRC = getStringTag(rawTags, "TSRC")
		metadata.Encoder = getStringTag(rawTags, "TSSE")

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
				logDebug("Unknown APIC type: %T, trying raw bytes fallback", apicData)
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

// BuildLoadInfo gives a “partial table.” If we’re wide enough, put artwork on the right.
func (m *Metadata) BuildLoadInfo(termWidth, termHeight int) string {
	// If the user’s terminal is at least this wide, we do side‐by‐side
	const neededForSideBySide = 60
	if termWidth < 30 {
		termWidth = 30
	}
	if termHeight < 10 {
		termHeight = 10
	}

	tableStr := m.renderTable(termWidth-2, false, true)

	if !m.HasArtwork || m.Artwork == nil || termWidth < neededForSideBySide {
		// Not enough columns => artwork on top, then table
		if m.HasArtwork && m.Artwork != nil {
			artStr := m.renderArtworkColorBlocks(termWidth-4, termHeight/2)
			return artStr + "\n" + tableStr
		}
		return tableStr
	}

	// Enough columns => side by side
	linesTable := strings.Split(tableStr, "\n")
	artStr := m.renderArtworkColorBlocks(30, termHeight-4)
	linesArt := strings.Split(artStr, "\n")

	maxLines := len(linesTable)
	if len(linesArt) > maxLines {
		maxLines = len(linesArt)
	}

	// measure max line length from the table
	maxTableWidth := 0
	for _, ln := range linesTable {
		if len(ln) > maxTableWidth {
			maxTableWidth = len(ln)
		}
	}
	sideSpacing := maxTableWidth + 2

	var sb strings.Builder
	for i := 0; i < maxLines; i++ {
		var left, right string
		if i < len(linesTable) {
			left = linesTable[i]
		}
		if i < len(linesArt) {
			right = linesArt[i]
		}

		sb.WriteString(left)
		if len(left) < sideSpacing {
			sb.WriteString(strings.Repeat(" ", sideSpacing-len(left)))
		} else {
			sb.WriteString(" ")
		}
		sb.WriteString(right)
		sb.WriteString("\n")
	}
	return sb.String()
}

// AdaptiveStringWithRaw: full table + raw tags, ignoring side-by-side logic
func (m *Metadata) AdaptiveStringWithRaw(termWidth, termHeight int) string {
	return m.renderTable(termWidth-2, true, false)
}

func (m *Metadata) renderTable(width int, includeRaw bool, includeArtworkMeta bool) string {
	if width < 20 {
		width = 20
	}

	headerWidth := width
	b := new(bytes.Buffer)

	topBorder := "┌" + strings.Repeat("─", headerWidth) + "┐\n"
	b.WriteString(topBorder)

	title := "TRACK INFORMATION"
	padding := (headerWidth - len(title)) / 2
	if padding < 0 {
		padding = 0
	}
	b.WriteString("│" + strings.Repeat(" ", padding) + title +
		strings.Repeat(" ", headerWidth-padding-len(title)) + "│\n")

	sep := "├" + strings.Repeat("─", headerWidth) + "┤\n"
	b.WriteString(sep)

	writeInfoSection(b, "Title", m.Title, headerWidth)
	writeInfoSection(b, "Artist", m.Artist, headerWidth)
	writeInfoSection(b, "Album", m.Album, headerWidth)
	if m.Track != "" {
		writeInfoSection(b, "Track", m.Track, headerWidth)
	}
	if m.Disc != "" {
		writeInfoSection(b, "Disc", m.Disc, headerWidth)
	}
	writeInfoSection(b, "Year", fmt.Sprintf("%d", m.Year), headerWidth)
	writeInfoSection(b, "Genre", m.Genre, headerWidth)
	if m.Comment != "" {
		writeInfoSection(b, "Comment", m.Comment, headerWidth)
	}
	if m.TSRC != "" {
		writeInfoSection(b, "TSRC", m.TSRC, headerWidth)
	}
	if m.EncodedBy != "" {
		writeInfoSection(b, "Encoded By", m.EncodedBy, headerWidth)
	}

	b.WriteString(sep)
	techTitle := "TECH DETAILS"
	tPad := (headerWidth - len(techTitle)) / 2
	if tPad < 0 {
		tPad = 0
	}
	b.WriteString("│" + strings.Repeat(" ", tPad) + techTitle +
		strings.Repeat(" ", headerWidth-tPad-len(techTitle)) + "│\n")
	b.WriteString(sep)

	writeInfoSection(b, "Duration", formatDuration(m.Duration), headerWidth)
	writeInfoSection(b, "Bit Rate", fmt.Sprintf("%d kb/s", m.BitRate), headerWidth)
	writeInfoSection(b, "Sample Rate", fmt.Sprintf("%d Hz", m.SampleRate), headerWidth)
	writeInfoSection(b, "Channels", fmt.Sprintf("%d", m.Channels), headerWidth)
	writeInfoSection(b, "File Size", formatFileSize(m.FileSize), headerWidth)

	if includeArtworkMeta && m.HasArtwork {
		b.WriteString(sep)
		artTitle := "ARTWORK"
		aPad := (headerWidth - len(artTitle)) / 2
		if aPad < 0 {
			aPad = 0
		}
		b.WriteString("│" + strings.Repeat(" ", aPad) + artTitle +
			strings.Repeat(" ", headerWidth-aPad-len(artTitle)) + "│\n")
		b.WriteString(sep)
		writeInfoSection(b, "MIME Type", m.ArtworkMIME, headerWidth)
		writeInfoSection(b, "Dimensions", fmt.Sprintf("%dx%d", m.ArtworkSize.X, m.ArtworkSize.Y), headerWidth)
	}

	if includeRaw && len(m.RawTags) > 0 {
		b.WriteString(sep)
		rawTitle := "RAW TAGS"
		rPad := (headerWidth - len(rawTitle)) / 2
		if rPad < 0 {
			rPad = 0
		}
		b.WriteString("│" + strings.Repeat(" ", rPad) + rawTitle +
			strings.Repeat(" ", headerWidth-rPad-len(rawTitle)) + "│\n")
		b.WriteString(sep)

		var keys []string
		for k := range m.RawTags {
			keys = append(keys, k)
		}
		for _, k := range keys {
			v := m.RawTags[k]
			writeInfoSection(b, k, fmt.Sprintf("%v", v), headerWidth)
		}
	}

	botBorder := "└" + strings.Repeat("─", headerWidth) + "┘\n"
	b.WriteString(botBorder)

	return b.String()
}

func (m *Metadata) renderArtworkColorBlocks(targetWidth, targetHeight int) string {
	if !m.HasArtwork || m.Artwork == nil {
		return ""
	}
	bounds := m.Artwork.Bounds()
	origW, origH := bounds.Dx(), bounds.Dy()
	if origW < 1 || origH < 1 {
		return ""
	}

	scaleX := float64(origW) / float64(targetWidth)
	scaleY := float64(origH) / float64(targetHeight)
	if scaleX < 1 {
		scaleX = 1
	}
	if scaleY < 1 {
		scaleY = 1
	}

	var sb strings.Builder
	for y := 0; y < targetHeight; y++ {
		yy := int(float64(y) * scaleY)
		if yy >= origH {
			break
		}
		for x := 0; x < targetWidth; x++ {
			xx := int(float64(x) * scaleX)
			if xx >= origW {
				break
			}
			r, g, b, _ := m.Artwork.At(xx, yy).RGBA()
			r >>= 8
			g >>= 8
			b >>= 8

			colorCode := fmt.Sprintf("#%02x%02x%02x", r, g, b)
			block := lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorCode)).
				Render("█")
			sb.WriteString(block)
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func extractAndSetArtwork(metadata *Metadata, data []byte, mimeType string) error {
	logDebug("Image data starts with bytes: % x", data[:min(16, len(data))])
	if len(data) < 12 {
		return fmt.Errorf("data too short for image")
	}

	var imgData []byte
	switch {
	case bytes.HasPrefix(data, []byte{0xff, 0xd8, 0xff}):
		imgData = data
	case bytes.HasPrefix(data, []byte{0x89, 0x50, 0x4e, 0x47}):
		imgData = data
	default:
		jpegIndex := bytes.Index(data, []byte{0xff, 0xd8, 0xff})
		pngIndex := bytes.Index(data, []byte{0x89, 0x50, 0x4e, 0x47})
		if jpegIndex >= 0 {
			imgData = data[jpegIndex:]
		} else if pngIndex >= 0 {
			imgData = data[pngIndex:]
		} else {
			imgData = data
		}
	}

	var img image.Image
	var format string
	var err error

	if img, err = jpeg.Decode(bytes.NewReader(imgData)); err == nil {
		format = "jpeg"
	} else {
		if img, err = png.Decode(bytes.NewReader(imgData)); err == nil {
			format = "png"
		} else {
			if img, format, err = image.Decode(bytes.NewReader(imgData)); err != nil {
				return fmt.Errorf("failed to decode image: %w", err)
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

func getRawBytes(data interface{}) ([]byte, bool) {
	if str, ok := data.(string); ok {
		return []byte(str), true
	}
	if byt, ok := data.([]byte); ok {
		return byt, true
	}
	return nil, false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func tryDecode(s string) string {
	if s == "" {
		return s
	}
	if utf8.ValidString(s) {
		return s
	}
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

func writeInfoSection(b *bytes.Buffer, label, value string, width int) {
	if value == "" {
		return
	}
	labelWidth := 15
	valueWidth := width - labelWidth - 5
	if valueWidth < 1 {
		valueWidth = 1
	}

	runes := []rune(value)
	if len(runes) > valueWidth-3 {
		runes = runes[:valueWidth-3]
		value = string(runes) + "..."
	}

	fmt.Fprintf(b, "│ %-*s│ %-*s │\n", labelWidth, label, valueWidth, value)
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
