// Package respfile owns response-record naming and matching.
package respfile

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const stampLayout = "20060102_150405"

var stampPattern = regexp.MustCompile(`^(.*)_(\d{8}_\d{6})(?:_(\d{3}))?$`)

// Record contains the paired paths for one stored response.
type Record struct {
	ID        string
	Timestamp time.Time
	MetaPath  string
	BodyPath  string
}

// BaseName returns the filename without its extension.
func BaseName(httpFilePath string) string {
	name := filepath.Base(httpFilePath)
	return strings.TrimSuffix(name, filepath.Ext(name))
}

// Stamp formats a local wall-clock timestamp with millisecond precision.
func Stamp(ts time.Time) string {
	return fmt.Sprintf("%s_%03d", ts.Format(stampLayout), ts.Nanosecond()/int(time.Millisecond))
}

// NewRecord returns the paths used for a response written at ts.
func NewRecord(httpFilePath string, ts time.Time) Record {
	id := BaseName(httpFilePath) + "_" + Stamp(ts)
	dir := filepath.Join(filepath.Dir(httpFilePath), "responses")
	return Record{
		ID:        id,
		Timestamp: ts,
		MetaPath:  filepath.Join(dir, id+".meta"),
		BodyPath:  filepath.Join(dir, id+".body"),
	}
}

// Match identifies a legacy or current response filename belonging to base.
// The returned record ID, not the timestamp, pairs .meta and .body files.
func Match(fileName, base string) (recordID string, timestamp time.Time, ok bool) {
	ext := filepath.Ext(fileName)
	if ext != ".meta" && ext != ".body" {
		return "", time.Time{}, false
	}

	stem := strings.TrimSuffix(fileName, ext)
	match := stampPattern.FindStringSubmatch(stem)
	if match == nil || match[1] != base {
		return "", time.Time{}, false
	}

	ts, err := time.ParseInLocation(stampLayout, match[2], time.Local)
	if err != nil {
		return "", time.Time{}, false
	}
	if match[3] != "" {
		milliseconds, err := strconv.Atoi(match[3])
		if err != nil {
			return "", time.Time{}, false
		}
		ts = ts.Add(time.Duration(milliseconds) * time.Millisecond)
	}
	return stem, ts, true
}
