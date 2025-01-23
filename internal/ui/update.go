package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"gowav/internal/audio"
)

// progressMsg is a bubble for manual progress commands if you ever want them.
type progressMsg float64

// Update is the main update function for our TUI’s bubbletea loop.
func (m AudioModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	//----------------------------------------------------------------------
	// Example streaming progress updates
	//----------------------------------------------------------------------
	case streamMsg:
		newModel, cmd := m.updateStreamProgress(msg)
		return newModel, cmd

	//----------------------------------------------------------------------
	// Example manual progress messages
	//----------------------------------------------------------------------
	case progressMsg:
		var cmd tea.Cmd
		newProgress, cmd := m.progress.Update(float64(msg))
		m.progress = newProgress.(progress.Model)
		return m, cmd

	//----------------------------------------------------------------------
	// Bubble Tea key events
	//----------------------------------------------------------------------
	case tea.KeyMsg:
		// If we are in visualization mode, interpret certain keys for scrolling/zooming/quit:
		if m.uiMode == ModeViz && m.commander.IsInTrackMode() {
			switch msg.String() {
			case "esc", "q":
				m.uiMode = ModeFull
				return m, nil

			case "tab":
				m.commander.GetProcessor().HandleVisualizationInput("next")
				return m, nil

			case "shift+tab":
				m.commander.GetProcessor().HandleVisualizationInput("prev")
				return m, nil

			case "+", "=":
				m.commander.GetProcessor().HandleVisualizationInput("zoom-in")
				return m, nil

			case "-", "_":
				m.commander.GetProcessor().HandleVisualizationInput("zoom-out")
				return m, nil

			case "left", "h":
				m.commander.GetProcessor().HandleVisualizationInput("left")
				return m, nil

			case "right", "l":
				m.commander.GetProcessor().HandleVisualizationInput("right")
				return m, nil

			case "0":
				m.commander.GetProcessor().HandleVisualizationInput("reset")
				return m, nil

				// Let “enter” or other editing keys fall through to normal input:
			}
		}

		switch msg.Type {
		case tea.KeyCtrlC:
			// If loading or analyzing can be canceled, do so:
			if m.loadingState.IsLoading && m.loadingState.CanCancel {
				if m.commander.GetProcessor() != nil {
					m.commander.GetProcessor().CancelProcessing()
				}
				m.loadingState.IsLoading = false
				m.mainOutput = "Operation cancelled."
				return m, nil
			}
			// If we’re already prompting to exit, this time we really quit:
			if m.exitPrompt {
				return m, tea.Quit
			}
			if m.uiMode == ModeViz {
				m.uiMode = ModeFull
				return m, nil
			}
			// If track loaded, user might want to unload on ctrl+c. Or simply prompt to exit:
			if m.commander.IsInTrackMode() {
				output, err, cmd := m.commander.Execute("unload")
				if err != nil {
					m.mainOutput = fmt.Sprintf("Error: %v", err)
				} else {
					m.mainOutput = output
				}
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
				return m, tea.Batch(cmds...)
			}
			// Normal “prompt to exit”:
			m.exitPrompt = true
			m.mainOutput = "Press Ctrl+C again to exit or any other key to continue..."
			return m, nil

		case tea.KeyUp:
			if msg.Alt {
				// Example: alt+↑ => volume-up
				output, err, cmd := m.commander.Execute("volume-up")
				if err != nil {
					m.mainOutput = fmt.Sprintf("Error: %v", err)
				} else if output != "" {
					m.mainOutput = output
				}
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
			} else {
				// Command history up
				if m.historyPos < len(m.history)-1 {
					m.historyPos++
					m.input.SetValue(m.history[len(m.history)-1-m.historyPos])
				}
			}

		case tea.KeyDown:
			if msg.Alt {
				// Example: alt+↓ => volume-down
				output, err, cmd := m.commander.Execute("volume-down")
				if err != nil {
					m.mainOutput = fmt.Sprintf("Error: %v", err)
				} else if output != "" {
					m.mainOutput = output
				}
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
			} else {
				// Command history down
				if m.historyPos > 0 {
					m.historyPos--
					m.input.SetValue(m.history[len(m.history)-1-m.historyPos])
				} else if m.historyPos == 0 {
					m.historyPos = -1
					m.input.SetValue("")
				}
			}

		case tea.KeyCtrlR:
			// Example: open a “search in history” mode (not fully implemented)
			if !m.searchMode {
				m.searchMode = true
				m.searchQuery = ""
				m.input.SetValue("")
				m.input.Placeholder = "Search history..."
			}

		case tea.KeyTab:
			// Tab completion (only if not in “search mode”):
			if m.searchMode {
				return m, nil
			}
			m.handleTabCompletion()

		case tea.KeyEnter:
			m.exitPrompt = false
			m.mainOutput = strings.TrimSuffix(m.mainOutput, m.tabOutput)
			command := m.input.Value()

			if command != "" {
				// Search-mode logic:
				if m.searchMode {
					m.searchMode = false
					m.input.Placeholder = "Enter command (type 'help' for list)"
					for i := len(m.history) - 1; i >= 0; i-- {
						if strings.Contains(m.history[i], command) {
							m.input.SetValue(m.history[i])
							break
						}
					}
				} else {
					// Normal command execution:
					if m.uiMode == ModeViz {
						// Let user type “q” or “help” from within Viz mode:
						switch command {
						case "q", "quit", "exit":
							m.uiMode = ModeFull
							m.input.SetValue("")
							return m, nil
						case "help", "h", "?":
							m.mainOutput = m.showVisualizationShortcuts()
							m.input.SetValue("")
							return m, nil
						}
					}

					// If user typed an HTTP/HTTPS address => example streaming
					if strings.HasPrefix(command, "http://") || strings.HasPrefix(command, "https://") {
						return m, m.handleStreamStart(command)
					}

					output, err, cmd := m.commander.Execute(command)

					// If “analysis in progress” or “not complete,” do NOT override spinner with an error:
					if err != nil {
						errStr := err.Error()
						if strings.Contains(errStr, "analysis in progress") ||
							strings.Contains(errStr, "analysis not complete") {
							// skip error display; spinner will continue
						} else {
							m.mainOutput = fmt.Sprintf("Error: %v", err)
						}
					} else {
						m.mainOutput = output
						// If user typed “viz wave” or just “viz spectrum,” we may switch to Viz mode:
						if strings.HasPrefix(command, "viz ") || command == "viz" {
							m.uiMode = ModeViz
						}
					}
					if cmd != nil {
						cmds = append(cmds, cmd)
					}

					m.history = append(m.history, command)
					m.historyPos = -1
					m.clearTabCompletion()
					m.input.SetValue("")
				}
			}

		case tea.KeyRunes:
			// “?” => show shortcuts
			if msg.Runes[0] == '?' {
				if m.uiMode == ModeViz {
					m.mainOutput = m.showVisualizationShortcuts()
				} else {
					m.mainOutput = m.showShortcuts()
				}
			}

		case tea.KeyEsc:
			if m.searchMode {
				m.searchMode = false
				m.input.Placeholder = "Enter command (type 'help' for list)"
				m.input.SetValue("")
			}
			if m.uiMode == ModeViz {
				m.uiMode = ModeFull
				return m, nil
			}
			m.clearTabCompletion()
			m.exitPrompt = false

		case tea.KeyBackspace:
			if len(m.input.Value()) == 0 {
				m.clearTabCompletion()
			}

		default:
			// Check if it’s a recognized “shortcut” we mapped (ctrl+p, etc.):
			output, err, shortcutCmd := m.handleShortcut(msg.String())
			if err != nil {
				m.mainOutput = fmt.Sprintf("Error: %v", err)
			} else if output != "" {
				m.mainOutput = output
			}
			if shortcutCmd != nil {
				cmds = append(cmds, shortcutCmd)
			}
			m.exitPrompt = false
			if m.searchMode {
				m.searchQuery = m.input.Value()
			}
		}

	//----------------------------------------------------------------------
	// Spinner (we always keep it ticking if needed)
	//----------------------------------------------------------------------
	case spinner.TickMsg:
		// Only update spinner if we're actually loading
		if m.loadingState.IsLoading {
			var cmd tea.Cmd
			newSpinner, cmd := m.spinner.Update(msg)
			m.spinner = newSpinner
			return m, cmd
		}

	//----------------------------------------------------------------------
	// Resize events
	//----------------------------------------------------------------------
	case tea.WindowSizeMsg:
		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-3)
			m.ready = true
		}
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 3
		m.progress.Width = msg.Width - 20

		// If in Viz mode, pass resize to processor’s visualization manager
		if m.uiMode == ModeViz && m.commander.IsInTrackMode() {
			resizeStr := fmt.Sprintf("resize:%dx%d", msg.Width, msg.Height-6)
			m.commander.GetProcessor().HandleVisualizationInput(resizeStr)
		}
	}

	if m.commander != nil && m.commander.GetProcessor() != nil {
		pStatus := m.commander.GetProcessor().GetStatus()
		switch pStatus.State {
		case audio.StateIdle:
			if m.loadingState.IsLoading {
				// Just finished loading - clear loading state and show metadata
				m.loadingState.IsLoading = false
				m.loadingState.Message = ""
				m.loadingState.Progress = 0
				m.loadingState.CanCancel = false

				// Get and display metadata
				if meta := m.commander.GetProcessor().GetMetadata(); meta != nil {
					m.mainOutput = meta.String()
				}
			}
		case audio.StateLoading:
			m.loadingState.IsLoading = true
			m.loadingState.Message = pStatus.Message
			m.loadingState.Progress = pStatus.Progress
			m.loadingState.StartTime = pStatus.StartTime
			m.loadingState.BytesLoaded = pStatus.BytesLoaded
			m.loadingState.FileSize = pStatus.TotalBytes
			m.loadingState.CanCancel = pStatus.CanCancel
		}
	}

	// Periodically check the Processor status => update loadingState => TUI can show spinner/progress
	pStatus := m.commander.GetProcessor().GetStatus()
	m.syncLoadingStateFromProcessor(pStatus)

	// Update the input field
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)

	// If ready, update the viewport
	if m.ready {
		newViewport, viewportCmd := m.viewport.Update(msg)
		m.viewport = newViewport
		cmds = append(cmds, viewportCmd)
	}

	return m, tea.Batch(cmds...)
}

// syncLoadingStateFromProcessor updates our TUI loadingState from the Processor’s status
func (m *AudioModel) syncLoadingStateFromProcessor(st audio.ProcessingStatus) {
	switch st.State {
	case audio.StateIdle:
		m.loadingState.IsLoading = false
		m.loadingState.Message = ""
		m.loadingState.Progress = 0
		m.loadingState.CanCancel = false
		m.loadingState.BytesLoaded = 0
		m.loadingState.FileSize = 0

	case audio.StateLoading:
		m.loadingState.IsLoading = true
		m.loadingState.CanCancel = st.CanCancel
		m.loadingState.Message = st.Message
		m.loadingState.StartTime = st.StartTime

		// Only update progress if we have valid data
		if st.TotalBytes > 0 {
			m.loadingState.UpdateProgress(st.BytesLoaded, st.TotalBytes)
		} else {
			// Reset progress tracking for unknown size files
			m.loadingState.BytesLoaded = st.BytesLoaded
			m.loadingState.FileSize = 0
			m.loadingState.Progress = 0
		}

	case audio.StateAnalyzing:
		m.loadingState.IsLoading = true
		m.loadingState.CanCancel = st.CanCancel
		m.loadingState.Message = st.Message
		m.loadingState.StartTime = st.StartTime
		m.loadingState.Progress = st.Progress
		// Reset byte tracking during analysis
		m.loadingState.BytesLoaded = 0
		m.loadingState.FileSize = 0
	}
}

// updateStreamProgress is an example “streaming” progress mechanism, left for reference.
func (m AudioModel) updateStreamProgress(msg streamMsg) (AudioModel, tea.Cmd) {
	if msg.error != nil {
		// Streaming error
		m.loadingState.IsLoading = false
		m.mainOutput = fmt.Sprintf("Streaming error: %v", msg.error)
		return m, nil
	}

	m.loadingState.IsLoading = true
	m.loadingState.Message = fmt.Sprintf("Streaming from %s... %.0f%%", msg.url, msg.progress*100)
	m.loadingState.Progress = msg.progress

	if msg.progress >= 1.0 {
		// Done streaming
		m.loadingState.IsLoading = false
		m.mainOutput = "Streaming completed."
		return m, nil
	}

	// Otherwise, continue incrementing progress
	return m, tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		nextProg := msg.progress + 0.1
		if nextProg > 1.0 {
			nextProg = 1.0
		}
		return streamMsg{url: msg.url, progress: nextProg}
	})
}
