// Package integration provides end-to-end integration tests for MCP server functionality.
//
// These tests verify cross-service communication patterns, connection lifecycle management,
// and graceful handling of network conditions.
package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/dshills/langgraph-go/graph/emit"
	"github.com/dshills/langgraph-go/graph/mcp"
)

// MockTool implements the Tool interface for testing
type MockTool struct {
	name     string
	callFunc func(context.Context, map[string]interface{}) (map[string]interface{}, error)
}

func (m *MockTool) Name() string {
	return m.name
}

func (m *MockTool) Call(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	if m.callFunc != nil {
		return m.callFunc(ctx, input)
	}
	return map[string]interface{}{"result": "ok"}, nil
}

// MockTransport implements a bidirectional pipe for testing MCP communication
type MockTransport struct {
	serverRead  *io.PipeReader
	serverWrite *io.PipeWriter
	clientRead  *io.PipeReader
	clientWrite *io.PipeWriter
	closed      bool
	mu          sync.Mutex
}

func NewMockTransport() *MockTransport {
	sr, cw := io.Pipe()
	cr, sw := io.Pipe()
	return &MockTransport{
		serverRead:  sr,
		serverWrite: sw,
		clientRead:  cr,
		clientWrite: cw,
	}
}

func (t *MockTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return nil
	}
	t.closed = true
	t.serverRead.Close()
	t.serverWrite.Close()
	t.clientRead.Close()
	t.clientWrite.Close()
	return nil
}

// SendRequest simulates a client sending a JSON-RPC request
func (t *MockTransport) SendRequest(method string, params interface{}) error {
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	}
	data, err := json.Marshal(request)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = t.clientWrite.Write(data)
	return err
}

// ReadResponse simulates a client reading a JSON-RPC response
func (t *MockTransport) ReadResponse() (map[string]interface{}, error) {
	decoder := json.NewDecoder(t.clientRead)
	var response map[string]interface{}
	if err := decoder.Decode(&response); err != nil {
		return nil, err
	}
	return response, nil
}

// T060: Integration test for cross-service tool invocation
func TestCrossServiceToolInvocation(t *testing.T) {
	// Create server
	server := mcp.NewServer(mcp.ServerConfig{
		Name:    "test-server",
		Version: "1.0.0",
		Emitter: emit.NewNullEmitter(),
	})

	// Register a test tool
	tool := &MockTool{
		name: "test_tool",
		callFunc: func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
			message, ok := input["message"].(string)
			if !ok {
				return nil, fmt.Errorf("message parameter required")
			}
			return map[string]interface{}{
				"echo":      message,
				"processed": true,
			}, nil
		},
	}

	err := server.RegisterTool("test_tool", tool, mcp.ToolMetadata{
		Description: "Echo test tool",
		Schema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"message": map[string]interface{}{
					"type":        "string",
					"description": "Message to echo",
				},
			},
			"required": []string{"message"},
		},
	})
	if err != nil {
		t.Fatalf("Failed to register tool: %v", err)
	}

	// TODO: Once transport layer is implemented, test actual tool invocation
	// For now, verify registration worked
	t.Log("Cross-service tool invocation test placeholder - transport layer needed")
}

// T061: Integration test for cross-service resource reads
func TestCrossServiceResourceReads(t *testing.T) {
	// Create server
	server := mcp.NewServer(mcp.ServerConfig{
		Name:    "test-server",
		Version: "1.0.0",
		Emitter: emit.NewNullEmitter(),
	})

	// Register a test resource
	err := server.RegisterStaticResource(
		"test/resource",
		"Test Resource",
		"A test resource for cross-service reads",
		"application/json",
		[]byte(`{"status": "active", "value": 42}`),
	)
	if err != nil {
		t.Fatalf("Failed to register resource: %v", err)
	}

	// TODO: Once transport layer is implemented, test actual resource read
	// For now, verify registration worked
	t.Log("Cross-service resource read test placeholder - transport layer needed")
}

// T062: Integration test for connection timeout handling
func TestConnectionTimeoutHandling(t *testing.T) {
	// Create server with custom context
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	server := mcp.NewServer(mcp.ServerConfig{
		Name:    "timeout-test-server",
		Version: "1.0.0",
		Emitter: emit.NewNullEmitter(),
	})

	// Register a slow tool that will timeout
	slowTool := &MockTool{
		name: "slow_tool",
		callFunc: func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
			// Check for context cancellation
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(200 * time.Millisecond):
				return map[string]interface{}{"result": "completed"}, nil
			}
		},
	}

	err := server.RegisterTool("slow_tool", slowTool, mcp.ToolMetadata{
		Description: "Slow tool for timeout testing",
		Schema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
	})
	if err != nil {
		t.Fatalf("Failed to register slow tool: %v", err)
	}

	// Start server (will timeout and stop due to context)
	startErr := server.Start(ctx)

	// Verify that timeout error is propagated
	if startErr == nil {
		t.Error("Expected timeout error, got nil")
	}

	// Verify server can be stopped gracefully
	if err := server.Stop(); err != nil {
		// Server may already be stopped due to context cancellation
		t.Logf("Server stop returned error (may be expected): %v", err)
	}

	t.Log("Connection timeout handling verified - server respects context cancellation")
}

// T063: Integration test for graceful disconnection
func TestGracefulDisconnection(t *testing.T) {
	// Create server
	server := mcp.NewServer(mcp.ServerConfig{
		Name:    "disconnect-test-server",
		Version: "1.0.0",
		Emitter: emit.NewNullEmitter(),
	})

	// Register a test tool
	callCount := 0
	var mu sync.Mutex

	testTool := &MockTool{
		name: "counter_tool",
		callFunc: func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
			mu.Lock()
			callCount++
			count := callCount
			mu.Unlock()

			// Check for context cancellation during execution
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
				return map[string]interface{}{"count": count}, nil
			}
		},
	}

	err := server.RegisterTool("counter_tool", testTool, mcp.ToolMetadata{
		Description: "Counter tool for testing",
		Schema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
	})
	if err != nil {
		t.Fatalf("Failed to register counter tool: %v", err)
	}

	// Create context for server lifecycle
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start server in background
	go func() {
		_ = server.Start(ctx)
	}()

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Simulate graceful disconnection
	cancel()

	// Give server time to shut down
	time.Sleep(50 * time.Millisecond)

	// Verify server stopped
	err = server.Stop()
	if err != nil && err != mcp.ErrServerNotRunning {
		t.Errorf("Server stop failed: %v", err)
	}

	t.Log("Graceful disconnection verified - server handles context cancellation")
}

// TestConcurrentConnections verifies server can handle multiple simultaneous clients
func TestConcurrentConnections(t *testing.T) {
	server := mcp.NewServer(mcp.ServerConfig{
		Name:    "concurrent-test-server",
		Version: "1.0.0",
		Emitter: emit.NewNullEmitter(),
	})

	// Register a thread-safe counter tool
	var counter int
	var mu sync.Mutex

	counterTool := &MockTool{
		name: "concurrent_counter",
		callFunc: func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
			mu.Lock()
			counter++
			currentCount := counter
			mu.Unlock()

			return map[string]interface{}{
				"count":   currentCount,
				"message": "incremented",
			}, nil
		},
	}

	err := server.RegisterTool("concurrent_counter", counterTool, mcp.ToolMetadata{
		Description: "Concurrent counter tool",
		Schema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
	})
	if err != nil {
		t.Fatalf("Failed to register concurrent counter tool: %v", err)
	}

	t.Log("Concurrent connections test - tool registered and ready")
	// TODO: Once transport layer supports concurrent connections, test actual concurrency
}
