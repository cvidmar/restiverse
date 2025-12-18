package vars

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// VarValues represents the current values for variables
type VarValues map[string]string

// VarValuesStore represents the variable values stored in .meta files
type VarValuesStore struct {
	Vars VarValues `yaml:"vars,omitempty"`
}

var varPattern = regexp.MustCompile(`\{([a-zA-Z_][a-zA-Z0-9_]*)\}`)

// ExtractVariables finds all variable placeholders in a string
func ExtractVariables(text string) []string {
	matches := varPattern.FindAllStringSubmatch(text, -1)
	varSet := make(map[string]bool)
	var vars []string

	for _, match := range matches {
		if len(match) > 1 {
			varName := match[1]
			if !varSet[varName] {
				varSet[varName] = true
				vars = append(vars, varName)
			}
		}
	}

	return vars
}

// SubstituteVariables replaces variable placeholders with their values
func SubstituteVariables(text string, values VarValues) string {
	return varPattern.ReplaceAllStringFunc(text, func(match string) string {
		varName := match[1 : len(match)-1] // Remove { and }
		if value, ok := values[varName]; ok {
			return value
		}
		return match // Keep the placeholder if no value found
	})
}

// GetDefaultValues returns the first value for each variable from config
func GetDefaultValues(varDefs map[string][]string, varNames []string) VarValues {
	values := make(VarValues)
	for _, name := range varNames {
		if vals, ok := varDefs[name]; ok && len(vals) > 0 {
			values[name] = vals[0]
		}
	}
	return values
}

// LoadVariableValues loads variable values from user configuration or most recent .meta file
func LoadVariableValues(httpFilePath string) (VarValues, error) {
	// First, try to load from user-configured vars file
	varFilePath := getVarFilePath(httpFilePath)
	if _, err := os.Stat(varFilePath); err == nil {
		data, err := os.ReadFile(varFilePath)
		if err == nil {
			var store VarValuesStore
			if err := yaml.Unmarshal(data, &store); err == nil && store.Vars != nil {
				return store.Vars, nil
			}
		}
	}

	// Fallback: load from most recent .meta file
	dir := filepath.Dir(httpFilePath)
	baseName := strings.TrimSuffix(filepath.Base(httpFilePath), ".http")

	// Check for responses directory
	responsesDir := filepath.Join(dir, "responses")
	if _, err := os.Stat(responsesDir); os.IsNotExist(err) {
		return VarValues{}, nil // No responses yet
	}

	entries, err := os.ReadDir(responsesDir)
	if err != nil {
		return VarValues{}, fmt.Errorf("failed to read responses directory: %w", err)
	}

	// Find the most recent .meta file
	var latestMeta string
	var latestTime int64 = 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasPrefix(name, baseName+"_") || !strings.HasSuffix(name, ".meta") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Unix() > latestTime {
			latestTime = info.ModTime().Unix()
			latestMeta = filepath.Join(responsesDir, name)
		}
	}

	// No meta file found
	if latestMeta == "" {
		return VarValues{}, nil
	}

	// Read and parse the meta file
	data, err := os.ReadFile(latestMeta)
	if err != nil {
		return VarValues{}, fmt.Errorf("failed to read meta file: %w", err)
	}

	// Try to parse vars from meta file
	var store VarValuesStore
	if err := yaml.Unmarshal(data, &store); err != nil {
		return VarValues{}, nil // If parsing fails, return empty values
	}

	if store.Vars == nil {
		return VarValues{}, nil
	}

	return store.Vars, nil
}

// SaveVariableValues saves variable values for an HTTP file
func SaveVariableValues(httpFilePath string, values VarValues) error {
	varFilePath := getVarFilePath(httpFilePath)

	// Ensure directory exists
	dir := filepath.Dir(varFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	store := VarValuesStore{Vars: values}
	data, err := yaml.Marshal(store)
	if err != nil {
		return fmt.Errorf("failed to marshal variable values: %w", err)
	}

	if err := os.WriteFile(varFilePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write variable values: %w", err)
	}

	return nil
}

// getVarFilePath returns the path to the variable values file for an HTTP file
func getVarFilePath(httpFilePath string) string {
	dir := filepath.Dir(httpFilePath)
	baseName := strings.TrimSuffix(filepath.Base(httpFilePath), ".http")
	return filepath.Join(dir, baseName+".vars")
}

// LoadDefaultValuesFromConfig loads default values from the config file
func LoadDefaultValuesFromConfig(httpFilePath string, varNames []string) (VarValues, error) {
	// Find the restiverse.yaml config file
	dir := filepath.Dir(httpFilePath)
	configPath := findConfigFile(dir)
	if configPath == "" {
		return VarValues{}, nil
	}

	// Read the config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return VarValues{}, nil
	}

	// Parse the config to extract vars
	var config struct {
		Vars map[string][]string `yaml:"vars"`
	}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return VarValues{}, nil
	}

	// Get default values
	return GetDefaultValues(config.Vars, varNames), nil
}

// findConfigFile searches for restiverse.yaml starting from the given directory
func findConfigFile(startDir string) string {
	currentDir := startDir
	for {
		configPath := filepath.Join(currentDir, "restiverse.yaml")
		if _, err := os.Stat(configPath); err == nil {
			return configPath
		}

		// Move up one directory
		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			// Reached the root
			break
		}
		currentDir = parentDir
	}
	return ""
}

// FormatURLWithVarValues formats a URL showing current variable values
// e.g., https://srv-{node:b}.{environ:prod}.example.com/api
func FormatURLWithVarValues(url string, values VarValues) string {
	return varPattern.ReplaceAllStringFunc(url, func(match string) string {
		varName := match[1 : len(match)-1] // Remove { and }
		if value, ok := values[varName]; ok {
			return fmt.Sprintf("{%s:%s}", varName, value)
		}
		return match
	})
}
