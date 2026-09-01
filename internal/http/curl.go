package http

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cvidmar/restiverse/internal/shellquote"
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
	headerNames := make([]string, 0, len(r.Headers))
	for name := range r.Headers {
		headerNames = append(headerNames, name)
	}
	sort.Strings(headerNames)
	for _, name := range headerNames {
		parts = append(parts, fmt.Sprintf("  -H %s \\", shellquote.Quote(name+": "+r.Headers[name])))
	}

	// Add body if present
	if r.HasBody() {
		parts = append(parts, fmt.Sprintf("  -d %s \\", shellquote.Quote(r.Body)))
	}

	// Add URL (last line, no backslash)
	parts = append(parts, fmt.Sprintf("  %s", shellquote.Quote(r.URL)))

	return strings.Join(parts, "\n")
}
