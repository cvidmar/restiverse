package tui

import (
	"sort"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/cvidmar/restiverse/internal/config"
	"github.com/cvidmar/restiverse/internal/files"
	"github.com/cvidmar/restiverse/internal/http"
	"github.com/cvidmar/restiverse/internal/vars"
)

// showActionsForFile shows the action modal for a .http file
func (m model) showActionsForFile(entry files.FileEntry) (model, tea.Cmd) {
	selected := m.getSelectedFiles()
	selectedCount := len(selected)
	var fileTypes []string

	// If no files are multi-selected, treat current file as the selection
	if selectedCount == 0 {
		selectedCount = 1
		fileTypes = []string{string(files.GetFileType(entry.Name))}
	} else {
		for _, index := range selected {
			if index >= 0 && index < len(m.fileEntries) {
				fileTypes = append(fileTypes, string(files.GetFileType(m.fileEntries[index].Name)))
			}
		}
	}

	applicable := m.config.FilterActions(fileTypes, selectedCount)

	// Add "Variables" pseudo-action if file has variables and config has vars
	if len(m.config.Vars) > 0 && checkFileHasVariables(entry.Path) {
		maxOne := 1
		variablesAction := config.Action{
			Name:       "Variables",
			Command:    "internal:variables",
			Keybinding: "v",
			MinFiles:   1,
			MaxFiles:   &maxOne,
			FileTypes:  []string{"http"},
		}
		applicable = append(applicable, variablesAction)
	}

	if len(applicable) == 0 {
		m.statusMessage = "No actions available"
		return m, nil
	}

	config.SortActionsByName(applicable)

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

	config.SortActionsByName(applicable)

	// Add "Custom command" pseudo-action for one-off shell commands
	applicable = append(applicable, config.Action{
		Name:      "Custom command…",
		Command:   "internal:custom-command",
		MinFiles:  1,
		FileTypes: []string{"body", "meta"},
	})

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
		entry, ok := m.currentEntry()
		if !ok {
			return m, nil
		}
		fileTypes = []string{string(files.GetFileType(entry.Name))}
		selectedCount = len(m.getSelectedFiles())
		if selectedCount == 0 {
			selectedCount = 1
		}
	} else if m.currentView == ViewHistory {
		selectedCount = len(m.getSelectedResponses())
		if selectedCount == 0 {
			selectedCount = 1
		}

		// For history view, check if action supports body OR meta (not both required)
		// This matches the modal menu behavior in showActionsForResponse()
		supportsBody := action.IsApplicable([]string{"body"}, selectedCount)
		supportsMeta := action.IsApplicable([]string{"meta"}, selectedCount)

		if !supportsBody && !supportsMeta {
			m.errorMessage = "Action not applicable"
			return m, nil
		}

		// Optimization: skip the general IsApplicable check since we already checked
		// Execute the action directly
		if action.IsInternal() {
			return m.executeInternalAction(action)
		}
		return m.executeExternalAction(action)
	} else {
		return m, nil
	}

	// Check if action is applicable (for non-history views)
	if !action.IsApplicable(fileTypes, selectedCount) {
		m.errorMessage = "Action not applicable"
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
	case "duplicate":
		return m.promptDuplicateFile()
	case "delete":
		return m.promptDeleteFile()
	case "variables":
		return m.showVariablesForCurrentFile()
	case "copy-as-curl":
		return m.copyAsCurl()
	case "custom-command":
		return m.promptCustomCommand()
	default:
		m.errorMessage = "Unknown internal command: " + cmd
		return m, nil
	}
}

// showVariablesForCurrentFile shows variable selection for the current file
func (m model) showVariablesForCurrentFile() (model, tea.Cmd) {
	if m.currentView != ViewFileBrowser || len(m.fileEntries) == 0 {
		return m, nil
	}

	entry, ok := m.currentEntry()
	if !ok {
		return m, nil
	}
	if !entry.IsHTTP {
		return m, nil
	}

	return m.showVariableSelection(entry)
}

// showHistory shows the history view for the current .http file
func (m model) showHistory() (model, tea.Cmd) {
	if m.currentView != ViewFileBrowser || len(m.fileEntries) == 0 {
		return m, nil
	}

	entry, ok := m.currentEntry()
	if !ok {
		return m, nil
	}
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
	sort.Ints(selected)
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
	sort.Ints(selected)
	return selected
}

// getSelectedFilePaths returns the file paths of selected files
func (m model) getSelectedFilePaths() []string {
	selected := m.getSelectedFiles()
	if len(selected) == 0 {
		// No multi-selection, use current cursor
		if entry, ok := m.currentEntry(); ok {
			return []string{entry.Path}
		}
		return []string{}
	}

	var paths []string
	for _, idx := range selected {
		if idx >= 0 && idx < len(m.fileEntries) {
			paths = append(paths, m.fileEntries[idx].Path)
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
		if resp, ok := m.currentResponse(); ok {
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
		if idx < 0 || idx >= len(m.responses) {
			continue
		}
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

// copyAsCurl copies the current .http file as a curl command to clipboard
func (m model) copyAsCurl() (model, tea.Cmd) {
	if m.currentView != ViewFileBrowser || len(m.fileEntries) == 0 {
		return m, nil
	}

	entry, ok := m.currentEntry()
	if !ok {
		return m, nil
	}
	if !entry.IsHTTP {
		m.errorMessage = "Not an HTTP file"
		return m, nil
	}

	// Parse the HTTP file
	req, err := http.ParseHTTPFile(entry.Path)
	if err != nil {
		m.errorMessage = "Failed to parse HTTP file: " + err.Error()
		return m, nil
	}

	varNames := vars.ExtractRequest(req.URL, req.Headers, req.Body)
	varValues, err := vars.ResolveValues(entry.Path, m.config.Vars, varNames)
	if err != nil {
		m.errorMessage = "Failed to load variables: " + err.Error()
		return m, nil
	}

	if err := vars.SubstituteRequest(&req.URL, req.Headers, &req.Body, varValues); err != nil {
		m.errorMessage = err.Error()
		return m, nil
	}

	// Generate curl command
	curlCmd := req.ToCurlCommand()

	// Copy to clipboard
	if err := clipboard.WriteAll(curlCmd); err != nil {
		m.errorMessage = "Failed to copy to clipboard: " + err.Error()
		return m, nil
	}

	m.statusMessage = "Copied curl command to clipboard"
	return m, m.clearStatusAfter(3)
}
