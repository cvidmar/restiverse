package vars

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestExtractRequestIncludesBodyInStableOrder(t *testing.T) {
	headers := map[string]string{
		"Z-Header": "{z}",
		"A-Header": "{a}",
	}
	want := []string{"url", "a", "z", "body"}
	for i := 0; i < 100; i++ {
		got := ExtractRequest("https://example.com/{url}", headers, `{"value":"{body}"}`)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("ExtractRequest = %v, want %v", got, want)
		}
	}
}

func TestSubstituteRequestReportsUnresolvedVariables(t *testing.T) {
	url := "https://{host}/api"
	headers := map[string]string{"Authorization": "Bearer {token}"}
	body := `{"id":"{id}"}`

	err := SubstituteRequest(&url, headers, &body, VarValues{"host": "example.com"})
	if err == nil {
		t.Fatal("SubstituteRequest: expected an error for unresolved variables")
	}
	if got := err.Error(); !strings.Contains(got, "token") || !strings.Contains(got, "id") {
		t.Errorf("error = %q, want it to name token and id", got)
	}
	// Nothing is substituted when a value is missing, so no half-built request is sent.
	if url != "https://{host}/api" {
		t.Errorf("url = %q, want it left untouched", url)
	}
}

func TestSubstituteRequestSubstitutesWhenAllResolved(t *testing.T) {
	url := "https://{host}/api"
	headers := map[string]string{"Authorization": "Bearer {token}"}
	body := `{"id":"{id}"}`

	values := VarValues{"host": "example.com", "token": "abc123", "id": "42"}
	if err := SubstituteRequest(&url, headers, &body, values); err != nil {
		t.Fatalf("SubstituteRequest: %v", err)
	}
	if url != "https://example.com/api" {
		t.Errorf("url = %q", url)
	}
	if headers["Authorization"] != "Bearer abc123" {
		t.Errorf("authorization = %q", headers["Authorization"])
	}
	if body != `{"id":"42"}` {
		t.Errorf("body = %q", body)
	}
}

func TestResolveValuesUsesDefaultsAndFillsPartialValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "request.http")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	defs := map[string][]string{"env": {"dev"}, "node": {"a"}}
	values, err := ResolveValues(path, defs, []string{"env", "node"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(values, VarValues{"env": "dev", "node": "a"}) {
		t.Fatalf("first-use values = %#v", values)
	}

	if err := SaveVariableValues(path, VarValues{"env": "prod"}); err != nil {
		t.Fatal(err)
	}
	values, err = ResolveValues(path, defs, []string{"env", "node"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(values, VarValues{"env": "prod", "node": "a"}) {
		t.Fatalf("partial values = %#v", values)
	}
}

func TestSaveVariableValuesUsesPrivatePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "request.http")
	if err := SaveVariableValues(path, VarValues{"token": "secret"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(filepath.Dir(path), "request.vars"))
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode = %o, want 600", got)
	}
}

func TestLoadVariableValuesDoesNotReadSiblingMetadata(t *testing.T) {
	dir := t.TempDir()
	responses := filepath.Join(dir, "responses")
	if err := os.Mkdir(responses, 0o755); err != nil {
		t.Fatal(err)
	}
	data := []byte("vars:\n  env: wrong\n")
	if err := os.WriteFile(filepath.Join(responses, "get_users_20260831_152955.meta"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	values, err := LoadVariableValues(filepath.Join(dir, "get.http"))
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 0 {
		t.Fatalf("loaded sibling values: %#v", values)
	}
}

func TestLoadVariableValuesFindsLegacyVarsBehindNewMetadata(t *testing.T) {
	dir := t.TempDir()
	responses := filepath.Join(dir, "responses")
	if err := os.Mkdir(responses, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(responses, "get_20260831_152954.meta"), []byte("vars:\n  env: prod\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(responses, "get_20260831_152955.meta"), []byte("status_code: 200\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	values, err := LoadVariableValues(filepath.Join(dir, "get.http"))
	if err != nil {
		t.Fatal(err)
	}
	if values["env"] != "prod" {
		t.Fatalf("legacy values = %#v", values)
	}
}
