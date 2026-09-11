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

func TestParseHTTPFilePreservesHashAndWhitespaceInBody(t *testing.T) {
	content := "# comment\nPOST https://example.com/api\nContent-Type: text/plain\n\n  leading\n# body data\ntrailing  \n"
	path := filepath.Join(t.TempDir(), "body.http")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	req, err := ParseHTTPFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "  leading\n# body data\ntrailing  "
	if req.Body != want {
		t.Fatalf("body = %q, want %q", req.Body, want)
	}
}

func TestParseHTTPFileRejectsHeadersPushedIntoBody(t *testing.T) {
	// A blank line directly after the request line makes every header below it body.
	content := "POST https://example.com/api\n\nAuthorization: Bearer token123\nContent-Type: application/json\n\n{\"a\":1}\n"
	path := filepath.Join(t.TempDir(), "blankline.http")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := ParseHTTPFile(path)
	if err == nil {
		t.Fatal("ParseHTTPFile: expected an error for headers swallowed into the body")
	}
	if !strings.Contains(err.Error(), "blank line") {
		t.Errorf("error = %q, want it to mention the blank line", err)
	}
}

func TestParseHTTPFileAllowsHeaderlessBody(t *testing.T) {
	// A request with no headers is legal; only a header-shaped first line is rejected.
	content := "POST https://example.com/api\n\n{\"a\":1}\n"
	path := filepath.Join(t.TempDir(), "headerless.http")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	req, err := ParseHTTPFile(path)
	if err != nil {
		t.Fatalf("ParseHTTPFile: %v", err)
	}
	if req.Body != `{"a":1}` {
		t.Errorf("body = %q", req.Body)
	}
}

func TestParseRequestLine(t *testing.T) {
	method, url, ok := ParseRequestLine("  get https://example.com  ")
	if !ok || method != "GET" || url != "https://example.com" {
		t.Fatalf("ParseRequestLine = %q %q %v", method, url, ok)
	}
}
