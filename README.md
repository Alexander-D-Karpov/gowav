# GoWav Music Player

GoWav is a terminal-based music player written in Go, featuring a modern TUI powered by Bubble Tea

## Features

- Multiple audio format support (MP3, FLAC, WAV, OGG)
- Album artwork display in terminal (ASCII art)
- Interactive terminal UI
- Music metadata display and management
- Configurable UI modes (full and mini)

## Installation

### Prerequisites

- Go 1.23.2 or higher
- Audio dependencies (platform specific)

### Building from Source

```bash
git clone https://github.com/Alexander-D-Karpov/gowav.git
cd gowav
go mod download
go build
```

## Usage

### Basic Commands

- `help`, `h` - Show available commands
- `load`, `l <path>` - Load an audio file
- `search`, `s <query>` - Search for music
- `quit`, `q` - Exit the application

### Keyboard Shortcuts

- `Ctrl+Q` - Quit
- `Ctrl+P` - Play
- `Ctrl+Space` - Pause
- `Ctrl+S` - Stop
- `Ctrl+M` - Toggle UI mode
- `?` - Show all shortcuts

## Dependencies

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - Terminal UI framework
- [oto](https://github.com/hajimehoshi/oto) - Audio playback
- [tag](https://github.com/dhowden/tag) - Metadata parsing
