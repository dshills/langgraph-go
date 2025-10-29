package tool

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// HTTPTool is a tool for making HTTP requests.
//
// It supports GET and POST methods and returns the HTTP response including
// status code, headers, and body. Useful for LLM agents that need to:
//   - Fetch data from REST APIs
//   - Send data to webhooks
//   - Scrape web pages
//   - Interact with external services
//
// Input Parameters:
//   - method: HTTP method ("GET" or "POST", defaults to "GET")
//   - url: Target URL (required)
//   - headers: Optional map of HTTP headers
//   - body: Optional request body (for POST requests)
//
// Output:
//   - status_code: HTTP status code (e.g., 200, 404)
//   - headers: Response headers as map
//   - body: Response body as string
//
// Example usage:
//
//	tool := NewHTTPTool()
//	result, err := tool.Call(ctx, map[string]interface{}{
//	    "method": "GET",
//	    "url": "https://api.example.com/data",
//	    "headers": map[string]interface{}{
//	        "Authorization": "Bearer token",
//	    },
//	})
//	fmt.Printf("Status: %d, Body: %s\n", result["status_code"], result["body"])
type HTTPTool struct {
	client *http.Client
}

// NewHTTPTool creates a new HTTP tool with default settings.
func NewHTTPTool() *HTTPTool {
	return &HTTPTool{
		client: &http.Client{
			// Timeout handled via context
		},
	}
}

// Name returns the tool identifier.
func (h *HTTPTool) Name() string {
	return "http_request"
}

// Call executes an HTTP request with the provided parameters.
func (h *HTTPTool) Call(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	// Extract and validate URL
	urlStr, ok := input["url"].(string)
	if !ok || urlStr == "" {
		return nil, fmt.Errorf("url parameter required (string)")
	}

	// Extract method (default to GET)
	method := "GET"
	if m, ok := input["method"].(string); ok && m != "" {
		method = strings.ToUpper(m)
	}

	// Validate method
	if method != "GET" && method != "POST" {
		return nil, fmt.Errorf("unsupported HTTP method: %s (supported: GET, POST)", method)
	}

	// Extract body
	var body io.Reader
	if bodyStr, ok := input["body"].(string); ok && bodyStr != "" {
		body = bytes.NewBufferString(bodyStr)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, method, urlStr, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	if headers, ok := input["headers"].(map[string]interface{}); ok {
		for key, value := range headers {
			if valueStr, ok := value.(string); ok {
				req.Header.Set(key, valueStr)
			}
		}
	}

	// Execute request
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Extract response headers
	respHeaders := make(map[string]interface{})
	for key, values := range resp.Header {
		if len(values) == 1 {
			respHeaders[key] = values[0]
		} else {
			respHeaders[key] = values
		}
	}

	// Build result
	result := map[string]interface{}{
		"status_code": resp.StatusCode,
		"headers":     respHeaders,
		"body":        string(respBody),
	}

	return result, nil
}
