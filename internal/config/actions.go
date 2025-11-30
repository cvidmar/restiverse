package config

import (
	"fmt"
	"strings"
)

// FilterActions returns actions that are applicable for the given file types and count
func (c *Config) FilterActions(fileTypes []string, fileCount int) []Action {
	var applicable []Action

	for _, action := range c.Actions {
		if action.IsApplicable(fileTypes, fileCount) {
			applicable = append(applicable, action)
		}
	}

	return applicable
}

// IsApplicable checks if an action can be performed on the given files
func (a *Action) IsApplicable(fileTypes []string, fileCount int) bool {
	// Check file count constraints
	if fileCount < a.MinFiles {
		return false
	}

	if a.MaxFiles != nil && fileCount > *a.MaxFiles {
		return false
	}

	// Check if all selected file types are supported by this action
	for _, ft := range fileTypes {
		if !a.SupportsFileType(ft) {
			return false
		}
	}

	return true
}

// SupportsFileType checks if an action supports a given file type
func (a *Action) SupportsFileType(fileType string) bool {
	for _, ft := range a.FileTypes {
		if ft == fileType {
			return true
		}
	}
	return false
}

// BuildCommand builds the command string with placeholder substitution
func (a *Action) BuildCommand(filePaths []string) (string, error) {
	cmd := a.Command

	// Handle generic {filename} placeholder (for single file)
	if strings.Contains(cmd, "{filename}") {
		if len(filePaths) != 1 {
			return "", fmt.Errorf("action requires exactly 1 file, got %d", len(filePaths))
		}
		cmd = strings.ReplaceAll(cmd, "{filename}", filePaths[0])
	}

	// Handle numbered placeholders {filename1}, {filename2}, etc.
	for i, path := range filePaths {
		placeholder := fmt.Sprintf("{filename%d}", i+1)
		cmd = strings.ReplaceAll(cmd, placeholder, path)
	}

	return cmd, nil
}

// GetActionByKeybinding finds an action by its keybinding
func (c *Config) GetActionByKeybinding(key string) *Action {
	for i := range c.Actions {
		if c.Actions[i].Keybinding == key {
			return &c.Actions[i]
		}
	}
	return nil
}

// GetActionByName finds an action by its name
func (c *Config) GetActionByName(name string) *Action {
	for i := range c.Actions {
		if c.Actions[i].Name == name {
			return &c.Actions[i]
		}
	}
	return nil
}
