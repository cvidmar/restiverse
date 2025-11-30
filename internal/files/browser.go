package files

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// FileEntry represents a file or directory in the browser
type FileEntry struct {
	Name      string
	Path      string
	IsDir     bool
	IsHTTP    bool
	ModTime   time.Time
	Size      int64
}

// FileType represents the type of a file for action filtering
type FileType string

const (
	FileTypeHTTP FileType = "http"
	FileTypeBody FileType = "body"
	FileTypeMeta FileType = "meta"
)

// ListDirectory returns all files and directories in the given path
// Folders are listed first, then .http files
func ListDirectory(dirPath string) ([]FileEntry, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var files []FileEntry
	for _, entry := range entries {
		// Skip hidden files and response directories
		if strings.HasPrefix(entry.Name(), ".") || entry.Name() == "responses" {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		fullPath := filepath.Join(dirPath, entry.Name())

		fileEntry := FileEntry{
			Name:    entry.Name(),
			Path:    fullPath,
			IsDir:   entry.IsDir(),
			IsHTTP:  !entry.IsDir() && strings.HasSuffix(entry.Name(), ".http"),
			ModTime: info.ModTime(),
			Size:    info.Size(),
		}

		// Only include directories and .http files
		if fileEntry.IsDir || fileEntry.IsHTTP {
			files = append(files, fileEntry)
		}
	}

	// Sort: directories first, then files, both alphabetically
	sort.Slice(files, func(i, j int) bool {
		if files[i].IsDir != files[j].IsDir {
			return files[i].IsDir
		}
		return files[i].Name < files[j].Name
	})

	return files, nil
}

// GetFileType returns the file type for action filtering
func GetFileType(filename string) FileType {
	if strings.HasSuffix(filename, ".http") {
		return FileTypeHTTP
	}
	if strings.HasSuffix(filename, ".body") {
		return FileTypeBody
	}
	if strings.HasSuffix(filename, ".meta") {
		return FileTypeMeta
	}
	return ""
}

// ResponseEntry represents a response in the history
type ResponseEntry struct {
	HTTPFile    string    // The .http file this response belongs to
	Timestamp   time.Time
	MetaPath    string
	BodyPath    string
	StatusCode  int
	Duration    time.Duration
	Size        int64
	HasError    bool
	ErrorMsg    string
}

// ListResponseHistory returns all responses for a given .http file
// Sorted by timestamp, most recent first
func ListResponseHistory(httpFilePath string) ([]ResponseEntry, error) {
	// Get the directory and filename
	dir := filepath.Dir(httpFilePath)
	baseName := strings.TrimSuffix(filepath.Base(httpFilePath), ".http")

	// Check for responses directory
	responsesDir := filepath.Join(dir, "responses")
	if _, err := os.Stat(responsesDir); os.IsNotExist(err) {
		return []ResponseEntry{}, nil // No responses yet
	}

	entries, err := os.ReadDir(responsesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read responses directory: %w", err)
	}

	// Map to group .meta and .body files by timestamp
	responseMap := make(map[string]*ResponseEntry)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()

		// Check if this file belongs to our .http file
		if !strings.HasPrefix(name, "."+baseName+"_") {
			continue
		}

		// Extract timestamp from filename: .filename_YYYYMMDD_HHMMSS.{meta|body}
		parts := strings.Split(name, ".")
		if len(parts) < 2 {
			continue
		}

		// Get the timestamp part (e.g., "filename_20241128_143045")
		timestampPart := parts[len(parts)-2]

		var respEntry *ResponseEntry
		if existing, ok := responseMap[timestampPart]; ok {
			respEntry = existing
		} else {
			respEntry = &ResponseEntry{
				HTTPFile: httpFilePath,
			}
			responseMap[timestampPart] = respEntry
		}

		fullPath := filepath.Join(responsesDir, name)

		if strings.HasSuffix(name, ".meta") {
			respEntry.MetaPath = fullPath
		} else if strings.HasSuffix(name, ".body") {
			respEntry.BodyPath = fullPath
			info, err := entry.Info()
			if err == nil {
				respEntry.Size = info.Size()
			}
		}
	}

	// Convert map to slice and parse meta files
	var responses []ResponseEntry
	for _, resp := range responseMap {
		// Parse metadata if available
		if resp.MetaPath != "" {
			if err := parseResponseMeta(resp); err != nil {
				// If we can't parse meta, skip this response
				continue
			}
		}
		responses = append(responses, *resp)
	}

	// Sort by timestamp, most recent first
	sort.Slice(responses, func(i, j int) bool {
		return responses[i].Timestamp.After(responses[j].Timestamp)
	})

	return responses, nil
}

// parseResponseMeta parses the .meta file and populates the ResponseEntry
func parseResponseMeta(resp *ResponseEntry) error {
	data, err := os.ReadFile(resp.MetaPath)
	if err != nil {
		return fmt.Errorf("failed to read meta file: %w", err)
	}

	// Simple YAML parsing for the fields we need
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "timestamp:") {
			timestampStr := strings.TrimSpace(strings.TrimPrefix(line, "timestamp:"))
			timestampStr = strings.Trim(timestampStr, "\"")
			t, err := time.Parse(time.RFC3339, timestampStr)
			if err == nil {
				resp.Timestamp = t
			}
		} else if strings.HasPrefix(line, "status_code:") {
			fmt.Sscanf(line, "status_code: %d", &resp.StatusCode)
		} else if strings.HasPrefix(line, "duration_ms:") {
			var ms int64
			fmt.Sscanf(line, "duration_ms: %d", &ms)
			resp.Duration = time.Duration(ms) * time.Millisecond
		} else if strings.HasPrefix(line, "error:") {
			resp.HasError = true
			resp.ErrorMsg = strings.TrimSpace(strings.TrimPrefix(line, "error:"))
			resp.ErrorMsg = strings.Trim(resp.ErrorMsg, "\"")
		}
	}

	return nil
}

// GetResponsesDir returns the responses directory for a given .http file
func GetResponsesDir(httpFilePath string) string {
	dir := filepath.Dir(httpFilePath)
	return filepath.Join(dir, "responses")
}

// EnsureResponsesDir creates the responses directory if it doesn't exist
func EnsureResponsesDir(httpFilePath string) error {
	responsesDir := GetResponsesDir(httpFilePath)
	if err := os.MkdirAll(responsesDir, 0755); err != nil {
		return fmt.Errorf("failed to create responses directory: %w", err)
	}
	return nil
}

// FindAllHTTPFiles recursively finds all .http files in a directory
func FindAllHTTPFiles(rootDir string) ([]string, error) {
	var httpFiles []string

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden files and directories
		if strings.HasPrefix(info.Name(), ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip responses directories
		if info.IsDir() && info.Name() == "responses" {
			return filepath.SkipDir
		}

		// Add .http files
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".http") {
			httpFiles = append(httpFiles, path)
		}

		return nil
	})

	return httpFiles, err
}

// FormatFileSize returns a human-readable file size string
func FormatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%dB", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
