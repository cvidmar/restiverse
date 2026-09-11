package vars

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/cvidmar/restiverse/internal/respfile"
	"gopkg.in/yaml.v3"
)

// VarValues represents the current values for variables
type VarValues map[string]string

// VarValuesStore represents the variable values stored in .meta files
type VarValuesStore struct {
	Vars VarValues `yaml:"vars,omitempty"`
}

var (
	varPattern  = regexp.MustCompile(`\{([a-zA-Z_][a-zA-Z0-9_]*)\}`)
	namePattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
)

// ValidName reports whether name can be used in a variable placeholder.
func ValidName(name string) bool {
	return namePattern.MatchString(name)
}

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

// ExtractRequest returns distinct request variables in a stable order.
func ExtractRequest(url string, headers map[string]string, body string) []string {
	seen := make(map[string]bool)
	var names []string
	add := func(text string) {
		for _, name := range ExtractVariables(text) {
			if !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
	}

	add(url)
	headerNames := make([]string, 0, len(headers))
	for name := range headers {
		headerNames = append(headerNames, name)
	}
	sort.Strings(headerNames)
	for _, name := range headerNames {
		add(headers[name])
	}
	add(body)
	return names
}

// SubstituteRequest replaces variables throughout a parsed request. A placeholder with no
// value is reported instead of being sent verbatim: an unresolved key or host reaches the
// server as literal braces and comes back as an opaque 401 or 404.
func SubstituteRequest(url *string, headers map[string]string, body *string, values VarValues) error {
	var bodyText string
	if body != nil {
		bodyText = *body
	}

	var missing []string
	for _, name := range ExtractRequest(*url, headers, bodyText) {
		if _, ok := values[name]; !ok {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("unresolved variable(s): %s", strings.Join(missing, ", "))
	}

	*url = SubstituteVariables(*url, values)
	for name, value := range headers {
		headers[name] = SubstituteVariables(value, values)
	}
	if body != nil {
		*body = SubstituteVariables(*body, values)
	}
	return nil
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
	baseName := respfile.BaseName(httpFilePath)

	// Check for responses directory
	responsesDir := filepath.Join(dir, "responses")
	if _, err := os.Stat(responsesDir); os.IsNotExist(err) {
		return VarValues{}, nil // No responses yet
	}

	entries, err := os.ReadDir(responsesDir)
	if err != nil {
		return VarValues{}, fmt.Errorf("failed to read responses directory: %w", err)
	}

	type metaFile struct {
		path      string
		timestamp time.Time
	}
	var metaFiles []metaFile

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if filepath.Ext(name) != ".meta" {
			continue
		}
		_, timestamp, ok := respfile.Match(name, baseName)
		if !ok {
			continue
		}
		metaFiles = append(metaFiles, metaFile{
			path:      filepath.Join(responsesDir, name),
			timestamp: timestamp,
		})
	}

	if len(metaFiles) == 0 {
		return VarValues{}, nil
	}
	sort.Slice(metaFiles, func(i, j int) bool {
		return metaFiles[i].timestamp.After(metaFiles[j].timestamp)
	})

	// New metadata omits Vars. Walk backwards until a legacy record with saved
	// values is found so a mixed old/new history remains compatible.
	for _, meta := range metaFiles {
		data, err := os.ReadFile(meta.path)
		if err != nil {
			return VarValues{}, fmt.Errorf("failed to read meta file: %w", err)
		}
		var store VarValuesStore
		if err := yaml.Unmarshal(data, &store); err != nil {
			continue
		}
		if len(store.Vars) > 0 {
			return store.Vars, nil
		}
	}

	return VarValues{}, nil
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

	if err := writePrivateFile(varFilePath, data); err != nil {
		return fmt.Errorf("failed to write variable values: %w", err)
	}

	return nil
}

// ResolveValues loads saved values and fills missing entries from definitions.
func ResolveValues(httpFilePath string, definitions map[string][]string, names []string) (VarValues, error) {
	values, err := LoadVariableValues(httpFilePath)
	if err != nil {
		return nil, err
	}
	if values == nil {
		values = make(VarValues)
	}
	for name, value := range GetDefaultValues(definitions, names) {
		if _, exists := values[name]; !exists {
			values[name] = value
		}
	}
	return values, nil
}

// getVarFilePath returns the path to the variable values file for an HTTP file
func getVarFilePath(httpFilePath string) string {
	dir := filepath.Dir(httpFilePath)
	baseName := respfile.BaseName(httpFilePath)
	return filepath.Join(dir, baseName+".vars")
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

func writePrivateFile(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := file.Chmod(0o600); err != nil {
		return err
	}
	_, err = file.Write(data)
	return err
}
