package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cvidmar/restiverse/internal/config"
	"github.com/cvidmar/restiverse/internal/files"
	"github.com/cvidmar/restiverse/internal/shellquote"
)

// executeExternalAction executes an external tool/command
func (m model) executeExternalAction(action *config.Action) (model, tea.Cmd) {
	// Get the file paths based on current view
	var filePaths []string

	// Resolve the view the action applies to. previousView is only meaningful
	// while a modal is open, so it must not be used as a general fallback
	contextView := m.currentView
	if contextView == ViewActionModal {
		contextView = m.previousView
	}

	switch contextView {
	case ViewHistory:
		filePaths = m.getSelectedResponsePathsForAction(action)
	case ViewFileBrowser:
		filePaths = m.getSelectedFilePaths()
	}

	if len(filePaths) == 0 {
		m.errorMessage = "No files selected"
		return m, nil
	}

	// Build the command with placeholder substitution
	cmdString, err := action.BuildCommand(filePaths)
	if err != nil {
		m.errorMessage = err.Error()
		return m, nil
	}

	// Close the modal if we're in one
	if m.currentView == ViewActionModal {
		m.currentView = m.previousView
	}

	// Execute the external command
	return m, m.executeExternalCmd(cmdString)
}

// executeExternalCmd returns a command that executes an external tool
func (m model) executeExternalCmd(cmdString string) tea.Cmd {
	return tea.ExecProcess(exec.Command("sh", "-c", cmdString), func(err error) tea.Msg {
		if err != nil {
			return externalToolErrorMsg{err}
		}
		return externalToolCompleteMsg{}
	})
}

// promptCustomCommand shows the input modal for a one-off shell command,
// capturing the files it applies to while the current selection is still known
func (m model) promptCustomCommand() (model, tea.Cmd) {
	var filePaths []string

	switch m.currentView {
	case ViewHistory:
		// Custom commands operate on the response body, like a single-file action
		bodyOnly := config.Action{FileTypes: []string{"body"}}
		filePaths = m.getSelectedResponsePathsForAction(&bodyOnly)
	case ViewFileBrowser:
		filePaths = m.getSelectedFilePaths()
	}

	if len(filePaths) == 0 {
		m.errorMessage = "No files selected"
		return m, nil
	}

	m.customCmdPaths = filePaths
	m.previousView = m.currentView
	m.currentView = ViewInputModal
	m.inputMode = "custom-command"
	m.inputTitle = "Custom Command"
	m.inputField.SetValue("")
	m.inputField.Placeholder = "jq . {filename} | fx"
	m.inputField.Focus()
	return m, nil
}

// runCustomCommand runs a one-off shell command against the files captured when
// the prompt was opened, using the same placeholder substitution as configured actions
func (m model) runCustomCommand(cmdString string) (model, tea.Cmd) {
	m.currentView = m.previousView

	if strings.TrimSpace(cmdString) == "" {
		return m, nil
	}

	if len(m.customCmdPaths) == 0 {
		m.errorMessage = "No files selected"
		return m, nil
	}

	action := config.Action{Command: cmdString}
	builtCmd, err := action.BuildCommand(m.customCmdPaths)
	if err != nil {
		m.errorMessage = err.Error()
		return m, nil
	}

	return m, m.executeExternalCmd(builtCmd)
}

// executeEditorCmd is a helper specifically for opening editors
func (m model) executeEditorCmd(filePath string, success tea.Msg) tea.Cmd {
	return tea.ExecProcess(editorCommand(m.config.Editor, filePath), func(err error) tea.Msg {
		if err != nil {
			return externalToolErrorMsg{fmt.Errorf("editor failed: %w", err)}
		}
		if success != nil {
			return success
		}
		return externalToolCompleteMsg{}
	})
}

func editorCommand(editor, filePath string) *exec.Cmd {
	if editor == "" {
		editor = "vi"
	}
	return exec.Command("sh", "-c", editor+" "+shellquote.Quote(filePath))
}

func resolveNewName(dir, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." {
		return "", fmt.Errorf("invalid filename")
	}
	if strings.ContainsAny(name, `/\`) {
		return "", fmt.Errorf("filename cannot contain path separators")
	}
	return filepath.Join(dir, name), nil
}

// promptForNewHTTPFile shows the input modal for creating a new .http file
func (m model) promptForNewHTTPFile() (model, tea.Cmd) {
	m.previousView = m.currentView
	m.currentView = ViewInputModal
	m.inputMode = "create"
	m.inputTitle = "Create New HTTP File"
	m.inputField.SetValue("")
	m.inputField.Placeholder = "filename.http"
	m.inputField.Focus()
	return m, nil
}

// createHTTPFileWithName creates a new .http file with the given filename
func (m model) createHTTPFileWithName(filename string) (model, tea.Cmd) {
	filename = strings.TrimSpace(filename)
	if _, err := resolveNewName(m.currentPath, filename); err != nil {
		m.errorMessage = err.Error()
		return m, nil
	}

	// Add .http extension if not present
	if !files.IsHTTPFileName(filename) {
		filename = filename + ".http"
	}

	filePath := filepath.Join(m.currentPath, filename)

	// Check if file already exists
	if _, err := os.Stat(filePath); err == nil {
		m.errorMessage = fmt.Sprintf("File '%s' already exists", filename)
		return m, nil
	}

	// Create a template .http file
	template := `GET https://api.example.com/endpoint
Accept: application/json
`

	if err := os.WriteFile(filePath, []byte(template), 0644); err != nil {
		m.errorMessage = fmt.Sprintf("Failed to create file: %v", err)
		return m, nil
	}

	// Return to file browser and open in editor
	m.currentView = ViewFileBrowser
	return m, tea.Batch(
		m.loadDirectoryCmd(),
		m.executeEditorCmd(filePath, externalToolCompleteMsg{}),
	)
}

// promptForRenameFile shows the input modal for renaming a file
func (m model) promptForRenameFile() (model, tea.Cmd) {
	entry, ok := m.currentEntry()
	if !ok {
		return m, nil
	}

	if entry.IsDir {
		m.errorMessage = "Cannot rename directories"
		return m, nil
	}

	m.previousView = m.currentView
	m.currentView = ViewInputModal
	m.inputMode = "rename"
	m.inputTitle = "Rename File"
	m.renameTargetIdx = m.cursor
	m.inputField.SetValue(entry.Name)
	m.inputField.Placeholder = entry.Name
	m.inputField.Focus()
	return m, nil
}

// renameFileWithName renames the file at the target index
func (m model) renameFileWithName(newName string) (model, tea.Cmd) {
	if m.renameTargetIdx < 0 || m.renameTargetIdx >= len(m.fileEntries) {
		m.errorMessage = "Invalid file selection"
		return m, nil
	}

	entry := m.fileEntries[m.renameTargetIdx]
	oldPath := entry.Path
	newPath, err := resolveNewName(m.currentPath, newName)
	if err != nil {
		m.errorMessage = err.Error()
		return m, nil
	}
	newName = filepath.Base(newPath)

	// Check if target already exists
	if _, err := os.Stat(newPath); err == nil && oldPath != newPath {
		m.errorMessage = fmt.Sprintf("File '%s' already exists", newName)
		return m, nil
	}

	// Rename the file
	if err := files.RenameRequestArtifacts(oldPath, newPath); err != nil {
		m.errorMessage = fmt.Sprintf("Failed to rename: %v", err)
		return m, nil
	}

	m.currentView = ViewFileBrowser
	m.statusMessage = fmt.Sprintf("Renamed '%s' to '%s'", entry.Name, newName)
	return m, tea.Batch(m.loadDirectoryCmd(), m.clearStatusAfter(3))
}

// promptDuplicateFile shows the input modal for duplicating a file,
// prefilled with the original name plus a "-copy" suffix
func (m model) promptDuplicateFile() (model, tea.Cmd) {
	entry, ok := m.currentEntry()
	if !ok {
		return m, nil
	}

	if entry.IsDir {
		m.errorMessage = "Cannot duplicate directories"
		return m, nil
	}

	ext := filepath.Ext(entry.Name)
	defaultName := strings.TrimSuffix(entry.Name, ext) + "-copy" + ext

	m.previousView = m.currentView
	m.currentView = ViewInputModal
	m.inputMode = "duplicate"
	m.inputTitle = "Duplicate File"
	m.duplicateTargetIdx = m.cursor
	m.inputField.SetValue(defaultName)
	m.inputField.Placeholder = defaultName
	m.inputField.Focus()
	return m, nil
}

// duplicateFileWithName copies the file at the target index to the given filename
func (m model) duplicateFileWithName(newName string) (model, tea.Cmd) {
	if m.duplicateTargetIdx < 0 || m.duplicateTargetIdx >= len(m.fileEntries) {
		m.errorMessage = "Invalid file selection"
		return m, nil
	}

	entry := m.fileEntries[m.duplicateTargetIdx]
	newPath, err := resolveNewName(m.currentPath, newName)
	if err != nil {
		m.errorMessage = err.Error()
		return m, nil
	}
	newName = filepath.Base(newPath)

	// Check if target already exists
	if _, err := os.Stat(newPath); err == nil {
		m.errorMessage = fmt.Sprintf("File '%s' already exists", newName)
		return m, nil
	}

	content, err := os.ReadFile(entry.Path)
	if err != nil {
		m.errorMessage = fmt.Sprintf("Failed to read '%s': %v", entry.Name, err)
		return m, nil
	}

	if err := os.WriteFile(newPath, content, 0644); err != nil {
		m.errorMessage = fmt.Sprintf("Failed to duplicate: %v", err)
		return m, nil
	}
	if err := files.CopyVariableSidecar(entry.Path, newPath); err != nil {
		if removeErr := os.Remove(newPath); removeErr != nil {
			m.errorMessage = fmt.Sprintf("Failed to copy saved variables: %v (also failed to remove duplicate: %v)", err, removeErr)
		} else {
			m.errorMessage = fmt.Sprintf("Failed to copy saved variables: %v", err)
		}
		return m, nil
	}

	m.currentView = ViewFileBrowser
	m.statusMessage = fmt.Sprintf("Duplicated '%s' to '%s'", entry.Name, newName)
	return m, tea.Batch(m.loadDirectoryCmd(), m.clearStatusAfter(3))
}

// promptDeleteFile shows confirmation modal for deleting a file
func (m model) promptDeleteFile() (model, tea.Cmd) {
	entry, ok := m.currentEntry()
	if !ok {
		return m, nil
	}

	if entry.IsDir {
		m.errorMessage = "Cannot delete directories (use rm -r manually)"
		return m, nil
	}

	m.previousView = m.currentView
	m.currentView = ViewConfirmModal
	m.confirmTitle = "Delete File"
	sidecars, err := files.RequestSidecars(entry.Path)
	if err != nil {
		m.errorMessage = fmt.Sprintf("Failed to inspect related files: %v", err)
		m.currentView = m.previousView
		return m, nil
	}
	m.confirmMessage = fmt.Sprintf("Delete '%s' and %d related file(s)?\nThis cannot be undone.", entry.Name, len(sidecars))
	m.confirmAction = func(model model) (model, tea.Cmd) {
		return model.deleteFile()
	}
	return m, nil
}

// deleteFile deletes the currently selected file
func (m model) deleteFile() (model, tea.Cmd) {
	entry, ok := m.currentEntry()
	if !ok {
		return m, nil
	}

	if err := files.DeleteRequestArtifacts(entry.Path); err != nil {
		m.errorMessage = fmt.Sprintf("Failed to delete: %v", err)
		return m, nil
	}

	m.currentView = ViewFileBrowser
	m.statusMessage = fmt.Sprintf("Deleted '%s'", entry.Name)

	// Adjust cursor if we deleted the last item
	if m.cursor >= len(m.fileEntries)-1 && m.cursor > 0 {
		m.cursor--
	}

	return m, tea.Batch(m.loadDirectoryCmd(), m.clearStatusAfter(3))
}

// clearStatusAfter returns a command that clears status message after the specified seconds
func (m model) clearStatusAfter(seconds int) tea.Cmd {
	return tea.Tick(time.Duration(seconds)*time.Second, func(t time.Time) tea.Msg {
		return clearStatusMsg{}
	})
}

// openConfigFile opens the base config file in the editor
func (m model) openConfigFile() (model, tea.Cmd) {
	configPath := filepath.Join(m.baseDir, "restiverse.yaml")
	if _, err := os.Stat(configPath); err != nil {
		m.errorMessage = fmt.Sprintf("Cannot open config: %v", err)
		return m, nil
	}

	return m, m.executeEditorCmd(configPath, configEditedMsg{})
}
