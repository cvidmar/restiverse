package config

import (
	"strings"
	"testing"
	"time"
)

func TestValidateRejectsEmptyVarList(t *testing.T) {
	cfg := &Config{Timeout: time.Second, Vars: map[string][]string{"env": {}}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate accepted an empty variable option list")
	}
}

func TestValidateAllowsEmptyVarValue(t *testing.T) {
	cfg := &Config{Timeout: time.Second, Vars: map[string][]string{"optional": {""}}}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate rejected a legitimate empty value: %v", err)
	}
}

func TestExpandEnvVarsPreservesShellVariables(t *testing.T) {
	t.Setenv("EDITOR", "code -w")
	cfg := &Config{
		Editor: "$EDITOR",
		Actions: []Action{{
			Name:      "test",
			Command:   `awk '{print $1}' "$PATTERN" "$EDITORIAL" "$EDITOR_HOME" $EDITOR ${EDITOR}`,
			FileTypes: []string{"body"},
		}},
	}

	cfg.ExpandEnvVars()
	want := `awk '{print $1}' "$PATTERN" "$EDITORIAL" "$EDITOR_HOME" code -w code -w`
	if got := cfg.Actions[0].Command; got != want {
		t.Fatalf("command = %q, want %q", got, want)
	}
}

func TestSensitiveDefaultsCannotBeRemoved(t *testing.T) {
	cfg := &Config{
		SensitiveHeaders:     []string{"X-Custom-Secret", "Authorization"},
		SensitiveQueryParams: []string{"nonce"},
	}
	headers := cfg.EffectiveSensitiveHeaders()
	queries := cfg.EffectiveSensitiveQueryParams()
	for _, want := range []string{"authorization", "cookie", "x-custom-secret"} {
		if !contains(headers, want) {
			t.Errorf("sensitive headers %v missing %q", headers, want)
		}
	}
	for _, want := range []string{"access_token", "nonce"} {
		if !contains(queries, want) {
			t.Errorf("sensitive query params %v missing %q", queries, want)
		}
	}
}

func TestLoadConfigFirstRunExpandsEditor(t *testing.T) {
	t.Setenv("EDITOR", "")
	cfg, err := LoadConfig(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Editor != "vi" {
		t.Fatalf("editor = %q, want vi", cfg.Editor)
	}
	for _, action := range cfg.Actions {
		if action.Name == "Edit File" && !strings.HasPrefix(action.Command, "vi ") {
			t.Fatalf("edit command = %q", action.Command)
		}
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
