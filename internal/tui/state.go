package tui

import "github.com/cvidmar/restiverse/internal/files"

func (m model) currentEntry() (files.FileEntry, bool) {
	if m.cursor < 0 || m.cursor >= len(m.fileEntries) {
		return files.FileEntry{}, false
	}
	return m.fileEntries[m.cursor], true
}

func (m model) currentResponse() (files.ResponseEntry, bool) {
	if m.historyCursor < 0 || m.historyCursor >= len(m.responses) {
		return files.ResponseEntry{}, false
	}
	return m.responses[m.historyCursor], true
}

func clampIndex(index, length int) int {
	if length == 0 || index < 0 {
		return 0
	}
	if index >= length {
		return length - 1
	}
	return index
}

func (m *model) clampCursors() {
	m.cursor = clampIndex(m.cursor, len(m.fileEntries))
	m.historyCursor = clampIndex(m.historyCursor, len(m.responses))
	m.modalCursor = clampIndex(m.modalCursor, len(m.actions))
	m.searchCursor = clampIndex(m.searchCursor, len(m.searchResults))
	m.browserOffset = scrollTo(m.browserOffset, m.cursor, len(m.fileEntries), m.browserRows())
	m.historyOffset = scrollTo(m.historyOffset, m.historyCursor, len(m.responses), m.historyRows())
	m.searchOffset = scrollTo(m.searchOffset, m.searchCursor, len(m.searchResults), m.searchRows())
}

func scrollTo(offset, cursor, length, height int) int {
	if height < 1 {
		height = 1
	}
	if cursor < offset {
		offset = cursor
	}
	if cursor >= offset+height {
		offset = cursor - height + 1
	}
	if maxOffset := length - height; offset > maxOffset {
		offset = maxOffset
	}
	if offset < 0 {
		return 0
	}
	return offset
}

func (m model) contentRows(hasHeader bool) int {
	rows := m.height - 4
	if hasHeader {
		rows -= 2
	}
	if rows < 1 {
		return 1
	}
	return rows
}

func (m model) browserRows() int {
	hasHeader := false
	for _, entry := range m.fileEntries {
		if entry.IsHTTP {
			hasHeader = true
			break
		}
	}
	return listItemRows(m.contentRows(hasHeader))
}

func (m model) historyRows() int {
	return listItemRows(m.contentRows(len(m.responses) > 0))
}

func (m model) searchRows() int {
	rows := m.height - 10
	if rows < 1 {
		return 1
	}
	return listItemRows(rows)
}

func listItemRows(available int) int {
	if available > 1 {
		return available - 1 // reserve a row for the "more below" indicator
	}
	return 1
}
