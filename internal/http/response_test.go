package http

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cvidmar/restiverse/internal/respfile"
	"gopkg.in/yaml.v3"
)

func TestSaveResponseMetadataRedactsSecretsAndUsesPrivatePermissions(t *testing.T) {
	dir := t.TempDir()
	httpPath := filepath.Join(dir, "get.http")
	record := respfile.NewRecord(httpPath, time.Date(2026, 8, 31, 15, 29, 55, 123000000, time.Local))
	if err := os.MkdirAll(filepath.Dir(record.MetaPath), 0o755); err != nil {
		t.Fatal(err)
	}
	req := &HTTPRequest{
		Method: "GET",
		URL:    "https://example.com?access_token=secret&visible=yes",
		Headers: map[string]string{
			"Authorization": "Bearer secret",
			"X-Api-Key":     "secret",
		},
	}
	resp := &Response{StatusCode: 200, Duration: time.Millisecond, Headers: map[string]string{"Set-Cookie": "session=secret", "Content-Type": "text/plain"}}
	opts := MetadataOptions{
		SensitiveHeaders:     []string{"authorization", "x-api-key", "set-cookie"},
		SensitiveQueryParams: []string{"access_token"},
	}
	if err := SaveResponseMetadata(record, req, resp, nil, opts); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(record.MetaPath)
	if err != nil {
		t.Fatal(err)
	}
	var meta ResponseMetadata
	if err := yaml.Unmarshal(data, &meta); err != nil {
		t.Fatal(err)
	}
	if meta.Request.Headers["Authorization"] != "Bearer ***" || meta.Request.Headers["X-Api-Key"] != "***" {
		t.Fatalf("request headers were not redacted: %#v", meta.Request.Headers)
	}
	if meta.Response.Headers["Set-Cookie"] != "***" {
		t.Fatalf("response headers were not redacted: %#v", meta.Response.Headers)
	}
	if meta.Request.URL != "https://example.com?access_token=%2A%2A%2A&visible=yes" {
		t.Fatalf("URL = %q", meta.Request.URL)
	}
	if meta.Vars != nil {
		t.Fatalf("new metadata leaked vars: %#v", meta.Vars)
	}
	info, err := os.Stat(record.MetaPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o", info.Mode().Perm())
	}
}

func TestSaveFailedResponseDoesNotLeaveBody(t *testing.T) {
	dir := t.TempDir()
	record := respfile.NewRecord(filepath.Join(dir, "get.http"), time.Now())
	if err := os.MkdirAll(filepath.Dir(record.BodyPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(record.BodyPath, []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	req := &HTTPRequest{Method: "GET", URL: "https://example.com", Headers: map[string]string{}}
	if err := SaveResponseMetadata(record, req, nil, errors.New("failed"), MetadataOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(record.BodyPath); !os.IsNotExist(err) {
		t.Fatalf("stale body still exists: %v", err)
	}
}

func TestCleanupDoesNotDeleteSiblingResponses(t *testing.T) {
	dir := t.TempDir()
	responses := filepath.Join(dir, "responses")
	if err := os.Mkdir(responses, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"get_20260831_152954.meta",
		"get_20260831_152955.meta",
		"get_users_20260831_152953.meta",
	} {
		if err := os.WriteFile(filepath.Join(responses, name), []byte("meta"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := CleanupOldResponses(filepath.Join(dir, "get.http"), 1); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(responses, "get_users_20260831_152953.meta")); err != nil {
		t.Fatalf("sibling response was deleted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(responses, "get_20260831_152954.meta")); !os.IsNotExist(err) {
		t.Fatalf("old response was not deleted: %v", err)
	}
}

func TestSameSecondTimestampsProduceDistinctRecords(t *testing.T) {
	dir := t.TempDir()
	httpPath := filepath.Join(dir, "get.http")
	first := respfile.NewRecord(httpPath, time.Date(2026, 8, 31, 15, 29, 55, 123000000, time.UTC))
	second := respfile.NewRecord(httpPath, time.Date(2026, 8, 31, 15, 29, 55, 456000000, time.UTC))
	if first.ID == second.ID {
		t.Fatalf("same-second records collided: %q", first.ID)
	}
	req := &HTTPRequest{Method: "GET", URL: "https://example.com", Headers: map[string]string{}}
	resp := &Response{StatusCode: 200, Headers: map[string]string{}}
	for _, record := range []respfile.Record{first, second} {
		if err := SaveResponseMetadata(record, req, resp, nil, MetadataOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(filepath.Join(dir, "responses"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("metadata files = %d, want 2", len(entries))
	}
}
