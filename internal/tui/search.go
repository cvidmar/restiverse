package tui

import (
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cvidmar/restiverse/internal/files"
)

type searchResultsMsg struct {
	results []files.FileEntry
}

// searchHTTPFilesCmd returns a command that searches for HTTP files
func (m model) searchHTTPFilesCmd(query string) tea.Cmd {
	return func() tea.Msg {
		if query == "" {
			return searchResultsMsg{[]files.FileEntry{}}
		}

		// Find all HTTP files in the base directory
		httpFiles, err := files.FindAllHTTPFiles(m.baseDir)
		if err != nil {
			return errMsg{err}
		}

		// Filter by query
		var results []files.FileEntry
		query = strings.ToLower(query)

		for _, httpFile := range httpFiles {
			// Check filename
			if strings.Contains(strings.ToLower(filepath.Base(httpFile)), query) {
				info, err := os.Stat(httpFile)
				if err != nil {
					continue
				}

				relPath, _ := filepath.Rel(m.baseDir, httpFile)
				results = append(results, files.FileEntry{
					Name:    relPath,
					Path:    httpFile,
					IsDir:   false,
					IsHTTP:  true,
					ModTime: info.ModTime(),
					Size:    info.Size(),
				})
				continue
			}

			// Check file content
			content, err := os.ReadFile(httpFile)
			if err != nil {
				continue
			}

			if strings.Contains(strings.ToLower(string(content)), query) {
				info, err := os.Stat(httpFile)
				if err != nil {
					continue
				}

				relPath, _ := filepath.Rel(m.baseDir, httpFile)
				results = append(results, files.FileEntry{
					Name:    relPath,
					Path:    httpFile,
					IsDir:   false,
					IsHTTP:  true,
					ModTime: info.ModTime(),
					Size:    info.Size(),
				})
			}
		}

		return searchResultsMsg{results}
	}
}

// Update the model to handle search results
func (m model) handleSearchResults(msg searchResultsMsg) (model, tea.Cmd) {
	m.searchResults = msg.results
	m.cursor = 0
	return m, nil
}
