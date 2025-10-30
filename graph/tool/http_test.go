package tool

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// T178: HTTP tool tests

func TestHTTPTool_Name(t *testing.T) {
	tool := NewHTTPTool()
	if tool.Name() != "http_request" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "http_request")
	}
}

func TestHTTPTool_GET_Success(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"message": "success",
			"status":  "ok",
		})
	}))
	defer server.Close()

	tool := NewHTTPTool()
	ctx := context.Background()

	input := map[string]interface{}{
		"method": "GET",
		"url":    server.URL,
	}

	result, err := tool.Call(ctx, input)
	if err != nil {
		t.Fatalf("Call() error = %v, want nil", err)
	}

	// Check status code
	statusCode, ok := result["status_code"].(int)
	if !ok {
		t.Fatalf("status_code has type %T, want int", result["status_code"])
	}
	if statusCode != 200 {
		t.Errorf("status_code = %d, want 200", statusCode)
	}

	// Check body
	body, ok := result["body"].(string)
	if !ok {
		t.Fatalf("body has type %T, want string", result["body"])
	}

	var bodyData map[string]string
	if err := json.Unmarshal([]byte(body), &bodyData); err != nil {
		t.Fatalf("Failed to parse response body: %v", err)
	}

	if bodyData["message"] != "success" {
		t.Errorf("body message = %q, want %q", bodyData["message"], "success")
	}
}

func TestHTTPTool_POST_Success(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		// Read and verify request body
		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}

		if reqBody["name"] != "test" {
			t.Errorf("Request body name = %v, want %q", reqBody["name"], "test")
		}

		// Send response
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":      123,
			"created": true,
		})
	}))
	defer server.Close()

	tool := NewHTTPTool()
	ctx := context.Background()

	requestBody := map[string]interface{}{
		"name": "test",
		"age":  30,
	}
	bodyJSON, _ := json.Marshal(requestBody)

	input := map[string]interface{}{
		"method": "POST",
		"url":    server.URL,
		"body":   string(bodyJSON),
		"headers": map[string]interface{}{
			"Content-Type": "application/json",
		},
	}

	result, err := tool.Call(ctx, input)
	if err != nil {
		t.Fatalf("Call() error = %v, want nil", err)
	}

	statusCode := result["status_code"].(int)
	if statusCode != 201 {
		t.Errorf("status_code = %d, want 201", statusCode)
	}
}

func TestHTTPTool_WithHeaders(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify custom header
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer token123" {
			t.Errorf("Authorization header = %q, want %q", authHeader, "Bearer token123")
		}

		userAgent := r.Header.Get("User-Agent")
		if userAgent != "CustomAgent/1.0" {
			t.Errorf("User-Agent header = %q, want %q", userAgent, "CustomAgent/1.0")
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("authenticated"))
	}))
	defer server.Close()

	tool := NewHTTPTool()
	ctx := context.Background()

	input := map[string]interface{}{
		"method": "GET",
		"url":    server.URL,
		"headers": map[string]interface{}{
			"Authorization": "Bearer token123",
			"User-Agent":    "CustomAgent/1.0",
		},
	}

	result, err := tool.Call(ctx, input)
	if err != nil {
		t.Fatalf("Call() error = %v, want nil", err)
	}

	body := result["body"].(string)
	if body != "authenticated" {
		t.Errorf("body = %q, want %q", body, "authenticated")
	}
}

func TestHTTPTool_ContextTimeout(t *testing.T) {
	// Create slow server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tool := NewHTTPTool()

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	input := map[string]interface{}{
		"method": "GET",
		"url":    server.URL,
	}

	_, err := tool.Call(ctx, input)
	if err == nil {
		t.Error("Call() error = nil, want timeout error")
	}
}

func TestHTTPTool_Error_InvalidURL(t *testing.T) {
	tool := NewHTTPTool()
	ctx := context.Background()

	input := map[string]interface{}{
		"method": "GET",
		"url":    "://invalid-url",
	}

	_, err := tool.Call(ctx, input)
	if err == nil {
		t.Error("Call() error = nil, want error for invalid URL")
	}
}

func TestHTTPTool_Error_MissingURL(t *testing.T) {
	tool := NewHTTPTool()
	ctx := context.Background()

	input := map[string]interface{}{
		"method": "GET",
	}

	_, err := tool.Call(ctx, input)
	if err == nil {
		t.Error("Call() error = nil, want error for missing URL")
	}
}

func TestHTTPTool_Error_UnsupportedMethod(t *testing.T) {
	tool := NewHTTPTool()
	ctx := context.Background()

	input := map[string]interface{}{
		"method": "DELETE",
		"url":    "http://example.com",
	}

	_, err := tool.Call(ctx, input)
	if err == nil {
		t.Error("Call() error = nil, want error for unsupported method")
	}
}

func TestHTTPTool_Error_ServerError(t *testing.T) {
	// Create server that returns 500
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	tool := NewHTTPTool()
	ctx := context.Background()

	input := map[string]interface{}{
		"method": "GET",
		"url":    server.URL,
	}

	result, err := tool.Call(ctx, input)
	if err != nil {
		t.Fatalf("Call() error = %v, want nil (errors returned in response)", err)
	}

	statusCode := result["status_code"].(int)
	if statusCode != 500 {
		t.Errorf("status_code = %d, want 500", statusCode)
	}

	body := result["body"].(string)
	if body != "Internal Server Error" {
		t.Errorf("body = %q, want %q", body, "Internal Server Error")
	}
}

func TestHTTPTool_DefaultMethod(t *testing.T) {
	// Test that GET is used as default when method not specified
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET (default method), got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tool := NewHTTPTool()
	ctx := context.Background()

	input := map[string]interface{}{
		"url": server.URL,
		// method not specified
	}

	_, err := tool.Call(ctx, input)
	if err != nil {
		t.Fatalf("Call() error = %v, want nil", err)
	}
}
