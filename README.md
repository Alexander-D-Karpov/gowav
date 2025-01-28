# GoWav Music Player

GoWav is a terminal-based music player written in Go, featuring a modern TUI (Terminal User Interface) powered by Bubble Tea

## Features

- Multiple audio format support (MP3, FLAC, WAV, OGG)
- Interactive visualizations:
    - Waveform display
    - Spectrogram
    - Beat/rhythm patterns
    - Tempo analysis
    - Audio density mapping
- Album artwork display in terminal (ASCII art)
- Detailed metadata display and management
- Multiple UI modes (full and mini)

## Keyboard Controls

### General Controls
- `Ctrl+Q` - Quit
- `Ctrl+M` - Toggle UI mode
- `Ctrl+L` - Clear screen
- `?` - Show keyboard shortcuts

### Playback Controls
- `Ctrl+P` - Play
- `Ctrl+Space` - Pause
- `Ctrl+S` - Stop
- `Ctrl+U/D` - Volume up/down

### Visualization Controls
- `v` - Enter visualization mode
- `Tab` - Next visualization
- `Shift+Tab` - Previous visualization
- `+/-` - Zoom in/out
- `←/→` - Scroll through track
- `0` - Reset view
- `Esc` - Exit visualization mode

## Basic Commands
```
help, h          Show help
load, l <path>   Load audio file
search, s        Search for music
viz              Enter visualization mode
quit, q          Exit application
```

### Visualization Types
- `viz wave` - Waveform visualization
- `viz spectrum` - Spectrogram display
- `viz tempo` - Tempo/energy analysis
- `viz density` - Audio density map
- `viz beat` - Beat and rhythm patterns

## Building from Source

### Prerequisites
- Go 1.23.2 or higher
- Audio dependencies (platform specific)

```bash
git clone https://github.com/Alexander-D-Karpov/gowav.git
cd gowav
go mod download
go build
```

## License
MIT License - see LICENSE file for details

## Dependencies

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - Terminal UI framework
- [oto](https://github.com/hajimehoshi/oto) - Audio playback
- [tag](https://github.com/dhowden/tag) - Metadata parsing


## TODO:
- loading folder with music
- music conversion
- music editing
- music streaming
- loading from online sources
- better UI (more interactive, more information, more customization)
- better audio support (more formats, more options)