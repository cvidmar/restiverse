package http

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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

// SaveResponse saves an HTTP response to .meta and .body files
func SaveResponse(httpFilePath string, req *HTTPRequest, resp *Response, err error) error {
	// Get responses directory
	responsesDir := filepath.Join(filepath.Dir(httpFilePath), "responses")
	if err := os.MkdirAll(responsesDir, 0755); err != nil {
		return fmt.Errorf("failed to create responses directory: %w", err)
	}

	// Generate timestamp-based filename
	timestamp := time.Now()
	baseName := strings.TrimSuffix(filepath.Base(httpFilePath), ".http")
	timestampStr := timestamp.Format("20060102_150405")
	baseFilename := fmt.Sprintf(".%s_%s", baseName, timestampStr)

	metaPath := filepath.Join(responsesDir, baseFilename+".meta")
	bodyPath := filepath.Join(responsesDir, baseFilename+".body")

	// Build metadata
	meta := ResponseMetadata{
		Timestamp: timestamp.Format(time.RFC3339),
	}

	// Add request metadata
	meta.Request = RequestMetadata{
		Method:  req.Method,
		URL:     req.URL,
		Headers: make(map[string]string),
	}

	// Mask auth headers
	for name, value := range req.Headers {
		if strings.ToLower(name) == "authorization" {
			meta.Request.Headers[name] = MaskAuthHeader(value)
		} else {
			meta.Request.Headers[name] = value
		}
	}

	if err != nil {
		// Request failed
		meta.StatusCode = 0
		meta.DurationMS = 0
		meta.Error = err.Error()
	} else {
		// Request succeeded
		meta.StatusCode = resp.StatusCode
		meta.DurationMS = resp.Duration.Milliseconds()
		meta.Response = ResponseHeaders{
			Headers: resp.Headers,
		}

		// Save response body
		if resp.Body != nil && len(resp.Body) > 0 {
			if err := os.WriteFile(bodyPath, resp.Body, 0644); err != nil {
				return fmt.Errorf("failed to write response body: %w", err)
			}
		}
	}

	// Save metadata
	metaData, err := yaml.Marshal(meta)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := os.WriteFile(metaPath, metaData, 0644); err != nil {
		return fmt.Errorf("failed to write metadata file: %w", err)
	}

	return nil
}

// SaveResponseWithStream saves an HTTP response when the body was streamed to a file
func SaveResponseWithStream(httpFilePath string, req *HTTPRequest, resp *Response, bodyPath string, err error) error {
	// Get responses directory
	responsesDir := filepath.Join(filepath.Dir(httpFilePath), "responses")

	// Generate timestamp-based filename
	timestamp := time.Now()
	baseName := strings.TrimSuffix(filepath.Base(httpFilePath), ".http")
	timestampStr := timestamp.Format("20060102_150405")
	baseFilename := fmt.Sprintf(".%s_%s", baseName, timestampStr)

	metaPath := filepath.Join(responsesDir, baseFilename+".meta")

	// Build metadata
	meta := ResponseMetadata{
		Timestamp: timestamp.Format(time.RFC3339),
	}

	// Add request metadata
	meta.Request = RequestMetadata{
		Method:  req.Method,
		URL:     req.URL,
		Headers: make(map[string]string),
	}

	// Mask auth headers
	for name, value := range req.Headers {
		if strings.ToLower(name) == "authorization" {
			meta.Request.Headers[name] = MaskAuthHeader(value)
		} else {
			meta.Request.Headers[name] = value
		}
	}

	if err != nil {
		// Request failed
		meta.StatusCode = 0
		meta.DurationMS = 0
		meta.Error = err.Error()
	} else {
		// Request succeeded
		meta.StatusCode = resp.StatusCode
		meta.DurationMS = resp.Duration.Milliseconds()
		meta.Response = ResponseHeaders{
			Headers: resp.Headers,
		}
	}

	// Save metadata
	metaData, err := yaml.Marshal(meta)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := os.WriteFile(metaPath, metaData, 0644); err != nil {
		return fmt.Errorf("failed to write metadata file: %w", err)
	}

	return nil
}
