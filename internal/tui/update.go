package tui

import (
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
)

// Update handles all events and updates the model
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case directoryLoadedMsg:
		// Preserve cursor position if we're reloading the same directory
		oldCursor := m.cursor
		oldEntriesLen := len(m.fileEntries)

		m.fileEntries = msg.entries
		m.cursor = 0
		m.selectedFiles = make(map[int]bool)

		// If we have a target file name (from fuzzy finder), position cursor on it
		if m.targetFileName != "" {
			for i, entry := range m.fileEntries {
				if entry.Name == m.targetFileName {
					m.cursor = i
					break
				}
			}
			m.targetFileName = "" // Clear the target
		} else if oldEntriesLen > 0 && oldCursor < len(m.fileEntries) {
			// Preserve cursor position when reloading (e.g., after editing)
			m.cursor = oldCursor
		}
		return m, nil

	case navigatedToDirMsg:
		m.currentPath = msg.path
		m.fileEntries = msg.entries
		m.cursor = 0
		m.selectedFiles = make(map[int]bool)
		m.errorMessage = ""

		// If we have a target file name (from fuzzy finder), position cursor on it
		if m.targetFileName != "" {
			for i, entry := range m.fileEntries {
				if entry.Name == m.targetFileName {
					m.cursor = i
					break
				}
			}
			m.targetFileName = "" // Clear the target
		}
		return m, nil

	case historyLoadedMsg:
		// Preserve cursor position if we're reloading the same file
		oldHistoryCursor := m.historyCursor
		oldResponsesLen := len(m.responses)
		isReload := m.currentHTTPFile == msg.httpFile

		m.currentHTTPFile = msg.httpFile
		m.responses = msg.responses
		m.historyCursor = 0
		m.selectedResponses = make(map[int]bool)
		m.currentView = ViewHistory

		// Preserve cursor position when reloading (e.g., after executing a request or editing)
		if isReload && oldResponsesLen > 0 && oldHistoryCursor < len(m.responses) {
			m.historyCursor = oldHistoryCursor
		}
		return m, nil

	case errMsg:
		m.errorMessage = msg.err.Error()
		return m, nil

	case requestCompleteMsg:
		m.requestRunning = false
		m.statusMessage = msg.duration
		// Reload history view for the current HTTP file
		return m, m.loadHistoryCmd(m.currentHTTPFile)

	case requestErrorMsg:
		m.requestRunning = false
		m.errorMessage = msg.err.Error()
		return m, nil

	case externalToolCompleteMsg:
		m.statusMessage = ""
		// Refresh current view
		if m.currentView == ViewFileBrowser {
			return m, m.loadDirectoryCmd()
		}
		return m, nil

	case externalToolErrorMsg:
		m.errorMessage = msg.err.Error()
		return m, nil

	case searchResultsMsg:
		return m.handleSearchResults(msg)

	case clearStatusMsg:
		m.statusMessage = ""
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyPress(msg)
	}

	return m, nil
}

// handleKeyPress routes key presses to the appropriate handler based on current view
func (m model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Dismiss any status message so the help hints come back. Handlers below can
	// still set a new one for the action they perform
	m.statusMessage = ""

	// Global keybindings. Ctrl+C quits immediately, 'q' asks for confirmation
	if msg.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}

	if matches(msg, m.keys.Quit) && m.quitKeyActive() {
		return m.promptQuit()
	}

	if matches(msg, m.keys.Cancel) {
		// Clear error on escape
		m.errorMessage = ""

		// Cancel request if running
		if m.requestRunning && m.cancelFunc != nil {
			m.cancelFunc()
			m.requestRunning = false
			m.statusMessage = "Request cancelled"
			return m, nil
		}

		// Close modals
		if m.currentView == ViewActionModal || m.currentView == ViewFuzzyFinder || m.currentView == ViewInputModal || m.currentView == ViewConfirmModal || m.currentView == ViewVariableSelect {
			m.currentView = m.previousView
			return m, nil
		}
	}

	if matches(msg, m.keys.FuzzyFind) && m.currentView != ViewFuzzyFinder {
		m.previousView = m.currentView
		m.currentView = ViewFuzzyFinder
		m.searchInput.SetValue("")
		return m, m.searchHTTPFilesCmd("")
	}

	// View-specific handling
	switch m.currentView {
	case ViewFileBrowser:
		return m.updateFileBrowser(msg)
	case ViewHistory:
		return m.updateHistory(msg)
	case ViewActionModal:
		return m.updateActionModal(msg)
	case ViewFuzzyFinder:
		return m.updateFuzzyFinder(msg)
	case ViewInputModal:
		return m.updateInputModal(msg)
	case ViewConfirmModal:
		return m.updateConfirmModal(msg)
	case ViewVariableSelect:
		return m.updateVariableSelect(msg)
	}

	return m, nil
}

// quitKeyActive reports whether the 'q' shortcut applies to the current view.
// Views with a focused text input consume printable keys, and the confirm modal
// already owns the confirmTitle/confirmMessage/confirmAction state
func (m model) quitKeyActive() bool {
	switch m.currentView {
	case ViewFuzzyFinder, ViewInputModal, ViewConfirmModal:
		return false
	}
	return true
}

// promptQuit shows the confirmation modal for quitting
func (m model) promptQuit() (model, tea.Cmd) {
	m.previousView = m.currentView
	m.currentView = ViewConfirmModal
	m.confirmTitle = "Quit"
	m.confirmMessage = "Quit Restiverse?"
	m.confirmAction = func(model model) (model, tea.Cmd) {
		return model, tea.Quit
	}
	return m, nil
}

// updateFileBrowser handles key presses in the file browser view
func (m model) updateFileBrowser(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Clear error on any key press
	m.errorMessage = ""

	// Navigation
	if matches(msg, m.keys.Up) {
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	}

	if matches(msg, m.keys.Down) {
		if m.cursor < len(m.fileEntries)-1 {
			m.cursor++
		}
		return m, nil
	}

	// Multi-selection
	if matches(msg, m.keys.Space) {
		if len(m.fileEntries) > 0 {
			m.selectedFiles[m.cursor] = !m.selectedFiles[m.cursor]
		}
		return m, nil
	}

	// Enter: navigate or show actions
	if matches(msg, m.keys.Enter) {
		if len(m.fileEntries) == 0 {
			return m, nil
		}

		entry := m.fileEntries[m.cursor]
		if entry.IsDir {
			// Navigate into directory
			return m, m.navigateToDir(entry.Path)
		} else if entry.IsHTTP {
			// Show action modal
			return m.showActionsForFile(entry)
		}
		return m, nil
	}

	// Back: navigate to parent directory
	if matches(msg, m.keys.Back) {
		parentDir := filepath.Dir(m.currentPath)
		// Don't navigate above base directory
		if len(parentDir) >= len(m.baseDir) && parentDir != m.currentPath {
			return m, m.navigateToDir(parentDir)
		}
		return m, nil
	}

	// Special keys
	switch msg.String() {
	case "n":
		return m.promptForNewHTTPFile()
	case "R": // Shift+R for rename
		return m.promptForRenameFile()
	case "D": // Shift+D for duplicate
		return m.promptDuplicateFile()
	case "X": // Shift+X for delete
		return m.promptDeleteFile()
	case "c":
		return m.openConfigFile()
	case "v": // Variables configuration
		if len(m.fileEntries) > 0 {
			entry := m.fileEntries[m.cursor]
			if entry.IsHTTP && len(m.config.Vars) > 0 && checkFileHasVariables(entry.Path) {
				return m.showVariableSelection(entry)
			}
		}
	}

	// Check for action keybindings
	if action := m.config.GetActionByKeybinding(msg.String()); action != nil {
		return m.executeActionKeybinding(action)
	}

	return m, nil
}

// updateHistory handles key presses in the history view
func (m model) updateHistory(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Clear error on any key press
	m.errorMessage = ""

	// Navigation
	if matches(msg, m.keys.Up) {
		if m.historyCursor > 0 {
			m.historyCursor--
		}
		return m, nil
	}

	if matches(msg, m.keys.Down) {
		if m.historyCursor < len(m.responses)-1 {
			m.historyCursor++
		}
		return m, nil
	}

	// Multi-selection
	if matches(msg, m.keys.Space) {
		if len(m.responses) > 0 {
			m.selectedResponses[m.historyCursor] = !m.selectedResponses[m.historyCursor]
		}
		return m, nil
	}

	// Enter: show actions for response
	if matches(msg, m.keys.Enter) {
		if len(m.responses) > 0 {
			return m.showActionsForResponse()
		}
		return m, nil
	}

	// Back: return to file browser
	if matches(msg, m.keys.Back) {
		m.currentView = ViewFileBrowser
		return m, nil
	}

	// Check for action keybindings
	if action := m.config.GetActionByKeybinding(msg.String()); action != nil {
		return m.executeActionKeybinding(action)
	}

	return m, nil
}

// updateActionModal handles key presses in the action modal
func (m model) updateActionModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Clear error on any key press
	m.errorMessage = ""

	if matches(msg, m.keys.Up) {
		if m.modalCursor > 0 {
			m.modalCursor--
		}
		return m, nil
	}

	if matches(msg, m.keys.Down) {
		if m.modalCursor < len(m.actions)-1 {
			m.modalCursor++
		}
		return m, nil
	}

	if matches(msg, m.keys.Enter) {
		if len(m.actions) > 0 {
			action := m.actions[m.modalCursor]
			return m.executeAction(&action)
		}
		return m, nil
	}

	return m, nil
}

// updateFuzzyFinder handles key presses in the fuzzy finder
func (m model) updateFuzzyFinder(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Clear error on any key press
	m.errorMessage = ""

	// Handle navigation in results (arrow keys only, letters go to the search input)
	if msg.Type == tea.KeyUp {
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	}

	if msg.Type == tea.KeyDown {
		if m.cursor < len(m.searchResults)-1 {
			m.cursor++
		}
		return m, nil
	}

	if matches(msg, m.keys.Enter) {
		// Navigate to selected file
		if len(m.searchResults) > 0 && m.cursor < len(m.searchResults) {
			selectedFile := m.searchResults[m.cursor]
			m.currentView = ViewFileBrowser // Always go to file browser when selecting a file
			m.currentPath = filepath.Dir(selectedFile.Path)
			m.targetFileName = filepath.Base(selectedFile.Path) // Remember which file to highlight
			return m, m.loadDirectoryCmd()
		}
		return m, nil
	}

	// Update search input
	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)

	// Trigger search
	query := m.searchInput.Value()
	return m, tea.Batch(cmd, m.searchHTTPFilesCmd(query))
}

// updateInputModal handles key presses in the input modal
func (m model) updateInputModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if matches(msg, m.keys.Enter) {
		value := m.inputField.Value()
		m.inputField.Blur()

		switch m.inputMode {
		case "create":
			return m.createHTTPFileWithName(value)
		case "rename":
			return m.renameFileWithName(value)
		case "duplicate":
			return m.duplicateFileWithName(value)
		}

		m.currentView = m.previousView
		return m, nil
	}

	// Update input field
	var cmd tea.Cmd
	m.inputField, cmd = m.inputField.Update(msg)
	return m, cmd
}

// updateConfirmModal handles key presses in the confirm modal
func (m model) updateConfirmModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		// Execute the confirmed action
		if m.confirmAction != nil {
			return m.confirmAction(m)
		}
		m.currentView = m.previousView
		return m, nil

	case "n", "N":
		// Cancel
		m.currentView = m.previousView
		return m, nil
	}

	return m, nil
}
