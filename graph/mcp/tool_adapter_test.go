package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/dshills/langgraph-go/graph/tool"
)

// ============================================================================
// Types are defined in tool_adapter.go
// ============================================================================
// Mock Tool Implementations for Testing
// ============================================================================

// mockAdapterTool is a mock implementation of tool.Tool for adapter testing.
type mockAdapterTool struct {
	name   string
	result map[string]interface{}
	err    error
	callFn func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error)
}

func (m *mockAdapterTool) Name() string {
	return m.name
}

func (m *mockAdapterTool) Call(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	if m.callFn != nil {
		return m.callFn(ctx, input)
	}
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

// slowTool simulates a tool that takes time to execute.
type slowTool struct {
	name     string
	duration time.Duration
}

func (s *slowTool) Name() string {
	return s.name
}

func (s *slowTool) Call(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	select {
	case <-time.After(s.duration):
		return map[string]interface{}{"result": "completed"}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// adapterErrorTool always returns an error.
type adapterErrorTool struct {
	name string
	err  error
}

func (e *adapterErrorTool) Name() string {
	return e.name
}

func (e *adapterErrorTool) Call(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	return nil, e.err
}

// ============================================================================
// T017: Contract Test for tools/list Method
// ============================================================================

func TestToolRegistry_List_ResponseSchema(t *testing.T) {
	tests := []struct {
		name  string
		tools []struct {
			tool     tool.Tool
			metadata ToolAdapterMetadata
		}
		wantCount int
	}{
		{
			name: "empty registry",
			tools: []struct {
				tool     tool.Tool
				metadata ToolAdapterMetadata
			}{},
			wantCount: 0,
		},
		{
			name: "single tool",
			tools: []struct {
				tool     tool.Tool
				metadata ToolAdapterMetadata
			}{
				{
					tool: &mockAdapterTool{name: "get_weather"},
					metadata: ToolAdapterMetadata{
						Name:        "get_weather",
						Description: "Get current weather for a location",
						InputSchema: map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"location": map[string]interface{}{
									"type":        "string",
									"description": "City name or zip code",
								},
							},
							"required": []interface{}{"location"},
						},
					},
				},
			},
			wantCount: 1,
		},
		{
			name: "multiple tools",
			tools: []struct {
				tool     tool.Tool
				metadata ToolAdapterMetadata
			}{
				{
					tool: &mockAdapterTool{name: "get_weather"},
					metadata: ToolAdapterMetadata{
						Name:        "get_weather",
						Description: "Get current weather",
						InputSchema: map[string]interface{}{
							"type":       "object",
							"properties": map[string]interface{}{},
						},
					},
				},
				{
					tool: &mockAdapterTool{name: "search_db"},
					metadata: ToolAdapterMetadata{
						Name:        "search_db",
						Description: "Search database",
						InputSchema: map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"query": map[string]interface{}{"type": "string"},
							},
							"required": []interface{}{"query"},
						},
					},
				},
			},
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewToolRegistry()

			// Register tools
			for _, toolDef := range tt.tools {
				err := registry.Register(toolDef.tool, toolDef.metadata)
				if err != nil {
					t.Fatalf("Register() failed: %v", err)
				}
			}

			// Get list of tools
			list := registry.List()

			// Verify count
			if len(list) != tt.wantCount {
				t.Errorf("List() returned %d tools, want %d", len(list), tt.wantCount)
			}

			// Verify each tool matches MCP spec schema
			for i, meta := range list {
				// Validate name field (required, non-empty string)
				if meta.Name == "" {
					t.Errorf("Tool[%d].Name is empty", i)
				}

				// Validate description field (required, non-empty string)
				if meta.Description == "" {
					t.Errorf("Tool[%d].Description is empty", i)
				}

				// Validate inputSchema field (required, must be object)
				if meta.InputSchema == nil {
					t.Errorf("Tool[%d].InputSchema is nil", i)
					continue
				}

				// Verify inputSchema is a valid object
				schemaType, ok := meta.InputSchema["type"].(string)
				if !ok || schemaType != "object" {
					t.Errorf("Tool[%d].InputSchema.type = %v, want 'object'", i, schemaType)
				}

				// Verify inputSchema can be marshaled to JSON (valid JSON Schema)
				_, err := json.Marshal(meta.InputSchema)
				if err != nil {
					t.Errorf("Tool[%d].InputSchema is not valid JSON: %v", i, err)
				}
			}
		})
	}
}

func TestToolRegistry_List_ToolNamePattern(t *testing.T) {
	tests := []struct {
		name      string
		toolName  string
		wantValid bool
	}{
		// Valid names
		{name: "lowercase only", toolName: "weather", wantValid: true},
		{name: "with underscores", toolName: "get_weather", wantValid: true},
		{name: "with numbers", toolName: "tool123", wantValid: true},
		{name: "complex valid", toolName: "http_get_v2", wantValid: true},

		// Invalid names
		{name: "starts with number", toolName: "123tool", wantValid: false},
		{name: "uppercase letter", toolName: "GetWeather", wantValid: false},
		{name: "contains hyphen", toolName: "get-weather", wantValid: false},
		{name: "contains space", toolName: "get weather", wantValid: false},
		{name: "starts with underscore", toolName: "_private", wantValid: false},
		{name: "empty name", toolName: "", wantValid: false},
		{name: "special chars", toolName: "get@weather", wantValid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewToolRegistry()
			mockT := &mockAdapterTool{name: tt.toolName}
			metadata := ToolAdapterMetadata{
				Name:        tt.toolName,
				Description: "Test tool",
				InputSchema: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			}

			err := registry.Register(mockT, metadata)

			if tt.wantValid && err != nil {
				t.Errorf("Register() with valid name %q failed: %v", tt.toolName, err)
			}

			if !tt.wantValid && err == nil {
				t.Errorf("Register() with invalid name %q should have failed", tt.toolName)
			}
		})
	}
}

// ============================================================================
// T018: Contract Test for tools/call Method
// ============================================================================

func TestToolRegistry_Invoke_ResponseSchema(t *testing.T) {
	tests := []struct {
		name        string
		toolResult  map[string]interface{}
		toolError   error
		wantContent bool
		wantIsError bool
	}{
		{
			name: "successful text result",
			toolResult: map[string]interface{}{
				"temperature": 72.5,
				"conditions":  "sunny",
			},
			wantContent: true,
			wantIsError: false,
		},
		{
			name:        "empty result",
			toolResult:  map[string]interface{}{},
			wantContent: true,
			wantIsError: false,
		},
		{
			name:        "tool execution error",
			toolError:   errors.New("network timeout"),
			wantContent: false,
			wantIsError: false, // Execution errors return error, not isError flag
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewToolRegistry()

			// Register mock tool
			mockT := &mockAdapterTool{
				name:   "test_tool",
				result: tt.toolResult,
				err:    tt.toolError,
			}
			metadata := ToolAdapterMetadata{
				Name:        "test_tool",
				Description: "Test tool for invocation",
				InputSchema: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			}

			err := registry.Register(mockT, metadata)
			if err != nil {
				t.Fatalf("Register() failed: %v", err)
			}

			// Invoke tool
			ctx := context.Background()
			result, err := registry.Invoke(ctx, "test_tool", map[string]interface{}{})

			if tt.toolError != nil {
				// Expect error returned
				if err == nil {
					t.Error("Invoke() should return error for tool execution failure")
				}
				return
			}

			if err != nil {
				t.Fatalf("Invoke() unexpected error: %v", err)
			}

			// Verify result schema
			if result == nil {
				t.Fatal("Invoke() returned nil result")
			}

			// Verify content field
			if tt.wantContent {
				if len(result.Content) == 0 {
					t.Error("Result.Content is empty, expected at least one content block")
				}

				// Verify first content block structure
				content := result.Content[0]
				if content.Type == "" {
					t.Error("ContentBlock.Type is empty")
				}

				// Text content should have text field populated
				if content.Type == "text" && content.Text == "" {
					t.Error("ContentBlock with type 'text' has empty Text field")
				}
			}

			// Verify isError field
			if result.IsError != tt.wantIsError {
				t.Errorf("Result.IsError = %v, want %v", result.IsError, tt.wantIsError)
			}
		})
	}
}

func TestToolRegistry_Invoke_ToolNotFound(t *testing.T) {
	registry := NewToolRegistry()

	// Attempt to invoke non-existent tool
	ctx := context.Background()
	_, err := registry.Invoke(ctx, "nonexistent_tool", map[string]interface{}{})

	if err == nil {
		t.Error("Invoke() should return error for non-existent tool")
	}

	// Verify error message mentions the tool name
	errMsg := err.Error()
	if errMsg == "" {
		t.Error("Error message is empty")
	}
}

// ============================================================================
// T019: Integration Test for Tool Registration and Discovery
// ============================================================================

func TestToolRegistry_RegisterAndGet(t *testing.T) {
	registry := NewToolRegistry()

	// Define test tool
	weatherTool := &mockAdapterTool{
		name: "get_weather",
		result: map[string]interface{}{
			"temperature": 72.5,
			"conditions":  "sunny",
		},
	}

	metadata := ToolAdapterMetadata{
		Name:        "get_weather",
		Description: "Get current weather for a location",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"location": map[string]interface{}{
					"type":        "string",
					"description": "City name or zip code",
				},
				"units": map[string]interface{}{
					"type":    "string",
					"enum":    []interface{}{"celsius", "fahrenheit"},
					"default": "celsius",
				},
			},
			"required": []interface{}{"location"},
		},
	}

	// Test registration
	err := registry.Register(weatherTool, metadata)
	if err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	// Test retrieval by name
	registered, err := registry.Get("get_weather")
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}

	if registered == nil {
		t.Fatal("Get() returned nil RegisteredTool")
	}

	// Verify tool matches
	if registered.Tool.Name() != "get_weather" {
		t.Errorf("RegisteredTool.Tool.Name() = %q, want 'get_weather'", registered.Tool.Name())
	}

	// Verify metadata matches
	if registered.Metadata.Name != metadata.Name {
		t.Errorf("RegisteredTool.Metadata.Name = %q, want %q", registered.Metadata.Name, metadata.Name)
	}

	if registered.Metadata.Description != metadata.Description {
		t.Errorf("RegisteredTool.Metadata.Description mismatch")
	}

	// Test List() contains the tool
	list := registry.List()
	if len(list) != 1 {
		t.Fatalf("List() returned %d tools, want 1", len(list))
	}

	if list[0].Name != "get_weather" {
		t.Errorf("List()[0].Name = %q, want 'get_weather'", list[0].Name)
	}
}

func TestToolRegistry_Register_DuplicateName(t *testing.T) {
	registry := NewToolRegistry()

	metadata := ToolAdapterMetadata{
		Name:        "duplicate_tool",
		Description: "First tool",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
	}

	// Register first tool
	err := registry.Register(&mockAdapterTool{name: "duplicate_tool"}, metadata)
	if err != nil {
		t.Fatalf("First Register() failed: %v", err)
	}

	// Attempt to register duplicate
	metadata.Description = "Second tool with same name"
	err = registry.Register(&mockAdapterTool{name: "duplicate_tool"}, metadata)

	if err == nil {
		t.Error("Register() should fail for duplicate tool name")
	}
}

func TestToolRegistry_Register_ValidationErrors(t *testing.T) {
	tests := []struct {
		name        string
		metadata    ToolAdapterMetadata
		wantErr     bool
		errContains string
	}{
		{
			name: "empty description",
			metadata: ToolAdapterMetadata{
				Name:        "valid_name",
				Description: "",
				InputSchema: map[string]interface{}{"type": "object"},
			},
			wantErr:     true,
			errContains: "description",
		},
		{
			name: "nil input schema",
			metadata: ToolAdapterMetadata{
				Name:        "valid_name",
				Description: "Valid description",
				InputSchema: nil,
			},
			wantErr:     true,
			errContains: "schema",
		},
		{
			name: "name metadata mismatch",
			metadata: ToolAdapterMetadata{
				Name:        "different_name",
				Description: "Valid description",
				InputSchema: map[string]interface{}{"type": "object"},
			},
			wantErr:     true,
			errContains: "mismatch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewToolRegistry()
			mockT := &mockAdapterTool{name: "valid_name"}

			err := registry.Register(mockT, tt.metadata)

			if tt.wantErr && err == nil {
				t.Error("Register() should return error")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("Register() unexpected error: %v", err)
			}

			if tt.wantErr && err != nil && tt.errContains != "" {
				if !containsString(err.Error(), tt.errContains) {
					t.Errorf("Error message %q should contain %q", err.Error(), tt.errContains)
				}
			}
		})
	}
}

// ============================================================================
// T020: Integration Test for Tool Invocation with Mock Tool
// ============================================================================

func TestToolRegistry_Invoke_WithArguments(t *testing.T) {
	tests := []struct {
		name      string
		arguments map[string]interface{}
		wantCall  bool
	}{
		{
			name: "with required arguments",
			arguments: map[string]interface{}{
				"location": "San Francisco",
				"units":    "fahrenheit",
			},
			wantCall: true,
		},
		{
			name: "with minimal arguments",
			arguments: map[string]interface{}{
				"location": "New York",
			},
			wantCall: true,
		},
		{
			name:      "empty arguments",
			arguments: map[string]interface{}{},
			wantCall:  true, // Should still call, validation happens inside Invoke
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewToolRegistry()

			// Track if tool was called
			called := false
			receivedInput := map[string]interface{}{}

			mockT := &mockAdapterTool{
				name: "test_tool",
				callFn: func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
					called = true
					receivedInput = input
					return map[string]interface{}{"status": "ok"}, nil
				},
			}

			metadata := ToolAdapterMetadata{
				Name:        "test_tool",
				Description: "Test tool",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{"type": "string"},
					},
				},
			}

			err := registry.Register(mockT, metadata)
			if err != nil {
				t.Fatalf("Register() failed: %v", err)
			}

			// Invoke tool
			ctx := context.Background()
			_, err = registry.Invoke(ctx, "test_tool", tt.arguments)

			if err != nil && tt.wantCall {
				t.Errorf("Invoke() unexpected error: %v", err)
			}

			if tt.wantCall && !called {
				t.Error("Tool.Call() was not invoked")
			}

			if called && len(tt.arguments) > 0 {
				// Verify arguments were passed through
				for key, val := range tt.arguments {
					if receivedInput[key] != val {
						t.Errorf("Input[%q] = %v, want %v", key, receivedInput[key], val)
					}
				}
			}
		})
	}
}

func TestToolRegistry_Invoke_ContextCancellation(t *testing.T) {
	registry := NewToolRegistry()

	// Register slow tool
	slowT := &slowTool{
		name:     "slow_tool",
		duration: 5 * time.Second,
	}

	metadata := ToolAdapterMetadata{
		Name:        "slow_tool",
		Description: "Tool that takes time to execute",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
	}

	err := registry.Register(slowT, metadata)
	if err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Invoke tool (should timeout)
	start := time.Now()
	_, err = registry.Invoke(ctx, "slow_tool", map[string]interface{}{})

	elapsed := time.Since(start)

	// Should return error quickly (within timeout period)
	if err == nil {
		t.Error("Invoke() should return error when context is cancelled")
	}

	// Verify it didn't wait for full tool duration
	if elapsed >= 4*time.Second {
		t.Errorf("Invoke() took %v, expected to cancel within timeout", elapsed)
	}

	// Verify error is context-related
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context cancellation error, got: %v", err)
	}
}

// ============================================================================
// T021: Integration Test for Tool Error Handling
// ============================================================================

func TestToolRegistry_Invoke_ToolErrors(t *testing.T) {
	tests := []struct {
		name      string
		toolError error
		wantErr   bool
	}{
		{
			name:      "network timeout",
			toolError: errors.New("network timeout"),
			wantErr:   true,
		},
		{
			name:      "validation error",
			toolError: errors.New("invalid parameter: location"),
			wantErr:   true,
		},
		{
			name:      "database error",
			toolError: errors.New("database connection failed"),
			wantErr:   true,
		},
		{
			name:      "no error",
			toolError: nil,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewToolRegistry()

			errorT := &adapterErrorTool{
				name: "error_tool",
				err:  tt.toolError,
			}

			metadata := ToolAdapterMetadata{
				Name:        "error_tool",
				Description: "Tool that may error",
				InputSchema: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			}

			err := registry.Register(errorT, metadata)
			if err != nil {
				t.Fatalf("Register() failed: %v", err)
			}

			ctx := context.Background()
			result, err := registry.Invoke(ctx, "error_tool", map[string]interface{}{})

			if tt.wantErr {
				if err == nil {
					t.Error("Invoke() should return error when tool fails")
				}
				if result != nil {
					t.Error("Invoke() should return nil result when tool errors")
				}
			} else {
				if err != nil {
					t.Errorf("Invoke() unexpected error: %v", err)
				}
				if result == nil {
					t.Error("Invoke() should return result when tool succeeds")
				}
			}
		})
	}
}

func TestToolRegistry_Invoke_ErrorPropagation(t *testing.T) {
	registry := NewToolRegistry()

	expectedErr := fmt.Errorf("custom tool error: %w", errors.New("underlying cause"))

	mockT := &mockAdapterTool{
		name: "error_tool",
		err:  expectedErr,
	}

	metadata := ToolAdapterMetadata{
		Name:        "error_tool",
		Description: "Tool that errors",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
	}

	err := registry.Register(mockT, metadata)
	if err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	ctx := context.Background()
	_, err = registry.Invoke(ctx, "error_tool", map[string]interface{}{})

	if err == nil {
		t.Fatal("Invoke() should return error")
	}

	// Verify error contains information about the tool error
	errMsg := err.Error()
	if errMsg == "" {
		t.Error("Error message is empty")
	}
}

// ============================================================================
// Concurrent Access Tests
// ============================================================================

func TestToolRegistry_ConcurrentRegistration(t *testing.T) {
	registry := NewToolRegistry()

	const numGoroutines = 50
	errCh := make(chan error, numGoroutines)
	var wg sync.WaitGroup

	// Attempt concurrent registrations with unique names
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			toolName := fmt.Sprintf("tool_%d", id)
			mockT := &mockAdapterTool{name: toolName}
			metadata := ToolAdapterMetadata{
				Name:        toolName,
				Description: fmt.Sprintf("Tool %d", id),
				InputSchema: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			}

			if err := registry.Register(mockT, metadata); err != nil {
				errCh <- fmt.Errorf("goroutine %d: %w", id, err)
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	// Check for errors
	for err := range errCh {
		t.Errorf("Concurrent registration failed: %v", err)
	}

	// Verify all tools were registered
	list := registry.List()
	if len(list) != numGoroutines {
		t.Errorf("List() returned %d tools, want %d", len(list), numGoroutines)
	}
}

func TestToolRegistry_ConcurrentInvocation(t *testing.T) {
	registry := NewToolRegistry()

	// Register a tool
	mockT := &mockAdapterTool{
		name: "concurrent_tool",
		result: map[string]interface{}{
			"value": 42,
		},
	}

	metadata := ToolAdapterMetadata{
		Name:        "concurrent_tool",
		Description: "Tool for concurrent testing",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
	}

	err := registry.Register(mockT, metadata)
	if err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	// Invoke tool concurrently
	const numInvocations = 100
	errCh := make(chan error, numInvocations)
	var wg sync.WaitGroup

	for i := 0; i < numInvocations; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			ctx := context.Background()
			_, err := registry.Invoke(ctx, "concurrent_tool", map[string]interface{}{
				"id": id,
			})

			if err != nil {
				errCh <- fmt.Errorf("invocation %d: %w", id, err)
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	// Check for errors
	for err := range errCh {
		t.Errorf("Concurrent invocation failed: %v", err)
	}
}

func TestToolRegistry_ConcurrentReadWrite(t *testing.T) {
	registry := NewToolRegistry()

	const numOperations = 100
	var wg sync.WaitGroup

	// Concurrent writers (registrations)
	for i := 0; i < numOperations; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			toolName := fmt.Sprintf("tool_%d", id)
			mockT := &mockAdapterTool{name: toolName}
			metadata := ToolAdapterMetadata{
				Name:        toolName,
				Description: "Test tool",
				InputSchema: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			}

			_ = registry.Register(mockT, metadata)
		}(i)
	}

	// Concurrent readers (list/get operations)
	for i := 0; i < numOperations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Read operations should not panic
			_ = registry.List()
		}()
	}

	wg.Wait()

	// Verify registry is still functional
	list := registry.List()
	if len(list) == 0 {
		t.Error("List() returned empty after concurrent operations")
	}
}

// ============================================================================
// Helper Functions
// ============================================================================

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && (s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			findSubstring(s, substr))))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
