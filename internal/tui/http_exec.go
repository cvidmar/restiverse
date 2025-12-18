package tui

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	httpPkg "github.com/cvidmar/restiverse/internal/http"
	"github.com/cvidmar/restiverse/internal/vars"
)

// executeHTTPRequest executes an HTTP request from a .http file
func (m model) executeHTTPRequest() (model, tea.Cmd) {
	if m.currentView != ViewFileBrowser || len(m.fileEntries) == 0 {
		return m, nil
	}

	entry := m.fileEntries[m.cursor]
	if !entry.IsHTTP {
		m.errorMessage = "Selected file is not an .http file"
		return m, nil
	}

	// Mark request as running
	m.requestRunning = true
	m.statusMessage = "Executing request..."
	m.errorMessage = ""
	m.currentHTTPFile = entry.Path

	return m, m.executeRequestCmd(entry.Path)
}

// executeRequestCmd returns a command that executes an HTTP request
func (m model) executeRequestCmd(httpFilePath string) tea.Cmd {
	return func() tea.Msg {
		// Parse HTTP file
		req, err := httpPkg.ParseHTTPFile(httpFilePath)
		if err != nil {
			return requestErrorMsg{fmt.Errorf("failed to parse .http file: %w", err)}
		}

		// Extract variables from the request
		varNames := vars.ExtractVariables(req.URL)
		for _, headerVal := range req.Headers {
			headerVars := vars.ExtractVariables(headerVal)
			for _, v := range headerVars {
				found := false
				for _, existing := range varNames {
					if existing == v {
						found = true
						break
					}
				}
				if !found {
					varNames = append(varNames, v)
				}
			}
		}

		// Load or get default variable values
		var varValues vars.VarValues
		if len(varNames) > 0 {
			// Try to load from most recent .meta file
			varValues, _ = vars.LoadVariableValues(httpFilePath)

			// Fill in missing values with defaults from config
			defaultValues := vars.GetDefaultValues(m.config.Vars, varNames)
			if varValues == nil {
				varValues = defaultValues
			} else {
				for name, value := range defaultValues {
					if _, ok := varValues[name]; !ok {
						varValues[name] = value
					}
				}
			}

			// Substitute variables in URL and headers
			req.URL = vars.SubstituteVariables(req.URL, varValues)
			for name, value := range req.Headers {
				req.Headers[name] = vars.SubstituteVariables(value, varValues)
			}
		}

		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), m.config.Timeout)
		defer cancel()

		// Execute request
		resp, err := httpPkg.ExecuteRequest(ctx, req)

		// Save response (even if there was an error)
		saveErr := httpPkg.SaveResponse(httpFilePath, req, resp, varValues, err)
		if saveErr != nil {
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

		if err != nil {
			return requestErrorMsg{err}
		}

		// Build success message
		duration := fmt.Sprintf("Request completed: %d %s (%dms)",
			resp.StatusCode,
			getStatusText(resp.StatusCode),
			resp.Duration.Milliseconds())

		return requestCompleteMsg{
			statusCode: resp.StatusCode,
			duration:   duration,
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
