package ui

import (
	"fmt"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"strings"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case streamMsg:
		newModel, cmd := m.updateStreamProgress(msg)
		return newModel, cmd

	case progressMsg:
		var cmd tea.Cmd
		newProgress, cmd := m.progress.Update(float64(msg))
		m.progress = newProgress.(progress.Model)
		return m, cmd

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			if m.exitPrompt {
				return m, tea.Quit
			}
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
			m.exitPrompt = true
			m.mainOutput = "Press Ctrl+C again to exit or any other key to continue..."
			return m, nil

		case tea.KeyUp:
			if msg.Alt {
				output, err, cmd := m.commander.Execute("volume-up")
				if err != nil {
					m.mainOutput = fmt.Sprintf("Error: %v", err)
				} else {
					m.mainOutput = output
				}
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
			} else {
				if m.historyPos < len(m.history)-1 {
					m.historyPos++
					m.input.SetValue(m.history[len(m.history)-1-m.historyPos])
				}
			}

		case tea.KeyDown:
			if msg.Alt {
				output, err, cmd := m.commander.Execute("volume-down")
				if err != nil {
					m.mainOutput = fmt.Sprintf("Error: %v", err)
				} else {
					m.mainOutput = output
				}
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
			} else {
				if m.historyPos > 0 {
					m.historyPos--
					m.input.SetValue(m.history[len(m.history)-1-m.historyPos])
				} else if m.historyPos == 0 {
					m.historyPos = -1
					m.input.SetValue("")
				}
			}

		case tea.KeyCtrlR:
			if !m.searchMode {
				m.searchMode = true
				m.searchQuery = ""
				m.input.SetValue("")
				m.input.Placeholder = "Search history..."
			}

		case tea.KeyTab:
			if m.searchMode {
				return m, nil
			}
			m.handleTabCompletion()

		case tea.KeyEnter:
			m.exitPrompt = false
			m.mainOutput = strings.TrimSuffix(m.mainOutput, m.tabOutput)
			command := m.input.Value()

			if command != "" {
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
					if strings.HasPrefix(command, "http://") || strings.HasPrefix(command, "https://") {
						return m, m.handleStreamStart(command)
					}

					output, err, cmd := m.commander.Execute(command)
					if err != nil {
						m.mainOutput = fmt.Sprintf("Error: %v", err)
					} else {
						m.mainOutput = output
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
			if msg.Runes[0] == '?' {
				m.mainOutput = m.showShortcuts()
			}

		case tea.KeyEsc:
			if m.searchMode {
				m.searchMode = false
				m.input.Placeholder = "Enter command (type 'help' for list)"
				m.input.SetValue("")
			}
			m.clearTabCompletion()
			m.exitPrompt = false

		case tea.KeyBackspace:
			if len(m.input.Value()) == 0 {
				m.clearTabCompletion()
			}

		default:
			output, err, cmd := m.handleShortcut(msg.String())
			if err != nil {
				m.mainOutput = fmt.Sprintf("Error: %v", err)
			} else if output != "" {
				m.mainOutput = output
			}
			if cmd != nil {
				cmds = append(cmds, cmd)
			}

			m.exitPrompt = false
			if m.searchMode {
				m.searchQuery = m.input.Value()
			}
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		newSpinner, cmd := m.spinner.Update(msg)
		m.spinner = newSpinner
		return m, cmd

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
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)

	newViewport, cmd := m.viewport.Update(msg)
	m.viewport = newViewport
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}
