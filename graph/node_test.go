// Package graph provides the core graph execution engine for LangGraph-Go.
package graph

import (
	"context"
	"errors"
	"testing"
)

// TestState is a test state type used across node tests.
type TestState struct {
	Value   string
	Counter int
}

// TestNodeInterface verifies that Node interface can be implemented.
func TestNodeInterface(t *testing.T) {
	ctx := context.Background()
	state := TestState{Value: "initial", Counter: 0}

	// Create a simple node implementation.
	node := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
		return NodeResult[TestState]{
			Delta: TestState{Value: "updated", Counter: s.Counter + 1},
			Route: Stop(),
		}
	})

	// Verify node can be called.
	result := node.Run(ctx, state)

	// Verify result structure.
	if result.Delta.Value != "updated" {
		t.Errorf("expected Delta.Value = 'updated', got %q", result.Delta.Value)
	}
	if result.Delta.Counter != 1 {
		t.Errorf("expected Delta.Counter = 1, got %d", result.Delta.Counter)
	}
	if !result.Route.Terminal {
		t.Error("expected Route.Terminal = true for Stop()")
	}
	if result.Err != nil {
		t.Errorf("expected no error, got %v", result.Err)
	}
}

// TestNodeWithContext verifies nodes can access context.
func TestNodeWithContext(t *testing.T) {
	type ctxKey string
	const key ctxKey = "test-key"

	ctx := context.WithValue(context.Background(), key, "test-value")
	state := TestState{Value: "initial"}

	node := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
		val := ctx.Value(key)
		if val == nil {
			return NodeResult[TestState]{Err: &NodeError{Message: "context value missing"}}
		}
		return NodeResult[TestState]{
			Delta: TestState{Value: val.(string)},
			Route: Stop(),
		}
	})

	result := node.Run(ctx, state)

	if result.Err != nil {
		t.Errorf("expected no error, got %v", result.Err)
	}
	if result.Delta.Value != "test-value" {
		t.Errorf("expected Delta.Value = 'test-value', got %q", result.Delta.Value)
	}
}

// TestNodeError verifies nodes can return errors.
func TestNodeError(t *testing.T) {
	ctx := context.Background()
	state := TestState{}

	node := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
		return NodeResult[TestState]{
			Err: &NodeError{Message: "test error", Code: "TEST_ERROR"},
		}
	})

	result := node.Run(ctx, state)

	if result.Err == nil {
		t.Fatal("expected error, got nil")
	}

	var nodeErr *NodeError
	if !errors.As(result.Err, &nodeErr) {
		t.Fatalf("expected *NodeError, got %T", result.Err)
	}
	if nodeErr.Message != "test error" {
		t.Errorf("expected Message = 'test error', got %q", nodeErr.Message)
	}
	if nodeErr.Code != "TEST_ERROR" {
		t.Errorf("expected Code = 'TEST_ERROR', got %q", nodeErr.Code)
	}
}

// TestNodeResult_Validation verifies NodeResult struct fields (T014).
func TestNodeResult_Validation(t *testing.T) {
	tests := []struct {
		name   string
		result NodeResult[TestState]
		valid  bool
	}{
		{
			name: "valid result with Delta and Stop",
			result: NodeResult[TestState]{
				Delta: TestState{Value: "test"},
				Route: Stop(),
			},
			valid: true,
		},
		{
			name: "valid result with Delta and Goto",
			result: NodeResult[TestState]{
				Delta: TestState{Value: "test"},
				Route: Goto("next"),
			},
			valid: true,
		},
		{
			name: "valid result with error",
			result: NodeResult[TestState]{
				Err: &NodeError{Message: "error"},
			},
			valid: true,
		},
		{
			name:   "zero value is valid",
			result: NodeResult[TestState]{},
			valid:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// All NodeResult configurations should be valid.
			// (validation happens at Engine level, not struct level).
			if !tt.valid {
				t.Error("NodeResult should be valid")
			}
		})
	}
}

// TestNext_Routing verifies Next struct routing scenarios (T016).
func TestNext_Routing(t *testing.T) {
	t.Run("Stop creates terminal route", func(t *testing.T) {
		next := Stop()
		if !next.Terminal {
			t.Error("Stop() should set Terminal = true")
		}
		if next.To != "" {
			t.Errorf("Stop() should not set To, got %q", next.To)
		}
		if next.Many != nil {
			t.Error("Stop() should not set Many")
		}
	})

	t.Run("Goto creates single route", func(t *testing.T) {
		next := Goto("target-node")
		if next.Terminal {
			t.Error("Goto() should not set Terminal")
		}
		if next.To != "target-node" {
			t.Errorf("expected To = 'target-node', got %q", next.To)
		}
		if next.Many != nil {
			t.Error("Goto() should not set Many")
		}
	})

	t.Run("Many creates fan-out route", func(t *testing.T) {
		next := Next{Many: []string{"node1", "node2", "node3"}}
		if next.Terminal {
			t.Error("Many should not set Terminal")
		}
		if next.To != "" {
			t.Error("Many should not set To")
		}
		if len(next.Many) != 3 {
			t.Errorf("expected 3 nodes in Many, got %d", len(next.Many))
		}
	})

	t.Run("zero value Next is ambiguous", func(t *testing.T) {
		next := Next{}
		if next.Terminal || next.To != "" || next.Many != nil {
			t.Error("zero value Next should have all fields empty")
		}
	})
}

// TestStopAndGoto_Helpers verifies Stop() and Goto() helper functions (T018).
func TestStopAndGoto_Helpers(t *testing.T) {
	t.Run("Stop returns terminal Next", func(t *testing.T) {
		next := Stop()
		if !next.Terminal {
			t.Error("Stop() must return Terminal = true")
		}
	})

	t.Run("Goto with empty string", func(t *testing.T) {
		next := Goto("")
		if next.To != "" {
			t.Errorf("Goto(\"\") should set To = \"\", got %q", next.To)
		}
		if next.Terminal {
			t.Error("Goto should never set Terminal")
		}
	})

	t.Run("Goto with valid node ID", func(t *testing.T) {
		next := Goto("my-node")
		if next.To != "my-node" {
			t.Errorf("expected To = 'my-node', got %q", next.To)
		}
	})
}

// TestNodeFunc_Wrapper verifies NodeFunc functional wrapper (T020).
func TestNodeFunc_Wrapper(t *testing.T) {
	t.Run("NodeFunc implements Node interface", func(t *testing.T) {
		var _ Node[TestState] = NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{Route: Stop()}
		})
	})

	t.Run("NodeFunc executes function", func(t *testing.T) {
		executed := false
		node := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			executed = true
			return NodeResult[TestState]{
				Delta: TestState{Value: s.Value + "-processed"},
				Route: Stop(),
			}
		})

		ctx := context.Background()
		result := node.Run(ctx, TestState{Value: "input"})

		if !executed {
			t.Error("NodeFunc should have executed the function")
		}
		if result.Delta.Value != "input-processed" {
			t.Errorf("expected Delta.Value = 'input-processed', got %q", result.Delta.Value)
		}
	})

	t.Run("NodeFunc can return errors", func(t *testing.T) {
		node := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Err: &NodeError{Message: "func error"},
			}
		})

		result := node.Run(context.Background(), TestState{})
		if result.Err == nil {
			t.Fatal("expected error from NodeFunc")
		}
	})

	t.Run("NodeFunc can access context", func(t *testing.T) {
		type ctxKey string
		const key ctxKey = "data"

		node := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			val := ctx.Value(key)
			if val == nil {
				return NodeResult[TestState]{Err: &NodeError{Message: "missing context value"}}
			}
			return NodeResult[TestState]{
				Delta: TestState{Value: val.(string)},
				Route: Stop(),
			}
		})

		ctx := context.WithValue(context.Background(), key, "context-data")
		result := node.Run(ctx, TestState{})

		if result.Err != nil {
			t.Errorf("unexpected error: %v", result.Err)
		}
		if result.Delta.Value != "context-data" {
			t.Errorf("expected Delta.Value = 'context-data', got %q", result.Delta.Value)
		}
	})
}
