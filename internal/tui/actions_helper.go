package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/cvidmar/restiverse/internal/config"
	"github.com/cvidmar/restiverse/internal/files"
)

// showActionsForFile shows the action modal for a .http file
func (m model) showActionsForFile(entry files.FileEntry) (model, tea.Cmd) {
	fileTypes := []string{string(files.GetFileType(entry.Name))}
	selectedCount := len(m.getSelectedFiles())

	// If no files are multi-selected, treat current file as the selection
	if selectedCount == 0 {
		selectedCount = 1
	}

	applicable := m.config.FilterActions(fileTypes, selectedCount)
	if len(applicable) == 0 {
		m.statusMessage = "No actions available"
		return m, nil
	}

	m.previousView = m.currentView
	m.currentView = ViewActionModal
	m.actions = applicable
	m.modalCursor = 0
	m.modalTitle = "Actions for: " + entry.Name

	return m, nil
}

// showActionsForResponse shows the action modal for response file(s)
func (m model) showActionsForResponse() (model, tea.Cmd) {
	selected := m.getSelectedResponses()
	if len(selected) == 0 {
		selected = []int{m.historyCursor}
	}

	// Get all applicable actions (for both body and meta files)
	// We need to gather actions that work with either file type
	var applicable []config.Action
	actionMap := make(map[string]config.Action) // Use map to avoid duplicates

	// Check for body file actions
	bodyActions := m.config.FilterActions([]string{"body"}, len(selected))
	for _, action := range bodyActions {
		actionMap[action.Name] = action
	}

	// Check for meta file actions
	metaActions := m.config.FilterActions([]string{"meta"}, len(selected))
	for _, action := range metaActions {
		actionMap[action.Name] = action
	}

	// Convert map back to slice
	for _, action := range actionMap {
		applicable = append(applicable, action)
	}

	if len(applicable) == 0 {
		m.statusMessage = "No actions available"
		return m, nil
	}

	m.previousView = m.currentView
	m.currentView = ViewActionModal
	m.actions = applicable
	m.modalCursor = 0

	if len(selected) == 1 {
		m.modalTitle = "Actions for response"
	} else {
		m.modalTitle = "Actions for responses"
	}

	return m, nil
}

// executeAction executes the selected action
func (m model) executeAction(action *config.Action) (model, tea.Cmd) {
	// Close modal
	m.currentView = m.previousView

	// Handle internal commands
	if action.IsInternal() {
		return m.executeInternalAction(action)
	}

	// Execute external command
	return m.executeExternalAction(action)
}

// executeActionKeybinding executes an action by its keybinding
func (m model) executeActionKeybinding(action *config.Action) (model, tea.Cmd) {
	// Determine file types and selection based on current view
	var fileTypes []string
	var selectedCount int

	if m.currentView == ViewFileBrowser {
		if len(m.fileEntries) == 0 {
			return m, nil
		}
		entry := m.fileEntries[m.cursor]
		fileTypes = []string{string(files.GetFileType(entry.Name))}
		selectedCount = len(m.getSelectedFiles())
		if selectedCount == 0 {
			selectedCount = 1
		}
	} else if m.currentView == ViewHistory {
		fileTypes = []string{"body", "meta"}
		selectedCount = len(m.getSelectedResponses())
		if selectedCount == 0 {
			selectedCount = 1
		}
	} else {
		return m, nil
	}

	// Check if action is applicable
	if !action.IsApplicable(fileTypes, selectedCount) {
		m.statusMessage = "Action not applicable"
		return m, nil
	}

	// Handle internal commands
	if action.IsInternal() {
		return m.executeInternalAction(action)
	}

	// Execute external command
	return m.executeExternalAction(action)
}

// executeInternalAction handles internal actions
func (m model) executeInternalAction(action *config.Action) (model, tea.Cmd) {
	cmd := action.GetInternalCommand()

	switch cmd {
	case "execute":
		return m.executeHTTPRequest()
	case "history":
		return m.showHistory()
	case "rename":
		return m.promptForRenameFile()
	case "delete":
		return m.promptDeleteFile()
	default:
		m.errorMessage = "Unknown internal command: " + cmd
		return m, nil
	}
}

// showHistory shows the history view for the current .http file
func (m model) showHistory() (model, tea.Cmd) {
	if m.currentView != ViewFileBrowser || len(m.fileEntries) == 0 {
		return m, nil
	}

	entry := m.fileEntries[m.cursor]
	if !entry.IsHTTP {
		return m, nil
	}

	return m, m.loadHistoryCmd(entry.Path)
}

// loadHistoryCmd returns a command that loads the history for an HTTP file
func (m model) loadHistoryCmd(httpFile string) tea.Cmd {
	return func() tea.Msg {
		responses, err := files.ListResponseHistory(httpFile)
		if err != nil {
			return errMsg{err}
		}
		return historyLoadedMsg{httpFile, responses}
	}
}

// getSelectedFiles returns the indices of selected files
func (m model) getSelectedFiles() []int {
	var selected []int
	for i := range m.selectedFiles {
		if m.selectedFiles[i] {
			selected = append(selected, i)
		}
	}
	return selected
}

// getSelectedResponses returns the indices of selected responses
func (m model) getSelectedResponses() []int {
	var selected []int
	for i := range m.selectedResponses {
		if m.selectedResponses[i] {
			selected = append(selected, i)
		}
	}
	return selected
}

// getSelectedFilePaths returns the file paths of selected files
func (m model) getSelectedFilePaths() []string {
	selected := m.getSelectedFiles()
	if len(selected) == 0 {
		// No multi-selection, use current cursor
		if len(m.fileEntries) > 0 {
			return []string{m.fileEntries[m.cursor].Path}
		}
		return []string{}
	}

	paths := make([]string, len(selected))
	for i, idx := range selected {
		paths[i] = m.fileEntries[idx].Path
	}
	return paths
}

// getSelectedResponsePaths returns the file paths of selected responses
func (m model) getSelectedResponsePaths() []string {
	selected := m.getSelectedResponses()
	if len(selected) == 0 {
		// No multi-selection, use current cursor
		if len(m.responses) > 0 {
			resp := m.responses[m.historyCursor]
			return []string{resp.BodyPath, resp.MetaPath}
		}
		return []string{}
	}

	var paths []string
	for _, idx := range selected {
		resp := m.responses[idx]
		if resp.BodyPath != "" {
			paths = append(paths, resp.BodyPath)
		}
		if resp.MetaPath != "" {
			paths = append(paths, resp.MetaPath)
		}
	}
	return paths
}

// getSelectedResponsePathsForAction returns the file paths filtered by action's supported file types
func (m model) getSelectedResponsePathsForAction(action *config.Action) []string {
	selected := m.getSelectedResponses()

	// For single-file actions (max_files = 1), prioritize body over meta
	isSingleFileAction := action.MaxFiles != nil && *action.MaxFiles == 1

	if len(selected) == 0 {
		// No multi-selection, use current cursor
		if len(m.responses) > 0 {
			resp := m.responses[m.historyCursor]
			var paths []string

			// For single-file actions, return only one file (prefer body over meta)
			if isSingleFileAction {
				if action.SupportsFileType("body") && resp.BodyPath != "" {
					return []string{resp.BodyPath}
				}
				if action.SupportsFileType("meta") && resp.MetaPath != "" {
					return []string{resp.MetaPath}
				}
				return []string{}
			}

			// For multi-file actions, include all supported file types
			if action.SupportsFileType("body") && resp.BodyPath != "" {
				paths = append(paths, resp.BodyPath)
			}
			if action.SupportsFileType("meta") && resp.MetaPath != "" {
				paths = append(paths, resp.MetaPath)
			}

			return paths
		}
		return []string{}
	}

	var paths []string
	for _, idx := range selected {
		resp := m.responses[idx]

		// For single-file actions with multiple selections, only include body (or meta if no body)
		if isSingleFileAction {
			if action.SupportsFileType("body") && resp.BodyPath != "" {
				paths = append(paths, resp.BodyPath)
			} else if action.SupportsFileType("meta") && resp.MetaPath != "" {
				paths = append(paths, resp.MetaPath)
			}
		} else {
			// For multi-file actions, include all supported file types
			if action.SupportsFileType("body") && resp.BodyPath != "" {
				paths = append(paths, resp.BodyPath)
			}
			if action.SupportsFileType("meta") && resp.MetaPath != "" {
				paths = append(paths, resp.MetaPath)
			}
		}
	}
	return paths
}
