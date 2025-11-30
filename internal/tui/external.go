package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/cvidmar/restiverse/internal/config"
)

// executeExternalAction executes an external tool/command
func (m model) executeExternalAction(action *config.Action) (model, tea.Cmd) {
	// This will be implemented in Phase 5
	m.statusMessage = "External tool execution not yet implemented: " + action.Name
	return m, nil
}
