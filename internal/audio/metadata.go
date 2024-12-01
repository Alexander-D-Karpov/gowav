package audio

import (
	"bytes"
	"fmt"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/encoding/unicode"
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
	Disc        string
	BPM         string
	ReleaseDate string
	Duration    time.Duration
	BitRate     int
	SampleRate  int
	Channels    int
	Format      string
	FileSize    int64
	Lyrics      string
	AlbumArtist string
	Artwork     image.Image
	HasArtwork  bool
}

func ExtractMetadata(data []byte) (*Metadata, error) {
	reader := bytes.NewReader(data)

	// First try standard reading
	m, err := tag.ReadFrom(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	// Try to decode text with different encodings
	title := tryDecode(m.Title())
	artist := tryDecode(m.Artist())
	album := tryDecode(m.Album())
	genre := tryDecode(m.Genre())
	albumArtist := tryDecode(m.AlbumArtist())

	metadata := &Metadata{
		Title:       title,
		Artist:      artist,
		Album:       album,
		Year:        m.Year(),
		Genre:       genre,
		FileSize:    int64(len(data)),
		AlbumArtist: albumArtist,
	}

	// Get audio properties
	reader.Seek(0, io.SeekStart)
	decoder, err := mp3.NewDecoder(reader)
	if err == nil {
		sampleRate := int64(decoder.SampleRate())
		metadata.Duration = time.Duration(decoder.Length()) * time.Second / time.Duration(sampleRate)
		metadata.SampleRate = decoder.SampleRate()
		metadata.Channels = 2
		bitrate := int64(len(data)*8) * sampleRate / decoder.Length()
		metadata.BitRate = int(bitrate / 1000)
	} else {
		metadata.Duration = 0
		metadata.SampleRate = 44100
		metadata.Channels = 2
		metadata.BitRate = 320
	}

	// Get additional metadata
	if rawTags := m.Raw(); rawTags != nil {
		metadata.Track = tryDecode(getStringTag(rawTags, "track"))
		metadata.Disc = tryDecode(getStringTag(rawTags, "disc"))
		metadata.BPM = tryDecode(getStringTag(rawTags, "TBPM"))
		if metadata.BPM == "" {
			metadata.BPM = tryDecode(getStringTag(rawTags, "BPM"))
		}

		metadata.ReleaseDate = tryDecode(getStringTag(rawTags, "TDOR"))
		if metadata.ReleaseDate == "" {
			metadata.ReleaseDate = tryDecode(getStringTag(rawTags, "DATE"))
		}

		metadata.Lyrics = tryDecode(getStringTag(rawTags, "lyrics-eng"))
		if metadata.Lyrics == "" {
			metadata.Lyrics = tryDecode(getStringTag(rawTags, "LYRICS"))
		}
	}

	// Handle artwork
	if picture := m.Picture(); picture != nil {
		img, _, err := image.Decode(bytes.NewReader(picture.Data))
		if err == nil {
			metadata.Artwork = img
			metadata.HasArtwork = true
		}
	}

	if metadata.Title == "" {
		metadata.Title = "Unknown Title"
	}
	if metadata.Artist == "" {
		metadata.Artist = "Unknown Artist"
	}
	if metadata.Album == "" {
		metadata.Album = "Unknown Album"
	}

	return metadata, nil
}

func tryDecode(text string) string {
	if text == "" {
		return ""
	}

	// List of encodings to try
	decoders := []struct {
		name    string
		decoder func([]byte) (string, error)
	}{
		{"UTF-8", func(b []byte) (string, error) { return string(b), nil }},
		{"Windows-1251", func(b []byte) (string, error) {
			decoder := charmap.Windows1251.NewDecoder()
			return decoder.String(string(b))
		}},
		{"KOI8-R", func(b []byte) (string, error) {
			decoder := charmap.KOI8R.NewDecoder()
			return decoder.String(string(b))
		}},
		{"ISO-8859-5", func(b []byte) (string, error) {
			decoder := charmap.ISO8859_5.NewDecoder()
			return decoder.String(string(b))
		}},
		{"CP866", func(b []byte) (string, error) {
			decoder := charmap.CodePage866.NewDecoder()
			return decoder.String(string(b))
		}},
		{"GB18030", func(b []byte) (string, error) {
			decoder := simplifiedchinese.GB18030.NewDecoder()
			return decoder.String(string(b))
		}},
		{"Big5", func(b []byte) (string, error) {
			decoder := traditionalchinese.Big5.NewDecoder()
			return decoder.String(string(b))
		}},
		{"EUC-JP", func(b []byte) (string, error) {
			decoder := japanese.EUCJP.NewDecoder()
			return decoder.String(string(b))
		}},
		{"EUC-KR", func(b []byte) (string, error) {
			decoder := korean.EUCKR.NewDecoder()
			return decoder.String(string(b))
		}},
	}

	// Try each decoder
	input := []byte(text)
	for _, dec := range decoders {
		decoded, err := dec.decoder(input)
		if err == nil && isReadable(decoded) {
			return decoded
		}
	}

	// If nothing worked, try UTF-16
	decoder := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewDecoder()
	if decoded, err := decoder.String(text); err == nil && isReadable(decoded) {
		return decoded
	}

	// As a last resort, try to clean up the string
	return cleanString(text)
}

// isReadable checks if the string contains readable characters
func isReadable(s string) bool {
	if s == "" {
		return false
	}

	readable := 0
	for _, r := range s {
		if r >= 32 && r < 127 || r >= 0x400 && r <= 0x4FF || r >= 0x3040 && r <= 0x30FF || r >= 0x4E00 && r <= 0x9FFF {
			readable++
		}
	}
	return float64(readable)/float64(len([]rune(s))) > 0.5
}

// cleanString removes or replaces problematic characters
func cleanString(s string) string {
	var result strings.Builder
	for _, r := range s {
		if r >= 32 && r < 127 || r >= 0x400 && r <= 0x4FF || r >= 0x3040 && r <= 0x30FF || r >= 0x4E00 && r <= 0x9FFF {
			result.WriteRune(r)
		} else {
			result.WriteRune('?')
		}
	}
	return result.String()
}

func getStringTag(tags map[string]interface{}, key string) string {
	if val, ok := tags[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func (m *Metadata) String() string {
	var b strings.Builder

	b.WriteString("┌─── Track Information ──────────────────────────────\n")
	fmt.Fprintf(&b, "│ %-15s: %s\n", "Title", m.Title)
	fmt.Fprintf(&b, "│ %-15s: %s\n", "Artist", m.Artist)
	if m.AlbumArtist != "" && m.AlbumArtist != m.Artist {
		fmt.Fprintf(&b, "│ %-15s: %s\n", "Album Artist", m.AlbumArtist)
	}
	fmt.Fprintf(&b, "│ %-15s: %s\n", "Album", m.Album)
	if m.Track != "" {
		fmt.Fprintf(&b, "│ %-15s: %s\n", "Track", m.Track)
	}
	if m.Disc != "" {
		fmt.Fprintf(&b, "│ %-15s: %s\n", "Disc", m.Disc)
	}
	b.WriteString("├─── Technical Details ─────────────────────────────\n")
	fmt.Fprintf(&b, "│ %-15s: %s\n", "Format", m.Format)
	fmt.Fprintf(&b, "│ %-15s: %d bytes\n", "File Size", m.FileSize)
	if m.Year != 0 {
		fmt.Fprintf(&b, "│ %-15s: %d\n", "Year", m.Year)
	}
	if m.ReleaseDate != "" {
		fmt.Fprintf(&b, "│ %-15s: %s\n", "Release Date", m.ReleaseDate)
	}
	fmt.Fprintf(&b, "│ %-15s: %s\n", "Genre", m.Genre)
	if m.BPM != "" {
		fmt.Fprintf(&b, "│ %-15s: %s\n", "BPM", m.BPM)
	}
	b.WriteString("├─── Audio Specifications ──────────────────────────\n")
	fmt.Fprintf(&b, "│ %-15s: %s\n", "Duration", formatDuration(m.Duration))
	fmt.Fprintf(&b, "│ %-15s: %d kbps\n", "Bit Rate", m.BitRate)
	fmt.Fprintf(&b, "│ %-15s: %d Hz\n", "Sample Rate", m.SampleRate)
	fmt.Fprintf(&b, "│ %-15s: %d\n", "Channels", m.Channels)
	if m.HasArtwork {
		b.WriteString("├─── Artwork ──────────────────────────────────────\n")
		bounds := m.Artwork.Bounds()
		fmt.Fprintf(&b, "│ %-15s: %dx%d pixels\n", "Dimensions", bounds.Dx(), bounds.Dy())
	}
	b.WriteString("└──────────────────────────────────────────────────\n")

	return b.String()
}
