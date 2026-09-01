package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	varsPkg "github.com/cvidmar/restiverse/internal/vars"
	"gopkg.in/yaml.v3"
)

// Config represents the complete configuration for Restiverse
type Config struct {
	Timeout              time.Duration       `yaml:"timeout"`
	Editor               string              `yaml:"editor"`
	MaxResponses         int                 `yaml:"max_responses,omitempty"` // Max response files to keep per .http file (0 = unlimited)
	Actions              []Action            `yaml:"actions"`
	Vars                 map[string][]string `yaml:"vars,omitempty"` // Custom variables for URL substitution
	SensitiveHeaders     []string            `yaml:"sensitive_headers,omitempty"`
	SensitiveQueryParams []string            `yaml:"sensitive_query_params,omitempty"`
}

// Action represents a configurable action that can be performed on files
type Action struct {
	Name       string   `yaml:"name"`
	Command    string   `yaml:"command"`
	Keybinding string   `yaml:"keybinding,omitempty"`
	MinFiles   int      `yaml:"min_files"`
	MaxFiles   *int     `yaml:"max_files"` // nil means unlimited
	FileTypes  []string `yaml:"file_types"`
}

// LoadConfig loads configuration from a restiverse.yaml file
// If the file doesn't exist, it creates a default configuration
func LoadConfig(dir string) (*Config, error) {
	configPath := filepath.Join(dir, "restiverse.yaml")

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Create default config
		config := DefaultConfig()
		if err := SaveConfig(configPath, config); err != nil {
			return nil, fmt.Errorf("failed to create default config: %w", err)
		}
		config.ExpandEnvVars()
		return config, nil
	}

	// Read existing config
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Validate config
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// Expand environment variables
	config.ExpandEnvVars()

	return &config, nil
}

// DefaultConfig returns a sensible default configuration
func DefaultConfig() *Config {
	maxOne := 1

	return &Config{
		Timeout:      30 * time.Second,
		Editor:       "$EDITOR",
		MaxResponses: 5, // Keep last 5 responses by default
		Actions: []Action{
			{
				Name:       "Execute Request",
				Command:    "internal:execute",
				Keybinding: "r",
				MinFiles:   1,
				MaxFiles:   &maxOne,
				FileTypes:  []string{"http"},
			},
			{
				Name:       "Edit File",
				Command:    "$EDITOR {filename}",
				Keybinding: "e",
				MinFiles:   1,
				MaxFiles:   &maxOne,
				FileTypes:  []string{"http", "body", "meta"},
			},
			{
				Name:       "View Output History",
				Command:    "internal:history",
				Keybinding: "h",
				MinFiles:   1,
				MaxFiles:   &maxOne,
				FileTypes:  []string{"http"},
			},
			{
				Name:      "Copy as curl",
				Command:   "internal:copy-as-curl",
				MinFiles:  1,
				MaxFiles:  &maxOne,
				FileTypes: []string{"http"},
			},
			{
				Name:       "Rename File",
				Command:    "internal:rename",
				Keybinding: "R",
				MinFiles:   1,
				MaxFiles:   &maxOne,
				FileTypes:  []string{"http"},
			},
			{
				Name:       "Duplicate File",
				Command:    "internal:duplicate",
				Keybinding: "D",
				MinFiles:   1,
				MaxFiles:   &maxOne,
				FileTypes:  []string{"http"},
			},
			{
				Name:       "Delete File",
				Command:    "internal:delete",
				Keybinding: "X",
				MinFiles:   1,
				MaxFiles:   &maxOne,
				FileTypes:  []string{"http"},
			},
			{
				Name:      "View Body",
				Command:   "less {filename}",
				MinFiles:  1,
				MaxFiles:  &maxOne,
				FileTypes: []string{"body"},
			},
			{
				Name:      "View Meta",
				Command:   "less {filename}",
				MinFiles:  1,
				MaxFiles:  &maxOne,
				FileTypes: []string{"meta"},
			},
			{
				Name:      "Delete Response",
				Command:   "rm {filename}",
				MinFiles:  1,
				MaxFiles:  nil, // unlimited
				FileTypes: []string{"body", "meta"},
			},
		},
	}
}

// SaveConfig saves a configuration to a file
func SaveConfig(path string, config *Config) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}

	if c.MaxResponses < 0 {
		return fmt.Errorf("max_responses cannot be negative (use 0 for unlimited)")
	}

	varNames := make([]string, 0, len(c.Vars))
	for name := range c.Vars {
		varNames = append(varNames, name)
	}
	sort.Strings(varNames)
	for _, name := range varNames {
		if !varsPkg.ValidName(name) {
			return fmt.Errorf("invalid variable name %q: must match [a-zA-Z_][a-zA-Z0-9_]*", name)
		}
		if len(c.Vars[name]) == 0 {
			return fmt.Errorf("variable %q has no values", name)
		}
	}

	// Check for keybinding conflicts
	keybindings := make(map[string]string)
	for _, action := range c.Actions {
		if action.Keybinding != "" {
			if existing, exists := keybindings[action.Keybinding]; exists {
				return fmt.Errorf("keybinding conflict: '%s' used by both '%s' and '%s'",
					action.Keybinding, existing, action.Name)
			}
			keybindings[action.Keybinding] = action.Name
		}

		// Validate action
		if err := action.Validate(); err != nil {
			return fmt.Errorf("invalid action '%s': %w", action.Name, err)
		}
	}

	return nil
}

// Validate checks if an action is valid
func (a *Action) Validate() error {
	if a.Name == "" {
		return fmt.Errorf("action name cannot be empty")
	}

	if a.Command == "" {
		return fmt.Errorf("action command cannot be empty")
	}

	if a.MinFiles < 0 {
		return fmt.Errorf("min_files cannot be negative")
	}

	if a.MaxFiles != nil && *a.MaxFiles < a.MinFiles {
		return fmt.Errorf("max_files (%d) cannot be less than min_files (%d)", *a.MaxFiles, a.MinFiles)
	}

	if len(a.FileTypes) == 0 {
		return fmt.Errorf("file_types cannot be empty")
	}

	// Validate file types
	validTypes := map[string]bool{"http": true, "body": true, "meta": true}
	for _, ft := range a.FileTypes {
		if !validTypes[ft] {
			return fmt.Errorf("invalid file type: %s (must be http, body, or meta)", ft)
		}
	}

	return nil
}

var editorTokenPattern = regexp.MustCompile(`\$(?:\{EDITOR\}|EDITOR\b)`)

// ExpandEnvVars expands the editor setting while preserving shell variables in actions.
func (c *Config) ExpandEnvVars() {
	c.Editor = os.ExpandEnv(c.Editor)

	// If editor is still empty after expansion, use vi as fallback
	if c.Editor == "" {
		c.Editor = "vi"
	}

	for i := range c.Actions {
		c.Actions[i].Command = editorTokenPattern.ReplaceAllStringFunc(c.Actions[i].Command, func(string) string {
			return c.Editor
		})
	}
}

// IsInternal checks if an action is an internal command
func (a *Action) IsInternal() bool {
	return strings.HasPrefix(a.Command, "internal:")
}

// GetInternalCommand returns the internal command name (without "internal:" prefix)
func (a *Action) GetInternalCommand() string {
	return strings.TrimPrefix(a.Command, "internal:")
}

var defaultSensitiveHeaders = []string{
	"authorization", "proxy-authorization", "cookie", "set-cookie",
	"x-api-key", "x-auth-token", "api-key",
}

var defaultSensitiveQueryParams = []string{
	"access_token", "token", "api_key", "key", "signature",
}

// EffectiveSensitiveHeaders returns the non-removable defaults plus configured names.
func (c *Config) EffectiveSensitiveHeaders() []string {
	return mergeNames(defaultSensitiveHeaders, c.SensitiveHeaders)
}

// EffectiveSensitiveQueryParams returns the non-removable defaults plus configured names.
func (c *Config) EffectiveSensitiveQueryParams() []string {
	return mergeNames(defaultSensitiveQueryParams, c.SensitiveQueryParams)
}

func mergeNames(defaults, configured []string) []string {
	seen := make(map[string]bool, len(defaults)+len(configured))
	merged := make([]string, 0, len(defaults)+len(configured))
	for _, names := range [][]string{defaults, configured} {
		for _, name := range names {
			name = strings.ToLower(strings.TrimSpace(name))
			if name != "" && !seen[name] {
				seen[name] = true
				merged = append(merged, name)
			}
		}
	}
	return merged
}
