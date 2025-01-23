package audio

import (
	"github.com/hajimehoshi/go-mp3"
	"io"
	"time"
)

type AudioProperties struct {
	Duration   time.Duration
	SampleRate int
	Channels   int
	BitRate    int
}

func extractAudioProperties(reader io.ReadSeeker) (AudioProperties, error) {
	props := AudioProperties{}

	// Try MP3 decoder first
	decoder, err := mp3.NewDecoder(reader)
	if err == nil {
		sampleRate := int64(decoder.SampleRate())
		props.SampleRate = decoder.SampleRate()
		props.Channels = 2
		props.Duration = time.Duration(decoder.Length()) * time.Second / time.Duration(sampleRate)

		// Calculate bitrate based on file size and duration
		reader.Seek(0, io.SeekEnd)
		size, _ := reader.Seek(0, io.SeekCurrent)
		reader.Seek(0, io.SeekStart)

		if props.Duration > 0 {
			bitrate := int64(size*8) * sampleRate / decoder.Length()
			props.BitRate = int(bitrate / 1000)
		} else {
			props.BitRate = 320 // Default assumption
		}

		return props, nil
	}

	// Reset reader position
	reader.Seek(0, io.SeekStart)

	// Fallback to sensible defaults
	props.Duration = time.Duration(0)
	props.SampleRate = 44100
	props.Channels = 2
	props.BitRate = 320

	return props, nil
}
