package http

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseHTTPFileLargeSingleLineBody(t *testing.T) {
	// A single body line larger than bufio.Scanner's 64KB default token limit.
	payload := `{"data":"` + strings.Repeat("x", 300*1024) + `"}`
	content := "POST https://example.com/api\nContent-Type: application/json\n\n" + payload + "\n"

	path := filepath.Join(t.TempDir(), "large.http")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	req, err := ParseHTTPFile(path)
	if err != nil {
		t.Fatalf("ParseHTTPFile: %v", err)
	}
	if req.Method != "POST" || req.URL != "https://example.com/api" {
		t.Errorf("got %s %s", req.Method, req.URL)
	}
	if req.GetContentType() != "application/json" {
		t.Errorf("content type = %q", req.GetContentType())
	}
	if req.Body != payload {
		t.Errorf("body length = %d, want %d", len(req.Body), len(payload))
	}
}

func TestParseHTTPFileCRLF(t *testing.T) {
	content := "POST https://example.com/api\r\nContent-Type: application/json\r\n\r\n{\r\n  \"a\": 1\r\n}\r\n"

	path := filepath.Join(t.TempDir(), "crlf.http")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	req, err := ParseHTTPFile(path)
	if err != nil {
		t.Fatalf("ParseHTTPFile: %v", err)
	}
	if req.Body != "{\n  \"a\": 1\n}" {
		t.Errorf("body = %q", req.Body)
	}
}
