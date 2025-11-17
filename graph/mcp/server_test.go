package mcp

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/dshills/langgraph-go/graph/emit"
	"github.com/dshills/langgraph-go/graph/tool"
	"github.com/sourcegraph/jsonrpc2"
)

// mockTool is a simple tool implementation for testing.
type mockTool struct {
	name string
}

func (m *mockTool) Name() string {
	return m.name
}

func (m *mockTool) Call(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{"result": "ok"}, nil
}

// mockResource is a simple resource implementation for testing.
type mockResource struct {
	uri         string
	name        string
	description string
	mimeType    string
	content     []byte
}

func (m *mockResource) URI() string {
	return m.uri
}

func (m *mockResource) MimeType() string {
	return m.mimeType
}

func (m *mockResource) Read(ctx context.Context) ([]byte, error) {
	return m.content, nil
}

func (m *mockResource) Info() ResourceInfo {
	return ResourceInfo{
		URI:         m.uri,
		Name:        m.name,
		Description: m.description,
		MimeType:    m.mimeType,
	}
}

// createMockPromptTemplate creates a simple prompt template for testing.
func createMockPromptTemplate(name string) PromptTemplate {
	return PromptTemplate{
		Name:        name,
		Description: "Test prompt template",
		Parameters:  []PromptParameter{},
		Template:    "Test template text",
	}
}

func TestNewServer(t *testing.T) {
	config := ServerConfig{
		Name:    "test-server",
		Version: "1.0.0",
		Emitter: nil,
	}

	server := NewServer(config)
	if server == nil {
		t.Fatal("NewServer returned nil")
	}

	// Verify server can be used immediately (basic smoke test)
	// Internal state is tested through behavior, not direct access
}

func TestRegisterTool(t *testing.T) {
	server := NewServer(ServerConfig{Name: "test", Version: "1.0.0"})

	tool := &mockTool{name: "test_tool"}
	metadata := ToolMetadata{
		Description: "A test tool",
		Schema: map[string]interface{}{
			"type": "object",
		},
	}

	// Register tool successfully
	err := server.RegisterTool("test_tool", tool, metadata)
	if err != nil {
		t.Errorf("RegisterTool failed: %v", err)
	}

	// Attempt to register duplicate tool
	err = server.RegisterTool("test_tool", tool, metadata)
	if err == nil {
		t.Error("expected error when registering duplicate tool, got nil")
	}
}

func TestRegisterToolNameMismatch(t *testing.T) {
	server := NewServer(ServerConfig{Name: "test", Version: "1.0.0"})

	tool := &mockTool{name: "actual_name"}
	metadata := ToolMetadata{Description: "Test"}

	// Attempt to register with mismatched name
	err := server.RegisterTool("different_name", tool, metadata)
	if err == nil {
		t.Error("expected error when tool name doesn't match, got nil")
	}
}

func TestRegisterResource(t *testing.T) {
	server := NewServer(ServerConfig{Name: "test", Version: "1.0.0"})

	// Register static resource successfully
	err := server.RegisterStaticResource(
		"file:///test.txt",
		"Test Resource",
		"A test resource",
		"text/plain",
		[]byte("test content"),
	)
	if err != nil {
		t.Errorf("RegisterStaticResource failed: %v", err)
	}

	// Attempt to register duplicate resource
	err = server.RegisterStaticResource(
		"file:///test.txt",
		"Test Resource Duplicate",
		"A duplicate test resource",
		"text/plain",
		[]byte("test content 2"),
	)
	if err == nil {
		t.Error("expected error when registering duplicate resource, got nil")
	}
}

func TestRegisterResourceURIMismatch(t *testing.T) {
	server := NewServer(ServerConfig{Name: "test", Version: "1.0.0"})

	resource := &mockResource{uri: "file:///actual.txt"}

	// Attempt to register with mismatched URI
	err := server.RegisterResource("file:///different.txt", resource)
	if err == nil {
		t.Error("expected error when resource URI doesn't match, got nil")
	}
}

func TestRegisterPrompt(t *testing.T) {
	server := NewServer(ServerConfig{Name: "test", Version: "1.0.0"})

	prompt := createMockPromptTemplate("test_prompt")

	// Register prompt successfully
	err := server.RegisterPrompt(prompt)
	if err != nil {
		t.Errorf("RegisterPrompt failed: %v", err)
	}

	// Attempt to register duplicate prompt
	err = server.RegisterPrompt(prompt)
	if err == nil {
		t.Error("expected error when registering duplicate prompt, got nil")
	}
}

func TestServerLifecycleStates(t *testing.T) {
	server := NewServer(ServerConfig{Name: "test", Version: "1.0.0"})

	// Stop without starting should work
	err := server.Stop()
	if err != nil {
		t.Errorf("Stop failed: %v", err)
	}

	// Registering capabilities after stop should fail
	tool := &mockTool{name: "test"}
	err = server.RegisterTool("test", tool, ToolMetadata{})
	if err == nil {
		t.Error("expected error when registering tool on stopped server, got nil")
	}
}

func TestStopIdempotent(t *testing.T) {
	server := NewServer(ServerConfig{Name: "test", Version: "1.0.0"})

	// Call Stop multiple times
	err1 := server.Stop()
	err2 := server.Stop()
	err3 := server.Stop()

	if err1 != nil {
		t.Errorf("first Stop() failed: %v", err1)
	}
	if err2 != nil {
		t.Errorf("second Stop() failed: %v", err2)
	}
	if err3 != nil {
		t.Errorf("third Stop() failed: %v", err3)
	}
}

func TestRegisterToolValidation(t *testing.T) {
	tests := []struct {
		name        string
		toolName    string
		tool        tool.Tool
		metadata    ToolMetadata
		wantErr     bool
		errContains string
	}{
		{
			name:     "valid registration",
			toolName: "valid_tool",
			tool:     &mockTool{name: "valid_tool"},
			metadata: ToolMetadata{
				Description: "A valid tool",
				Schema: map[string]interface{}{
					"type": "object",
				},
			},
			wantErr: false,
		},
		{
			name:        "name mismatch",
			toolName:    "registry_name",
			tool:        &mockTool{name: "tool_name"},
			metadata:    ToolMetadata{Description: "Mismatched"},
			wantErr:     true,
			errContains: "mismatch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewServer(ServerConfig{Name: "test", Version: "1.0.0"})
			err := server.RegisterTool(tt.toolName, tt.tool, tt.metadata)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestServerCapabilityNegotiation(t *testing.T) {
	server := NewServer(ServerConfig{Name: "test", Version: "1.0.0"})

	// Register various capabilities
	tool := &mockTool{name: "test_tool"}
	prompt := createMockPromptTemplate("test_prompt")

	err := server.RegisterTool("test_tool", tool, ToolMetadata{
		Description: "Test",
		Schema: map[string]interface{}{
			"type": "object",
		},
	})
	if err != nil {
		t.Errorf("RegisterTool failed: %v", err)
	}

	err = server.RegisterStaticResource(
		"file:///test.txt",
		"Test Resource",
		"A test resource",
		"text/plain",
		[]byte("test content"),
	)
	if err != nil {
		t.Errorf("RegisterStaticResource failed: %v", err)
	}

	err = server.RegisterPrompt(prompt)
	if err != nil {
		t.Errorf("RegisterPrompt failed: %v", err)
	}

	// Verify capabilities were registered successfully by trying to register duplicates
	err = server.RegisterTool("test_tool", tool, ToolMetadata{Description: "Test"})
	if err == nil {
		t.Error("expected error when registering duplicate tool")
	}

	err = server.RegisterStaticResource(
		"file:///test.txt",
		"Test Resource Duplicate",
		"A duplicate test resource",
		"text/plain",
		[]byte("test content 2"),
	)
	if err == nil {
		t.Error("expected error when registering duplicate resource")
	}

	err = server.RegisterPrompt(prompt)
	if err == nil {
		t.Error("expected error when registering duplicate prompt")
	}
}

// errorTool is a tool that returns errors for testing error handling.
type errorTool struct {
	name string
	err  error
}

func (e *errorTool) Name() string {
	return e.name
}

func (e *errorTool) Call(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	return nil, e.err
}

func TestToolErrorHandling(t *testing.T) {
	server := NewServer(ServerConfig{Name: "test", Version: "1.0.0"})

	tool := &errorTool{
		name: "error_tool",
		err:  errors.New("tool execution failed"),
	}

	metadata := ToolMetadata{
		Description: "Error tool",
		Schema: map[string]interface{}{
			"type": "object",
		},
	}

	err := server.RegisterTool("error_tool", tool, metadata)
	if err != nil {
		t.Errorf("RegisterTool failed: %v", err)
	}

	// Tool should be registered even though it returns errors when called
	// Verify by attempting to register a duplicate
	err = server.RegisterTool("error_tool", tool, metadata)
	if err == nil {
		t.Error("expected error when registering duplicate tool")
	}
}

// TestToolsListHandler tests the tools/list JSON-RPC handler
func TestToolsListHandler(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(MCPServer)
		expectErr     bool
		expectErrCode int
		validateResp  func(*testing.T, interface{})
	}{
		{
			name: "list tools with no registrations",
			setup: func(srv MCPServer) {
				// No tools registered
			},
			expectErr:     true,
			expectErrCode: ErrCodeMethodNotFound,
		},
		{
			name: "list tools with single tool",
			setup: func(srv MCPServer) {
				tool := &mockTool{name: "test_tool"}
				metadata := ToolMetadata{
					Description: "A test tool",
					Schema: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"param1": map[string]interface{}{
								"type": "string",
							},
						},
					},
				}
				srv.RegisterTool("test_tool", tool, metadata)
			},
			expectErr:     true,
			expectErrCode: ErrCodeMethodNotFound,
		},
		{
			name: "list tools with multiple tools",
			setup: func(srv MCPServer) {
				tool1 := &mockTool{name: "tool_one"}
				tool2 := &mockTool{name: "tool_two"}

				srv.RegisterTool("tool_one", tool1, ToolMetadata{
					Description: "First tool",
					Schema: map[string]interface{}{
						"type": "object",
					},
				})
				srv.RegisterTool("tool_two", tool2, ToolMetadata{
					Description: "Second tool",
					Schema: map[string]interface{}{
						"type": "object",
					},
				})
			},
			expectErr:     true,
			expectErrCode: ErrCodeMethodNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := NewServer(ServerConfig{Name: "test", Version: "1.0.0"})
			s := srv.(*server)

			if tt.setup != nil {
				tt.setup(srv)
			}

			// Create a jsonrpc2.Request for tools/list
			ctx := context.Background()
			req := &jsonrpc2.Request{
				Method: "tools/list",
				ID:     jsonrpc2.ID{Num: 1},
				Params: nil,
			}

			result, err := s.handleToolsList(ctx, req)

			if tt.expectErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if tt.validateResp != nil {
					tt.validateResp(t, result)
				}
			}
		})
	}
}

// TestToolsCallHandler tests the tools/call JSON-RPC handler
func TestToolsCallHandler(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(MCPServer)
		params        interface{}
		expectErr     bool
		expectErrCode int
		validateResp  func(*testing.T, interface{})
	}{
		{
			name: "call tool that doesn't exist",
			setup: func(srv MCPServer) {
				// No tools registered
			},
			params: map[string]interface{}{
				"name":      "nonexistent_tool",
				"arguments": map[string]interface{}{},
			},
			expectErr:     true,
			expectErrCode: ErrCodeMethodNotFound,
		},
		{
			name: "call tool with missing name parameter",
			setup: func(srv MCPServer) {
				tool := &mockTool{name: "test_tool"}
				srv.RegisterTool("test_tool", tool, ToolMetadata{
					Description: "Test",
					Schema: map[string]interface{}{
						"type": "object",
					},
				})
			},
			params: map[string]interface{}{
				"arguments": map[string]interface{}{},
			},
			expectErr:     true,
			expectErrCode: ErrCodeMethodNotFound,
		},
		{
			name: "call tool with invalid arguments type",
			setup: func(srv MCPServer) {
				tool := &mockTool{name: "test_tool"}
				srv.RegisterTool("test_tool", tool, ToolMetadata{
					Description: "Test",
					Schema: map[string]interface{}{
						"type": "object",
					},
				})
			},
			params: map[string]interface{}{
				"name":      "test_tool",
				"arguments": "not-an-object",
			},
			expectErr:     true,
			expectErrCode: ErrCodeMethodNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := NewServer(ServerConfig{Name: "test", Version: "1.0.0"})
			s := srv.(*server)

			if tt.setup != nil {
				tt.setup(srv)
			}

			// Create a jsonrpc2.Request for tools/call
			ctx := context.Background()
			req := &jsonrpc2.Request{
				Method: "tools/call",
				ID:     jsonrpc2.ID{Num: 1},
			}
			if tt.params != nil {
				req.SetParams(tt.params)
			}

			result, err := s.handleToolsCall(ctx, req)

			if tt.expectErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if tt.validateResp != nil {
					tt.validateResp(t, result)
				}
			}
		})
	}
}

// TestConcurrentToolRegistration tests concurrent tool registration
func TestConcurrentToolRegistration(t *testing.T) {
	srv := NewServer(ServerConfig{Name: "test", Version: "1.0.0"})

	const numGoroutines = 10
	errChan := make(chan error, numGoroutines)

	// Launch concurrent registrations
	for i := 0; i < numGoroutines; i++ {
		go func(index int) {
			toolName := fmt.Sprintf("tool_%d", index)
			tool := &mockTool{name: toolName}
			metadata := ToolMetadata{
				Description: fmt.Sprintf("Tool %d", index),
				Schema: map[string]interface{}{
					"type": "object",
				},
			}
			errChan <- srv.RegisterTool(toolName, tool, metadata)
		}(i)
	}

	// Collect results
	for i := 0; i < numGoroutines; i++ {
		if err := <-errChan; err != nil {
			t.Errorf("concurrent registration %d failed: %v", i, err)
		}
	}

	// Verify all tools were registered
	s := srv.(*server)
	s.mu.RLock()
	registeredCount := len(s.toolRegistry)
	s.mu.RUnlock()

	if registeredCount != numGoroutines {
		t.Errorf("expected %d tools registered, got %d", numGoroutines, registeredCount)
	}
}

// TestConcurrentToolInvocation tests concurrent tool calls
func TestConcurrentToolInvocation(t *testing.T) {
	srv := NewServer(ServerConfig{Name: "test", Version: "1.0.0"})
	s := srv.(*server)

	// Register a tool
	tool := &mockTool{name: "concurrent_tool"}
	metadata := ToolMetadata{
		Description: "Test concurrent invocation",
		Schema: map[string]interface{}{
			"type": "object",
		},
	}

	err := srv.RegisterTool("concurrent_tool", tool, metadata)
	if err != nil {
		t.Fatalf("failed to register tool: %v", err)
	}

	const numGoroutines = 20
	errChan := make(chan error, numGoroutines)

	// Launch concurrent calls
	for i := 0; i < numGoroutines; i++ {
		go func() {
			ctx := context.Background()
			req := &jsonrpc2.Request{
				Method: "tools/call",
				ID:     jsonrpc2.ID{Num: uint64(i)},
			}
			params := map[string]interface{}{
				"name":      "concurrent_tool",
				"arguments": map[string]interface{}{},
			}
			req.SetParams(params)

			_, err := s.handleToolsCall(ctx, req)

			errChan <- err
		}()
	}

	// Collect results - all should fail with method not found (handler not implemented)
	// This tests that concurrent access to the registry doesn't cause races
	for i := 0; i < numGoroutines; i++ {
		<-errChan // Just verify no panic/race
	}
}

// TestToolRegistrationRaceConditions tests for race conditions in tool operations
func TestToolRegistrationRaceConditions(t *testing.T) {
	srv := NewServer(ServerConfig{Name: "test", Version: "1.0.0"})
	s := srv.(*server)

	// Register initial tool
	tool := &mockTool{name: "race_tool"}
	metadata := ToolMetadata{
		Description: "Test race conditions",
		Schema: map[string]interface{}{
			"type": "object",
		},
	}

	err := srv.RegisterTool("race_tool", tool, metadata)
	if err != nil {
		t.Fatalf("failed to register tool: %v", err)
	}

	done := make(chan bool)

	// Goroutine 1: Try to read tool registry repeatedly
	go func() {
		for i := 0; i < 100; i++ {
			s.mu.RLock()
			_ = len(s.toolRegistry)
			s.mu.RUnlock()
		}
		done <- true
	}()

	// Goroutine 2: Try to register duplicate tool repeatedly
	go func() {
		for i := 0; i < 100; i++ {
			duplicateTool := &mockTool{name: "race_tool"}
			_ = srv.RegisterTool("race_tool", duplicateTool, metadata)
		}
		done <- true
	}()

	// Goroutine 3: Try to access handleToolsList
	go func() {
		ctx := context.Background()
		for i := 0; i < 100; i++ {
			req := &jsonrpc2.Request{
				Method: "tools/list",
				ID:     jsonrpc2.ID{Num: uint64(i)},
			}
			_, _ = s.handleToolsList(ctx, req)
		}
		done <- true
	}()

	// Wait for all goroutines
	<-done
	<-done
	<-done
}

// TestToolInvocationWithContext tests tool invocation with context cancellation
func TestToolInvocationWithContext(t *testing.T) {
	srv := NewServer(ServerConfig{Name: "test", Version: "1.0.0"})
	s := srv.(*server)

	// Register a slow tool
	slowTool := &slowMockTool{
		name:     "slow_tool",
		duration: 100 * time.Millisecond,
	}

	metadata := ToolMetadata{
		Description: "Slow tool for testing cancellation",
		Schema: map[string]interface{}{
			"type": "object",
		},
	}

	err := srv.RegisterTool("slow_tool", slowTool, metadata)
	if err != nil {
		t.Fatalf("failed to register tool: %v", err)
	}

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	req := &jsonrpc2.Request{
		Method: "tools/call",
		ID:     jsonrpc2.ID{Num: 1},
	}
	params := map[string]interface{}{
		"name":      "slow_tool",
		"arguments": map[string]interface{}{},
	}
	req.SetParams(params)

	_, err = s.handleToolsCall(ctx, req)

	// Should fail (method not implemented yet), but shouldn't panic
	// This tests that context cancellation doesn't cause races
	_ = err
}

// slowMockTool is a tool that simulates slow execution
type slowMockTool struct {
	name     string
	duration time.Duration
}

func (s *slowMockTool) Name() string {
	return s.name
}

func (s *slowMockTool) Call(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	select {
	case <-time.After(s.duration):
		return map[string]interface{}{"result": "completed"}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// ========== T040: Static Resource Registration Tests ==========

// TestMCPServer_RegisterStaticResource_Success tests successful registration of a static resource
func TestMCPServer_RegisterStaticResource_Success(t *testing.T) {
	server := NewServer(ServerConfig{Name: "test", Version: "1.0.0"})

	err := server.RegisterStaticResource(
		"file:///docs/readme.txt",
		"README Document",
		"Project README file",
		"text/plain",
		[]byte("# Project README\n\nWelcome to the project."),
	)

	if err != nil {
		t.Errorf("RegisterStaticResource failed: %v", err)
	}

	// Verify resource was registered by attempting duplicate registration
	err = server.RegisterStaticResource(
		"file:///docs/readme.txt",
		"README Document",
		"Project README file",
		"text/plain",
		[]byte("# Project README\n\nWelcome to the project."),
	)

	if err == nil {
		t.Error("expected error when registering duplicate resource URI, got nil")
	}
}

// TestMCPServer_RegisterStaticResource_InvalidURI tests rejection of invalid URI patterns
func TestMCPServer_RegisterStaticResource_InvalidURI(t *testing.T) {
	tests := []struct {
		name        string
		uri         string
		errContains string
	}{
		{
			name:        "uppercase in URI",
			uri:         "file:///docs/README.txt",
			errContains: "URI must be lowercase",
		},
		{
			name:        "special characters",
			uri:         "file:///docs/read me!.txt",
			errContains: "invalid URI characters",
		},
		{
			name:        "empty URI",
			uri:         "",
			errContains: "URI cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewServer(ServerConfig{Name: "test", Version: "1.0.0"})

			err := server.RegisterStaticResource(
				tt.uri,
				"Test Resource",
				"Test description",
				"text/plain",
				[]byte("test content"),
			)

			if err == nil {
				t.Errorf("expected error containing %q, got nil", tt.errContains)
			}
		})
	}
}

// TestMCPServer_RegisterStaticResource_DuplicateURI tests rejection of duplicate resource URIs
func TestMCPServer_RegisterStaticResource_DuplicateURI(t *testing.T) {
	server := NewServer(ServerConfig{Name: "test", Version: "1.0.0"})

	uri := "file:///docs/config.yaml"

	// Register first resource
	err := server.RegisterStaticResource(
		uri,
		"Config File",
		"Application configuration",
		"application/yaml",
		[]byte("app:\n  port: 8080"),
	)

	if err != nil {
		t.Fatalf("first registration failed: %v", err)
	}

	// Attempt duplicate registration with different content
	err = server.RegisterStaticResource(
		uri,
		"Config File Updated",
		"Updated application configuration",
		"application/yaml",
		[]byte("app:\n  port: 9090"),
	)

	if err == nil {
		t.Error("expected error when registering duplicate URI, got nil")
	}

	if err != nil && !errors.Is(err, ErrResourceAlreadyExists) {
		t.Errorf("expected ErrResourceAlreadyExists, got: %v", err)
	}
}

// TestMCPServer_RegisterStaticResource_EmptyName tests rejection of empty name/description
func TestMCPServer_RegisterStaticResource_EmptyName(t *testing.T) {
	tests := []struct {
		name         string
		resourceName string
		description  string
		errContains  string
	}{
		{
			name:         "empty name",
			resourceName: "",
			description:  "Valid description",
			errContains:  "name cannot be empty",
		},
		{
			name:         "empty description",
			resourceName: "Valid Name",
			description:  "",
			errContains:  "description cannot be empty",
		},
		{
			name:         "both empty",
			resourceName: "",
			description:  "",
			errContains:  "name cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewServer(ServerConfig{Name: "test", Version: "1.0.0"})

			err := server.RegisterStaticResource(
				"file:///docs/test.txt",
				tt.resourceName,
				tt.description,
				"text/plain",
				[]byte("test content"),
			)

			if err == nil {
				t.Errorf("expected error containing %q, got nil", tt.errContains)
			}
		})
	}
}

// TestMCPServer_RegisterStaticResource_AfterStart tests rejection of registration after server started
func TestMCPServer_RegisterStaticResource_AfterStart(t *testing.T) {
	server := NewServer(ServerConfig{Name: "test", Version: "1.0.0"})

	// Simulate server start by stopping it (which sets state)
	err := server.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Attempt registration after server has been started/stopped
	err = server.RegisterStaticResource(
		"file:///docs/late.txt",
		"Late Resource",
		"Resource registered after start",
		"text/plain",
		[]byte("too late"),
	)

	if err == nil {
		t.Error("expected error when registering resource after server start, got nil")
	}

	if err != nil && !errors.Is(err, ErrServerNotRunning) {
		t.Errorf("expected ErrServerNotRunning, got: %v", err)
	}
}

// ========== T041: Dynamic Resource Registration Tests ==========

// TestMCPServer_RegisterDynamicResource_Success tests successful registration of a dynamic resource
func TestMCPServer_RegisterDynamicResource_Success(t *testing.T) {
	server := NewServer(ServerConfig{Name: "test", Version: "1.0.0"})

	generatorFunc := func(ctx context.Context) ([]byte, error) {
		return []byte(fmt.Sprintf("Generated content at %s", time.Now().Format(time.RFC3339))), nil
	}

	err := server.RegisterDynamicResource(
		"dynamic:///status/system",
		"System Status",
		"Current system status information",
		"application/json",
		generatorFunc,
	)

	if err != nil {
		t.Errorf("RegisterDynamicResource failed: %v", err)
	}

	// Verify resource was registered by attempting duplicate registration
	err = server.RegisterDynamicResource(
		"dynamic:///status/system",
		"System Status Duplicate",
		"Duplicate system status",
		"application/json",
		generatorFunc,
	)

	if err == nil {
		t.Error("expected error when registering duplicate dynamic resource URI, got nil")
	}
}

// TestMCPServer_RegisterDynamicResource_NilGenerator tests rejection of nil generator function
func TestMCPServer_RegisterDynamicResource_NilGenerator(t *testing.T) {
	server := NewServer(ServerConfig{Name: "test", Version: "1.0.0"})

	err := server.RegisterDynamicResource(
		"dynamic:///status/health",
		"Health Status",
		"System health check",
		"application/json",
		nil, // Nil generator
	)

	if err == nil {
		t.Error("expected error when registering dynamic resource with nil generator, got nil")
	}

	if err != nil && !errors.Is(err, ErrInvalidGenerator) {
		t.Errorf("expected ErrInvalidGenerator, got: %v", err)
	}
}

// TestMCPServer_RegisterDynamicResource_InvalidURI tests rejection of invalid URI patterns
func TestMCPServer_RegisterDynamicResource_InvalidURI(t *testing.T) {
	generatorFunc := func(ctx context.Context) ([]byte, error) {
		return []byte("content"), nil
	}

	tests := []struct {
		name        string
		uri         string
		errContains string
	}{
		{
			name:        "uppercase in URI",
			uri:         "dynamic:///Status/Health",
			errContains: "URI must be lowercase",
		},
		{
			name:        "special characters",
			uri:         "dynamic:///status/health check!",
			errContains: "invalid URI characters",
		},
		{
			name:        "empty URI",
			uri:         "",
			errContains: "URI cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewServer(ServerConfig{Name: "test", Version: "1.0.0"})

			err := server.RegisterDynamicResource(
				tt.uri,
				"Test Dynamic Resource",
				"Test description",
				"application/json",
				generatorFunc,
			)

			if err == nil {
				t.Errorf("expected error containing %q, got nil", tt.errContains)
			}
		})
	}
}

// TestMCPServer_RegisterDynamicResource_GeneratorReturnsError tests handling of generator errors
func TestMCPServer_RegisterDynamicResource_GeneratorReturnsError(t *testing.T) {
	server := NewServer(ServerConfig{Name: "test", Version: "1.0.0"})

	generatorError := errors.New("generator failed to produce content")
	errorGeneratorFunc := func(ctx context.Context) ([]byte, error) {
		return nil, generatorError
	}

	err := server.RegisterDynamicResource(
		"dynamic:///failing/resource",
		"Failing Resource",
		"A resource that fails to generate content",
		"text/plain",
		errorGeneratorFunc,
	)

	if err != nil {
		t.Fatalf("RegisterDynamicResource should succeed even if generator may fail later: %v", err)
	}

	// TODO: When resource read endpoint is implemented, verify that
	// attempting to read this resource returns the generator error
}

// TestMCPServer_RegisterDynamicResource_ContextCancellation tests that generator respects context cancellation
func TestMCPServer_RegisterDynamicResource_ContextCancellation(t *testing.T) {
	server := NewServer(ServerConfig{Name: "test", Version: "1.0.0"})

	slowGeneratorFunc := func(ctx context.Context) ([]byte, error) {
		// Simulate slow generation
		select {
		case <-time.After(500 * time.Millisecond):
			return []byte("slow content"), nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	err := server.RegisterDynamicResource(
		"dynamic:///slow/resource",
		"Slow Resource",
		"A resource with slow generation",
		"text/plain",
		slowGeneratorFunc,
	)

	if err != nil {
		t.Fatalf("RegisterDynamicResource failed: %v", err)
	}

	// TODO: When resource read endpoint is implemented, test that:
	// 1. Create context with short timeout (e.g., 100ms)
	// 2. Attempt to read the resource
	// 3. Verify that context.DeadlineExceeded is returned
	// 4. Verify that generator respected cancellation
}

// ========== T072: Connection Handling Tests ==========

// captureEmitter captures emitted events for testing
type captureEmitter struct {
	events []map[string]interface{}
	mu     sync.Mutex
}

func (c *captureEmitter) Emit(event emit.Event) {
	c.mu.Lock()
	defer c.mu.Unlock()

	eventData := map[string]interface{}{
		"runID":  event.RunID,
		"nodeID": event.NodeID,
		"msg":    event.Msg,
		"meta":   event.Meta,
	}
	c.events = append(c.events, eventData)
}

func (c *captureEmitter) EmitBatch(_ context.Context, events []emit.Event) error {
	for _, e := range events {
		c.Emit(e)
	}
	return nil
}

func (c *captureEmitter) Flush(_ context.Context) error {
	return nil
}

func (c *captureEmitter) GetEvents() []map[string]interface{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Return copy to prevent external modifications
	eventsCopy := make([]map[string]interface{}, len(c.events))
	copy(eventsCopy, c.events)
	return eventsCopy
}

// TestConnectionSession_Creation tests that ConnectionSession is created during initialize
func TestConnectionSession_Creation(t *testing.T) {
	emitter := &captureEmitter{}
	srv := NewServer(ServerConfig{
		Name:    "test-server",
		Version: "1.0.0",
		Emitter: emitter,
	})
	s := srv.(*server)

	// Set server to initializing state
	s.state.Store(StateInitializing)

	// Create initialize request
	initParams := InitializeRequest{
		ProtocolVersion: "2025-06-18",
		ClientInfo: ClientInfo{
			Name:    "test-client",
			Version: "1.0.0",
		},
		Capabilities: Capabilities{
			Tools:     true,
			Resources: true,
			Prompts:   false,
		},
	}

	// Wrap in jsonrpc2.Request
	req := &jsonrpc2.Request{
		Method: "initialize",
		ID:     jsonrpc2.ID{Num: 1},
	}
	req.SetParams(initParams)

	// Create context with connection_time for deterministic testing
	connectionTime := time.Now().UnixNano()
	ctx := context.WithValue(context.Background(), "connection_time", connectionTime)

	// Call handleInitialize
	_, err := s.handleInitialize(ctx, req)
	if err != nil {
		t.Fatalf("handleInitialize failed: %v", err)
	}

	// Verify session was created
	s.sessionMu.RLock()
	session := s.session
	s.sessionMu.RUnlock()

	if session == nil {
		t.Fatal("expected session to be created, got nil")
	}

	// Verify session metadata
	if session.ProtocolVersion != "2025-06-18" {
		t.Errorf("expected ProtocolVersion = '2025-06-18', got %q", session.ProtocolVersion)
	}

	clientName, ok := session.ClientInfo["name"].(string)
	if !ok || clientName != "test-client" {
		t.Errorf("expected ClientInfo.name = 'test-client', got %v", session.ClientInfo["name"])
	}

	clientVersion, ok := session.ClientInfo["version"].(string)
	if !ok || clientVersion != "1.0.0" {
		t.Errorf("expected ClientInfo.version = '1.0.0', got %v", session.ClientInfo["version"])
	}

	if session.ConnectionTime != connectionTime {
		t.Errorf("expected ConnectionTime = %d, got %d", connectionTime, session.ConnectionTime)
	}

	// Verify capabilities
	if toolsCap, ok := session.Capabilities["tools"].(bool); !ok || !toolsCap {
		t.Errorf("expected Capabilities.tools = true, got %v", session.Capabilities["tools"])
	}

	if resourcesCap, ok := session.Capabilities["resources"].(bool); !ok || !resourcesCap {
		t.Errorf("expected Capabilities.resources = true, got %v", session.Capabilities["resources"])
	}

	if promptsCap, ok := session.Capabilities["prompts"].(bool); !ok || promptsCap {
		t.Errorf("expected Capabilities.prompts = false, got %v", session.Capabilities["prompts"])
	}
}

// TestConnectionSession_ClientConnectEvent tests that client_connect event is emitted
func TestConnectionSession_ClientConnectEvent(t *testing.T) {
	emitter := &captureEmitter{}
	srv := NewServer(ServerConfig{
		Name:    "connect-test-server",
		Version: "1.0.0",
		Emitter: emitter,
	})
	s := srv.(*server)

	// Set server to initializing state
	s.state.Store(StateInitializing)

	// Create initialize request
	initParams := InitializeRequest{
		ProtocolVersion: "2025-06-18",
		ClientInfo: ClientInfo{
			Name:    "claude-desktop",
			Version: "0.7.2",
		},
		Capabilities: Capabilities{
			Tools:     true,
			Resources: true,
			Prompts:   true,
		},
	}

	// Wrap in jsonrpc2.Request
	req := &jsonrpc2.Request{
		Method: "initialize",
		ID:     jsonrpc2.ID{Num: 1},
	}
	req.SetParams(initParams)

	ctx := context.Background()

	// Call handleInitialize
	_, err := s.handleInitialize(ctx, req)
	if err != nil {
		t.Fatalf("handleInitialize failed: %v", err)
	}

	// Verify client_connect event was emitted
	events := emitter.GetEvents()
	if len(events) == 0 {
		t.Fatal("expected at least one event, got none")
	}

	// Find client_connect event
	var connectEvent map[string]interface{}
	for _, event := range events {
		if event["msg"] == "client_connect" {
			connectEvent = event
			break
		}
	}

	if connectEvent == nil {
		t.Fatal("client_connect event not found in emitted events")
	}

	// Verify event content
	if connectEvent["runID"] != "server" {
		t.Errorf("expected runID = 'server', got %v", connectEvent["runID"])
	}

	if connectEvent["nodeID"] != "connect-test-server" {
		t.Errorf("expected nodeID = 'connect-test-server', got %v", connectEvent["nodeID"])
	}

	// Verify metadata
	meta, ok := connectEvent["meta"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected meta to be map[string]interface{}, got %T", connectEvent["meta"])
	}

	if meta["protocol_version"] != "2025-06-18" {
		t.Errorf("expected meta.protocol_version = '2025-06-18', got %v", meta["protocol_version"])
	}

	if meta["client_name"] != "claude-desktop" {
		t.Errorf("expected meta.client_name = 'claude-desktop', got %v", meta["client_name"])
	}

	if meta["client_version"] != "0.7.2" {
		t.Errorf("expected meta.client_version = '0.7.2', got %v", meta["client_version"])
	}

	// Verify capabilities in metadata
	caps, ok := meta["client_capabilities"].(map[string]bool)
	if !ok {
		t.Fatalf("expected meta.client_capabilities to be map[string]bool, got %T", meta["client_capabilities"])
	}

	if !caps["tools"] {
		t.Error("expected client_capabilities.tools = true")
	}
	if !caps["resources"] {
		t.Error("expected client_capabilities.resources = true")
	}
	if !caps["prompts"] {
		t.Error("expected client_capabilities.prompts = true")
	}
}

// TestConnectionSession_ClientDisconnectEvent tests that client_disconnect event is emitted on Stop()
func TestConnectionSession_ClientDisconnectEvent(t *testing.T) {
	emitter := &captureEmitter{}
	srv := NewServer(ServerConfig{
		Name:    "disconnect-test-server",
		Version: "1.0.0",
		Emitter: emitter,
	})
	s := srv.(*server)

	// Set server to initializing state
	s.state.Store(StateInitializing)

	// Create a session first
	initParams := InitializeRequest{
		ProtocolVersion: "2025-06-18",
		ClientInfo: ClientInfo{
			Name:    "test-client",
			Version: "1.0.0",
		},
		Capabilities: Capabilities{
			Tools:     true,
			Resources: false,
			Prompts:   false,
		},
	}

	// Wrap in jsonrpc2.Request
	req := &jsonrpc2.Request{
		Method: "initialize",
		ID:     jsonrpc2.ID{Num: 1},
	}
	req.SetParams(initParams)

	ctx := context.Background()
	_, err := s.handleInitialize(ctx, req)
	if err != nil {
		t.Fatalf("handleInitialize failed: %v", err)
	}

	// Clear events emitted during initialize
	emitter.mu.Lock()
	emitter.events = nil
	emitter.mu.Unlock()

	// Call Stop to trigger disconnect event
	err = srv.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Verify client_disconnect event was emitted
	events := emitter.GetEvents()
	if len(events) == 0 {
		t.Fatal("expected at least one event after Stop(), got none")
	}

	// Find client_disconnect event
	var disconnectEvent map[string]interface{}
	for _, event := range events {
		if event["msg"] == "client_disconnect" {
			disconnectEvent = event
			break
		}
	}

	if disconnectEvent == nil {
		t.Fatal("client_disconnect event not found in emitted events")
	}

	// Verify event content
	if disconnectEvent["runID"] != "server" {
		t.Errorf("expected runID = 'server', got %v", disconnectEvent["runID"])
	}

	if disconnectEvent["nodeID"] != "disconnect-test-server" {
		t.Errorf("expected nodeID = 'disconnect-test-server', got %v", disconnectEvent["nodeID"])
	}

	// Verify metadata
	meta, ok := disconnectEvent["meta"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected meta to be map[string]interface{}, got %T", disconnectEvent["meta"])
	}

	if meta["server_name"] != "disconnect-test-server" {
		t.Errorf("expected meta.server_name = 'disconnect-test-server', got %v", meta["server_name"])
	}
}

// TestConnectionSession_NoDisconnectEventWithoutSession tests that no disconnect event is emitted if no session exists
func TestConnectionSession_NoDisconnectEventWithoutSession(t *testing.T) {
	emitter := &captureEmitter{}
	srv := NewServer(ServerConfig{
		Name:    "no-session-server",
		Version: "1.0.0",
		Emitter: emitter,
	})

	// Call Stop without ever initializing (no session created)
	err := srv.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Verify no client_disconnect event was emitted
	events := emitter.GetEvents()
	for _, event := range events {
		if event["msg"] == "client_disconnect" {
			t.Error("client_disconnect event should not be emitted when no session exists")
		}
	}
}

// TestConnectionSession_ThreadSafety tests concurrent access to session tracking
func TestConnectionSession_ThreadSafety(t *testing.T) {
	srv := NewServer(ServerConfig{
		Name:    "thread-safety-server",
		Version: "1.0.0",
		Emitter: &captureEmitter{},
	})
	s := srv.(*server)

	// Set server to initializing state
	s.state.Store(StateInitializing)

	// Create session
	initParams := InitializeRequest{
		ProtocolVersion: "2025-06-18",
		ClientInfo: ClientInfo{
			Name:    "concurrent-client",
			Version: "1.0.0",
		},
		Capabilities: Capabilities{
			Tools:     true,
			Resources: true,
			Prompts:   true,
		},
	}

	// Wrap in jsonrpc2.Request
	req := &jsonrpc2.Request{
		Method: "initialize",
		ID:     jsonrpc2.ID{Num: 1},
	}
	req.SetParams(initParams)

	ctx := context.Background()
	_, err := s.handleInitialize(ctx, req)
	if err != nil {
		t.Fatalf("handleInitialize failed: %v", err)
	}

	const numGoroutines = 50
	done := make(chan bool, numGoroutines*2)

	// Concurrent readers
	for i := 0; i < numGoroutines; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				s.sessionMu.RLock()
				session := s.session
				s.sessionMu.RUnlock()

				if session == nil {
					t.Error("session should not be nil")
				}

				// Access session fields
				_ = session.ProtocolVersion
				_ = session.ClientInfo
				_ = session.Capabilities
				_ = session.ConnectionTime
			}
			done <- true
		}()
	}

	// Concurrent Stop() calls (to test disconnect logic)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				// Stop is idempotent, so multiple calls should be safe
				_ = srv.Stop()
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines*2; i++ {
		<-done
	}
}

// TestConnectionSession_MetadataAccuracy tests that session metadata is accurately captured
func TestConnectionSession_MetadataAccuracy(t *testing.T) {
	tests := []struct {
		name            string
		protocolVersion string
		clientName      string
		clientVersion   string
		tools           bool
		resources       bool
		prompts         bool
	}{
		{
			name:            "all capabilities enabled",
			protocolVersion: "2025-06-18",
			clientName:      "claude-desktop",
			clientVersion:   "0.7.2",
			tools:           true,
			resources:       true,
			prompts:         true,
		},
		{
			name:            "only tools enabled",
			protocolVersion: "2025-06-18",
			clientName:      "custom-client",
			clientVersion:   "2.0.0",
			tools:           true,
			resources:       false,
			prompts:         false,
		},
		{
			name:            "no capabilities enabled",
			protocolVersion: "2024-11-05",
			clientName:      "minimal-client",
			clientVersion:   "0.1.0",
			tools:           false,
			resources:       false,
			prompts:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := NewServer(ServerConfig{
				Name:    "metadata-test-server",
				Version: "1.0.0",
				Emitter: &captureEmitter{},
			})
			s := srv.(*server)

			// Set server to initializing state
			s.state.Store(StateInitializing)

			initParams := InitializeRequest{
				ProtocolVersion: tt.protocolVersion,
				ClientInfo: ClientInfo{
					Name:    tt.clientName,
					Version: tt.clientVersion,
				},
				Capabilities: Capabilities{
					Tools:     tt.tools,
					Resources: tt.resources,
					Prompts:   tt.prompts,
				},
			}

			// Wrap in jsonrpc2.Request
			req := &jsonrpc2.Request{
				Method: "initialize",
				ID:     jsonrpc2.ID{Num: 1},
			}
			req.SetParams(initParams)

			ctx := context.Background()
			_, err := s.handleInitialize(ctx, req)
			if err != nil {
				t.Fatalf("handleInitialize failed: %v", err)
			}

			// Verify all metadata fields
			s.sessionMu.RLock()
			session := s.session
			s.sessionMu.RUnlock()

			if session == nil {
				t.Fatal("expected session to be created")
			}

			// Check protocol version
			if session.ProtocolVersion != tt.protocolVersion {
				t.Errorf("ProtocolVersion: expected %q, got %q", tt.protocolVersion, session.ProtocolVersion)
			}

			// Check client info
			if name, ok := session.ClientInfo["name"].(string); !ok || name != tt.clientName {
				t.Errorf("ClientInfo.name: expected %q, got %v", tt.clientName, session.ClientInfo["name"])
			}

			if version, ok := session.ClientInfo["version"].(string); !ok || version != tt.clientVersion {
				t.Errorf("ClientInfo.version: expected %q, got %v", tt.clientVersion, session.ClientInfo["version"])
			}

			// Check capabilities
			if tools, ok := session.Capabilities["tools"].(bool); !ok || tools != tt.tools {
				t.Errorf("Capabilities.tools: expected %v, got %v", tt.tools, session.Capabilities["tools"])
			}

			if resources, ok := session.Capabilities["resources"].(bool); !ok || resources != tt.resources {
				t.Errorf("Capabilities.resources: expected %v, got %v", tt.resources, session.Capabilities["resources"])
			}

			if prompts, ok := session.Capabilities["prompts"].(bool); !ok || prompts != tt.prompts {
				t.Errorf("Capabilities.prompts: expected %v, got %v", tt.prompts, session.Capabilities["prompts"])
			}

			// Check connection time is set
			if session.ConnectionTime == 0 {
				t.Error("ConnectionTime should be set to non-zero value")
			}
		})
	}
}
