package files

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListResponseHistoryDoesNotClaimSiblingResponses(t *testing.T) {
	dir := t.TempDir()
	responses := filepath.Join(dir, "responses")
	if err := os.Mkdir(responses, 0o755); err != nil {
		t.Fatal(err)
	}
	meta := "timestamp: 2026-08-31T15:29:55Z\nstatus_code: 200\nduration_ms: 1\nrequest:\n  method: GET\n  url: https://example.com\n  headers: {}\n"
	if err := os.WriteFile(filepath.Join(responses, "get_users_20260831_152955.meta"), []byte(meta), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := ListResponseHistory(filepath.Join(dir, "get.http"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("history contained sibling responses: %#v", got)
	}
}

func TestListResponseHistoryDoesNotTreatNestedErrorHeaderAsFailure(t *testing.T) {
	dir := t.TempDir()
	responses := filepath.Join(dir, "responses")
	if err := os.Mkdir(responses, 0o755); err != nil {
		t.Fatal(err)
	}
	meta := "timestamp: 2026-08-31T15:29:55Z\nstatus_code: 200\nduration_ms: 1\nrequest:\n  method: GET\n  url: https://example.com\n  headers:\n    error: harmless\n"
	if err := os.WriteFile(filepath.Join(responses, "get_20260831_152955.meta"), []byte(meta), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := ListResponseHistory(filepath.Join(dir, "get.http"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].HasError || got[0].StatusCode != 200 {
		t.Fatalf("history = %#v", got)
	}
}

func TestMalformedMetaRemainsVisible(t *testing.T) {
	dir := t.TempDir()
	responses := filepath.Join(dir, "responses")
	if err := os.Mkdir(responses, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(responses, "get_20260831_152955.meta"), []byte("timestamp: ["), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := ListResponseHistory(filepath.Join(dir, "get.http"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !got[0].HasError {
		t.Fatalf("history = %#v", got)
	}
}

func TestGetFileTypeIsCaseInsensitive(t *testing.T) {
	for name, want := range map[string]FileType{
		"a.HTTP": FileTypeHTTP,
		"a.Body": FileTypeBody,
		"a.META": FileTypeMeta,
		"a.txt":  "",
	} {
		if got := GetFileType(name); got != want {
			t.Errorf("GetFileType(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestExtractHTTPInfoSkipsLeadingComments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "get.http")
	content := "# fetch users\n\nGET https://example.com/users\nAccept: application/json\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	method, url := extractHTTPInfo(path)
	if method != "GET" || url != "https://example.com/users" {
		t.Fatalf("method=%q url=%q", method, url)
	}
}
