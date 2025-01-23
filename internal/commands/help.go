package commands

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (c *Commander) handleHelp() (string, error, tea.Cmd) {
	help := `Available Commands:
    
help, h          Show this help message
load, l <path>   Load audio file from path or URL
search, s <query> Search for tracks
quit, q, exit    Exit application

(type 'help' for more info)`

	return help, nil, nil
}

func (c *Commander) handleTrackHelp() (string, error, tea.Cmd) {
	help := `Track Mode Commands:

info, i          Show detailed track information
play, p          Play current track
pause            Pause playback
stop             Stop playback
artwork          Show album artwork in ASCII
unload           Unload current track, return to normal mode

viz wave         Waveform visualization
viz spectrum     Frequency (Spectrogram) visualization
viz tempo        Tempo/energy analysis
viz density      Density map
viz beat         Beat/rhythm patterns

help, h          Show this help message
`
	return help, nil, nil
}
