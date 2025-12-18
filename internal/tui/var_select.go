package tui

import (
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cvidmar/restiverse/internal/files"
	httpPkg "github.com/cvidmar/restiverse/internal/http"
	"github.com/cvidmar/restiverse/internal/vars"
)

// showVariableSelection shows the variable selection view for a .http file
func (m model) showVariableSelection(entry files.FileEntry) (model, tea.Cmd) {
	// Read the .http file to extract variables
	req, err := httpPkg.ParseHTTPFile(entry.Path)
	if err != nil {
		m.errorMessage = "Failed to parse HTTP file: " + err.Error()
		return m, nil
	}

	// Extract variables from URL and headers
	varNames := vars.ExtractVariables(req.URL)
	for _, headerVal := range req.Headers {
		headerVars := vars.ExtractVariables(headerVal)
		for _, v := range headerVars {
			// Add if not already in list
			found := false
			for _, existing := range varNames {
				if existing == v {
					found = true
					break
				}
			}
			if !found {
				varNames = append(varNames, v)
			}
		}
	}

	// Check if there are any variables
	if len(varNames) == 0 {
		m.statusMessage = "No variables found in this request"
		return m, nil
	}

	// Check if all variables are defined in config
	for _, varName := range varNames {
		if _, ok := m.config.Vars[varName]; !ok {
			m.errorMessage = "Variable '" + varName + "' not defined in restiverse.yaml"
			return m, nil
		}
	}

	// Load current variable values from most recent .meta file or use defaults
	currentValues, _ := vars.LoadVariableValues(entry.Path)
	defaultValues := vars.GetDefaultValues(m.config.Vars, varNames)

	// Merge loaded values with defaults (loaded values take precedence)
	for name, value := range defaultValues {
		if _, ok := currentValues[name]; !ok {
			currentValues[name] = value
		}
	}

	// Enter variable selection view
	m.previousView = m.currentView
	m.currentView = ViewVariableSelect
	m.varSelectHTTPFile = entry.Path
	m.varNames = varNames
	m.varValues = currentValues
	m.varDefinitions = m.config.Vars
	m.varCurrentIdx = 0
	m.varOptionCursor = 0

	// Set cursor to current value's index
	if currentVal, ok := m.varValues[varNames[0]]; ok {
		if options, ok := m.varDefinitions[varNames[0]]; ok {
			for i, opt := range options {
				if opt == currentVal {
					m.varOptionCursor = i
					break
				}
			}
		}
	}

	return m, nil
}

// updateVariableSelect handles key presses in the variable selection view
func (m model) updateVariableSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if len(m.varNames) == 0 {
		return m, nil
	}

	currentVar := m.varNames[m.varCurrentIdx]
	options := m.varDefinitions[currentVar]

	// Navigation within options
	if matches(msg, m.keys.Up) {
		if m.varOptionCursor > 0 {
			m.varOptionCursor--
		}
		return m, nil
	}

	if matches(msg, m.keys.Down) {
		if m.varOptionCursor < len(options)-1 {
			m.varOptionCursor++
		}
		return m, nil
	}

	// Select current option and move to next variable or finish
	if matches(msg, m.keys.Enter) {
		// Save the selected value
		m.varValues[currentVar] = options[m.varOptionCursor]

		// Move to next variable
		if m.varCurrentIdx < len(m.varNames)-1 {
			m.varCurrentIdx++
			m.varOptionCursor = 0

			// Set cursor to current value's index for the new variable
			nextVar := m.varNames[m.varCurrentIdx]
			if currentVal, ok := m.varValues[nextVar]; ok {
				if nextOptions, ok := m.varDefinitions[nextVar]; ok {
					for i, opt := range nextOptions {
						if opt == currentVal {
							m.varOptionCursor = i
							break
						}
					}
				}
			}
		} else {
			// All variables configured, save them and return to previous view
			return m.saveVariableValues()
		}
		return m, nil
	}

	return m, nil
}

// saveVariableValues saves the configured variable values
func (m model) saveVariableValues() (model, tea.Cmd) {
	m.currentView = m.previousView
	return m, m.saveVariableValuesCmd(m.varSelectHTTPFile, m.varValues)
}

// saveVariableValuesCmd returns a command that saves variable values
func (m model) saveVariableValuesCmd(httpFilePath string, varValues vars.VarValues) tea.Cmd {
	return func() tea.Msg {
		err := vars.SaveVariableValues(httpFilePath, varValues)
		if err != nil {
			return errMsg{err}
		}
		// Reload directory to update the URL display
		entries, err := files.ListDirectory(m.currentPath)
		if err != nil {
			return errMsg{err}
		}
		return directoryLoadedMsg{entries}
	}
}

// checkFileHasVariables checks if a .http file contains any variables
func checkFileHasVariables(filePath string) bool {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return false
	}

	varNames := vars.ExtractVariables(string(content))
	return len(varNames) > 0
}
