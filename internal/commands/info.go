package commands

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
)

// ShowFullInfoMsg is exported so the UI can reference it.
type ShowFullInfoMsg struct{}

// handleInfo triggers the UI to display "full" metadata mode.
func (c *Commander) handleInfo() (string, error, tea.Cmd) {
	meta := c.processor.GetMetadata()
	if meta == nil {
		return "", fmt.Errorf("no track loaded"), nil
	}

	// Instead of returning info, return a message that the UI will interpret.
	return "", nil, func() tea.Msg {
		return ShowFullInfoMsg{}
	}
}
