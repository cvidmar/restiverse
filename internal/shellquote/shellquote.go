// Package shellquote quotes individual arguments for POSIX shell commands.
package shellquote

import "strings"

// Quote returns s as one safely single-quoted shell word.
func Quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
