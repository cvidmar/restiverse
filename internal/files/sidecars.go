package files

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cvidmar/restiverse/internal/respfile"
)

type pathMove struct {
	from string
	to   string
}

// VariableSidecarPath returns the saved-variable path for an HTTP request.
func VariableSidecarPath(httpFilePath string) string {
	return filepath.Join(filepath.Dir(httpFilePath), respfile.BaseName(httpFilePath)+".vars")
}

// RequestSidecars lists saved variables and response files owned by a request.
func RequestSidecars(httpFilePath string) ([]string, error) {
	var sidecars []string
	varPath := VariableSidecarPath(httpFilePath)
	if _, err := os.Stat(varPath); err == nil {
		sidecars = append(sidecars, varPath)
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	responsesDir := filepath.Join(filepath.Dir(httpFilePath), "responses")
	entries, err := os.ReadDir(responsesDir)
	if os.IsNotExist(err) {
		return sidecars, nil
	}
	if err != nil {
		return nil, err
	}
	base := respfile.BaseName(httpFilePath)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if _, _, ok := respfile.Match(entry.Name(), base); ok {
			sidecars = append(sidecars, filepath.Join(responsesDir, entry.Name()))
		}
	}
	return sidecars, nil
}

// RenameRequestArtifacts moves a request and all of its sidecars as one operation.
func RenameRequestArtifacts(oldPath, newPath string) error {
	if oldPath == newPath {
		return nil
	}
	sidecars, err := RequestSidecars(oldPath)
	if err != nil {
		return fmt.Errorf("list sidecars: %w", err)
	}

	oldBase := respfile.BaseName(oldPath)
	newBase := respfile.BaseName(newPath)
	moves := make([]pathMove, 0, len(sidecars)+1)
	if oldBase != newBase || filepath.Dir(oldPath) != filepath.Dir(newPath) {
		for _, source := range sidecars {
			var target string
			if source == VariableSidecarPath(oldPath) {
				target = VariableSidecarPath(newPath)
			} else {
				name := filepath.Base(source)
				targetName := newBase + strings.TrimPrefix(name, oldBase)
				target = filepath.Join(filepath.Dir(newPath), "responses", targetName)
			}
			moves = append(moves, pathMove{from: source, to: target})
		}
	}
	moves = append(moves, pathMove{from: oldPath, to: newPath})

	for _, move := range moves {
		if _, err := os.Stat(move.to); err == nil {
			return fmt.Errorf("target already exists: %s", move.to)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("inspect target %s: %w", move.to, err)
		}
	}

	moved := make([]pathMove, 0, len(moves))
	for _, move := range moves {
		if err := os.MkdirAll(filepath.Dir(move.to), 0o755); err != nil {
			rollbackMoves(moved)
			return err
		}
		if err := os.Rename(move.from, move.to); err != nil {
			rollbackErr := rollbackMoves(moved)
			if rollbackErr != nil {
				return fmt.Errorf("rename %s: %w; rollback failed: %v", move.from, err, rollbackErr)
			}
			return fmt.Errorf("rename %s: %w", move.from, err)
		}
		moved = append(moved, move)
	}
	return nil
}

func rollbackMoves(moves []pathMove) error {
	var firstErr error
	for i := len(moves) - 1; i >= 0; i-- {
		if err := os.Rename(moves[i].to, moves[i].from); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// DeleteRequestArtifacts deletes sidecars first and the request file last.
func DeleteRequestArtifacts(httpFilePath string) error {
	sidecars, err := RequestSidecars(httpFilePath)
	if err != nil {
		return fmt.Errorf("list sidecars: %w", err)
	}
	for _, path := range sidecars {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("delete sidecar %s: %w", path, err)
		}
	}
	if err := os.Remove(httpFilePath); err != nil {
		return fmt.Errorf("delete request: %w", err)
	}
	return nil
}

// CopyVariableSidecar copies saved variables without overwriting an orphan target.
func CopyVariableSidecar(oldHTTPPath, newHTTPPath string) error {
	source := VariableSidecarPath(oldHTTPPath)
	data, err := os.ReadFile(source)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	target := VariableSidecarPath(newHTTPPath)
	file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(data)
	return err
}
