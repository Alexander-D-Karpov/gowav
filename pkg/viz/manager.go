package viz

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"sort"
	"strings"
	"sync"
	"time"
)

type Manager struct {
	visualizations map[ViewMode]Visualization
	currentMode    ViewMode
	state          ViewState
	mu             sync.RWMutex

	position    time.Duration
	duration    time.Duration
	windowSize  time.Duration
	scrollSpeed time.Duration
	zoomFactor  float64
	colorScheme string
}

func NewManager() *Manager {
	return &Manager{
		visualizations: make(map[ViewMode]Visualization),
		state: ViewState{
			Mode:          WaveformMode,
			Zoom:          1.0,
			Width:         80,
			Height:        24,
			ColorScheme:   DefaultColorScheme(),
			IsInteractive: true,
			WindowSize:    10 * time.Second,
			ScrollSpeed:   1 * time.Second,
		},
		windowSize:  10 * time.Second,
		scrollSpeed: 1 * time.Second,
		zoomFactor:  1.2,
		colorScheme: "default",
	}
}

func (m *Manager) AddVisualization(mode ViewMode, viz Visualization) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.visualizations[mode] = viz
}

func (m *Manager) SetAudioDuration(duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.duration = duration
	m.state.TotalDuration = duration
}

func (m *Manager) HandleInput(key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch key {
	case "left", "h":
		if m.position > 0 {
			m.position -= time.Duration(float64(m.scrollSpeed) * m.state.Zoom)
			if m.position < 0 {
				m.position = 0
			}
			m.state.Offset = m.position
		}
		return true

	case "right", "l":
		maxPos := m.duration - time.Duration(float64(m.windowSize)/m.state.Zoom)
		m.position += time.Duration(float64(m.scrollSpeed) * m.state.Zoom)
		if m.position > maxPos {
			m.position = maxPos
		}
		m.state.Offset = m.position
		return true

	case "+", "=":
		if m.state.Zoom < 10.0 {
			m.state.Zoom *= m.zoomFactor
			// Adjust position to maintain center point
			m.position = time.Duration(float64(m.position) * m.zoomFactor)
			m.state.Offset = m.position
		}
		return true

	case "-", "_":
		if m.state.Zoom > 0.1 {
			m.state.Zoom /= m.zoomFactor
			// Adjust position to maintain center point
			m.position = time.Duration(float64(m.position) / m.zoomFactor)
			m.state.Offset = m.position
		}
		return true

	case "0":
		m.position = 0
		m.state.Offset = 0
		m.state.Zoom = 1.0
		return true
	}

	if viz, ok := m.visualizations[m.currentMode]; ok {
		return viz.HandleInput(key, &m.state)
	}

	return false
}

func (m *Manager) cycleMode(direction int) bool {
	modes := []ViewMode{
		WaveformMode,
		SpectrogramMode,
		DensityMode,
		TempoMode,
		BeatMapMode,
	}

	currentIndex := -1
	for i, mode := range modes {
		if mode == m.currentMode {
			currentIndex = i
			break
		}
	}

	if currentIndex == -1 {
		return false
	}

	newIndex := (currentIndex + direction + len(modes)) % len(modes)
	newMode := modes[newIndex]

	if _, ok := m.visualizations[newMode]; ok {
		m.currentMode = newMode
		return true
	}

	return false
}

func (m *Manager) SetMode(mode ViewMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.visualizations[mode]; !exists {
		return fmt.Errorf("visualization mode not available: %v", mode)
	}
	m.currentMode = mode
	return nil
}

func (m *Manager) SetDimensions(width, height int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state.Width = width
	m.state.Height = height
}

func (m *Manager) Render() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	viz, ok := m.visualizations[m.currentMode]
	if !ok {
		return "No visualization available for the current mode."
	}

	var sb strings.Builder
	sb.WriteString(m.renderHeader())
	sb.WriteString("\n")
	sb.WriteString(viz.Render(m.state))
	sb.WriteString("\n")
	sb.WriteString(m.renderControls())
	return sb.String()
}

func (m *Manager) renderHeader() string {
	viz := m.visualizations[m.currentMode]
	if viz == nil {
		return ""
	}

	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(m.state.ColorScheme.Text).
		Background(m.state.ColorScheme.Background)

	pos := fmt.Sprintf("Position: %s/%s",
		formatDuration(m.position),
		formatDuration(m.duration))
	zoom := fmt.Sprintf("Zoom: %.2fx", m.state.Zoom)

	return style.Render(fmt.Sprintf(" %s - %s | %s | %s ",
		viz.Name(), viz.Description(), pos, zoom))
}

func (m *Manager) renderControls() string {
	controls := []string{
		"←/→: Scroll",
		"+/-: Zoom",
		"0: Reset",
		"Tab: Next Viz",
		"q: Quit Viz",
	}

	style := lipgloss.NewStyle().
		Foreground(m.state.ColorScheme.Text).
		Background(m.state.ColorScheme.Background)
	return style.Render(" " + strings.Join(controls, " | ") + " ")
}

func (m *Manager) cycleColorScheme() {
	// Get all available schemes
	var schemes []string
	for name := range ColorSchemes {
		schemes = append(schemes, name)
	}
	sort.Strings(schemes) // Keep order consistent

	// Find current scheme index
	currentIndex := -1
	for i, name := range schemes {
		if name == m.colorScheme {
			currentIndex = i
			break
		}
	}

	// Cycle to next scheme
	nextIndex := (currentIndex + 1) % len(schemes)
	m.colorScheme = schemes[nextIndex]
	m.state.ColorScheme = ColorSchemes[m.colorScheme]
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	m := d / time.Minute
	s := (d - m*time.Minute) / time.Second
	return fmt.Sprintf("%02d:%02d", m, s)
}
