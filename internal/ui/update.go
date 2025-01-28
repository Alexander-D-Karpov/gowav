package ui

import (
	"fmt"
	"gowav/internal/audio"
	"gowav/internal/commands"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// downloadMsg is used for streaming or downloading progress updates.
type downloadMsg struct {
	url      string
	progress float64
	err      error
}

// progressMsg is for manual progress (rarely used).
type progressMsg float64

// Update is the main TUI update loop, handling user inputs and state changes.
func (m AudioModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	//----------------------------------------------------------------------
	// Our custom ShowFullInfoMsg from commands/info.go
	//----------------------------------------------------------------------
	case commands.ShowFullInfoMsg:
		// The user typed ":info"; we switch to full metadata mode
		m.showFullInfo = true
		if meta := m.commander.GetProcessor().GetMetadata(); meta != nil {
			m.mainOutput = m.BuildMetadataOutput(meta)
		}
		return m, nil

	//----------------------------------------------------------------------
	// downloadMsg: streaming or download progress
	//----------------------------------------------------------------------
	case downloadMsg:
		if msg.err != nil {
			m.loadingState.IsLoading = false
			m.mainOutput = fmt.Sprintf("Download/Stream error: %v", msg.err)
			return m, nil
		}
		m.loadingState.IsLoading = true
		m.loadingState.Message = fmt.Sprintf("Downloading... %.1f%%", msg.progress*100)
		m.loadingState.Progress = msg.progress
		if msg.progress >= 1.0 {
			m.loadingState.IsLoading = false
			m.mainOutput = "Download complete."
		}
		return m, nil

	case progressMsg:
		var _ tea.Cmd
		newProg, c2 := m.progress.Update(float64(msg))
		m.progress = newProg.(progress.Model)
		cmds = append(cmds, c2)
		return m, tea.Batch(cmds...)

	//----------------------------------------------------------------------
	// Key events
	//----------------------------------------------------------------------
	case tea.KeyMsg:
		if m.uiMode == ModeViz && m.commander.IsInTrackMode() {
			// Visualization shortcuts
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
			}
		}

		// Normal keys
		switch msg.Type {
		case tea.KeyCtrlC:
			// Possibly cancel load
			if m.loadingState.IsLoading && m.loadingState.CanCancel {
				m.commander.GetProcessor().CancelProcessing()
				m.loadingState.IsLoading = false
				m.mainOutput = "Operation cancelled."
				return m, nil
			}
			// If we’re already prompting exit, quit
			if m.exitPrompt {
				return m, tea.Quit
			}
			// If in viz mode, exit
			if m.uiMode == ModeViz {
				m.uiMode = ModeFull
				return m, nil
			}
			// If track loaded => unload
			if m.commander.IsInTrackMode() {
				out, err, cmd := m.commander.Execute("unload")
				if err != nil {
					m.mainOutput = fmt.Sprintf("Error: %v", err)
				} else {
					m.mainOutput = out
				}
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
				return m, tea.Batch(cmds...)
			}
			// Otherwise, prompt exit
			m.exitPrompt = true
			m.mainOutput = "Press Ctrl+C again to exit or any other key to continue..."
			return m, nil

		case tea.KeyUp:
			if msg.Alt {
				out, err, cmd := m.commander.Execute("volume-up")
				if err != nil {
					m.mainOutput = fmt.Sprintf("Error: %v", err)
				} else if out != "" {
					m.mainOutput = out
				}
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
			} else {
				if m.historyPos < len(m.history)-1 {
					m.historyPos++
					m.setInputValue(m.history[len(m.history)-1-m.historyPos])
				}
			}

		case tea.KeyDown:
			if msg.Alt {
				out, err, cmd := m.commander.Execute("volume-down")
				if err != nil {
					m.mainOutput = fmt.Sprintf("Error: %v", err)
				} else if out != "" {
					m.mainOutput = out
				}
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
			} else {
				if m.historyPos > 0 {
					m.historyPos--
					m.setInputValue(m.history[len(m.history)-1-m.historyPos])
				} else if m.historyPos == 0 {
					m.historyPos = -1
					m.setInputValue("")
				}
			}

		case tea.KeyCtrlR:
			if !m.searchMode {
				m.searchMode = true
				m.searchQuery = ""
				m.setInputValue("")
				m.setInputPlaceholder("Search history...")
			}

		case tea.KeyTab:
			if !m.searchMode {
				m.handleTabCompletion()
			}

		case tea.KeyEnter:
			m.exitPrompt = false
			m.mainOutput = strings.TrimSuffix(m.mainOutput, m.tabOutput)
			cmdStr := m.getInputValue()
			if cmdStr != "" {
				if m.searchMode {
					m.searchMode = false
					m.setInputPlaceholder("Enter command (type 'help' for list)")
					for i := len(m.history) - 1; i >= 0; i-- {
						if strings.Contains(m.history[i], cmdStr) {
							m.setInputValue(m.history[i])
							break
						}
					}
				} else {
					if m.uiMode == ModeViz {
						switch cmdStr {
						case "q", "quit", "exit":
							m.uiMode = ModeFull
							m.setInputValue("")
							return m, nil
						case "help", "h", "?":
							m.mainOutput = m.showVisualizationShortcuts()
							m.setInputValue("")
							return m, nil
						}
					}

					if strings.HasPrefix(cmdStr, "http://") || strings.HasPrefix(cmdStr, "https://") {
						return m, m.handleStreamStart(cmdStr)
					}

					out, err, c2 := m.commander.Execute(cmdStr)
					if err != nil {
						if !strings.Contains(err.Error(), "analysis in progress") &&
							!strings.Contains(err.Error(), "analysis not complete") {
							m.mainOutput = "Error: " + err.Error()
						}
					} else {
						m.mainOutput = out
						if strings.HasPrefix(cmdStr, "viz ") || cmdStr == "viz" {
							m.uiMode = ModeViz
						}
					}
					if c2 != nil {
						cmds = append(cmds, c2)
					}
					m.history = append(m.history, cmdStr)
					m.historyPos = -1
					m.clearTabCompletion()
					m.setInputValue("")
				}
			}

		case tea.KeyRunes:
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
				m.setInputPlaceholder("Enter command (type 'help' for list)")
				m.setInputValue("")
			}
			if m.uiMode == ModeViz {
				m.uiMode = ModeFull
				return m, nil
			}
			m.clearTabCompletion()
			m.exitPrompt = false

		case tea.KeyBackspace:
			if len(m.getInputValue()) == 0 {
				m.clearTabCompletion()
			}
		default:
			out, err, c2 := m.handleShortcut(msg.String())
			if err != nil {
				m.mainOutput = fmt.Sprintf("Error: %v", err)
			} else if out != "" {
				m.mainOutput = out
			}
			if c2 != nil {
				cmds = append(cmds, c2)
			}
			m.exitPrompt = false
			if m.searchMode {
				m.searchQuery = m.getInputValue()
			}
		}

	//----------------------------------------------------------------------
	// spinner.TickMsg: maintain spinner if loading
	//----------------------------------------------------------------------
	case spinner.TickMsg:
		if m.loadingState.IsLoading {
			var _ tea.Cmd
			newSpin, c2 := m.spinner.Update(msg)
			m.spinner = newSpin
			cmds = append(cmds, c2)
			return m, tea.Batch(cmds...)
		}

	//----------------------------------------------------------------------
	// Window resizing
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

		// If not loading, re-render metadata if present
		if !m.loadingState.IsLoading {
			if meta := m.commander.GetProcessor().GetMetadata(); meta != nil {
				m.mainOutput = m.BuildMetadataOutput(meta)
			}
		}

		if m.uiMode == ModeViz && m.commander.IsInTrackMode() {
			resizeStr := fmt.Sprintf("resize:%dx%d", msg.Width, msg.Height-6)
			m.commander.GetProcessor().HandleVisualizationInput(resizeStr)
		}
	}

	// Check Processor status
	if p := m.commander.GetProcessor(); p != nil {
		st := p.GetStatus()
		switch st.State {
		case audio.StateIdle:
			if m.loadingState.IsLoading {
				m.loadingState.IsLoading = false
				m.loadingState.Message = ""
				m.loadingState.Progress = 0
				m.loadingState.CanCancel = false

				// Show partial after load
				m.showFullInfo = false
				if meta := p.GetMetadata(); meta != nil {
					m.mainOutput = m.BuildMetadataOutput(meta)
				}
			}

		case audio.StateLoading:
			m.loadingState.IsLoading = true
			m.loadingState.Message = st.Message
			m.loadingState.Progress = st.Progress
			m.loadingState.StartTime = st.StartTime
			m.loadingState.BytesLoaded = st.BytesLoaded
			m.loadingState.FileSize = st.TotalBytes
			m.loadingState.CanCancel = st.CanCancel
		}
		m.syncLoadingStateFromProcessor(st)
	}

	// Update input
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)

	// Update viewport
	if m.ready {
		newVP, cmd2 := m.viewport.Update(msg)
		m.viewport = newVP
		cmds = append(cmds, cmd2)
	}

	return m, tea.Batch(cmds...)
}

// syncLoadingStateFromProcessor updates progress or message from processor status.
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
		if st.TotalBytes > 0 {
			m.loadingState.UpdateProgress(st.BytesLoaded, st.TotalBytes)
		} else {
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
		m.loadingState.BytesLoaded = 0
		m.loadingState.FileSize = 0
	}
}

func (m *AudioModel) setInputValue(val string) {
	m.input.SetValue(val)
}

func (m *AudioModel) setInputPlaceholder(ph string) {
	m.input.Placeholder = ph
}

func (m *AudioModel) getInputValue() string {
	return m.input.Value()
}
