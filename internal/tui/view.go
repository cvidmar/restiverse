package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/cvidmar/restiverse/internal/config"
	"github.com/cvidmar/restiverse/internal/files"
	"github.com/rivo/uniseg"
)

// View renders the UI based on current model state
func (m model) View() string {
	if m.width == 0 {
		return "Initializing..."
	}

	var content string

	// Render based on current view
	switch m.currentView {
	case ViewFileBrowser:
		content = m.renderFileBrowser()
	case ViewHistory:
		content = m.renderHistory()
	case ViewActionModal:
		content = m.renderActionModal()
	case ViewFuzzyFinder:
		content = m.renderFuzzyFinder()
	case ViewInputModal:
		content = m.renderInputModal()
	case ViewConfirmModal:
		content = m.renderConfirmModal()
	case ViewVariableSelect:
		content = m.renderVariableSelect()
	}

	// Build the full UI
	topBar := m.renderTopBar()
	statusBar := m.renderStatusBar()

	// Calculate content height (reserve space for top and status bars)
	contentHeight := m.height - 2
	if contentHeight < 5 {
		contentHeight = 5
	}

	// Apply content styling with proper sizing
	styledContent := m.styles.Content.
		Width(m.width).
		Height(contentHeight).
		Render(content)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		topBar,
		styledContent,
		statusBar,
	)
}

// renderTopBar renders the top bar with current path
func (m model) renderTopBar() string {
	var title string

	switch m.currentView {
	case ViewFileBrowser:
		relPath, err := filepath.Rel(m.baseDir, m.currentPath)
		if err != nil || relPath == "." {
			title = "Restiverse"
		} else {
			title = "Restiverse — " + relPath
		}
	case ViewHistory:
		relPath, _ := filepath.Rel(m.baseDir, m.currentHTTPFile)
		title = "Restiverse — " + relPath + " — History"
	case ViewActionModal:
		title = "Restiverse — Actions"
	case ViewFuzzyFinder:
		title = "Restiverse — Find HTTP Request"
	case ViewInputModal:
		title = "Restiverse — " + m.inputTitle
	case ViewConfirmModal:
		title = "Restiverse — " + m.confirmTitle
	case ViewVariableSelect:
		title = "Restiverse — Configure Variables"
	}

	return m.styles.TopBar.
		Width(m.width).
		Render(title)
}

// renderStatusBar renders the bottom status bar with help hints
func (m model) renderStatusBar() string {
	var content string

	// Show error message if present
	if m.errorMessage != "" {
		content = m.styles.ErrorMsg.Render("Error: " + m.errorMessage)
	} else if m.statusMessage != "" {
		content = m.styles.SuccessMsg.Render(m.statusMessage)
	} else if m.requestRunning {
		content = m.styles.InfoMsg.Render("Executing request... [ESC to cancel]")
	} else {
		// Show context-appropriate help
		content = m.renderHelpHints()
	}

	return m.styles.StatusBar.
		Width(m.width).
		Render(content)
}

// renderHelpHints renders context-appropriate help hints
func (m model) renderHelpHints() string {
	var hints []string

	switch m.currentView {
	case ViewFileBrowser:
		fileCount := len(m.fileEntries)
		dirCount := 0
		for _, entry := range m.fileEntries {
			if entry.IsDir {
				dirCount++
			}
		}

		info := fmt.Sprintf("%d files, %d dirs", fileCount-dirCount, dirCount)
		if entry, ok := m.currentEntry(); ok {
			info += " | " + entry.Name
		}
		hints = []string{info, "n new", "c config", "R rename", "D duplicate", "X delete", "/ fuzzy", "q quit"}

	case ViewHistory:
		info := fmt.Sprintf("%d responses", len(m.responses))
		hints = []string{info, "Enter actions", "Space select", "backspace back"}

	case ViewActionModal:
		hints = []string{"Enter execute", "ESC cancel"}

	case ViewFuzzyFinder:
		hints = []string{"Type to search", "Enter select", "ESC cancel"}

	case ViewInputModal:
		prompt := "Type filename"
		if m.inputMode == "custom-command" {
			prompt = "Type command"
		}
		hints = []string{prompt, "Enter confirm", "ESC cancel"}

	case ViewConfirmModal:
		hints = []string{"Y confirm", "N cancel"}
	}

	return m.styles.HelpText.Render(strings.Join(hints, " | "))
}

// renderFileBrowser renders the file browser view
func (m model) renderFileBrowser() string {
	if len(m.fileEntries) == 0 {
		return m.renderEmptyDirectory()
	}

	var rows []string

	// Check if we have any .http files to display the table header
	hasHTTPFiles := false
	for _, entry := range m.fileEntries {
		if entry.IsHTTP {
			hasHTTPFiles = true
			break
		}
	}

	maxNameWidth := 20
	for _, entry := range m.fileEntries {
		if entry.IsHTTP && lipgloss.Width(entry.Name) > maxNameWidth {
			maxNameWidth = lipgloss.Width(entry.Name)
		}
	}
	if maxNameWidth > 40 {
		maxNameWidth = 40
	}

	// Add table header if we have .http files
	if hasHTTPFiles {
		header := "  " + padDisplay("File", maxNameWidth) + "  " + padDisplay("Method", 6) + "  URL"
		rows = append(rows, m.styles.TableHeader.Render(header))
		rows = append(rows, rule(m.width-4))
	}

	start := scrollTo(m.browserOffset, m.cursor, len(m.fileEntries), m.browserRows())
	end := min(start+m.browserRows(), len(m.fileEntries))
	for i := start; i < end; i++ {
		rows = append(rows, m.renderFileEntry(i, m.fileEntries[i], maxNameWidth))
	}
	if end < len(m.fileEntries) {
		rows = append(rows, m.styles.HelpText.Render(fmt.Sprintf("… %d more below", len(m.fileEntries)-end)))
	}

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// renderFileEntry renders a single file or directory entry
func (m model) renderFileEntry(index int, entry files.FileEntry, maxNameWidth int) string {
	var style lipgloss.Style
	isSelected := m.selectedFiles[index]
	isCursor := index == m.cursor

	// Determine style
	if isCursor {
		style = m.styles.TableRowSelected
	} else if isSelected {
		style = m.styles.MarkedItem
	} else if entry.IsDir {
		style = m.styles.Directory
	} else {
		style = m.styles.TableRow
	}

	// Format selection prefix
	prefix := "  "
	if isSelected {
		prefix = "* "
	}

	// For .http files, use table format
	if entry.IsHTTP {
		name := truncateDisplay(entry.Name, maxNameWidth)

		// Truncate URL if too long
		url := entry.URL
		maxURLWidth := m.width - maxNameWidth - 20 // Leave space for name, method, and padding
		if maxURLWidth < 20 {
			maxURLWidth = 20
		}
		url = truncateDisplay(url, maxURLWidth)

		method := entry.Method
		if method == "" {
			method = "?"
		}

		row := prefix + padDisplay(name, maxNameWidth) + "  " + padDisplay(method, 6) + "  " + url
		return style.Render(row)
	}

	// For directories, use simple format
	name := entry.Name
	if entry.IsDir {
		name = "/ " + name
	}

	return style.Render(prefix + name)
}

// renderHistory renders the history view
func (m model) renderHistory() string {
	if len(m.responses) == 0 {
		return m.renderEmptyHistory()
	}

	var rows []string

	// Header
	header := fmt.Sprintf("%-20s %-8s %-10s %-10s", "DateTime", "Status", "Duration", "Size")
	rows = append(rows, m.styles.TableHeader.Render(header))
	rows = append(rows, rule(m.width-4))

	// Rows
	start := scrollTo(m.historyOffset, m.historyCursor, len(m.responses), m.historyRows())
	end := min(start+m.historyRows(), len(m.responses))
	for i := start; i < end; i++ {
		rows = append(rows, m.renderResponseRow(i, m.responses[i]))
	}
	if end < len(m.responses) {
		rows = append(rows, m.styles.HelpText.Render(fmt.Sprintf("… %d more below", len(m.responses)-end)))
	}

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// renderResponseRow renders a single response row in the history table
func (m model) renderResponseRow(index int, resp files.ResponseEntry) string {
	var style lipgloss.Style
	isSelected := m.selectedResponses[index]
	isCursor := index == m.historyCursor

	if isCursor {
		style = m.styles.TableRowSelected
	} else if isSelected {
		style = m.styles.MarkedItem
	} else {
		style = m.styles.TableRow
	}

	// Format timestamp
	timeStr := resp.Timestamp.Format("2006-01-02 15:04")

	// Format status code
	statusStr := fmt.Sprintf("%d", resp.StatusCode)
	if resp.HasError {
		statusStr = "ERR"
	}

	// Format duration
	durationStr := fmt.Sprintf("%dms", resp.Duration.Milliseconds())

	// Format size
	sizeStr := files.FormatFileSize(resp.Size)

	prefix := "  "
	if isSelected {
		prefix = "* "
	}

	row := fmt.Sprintf("%s%-20s %-8s %-10s %-10s",
		prefix, timeStr, statusStr, durationStr, sizeStr)

	return style.Render(row)
}

// renderActionModal renders the action modal
func (m model) renderActionModal() string {
	if len(m.actions) == 0 {
		return m.styles.Modal.Render("No actions available")
	}

	var items []string

	// Title
	items = append(items, m.styles.ModalTitle.Render(m.modalTitle))
	items = append(items, "")

	// Actions
	for i, action := range m.actions {
		items = append(items, m.renderActionItem(i, action))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, items...)

	modal := m.styles.Modal.Render(content)

	// Center the modal
	return lipgloss.Place(
		m.width,
		m.height-2,
		lipgloss.Center,
		lipgloss.Center,
		modal,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.AdaptiveColor{Light: "#999", Dark: "#666"}),
	)
}

// renderActionItem renders a single action in the modal
func (m model) renderActionItem(index int, action config.Action) string {
	var style lipgloss.Style
	isCursor := index == m.modalCursor

	if isCursor {
		style = m.styles.ModalSelected
	} else {
		style = m.styles.ModalItem
	}

	name := action.Name
	if action.Keybinding != "" {
		name += fmt.Sprintf(" [%s]", action.Keybinding)
	}

	prefix := "  "
	if isCursor {
		prefix = "> "
	}

	return style.Render(prefix + name)
}

// renderFuzzyFinder renders the fuzzy finder view
func (m model) renderFuzzyFinder() string {
	var items []string

	// Search input
	items = append(items, "Search: "+m.searchInput.View())
	items = append(items, "")

	// Results
	if len(m.searchResults) == 0 {
		items = append(items, m.styles.HelpText.Render("No results"))
	} else {
		start := scrollTo(m.searchOffset, m.searchCursor, len(m.searchResults), m.searchRows())
		end := min(start+m.searchRows(), len(m.searchResults))
		for i := start; i < end; i++ {
			result := m.searchResults[i]
			var style lipgloss.Style
			if i == m.searchCursor {
				style = m.styles.SelectedItem
			} else {
				style = m.styles.File
			}

			prefix := "  "
			if i == m.searchCursor {
				prefix = "> "
			}

			items = append(items, style.Render(prefix+result.Name))
		}
		if end < len(m.searchResults) {
			items = append(items, m.styles.HelpText.Render(fmt.Sprintf("… %d more below", len(m.searchResults)-end)))
		}
	}

	content := lipgloss.JoinVertical(lipgloss.Left, items...)

	// Render as a modal overlay
	modal := m.styles.Modal.Render(content)

	return lipgloss.Place(
		m.width,
		m.height-2,
		lipgloss.Center,
		lipgloss.Center,
		modal,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.AdaptiveColor{Light: "#999", Dark: "#666"}),
	)
}

// renderEmptyDirectory renders the empty state for file browser
func (m model) renderEmptyDirectory() string {
	msg := "No .http files in this directory\n\n"
	msg += "Press 'n' to create a new .http file\n"
	msg += "Or navigate to a subdirectory with the arrow keys"
	return m.styles.HelpText.Render(msg)
}

// renderEmptyHistory renders the empty state for history view
func (m model) renderEmptyHistory() string {
	msg := "No response history for this request\n\n"
	msg += "Press 'r' to execute the request\n"
	msg += "Press 'backspace' to return to file browser"
	return m.styles.HelpText.Render(msg)
}

// renderInputModal renders the input modal
func (m model) renderInputModal() string {
	var items []string

	// Title
	items = append(items, m.styles.ModalTitle.Render(m.inputTitle))
	items = append(items, "")

	// Input field
	items = append(items, m.inputField.View())
	items = append(items, "")

	// Help text
	help := "Enter to confirm | ESC to cancel"
	if m.inputMode == "custom-command" {
		help = "{filename} is replaced with the selected file | " + help
	}
	items = append(items, m.styles.HelpText.Render(help))

	content := lipgloss.JoinVertical(lipgloss.Left, items...)

	modal := m.styles.Modal.Render(content)

	// Center the modal
	return lipgloss.Place(
		m.width,
		m.height-2,
		lipgloss.Center,
		lipgloss.Center,
		modal,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.AdaptiveColor{Light: "#999", Dark: "#666"}),
	)
}

// renderConfirmModal renders the confirmation modal
func (m model) renderConfirmModal() string {
	var items []string

	// Title
	items = append(items, m.styles.ModalTitle.Render(m.confirmTitle))
	items = append(items, "")

	// Message (split by newlines)
	for _, line := range strings.Split(m.confirmMessage, "\n") {
		items = append(items, line)
	}
	items = append(items, "")

	// Help text
	items = append(items, m.styles.HelpText.Render("Y to confirm | N to cancel"))

	content := lipgloss.JoinVertical(lipgloss.Left, items...)

	modal := m.styles.Modal.Render(content)

	// Center the modal
	return lipgloss.Place(
		m.width,
		m.height-2,
		lipgloss.Center,
		lipgloss.Center,
		modal,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.AdaptiveColor{Light: "#999", Dark: "#666"}),
	)
}

// renderVariableSelect renders the variable selection modal
func (m model) renderVariableSelect() string {
	if len(m.varNames) == 0 || m.varCurrentIdx < 0 || m.varCurrentIdx >= len(m.varNames) {
		return m.styles.Modal.Render("No variables to configure")
	}

	var items []string

	// Title
	currentVar := m.varNames[m.varCurrentIdx]
	title := fmt.Sprintf("Configure Variable: %s (%d/%d)", currentVar, m.varCurrentIdx+1, len(m.varNames))
	items = append(items, m.styles.ModalTitle.Render(title))
	items = append(items, "")

	// Options
	options := m.varDefinitions[currentVar]
	for i, option := range options {
		var itemStr string
		if i == m.varOptionCursor {
			itemStr = m.styles.SelectedItem.Render("▸ " + option)
		} else {
			itemStr = "  " + option
		}
		items = append(items, itemStr)
	}

	items = append(items, "")

	// Show current values for all variables
	items = append(items, m.styles.HelpText.Render("Current values:"))
	for i, varName := range m.varNames {
		if value, ok := m.varValues[varName]; ok {
			indicator := " "
			if i == m.varCurrentIdx {
				indicator = "▸"
			}
			items = append(items, fmt.Sprintf("%s %s: %s", indicator, varName, value))
		}
	}

	items = append(items, "")
	items = append(items, m.styles.HelpText.Render("↑/↓ to select | Enter to confirm | ESC to cancel"))

	content := lipgloss.JoinVertical(lipgloss.Left, items...)

	modal := m.styles.Modal.Render(content)

	// Center the modal
	return lipgloss.Place(
		m.width,
		m.height-2,
		lipgloss.Center,
		lipgloss.Center,
		modal,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.AdaptiveColor{Light: "#999", Dark: "#666"}),
	)
}

func rule(width int) string {
	if width < 1 {
		return ""
	}
	return strings.Repeat("─", width)
}

func padDisplay(value string, width int) string {
	if padding := width - lipgloss.Width(value); padding > 0 {
		return value + strings.Repeat(" ", padding)
	}
	return value
}

func truncateDisplay(value string, width int) string {
	if width < 1 {
		return ""
	}
	if lipgloss.Width(value) <= width {
		return value
	}
	const suffix = "..."
	if width <= lipgloss.Width(suffix) {
		return strings.Repeat(".", width)
	}
	remaining := width - lipgloss.Width(suffix)
	var builder strings.Builder
	graphemes := uniseg.NewGraphemes(value)
	used := 0
	for graphemes.Next() {
		cluster := graphemes.Str()
		clusterWidth := uniseg.StringWidth(cluster)
		if used+clusterWidth > remaining {
			break
		}
		builder.WriteString(cluster)
		used += clusterWidth
	}
	return builder.String() + suffix
}
