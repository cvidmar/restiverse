package tui

import "github.com/charmbracelet/bubbletea"

// KeyMap holds all the key mappings for the application
type KeyMap struct {
	Up        []string
	Down      []string
	Enter     []string
	Back      []string
	Space     []string
	FuzzyFind []string
	Quit      []string
	Cancel    []string
}

// DefaultKeyMap returns the default key mappings
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up:        []string{"up", "k"},
		Down:      []string{"down", "j"},
		Enter:     []string{"enter"},
		Back:      []string{"backspace"},
		Space:     []string{" ", "insert"},
		FuzzyFind: []string{"/"},
		Quit:      []string{"q"},
		Cancel:    []string{"esc"},
	}
}

// Matches checks if the key message matches any of the keys in the list
func matches(msg tea.KeyMsg, keys []string) bool {
	for _, key := range keys {
		if msg.String() == key {
			return true
		}
	}
	return false
}
