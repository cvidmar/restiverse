package http

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseHTTPFileWithComments(t *testing.T) {
	// Create a temporary test file with comments
	content := `# This is a comment
# Another comment
GET https://api.example.com/users
# Comment in headers section
Accept: application/json
Authorization: Bearer token123
# Comment before body

`
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.http")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	req, err := ParseHTTPFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to parse file: %v", err)
	}

	if req.Method != "GET" {
		t.Errorf("Expected method GET, got %s", req.Method)
	}

	if req.URL != "https://api.example.com/users" {
		t.Errorf("Expected URL https://api.example.com/users, got %s", req.URL)
	}

	if len(req.Headers) != 2 {
		t.Errorf("Expected 2 headers, got %d", len(req.Headers))
	}

	if req.Headers["Accept"] != "application/json" {
		t.Errorf("Expected Accept header 'application/json', got %s", req.Headers["Accept"])
	}

	if req.Headers["Authorization"] != "Bearer token123" {
		t.Errorf("Expected Authorization header 'Bearer token123', got %s", req.Headers["Authorization"])
	}
}

func TestToCurlCommand(t *testing.T) {
	tests := []struct {
		name     string
		request  HTTPRequest
		expected []string // Lines that should appear in the output
	}{
		{
			name: "Simple GET request",
			request: HTTPRequest{
				Method: "GET",
				URL:    "https://api.example.com/users",
				Headers: map[string]string{
					"Accept": "application/json",
				},
				Body: "",
			},
			expected: []string{
				"curl \\",
				"-H 'Accept: application/json' \\",
				"'https://api.example.com/users'",
			},
		},
		{
			name: "POST request with body",
			request: HTTPRequest{
				Method: "POST",
				URL:    "https://api.example.com/users",
				Headers: map[string]string{
					"Content-Type": "application/json",
					"Accept":       "application/json",
				},
				Body: `{"name":"John","age":30}`,
			},
			expected: []string{
				"curl -X POST \\",
				"-H 'Content-Type: application/json' \\",
				"-H 'Accept: application/json' \\",
				"-d '{\"name\":\"John\",\"age\":30}' \\",
				"'https://api.example.com/users'",
			},
		},
		{
			name: "Request with Authorization header",
			request: HTTPRequest{
				Method: "GET",
				URL:    "https://api.example.com/users",
				Headers: map[string]string{
					"Authorization": "Bearer token123",
					"Accept":        "application/json",
				},
				Body: "",
			},
			expected: []string{
				"curl \\",
				"-H 'Authorization: Bearer token123' \\",
				"-H 'Accept: application/json' \\",
				"'https://api.example.com/users'",
			},
		},
		{
			name: "Request with single quote in header value",
			request: HTTPRequest{
				Method: "GET",
				URL:    "https://api.example.com/users",
				Headers: map[string]string{
					"X-Custom": "value's with quote",
				},
				Body: "",
			},
			expected: []string{
				"curl \\",
				"-H 'X-Custom: value'\\''s with quote' \\",
				"'https://api.example.com/users'",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.request.ToCurlCommand()

			// Check that result contains all expected lines
			for _, expectedLine := range tt.expected {
				if !strings.Contains(result, expectedLine) {
					t.Errorf("Expected output to contain:\n%s\n\nGot:\n%s", expectedLine, result)
				}
			}

			// Verify it has backslash line continuations (except last line)
			lines := strings.Split(result, "\n")
			for i := 0; i < len(lines)-1; i++ {
				if !strings.HasSuffix(lines[i], " \\") {
					t.Errorf("Line %d should end with backslash:\n%s", i, lines[i])
				}
			}

			// Verify last line doesn't have backslash
			lastLine := lines[len(lines)-1]
			if strings.HasSuffix(lastLine, "\\") {
				t.Errorf("Last line should not end with backslash:\n%s", lastLine)
			}
		})
	}
}
