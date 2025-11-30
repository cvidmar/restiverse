package tui

import "github.com/charmbracelet/lipgloss"

// Styles holds all the lipgloss styles for the TUI
type Styles struct {
	TopBar       lipgloss.Style
	StatusBar    lipgloss.Style
	Content      lipgloss.Style

	Directory    lipgloss.Style
	File         lipgloss.Style
	SelectedItem lipgloss.Style
	MarkedItem   lipgloss.Style

	Modal        lipgloss.Style
	ModalTitle   lipgloss.Style
	ModalItem    lipgloss.Style
	ModalSelected lipgloss.Style

	Table        lipgloss.Style
	TableHeader  lipgloss.Style
	TableRow     lipgloss.Style
	TableRowSelected lipgloss.Style

	ErrorMsg     lipgloss.Style
	SuccessMsg   lipgloss.Style
	InfoMsg      lipgloss.Style

	HelpText     lipgloss.Style
}

// DefaultStyles returns the default styling for the TUI
// Designed to work with both dark and light terminal backgrounds
func DefaultStyles() Styles {
	return Styles{
		TopBar: lipgloss.NewStyle().
			Background(lipgloss.Color("62")).  // Blue
			Foreground(lipgloss.Color("230")). // Light
			Bold(true).
			Padding(0, 1),

		StatusBar: lipgloss.NewStyle().
			Background(lipgloss.Color("236")). // Dark gray
			Foreground(lipgloss.Color("243")). // Light gray
			Padding(0, 1),

		Content: lipgloss.NewStyle().
			Padding(1, 2),

		Directory: lipgloss.NewStyle().
			Foreground(lipgloss.Color("cyan")).
			Bold(false),

		File: lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{
				Light: "#000000",
				Dark:  "#FFFFFF",
			}),

		SelectedItem: lipgloss.NewStyle().
			Foreground(lipgloss.Color("black")).
			Background(lipgloss.Color("214")). // Orange
			Bold(true),

		MarkedItem: lipgloss.NewStyle().
			Foreground(lipgloss.Color("green")).
			Bold(true),

		Modal: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2).
			Width(60),

		ModalTitle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("62")).
			Bold(true).
			Padding(0, 0, 1, 0),

		ModalItem: lipgloss.NewStyle().
			Padding(0, 2),

		ModalSelected: lipgloss.NewStyle().
			Foreground(lipgloss.Color("black")).
			Background(lipgloss.Color("214")). // Orange
			Padding(0, 2).
			Bold(true),

		Table: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("240")),

		TableHeader: lipgloss.NewStyle().
			Foreground(lipgloss.Color("62")).
			Bold(true).
			Padding(0, 1),

		TableRow: lipgloss.NewStyle().
			Padding(0, 1),

		TableRowSelected: lipgloss.NewStyle().
			Foreground(lipgloss.Color("black")).
			Background(lipgloss.Color("214")). // Orange
			Padding(0, 1).
			Bold(true),

		ErrorMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("red")).
			Bold(true),

		SuccessMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("green")).
			Bold(true),

		InfoMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("blue")),

		HelpText: lipgloss.NewStyle().
			Foreground(lipgloss.Color("243")). // Light gray
			Italic(true),
	}
}
