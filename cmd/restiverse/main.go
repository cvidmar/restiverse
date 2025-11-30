package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cvidmar/restiverse/internal/config"
	"github.com/cvidmar/restiverse/internal/tui"
)

const version = "0.1.0"

func main() {
	// Parse command line arguments
	baseDir := "."
	if len(os.Args) > 1 {
		arg := os.Args[1]

		// Handle flags
		if arg == "--version" || arg == "-v" {
			fmt.Printf("restiverse version %s\n", version)
			os.Exit(0)
		}

		if arg == "--help" || arg == "-h" {
			printHelp()
			os.Exit(0)
		}

		// Otherwise treat as directory path
		baseDir = arg
	}

	// Resolve to absolute path
	absDir, err := filepath.Abs(baseDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid path: %v\n", err)
		os.Exit(3)
	}

	// Check if directory exists
	info, err := os.Stat(absDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Error: directory does not exist: %s\n", absDir)
			os.Exit(3)
		}
		fmt.Fprintf(os.Stderr, "Error: cannot access directory: %v\n", err)
		os.Exit(3)
	}

	if !info.IsDir() {
		fmt.Fprintf(os.Stderr, "Error: not a directory: %s\n", absDir)
		os.Exit(3)
	}

	// Load configuration
	cfg, err := config.LoadConfig(absDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to load configuration: %v\n", err)
		os.Exit(2)
	}

	// Enable debug logging if DEBUG env var is set
	if os.Getenv("DEBUG") != "" {
		f, err := tea.LogToFile("restiverse-debug.log", "debug")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to open debug log: %v\n", err)
		} else {
			defer f.Close()
		}
	}

	// Create and run the TUI program
	p := tea.NewProgram(
		tui.NewModel(cfg, absDir),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Printf(`Restiverse - Terminal-based REST API client

Usage:
  restiverse [directory]
  restiverse [flags]

Arguments:
  directory    Path to project directory (default: current directory)

Flags:
  -h, --help     Show this help message
  -v, --version  Show version information

Examples:
  restiverse                    # Start in current directory
  restiverse ~/projects/api     # Start in specific directory
  DEBUG=1 restiverse .          # Enable debug logging

For more information, visit: https://github.com/cvidmar/restiverse
`)
}
