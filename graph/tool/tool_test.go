// Package tool provides tool interfaces for graph nodes.
package tool

import (
	"context"
	"errors"
	"testing"
)

// T176: Tool interface tests.

// TestTool_InterfaceContract verifies Tool interface can be implemented.
func TestTool_InterfaceContract(t *testing.T) {
	var _ Tool = (*mockTool)(nil)
}

// mockTool is a minimal Tool implementation for testing.
type mockTool struct {
	name   string
	called bool
	input  map[string]interface{}
	output map[string]interface{}
	err    error
}

func (m *mockTool) Name() string {
	return m.name
}

func (m *mockTool) Call(_ context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	m.called = true
	m.input = input
	if m.err != nil {
		return nil, m.err
	}
	return m.output, nil
}

func TestTool_Name(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
	}{
		{"simple name", "calculator"},
		{"descriptive name", "weather_api"},
		{"namespaced", "tools.database.query"},
		{"with hyphens", "http-client"},
		{"with underscores", "data_processor"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := &mockTool{name: tt.toolName}

			got := tool.Name()
			if got != tt.toolName {
				t.Errorf("Name() = %q, want %q", got, tt.toolName)
			}
		})
	}
}

func TestTool_Call_Success(t *testing.T) {
	t.Run("simple call with map input", func(t *testing.T) {
		tool := &mockTool{
			name:   "echo",
			output: map[string]interface{}{"message": "hello world"},
		}

		ctx := context.Background()
		input := map[string]interface{}{"text": "hello world"}
		result, err := tool.Call(ctx, input)

		if err != nil {
			t.Fatalf("Call() error = %v, want nil", err)
		}
		if result["message"] != "hello world" {
			t.Errorf("Call() = %v, want 'hello world'", result["message"])
		}
		if !tool.called {
			t.Error("Tool.Call() was not called")
		}
		if tool.input["text"] != "hello world" {
			t.Errorf("Tool received input %v, want 'hello world'", tool.input["text"])
		}
	})

	t.Run("call with structured input", func(t *testing.T) {
		input := map[string]interface{}{
			"query": "test",
			"limit": 10,
		}
		output := map[string]interface{}{
			"results": []string{"a", "b"},
			"count":   2,
		}

		tool := &mockTool{
			name:   "search",
			output: output,
		}

		ctx := context.Background()
		result, err := tool.Call(ctx, input)

		if err != nil {
			t.Fatalf("Call() error = %v, want nil", err)
		}

		count, ok := result["count"].(int)
		if !ok {
			t.Fatalf("Call() count field has type %T, want int", result["count"])
		}
		if count != 2 {
			t.Errorf("Call() returned count %d, want 2", count)
		}

		results, ok := result["results"].([]string)
		if !ok {
			t.Fatalf("Call() results field has type %T, want []string", result["results"])
		}
		if len(results) != 2 {
			t.Errorf("Call() returned %d results, want 2", len(results))
		}
	})

	t.Run("call with nil input", func(t *testing.T) {
		tool := &mockTool{
			name:   "no-input",
			output: map[string]interface{}{"status": "done"},
		}

		ctx := context.Background()
		result, err := tool.Call(ctx, nil)

		if err != nil {
			t.Fatalf("Call() error = %v, want nil", err)
		}
		if result["status"] != "done" {
			t.Errorf("Call() status = %v, want 'done'", result["status"])
		}
	})

	t.Run("call with context cancellation", func(t *testing.T) {
		tool := &mockTool{
			name: "context-aware",
			err:  context.Canceled,
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := tool.Call(ctx, nil)

		if err == nil {
			t.Error("Call() error = nil, want context.Canceled")
		}
		if !errors.Is(err, context.Canceled) && err != context.Canceled {
			t.Errorf("Call() error = %v, want context.Canceled", err)
		}
	})
}

func TestTool_Call_Error(t *testing.T) {
	t.Run("tool returns error", func(t *testing.T) {
		expectedErr := errors.New("tool execution failed")
		tool := &mockTool{
			name: "failing-tool",
			err:  expectedErr,
		}

		ctx := context.Background()
		input := map[string]interface{}{"test": "input"}
		result, err := tool.Call(ctx, input)

		if err == nil {
			t.Fatal("Call() error = nil, want error")
		}
		if err != expectedErr {
			t.Errorf("Call() error = %v, want %v", err, expectedErr)
		}
		if result != nil {
			t.Errorf("Call() result = %v, want nil", result)
		}
	})

	t.Run("tool returns wrapped error", func(t *testing.T) {
		baseErr := errors.New("base error")
		wrappedErr := errors.Join(errors.New("wrapper"), baseErr)
		tool := &mockTool{
			name: "error-tool",
			err:  wrappedErr,
		}

		ctx := context.Background()
		_, err := tool.Call(ctx, nil)

		if err == nil {
			t.Fatal("Call() error = nil, want error")
		}
		if !errors.Is(err, baseErr) {
			t.Errorf("Call() error does not wrap base error")
		}
	})
}

func TestTool_MultipleCallsIdempotent(t *testing.T) {
	tool := &mockTool{
		name:   "stateless",
		output: map[string]interface{}{"result": "success"},
	}

	ctx := context.Background()

	// First call.
	input1 := map[string]interface{}{"id": 1}
	result1, err1 := tool.Call(ctx, input1)
	if err1 != nil {
		t.Fatalf("First Call() error = %v", err1)
	}

	// Second call.
	input2 := map[string]interface{}{"id": 2}
	result2, err2 := tool.Call(ctx, input2)
	if err2 != nil {
		t.Fatalf("Second Call() error = %v", err2)
	}

	// Both should succeed.
	if result1["result"] != result2["result"] {
		t.Errorf("Results differ: %v vs %v", result1, result2)
	}
}

// TestTool_NameConsistency verifies Name() returns consistent values.
func TestTool_NameConsistency(t *testing.T) {
	tool := &mockTool{name: "consistent-tool"}

	name1 := tool.Name()
	name2 := tool.Name()
	name3 := tool.Name()

	if name1 != name2 || name2 != name3 {
		t.Errorf("Name() inconsistent: %q, %q, %q", name1, name2, name3)
	}
}

// TestTool_ConcurrentCalls verifies tools are safe for concurrent use.
func TestTool_ConcurrentCalls(t *testing.T) {
	tool := &mockTool{
		name:   "concurrent",
		output: map[string]interface{}{"status": "success"},
	}

	ctx := context.Background()
	const numGoroutines = 10
	errChan := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			input := map[string]interface{}{"id": id}
			_, err := tool.Call(ctx, input)
			errChan <- err
		}(i)
	}

	// Check all calls succeeded.
	for i := 0; i < numGoroutines; i++ {
		if err := <-errChan; err != nil {
			t.Errorf("Concurrent call %d failed: %v", i, err)
		}
	}
}
