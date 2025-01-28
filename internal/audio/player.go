package audio

import (
	"fmt"
	"github.com/hajimehoshi/oto"
	"strings"
	"sync"
	"time"
)

// PlaybackState enumerates whether the track is playing, paused, or stopped.
type PlaybackState int

const (
	StateStopped PlaybackState = iota
	StatePlaying
	StatePaused
)

// Player holds the audio playback context and position/duration information.
type Player struct {
	mutex       sync.Mutex
	context     *oto.Context
	player      *oto.Player
	state       PlaybackState
	buffer      []byte
	position    time.Duration
	duration    time.Duration
	sampleRate  int
	numChannels int
	lastUpdate  time.Time
}

// NewPlayer creates a Player with default sampleRate=44100, stereo.
func NewPlayer() *Player {
	return &Player{
		state:       StateStopped,
		lastUpdate:  time.Now(),
		sampleRate:  44100,
		numChannels: 2,
	}
}

// Play writes the provided data to the Oto player. If already playing, does nothing.
func (p *Player) Play(data []byte) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.state == StatePlaying {
		return nil
	}

	if p.context == nil {
		ctx, err := oto.NewContext(p.sampleRate, p.numChannels, 2, 4096)
		if err != nil {
			return fmt.Errorf("failed to create audio context: %w", err)
		}
		p.context = ctx
	}

	// If resuming from paused, skip re-buffer. Otherwise, create new Oto player.
	if p.state != StatePaused {
		if p.player != nil {
			p.player.Close()
		}
		p.player = p.context.NewPlayer()
		p.buffer = data
		_, err := p.player.Write(data)
		if err != nil {
			return fmt.Errorf("failed to write to player: %w", err)
		}
	}

	p.state = StatePlaying
	p.lastUpdate = time.Now()
	return nil
}

// Pause halts playback but retains the current track position for potential resume.
func (p *Player) Pause() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.state != StatePlaying || p.player == nil {
		return nil
	}

	p.updatePosition()
	if p.player != nil {
		p.player.Close()
		p.player = nil
	}
	p.state = StatePaused
	return nil
}

// Stop fully resets playback and position.
func (p *Player) Stop() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.player != nil {
		p.player.Close()
		p.player = nil
	}
	p.buffer = nil
	p.state = StateStopped
	p.position = 0
	return nil
}

// GetState returns whether the player is playing, paused, or stopped.
func (p *Player) GetState() PlaybackState {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.state
}

// GetPosition returns the current playback position.
func (p *Player) GetPosition() time.Duration {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	if p.state == StatePlaying {
		p.updatePosition()
	}
	return p.position
}

// updatePosition accumulates how long we've been playing since lastUpdate.
func (p *Player) updatePosition() {
	if p.state == StatePlaying {
		elapsed := time.Since(p.lastUpdate)
		p.position += elapsed
		p.lastUpdate = time.Now()
	}
}

// SetDuration allows the Player to show the correct total track length for UI displays.
func (p *Player) SetDuration(d time.Duration) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.duration = d
}

// GetDuration returns the total duration of the track, if known.
func (p *Player) GetDuration() time.Duration {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.duration
}

// RenderTrackBar draws a simple text-based “progress bar” for the track’s current position.
func (p *Player) RenderTrackBar(width int) string {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.state == StateStopped {
		return ""
	}

	p.updatePosition()
	progress := float64(p.position) / float64(p.duration)
	if progress > 1.0 {
		progress = 1.0
	}

	barWidth := width - 20
	if barWidth < 1 {
		barWidth = 1
	}
	completed := int(float64(barWidth) * progress)

	var bar strings.Builder
	bar.WriteString("\r[")

	for i := 0; i < barWidth; i++ {
		if i < completed {
			bar.WriteString("━")
		} else if i == completed {
			if p.state == StatePlaying {
				bar.WriteString("⭘")
			} else {
				bar.WriteString("□")
			}
		} else {
			bar.WriteString("─")
		}
	}

	posStr := formatDuration(p.position)
	durStr := formatDuration(p.duration)
	bar.WriteString(fmt.Sprintf("] %s/%s", posStr, durStr))

	return bar.String()
}

// RefreshPosition updates the player's position if it is playing.
func (p *Player) RefreshPosition() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.updatePosition()
}
