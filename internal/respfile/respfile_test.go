package respfile

import (
	"path/filepath"
	"testing"
	"time"
)

func TestMatchUsesExactHTTPBase(t *testing.T) {
	tests := []struct {
		name   string
		base   string
		wantID string
		wantOK bool
	}{
		{"get_users_20250101_120000.meta", "get", "", false},
		{"get_users_20250101_120000.meta", "get_users", "get_users_20250101_120000", true},
		{"get_20250101_120000.body", "get", "get_20250101_120000", true},
		{"get_20250101_120000_456.body", "get", "get_20250101_120000_456", true},
		{"my.api_20250101_120000.meta", "my.api", "my.api_20250101_120000", true},
		{"get_20251301_120000.meta", "get", "", false},
		{"get_20250101_120000.txt", "get", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name+"/"+tt.base, func(t *testing.T) {
			id, _, ok := Match(tt.name, tt.base)
			if ok != tt.wantOK || id != tt.wantID {
				t.Fatalf("Match(%q, %q) = (%q, %v), want (%q, %v)", tt.name, tt.base, id, ok, tt.wantID, tt.wantOK)
			}
		})
	}
}

func TestNewRecordUsesMillisecondsAndSharedStem(t *testing.T) {
	ts := time.Date(2026, 8, 31, 15, 29, 55, 456789000, time.Local)
	record := NewRecord(filepath.Join("tmp", "GET.HTTP"), ts)

	if record.ID != "GET_20260831_152955_456" {
		t.Fatalf("ID = %q", record.ID)
	}
	if filepath.Base(record.MetaPath) != record.ID+".meta" {
		t.Fatalf("MetaPath = %q", record.MetaPath)
	}
	if filepath.Base(record.BodyPath) != record.ID+".body" {
		t.Fatalf("BodyPath = %q", record.BodyPath)
	}
}

func TestBaseNameIsExtensionCaseInsensitive(t *testing.T) {
	if got := BaseName("a/b/GET.HTTP"); got != "GET" {
		t.Fatalf("BaseName = %q", got)
	}
}
