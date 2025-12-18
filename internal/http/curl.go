package http

import (
	"fmt"
	"strings"
)

// ToCurlCommand converts an HTTPRequest to a multi-line curl command
func (r *HTTPRequest) ToCurlCommand() string {
	var parts []string

	// Start with curl and the method (if not GET)
	if r.Method != "GET" {
		parts = append(parts, fmt.Sprintf("curl -X %s \\", r.Method))
	} else {
		parts = append(parts, "curl \\")
	}

	// Add headers
	for name, value := range r.Headers {
		escapedValue := shellEscape(value)
		parts = append(parts, fmt.Sprintf("  -H '%s: %s' \\", name, escapedValue))
	}

	// Add body if present
	if r.HasBody() {
		escapedBody := shellEscape(r.Body)
		parts = append(parts, fmt.Sprintf("  -d '%s' \\", escapedBody))
	}

	// Add URL (last line, no backslash)
	parts = append(parts, fmt.Sprintf("  '%s'", r.URL))

	return strings.Join(parts, "\n")
}

// shellEscape escapes single quotes in a string for use in shell single quotes
// by replacing ' with '\''
func shellEscape(s string) string {
	return strings.ReplaceAll(s, "'", "'\\''")
}
