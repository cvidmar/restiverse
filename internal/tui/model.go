package tui

import (
	"context"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/cvidmar/restiverse/internal/config"
	"github.com/cvidmar/restiverse/internal/files"
)

// ViewType represents the different views in the application
type ViewType int

const (
	ViewFileBrowser ViewType = iota
	ViewHistory
	ViewActionModal
	ViewFuzzyFinder
	ViewInputModal
	ViewConfirmModal
)

// model represents the application state
type model struct {
	// Configuration
	config  *config.Config
	baseDir string

	// Current view
	currentView  ViewType
	previousView ViewType // For returning from modal

	// File browser state
	currentPath   string
	fileEntries   []files.FileEntry
	cursor        int
	selectedFiles map[int]bool // Multi-selection

	// History view state
	currentHTTPFile   string
	responses         []files.ResponseEntry
	historyCursor     int
	selectedResponses map[int]bool

	// Action modal state
	actions     []config.Action
	modalCursor int
	modalTitle  string

	// Fuzzy finder state
	searchInput   textinput.Model
	searchResults []files.FileEntry

	// Input modal state
	inputField      textinput.Model
	inputMode       string // "create", "rename"
	inputTitle      string
	renameTargetIdx int // For rename operations

	// Confirm modal state
	confirmTitle   string
	confirmMessage string
	confirmAction  func(model) (model, tea.Cmd) // Callback for confirmed action

	// HTTP execution state
	requestRunning bool
	cancelFunc     context.CancelFunc

	// Status and messages
	statusMessage string
	errorMessage  string

	// Terminal dimensions
	width  int
	height int

	// Styling
	styles Styles
	keys   KeyMap
}

// NewModel creates a new model with the given configuration and base directory
func NewModel(cfg *config.Config, baseDir string) model {
	ti := textinput.New()
	ti.Placeholder = "Search..."
	ti.Focus()

	inputField := textinput.New()
	inputField.Placeholder = "filename.http"
	inputField.CharLimit = 255

	return model{
		config:            cfg,
		baseDir:           baseDir,
		currentView:       ViewFileBrowser,
		currentPath:       baseDir,
		selectedFiles:     make(map[int]bool),
		selectedResponses: make(map[int]bool),
		searchInput:       ti,
		inputField:        inputField,
		styles:            DefaultStyles(),
		keys:              DefaultKeyMap(),
	}
}

// Init is the first function called in the Bubbletea program
func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.loadDirectoryCmd(),
	)
}

// loadDirectoryCmd returns a command that loads the current directory
func (m model) loadDirectoryCmd() tea.Cmd {
	return func() tea.Msg {
		entries, err := files.ListDirectory(m.currentPath)
		if err != nil {
			return errMsg{err}
		}
		return directoryLoadedMsg{entries}
	}
}

// navigateToDir returns a command that navigates to a new directory
func (m model) navigateToDir(newPath string) tea.Cmd {
	return func() tea.Msg {
		entries, err := files.ListDirectory(newPath)
		if err != nil {
			return errMsg{err}
		}
		return navigatedToDirMsg{newPath, entries}
	}
}

// Messages

type directoryLoadedMsg struct {
	entries []files.FileEntry
}

type navigatedToDirMsg struct {
	path    string
	entries []files.FileEntry
}

type errMsg struct {
	err error
}

type historyLoadedMsg struct {
	httpFile  string
	responses []files.ResponseEntry
}

type requestStartedMsg struct{}

type requestCompleteMsg struct {
	statusCode int
	duration   string
}

type requestErrorMsg struct {
	err error
}

type externalToolCompleteMsg struct{}

type externalToolErrorMsg struct {
	err error
}

type clearStatusMsg struct{}
