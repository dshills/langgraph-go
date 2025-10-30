package graph

import (
	"context"
	"testing"

	"github.com/dshills/langgraph-go/graph/emit"
	"github.com/dshills/langgraph-go/graph/store"
)

// testEmitter is a minimal emitter for testing that satisfies the emit.Emitter interface
type testEmitter struct{}

func (e *testEmitter) Emit(event emit.Event) {
	// No-op for testing
}

func (e *testEmitter) EmitBatch(ctx context.Context, events []emit.Event) error {
	return nil
}

func (e *testEmitter) Flush(ctx context.Context) error {
	return nil
}

// TestEngine_ZeroMaxConcurrentNodes verifies proper handling when MaxConcurrentNodes is 0.
// This test ensures that the results channel buffer size doesn't become 0 when
// MaxConcurrentNodes*2 is calculated, which would create an unbuffered channel and
// potentially cause deadlocks when multiple workers try to send results simultaneously.
//
// Related issue: review-report line 869-878 (Critical: Results channel buffer size)
// File: graph/engine.go:815
func TestEngine_ZeroMaxConcurrentNodes(t *testing.T) {
	t.Run("engine with MaxConcurrentNodes=0 should not deadlock", func(t *testing.T) {
		// Create reducer
		reducer := func(prev, delta TestState) TestState {
			prev.Counter += delta.Counter
			if delta.Value != "" {
				prev.Value = delta.Value
			}
			return prev
		}

		// Create engine with MaxConcurrentNodes = 0
		st := store.NewMemStore[TestState]()
		emitter := &testEmitter{}
		opts := Options{
			MaxSteps:           100,
			MaxConcurrentNodes: 0, // Explicitly set to 0 to test edge case
		}
		engine := New(reducer, st, emitter, opts)

		// Create simple node
		testNode := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Counter: 1, Value: "done"},
				Route: Stop(),
			}
		})

		_ = engine.Add("test", testNode)
		_ = engine.StartAt("test")

		// Execute - should not deadlock or panic
		ctx := context.Background()
		final, err := engine.Run(ctx, "zero-concurrent-test", TestState{})

		if err != nil {
			t.Fatalf("Run failed with MaxConcurrentNodes=0: %v", err)
		}

		if final.Counter != 1 {
			t.Errorf("expected Counter=1, got %d", final.Counter)
		}
	})

	t.Run("engine with MaxConcurrentNodes=0 should handle multiple nodes", func(t *testing.T) {
		// Test that even with 0 setting, multiple nodes can execute without deadlock
		reducer := func(prev, delta TestState) TestState {
			prev.Counter += delta.Counter
			return prev
		}

		st := store.NewMemStore[TestState]()
		emitter := &testEmitter{}
		opts := Options{
			MaxSteps:           100,
			MaxConcurrentNodes: 0,
		}
		engine := New(reducer, st, emitter, opts)

		// Create a chain of nodes
		node1 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Counter: 1},
				Route: Goto("node2"),
			}
		})
		node2 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Counter: 1},
				Route: Goto("node3"),
			}
		})
		node3 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Counter: 1},
				Route: Stop(),
			}
		})

		_ = engine.Add("node1", node1)
		_ = engine.Add("node2", node2)
		_ = engine.Add("node3", node3)
		_ = engine.StartAt("node1")

		ctx := context.Background()
		final, err := engine.Run(ctx, "multi-node-test", TestState{})

		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Should have executed 3 nodes
		if final.Counter != 3 {
			t.Errorf("expected Counter=3, got %d", final.Counter)
		}
	})
}

// TestEngine_NilReceiver verifies nil checks on Engine receiver methods.
// These tests ensure that calling methods on a nil Engine pointer returns
// appropriate errors instead of causing nil pointer dereference panics.
//
// Related issue: Critical security - nil pointer dereferences
func TestEngine_NilReceiver(t *testing.T) {
	t.Run("nil engine Run should not panic", func(t *testing.T) {
		var engine *Engine[TestState]

		// Attempt to run - should return error, not panic
		ctx := context.Background()
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Run on nil Engine panicked: %v", r)
			}
		}()

		_, err := engine.Run(ctx, "test", TestState{})
		if err == nil {
			t.Error("expected error from nil Engine.Run, got nil")
		}
	})

	t.Run("nil engine Add should not panic", func(t *testing.T) {
		var engine *Engine[TestState]

		testNode := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{Route: Stop()}
		})

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Add on nil Engine panicked: %v", r)
			}
		}()

		err := engine.Add("test", testNode)
		if err == nil {
			t.Error("expected error from nil Engine.Add, got nil")
		}
	})

	t.Run("nil engine StartAt should not panic", func(t *testing.T) {
		var engine *Engine[TestState]

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("StartAt on nil Engine panicked: %v", r)
			}
		}()

		err := engine.StartAt("test")
		if err == nil {
			t.Error("expected error from nil Engine.StartAt, got nil")
		}
	})
}

// TestEngine_NilState verifies proper handling of nil state scenarios.
// These tests ensure that nil or zero-value state is handled gracefully
// throughout the execution pipeline.
//
// Related issue: Critical security - nil pointer dereferences in state access
func TestEngine_NilState(t *testing.T) {
	t.Run("engine handles zero-value initial state", func(t *testing.T) {
		// Create reducer that handles zero values
		reducer := func(prev, delta TestState) TestState {
			// Should not panic with zero values
			prev.Counter += delta.Counter
			if delta.Value != "" {
				prev.Value = delta.Value
			}
			return prev
		}

		st := store.NewMemStore[TestState]()
		emitter := &testEmitter{}
		opts := Options{MaxSteps: 10}
		engine := New(reducer, st, emitter, opts)

		testNode := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			// Node should receive zero-value state without panic
			return NodeResult[TestState]{
				Delta: TestState{Counter: 1, Value: "initialized"},
				Route: Stop(),
			}
		})

		_ = engine.Add("test", testNode)
		_ = engine.StartAt("test")

		// Pass zero-value state
		ctx := context.Background()
		var zeroState TestState // Explicitly zero-value
		final, err := engine.Run(ctx, "zero-state-test", zeroState)

		if err != nil {
			t.Fatalf("Run failed with zero state: %v", err)
		}

		if final.Counter != 1 {
			t.Errorf("expected Counter=1, got %d", final.Counter)
		}

		if final.Value != "initialized" {
			t.Errorf("expected Value='initialized', got '%s'", final.Value)
		}
	})

	t.Run("node returning nil delta should not panic", func(t *testing.T) {
		reducer := func(prev, delta TestState) TestState {
			// Reducer should handle cases where delta might be zero-value
			prev.Counter += delta.Counter
			return prev
		}

		st := store.NewMemStore[TestState]()
		emitter := &testEmitter{}
		opts := Options{MaxSteps: 10}
		engine := New(reducer, st, emitter, opts)

		// Node that returns zero-value delta
		testNode := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			// Return zero-value delta (effectively no change)
			return NodeResult[TestState]{
				Delta: TestState{}, // Zero value
				Route: Stop(),
			}
		})

		_ = engine.Add("test", testNode)
		_ = engine.StartAt("test")

		ctx := context.Background()
		initial := TestState{Counter: 5, Value: "initial"}
		final, err := engine.Run(ctx, "nil-delta-test", initial)

		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Counter should remain unchanged since delta was zero
		if final.Counter != 5 {
			t.Errorf("expected Counter=5 (unchanged), got %d", final.Counter)
		}
	})
}

// TestEngine_NilStore verifies proper handling when store is nil.
// This ensures the engine validates required dependencies before execution.
func TestEngine_NilStore(t *testing.T) {
	t.Run("engine with nil store returns error on Run", func(t *testing.T) {
		reducer := func(prev, delta TestState) TestState { return prev }
		emitter := &testEmitter{}
		opts := Options{MaxSteps: 10}

		// Create engine with nil store
		engine := New(reducer, nil, emitter, opts)

		testNode := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{Route: Stop()}
		})

		_ = engine.Add("test", testNode)
		_ = engine.StartAt("test")

		ctx := context.Background()
		_, err := engine.Run(ctx, "nil-store-test", TestState{})

		// Should return error about missing store
		if err == nil {
			t.Error("expected error for nil store, got nil")
		}
	})
}

// TestEngine_NilEmitter verifies proper handling when emitter is nil.
// Engine should still work even without an emitter (observability is optional).
func TestEngine_NilEmitter(t *testing.T) {
	t.Run("engine with nil emitter should work", func(t *testing.T) {
		reducer := func(prev, delta TestState) TestState {
			prev.Counter += delta.Counter
			return prev
		}

		st := store.NewMemStore[TestState]()
		opts := Options{MaxSteps: 10}

		// Create engine with nil emitter
		engine := New(reducer, st, nil, opts)

		testNode := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Counter: 1},
				Route: Stop(),
			}
		})

		_ = engine.Add("test", testNode)
		_ = engine.StartAt("test")

		ctx := context.Background()
		final, err := engine.Run(ctx, "nil-emitter-test", TestState{})

		if err != nil {
			t.Fatalf("Run failed with nil emitter: %v", err)
		}

		if final.Counter != 1 {
			t.Errorf("expected Counter=1, got %d", final.Counter)
		}
	})
}
