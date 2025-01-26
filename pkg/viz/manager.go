package viz

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"strings"
	"sync"
	"time"
)

type Manager struct {
	visualizations map[ViewMode]Visualization
	currentMode    ViewMode
	state          ViewState
	mu             sync.RWMutex
}

func NewManager() *Manager {
	return &Manager{
		visualizations: make(map[ViewMode]Visualization),
		state: ViewState{
			Mode:   WaveformMode,
			Zoom:   1.0,
			Width:  80,
			Height: 24,
			ColorScheme: ColorScheme{
				Primary:   lipgloss.Color("#00ff00"),
				Secondary: lipgloss.Color("#0088ff"),
				Text:      lipgloss.Color("#ffffff"),
			},
		},
	}
}

func (m *Manager) AddVisualization(mode ViewMode, viz Visualization) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.visualizations[mode] = viz
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

func (m *Manager) CycleMode(direction int) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	modes := []ViewMode{
		WaveformMode,
		SpectrogramMode,
		TempoMode,
		BeatMapMode,
	}

	// Find current index
	currentIdx := -1
	for i, mode := range modes {
		if mode == m.currentMode {
			currentIdx = i
			break
		}
	}

	// Calculate next mode
	nextIdx := 0
	if currentIdx != -1 {
		nextIdx = (currentIdx + direction + len(modes)) % len(modes)
	}

	nextMode := modes[nextIdx]
	if _, ok := m.visualizations[nextMode]; !ok {
		return "", fmt.Errorf("visualization not available: %v", nextMode)
	}

	m.currentMode = nextMode
	return m.visualizations[nextMode].Name(), nil
}

func (m *Manager) Render() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	viz, ok := m.visualizations[m.currentMode]
	if !ok {
		return "No visualization available"
	}

	var sb strings.Builder

	// Render title and description
	title := fmt.Sprintf("%s - %s", viz.Name(), viz.Description())
	sb.WriteString(lipgloss.NewStyle().
		Bold(true).
		Foreground(m.state.ColorScheme.Text).
		Render(title))
	sb.WriteString("\n")

	// Render main visualization
	sb.WriteString(viz.Render(m.state))

	// Render controls info
	controlsText := "←/→: Scroll | +/-: Zoom | 0: Reset | Tab: Next View"
	sb.WriteString("\n")
	sb.WriteString(lipgloss.NewStyle().
		Foreground(m.state.ColorScheme.Text).
		Render(controlsText))

	return sb.String()
}

func (m *Manager) UpdateZoom(factor float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	newZoom := m.state.Zoom * factor
	if newZoom >= 0.1 && newZoom <= 10.0 {
		m.state.Zoom = newZoom
	}
}

func (m *Manager) UpdateOffset(delta time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.state.Offset += delta
	if m.state.Offset < 0 {
		m.state.Offset = 0
	}
	if m.state.TotalDuration > 0 && m.state.Offset > m.state.TotalDuration {
		m.state.Offset = m.state.TotalDuration
	}
}

func (m *Manager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.state.Zoom = 1.0
	m.state.Offset = 0
}

func (m *Manager) SetDimensions(width, height int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.state.Width = width
	m.state.Height = height
}
