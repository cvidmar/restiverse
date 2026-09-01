package http

import (
	"fmt"
	"os"
	"strings"
)

// HTTPRequest represents a parsed HTTP request from a .http file
type HTTPRequest struct {
	Method  string
	URL     string
	Headers map[string]string
	Body    string
}

// ParseHTTPFile parses a .http file and returns an HTTPRequest
func ParseHTTPFile(filePath string) (*HTTPRequest, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	var method, url string
	headers := make(map[string]string)
	var bodyLines []string
	inBody := false

	for _, line := range strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n") {
		line = strings.TrimSuffix(line, "\r")
		trimmed := strings.TrimSpace(line)

		if inBody {
			bodyLines = append(bodyLines, line)
			continue
		}

		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			if method != "" && trimmed == "" {
				inBody = true
			}
			continue
		}

		if method == "" {
			var ok bool
			method, url, ok = ParseRequestLine(line)
			if !ok {
				continue
			}
			continue
		}

		// Parse headers (before empty line)
		if strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				headerName := strings.TrimSpace(parts[0])
				headerValue := strings.TrimSpace(parts[1])
				headers[headerName] = headerValue
			}
		}
	}

	// Validate required fields
	if method == "" || url == "" {
		return nil, fmt.Errorf("invalid HTTP file: missing method or URL")
	}

	// Validate HTTP method
	validMethods := map[string]bool{
		"GET": true, "POST": true, "PUT": true, "DELETE": true,
		"PATCH": true, "HEAD": true, "OPTIONS": true, "CONNECT": true,
		"TRACE": true,
	}
	if !validMethods[method] {
		return nil, fmt.Errorf("invalid HTTP method: %s", method)
	}

	// Join body lines
	body := strings.TrimSuffix(strings.Join(bodyLines, "\n"), "\n")

	return &HTTPRequest{
		Method:  method,
		URL:     url,
		Headers: headers,
		Body:    body,
	}, nil
}

// ParseRequestLine parses METHOD URL from a request line.
func ParseRequestLine(line string) (method, url string, ok bool) {
	parts := strings.Fields(strings.TrimSpace(line))
	if len(parts) < 2 {
		return "", "", false
	}
	return strings.ToUpper(parts[0]), parts[1], true
}

// GetContentType returns the Content-Type header value, or empty string if not set
func (r *HTTPRequest) GetContentType() string {
	// Check both capitalization variants
	if ct, ok := r.Headers["Content-Type"]; ok {
		return ct
	}
	if ct, ok := r.Headers["content-type"]; ok {
		return ct
	}
	return ""
}

// GetAuthorization returns the Authorization header value, or empty string if not set
func (r *HTTPRequest) GetAuthorization() string {
	// Check both capitalization variants
	if auth, ok := r.Headers["Authorization"]; ok {
		return auth
	}
	if auth, ok := r.Headers["authorization"]; ok {
		return auth
	}
	return ""
}

// HasBody returns true if the request has a non-empty body
func (r *HTTPRequest) HasBody() bool {
	return r.Body != ""
}
