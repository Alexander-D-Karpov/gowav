package audio

import (
	"io"
	"time"

	"github.com/hajimehoshi/go-mp3"
)

// AudioProperties holds basic format details like duration or sample rate.
type AudioProperties struct {
	Duration   time.Duration
	SampleRate int
	Channels   int
	BitRate    int
}

// extractAudioProperties attempts to read an MP3 stream to find duration, sample rate, and so forth.
func extractAudioProperties(reader io.ReadSeeker) (AudioProperties, error) {
	props := AudioProperties{}
	dec, err := mp3.NewDecoder(reader)
	if err == nil {
		props.SampleRate = dec.SampleRate()
		props.Channels = 2

		var totalPCMFrames int64
		buf := make([]byte, 8192)
		for {
			n, readErr := dec.Read(buf)
			if n > 0 {
				totalPCMFrames += int64(n / 4)
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
	return props, nil
}
