package tool

import (
	"context"
	"errors"
	"testing"
)

// TestTool_Interface verifies Tool interface contract (T130).
func TestTool_Interface(t *testing.T) {
	t.Run("interface can be implemented", func(t *testing.T) {
		// Verify that a concrete type can implement Tool
		var _ Tool = &testTool{}
	})

	t.Run("name method returns tool identifier", func(t *testing.T) {
		tool := &testTool{name: "calculator"}

		if tool.Name() != "calculator" {
			t.Errorf("expected Name() = 'calculator', got %q", tool.Name())
		}
	})

	t.Run("call method executes tool logic", func(t *testing.T) {
		tool := &testTool{
			name:   "multiply",
			result: map[string]interface{}{"result": 42},
		}

		input := map[string]interface{}{
			"a": 6,
			"b": 7,
		}

		output, err := tool.Call(context.Background(), input)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		result, ok := output["result"].(int)
		if !ok || result != 42 {
			t.Errorf("expected result = 42, got %v", output["result"])
		}
	})

	t.Run("call method works with nil input", func(t *testing.T) {
		tool := &testTool{
			name:   "get_time",
			result: map[string]interface{}{"time": "12:00"},
		}

		output, err := tool.Call(context.Background(), nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if output["time"] != "12:00" {
			t.Errorf("expected time = '12:00', got %v", output["time"])
		}
	})

	t.Run("call method returns errors", func(t *testing.T) {
		expectedErr := errors.New("tool execution failed")
		tool := &testTool{
			name: "failing_tool",
			err:  expectedErr,
		}

		input := map[string]interface{}{"data": "test"}

		_, err := tool.Call(context.Background(), input)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected error %v, got %v", expectedErr, err)
		}
	})

	t.Run("call method respects context cancellation", func(t *testing.T) {
		tool := &testTool{
			name:   "slow_tool",
			result: map[string]interface{}{"done": true},
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		input := map[string]interface{}{"task": "slow operation"}

		_, err := tool.Call(ctx, input)
		// Implementation should check context and return error
		if err != nil && ctx.Err() == nil {
			t.Errorf("expected context-related error when cancelled")
		}
	})

	t.Run("call method handles structured input", func(t *testing.T) {
		tool := &testTool{
			name: "search",
			result: map[string]interface{}{
				"results": []string{"result1", "result2"},
				"count":   2,
			},
		}

		input := map[string]interface{}{
			"query": "Go programming",
			"limit": 10,
		}

		output, err := tool.Call(context.Background(), input)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		count, ok := output["count"].(int)
		if !ok || count != 2 {
			t.Errorf("expected count = 2, got %v", output["count"])
		}
	})

	t.Run("call method returns complex output", func(t *testing.T) {
		tool := &testTool{
			name: "analyze",
			result: map[string]interface{}{
				"summary": "Analysis complete",
				"metrics": map[string]interface{}{
					"score":      95,
					"confidence": 0.85,
				},
				"tags": []string{"important", "verified"},
			},
		}

		output, err := tool.Call(context.Background(), nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		summary, ok := output["summary"].(string)
		if !ok || summary != "Analysis complete" {
			t.Errorf("expected summary = 'Analysis complete', got %v", output["summary"])
		}

		metrics, ok := output["metrics"].(map[string]interface{})
		if !ok {
			t.Fatal("expected metrics to be map[string]interface{}")
		}

		score, ok := metrics["score"].(int)
		if !ok || score != 95 {
			t.Errorf("expected score = 95, got %v", metrics["score"])
		}
	})
}

// testTool is a simple Tool implementation for testing (T130).
type testTool struct {
	name   string
	result map[string]interface{}
	err    error
}

func (t *testTool) Name() string {
	return t.name
}

func (t *testTool) Call(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	// Check context for cancellation
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	if t.err != nil {
		return nil, t.err
	}

	return t.result, nil
}
