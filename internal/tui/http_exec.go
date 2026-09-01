package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	httpPkg "github.com/cvidmar/restiverse/internal/http"
	"github.com/cvidmar/restiverse/internal/respfile"
	"github.com/cvidmar/restiverse/internal/vars"
)

// executeHTTPRequest executes an HTTP request from a .http file
func (m model) executeHTTPRequest() (model, tea.Cmd) {
	if m.requestRunning {
		m.statusMessage = "A request is already running — ESC to cancel"
		return m, nil
	}
	if m.currentView != ViewFileBrowser || len(m.fileEntries) == 0 {
		return m, nil
	}

	entry, ok := m.currentEntry()
	if !ok {
		return m, nil
	}
	if !entry.IsHTTP {
		m.errorMessage = "Selected file is not an .http file"
		return m, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), m.config.Timeout)
	m.cancelFunc = cancel
	m.requestRunning = true
	m.statusMessage = "Executing request..."
	m.errorMessage = ""
	m.currentHTTPFile = entry.Path

	return m, m.executeRequestCmd(ctx, entry.Path)
}

// executeRequestCmd returns a command that executes an HTTP request
func (m model) executeRequestCmd(ctx context.Context, httpFilePath string) tea.Cmd {
	return func() tea.Msg {
		// Parse HTTP file
		req, err := httpPkg.ParseHTTPFile(httpFilePath)
		if err != nil {
			return requestErrorMsg{fmt.Errorf("failed to parse .http file: %w", err)}
		}

		varNames := vars.ExtractRequest(req.URL, req.Headers, req.Body)
		if len(varNames) > 0 {
			varValues, err := vars.ResolveValues(httpFilePath, m.config.Vars, varNames)
			if err != nil {
				return requestErrorMsg{fmt.Errorf("failed to load variables: %w", err)}
			}
			vars.SubstituteRequest(&req.URL, req.Headers, &req.Body, varValues)
		}

		record := respfile.NewRecord(httpFilePath, time.Now())
		if err := os.MkdirAll(filepath.Dir(record.BodyPath), 0o755); err != nil {
			return requestErrorMsg{fmt.Errorf("failed to create responses directory: %w", err)}
		}
		bodyFile, err := os.OpenFile(record.BodyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
		if err != nil {
			return requestErrorMsg{fmt.Errorf("failed to create response body: %w", err)}
		}
		if err := bodyFile.Chmod(0o600); err != nil {
			bodyFile.Close()
			os.Remove(record.BodyPath)
			return requestErrorMsg{fmt.Errorf("failed to secure response body: %w", err)}
		}

		resp, requestErr := httpPkg.StreamResponseToFile(ctx, req, bodyFile)
		closeErr := bodyFile.Close()
		if errors.Is(ctx.Err(), context.Canceled) {
			os.Remove(record.BodyPath)
			return requestCancelledMsg{}
		}
		if closeErr != nil && requestErr == nil {
			requestErr = fmt.Errorf("failed to close response body: %w", closeErr)
		}
		if requestErr != nil {
			os.Remove(record.BodyPath)
		} else if info, statErr := os.Stat(record.BodyPath); statErr == nil && info.Size() == 0 {
			os.Remove(record.BodyPath)
		}

		options := httpPkg.MetadataOptions{
			SensitiveHeaders:     m.config.EffectiveSensitiveHeaders(),
			SensitiveQueryParams: m.config.EffectiveSensitiveQueryParams(),
		}
		if saveErr := httpPkg.SaveResponseMetadata(record, req, resp, requestErr, options); saveErr != nil {
			return requestErrorMsg{fmt.Errorf("failed to save response: %w", saveErr)}
		}

		// Cleanup old responses if configured
		if m.config.MaxResponses > 0 {
			cleanupErr := httpPkg.CleanupOldResponses(httpFilePath, m.config.MaxResponses)
			if cleanupErr != nil {
				// Log warning but don't fail the request
				fmt.Fprintf(os.Stderr, "Warning: failed to cleanup old responses: %v\n", cleanupErr)
			}
		}

		if requestErr != nil {
			return requestErrorMsg{requestErr}
		}

		// Build success message
		duration := fmt.Sprintf("Request completed: %d %s (%dms)",
			resp.StatusCode,
			getStatusText(resp.StatusCode),
			resp.Duration.Milliseconds())

		return requestCompleteMsg{
			duration: duration,
		}
	}
}

// getStatusText returns a human-readable status text for a status code
func getStatusText(code int) string {
	switch code {
	case 200:
		return "OK"
	case 201:
		return "Created"
	case 204:
		return "No Content"
	case 400:
		return "Bad Request"
	case 401:
		return "Unauthorized"
	case 403:
		return "Forbidden"
	case 404:
		return "Not Found"
	case 500:
		return "Internal Server Error"
	case 502:
		return "Bad Gateway"
	case 503:
		return "Service Unavailable"
	default:
		if code >= 200 && code < 300 {
			return "Success"
		} else if code >= 400 && code < 500 {
			return "Client Error"
		} else if code >= 500 {
			return "Server Error"
		}
		return ""
	}
}
