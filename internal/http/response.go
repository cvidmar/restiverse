package http

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cvidmar/restiverse/internal/respfile"
	"gopkg.in/yaml.v3"
)

// ResponseMetadata represents the metadata stored in .meta files
type ResponseMetadata struct {
	Timestamp  string            `yaml:"timestamp"`
	StatusCode int               `yaml:"status_code"`
	DurationMS int64             `yaml:"duration_ms"`
	Error      string            `yaml:"error,omitempty"`
	Request    RequestMetadata   `yaml:"request"`
	Response   ResponseHeaders   `yaml:"response,omitempty"`
	Vars       map[string]string `yaml:"vars,omitempty"` // Variable values used in this request
}

// RequestMetadata represents the request portion of metadata
type RequestMetadata struct {
	Method  string            `yaml:"method"`
	URL     string            `yaml:"url"`
	Headers map[string]string `yaml:"headers"`
}

// ResponseHeaders represents the response headers in metadata
type ResponseHeaders struct {
	Headers map[string]string `yaml:"headers"`
}

// MetadataOptions controls redaction in stored metadata.
type MetadataOptions struct {
	SensitiveHeaders     []string
	SensitiveQueryParams []string
}

// SaveResponseMetadata writes metadata for an already allocated response record.
func SaveResponseMetadata(record respfile.Record, req *HTTPRequest, resp *Response, requestErr error, options MetadataOptions) error {
	if err := os.MkdirAll(filepath.Dir(record.MetaPath), 0o755); err != nil {
		return fmt.Errorf("failed to create responses directory: %w", err)
	}

	meta := ResponseMetadata{
		Timestamp: record.Timestamp.Format(time.RFC3339Nano),
		Request: RequestMetadata{
			Method:  req.Method,
			URL:     redactURL(req.URL, options.SensitiveQueryParams),
			Headers: redactHeaders(req.Headers, options.SensitiveHeaders),
		},
	}

	if requestErr != nil {
		meta.Error = requestErr.Error()
		if err := os.Remove(record.BodyPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove stale response body: %w", err)
		}
	} else {
		meta.StatusCode = resp.StatusCode
		meta.DurationMS = resp.Duration.Milliseconds()
		meta.Response = ResponseHeaders{Headers: redactHeaders(resp.Headers, options.SensitiveHeaders)}
	}

	data, err := yaml.Marshal(meta)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}
	if err := writePrivateFile(record.MetaPath, data); err != nil {
		return fmt.Errorf("failed to write metadata file: %w", err)
	}
	return nil
}

func redactHeaders(headers map[string]string, sensitive []string) map[string]string {
	sensitiveSet := make(map[string]bool, len(sensitive))
	for _, name := range sensitive {
		sensitiveSet[strings.ToLower(name)] = true
	}
	redacted := make(map[string]string, len(headers))
	for name, value := range headers {
		lowerName := strings.ToLower(name)
		if sensitiveSet[lowerName] {
			if lowerName == "authorization" || lowerName == "proxy-authorization" {
				redacted[name] = MaskAuthHeader(value)
			} else {
				redacted[name] = "***"
			}
		} else {
			redacted[name] = value
		}
	}
	return redacted
}

func redactURL(rawURL string, sensitive []string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	sensitiveSet := make(map[string]bool, len(sensitive))
	for _, name := range sensitive {
		sensitiveSet[strings.ToLower(name)] = true
	}
	query := parsed.Query()
	for name := range query {
		if sensitiveSet[strings.ToLower(name)] {
			query[name] = []string{"***"}
		}
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
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

// CleanupOldResponses removes old response files, keeping only the most recent maxResponses files
// If maxResponses is 0, no cleanup is performed
func CleanupOldResponses(httpFilePath string, maxResponses int) error {
	if maxResponses <= 0 {
		return nil // Unlimited responses, no cleanup
	}

	// Get responses directory
	responsesDir := filepath.Join(filepath.Dir(httpFilePath), "responses")
	if _, err := os.Stat(responsesDir); os.IsNotExist(err) {
		return nil // No responses directory, nothing to clean
	}

	// Get base name for this HTTP file
	baseName := respfile.BaseName(httpFilePath)

	// Find all response files for this HTTP file
	entries, err := os.ReadDir(responsesDir)
	if err != nil {
		return fmt.Errorf("failed to read responses directory: %w", err)
	}

	// Collect meta files for this HTTP file with their modification times
	type responseFile struct {
		metaPath  string
		bodyPath  string
		timestamp time.Time
		id        string
	}
	var responses []responseFile

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if filepath.Ext(name) != ".meta" {
			continue
		}
		id, timestamp, ok := respfile.Match(name, baseName)
		if !ok {
			continue
		}

		responses = append(responses, responseFile{
			metaPath:  filepath.Join(responsesDir, name),
			bodyPath:  filepath.Join(responsesDir, id+".body"),
			timestamp: timestamp,
			id:        id,
		})
	}

	// If we have fewer responses than the max, nothing to delete
	if len(responses) <= maxResponses {
		return nil
	}

	// Sort by the encoded timestamp (newest first), with a stable ID tie-breaker.
	sort.Slice(responses, func(i, j int) bool {
		if responses[i].timestamp.Equal(responses[j].timestamp) {
			return responses[i].id > responses[j].id
		}
		return responses[i].timestamp.After(responses[j].timestamp)
	})

	// Delete old responses (keep only the first maxResponses)
	for i := maxResponses; i < len(responses); i++ {
		// Delete meta file
		if err := os.Remove(responses[i].metaPath); err != nil && !os.IsNotExist(err) {
			// Log error but continue with other files
			fmt.Fprintf(os.Stderr, "Warning: failed to delete %s: %v\n", responses[i].metaPath, err)
		}

		// Delete body file (may not exist for failed requests)
		if _, err := os.Stat(responses[i].bodyPath); err == nil {
			if err := os.Remove(responses[i].bodyPath); err != nil && !os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "Warning: failed to delete %s: %v\n", responses[i].bodyPath, err)
			}
		}
	}

	return nil
}
