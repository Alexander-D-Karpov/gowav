package audio

import (
	"io"
	"time"

	"github.com/hajimehoshi/go-mp3"
)

type AudioProperties struct {
	Duration   time.Duration
	SampleRate int
	Channels   int
	BitRate    int
}

func extractAudioProperties(reader io.ReadSeeker) (AudioProperties, error) {
	props := AudioProperties{}

	// MP3 decode approach.
	dec, err := mp3.NewDecoder(reader)
	if err == nil {
		props.SampleRate = dec.SampleRate()
		props.Channels = 2

		// Fully read to get the total sample count.
		var totalPCMFrames int64
		buf := make([]byte, 8192)
		for {
			n, readErr := dec.Read(buf)
			if n > 0 {
				totalPCMFrames += int64(n / 4) // 4 bytes per stereo frame
			}
			if readErr == io.EOF {
				break
			}
			if readErr != nil {
				return props, readErr
			}
		}
		durSeconds := float64(totalPCMFrames) / float64(props.SampleRate)
		props.Duration = time.Duration(durSeconds * float64(time.Second))
		return props, nil
	}

	// If not MP3 or decode fails, return defaults (further expansions can be added for FLAC, OGG, etc.).
	return props, nil
}
