package tui

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cvidmar/restiverse/internal/config"
	"github.com/cvidmar/restiverse/internal/files"
)

func testModel(t *testing.T) model {
	t.Helper()
	cfg := config.DefaultConfig()
	return NewModel(cfg, t.TempDir())
}

func TestFuzzyFinderCursorDoesNotLeak(t *testing.T) {
	m := testModel(t)
	m.fileEntries = []files.FileEntry{{Name: "a.http", IsHTTP: true}, {Name: "b.http", IsHTTP: true}}
	m.cursor = 1
	m.currentView = ViewFuzzyFinder
	m.previousView = ViewFileBrowser
	m.searchResults = make([]files.FileEntry, 6)
	m.searchCursor = 5

	updated, _ := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyEsc})
	got := updated.(model)
	if got.currentView != ViewFileBrowser || got.cursor != 1 {
		t.Fatalf("view=%v cursor=%d", got.currentView, got.cursor)
	}
	got.handleKeyPress(tea.KeyMsg{Type: tea.KeyEnter}) // must not panic
}

func TestVariableSelectorSurvivesEmptyOptions(t *testing.T) {
	m := testModel(t)
	m.currentView = ViewVariableSelect
	m.previousView = ViewFileBrowser
	m.varNames = []string{"env"}
	m.varDefinitions = map[string][]string{"env": {}}
	m.varValues = map[string]string{}

	updated, _ := m.updateVariableSelect(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(model)
	if got.currentView != ViewFileBrowser || !strings.Contains(got.errorMessage, "no values") {
		t.Fatalf("view=%v error=%q", got.currentView, got.errorMessage)
	}
}

func TestRenderAtTinyWidths(t *testing.T) {
	for width := 1; width <= 10; width++ {
		m := testModel(t)
		m.width = width
		m.height = 8
		m.fileEntries = []files.FileEntry{{Name: "请求.http", IsHTTP: true, Method: "GET", URL: "https://例.example"}}
		_ = m.View()
	}
}

func TestScrollToAndCursorVisibility(t *testing.T) {
	if got := scrollTo(0, 9, 10, 3); got != 7 {
		t.Fatalf("scrollTo = %d, want 7", got)
	}
	m := testModel(t)
	m.width = 80
	m.height = 10
	m.fileEntries = make([]files.FileEntry, 100)
	for i := range m.fileEntries {
		m.fileEntries[i] = files.FileEntry{Name: "a.http", IsHTTP: true}
	}
	for i := 0; i < 200; i++ {
		updated, _ := m.updateFileBrowser(tea.KeyMsg{Type: tea.KeyDown})
		m = updated.(model)
		rows := m.browserRows()
		if m.cursor < m.browserOffset || m.cursor >= m.browserOffset+rows {
			t.Fatalf("cursor %d outside [%d,%d)", m.cursor, m.browserOffset, m.browserOffset+rows)
		}
	}
}

func TestSecondExecuteIsRejectedWhileRunning(t *testing.T) {
	m := testModel(t)
	m.requestRunning = true
	updated, cmd := m.executeHTTPRequest()
	if cmd != nil || !strings.Contains(updated.statusMessage, "already running") {
		t.Fatalf("cmd=%v status=%q", cmd, updated.statusMessage)
	}
}

func TestEscCancelsRequestAndKeepsGuardUntilAcknowledged(t *testing.T) {
	m := testModel(t)
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelFunc = cancel
	m.requestRunning = true

	updated, _ := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyEsc})
	cancelling := updated.(model)
	select {
	case <-ctx.Done():
	default:
		t.Fatal("ESC did not cancel the request context")
	}
	if !cancelling.requestRunning || cancelling.statusMessage != "Cancelling request..." {
		t.Fatalf("running=%v status=%q", cancelling.requestRunning, cancelling.statusMessage)
	}
	if _, cmd := cancelling.executeHTTPRequest(); cmd != nil {
		t.Fatal("a second request was allowed while cancellation was pending")
	}

	acknowledged, _ := cancelling.Update(requestCancelledMsg{})
	done := acknowledged.(model)
	if done.requestRunning || done.cancelFunc != nil {
		t.Fatalf("cancel acknowledgement did not release state: running=%v cancel=%v", done.requestRunning, done.cancelFunc)
	}
}

func TestFilenamePromptRejectsTraversal(t *testing.T) {
	for _, name := range []string{"../x", "a/b", `a\b`, "..", "", "  "} {
		if _, err := resolveNewName(t.TempDir(), name); err == nil {
			t.Errorf("resolveNewName accepted %q", name)
		}
	}
	if _, err := resolveNewName(t.TempDir(), "valid.http"); err != nil {
		t.Fatalf("valid filename rejected: %v", err)
	}
	m := testModel(t)
	updated, cmd := m.createHTTPFileWithName("  ")
	if cmd != nil || updated.errorMessage == "" {
		t.Fatalf("whitespace-only create was accepted: cmd=%v error=%q", cmd, updated.errorMessage)
	}
}

func TestEditorCommandSupportsArguments(t *testing.T) {
	cmd := editorCommand("code -w", "/tmp/a file.http")
	if len(cmd.Args) != 3 || cmd.Args[0] != "sh" || cmd.Args[1] != "-c" || cmd.Args[2] != "code -w '/tmp/a file.http'" {
		t.Fatalf("editor command args = %#v", cmd.Args)
	}
}

func TestSelectedOrderIsAscending(t *testing.T) {
	m := testModel(t)
	m.selectedFiles = map[int]bool{4: true, 1: true, 3: true}
	got := m.getSelectedFiles()
	want := []int{1, 3, 4}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("selected = %v", got)
		}
	}
}

func TestExecuteStreamsResponseAndSubstitutesBody(t *testing.T) {
	received := make(chan string, 1)
	oldTransport := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		data, _ := io.ReadAll(r.Body)
		received <- string(data)
		return &http.Response{
			StatusCode: 200,
			Header:     http.Header{"Content-Type": {"text/plain"}},
			Body:       io.NopCloser(strings.NewReader("streamed response")),
			Request:    r,
		}, nil
	})
	defer func() { http.DefaultTransport = oldTransport }()

	m := testModel(t)
	m.config.Vars = map[string][]string{"value": {"resolved"}}
	httpPath := filepath.Join(m.baseDir, "post.http")
	request := "POST https://example.test\nContent-Type: text/plain\n\n{value}\n"
	if err := os.WriteFile(httpPath, []byte(request), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	msg := m.executeRequestCmd(ctx, httpPath)()
	if _, ok := msg.(requestCompleteMsg); !ok {
		t.Fatalf("message = %#v", msg)
	}
	if got := <-received; got != "resolved" {
		t.Fatalf("received body = %q", got)
	}

	entries, err := os.ReadDir(filepath.Join(m.baseDir, "responses"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("response artifacts = %d, want 2", len(entries))
	}
	stems := map[string]bool{}
	for _, entry := range entries {
		stems[strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))] = true
		info, err := entry.Info()
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Errorf("%s mode = %o", entry.Name(), info.Mode().Perm())
		}
	}
	if len(stems) != 1 {
		t.Fatalf("body and metadata record IDs differ: %v", stems)
	}
}

func TestCancelledRequestLeavesNoArtifact(t *testing.T) {
	oldTransport := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return nil, r.Context().Err()
	})
	defer func() { http.DefaultTransport = oldTransport }()

	m := testModel(t)
	httpPath := filepath.Join(m.baseDir, "get.http")
	if err := os.WriteFile(httpPath, []byte("GET https://example.invalid\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	msg := m.executeRequestCmd(ctx, httpPath)()
	if _, ok := msg.(requestCancelledMsg); !ok {
		t.Fatalf("message = %#v", msg)
	}
	entries, err := os.ReadDir(filepath.Join(m.baseDir, "responses"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("cancelled request left artifacts: %v", entries)
	}
}

func TestConfigEditReloadsBaseConfigAndKeepsOldOnError(t *testing.T) {
	baseDir := t.TempDir()
	oldConfig := config.DefaultConfig()
	m := NewModel(oldConfig, baseDir)
	newConfig := config.DefaultConfig()
	newConfig.Timeout = 2 * time.Second
	if err := config.SaveConfig(filepath.Join(baseDir, "restiverse.yaml"), newConfig); err != nil {
		t.Fatal(err)
	}

	updated, _ := m.Update(configEditedMsg{})
	got := updated.(model)
	if got.config.Timeout != 2*time.Second || got.statusMessage != "Configuration reloaded" {
		t.Fatalf("timeout=%v status=%q", got.config.Timeout, got.statusMessage)
	}

	if err := os.WriteFile(filepath.Join(baseDir, "restiverse.yaml"), []byte("timeout: ["), 0o600); err != nil {
		t.Fatal(err)
	}
	updated, _ = got.Update(configEditedMsg{})
	afterError := updated.(model)
	if afterError.config != got.config || !strings.Contains(afterError.errorMessage, "Config not reloaded") {
		t.Fatalf("config changed after invalid edit: error=%q", afterError.errorMessage)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestUnicodeTruncationUsesDisplayWidth(t *testing.T) {
	got := truncateDisplay("é界e\u0301abcdef", 6)
	if !utf8.ValidString(got) || lipgloss.Width(got) > 6 {
		t.Fatalf("truncateDisplay = %q (width %d)", got, lipgloss.Width(got))
	}
}
