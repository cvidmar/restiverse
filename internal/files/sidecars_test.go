package files

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRenameRequestArtifactsMovesVarsAndResponses(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "get.http")
	newPath := filepath.Join(dir, "list.http")
	mustWrite(t, oldPath, "GET https://example.com")
	mustWrite(t, filepath.Join(dir, "get.vars"), "vars:\n  env: prod\n")
	mustWrite(t, filepath.Join(dir, "responses", "get_20260831_152955_123.meta"), "meta")
	mustWrite(t, filepath.Join(dir, "responses", "get_20260831_152955_123.body"), "body")
	mustWrite(t, filepath.Join(dir, "responses", "get_users_20260831_152955.meta"), "sibling")

	if err := RenameRequestArtifacts(oldPath, newPath); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		newPath,
		filepath.Join(dir, "list.vars"),
		filepath.Join(dir, "responses", "list_20260831_152955_123.meta"),
		filepath.Join(dir, "responses", "list_20260831_152955_123.body"),
		filepath.Join(dir, "responses", "get_users_20260831_152955.meta"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected %s: %v", path, err)
		}
	}
	for _, path := range []string{oldPath, filepath.Join(dir, "get.vars")} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("old artifact remains at %s: %v", path, err)
		}
	}
}

func TestRenameRequestArtifactsCollisionDoesNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "get.http")
	newPath := filepath.Join(dir, "list.http")
	mustWrite(t, oldPath, "request")
	mustWrite(t, filepath.Join(dir, "get.vars"), "old vars")
	mustWrite(t, filepath.Join(dir, "list.vars"), "orphan vars")

	if err := RenameRequestArtifacts(oldPath, newPath); err == nil {
		t.Fatal("rename unexpectedly overwrote a sidecar")
	}
	data, err := os.ReadFile(filepath.Join(dir, "list.vars"))
	if err != nil || string(data) != "orphan vars" {
		t.Fatalf("target sidecar changed: %q, %v", data, err)
	}
	if _, err := os.Stat(oldPath); err != nil {
		t.Fatalf("source request moved after failed preflight: %v", err)
	}
}

func TestDeleteRequestArtifactsRemovesOwnedSidecarsOnly(t *testing.T) {
	dir := t.TempDir()
	httpPath := filepath.Join(dir, "get.http")
	mustWrite(t, httpPath, "request")
	mustWrite(t, filepath.Join(dir, "get.vars"), "vars")
	mustWrite(t, filepath.Join(dir, "responses", "get_20260831_152955.meta"), "meta")
	mustWrite(t, filepath.Join(dir, "responses", "get_users_20260831_152955.meta"), "sibling")

	if err := DeleteRequestArtifacts(httpPath); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{httpPath, filepath.Join(dir, "get.vars"), filepath.Join(dir, "responses", "get_20260831_152955.meta")} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("artifact remains at %s: %v", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "responses", "get_users_20260831_152955.meta")); err != nil {
		t.Fatalf("sibling response was deleted: %v", err)
	}
}

func TestCopyVariableSidecarCopiesVarsOnly(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "get.http")
	newPath := filepath.Join(dir, "copy.http")
	mustWrite(t, filepath.Join(dir, "get.vars"), "vars")
	mustWrite(t, filepath.Join(dir, "responses", "get_20260831_152955.meta"), "meta")

	if err := CopyVariableSidecar(oldPath, newPath); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(dir, "copy.vars")); err != nil || string(data) != "vars" {
		t.Fatalf("copied vars = %q, %v", data, err)
	}
	if matches, err := filepath.Glob(filepath.Join(dir, "responses", "copy_*")); err != nil || len(matches) != 0 {
		t.Fatalf("response history was copied: %v, %v", matches, err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
