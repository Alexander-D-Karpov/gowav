package audio

import (
	"fmt"
	"github.com/hajimehoshi/oto"
	"strings"
	"sync"
	"time"
)

type PlaybackState int

const (
	StateStopped PlaybackState = iota
	StatePlaying
	StatePaused
)

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

func NewPlayer() *Player {
	return &Player{
		state:       StateStopped,
		lastUpdate:  time.Now(),
		sampleRate:  44100,
		numChannels: 2,
	}
}

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

func (p *Player) GetState() PlaybackState {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.state
}

func (p *Player) GetPosition() time.Duration {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	if p.state == StatePlaying {
		p.updatePosition()
	}
	return p.position
}

func (p *Player) updatePosition() {
	if p.state == StatePlaying {
		elapsed := time.Since(p.lastUpdate)
		p.position += elapsed
		p.lastUpdate = time.Now()
	}
}

func (p *Player) SetDuration(d time.Duration) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.duration = d
}

func (p *Player) GetDuration() time.Duration {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.duration
}

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

func (p *Player) RefreshPosition() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.updatePosition()
}
