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

Commands can be used with or without a colon prefix (:)
Example: Both "help" and ":help" will work`

	return help, nil, nil
}

func (c *Commander) handleTrackHelp() (string, error, tea.Cmd) {
	help := `Track Mode Commands:

info, i          Show detailed track information
play, p          Play current track
pause            Pause playback
stop             Stop playback
artwork          Show album artwork in ASCII art
unload           Unload current track and return to normal mode
help, h          Show this help message`

	return help, nil, nil
}
