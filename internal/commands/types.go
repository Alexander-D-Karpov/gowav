package commands

import "time"

type Mode int

const (
	ModeNormal Mode = iota
	ModeTrack
)

type Track struct {
	Title    string
	Artist   string
	Album    string
	Duration int
}

type SearchResult struct {
	Title    string
	Artist   string
	Album    string
	Duration int
	URL      string
}

type playbackUpdateMsg struct{}

func FormatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	min := int(d.Minutes())
	sec := int(d.Seconds()) % 60
	return twoDigits(min) + ":" + twoDigits(sec)
}

func twoDigits(val int) string {
	if val < 10 {
		return "0" + string('0'+rune(val))
	}
	return string('0'+rune(val/10)) + string('0'+rune(val%10))
}
