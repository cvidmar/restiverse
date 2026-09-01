package tui

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cvidmar/restiverse/internal/files"
)

type searchCandidate struct {
	entry      files.FileEntry
	searchText string
}

type searchCandidatesLoadedMsg struct {
	session    int
	candidates []searchCandidate
}

// loadSearchCandidatesCmd walks and reads the request tree once per finder session.
func (m model) loadSearchCandidatesCmd(session int) tea.Cmd {
	return func() tea.Msg {
		httpFiles, err := files.FindAllHTTPFiles(m.baseDir)
		if err != nil {
			return errMsg{err}
		}

		candidates := make([]searchCandidate, 0, len(httpFiles))
		for _, httpFile := range httpFiles {
			info, err := os.Stat(httpFile)
			if err != nil {
				continue
			}
			file, err := os.Open(httpFile)
			if err != nil {
				continue
			}
			content, _ := io.ReadAll(io.LimitReader(file, 64*1024))
			file.Close()
			relPath, _ := filepath.Rel(m.baseDir, httpFile)
			entry := files.FileEntry{
				Name:    relPath,
				Path:    httpFile,
				IsHTTP:  true,
				ModTime: info.ModTime(),
				Size:    info.Size(),
			}
			candidates = append(candidates, searchCandidate{
				entry:      entry,
				searchText: strings.ToLower(relPath + "\n" + string(content)),
			})
		}
		return searchCandidatesLoadedMsg{session: session, candidates: candidates}
	}
}

func (m model) handleSearchCandidatesLoaded(msg searchCandidatesLoadedMsg) (model, tea.Cmd) {
	if msg.session != m.searchSession || m.currentView != ViewFuzzyFinder {
		return m, nil
	}
	m.searchCandidates = msg.candidates
	m.filterSearchCandidates(m.searchInput.Value())
	return m, nil
}

func (m *model) filterSearchCandidates(query string) {
	query = strings.ToLower(strings.TrimSpace(query))
	m.searchResults = nil
	if query != "" {
		for _, candidate := range m.searchCandidates {
			if strings.Contains(candidate.searchText, query) {
				m.searchResults = append(m.searchResults, candidate.entry)
			}
		}
	}
	m.searchCursor = 0
	m.searchOffset = 0
}
