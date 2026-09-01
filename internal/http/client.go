package http

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Response represents an HTTP response with metadata
type Response struct {
	StatusCode int
	Headers    map[string]string
	Duration   time.Duration
}

// StreamResponseToFile executes an HTTP request and streams the response body to a file
func StreamResponseToFile(ctx context.Context, req *HTTPRequest, writer io.Writer) (*Response, error) {
	startTime := time.Now()

	// Create HTTP request
	var bodyReader io.Reader
	if req.HasBody() {
		bodyReader = strings.NewReader(req.Body)
	}

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Add headers
	for name, value := range req.Headers {
		httpReq.Header.Set(name, value)
	}

	// Execute request
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Stream response body to writer
	_, err = io.Copy(writer, resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to write response body: %w", err)
	}

	duration := time.Since(startTime)

	// Extract headers
	headers := make(map[string]string)
	for name, values := range resp.Header {
		if len(values) > 0 {
			headers[name] = values[0]
		}
	}

	return &Response{
		StatusCode: resp.StatusCode,
		Headers:    headers,
		Duration:   duration,
	}, nil
}

// MaskAuthHeader masks the Authorization header value for display
func MaskAuthHeader(value string) string {
	if strings.HasPrefix(value, "Bearer ") {
		return "Bearer ***"
	}
	if strings.HasPrefix(value, "Basic ") {
		return "Basic ***"
	}
	return "***"
}
