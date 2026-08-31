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

	lineNum := 0
	var method, url string
	headers := make(map[string]string)
	var bodyLines []string
	inBody := false

	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSuffix(line, "\r")
		lineNum++

		// Skip comment lines (lines starting with #)
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}

		// First line: METHOD URL
		if lineNum == 1 || (method == "" && url == "") {
			parts := strings.Fields(line)
			if len(parts) < 2 {
				// Skip if not a valid method line yet
				continue
			}
			method = strings.ToUpper(parts[0])
			url = parts[1]
			continue
		}

		// Empty line signals start of body
		if trimmed == "" {
			if !inBody {
				inBody = true
				continue
			}
		}

		// If we're in the body, collect all remaining lines
		if inBody {
			bodyLines = append(bodyLines, line)
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
	body := strings.Join(bodyLines, "\n")

	return &HTTPRequest{
		Method:  method,
		URL:     url,
		Headers: headers,
		Body:    strings.TrimSpace(body),
	}, nil
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
