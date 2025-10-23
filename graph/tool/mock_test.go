package tool

import (
	"context"
	"errors"
	"testing"
)

// TestMockTool_Name verifies Name() behavior (T133).
func TestMockTool_Name(t *testing.T) {
	t.Run("returns configured tool name", func(t *testing.T) {
		mock := &MockTool{
			ToolName: "search_web",
		}

		if mock.Name() != "search_web" {
			t.Errorf("expected Name() = 'search_web', got %q", mock.Name())
		}
	})

	t.Run("returns empty string when not configured", func(t *testing.T) {
		mock := &MockTool{}

		if mock.Name() != "" {
			t.Errorf("expected Name() = '', got %q", mock.Name())
		}
	})
}

// TestMockTool_SingleResponse verifies basic response behavior (T133).
func TestMockTool_SingleResponse(t *testing.T) {
	t.Run("returns configured response", func(t *testing.T) {
		mock := &MockTool{
			ToolName: "calculator",
			Responses: []map[string]interface{}{
				{"result": 42},
			},
		}

		input := map[string]interface{}{"operation": "add", "a": 40, "b": 2}

		output, err := mock.Call(context.Background(), input)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		result, ok := output["result"].(int)
		if !ok || result != 42 {
			t.Errorf("expected result = 42, got %v", output["result"])
		}
	})

	t.Run("repeats last response when exhausted", func(t *testing.T) {
		mock := &MockTool{
			ToolName: "echo",
			Responses: []map[string]interface{}{
				{"echo": "response"},
			},
		}

		input := map[string]interface{}{"message": "test"}

		// First call
		out1, err := mock.Call(context.Background(), input)
		if err != nil {
			t.Fatalf("first call failed: %v", err)
		}

		// Second call should repeat the response
		out2, err := mock.Call(context.Background(), input)
		if err != nil {
			t.Fatalf("second call failed: %v", err)
		}

		if out1["echo"] != out2["echo"] {
			t.Errorf("expected same response, got %v and %v", out1["echo"], out2["echo"])
		}
	})

	t.Run("returns empty map when no responses configured", func(t *testing.T) {
		mock := &MockTool{
			ToolName: "empty_tool",
		}

		input := map[string]interface{}{"test": "data"}

		output, err := mock.Call(context.Background(), input)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if len(output) != 0 {
			t.Errorf("expected empty map, got %v", output)
		}
	})
}

// TestMockTool_MultipleResponses verifies sequence behavior (T133).
func TestMockTool_MultipleResponses(t *testing.T) {
	t.Run("returns responses in sequence", func(t *testing.T) {
		mock := &MockTool{
			ToolName: "counter",
			Responses: []map[string]interface{}{
				{"count": 1},
				{"count": 2},
				{"count": 3},
			},
		}

		input := map[string]interface{}{}

		// Call 1
		out1, err := mock.Call(context.Background(), input)
		if err != nil {
			t.Fatalf("call 1 failed: %v", err)
		}
		if out1["count"] != 1 {
			t.Errorf("call 1: expected count = 1, got %v", out1["count"])
		}

		// Call 2
		out2, err := mock.Call(context.Background(), input)
		if err != nil {
			t.Fatalf("call 2 failed: %v", err)
		}
		if out2["count"] != 2 {
			t.Errorf("call 2: expected count = 2, got %v", out2["count"])
		}

		// Call 3
		out3, err := mock.Call(context.Background(), input)
		if err != nil {
			t.Fatalf("call 3 failed: %v", err)
		}
		if out3["count"] != 3 {
			t.Errorf("call 3: expected count = 3, got %v", out3["count"])
		}

		// Call 4 should repeat last response
		out4, err := mock.Call(context.Background(), input)
		if err != nil {
			t.Fatalf("call 4 failed: %v", err)
		}
		if out4["count"] != 3 {
			t.Errorf("call 4: expected count = 3 (repeat), got %v", out4["count"])
		}
	})
}

// TestMockTool_ErrorInjection verifies error behavior (T133).
func TestMockTool_ErrorInjection(t *testing.T) {
	t.Run("returns configured error", func(t *testing.T) {
		expectedErr := errors.New("tool execution failed")
		mock := &MockTool{
			ToolName: "failing_tool",
			Err:      expectedErr,
			Responses: []map[string]interface{}{
				{"should": "not return"},
			},
		}

		input := map[string]interface{}{"test": "data"}

		_, err := mock.Call(context.Background(), input)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected error %v, got %v", expectedErr, err)
		}
	})

	t.Run("error takes precedence over responses", func(t *testing.T) {
		mock := &MockTool{
			ToolName:  "error_tool",
			Err:       errors.New("error"),
			Responses: []map[string]interface{}{{"data": "value"}},
		}

		input := map[string]interface{}{}

		_, err := mock.Call(context.Background(), input)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

// TestMockTool_CallHistory verifies tracking behavior (T133).
func TestMockTool_CallHistory(t *testing.T) {
	t.Run("records all calls", func(t *testing.T) {
		mock := &MockTool{
			ToolName:  "tracker",
			Responses: []map[string]interface{}{{"ok": true}},
		}

		// Make multiple calls with different inputs
		input1 := map[string]interface{}{"query": "first"}
		input2 := map[string]interface{}{"query": "second", "limit": 10}

		_, _ = mock.Call(context.Background(), input1)
		_, _ = mock.Call(context.Background(), input2)

		if len(mock.Calls) != 2 {
			t.Fatalf("expected 2 calls recorded, got %d", len(mock.Calls))
		}

		// Verify first call
		if len(mock.Calls[0].Input) != 1 {
			t.Errorf("call 0: expected 1 input param, got %d", len(mock.Calls[0].Input))
		}
		if mock.Calls[0].Input["query"] != "first" {
			t.Errorf("call 0: expected query = 'first', got %v", mock.Calls[0].Input["query"])
		}

		// Verify second call
		if len(mock.Calls[1].Input) != 2 {
			t.Errorf("call 1: expected 2 input params, got %d", len(mock.Calls[1].Input))
		}
		if mock.Calls[1].Input["query"] != "second" {
			t.Errorf("call 1: expected query = 'second', got %v", mock.Calls[1].Input["query"])
		}
		if mock.Calls[1].Input["limit"] != 10 {
			t.Errorf("call 1: expected limit = 10, got %v", mock.Calls[1].Input["limit"])
		}
	})

	t.Run("records calls even when error configured", func(t *testing.T) {
		mock := &MockTool{
			ToolName: "error_tracker",
			Err:      errors.New("error"),
		}

		input := map[string]interface{}{"test": "data"}

		_, _ = mock.Call(context.Background(), input)

		if len(mock.Calls) != 1 {
			t.Errorf("expected 1 call recorded, got %d", len(mock.Calls))
		}
	})

	t.Run("records nil input", func(t *testing.T) {
		mock := &MockTool{
			ToolName:  "nil_input_tool",
			Responses: []map[string]interface{}{{"time": "now"}},
		}

		_, _ = mock.Call(context.Background(), nil)

		if len(mock.Calls) != 1 {
			t.Fatalf("expected 1 call recorded, got %d", len(mock.Calls))
		}

		if mock.Calls[0].Input != nil {
			t.Errorf("expected nil input, got %v", mock.Calls[0].Input)
		}
	})
}

// TestMockTool_Reset verifies reset behavior (T133).
func TestMockTool_Reset(t *testing.T) {
	t.Run("clears call history", func(t *testing.T) {
		mock := &MockTool{
			ToolName:  "resettable",
			Responses: []map[string]interface{}{{"ok": true}},
		}

		input := map[string]interface{}{"test": "data"}

		// Make some calls
		_, _ = mock.Call(context.Background(), input)
		_, _ = mock.Call(context.Background(), input)

		if len(mock.Calls) != 2 {
			t.Fatalf("expected 2 calls before reset, got %d", len(mock.Calls))
		}

		// Reset
		mock.Reset()

		if len(mock.Calls) != 0 {
			t.Errorf("expected 0 calls after reset, got %d", len(mock.Calls))
		}
	})

	t.Run("resets response index", func(t *testing.T) {
		mock := &MockTool{
			ToolName: "sequence",
			Responses: []map[string]interface{}{
				{"value": "first"},
				{"value": "second"},
			},
		}

		input := map[string]interface{}{}

		// Exhaust first response
		out1, _ := mock.Call(context.Background(), input)
		if out1["value"] != "first" {
			t.Fatalf("expected 'first', got %v", out1["value"])
		}

		// Reset and verify we get first response again
		mock.Reset()

		out2, _ := mock.Call(context.Background(), input)
		if out2["value"] != "first" {
			t.Errorf("expected 'first' after reset, got %v", out2["value"])
		}
	})
}

// TestMockTool_CallCount verifies count behavior (T133).
func TestMockTool_CallCount(t *testing.T) {
	t.Run("returns correct count", func(t *testing.T) {
		mock := &MockTool{
			ToolName:  "counted",
			Responses: []map[string]interface{}{{"ok": true}},
		}

		if mock.CallCount() != 0 {
			t.Errorf("expected 0 calls initially, got %d", mock.CallCount())
		}

		input := map[string]interface{}{"test": "data"}

		_, _ = mock.Call(context.Background(), input)
		if mock.CallCount() != 1 {
			t.Errorf("expected 1 call, got %d", mock.CallCount())
		}

		_, _ = mock.Call(context.Background(), input)
		if mock.CallCount() != 2 {
			t.Errorf("expected 2 calls, got %d", mock.CallCount())
		}
	})

	t.Run("resets with Reset()", func(t *testing.T) {
		mock := &MockTool{
			ToolName:  "resetcount",
			Responses: []map[string]interface{}{{"ok": true}},
		}

		input := map[string]interface{}{}

		_, _ = mock.Call(context.Background(), input)
		_, _ = mock.Call(context.Background(), input)

		if mock.CallCount() != 2 {
			t.Fatalf("expected 2 calls before reset, got %d", mock.CallCount())
		}

		mock.Reset()

		if mock.CallCount() != 0 {
			t.Errorf("expected 0 calls after reset, got %d", mock.CallCount())
		}
	})
}

// TestMockTool_ContextCancellation verifies context handling (T133).
func TestMockTool_ContextCancellation(t *testing.T) {
	t.Run("respects context cancellation", func(t *testing.T) {
		mock := &MockTool{
			ToolName:  "cancellable",
			Responses: []map[string]interface{}{{"should": "not return"}},
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel before calling

		input := map[string]interface{}{"test": "data"}

		_, err := mock.Call(ctx, input)
		if err == nil {
			t.Fatal("expected context.Canceled error, got nil")
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled error, got %v", err)
		}
	})

	t.Run("does not record call when context cancelled", func(t *testing.T) {
		mock := &MockTool{
			ToolName:  "no_record_cancel",
			Responses: []map[string]interface{}{{"data": "value"}},
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		input := map[string]interface{}{"test": "data"}

		_, _ = mock.Call(ctx, input)

		// Call should not be recorded when context is cancelled
		if mock.CallCount() != 0 {
			t.Errorf("expected 0 calls when context cancelled, got %d", mock.CallCount())
		}
	})
}

// TestMockTool_ComplexResponses verifies complex output handling (T133).
func TestMockTool_ComplexResponses(t *testing.T) {
	t.Run("returns nested structures", func(t *testing.T) {
		mock := &MockTool{
			ToolName: "complex_tool",
			Responses: []map[string]interface{}{
				{
					"results": []interface{}{
						map[string]interface{}{"id": 1, "name": "item1"},
						map[string]interface{}{"id": 2, "name": "item2"},
					},
					"metadata": map[string]interface{}{
						"total": 2,
						"page":  1,
					},
				},
			},
		}

		input := map[string]interface{}{"query": "test"}

		output, err := mock.Call(context.Background(), input)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		results, ok := output["results"].([]interface{})
		if !ok {
			t.Fatal("expected results to be []interface{}")
		}

		if len(results) != 2 {
			t.Errorf("expected 2 results, got %d", len(results))
		}

		metadata, ok := output["metadata"].(map[string]interface{})
		if !ok {
			t.Fatal("expected metadata to be map[string]interface{}")
		}

		total, ok := metadata["total"].(int)
		if !ok || total != 2 {
			t.Errorf("expected total = 2, got %v", metadata["total"])
		}
	})
}

// TestMockTool_Concurrency verifies thread-safety (T133).
func TestMockTool_Concurrency(t *testing.T) {
	t.Run("handles concurrent calls safely", func(t *testing.T) {
		mock := &MockTool{
			ToolName:  "concurrent",
			Responses: []map[string]interface{}{{"ok": true}},
		}

		input := map[string]interface{}{"test": "data"}

		// Launch multiple concurrent calls
		const goroutines = 10
		done := make(chan bool, goroutines)

		for i := 0; i < goroutines; i++ {
			go func() {
				_, _ = mock.Call(context.Background(), input)
				done <- true
			}()
		}

		// Wait for all to complete
		for i := 0; i < goroutines; i++ {
			<-done
		}

		// Verify all calls were recorded
		if mock.CallCount() != goroutines {
			t.Errorf("expected %d calls, got %d", goroutines, mock.CallCount())
		}
	})
}
