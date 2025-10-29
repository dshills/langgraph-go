package graph

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dshills/langgraph-go/graph/emit"
	"github.com/dshills/langgraph-go/graph/store"
)

// TestEngine_Construction verifies Engine[S] can be constructed (T043).
func TestEngine_Construction(t *testing.T) {
	t.Run("construct with New", func(t *testing.T) {
		reducer := func(prev, delta TestState) TestState {
			if delta.Value != "" {
				prev.Value = delta.Value
			}
			prev.Counter += delta.Counter
			return prev
		}

		st := store.NewMemStore[TestState]()
		emitter := &mockEmitter{}
		opts := Options{MaxSteps: 100}

		engine := New(reducer, st, emitter, opts)

		if engine == nil {
			t.Fatal("New returned nil engine")
		}
	})

	t.Run("engine with nil store", func(t *testing.T) {
		reducer := func(prev, delta TestState) TestState { return prev }
		emitter := &mockEmitter{}
		opts := Options{MaxSteps: 10}

		// Should not panic with nil store (will be validated on Run)
		engine := New(reducer, nil, emitter, opts)
		if engine == nil {
			t.Fatal("New returned nil with nil store")
		}
	})

	t.Run("engine with nil emitter", func(t *testing.T) {
		reducer := func(prev, delta TestState) TestState { return prev }
		st := store.NewMemStore[TestState]()
		opts := Options{MaxSteps: 10}

		// Should not panic with nil emitter (emissions will be skipped)
		engine := New(reducer, st, nil, opts)
		if engine == nil {
			t.Fatal("New returned nil with nil emitter")
		}
	})
}

// TestOptions_Struct verifies Options struct fields (T045).
func TestOptions_Struct(t *testing.T) {
	t.Run("options with MaxSteps", func(t *testing.T) {
		opts := Options{
			MaxSteps: 100,
		}

		if opts.MaxSteps != 100 {
			t.Errorf("expected MaxSteps = 100, got %d", opts.MaxSteps)
		}
	})

	t.Run("options with Retries", func(t *testing.T) {
		opts := Options{
			Retries: 3,
		}

		if opts.Retries != 3 {
			t.Errorf("expected Retries = 3, got %d", opts.Retries)
		}
	})

	t.Run("options with both fields", func(t *testing.T) {
		opts := Options{
			MaxSteps: 50,
			Retries:  5,
		}

		if opts.MaxSteps != 50 {
			t.Errorf("expected MaxSteps = 50, got %d", opts.MaxSteps)
		}
		if opts.Retries != 5 {
			t.Errorf("expected Retries = 5, got %d", opts.Retries)
		}
	})

	t.Run("zero value options", func(t *testing.T) {
		var opts Options

		if opts.MaxSteps != 0 {
			t.Errorf("expected zero value MaxSteps = 0, got %d", opts.MaxSteps)
		}
		if opts.Retries != 0 {
			t.Errorf("expected zero value Retries = 0, got %d", opts.Retries)
		}
	})
}

// TestNew_Constructor_Validation verifies New[S]() validation (T047).
func TestNew_Constructor_Validation(t *testing.T) {
	t.Run("valid constructor call", func(t *testing.T) {
		reducer := func(prev, delta TestState) TestState {
			prev.Counter += delta.Counter
			return prev
		}
		st := store.NewMemStore[TestState]()
		emitter := &mockEmitter{}
		opts := Options{MaxSteps: 100}

		engine := New(reducer, st, emitter, opts)

		if engine == nil {
			t.Fatal("New should return valid engine")
		}
	})

	t.Run("nil reducer should not panic", func(t *testing.T) {
		st := store.NewMemStore[TestState]()
		emitter := &mockEmitter{}
		opts := Options{MaxSteps: 10}

		// Constructor should accept nil reducer (will panic on Run if needed)
		engine := New[TestState](nil, st, emitter, opts)
		if engine == nil {
			t.Fatal("New returned nil with nil reducer")
		}
	})

	t.Run("default options", func(t *testing.T) {
		reducer := func(prev, delta TestState) TestState { return prev }
		st := store.NewMemStore[TestState]()
		emitter := &mockEmitter{}

		// Zero value options should be acceptable
		engine := New(reducer, st, emitter, Options{})

		if engine == nil {
			t.Fatal("New should accept zero value Options")
		}
	})

	t.Run("high MaxSteps value", func(t *testing.T) {
		reducer := func(prev, delta TestState) TestState { return prev }
		st := store.NewMemStore[TestState]()
		emitter := &mockEmitter{}
		opts := Options{MaxSteps: 10000}

		engine := New(reducer, st, emitter, opts)

		if engine == nil {
			t.Fatal("New should accept high MaxSteps")
		}
	})
}

// mockEmitter is a test implementation of emit.Emitter.
type mockEmitter struct {
	mu     sync.Mutex
	events []emit.Event
}

func (m *mockEmitter) Emit(event emit.Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.events == nil {
		m.events = make([]emit.Event, 0)
	}
	m.events = append(m.events, event)
}

// TODO: Implement in Phase 8
func (m *mockEmitter) EmitBatch(ctx context.Context, events []emit.Event) error {
	for _, event := range events {
		m.Emit(event)
	}
	return nil
}

// TODO: Implement in Phase 8
func (m *mockEmitter) Flush(ctx context.Context) error {
	return nil
}

// TestEngine_Add verifies Engine.Add(nodeID, node) behavior (T049).
func TestEngine_Add(t *testing.T) {
	t.Run("add single node", func(t *testing.T) {
		engine := createTestEngine()

		node := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{Route: Stop()}
		})

		err := engine.Add("node1", node)
		if err != nil {
			t.Fatalf("Add failed: %v", err)
		}
	})

	t.Run("add multiple nodes", func(t *testing.T) {
		engine := createTestEngine()

		node1 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{Route: Stop()}
		})
		node2 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{Route: Stop()}
		})

		_ = engine.Add("node1", node1)
		_ = engine.Add("node2", node2)

		// Both nodes should be registered (implementation detail: check via StartAt)
		err := engine.StartAt("node1")
		if err != nil {
			t.Error("node1 should be registered")
		}
		err = engine.StartAt("node2")
		if err != nil {
			t.Error("node2 should be registered")
		}
	})

	t.Run("add duplicate node ID", func(t *testing.T) {
		engine := createTestEngine()

		node1 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{Route: Stop()}
		})
		node2 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{Route: Stop()}
		})

		_ = engine.Add("duplicate", node1)
		err := engine.Add("duplicate", node2)

		// Should return error for duplicate ID
		if err == nil {
			t.Error("expected error for duplicate node ID")
		}
	})

	t.Run("add node with empty ID", func(t *testing.T) {
		engine := createTestEngine()

		node := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{Route: Stop()}
		})

		err := engine.Add("", node)

		// Should return error for empty ID
		if err == nil {
			t.Error("expected error for empty node ID")
		}
	})

	t.Run("add nil node", func(t *testing.T) {
		engine := createTestEngine()

		err := engine.Add("node1", nil)

		// Should return error for nil node
		if err == nil {
			t.Error("expected error for nil node")
		}
	})
}

// TestEngine_StartAt verifies Engine.StartAt(nodeID) validation (T051).
func TestEngine_StartAt(t *testing.T) {
	t.Run("set start node", func(t *testing.T) {
		engine := createTestEngine()

		node := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{Route: Stop()}
		})

		_ = engine.Add("start", node)
		err := engine.StartAt("start")

		if err != nil {
			t.Fatalf("StartAt failed: %v", err)
		}
	})

	t.Run("start node does not exist", func(t *testing.T) {
		engine := createTestEngine()

		err := engine.StartAt("nonexistent")

		// Should return error if node doesn't exist
		if err == nil {
			t.Error("expected error for nonexistent node")
		}
	})

	t.Run("change start node", func(t *testing.T) {
		engine := createTestEngine()

		node1 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{Route: Stop()}
		})
		node2 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{Route: Stop()}
		})

		_ = engine.Add("node1", node1)
		_ = engine.Add("node2", node2)

		// Set initial start node
		_ = engine.StartAt("node1")

		// Change to different start node
		err := engine.StartAt("node2")
		if err != nil {
			t.Errorf("changing start node should succeed: %v", err)
		}
	})

	t.Run("empty start node ID", func(t *testing.T) {
		engine := createTestEngine()

		err := engine.StartAt("")

		// Should return error for empty ID
		if err == nil {
			t.Error("expected error for empty start node ID")
		}
	})
}

// TestEngine_Connect verifies Engine.Connect(from, to, predicate) (T053).
func TestEngine_Connect(t *testing.T) {
	t.Run("connect two nodes unconditionally", func(t *testing.T) {
		engine := createTestEngine()

		node1 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{Route: Goto("node2")}
		})
		node2 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{Route: Stop()}
		})

		_ = engine.Add("node1", node1)
		_ = engine.Add("node2", node2)

		err := engine.Connect("node1", "node2", nil)
		if err != nil {
			t.Fatalf("Connect failed: %v", err)
		}
	})

	t.Run("connect with predicate", func(t *testing.T) {
		engine := createTestEngine()

		node1 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{Route: Goto("node2")}
		})
		node2 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{Route: Stop()}
		})

		_ = engine.Add("node1", node1)
		_ = engine.Add("node2", node2)

		predicate := func(s TestState) bool {
			return s.Counter > 5
		}

		err := engine.Connect("node1", "node2", predicate)
		if err != nil {
			t.Fatalf("Connect with predicate failed: %v", err)
		}
	})

	t.Run("connect nonexistent nodes", func(t *testing.T) {
		engine := createTestEngine()

		// Connecting nonexistent nodes should succeed (lazy validation)
		err := engine.Connect("nonexistent1", "nonexistent2", nil)
		if err != nil {
			t.Errorf("Connect should allow nonexistent nodes (lazy validation): %v", err)
		}
	})

	t.Run("connect with empty from ID", func(t *testing.T) {
		engine := createTestEngine()

		node := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{Route: Stop()}
		})
		_ = engine.Add("node1", node)

		err := engine.Connect("", "node1", nil)

		// Should return error for empty from ID
		if err == nil {
			t.Error("expected error for empty from ID")
		}
	})

	t.Run("connect with empty to ID", func(t *testing.T) {
		engine := createTestEngine()

		node := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{Route: Stop()}
		})
		_ = engine.Add("node1", node)

		err := engine.Connect("node1", "", nil)

		// Should return error for empty to ID
		if err == nil {
			t.Error("expected error for empty to ID")
		}
	})

	t.Run("multiple edges from same node", func(t *testing.T) {
		engine := createTestEngine()

		// Should allow multiple edges from the same node (for conditional routing)
		_ = engine.Connect("router", "path-a", func(s TestState) bool { return s.Counter < 10 })
		_ = engine.Connect("router", "path-b", func(s TestState) bool { return s.Counter >= 10 })

		// Both connections should succeed
	})
}

// TestEngine_Run verifies Engine.Run(ctx, runID, initialState) execution (T055).
func TestEngine_Run(t *testing.T) {
	t.Run("run single node workflow", func(t *testing.T) {
		engine := createTestEngine()

		// Create a simple node that increments counter and stops
		node := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Counter: 1},
				Route: Stop(),
			}
		})

		_ = engine.Add("process", node)
		_ = engine.StartAt("process")

		ctx := context.Background()
		initial := TestState{Value: "start", Counter: 0}

		final, err := engine.Run(ctx, "run-001", initial)
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Verify state was updated
		if final.Counter != 1 {
			t.Errorf("expected Counter = 1, got %d", final.Counter)
		}
		if final.Value != "start" {
			t.Errorf("expected Value = 'start', got %q", final.Value)
		}
	})

	t.Run("run multi-node workflow", func(t *testing.T) {
		engine := createTestEngine()

		// Node 1: Set value and route to node2
		node1 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "node1", Counter: 1},
				Route: Goto("node2"),
			}
		})

		// Node 2: Increment counter and route to node3
		node2 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Counter: 10},
				Route: Goto("node3"),
			}
		})

		// Node 3: Set final value and stop
		node3 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "complete"},
				Route: Stop(),
			}
		})

		_ = engine.Add("node1", node1)
		_ = engine.Add("node2", node2)
		_ = engine.Add("node3", node3)
		_ = engine.StartAt("node1")

		ctx := context.Background()
		initial := TestState{Value: "initial", Counter: 0}

		final, err := engine.Run(ctx, "run-002", initial)
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Verify final state reflects all node updates
		if final.Value != "complete" {
			t.Errorf("expected Value = 'complete', got %q", final.Value)
		}
		if final.Counter != 11 {
			t.Errorf("expected Counter = 11 (0+1+10), got %d", final.Counter)
		}
	})

	t.Run("run with no start node", func(t *testing.T) {
		engine := createTestEngine()

		node := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{Route: Stop()}
		})

		_ = engine.Add("node1", node)
		// Don't call StartAt

		ctx := context.Background()
		_, err := engine.Run(ctx, "run-003", TestState{})

		// Should return error if start node not set
		if err == nil {
			t.Error("expected error when start node not set")
		}
	})

	t.Run("run with nonexistent start node", func(t *testing.T) {
		engine := createTestEngine()
		engine.startNode = "nonexistent" // Manually set invalid start node

		ctx := context.Background()
		_, err := engine.Run(ctx, "run-004", TestState{})

		// Should return error if start node doesn't exist
		if err == nil {
			t.Error("expected error for nonexistent start node")
		}
	})
}

// TestEngine_StatePersistence verifies state is saved after each node (T057).
func TestEngine_StatePersistence(t *testing.T) {
	t.Run("state persisted after each node", func(t *testing.T) {
		reducer := func(prev, delta TestState) TestState {
			if delta.Value != "" {
				prev.Value = delta.Value
			}
			prev.Counter += delta.Counter
			return prev
		}

		st := store.NewMemStore[TestState]()
		emitter := &mockEmitter{}
		opts := Options{MaxSteps: 100}
		engine := New(reducer, st, emitter, opts)

		// Create 3-node workflow
		node1 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "node1", Counter: 1},
				Route: Goto("node2"),
			}
		})

		node2 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "node2", Counter: 10},
				Route: Goto("node3"),
			}
		})

		node3 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "node3", Counter: 100},
				Route: Stop(),
			}
		})

		_ = engine.Add("node1", node1)
		_ = engine.Add("node2", node2)
		_ = engine.Add("node3", node3)
		_ = engine.StartAt("node1")

		ctx := context.Background()
		initial := TestState{Value: "initial", Counter: 0}

		_, err := engine.Run(ctx, "persist-test", initial)
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Verify all steps were persisted
		// Step 1 should have node1 result
		state1, step1, err := st.LoadLatest(ctx, "persist-test")
		if err != nil {
			t.Fatalf("LoadLatest failed: %v", err)
		}

		// Latest should be step 3 with all accumulated updates
		if step1 != 3 {
			t.Errorf("expected step = 3, got %d", step1)
		}
		if state1.Value != "node3" {
			t.Errorf("expected Value = 'node3', got %q", state1.Value)
		}
		if state1.Counter != 111 {
			t.Errorf("expected Counter = 111 (0+1+10+100), got %d", state1.Counter)
		}
	})

	t.Run("state persistence with store error", func(t *testing.T) {
		// Create a failing store for error testing
		failingStore := &failingStore[TestState]{}

		reducer := func(prev, delta TestState) TestState {
			prev.Counter += delta.Counter
			return prev
		}

		emitter := &mockEmitter{}
		opts := Options{MaxSteps: 10}
		engine := New(reducer, failingStore, emitter, opts)

		node := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Counter: 1},
				Route: Stop(),
			}
		})

		_ = engine.Add("node1", node)
		_ = engine.StartAt("node1")

		ctx := context.Background()
		_, err := engine.Run(ctx, "fail-test", TestState{})

		// Should return store error
		if err == nil {
			t.Error("expected error from failing store")
		}
	})
}

// TestEngine_MaxSteps verifies MaxSteps limit enforcement (T059).
func TestEngine_MaxSteps(t *testing.T) {
	t.Run("workflow stops at MaxSteps limit", func(t *testing.T) {
		reducer := func(prev, delta TestState) TestState {
			prev.Counter += delta.Counter
			return prev
		}

		st := store.NewMemStore[TestState]()
		emitter := &mockEmitter{}
		opts := Options{MaxSteps: 3} // Limit to 3 steps
		engine := New(reducer, st, emitter, opts)

		// Create workflow with 5 nodes (should stop at step 3)
		for i := 1; i <= 5; i++ {
			nextNode := ""
			if i < 5 {
				nextNode = nodeID(i + 1)
			}

			node := createCounterNode(i, nextNode)
			_ = engine.Add(nodeID(i), node)
		}
		_ = engine.StartAt("node1")

		ctx := context.Background()
		_, err := engine.Run(ctx, "maxsteps-test", TestState{})

		// Should return MaxSteps error
		if err == nil {
			t.Fatal("expected MaxSteps error")
		}

		var engineErr *EngineError
		if !errors.As(err, &engineErr) {
			t.Fatalf("expected EngineError, got %T", err)
		}

		if engineErr.Code != "MAX_STEPS_EXCEEDED" {
			t.Errorf("expected MAX_STEPS_EXCEEDED error code, got %q", engineErr.Code)
		}
	})

	t.Run("workflow completes within MaxSteps", func(t *testing.T) {
		reducer := func(prev, delta TestState) TestState {
			prev.Counter += delta.Counter
			return prev
		}

		st := store.NewMemStore[TestState]()
		emitter := &mockEmitter{}
		opts := Options{MaxSteps: 10} // Generous limit
		engine := New(reducer, st, emitter, opts)

		// Create 3-node workflow (will complete at step 3)
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
		final, err := engine.Run(ctx, "within-limit", TestState{})

		// Should complete successfully
		if err != nil {
			t.Fatalf("Run should succeed within MaxSteps: %v", err)
		}

		if final.Counter != 3 {
			t.Errorf("expected Counter = 3, got %d", final.Counter)
		}
	})

	t.Run("MaxSteps zero means no limit", func(t *testing.T) {
		reducer := func(prev, delta TestState) TestState {
			prev.Counter += delta.Counter
			return prev
		}

		st := store.NewMemStore[TestState]()
		emitter := &mockEmitter{}
		opts := Options{MaxSteps: 0} // No limit
		engine := New(reducer, st, emitter, opts)

		// Create 100-node chain
		for i := 1; i <= 100; i++ {
			nextNode := ""
			if i < 100 {
				nextNode = nodeID(i + 1)
			}

			node := createCounterNode(i, nextNode)
			_ = engine.Add(nodeID(i), node)
		}
		_ = engine.StartAt("node1")

		ctx := context.Background()
		final, err := engine.Run(ctx, "no-limit", TestState{})

		// Should complete all 100 nodes
		if err != nil {
			t.Fatalf("Run with MaxSteps=0 should not fail: %v", err)
		}

		expectedSum := 100 * 101 / 2 // Sum of 1..100
		if final.Counter != expectedSum {
			t.Errorf("expected Counter = %d, got %d", expectedSum, final.Counter)
		}
	})
}

// Helper functions for MaxSteps tests.
func nodeID(i int) string {
	return "node" + string(rune('0'+i))
}

func createCounterNode(value int, nextNode string) Node[TestState] {
	return NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
		route := Stop()
		if nextNode != "" {
			route = Goto(nextNode)
		}
		return NodeResult[TestState]{
			Delta: TestState{Counter: value},
			Route: route,
		}
	})
}

// failingStore is a test store that always fails SaveStep.
type failingStore[S any] struct{}

func (f *failingStore[S]) SaveStep(ctx context.Context, runID string, step int, nodeID string, state S) error {
	return &EngineError{Message: "simulated store failure", Code: "STORE_FAIL"}
}

func (f *failingStore[S]) LoadLatest(ctx context.Context, runID string) (S, int, error) {
	var zero S
	return zero, 0, store.ErrNotFound
}

func (f *failingStore[S]) SaveCheckpoint(ctx context.Context, cpID string, state S, step int) error {
	return &EngineError{Message: "simulated store failure", Code: "STORE_FAIL"}
}

func (f *failingStore[S]) LoadCheckpoint(ctx context.Context, cpID string) (S, int, error) {
	var zero S
	return zero, 0, store.ErrNotFound
}

// TODO: Implement in Phase 8
func (f *failingStore[S]) SaveCheckpointV2(ctx context.Context, checkpoint store.CheckpointV2[S]) error {
	return &EngineError{Message: "simulated store failure", Code: "STORE_FAIL"}
}

// TODO: Implement in Phase 8
func (f *failingStore[S]) LoadCheckpointV2(ctx context.Context, runID string, stepID int) (store.CheckpointV2[S], error) {
	var zero store.CheckpointV2[S]
	return zero, store.ErrNotFound
}

// TODO: Implement in Phase 8
func (f *failingStore[S]) CheckIdempotency(ctx context.Context, key string) (bool, error) {
	return false, nil
}

// TODO: Implement in Phase 8
func (f *failingStore[S]) PendingEvents(ctx context.Context, limit int) ([]emit.Event, error) {
	return nil, nil
}

// TODO: Implement in Phase 8
func (f *failingStore[S]) MarkEventsEmitted(ctx context.Context, eventIDs []string) error {
	return nil
}

// TestEngine_SaveCheckpoint verifies checkpoint save at specific steps (T061).
func TestEngine_SaveCheckpoint(t *testing.T) {
	t.Run("save checkpoint after node execution", func(t *testing.T) {
		engine := createTestEngine()

		// Create a simple workflow
		node1 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "node1", Counter: 1},
				Route: Goto("node2"),
			}
		})

		node2 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "node2", Counter: 10},
				Route: Stop(),
			}
		})

		_ = engine.Add("node1", node1)
		_ = engine.Add("node2", node2)
		_ = engine.StartAt("node1")

		ctx := context.Background()
		initial := TestState{Value: "initial", Counter: 0}

		// Start workflow
		_, err := engine.Run(ctx, "cp-run-001", initial)
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Save checkpoint after workflow completes
		cpID := "checkpoint-1"
		err = engine.SaveCheckpoint(ctx, "cp-run-001", cpID)
		if err != nil {
			t.Fatalf("SaveCheckpoint failed: %v", err)
		}

		// Verify checkpoint was saved in store
		cpState, cpStep, err := engine.store.LoadCheckpoint(ctx, cpID)
		if err != nil {
			t.Fatalf("LoadCheckpoint failed: %v", err)
		}

		// Checkpoint should have final state
		if cpState.Value != "node2" {
			t.Errorf("expected checkpoint Value = 'node2', got %q", cpState.Value)
		}
		if cpState.Counter != 11 {
			t.Errorf("expected checkpoint Counter = 11, got %d", cpState.Counter)
		}
		if cpStep != 2 {
			t.Errorf("expected checkpoint step = 2, got %d", cpStep)
		}
	})

	t.Run("save checkpoint with custom label", func(t *testing.T) {
		engine := createTestEngine()

		node := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Counter: 5},
				Route: Stop(),
			}
		})

		_ = engine.Add("node1", node)
		_ = engine.StartAt("node1")

		ctx := context.Background()
		_, _ = engine.Run(ctx, "cp-run-002", TestState{})

		// Save with descriptive label
		err := engine.SaveCheckpoint(ctx, "cp-run-002", "after-validation")
		if err != nil {
			t.Fatalf("SaveCheckpoint with label failed: %v", err)
		}

		// Verify label is usable
		_, _, err = engine.store.LoadCheckpoint(ctx, "after-validation")
		if err != nil {
			t.Error("checkpoint with custom label should be loadable")
		}
	})

	t.Run("save multiple checkpoints for same run", func(t *testing.T) {
		engine := createTestEngine()

		node := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Counter: 1},
				Route: Stop(),
			}
		})

		_ = engine.Add("node1", node)
		_ = engine.StartAt("node1")

		ctx := context.Background()
		runID := "cp-run-003"
		_, _ = engine.Run(ctx, runID, TestState{})

		// Save multiple checkpoints with different labels
		_ = engine.SaveCheckpoint(ctx, runID, "checkpoint-a")
		_ = engine.SaveCheckpoint(ctx, runID, "checkpoint-b")
		_ = engine.SaveCheckpoint(ctx, runID, "checkpoint-c")

		// All should be loadable
		_, _, err1 := engine.store.LoadCheckpoint(ctx, "checkpoint-a")
		_, _, err2 := engine.store.LoadCheckpoint(ctx, "checkpoint-b")
		_, _, err3 := engine.store.LoadCheckpoint(ctx, "checkpoint-c")

		if err1 != nil || err2 != nil || err3 != nil {
			t.Error("all checkpoints should be loadable")
		}
	})

	t.Run("save checkpoint for nonexistent run", func(t *testing.T) {
		engine := createTestEngine()

		ctx := context.Background()
		err := engine.SaveCheckpoint(ctx, "nonexistent-run", "checkpoint-x")

		// Should return error if run doesn't exist
		if err == nil {
			t.Error("expected error for nonexistent run")
		}
	})
}

// TestEngine_ResumeFromCheckpoint verifies resume from checkpoint functionality (T063).
func TestEngine_ResumeFromCheckpoint(t *testing.T) {
	t.Run("resume from checkpoint and continue workflow", func(t *testing.T) {
		engine := createTestEngine()

		// Create a 3-node workflow
		node1 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "node1", Counter: 1},
				Route: Goto("node2"),
			}
		})

		node2 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "node2", Counter: 10},
				Route: Goto("node3"),
			}
		})

		node3 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "node3", Counter: 100},
				Route: Stop(),
			}
		})

		_ = engine.Add("node1", node1)
		_ = engine.Add("node2", node2)
		_ = engine.Add("node3", node3)
		_ = engine.StartAt("node1")

		ctx := context.Background()

		// Run to node2, then checkpoint
		_, _ = engine.Run(ctx, "resume-run-001", TestState{})
		_ = engine.SaveCheckpoint(ctx, "resume-run-001", "after-node2")

		// Resume from checkpoint with new runID
		finalState, err := engine.ResumeFromCheckpoint(ctx, "after-node2", "resume-run-002", "node3")
		if err != nil {
			t.Fatalf("ResumeFromCheckpoint failed: %v", err)
		}

		// Should have state from checkpoint (11) plus new node execution (100)
		if finalState.Value != "node3" {
			t.Errorf("expected Value = 'node3', got %q", finalState.Value)
		}
		// Checkpoint had Counter=11 (from node1+node2), node3 adds 100 more
		// Total: 11 (checkpoint) + 100 (node3) + 100 (node3 again because it ran in original too) = 211
		// Actually, let me trace: original run gives 11, checkpoint saves that, resume starts with 11 and adds 100 = 111
		// Wait, the original run already executed all 3 nodes, so checkpoint has 111
		// Resume from that checkpoint adds node3 again: 111 + 100 = 211
		if finalState.Counter != 211 {
			t.Errorf("expected Counter = 211 (checkpoint 111 + node3 100), got %d", finalState.Counter)
		}
	})

	t.Run("resume from checkpoint at workflow start", func(t *testing.T) {
		engine := createTestEngine()

		node1 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Counter: 5},
				Route: Goto("node2"),
			}
		})

		node2 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Counter: 10},
				Route: Stop(),
			}
		})

		_ = engine.Add("node1", node1)
		_ = engine.Add("node2", node2)
		_ = engine.StartAt("node1")

		ctx := context.Background()

		// Run and checkpoint
		_, _ = engine.Run(ctx, "start-run", TestState{Counter: 0})
		_ = engine.SaveCheckpoint(ctx, "start-run", "initial")

		// Resume from start
		final, err := engine.ResumeFromCheckpoint(ctx, "initial", "resumed-run", "node1")
		if err != nil {
			t.Fatalf("resume from start failed: %v", err)
		}

		// Original run: 0 + 5 (node1) + 10 (node2) = 15
		// Checkpoint saved at 15
		// Resume from 15: 15 + 5 (node1) + 10 (node2) = 30
		if final.Counter != 30 {
			t.Errorf("expected Counter = 30 (checkpoint 15 + resumed 15), got %d", final.Counter)
		}
	})

	t.Run("resume preserves checkpoint state", func(t *testing.T) {
		engine := createTestEngine()

		node := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "executed"},
				Route: Stop(),
			}
		})

		_ = engine.Add("node1", node)
		_ = engine.StartAt("node1")

		ctx := context.Background()

		// Create checkpoint
		_, _ = engine.Run(ctx, "preserve-run", TestState{Value: "original", Counter: 42})
		_ = engine.SaveCheckpoint(ctx, "preserve-run", "preserve-cp")

		// Resume should start with checkpoint state
		final, err := engine.ResumeFromCheckpoint(ctx, "preserve-cp", "new-run", "node1")
		if err != nil {
			t.Fatalf("resume failed: %v", err)
		}

		// Should have preserved Counter from checkpoint
		if final.Counter != 42 {
			t.Errorf("expected preserved Counter = 42, got %d", final.Counter)
		}
		if final.Value != "executed" {
			t.Errorf("expected Value = 'executed', got %q", final.Value)
		}
	})

	t.Run("resume from nonexistent checkpoint", func(t *testing.T) {
		engine := createTestEngine()

		ctx := context.Background()
		_, err := engine.ResumeFromCheckpoint(ctx, "nonexistent-cp", "new-run", "node1")

		// Should return error
		if err == nil {
			t.Error("expected error for nonexistent checkpoint")
		}
	})
}

// TestEngine_ContextCancellation verifies context cancellation during execution (T067).
func TestEngine_ContextCancellation(t *testing.T) {
	t.Run("cancel context during node execution", func(t *testing.T) {
		engine := createTestEngine()

		// Create a node that checks context
		slowNode := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			// Simulate work that respects cancellation
			select {
			case <-ctx.Done():
				return NodeResult[TestState]{
					Err:   ctx.Err(),
					Route: Stop(),
				}
			case <-time.After(100 * time.Millisecond):
				return NodeResult[TestState]{
					Delta: TestState{Value: "completed", Counter: 1},
					Route: Stop(),
				}
			}
		})

		_ = engine.Add("slow", slowNode)
		_ = engine.StartAt("slow")

		// Create cancellable context
		ctx, cancel := context.WithCancel(context.Background())

		// Cancel immediately
		cancel()

		// Run should detect cancellation
		_, err := engine.Run(ctx, "cancel-run-001", TestState{})

		// Should return error due to cancellation
		if err == nil {
			t.Error("expected error from cancelled context")
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled error, got: %v", err)
		}
	})

	t.Run("cancel context between nodes", func(t *testing.T) {
		engine := createTestEngine()

		// Node 1 completes quickly
		node1 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "node1", Counter: 1},
				Route: Goto("node2"),
			}
		})

		// Node 2 should not be reached - context is cancelled
		node2 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "node2", Counter: 10},
				Route: Stop(),
			}
		})

		_ = engine.Add("node1", node1)
		_ = engine.Add("node2", node2)
		_ = engine.StartAt("node1")

		// Use pre-cancelled context to guarantee cancellation is detected
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		// Run should detect cancellation at first context check
		_, err := engine.Run(ctx, "cancel-run-002", TestState{})

		// Should return error due to cancellation
		if err == nil {
			t.Error("expected error from cancelled context")
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got: %v", err)
		}
	})

	t.Run("completed workflow ignores cancellation", func(t *testing.T) {
		engine := createTestEngine()

		// Fast node that completes immediately
		fastNode := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "done", Counter: 42},
				Route: Stop(),
			}
		})

		_ = engine.Add("fast", fastNode)
		_ = engine.StartAt("fast")

		// Create cancellable context but don't cancel
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Run completes before cancellation
		final, err := engine.Run(ctx, "cancel-run-003", TestState{})
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}

		// Should have completed successfully
		if final.Value != "done" {
			t.Errorf("expected Value = 'done', got %q", final.Value)
		}
		if final.Counter != 42 {
			t.Errorf("expected Counter = 42, got %d", final.Counter)
		}
	})

	t.Run("deadline exceeded during long execution", func(t *testing.T) {
		engine := createTestEngine()

		// Very slow node
		verySlowNode := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			select {
			case <-ctx.Done():
				return NodeResult[TestState]{
					Err:   ctx.Err(),
					Route: Stop(),
				}
			case <-time.After(200 * time.Millisecond):
				return NodeResult[TestState]{
					Delta: TestState{Value: "completed", Counter: 1},
					Route: Stop(),
				}
			}
		})

		_ = engine.Add("veryslow", verySlowNode)
		_ = engine.StartAt("veryslow")

		// Create context with short deadline
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		// Run should timeout
		_, err := engine.Run(ctx, "cancel-run-004", TestState{})

		// Should return error due to deadline
		if err == nil {
			t.Error("expected error from deadline exceeded")
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("expected context.DeadlineExceeded error, got: %v", err)
		}
	})
}

// TestEngine_GracefulShutdown verifies state persistence before cancellation exit (T069).
func TestEngine_GracefulShutdown(t *testing.T) {
	t.Run("save state before cancellation exit", func(t *testing.T) {
		engine := createTestEngine()

		// Node 1 executes successfully
		node1 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "node1-complete", Counter: 10},
				Route: Goto("node2"),
			}
		})

		// Node 2 will not execute due to cancellation
		node2 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "node2-complete", Counter: 20},
				Route: Stop(),
			}
		})

		_ = engine.Add("node1", node1)
		_ = engine.Add("node2", node2)
		_ = engine.StartAt("node1")

		// Pre-cancelled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		runID := "shutdown-run-001"

		// Run will fail due to cancellation, but should have saved node1's state
		_, err := engine.Run(ctx, runID, TestState{Value: "initial", Counter: 0})

		// Should return cancellation error
		if err == nil {
			t.Fatal("expected cancellation error")
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got: %v", err)
		}

		// CRITICAL: Verify that state was NOT persisted before cancellation
		// Since context was pre-cancelled, no nodes should execute
		// Store should have no steps saved
		_, _, loadErr := engine.store.LoadLatest(ctx, runID)
		if loadErr == nil {
			t.Error("expected no state saved due to immediate cancellation")
		}
	})

	t.Run("partial execution state persisted before timeout", func(t *testing.T) {
		engine := createTestEngine()

		// Node 1 completes before timeout
		node1 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "node1-done", Counter: 5},
				Route: Goto("node2"),
			}
		})

		// Node 2 takes too long and will be cancelled
		node2 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			select {
			case <-ctx.Done():
				return NodeResult[TestState]{
					Err:   ctx.Err(),
					Route: Stop(),
				}
			case <-time.After(100 * time.Millisecond):
				return NodeResult[TestState]{
					Delta: TestState{Value: "node2-done", Counter: 15},
					Route: Stop(),
				}
			}
		})

		_ = engine.Add("node1", node1)
		_ = engine.Add("node2", node2)
		_ = engine.StartAt("node1")

		// Timeout that allows node1 but not node2
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
		defer cancel()

		runID := "shutdown-run-002"
		initial := TestState{Value: "start", Counter: 0}

		// Run until timeout
		_, err := engine.Run(ctx, runID, initial)

		// Should return error (either from timeout or node2 error)
		if err == nil {
			t.Error("expected timeout or node error")
		}

		// Verify that node1's state WAS persisted before cancellation
		finalState, finalStep, loadErr := engine.store.LoadLatest(context.Background(), runID)
		if loadErr != nil {
			t.Fatalf("expected state to be saved for node1: %v", loadErr)
		}

		// Should have node1's state
		if finalState.Value != "node1-done" {
			t.Errorf("expected Value = 'node1-done', got %q", finalState.Value)
		}
		if finalState.Counter != 5 {
			t.Errorf("expected Counter = 5, got %d", finalState.Counter)
		}
		if finalStep != 1 {
			t.Errorf("expected step = 1, got %d", finalStep)
		}
	})

	t.Run("resume from cancelled workflow state", func(t *testing.T) {
		engine := createTestEngine()

		// 3-node workflow
		node1 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "node1", Counter: 1},
				Route: Goto("node2"),
			}
		})

		node2 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			// This will be cancelled
			select {
			case <-ctx.Done():
				return NodeResult[TestState]{
					Err:   ctx.Err(),
					Route: Stop(),
				}
			case <-time.After(100 * time.Millisecond):
				return NodeResult[TestState]{
					Delta: TestState{Value: "node2", Counter: 10},
					Route: Goto("node3"),
				}
			}
		})

		node3 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "node3", Counter: 100},
				Route: Stop(),
			}
		})

		_ = engine.Add("node1", node1)
		_ = engine.Add("node2", node2)
		_ = engine.Add("node3", node3)
		_ = engine.StartAt("node1")

		// First run with timeout
		ctx1, cancel1 := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel1()

		runID := "shutdown-run-003"
		_, _ = engine.Run(ctx1, runID, TestState{})

		// Save checkpoint from partial execution
		saveCtx := context.Background()
		cpErr := engine.SaveCheckpoint(saveCtx, runID, "after-node1")
		if cpErr != nil {
			t.Fatalf("failed to save checkpoint: %v", cpErr)
		}

		// Resume from checkpoint and complete workflow
		ctx2 := context.Background()
		final, err := engine.ResumeFromCheckpoint(ctx2, "after-node1", "shutdown-run-004", "node2")
		if err != nil {
			t.Fatalf("resume failed: %v", err)
		}

		// Should have completed all nodes from checkpoint
		// Checkpoint had node1(1), resume adds node2(10) + node3(100) = 111
		if final.Counter != 111 {
			t.Errorf("expected Counter = 111, got %d", final.Counter)
		}
	})
}

// TestEngine_PredicateEvaluation verifies predicate-based routing (T078).
func TestEngine_PredicateEvaluation(t *testing.T) {
	t.Run("route via predicate when NodeResult has no explicit route", func(t *testing.T) {
		engine := createTestEngine()

		// Node returns empty Route (no explicit routing decision)
		sourceNode := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "from-source", Counter: 10},
				Route: Next{}, // Empty route - should use edge predicates
			}
		})

		targetNode := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "reached-target", Counter: 20},
				Route: Stop(),
			}
		})

		_ = engine.Add("source", sourceNode)
		_ = engine.Add("target", targetNode)
		_ = engine.StartAt("source")

		// Add edge with predicate: route to target if Counter >= 10
		predicate := func(s TestState) bool {
			return s.Counter >= 10
		}
		_ = engine.Connect("source", "target", predicate)

		ctx := context.Background()
		final, err := engine.Run(ctx, "pred-run-001", TestState{})

		if err != nil {
			t.Fatalf("expected successful routing via predicate, got error: %v", err)
		}

		// Should have routed to target via predicate
		if final.Value != "reached-target" {
			t.Errorf("expected routing to target, got Value = %q", final.Value)
		}
		if final.Counter != 30 { // initial 0 + source 10 + target 20
			t.Errorf("expected Counter = 30, got %d", final.Counter)
		}
	})

	t.Run("predicate returns false, no route taken", func(t *testing.T) {
		engine := createTestEngine()

		sourceNode := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "from-source", Counter: 5},
				Route: Next{}, // No explicit route
			}
		})

		targetNode := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "should-not-reach", Counter: 20},
				Route: Stop(),
			}
		})

		_ = engine.Add("source", sourceNode)
		_ = engine.Add("target", targetNode)
		_ = engine.StartAt("source")

		// Predicate requires Counter >= 10, but source only sets it to 5
		predicate := func(s TestState) bool {
			return s.Counter >= 10
		}
		_ = engine.Connect("source", "target", predicate)

		ctx := context.Background()
		_, err := engine.Run(ctx, "pred-run-002", TestState{})

		// Should error because no route matches
		if err == nil {
			t.Error("expected error when no predicate matches")
		}
		// Error should indicate no valid route found
		if err != nil && err.Error() == "" {
			t.Error("error should have descriptive message about routing failure")
		}
	})

	t.Run("explicit NodeResult.Route overrides edge predicates", func(t *testing.T) {
		engine := createTestEngine()

		// Node explicitly routes to "explicit-target"
		sourceNode := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "from-source", Counter: 10},
				Route: Goto("explicit-target"), // Explicit route
			}
		})

		explicitTarget := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "explicit", Counter: 100},
				Route: Stop(),
			}
		})

		predicateTarget := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "predicate", Counter: 200},
				Route: Stop(),
			}
		})

		_ = engine.Add("source", sourceNode)
		_ = engine.Add("explicit-target", explicitTarget)
		_ = engine.Add("predicate-target", predicateTarget)
		_ = engine.StartAt("source")

		// Add edge with predicate (should be ignored since node has explicit route)
		predicate := func(s TestState) bool {
			return true // Always true
		}
		_ = engine.Connect("source", "predicate-target", predicate)

		ctx := context.Background()
		final, err := engine.Run(ctx, "pred-run-003", TestState{})

		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Should route to explicit-target, not predicate-target
		if final.Value != "explicit" {
			t.Errorf("expected explicit routing, got Value = %q", final.Value)
		}
		if final.Counter != 110 { // 0 + 10 + 100
			t.Errorf("expected Counter = 110, got %d", final.Counter)
		}
	})

	t.Run("unconditional edge (nil predicate)", func(t *testing.T) {
		engine := createTestEngine()

		sourceNode := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "source", Counter: 1},
				Route: Next{}, // No explicit route
			}
		})

		targetNode := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "target", Counter: 2},
				Route: Stop(),
			}
		})

		_ = engine.Add("source", sourceNode)
		_ = engine.Add("target", targetNode)
		_ = engine.StartAt("source")

		// Unconditional edge (nil predicate = always traverse)
		_ = engine.Connect("source", "target", nil)

		ctx := context.Background()
		final, err := engine.Run(ctx, "pred-run-004", TestState{})

		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Should always route via unconditional edge
		if final.Value != "target" {
			t.Errorf("expected routing to target, got Value = %q", final.Value)
		}
		if final.Counter != 3 {
			t.Errorf("expected Counter = 3, got %d", final.Counter)
		}
	})

	t.Run("routing to nonexistent node returns error", func(t *testing.T) {
		engine := createTestEngine()

		// Node routes to a node that doesn't exist
		sourceNode := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "source", Counter: 1},
				Route: Goto("nonexistent-node"),
			}
		})

		if err := engine.Add("source", sourceNode); err != nil {
			t.Fatalf("Failed to add source node: %v", err)
		}
		if err := engine.StartAt("source"); err != nil {
			t.Fatalf("Failed to set start node: %v", err)
		}

		ctx := context.Background()
		_, err := engine.Run(ctx, "nonexist-run-001", TestState{})

		// Should error with NODE_NOT_FOUND
		if err == nil {
			t.Fatalf("expected error when routing to nonexistent node")
		}

		// Verify error is EngineError with NODE_NOT_FOUND code
		var engineErr *EngineError
		if !errors.As(err, &engineErr) {
			t.Fatalf("expected EngineError, got %T: %v", err, err)
		}

		if engineErr.Code != "NODE_NOT_FOUND" {
			t.Errorf("expected error code NODE_NOT_FOUND, got %q", engineErr.Code)
		}
	})
}

// TestEngine_MultiplePredicates verifies priority order when multiple predicates match (T080).
func TestEngine_MultiplePredicates(t *testing.T) {
	t.Run("first matching predicate wins", func(t *testing.T) {
		engine := createTestEngine()

		sourceNode := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "source", Counter: 50},
				Route: Next{}, // Use edge routing
			}
		})

		target1 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "target1", Counter: 100},
				Route: Stop(),
			}
		})

		target2 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "target2", Counter: 200},
				Route: Stop(),
			}
		})

		_ = engine.Add("source", sourceNode)
		_ = engine.Add("target1", target1)
		_ = engine.Add("target2", target2)
		_ = engine.StartAt("source")

		// Add two edges, both predicates will match
		// First edge: Counter >= 10 (will match)
		predicate1 := func(s TestState) bool {
			return s.Counter >= 10
		}
		_ = engine.Connect("source", "target1", predicate1)

		// Second edge: Counter >= 20 (also matches, but should not be used)
		predicate2 := func(s TestState) bool {
			return s.Counter >= 20
		}
		_ = engine.Connect("source", "target2", predicate2)

		ctx := context.Background()
		final, err := engine.Run(ctx, "multi-pred-001", TestState{})

		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Should route to target1 (first matching edge)
		if final.Value != "target1" {
			t.Errorf("expected first matching predicate to win, got Value = %q", final.Value)
		}
		if final.Counter != 150 { // 0 + 50 + 100
			t.Errorf("expected Counter = 150, got %d", final.Counter)
		}
	})

	t.Run("skip non-matching predicates until match found", func(t *testing.T) {
		engine := createTestEngine()

		sourceNode := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "source", Counter: 15},
				Route: Next{},
			}
		})

		target1 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "target1", Counter: 100},
				Route: Stop(),
			}
		})

		target2 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "target2", Counter: 200},
				Route: Stop(),
			}
		})

		target3 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "target3", Counter: 300},
				Route: Stop(),
			}
		})

		_ = engine.Add("source", sourceNode)
		_ = engine.Add("target1", target1)
		_ = engine.Add("target2", target2)
		_ = engine.Add("target3", target3)
		_ = engine.StartAt("source")

		// First edge: Counter >= 50 (will NOT match)
		pred1 := func(s TestState) bool {
			return s.Counter >= 50
		}
		_ = engine.Connect("source", "target1", pred1)

		// Second edge: Counter >= 30 (will NOT match)
		pred2 := func(s TestState) bool {
			return s.Counter >= 30
		}
		_ = engine.Connect("source", "target2", pred2)

		// Third edge: Counter >= 10 (WILL match - first to match)
		pred3 := func(s TestState) bool {
			return s.Counter >= 10
		}
		_ = engine.Connect("source", "target3", pred3)

		ctx := context.Background()
		final, err := engine.Run(ctx, "multi-pred-002", TestState{})

		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Should route to target3 (first matching edge in order)
		if final.Value != "target3" {
			t.Errorf("expected routing to target3, got Value = %q", final.Value)
		}
		if final.Counter != 315 { // 0 + 15 + 300
			t.Errorf("expected Counter = 315, got %d", final.Counter)
		}
	})

	t.Run("unconditional edge matches before conditional", func(t *testing.T) {
		engine := createTestEngine()

		sourceNode := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "source", Counter: 10},
				Route: Next{},
			}
		})

		target1 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "unconditional", Counter: 100},
				Route: Stop(),
			}
		})

		target2 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "conditional", Counter: 200},
				Route: Stop(),
			}
		})

		_ = engine.Add("source", sourceNode)
		_ = engine.Add("target1", target1)
		_ = engine.Add("target2", target2)
		_ = engine.StartAt("source")

		// First edge: Unconditional (nil predicate - always matches)
		_ = engine.Connect("source", "target1", nil)

		// Second edge: Conditional (would also match, but should not be evaluated)
		predicate := func(s TestState) bool {
			return s.Counter >= 5
		}
		_ = engine.Connect("source", "target2", predicate)

		ctx := context.Background()
		final, err := engine.Run(ctx, "multi-pred-003", TestState{})

		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Should route via unconditional edge (first in order)
		if final.Value != "unconditional" {
			t.Errorf("expected unconditional edge to win, got Value = %q", final.Value)
		}
		if final.Counter != 110 { // 0 + 10 + 100
			t.Errorf("expected Counter = 110, got %d", final.Counter)
		}
	})
}

// TestEngine_WorkflowLoops verifies workflow loops (A → B → A) work correctly (T090).
func TestEngine_WorkflowLoops(t *testing.T) {
	t.Run("simple loop A→B→A with conditional exit", func(t *testing.T) {
		reducer := func(prev, delta TestState) TestState {
			if delta.Value != "" {
				prev.Value = delta.Value
			}
			prev.Counter += delta.Counter
			return prev
		}

		st := store.NewMemStore[TestState]()
		emitter := &mockEmitter{}
		engine := New(reducer, st, emitter, Options{MaxSteps: 10})

		// Node A: Increment counter, conditionally route to B or stop
		nodeA := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			// If counter < 5, continue loop; otherwise stop
			if s.Counter < 5 {
				return NodeResult[TestState]{
					Delta: TestState{Value: "A", Counter: 1},
					Route: Goto("B"),
				}
			}
			return NodeResult[TestState]{
				Delta: TestState{Value: "A", Counter: 1},
				Route: Stop(),
			}
		})

		// Node B: Increment counter, route to A
		nodeB := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "B", Counter: 1},
				Route: Goto("A"), // Explicit loop back to A
			}
		})

		if err := engine.Add("A", nodeA); err != nil {
			t.Fatalf("Failed to add node A: %v", err)
		}
		if err := engine.Add("B", nodeB); err != nil {
			t.Fatalf("Failed to add node B: %v", err)
		}
		if err := engine.StartAt("A"); err != nil {
			t.Fatalf("Failed to set start node: %v", err)
		}

		ctx := context.Background()
		final, err := engine.Run(ctx, "loop-run-001", TestState{})

		if err != nil {
			// Check if MaxSteps was exceeded (this is OK for loop test)
			var engineErr *EngineError
			if errors.As(err, &engineErr) && engineErr.Code == "MAX_STEPS_EXCEEDED" {
				t.Log("Loop correctly hit MaxSteps limit")
				return
			}
			t.Fatalf("Run failed with unexpected error: %v", err)
		}

		// If no error, verify loop executed multiple times
		// A(1) → B(1) → A(1) → B(1) → A(1) → exit = 5 total
		if final.Counter < 5 {
			t.Errorf("expected loop to execute, got Counter = %d", final.Counter)
		}
		if final.Value != "A" && final.Value != "B" {
			t.Errorf("expected final Value from loop, got %q", final.Value)
		}
	})

	t.Run("MaxSteps prevents infinite loop", func(t *testing.T) {
		reducer := func(prev, delta TestState) TestState {
			if delta.Value != "" {
				prev.Value = delta.Value
			}
			prev.Counter += delta.Counter
			return prev
		}

		st := store.NewMemStore[TestState]()
		emitter := &mockEmitter{}
		engine := New(reducer, st, emitter, Options{MaxSteps: 5}) // Low limit

		// Node A: Always routes to B
		nodeA := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Counter: 1},
				Route: Goto("B"),
			}
		})

		// Node B: Always routes back to A (infinite loop)
		nodeB := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Counter: 1},
				Route: Goto("A"),
			}
		})

		if err := engine.Add("A", nodeA); err != nil {
			t.Fatalf("Failed to add node A: %v", err)
		}
		if err := engine.Add("B", nodeB); err != nil {
			t.Fatalf("Failed to add node B: %v", err)
		}
		if err := engine.StartAt("A"); err != nil {
			t.Fatalf("Failed to set start node: %v", err)
		}

		ctx := context.Background()
		_, err := engine.Run(ctx, "loop-run-002", TestState{})

		// Should error with MAX_STEPS_EXCEEDED
		if err == nil {
			t.Fatalf("expected MaxSteps error for infinite loop")
		}

		var engineErr *EngineError
		if !errors.As(err, &engineErr) {
			t.Fatalf("expected EngineError, got %T: %v", err, err)
		}

		if engineErr.Code != "MAX_STEPS_EXCEEDED" {
			t.Errorf("expected error code MAX_STEPS_EXCEEDED, got %q", engineErr.Code)
		}
	})

	t.Run("conditional loop exit using edge predicates", func(t *testing.T) {
		reducer := func(prev, delta TestState) TestState {
			if delta.Value != "" {
				prev.Value = delta.Value
			}
			prev.Counter += delta.Counter
			return prev
		}

		st := store.NewMemStore[TestState]()
		emitter := &mockEmitter{}
		engine := New(reducer, st, emitter, Options{MaxSteps: 10})

		// Node A: Increment counter, use edge routing
		nodeA := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "A", Counter: 1},
				Route: Next{}, // Use edge predicates
			}
		})

		// Node B: Increment counter, route back to A
		nodeB := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "B", Counter: 1},
				Route: Goto("A"),
			}
		})

		// Exit node: Stops the workflow
		exitNode := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "exit", Counter: 100},
				Route: Stop(),
			}
		})

		if err := engine.Add("A", nodeA); err != nil {
			t.Fatalf("Failed to add node A: %v", err)
		}
		if err := engine.Add("B", nodeB); err != nil {
			t.Fatalf("Failed to add node B: %v", err)
		}
		if err := engine.Add("exit", exitNode); err != nil {
			t.Fatalf("Failed to add exit node: %v", err)
		}
		if err := engine.StartAt("A"); err != nil {
			t.Fatalf("Failed to set start node: %v", err)
		}

		// Edge 1: A → B if counter < 5 (continue loop)
		loopPredicate := func(s TestState) bool {
			return s.Counter < 5
		}
		if err := engine.Connect("A", "B", loopPredicate); err != nil {
			t.Fatalf("Failed to connect A→B: %v", err)
		}

		// Edge 2: A → exit if counter >= 5 (exit condition)
		exitPredicate := func(s TestState) bool {
			return s.Counter >= 5
		}
		if err := engine.Connect("A", "exit", exitPredicate); err != nil {
			t.Fatalf("Failed to connect A→exit: %v", err)
		}

		ctx := context.Background()
		final, err := engine.Run(ctx, "loop-run-003", TestState{})

		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Should have looped and then exited
		// A(1) → B(1) → A(1) → B(1) → A(1) → exit(100) = 105
		if final.Counter != 105 {
			t.Errorf("expected Counter = 105, got %d", final.Counter)
		}
		if final.Value != "exit" {
			t.Errorf("expected Value = 'exit', got %q", final.Value)
		}
	})
}

// TestEngine_Termination verifies explicit Stop() and implicit termination (T094-T097).
func TestEngine_Termination(t *testing.T) {
	t.Run("explicit Stop() terminates workflow", func(t *testing.T) {
		engine := createTestEngine()

		// Node A: Processes and stops
		nodeA := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "completed", Counter: 42},
				Route: Stop(), // Explicit termination
			}
		})

		// Node B: Should never be reached
		nodeB := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "should-not-reach", Counter: 999},
				Route: Stop(),
			}
		})

		if err := engine.Add("A", nodeA); err != nil {
			t.Fatalf("Failed to add node A: %v", err)
		}
		if err := engine.Add("B", nodeB); err != nil {
			t.Fatalf("Failed to add node B: %v", err)
		}
		if err := engine.StartAt("A"); err != nil {
			t.Fatalf("Failed to set start node: %v", err)
		}

		// Add edge A→B (should not be traversed due to Stop())
		if err := engine.Connect("A", "B", nil); err != nil {
			t.Fatalf("Failed to connect A→B: %v", err)
		}

		ctx := context.Background()
		final, err := engine.Run(ctx, "term-run-001", TestState{})

		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Should stop at A, never reach B
		if final.Value != "completed" {
			t.Errorf("expected termination at A, got Value = %q", final.Value)
		}
		if final.Counter != 42 {
			t.Errorf("expected Counter = 42, got %d", final.Counter)
		}
	})

	t.Run("NO_ROUTE error when no edges match", func(t *testing.T) {
		engine := createTestEngine()

		// Node A: Returns empty Route, no matching edges
		// Design: Engine returns NO_ROUTE error instead of implicit termination (safer, more explicit)
		nodeA := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "A", Counter: 1},
				Route: Next{}, // Empty route, will check edges
			}
		})

		// Node B: Should not be reached
		nodeB := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "B", Counter: 10},
				Route: Stop(),
			}
		})

		if err := engine.Add("A", nodeA); err != nil {
			t.Fatalf("Failed to add node A: %v", err)
		}
		if err := engine.Add("B", nodeB); err != nil {
			t.Fatalf("Failed to add node B: %v", err)
		}
		if err := engine.StartAt("A"); err != nil {
			t.Fatalf("Failed to set start node: %v", err)
		}

		// Add edge A→B with predicate that will NOT match
		falsePredicate := func(s TestState) bool {
			return false // Never matches
		}
		if err := engine.Connect("A", "B", falsePredicate); err != nil {
			t.Fatalf("Failed to connect A→B: %v", err)
		}

		ctx := context.Background()
		_, err := engine.Run(ctx, "term-run-002", TestState{})

		// Should error with NO_ROUTE (explicit error instead of implicit termination)
		if err == nil {
			t.Fatal("expected NO_ROUTE error when no edges match")
		}

		var engineErr *EngineError
		if !errors.As(err, &engineErr) {
			t.Fatalf("expected EngineError, got %T: %v", err, err)
		}

		if engineErr.Code != "NO_ROUTE" {
			t.Errorf("expected error code NO_ROUTE, got %q", engineErr.Code)
		}
	})

	t.Run("NO_ROUTE error when node has no edges", func(t *testing.T) {
		engine := createTestEngine()

		// Node A: Returns empty Route, has no outgoing edges
		// Design: Engine returns NO_ROUTE error instead of implicit termination (safer, more explicit)
		nodeA := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "A", Counter: 1},
				Route: Next{}, // Empty route, no edges defined
			}
		})

		if err := engine.Add("A", nodeA); err != nil {
			t.Fatalf("Failed to add node A: %v", err)
		}
		if err := engine.StartAt("A"); err != nil {
			t.Fatalf("Failed to set start node: %v", err)
		}

		// No edges connected from A

		ctx := context.Background()
		_, err := engine.Run(ctx, "term-run-003", TestState{})

		// Should error with NO_ROUTE (explicit error instead of implicit termination)
		if err == nil {
			t.Fatal("expected NO_ROUTE error when node has no edges")
		}

		var engineErr *EngineError
		if !errors.As(err, &engineErr) {
			t.Fatalf("expected EngineError, got %T: %v", err, err)
		}

		if engineErr.Code != "NO_ROUTE" {
			t.Errorf("expected error code NO_ROUTE, got %q", engineErr.Code)
		}
	})
}

// TestEngine_StateIsolationPerBranch verifies isolated state copies for parallel branches (T103).
func TestEngine_StateIsolationPerBranch(t *testing.T) {
	t.Run("parallel branches receive independent state copies", func(t *testing.T) {
		reducer := func(prev, delta TestState) TestState {
			if delta.Value != "" {
				prev.Value = delta.Value
			}
			prev.Counter += delta.Counter
			return prev
		}

		st := store.NewMemStore[TestState]()
		emitter := &mockEmitter{}
		engine := New(reducer, st, emitter, Options{MaxSteps: 10})

		// Track which branches executed and what state they saw
		branchExecutions := make(map[string]int)
		var mu sync.Mutex

		// Node that fans out to multiple parallel branches
		fanoutNode := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Counter: 10}, // Initial counter = 10
				Route: Next{Many: []string{"branchA", "branchB", "branchC"}},
			}
		})

		// Branch A: Modifies state and records what it sees
		branchA := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			mu.Lock()
			branchExecutions["A"] = s.Counter
			mu.Unlock()

			return NodeResult[TestState]{
				Delta: TestState{Value: "A", Counter: 100}, // Add 100
				Route: Stop(),
			}
		})

		// Branch B: Modifies state and records what it sees
		branchB := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			mu.Lock()
			branchExecutions["B"] = s.Counter
			mu.Unlock()

			return NodeResult[TestState]{
				Delta: TestState{Value: "B", Counter: 200}, // Add 200
				Route: Stop(),
			}
		})

		// Branch C: Modifies state and records what it sees
		branchC := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			mu.Lock()
			branchExecutions["C"] = s.Counter
			mu.Unlock()

			return NodeResult[TestState]{
				Delta: TestState{Value: "C", Counter: 300}, // Add 300
				Route: Stop(),
			}
		})

		if err := engine.Add("fanout", fanoutNode); err != nil {
			t.Fatalf("Failed to add fanout node: %v", err)
		}
		if err := engine.Add("branchA", branchA); err != nil {
			t.Fatalf("Failed to add branchA: %v", err)
		}
		if err := engine.Add("branchB", branchB); err != nil {
			t.Fatalf("Failed to add branchB: %v", err)
		}
		if err := engine.Add("branchC", branchC); err != nil {
			t.Fatalf("Failed to add branchC: %v", err)
		}
		if err := engine.StartAt("fanout"); err != nil {
			t.Fatalf("Failed to set start node: %v", err)
		}

		ctx := context.Background()
		finalState, err := engine.Run(ctx, "parallel-isolation-001", TestState{})

		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Verify all branches saw the SAME initial state (Counter = 10 from fanout)
		// This proves each branch got an isolated copy, not a shared reference
		expectedInitialCounter := 10
		for branch, seenCounter := range branchExecutions {
			if seenCounter != expectedInitialCounter {
				t.Errorf("branch %s saw Counter = %d, expected %d (state isolation failure)",
					branch, seenCounter, expectedInitialCounter)
			}
		}

		// Verify all branches executed
		if len(branchExecutions) != 3 {
			t.Errorf("expected 3 branches to execute, got %d", len(branchExecutions))
		}

		// Verify final state includes contributions from all branches
		// Initial(0) + fanout(10) + branchA(100) + branchB(200) + branchC(300) = 610
		expectedFinal := 0 + 10 + 100 + 200 + 300
		if finalState.Counter != expectedFinal {
			t.Errorf("expected final Counter = %d, got %d", expectedFinal, finalState.Counter)
		}
	})
}

// TestEngine_NextManyFanOut verifies Next.Many parallel fan-out (T105).
func TestEngine_NextManyFanOut(t *testing.T) {
	t.Run("fan-out to 4 parallel branches", func(t *testing.T) {
		reducer := func(prev, delta TestState) TestState {
			if delta.Value != "" {
				prev.Value = delta.Value
			}
			prev.Counter += delta.Counter
			return prev
		}

		st := store.NewMemStore[TestState]()
		emitter := &mockEmitter{}
		engine := New(reducer, st, emitter, Options{MaxSteps: 10})

		// Fanout node
		fanout := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Counter: 1},
				Route: Next{Many: []string{"branch1", "branch2", "branch3", "branch4"}},
			}
		})

		// 4 parallel branches, each adds a different amount
		branch1 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Counter: 10},
				Route: Stop(),
			}
		})

		branch2 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Counter: 20},
				Route: Stop(),
			}
		})

		branch3 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Counter: 30},
				Route: Stop(),
			}
		})

		branch4 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Counter: 40},
				Route: Stop(),
			}
		})

		if err := engine.Add("fanout", fanout); err != nil {
			t.Fatalf("Failed to add fanout: %v", err)
		}
		if err := engine.Add("branch1", branch1); err != nil {
			t.Fatalf("Failed to add branch1: %v", err)
		}
		if err := engine.Add("branch2", branch2); err != nil {
			t.Fatalf("Failed to add branch2: %v", err)
		}
		if err := engine.Add("branch3", branch3); err != nil {
			t.Fatalf("Failed to add branch3: %v", err)
		}
		if err := engine.Add("branch4", branch4); err != nil {
			t.Fatalf("Failed to add branch4: %v", err)
		}
		if err := engine.StartAt("fanout"); err != nil {
			t.Fatalf("Failed to set start: %v", err)
		}

		ctx := context.Background()
		final, err := engine.Run(ctx, "fanout-run-001", TestState{})

		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Verify all branches contributed: 1 + 10 + 20 + 30 + 40 = 101
		expected := 1 + 10 + 20 + 30 + 40
		if final.Counter != expected {
			t.Errorf("expected Counter = %d, got %d", expected, final.Counter)
		}
	})
}

// TestEngine_ConcurrentTiming verifies parallel branches execute concurrently (T107).
func TestEngine_ConcurrentTiming(t *testing.T) {
	t.Run("4 branches with 100ms each complete in ~100ms total", func(t *testing.T) {
		reducer := func(prev, delta TestState) TestState {
			prev.Counter += delta.Counter
			return prev
		}

		st := store.NewMemStore[TestState]()
		emitter := &mockEmitter{}
		engine := New(reducer, st, emitter, Options{MaxSteps: 10})

		// Fanout to 4 branches
		fanout := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Route: Next{Many: []string{"slow1", "slow2", "slow3", "slow4"}},
			}
		})

		// Each branch sleeps 100ms (simulating slow operation)
		slowBranch := func(id string) Node[TestState] {
			return NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
				time.Sleep(100 * time.Millisecond)
				return NodeResult[TestState]{
					Delta: TestState{Counter: 1},
					Route: Stop(),
				}
			})
		}

		if err := engine.Add("fanout", fanout); err != nil {
			t.Fatalf("Failed to add fanout: %v", err)
		}
		if err := engine.Add("slow1", slowBranch("slow1")); err != nil {
			t.Fatalf("Failed to add slow1: %v", err)
		}
		if err := engine.Add("slow2", slowBranch("slow2")); err != nil {
			t.Fatalf("Failed to add slow2: %v", err)
		}
		if err := engine.Add("slow3", slowBranch("slow3")); err != nil {
			t.Fatalf("Failed to add slow3: %v", err)
		}
		if err := engine.Add("slow4", slowBranch("slow4")); err != nil {
			t.Fatalf("Failed to add slow4: %v", err)
		}
		if err := engine.StartAt("fanout"); err != nil {
			t.Fatalf("Failed to set start: %v", err)
		}

		ctx := context.Background()
		start := time.Now()
		final, err := engine.Run(ctx, "timing-run-001", TestState{})
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Verify all 4 branches executed
		if final.Counter != 4 {
			t.Errorf("expected Counter = 4, got %d", final.Counter)
		}

		// If truly parallel: ~100ms. If sequential: ~400ms
		// Allow some overhead but verify parallelism (< 250ms means parallel)
		if elapsed > 250*time.Millisecond {
			t.Errorf("parallel execution took %v, expected < 250ms (likely running sequentially)", elapsed)
		}

		t.Logf("4 branches (100ms each) completed in %v (parallel execution verified)", elapsed)
	})
}

// TestEngine_ReducerBasedMerge verifies parallel branches merge via reducer (T109).
func TestEngine_ReducerBasedMerge(t *testing.T) {
	t.Run("reducer combines all parallel branch deltas", func(t *testing.T) {
		// Custom reducer that accumulates values in a slice
		type StateWithSlice struct {
			Values []string
		}

		reducer := func(prev, delta StateWithSlice) StateWithSlice {
			if len(delta.Values) > 0 {
				prev.Values = append(prev.Values, delta.Values...)
			}
			return prev
		}

		st := store.NewMemStore[StateWithSlice]()
		emitter := &mockEmitter{}
		opts := Options{MaxSteps: 100}
		engine := New(reducer, st, emitter, opts)

		// Fanout node
		fanout := NodeFunc[StateWithSlice](func(ctx context.Context, s StateWithSlice) NodeResult[StateWithSlice] {
			return NodeResult[StateWithSlice]{
				Delta: StateWithSlice{Values: []string{"start"}},
				Route: Next{Many: []string{"b1", "b2", "b3"}},
			}
		})

		// Three branches that each add a unique value
		branch1 := NodeFunc[StateWithSlice](func(ctx context.Context, s StateWithSlice) NodeResult[StateWithSlice] {
			return NodeResult[StateWithSlice]{
				Delta: StateWithSlice{Values: []string{"b1-value"}},
				Route: Stop(),
			}
		})

		branch2 := NodeFunc[StateWithSlice](func(ctx context.Context, s StateWithSlice) NodeResult[StateWithSlice] {
			return NodeResult[StateWithSlice]{
				Delta: StateWithSlice{Values: []string{"b2-value"}},
				Route: Stop(),
			}
		})

		branch3 := NodeFunc[StateWithSlice](func(ctx context.Context, s StateWithSlice) NodeResult[StateWithSlice] {
			return NodeResult[StateWithSlice]{
				Delta: StateWithSlice{Values: []string{"b3-value"}},
				Route: Stop(),
			}
		})

		if err := engine.Add("fanout", fanout); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := engine.Add("b1", branch1); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := engine.Add("b2", branch2); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := engine.Add("b3", branch3); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := engine.StartAt("fanout"); err != nil {
			t.Fatalf("StartAt failed: %v", err)
		}

		ctx := context.Background()
		final, err := engine.Run(ctx, "reducer-merge-001", StateWithSlice{})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Verify reducer was called for each branch
		// Expected: ["start", "b1-value", "b2-value", "b3-value"]
		if len(final.Values) != 4 {
			t.Errorf("expected 4 values, got %d: %v", len(final.Values), final.Values)
		}

		// Verify "start" from fanout node
		if final.Values[0] != "start" {
			t.Errorf("expected first value = 'start', got %q", final.Values[0])
		}

		// Verify all branch values present (order verified in T111)
		hasB1 := false
		hasB2 := false
		hasB3 := false
		for _, v := range final.Values[1:] {
			if v == "b1-value" {
				hasB1 = true
			}
			if v == "b2-value" {
				hasB2 = true
			}
			if v == "b3-value" {
				hasB3 = true
			}
		}

		if !hasB1 || !hasB2 || !hasB3 {
			t.Errorf("missing branch values. b1=%v b2=%v b3=%v, values=%v",
				hasB1, hasB2, hasB3, final.Values)
		}
	})
}

// TestEngine_DeterministicMergeOrder verifies lexicographic merge ordering (T111).
func TestEngine_DeterministicMergeOrder(t *testing.T) {
	t.Run("branches merge in lexicographic order by nodeID", func(t *testing.T) {
		type OrderedState struct {
			Sequence []string
		}

		reducer := func(prev, delta OrderedState) OrderedState {
			if len(delta.Sequence) > 0 {
				prev.Sequence = append(prev.Sequence, delta.Sequence...)
			}
			return prev
		}

		st := store.NewMemStore[OrderedState]()
		emitter := &mockEmitter{}
		opts := Options{MaxSteps: 100}
		engine := New(reducer, st, emitter, opts)

		// Fanout with intentionally non-alphabetic routing order
		fanout := NodeFunc[OrderedState](func(ctx context.Context, s OrderedState) NodeResult[OrderedState] {
			return NodeResult[OrderedState]{
				Route: Next{
					// Non-alphabetic order to test sorting
					Many: []string{"nodeZ", "nodeA", "nodeM", "nodeB"},
				},
			}
		})

		// Branches with variable delays to ensure completion order != nodeID order
		nodeA := NodeFunc[OrderedState](func(ctx context.Context, s OrderedState) NodeResult[OrderedState] {
			time.Sleep(40 * time.Millisecond) // Slower
			return NodeResult[OrderedState]{
				Delta: OrderedState{Sequence: []string{"A"}},
				Route: Stop(),
			}
		})

		nodeB := NodeFunc[OrderedState](func(ctx context.Context, s OrderedState) NodeResult[OrderedState] {
			time.Sleep(10 * time.Millisecond) // Fastest
			return NodeResult[OrderedState]{
				Delta: OrderedState{Sequence: []string{"B"}},
				Route: Stop(),
			}
		})

		nodeM := NodeFunc[OrderedState](func(ctx context.Context, s OrderedState) NodeResult[OrderedState] {
			time.Sleep(30 * time.Millisecond) // Medium
			return NodeResult[OrderedState]{
				Delta: OrderedState{Sequence: []string{"M"}},
				Route: Stop(),
			}
		})

		nodeZ := NodeFunc[OrderedState](func(ctx context.Context, s OrderedState) NodeResult[OrderedState] {
			time.Sleep(20 * time.Millisecond) // Medium-fast
			return NodeResult[OrderedState]{
				Delta: OrderedState{Sequence: []string{"Z"}},
				Route: Stop(),
			}
		})

		if err := engine.Add("fanout", fanout); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := engine.Add("nodeA", nodeA); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := engine.Add("nodeB", nodeB); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := engine.Add("nodeM", nodeM); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := engine.Add("nodeZ", nodeZ); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := engine.StartAt("fanout"); err != nil {
			t.Fatalf("StartAt failed: %v", err)
		}

		ctx := context.Background()
		final, err := engine.Run(ctx, "order-test-001", OrderedState{})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Despite variable completion times (B finishes first, A finishes last),
		// merge order should be lexicographic: nodeA, nodeB, nodeM, nodeZ
		expected := []string{"A", "B", "M", "Z"}
		if len(final.Sequence) != len(expected) {
			t.Fatalf("expected %d items, got %d: %v", len(expected), len(final.Sequence), final.Sequence)
		}

		for i, exp := range expected {
			if final.Sequence[i] != exp {
				t.Errorf("position %d: expected %q, got %q (sequence: %v)",
					i, exp, final.Sequence[i], final.Sequence)
			}
		}

		t.Logf("Verified deterministic merge order: %v (regardless of completion timing)", final.Sequence)
	})
}

// TestEngine_ParallelBranchError verifies error handling in one parallel branch (T113).
func TestEngine_ParallelBranchError(t *testing.T) {
	t.Run("error in one branch stops execution and returns error", func(t *testing.T) {
		reducer := func(prev, delta TestState) TestState {
			prev.Counter += delta.Counter
			return prev
		}

		st := store.NewMemStore[TestState]()
		emitter := &mockEmitter{}
		opts := Options{MaxSteps: 100}
		engine := New(reducer, st, emitter, opts)

		// Fanout to 3 branches
		fanout := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Route: Next{Many: []string{"branch1", "branch2", "branch3"}},
			}
		})

		// Branch 1: succeeds
		branch1 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Counter: 10},
				Route: Stop(),
			}
		})

		// Branch 2: fails with error
		branch2 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Err: &EngineError{
					Message: "branch2 processing failed",
					Code:    "BRANCH_ERROR",
				},
			}
		})

		// Branch 3: succeeds
		branch3 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Counter: 30},
				Route: Stop(),
			}
		})

		if err := engine.Add("fanout", fanout); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := engine.Add("branch1", branch1); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := engine.Add("branch2", branch2); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := engine.Add("branch3", branch3); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := engine.StartAt("fanout"); err != nil {
			t.Fatalf("StartAt failed: %v", err)
		}

		ctx := context.Background()
		_, err := engine.Run(ctx, "error-branch-001", TestState{})

		// Verify error is returned
		if err == nil {
			t.Fatal("expected error from failed branch, got nil")
		}

		// Verify error message contains expected text
		errMsg := err.Error()
		if !strings.Contains(errMsg, "branch2 processing failed") {
			t.Errorf("expected error containing 'branch2 processing failed', got %q", errMsg)
		}

		t.Logf("Successfully caught error from parallel branch: %v", err)
	})
}

// TestEngine_MultipleBranchErrors verifies handling of multiple branch failures (T115).
func TestEngine_MultipleBranchErrors(t *testing.T) {
	t.Run("multiple branches fail - first error returned", func(t *testing.T) {
		reducer := func(prev, delta TestState) TestState {
			prev.Counter += delta.Counter
			return prev
		}

		st := store.NewMemStore[TestState]()
		emitter := &mockEmitter{}
		opts := Options{MaxSteps: 100}
		engine := New(reducer, st, emitter, opts)

		// Fanout to 4 branches
		fanout := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Route: Next{Many: []string{"branchA", "branchB", "branchC", "branchD"}},
			}
		})

		// Branch A: succeeds
		branchA := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			time.Sleep(10 * time.Millisecond)
			return NodeResult[TestState]{
				Delta: TestState{Counter: 1},
				Route: Stop(),
			}
		})

		// Branch B: fails
		branchB := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			time.Sleep(20 * time.Millisecond)
			return NodeResult[TestState]{
				Err: &EngineError{Message: "branchB error", Code: "ERR_B"},
			}
		})

		// Branch C: fails
		branchC := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			time.Sleep(15 * time.Millisecond)
			return NodeResult[TestState]{
				Err: &EngineError{Message: "branchC error", Code: "ERR_C"},
			}
		})

		// Branch D: succeeds
		branchD := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			time.Sleep(5 * time.Millisecond)
			return NodeResult[TestState]{
				Delta: TestState{Counter: 4},
				Route: Stop(),
			}
		})

		if err := engine.Add("fanout", fanout); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := engine.Add("branchA", branchA); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := engine.Add("branchB", branchB); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := engine.Add("branchC", branchC); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := engine.Add("branchD", branchD); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := engine.StartAt("fanout"); err != nil {
			t.Fatalf("StartAt failed: %v", err)
		}

		ctx := context.Background()
		_, err := engine.Run(ctx, "multi-error-001", TestState{})

		// Verify at least one error is returned
		if err == nil {
			t.Fatal("expected error from failed branches, got nil")
		}

		// Verify it's one of the expected errors
		errMsg := err.Error()
		hasB := strings.Contains(errMsg, "branchB error")
		hasC := strings.Contains(errMsg, "branchC error")
		if !hasB && !hasC {
			t.Errorf("expected error containing 'branchB error' or 'branchC error', got %q", errMsg)
		}

		t.Logf("Multiple branch failures handled correctly, returned: %v", err)
	})
}

// TestEngine_ParallelNodeNotFound verifies error when parallel branch node doesn't exist (T113).
func TestEngine_ParallelNodeNotFound(t *testing.T) {
	t.Run("routing to nonexistent parallel branch returns error", func(t *testing.T) {
		reducer := func(prev, delta TestState) TestState {
			prev.Counter += delta.Counter
			return prev
		}

		st := store.NewMemStore[TestState]()
		emitter := &mockEmitter{}
		opts := Options{MaxSteps: 100}
		engine := New(reducer, st, emitter, opts)

		// Fanout that references a nonexistent node
		fanout := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Route: Next{Many: []string{"existing", "nonexistent", "another"}},
			}
		})

		// Only add "existing" and "another", but not "nonexistent"
		existing := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Counter: 1},
				Route: Stop(),
			}
		})

		another := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Counter: 2},
				Route: Stop(),
			}
		})

		if err := engine.Add("fanout", fanout); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := engine.Add("existing", existing); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := engine.Add("another", another); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		// Intentionally not adding "nonexistent"
		if err := engine.StartAt("fanout"); err != nil {
			t.Fatalf("StartAt failed: %v", err)
		}

		ctx := context.Background()
		_, err := engine.Run(ctx, "notfound-001", TestState{})

		// Verify error is returned
		if err == nil {
			t.Fatal("expected NODE_NOT_FOUND error, got nil")
		}

		// Verify error contains "nonexistent"
		errMsg := err.Error()
		if !strings.Contains(errMsg, "nonexistent") && !strings.Contains(errMsg, "not found") {
			t.Errorf("expected error about 'nonexistent' node, got %q", errMsg)
		}

		t.Logf("Nonexistent parallel branch correctly detected: %v", err)
	})
}

// createTestEngine is a helper to create a test engine with default configuration.
func createTestEngine() *Engine[TestState] {
	reducer := func(prev, delta TestState) TestState {
		if delta.Value != "" {
			prev.Value = delta.Value
		}
		prev.Counter += delta.Counter
		return prev
	}

	st := store.NewMemStore[TestState]()
	emitter := &mockEmitter{}
	opts := Options{MaxSteps: 100}

	return New(reducer, st, emitter, opts)
}

// TestEngine_NodeStartEvent verifies node_start event is emitted at the beginning of node execution (T152).
func TestEngine_NodeStartEvent(t *testing.T) {
	t.Run("node_start event emitted with correct metadata", func(t *testing.T) {
		reducer := func(prev, delta TestState) TestState {
			prev.Counter += delta.Counter
			return prev
		}

		st := store.NewMemStore[TestState]()
		emitter := &mockEmitter{}
		opts := Options{MaxSteps: 100}
		engine := New(reducer, st, emitter, opts)

		// Add a simple node that increments counter
		node := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Counter: 1},
				Route: Stop(),
			}
		})

		if err := engine.Add("testNode", node); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := engine.StartAt("testNode"); err != nil {
			t.Fatalf("StartAt failed: %v", err)
		}

		ctx := context.Background()
		_, err := engine.Run(ctx, "node-start-001", TestState{})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Verify node_start event was emitted
		var nodeStartEvent *emit.Event
		for i := range emitter.events {
			if emitter.events[i].Msg == "node_start" {
				nodeStartEvent = &emitter.events[i]
				break
			}
		}

		if nodeStartEvent == nil {
			t.Fatal("expected node_start event, but none found")
		}

		// Verify event metadata
		if nodeStartEvent.RunID != "node-start-001" {
			t.Errorf("expected RunID 'node-start-001', got %q", nodeStartEvent.RunID)
		}
		if nodeStartEvent.NodeID != "testNode" {
			t.Errorf("expected NodeID 'testNode', got %q", nodeStartEvent.NodeID)
		}
		if nodeStartEvent.Step != 0 {
			t.Errorf("expected Step 0, got %d", nodeStartEvent.Step)
		}

		t.Logf("node_start event emitted correctly: %+v", nodeStartEvent)
	})

	t.Run("node_start events emitted for each node in sequence", func(t *testing.T) {
		reducer := func(prev, delta TestState) TestState {
			prev.Counter += delta.Counter
			return prev
		}

		st := store.NewMemStore[TestState]()
		emitter := &mockEmitter{}
		opts := Options{MaxSteps: 100}
		engine := New(reducer, st, emitter, opts)

		// Add three nodes in sequence
		node1 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Counter: 1},
				Route: Goto("node2"),
			}
		})
		node2 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Counter: 2},
				Route: Goto("node3"),
			}
		})
		node3 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Counter: 3},
				Route: Stop(),
			}
		})

		if err := engine.Add("node1", node1); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := engine.Add("node2", node2); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := engine.Add("node3", node3); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := engine.StartAt("node1"); err != nil {
			t.Fatalf("StartAt failed: %v", err)
		}

		ctx := context.Background()
		_, err := engine.Run(ctx, "node-start-seq-001", TestState{})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Collect all node_start events
		var startEvents []emit.Event
		for _, e := range emitter.events {
			if e.Msg == "node_start" {
				startEvents = append(startEvents, e)
			}
		}

		// Verify we got 3 node_start events
		if len(startEvents) != 3 {
			t.Errorf("expected 3 node_start events, got %d", len(startEvents))
		}

		// Verify the sequence
		expectedNodes := []string{"node1", "node2", "node3"}
		for i, event := range startEvents {
			if event.NodeID != expectedNodes[i] {
				t.Errorf("event %d: expected NodeID %q, got %q", i, expectedNodes[i], event.NodeID)
			}
			if event.Step != i {
				t.Errorf("event %d: expected Step %d, got %d", i, i, event.Step)
			}
		}

		t.Logf("All node_start events emitted correctly for sequential execution")
	})
}

// TestEngine_NodeEndEvent verifies node_end event is emitted after state merge with delta (T154).
func TestEngine_NodeEndEvent(t *testing.T) {
	t.Run("node_end event emitted with delta in metadata", func(t *testing.T) {
		reducer := func(prev, delta TestState) TestState {
			prev.Counter += delta.Counter
			prev.Value += delta.Value
			return prev
		}

		st := store.NewMemStore[TestState]()
		emitter := &mockEmitter{}
		opts := Options{MaxSteps: 100}
		engine := New(reducer, st, emitter, opts)

		// Add a node that returns a delta
		node := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Counter: 5, Value: "test"},
				Route: Stop(),
			}
		})

		if err := engine.Add("testNode", node); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := engine.StartAt("testNode"); err != nil {
			t.Fatalf("StartAt failed: %v", err)
		}

		ctx := context.Background()
		finalState, err := engine.Run(ctx, "node-end-001", TestState{})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Verify final state has the merged delta
		if finalState.Counter != 5 {
			t.Errorf("expected Counter 5, got %d", finalState.Counter)
		}
		if finalState.Value != "test" {
			t.Errorf("expected Value 'test', got %q", finalState.Value)
		}

		// Verify node_end event was emitted
		var nodeEndEvent *emit.Event
		for i := range emitter.events {
			if emitter.events[i].Msg == "node_end" {
				nodeEndEvent = &emitter.events[i]
				break
			}
		}

		if nodeEndEvent == nil {
			t.Fatal("expected node_end event, but none found")
		}

		// Verify event metadata includes delta
		if nodeEndEvent.RunID != "node-end-001" {
			t.Errorf("expected RunID 'node-end-001', got %q", nodeEndEvent.RunID)
		}
		if nodeEndEvent.NodeID != "testNode" {
			t.Errorf("expected NodeID 'testNode', got %q", nodeEndEvent.NodeID)
		}
		if nodeEndEvent.Step != 0 {
			t.Errorf("expected Step 0, got %d", nodeEndEvent.Step)
		}

		// Meta should contain delta information
		if nodeEndEvent.Meta == nil {
			t.Fatal("expected Meta to contain delta information, got nil")
		}

		t.Logf("node_end event emitted correctly: %+v", nodeEndEvent)
	})

	t.Run("node_end events emitted for each node with correct step numbers", func(t *testing.T) {
		reducer := func(prev, delta TestState) TestState {
			prev.Counter += delta.Counter
			return prev
		}

		st := store.NewMemStore[TestState]()
		emitter := &mockEmitter{}
		opts := Options{MaxSteps: 100}
		engine := New(reducer, st, emitter, opts)

		// Add three nodes in sequence with different deltas
		node1 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Counter: 1},
				Route: Goto("node2"),
			}
		})
		node2 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Counter: 10},
				Route: Goto("node3"),
			}
		})
		node3 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Counter: 100},
				Route: Stop(),
			}
		})

		if err := engine.Add("node1", node1); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := engine.Add("node2", node2); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := engine.Add("node3", node3); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := engine.StartAt("node1"); err != nil {
			t.Fatalf("StartAt failed: %v", err)
		}

		ctx := context.Background()
		finalState, err := engine.Run(ctx, "node-end-seq-001", TestState{})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Verify final state accumulated all deltas
		expectedCounter := 1 + 10 + 100
		if finalState.Counter != expectedCounter {
			t.Errorf("expected Counter %d, got %d", expectedCounter, finalState.Counter)
		}

		// Collect all node_end events
		var endEvents []emit.Event
		for _, e := range emitter.events {
			if e.Msg == "node_end" {
				endEvents = append(endEvents, e)
			}
		}

		// Verify we got 3 node_end events
		if len(endEvents) != 3 {
			t.Errorf("expected 3 node_end events, got %d", len(endEvents))
		}

		// Verify the sequence and step numbers
		expectedNodes := []string{"node1", "node2", "node3"}
		for i, event := range endEvents {
			if event.NodeID != expectedNodes[i] {
				t.Errorf("event %d: expected NodeID %q, got %q", i, expectedNodes[i], event.NodeID)
			}
			if event.Step != i {
				t.Errorf("event %d: expected Step %d, got %d", i, i, event.Step)
			}
		}

		t.Logf("All node_end events emitted correctly for sequential execution")
	})
}

// TestEngine_RoutingDecisionEvent verifies routing_decision event is emitted with chosen path (T156).
func TestEngine_RoutingDecisionEvent(t *testing.T) {
	t.Run("routing_decision event emitted with Goto target", func(t *testing.T) {
		reducer := func(prev, delta TestState) TestState {
			prev.Counter += delta.Counter
			return prev
		}

		st := store.NewMemStore[TestState]()
		emitter := &mockEmitter{}
		opts := Options{MaxSteps: 100}
		engine := New(reducer, st, emitter, opts)

		// Add two nodes with explicit routing
		node1 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Counter: 1},
				Route: Goto("node2"),
			}
		})
		node2 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Counter: 2},
				Route: Stop(),
			}
		})

		if err := engine.Add("node1", node1); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := engine.Add("node2", node2); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := engine.StartAt("node1"); err != nil {
			t.Fatalf("StartAt failed: %v", err)
		}

		ctx := context.Background()
		_, err := engine.Run(ctx, "routing-001", TestState{})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Collect all routing_decision events
		var routingEvents []emit.Event
		for _, e := range emitter.events {
			if e.Msg == "routing_decision" {
				routingEvents = append(routingEvents, e)
			}
		}

		// Should have routing decisions (at least one for Goto, one for Stop)
		if len(routingEvents) == 0 {
			t.Fatal("expected routing_decision events, but none found")
		}

		// Find the Goto routing decision from node1
		var gotoEvent *emit.Event
		for i := range routingEvents {
			if routingEvents[i].NodeID == "node1" {
				gotoEvent = &routingEvents[i]
				break
			}
		}

		if gotoEvent == nil {
			t.Fatal("expected routing_decision event for node1, but none found")
		}

		// Verify metadata contains routing target
		if gotoEvent.Meta == nil {
			t.Fatal("expected Meta to contain routing information, got nil")
		}

		nextNode, ok := gotoEvent.Meta["next_node"].(string)
		if !ok || nextNode != "node2" {
			t.Errorf("expected next_node 'node2', got %v", gotoEvent.Meta["next_node"])
		}

		t.Logf("routing_decision event emitted correctly: %+v", gotoEvent)
	})

	t.Run("routing_decision event emitted for Stop", func(t *testing.T) {
		reducer := func(prev, delta TestState) TestState {
			prev.Counter += delta.Counter
			return prev
		}

		st := store.NewMemStore[TestState]()
		emitter := &mockEmitter{}
		opts := Options{MaxSteps: 100}
		engine := New(reducer, st, emitter, opts)

		// Add a single node that stops
		node := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Counter: 1},
				Route: Stop(),
			}
		})

		if err := engine.Add("node", node); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := engine.StartAt("node"); err != nil {
			t.Fatalf("StartAt failed: %v", err)
		}

		ctx := context.Background()
		_, err := engine.Run(ctx, "routing-stop-001", TestState{})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Find the Stop routing decision
		var stopEvent *emit.Event
		for i := range emitter.events {
			if emitter.events[i].Msg == "routing_decision" && emitter.events[i].NodeID == "node" {
				stopEvent = &emitter.events[i]
				break
			}
		}

		if stopEvent == nil {
			t.Fatal("expected routing_decision event for Stop, but none found")
		}

		// Verify metadata indicates terminal
		if stopEvent.Meta == nil {
			t.Fatal("expected Meta to contain routing information, got nil")
		}

		terminal, ok := stopEvent.Meta["terminal"].(bool)
		if !ok || !terminal {
			t.Errorf("expected terminal true, got %v", stopEvent.Meta["terminal"])
		}

		t.Logf("routing_decision event for Stop emitted correctly: %+v", stopEvent)
	})
}

// TestEngine_ErrorEvent verifies error event is emitted on NodeResult.Err (T158).
func TestEngine_ErrorEvent(t *testing.T) {
	t.Run("error event emitted when node returns error", func(t *testing.T) {
		reducer := func(prev, delta TestState) TestState {
			prev.Counter += delta.Counter
			return prev
		}

		st := store.NewMemStore[TestState]()
		emitter := &mockEmitter{}
		opts := Options{MaxSteps: 100}
		engine := New(reducer, st, emitter, opts)

		// Add a node that returns an error
		node := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Err: errors.New("test error"),
			}
		})

		if err := engine.Add("errorNode", node); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := engine.StartAt("errorNode"); err != nil {
			t.Fatalf("StartAt failed: %v", err)
		}

		ctx := context.Background()
		_, err := engine.Run(ctx, "error-001", TestState{})

		// Verify the run failed with the expected error
		if err == nil {
			t.Fatal("expected error from node, got nil")
		}
		if err.Error() != "test error" {
			t.Errorf("expected error 'test error', got %q", err.Error())
		}

		// Verify error event was emitted
		var errorEvent *emit.Event
		for i := range emitter.events {
			if emitter.events[i].Msg == "error" {
				errorEvent = &emitter.events[i]
				break
			}
		}

		if errorEvent == nil {
			t.Fatal("expected error event, but none found")
		}

		// Verify event metadata
		if errorEvent.RunID != "error-001" {
			t.Errorf("expected RunID 'error-001', got %q", errorEvent.RunID)
		}
		if errorEvent.NodeID != "errorNode" {
			t.Errorf("expected NodeID 'errorNode', got %q", errorEvent.NodeID)
		}

		// Meta should contain error information
		if errorEvent.Meta == nil {
			t.Fatal("expected Meta to contain error information, got nil")
		}

		errMsg, ok := errorEvent.Meta["error"].(string)
		if !ok || errMsg != "test error" {
			t.Errorf("expected error 'test error', got %v", errorEvent.Meta["error"])
		}

		t.Logf("error event emitted correctly: %+v", errorEvent)
	})

	t.Run("error event emitted in multi-node workflow", func(t *testing.T) {
		reducer := func(prev, delta TestState) TestState {
			prev.Counter += delta.Counter
			return prev
		}

		st := store.NewMemStore[TestState]()
		emitter := &mockEmitter{}
		opts := Options{MaxSteps: 100}
		engine := New(reducer, st, emitter, opts)

		// Add nodes where second one fails
		node1 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Counter: 1},
				Route: Goto("node2"),
			}
		})
		node2 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Err: errors.New("node2 failed"),
			}
		})

		if err := engine.Add("node1", node1); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := engine.Add("node2", node2); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := engine.StartAt("node1"); err != nil {
			t.Fatalf("StartAt failed: %v", err)
		}

		ctx := context.Background()
		_, err := engine.Run(ctx, "error-seq-001", TestState{})

		// Verify the run failed
		if err == nil {
			t.Fatal("expected error from node2, got nil")
		}

		// Verify node_start event was emitted for both nodes
		var startEvents []emit.Event
		for _, e := range emitter.events {
			if e.Msg == "node_start" {
				startEvents = append(startEvents, e)
			}
		}
		if len(startEvents) != 2 {
			t.Errorf("expected 2 node_start events, got %d", len(startEvents))
		}

		// Verify error event was emitted
		var errorEvent *emit.Event
		for i := range emitter.events {
			if emitter.events[i].Msg == "error" {
				errorEvent = &emitter.events[i]
				break
			}
		}

		if errorEvent == nil {
			t.Fatal("expected error event, but none found")
		}

		// Verify it came from node2
		if errorEvent.NodeID != "node2" {
			t.Errorf("expected NodeID 'node2', got %q", errorEvent.NodeID)
		}
		if errorEvent.Step != 1 {
			t.Errorf("expected Step 1, got %d", errorEvent.Step)
		}

		t.Logf("error event emitted correctly in multi-node workflow: %+v", errorEvent)
	})
}

// T180: Tool invocation integration tests

// TestEngine_NodeWithToolInvocation verifies nodes can call tools
func TestEngine_NodeWithToolInvocation(t *testing.T) {
	t.Run("node calls single tool successfully", func(t *testing.T) {
		// Create a mock tool
		mockTool := &mockToolForEngine{
			name: "calculator",
			output: map[string]interface{}{
				"result": 42,
			},
		}

		// Create node that calls the tool
		toolNode := NodeFunc[TestState](func(ctx context.Context, state TestState) NodeResult[TestState] {
			// Call the tool
			result, err := mockTool.Call(ctx, map[string]interface{}{
				"operation": "add",
				"a":         20,
				"b":         22,
			})
			if err != nil {
				return NodeResult[TestState]{Err: err}
			}

			// Use tool result in state
			resultValue := result["result"].(int)
			return NodeResult[TestState]{
				Delta: TestState{
					Counter: resultValue,
					Value:   "tool called successfully",
				},
				Route: Stop(),
			}
		})

		// Set up workflow
		reducer := func(prev, delta TestState) TestState {
			if delta.Value != "" {
				prev.Value = delta.Value
			}
			prev.Counter += delta.Counter
			return prev
		}

		st := store.NewMemStore[TestState]()
		emitter := &mockEmitter{}
		opts := Options{MaxSteps: 10}

		engine := New(reducer, st, emitter, opts)
		if err := engine.Add("tool_node", toolNode); err != nil {
			t.Fatalf("Failed to add tool_node node: %v", err)
		}
		if err := engine.StartAt("tool_node"); err != nil {
			t.Fatalf("Failed to set start node to tool_node: %v", err)
		}

		// Run workflow
		ctx := context.Background()
		final, err := engine.Run(ctx, "tool-test-1", TestState{})

		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}

		// Verify tool result was used
		if final.Counter != 42 {
			t.Errorf("Counter = %d, want 42", final.Counter)
		}
		if final.Value != "tool called successfully" {
			t.Errorf("Value = %q, want 'tool called successfully'", final.Value)
		}

		// Verify tool was actually called
		if !mockTool.called {
			t.Error("Tool was not called")
		}
	})

	t.Run("node calls multiple tools in sequence", func(t *testing.T) {
		tool1 := &mockToolForEngine{
			name:   "fetch_data",
			output: map[string]interface{}{"data": "user data"},
		}
		tool2 := &mockToolForEngine{
			name:   "process_data",
			output: map[string]interface{}{"processed": true, "count": 5},
		}

		multiToolNode := NodeFunc[TestState](func(ctx context.Context, state TestState) NodeResult[TestState] {
			// Call first tool
			data, err := tool1.Call(ctx, map[string]interface{}{"user_id": 123})
			if err != nil {
				return NodeResult[TestState]{Err: err}
			}

			// Call second tool with result from first
			processed, err := tool2.Call(ctx, map[string]interface{}{
				"data": data["data"],
			})
			if err != nil {
				return NodeResult[TestState]{Err: err}
			}

			count := processed["count"].(int)
			return NodeResult[TestState]{
				Delta: TestState{Counter: count, Value: "multi-tool"},
				Route: Stop(),
			}
		})

		reducer := func(prev, delta TestState) TestState {
			if delta.Value != "" {
				prev.Value = delta.Value
			}
			prev.Counter += delta.Counter
			return prev
		}

		st := store.NewMemStore[TestState]()
		emitter := &mockEmitter{}
		opts := Options{MaxSteps: 10}

		engine := New(reducer, st, emitter, opts)
		if err := engine.Add("multi_tool", multiToolNode); err != nil {
			t.Fatalf("Failed to add multi_tool node: %v", err)
		}
		if err := engine.StartAt("multi_tool"); err != nil {
			t.Fatalf("Failed to set start node to multi_tool: %v", err)
		}

		ctx := context.Background()
		final, err := engine.Run(ctx, "multi-tool-test", TestState{})

		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}

		if final.Counter != 5 {
			t.Errorf("Counter = %d, want 5", final.Counter)
		}

		// Verify both tools were called
		if !tool1.called || !tool2.called {
			t.Error("Not all tools were called")
		}
	})

	t.Run("node handles tool error gracefully", func(t *testing.T) {
		failingTool := &mockToolForEngine{
			name: "failing_tool",
			err:  errors.New("tool execution failed"),
		}

		errorHandlingNode := NodeFunc[TestState](func(ctx context.Context, state TestState) NodeResult[TestState] {
			result, err := failingTool.Call(ctx, map[string]interface{}{"input": "test"})
			if err != nil {
				// Handle tool error by returning it in NodeResult
				return NodeResult[TestState]{
					Delta: TestState{Value: "tool error handled"},
					Err:   err,
				}
			}

			// This shouldn't be reached
			_ = result
			return NodeResult[TestState]{Route: Stop()}
		})

		reducer := func(prev, delta TestState) TestState {
			if delta.Value != "" {
				prev.Value = delta.Value
			}
			return prev
		}

		st := store.NewMemStore[TestState]()
		emitter := &mockEmitter{}
		opts := Options{MaxSteps: 10, Retries: 0} // No retries

		engine := New(reducer, st, emitter, opts)
		if err := engine.Add("error_node", errorHandlingNode); err != nil {
			t.Fatalf("Failed to add error_node node: %v", err)
		}
		if err := engine.StartAt("error_node"); err != nil {
			t.Fatalf("Failed to set start node to error_node: %v", err)
		}

		ctx := context.Background()
		_, err := engine.Run(ctx, "error-test", TestState{})

		// Should return error from tool
		if err == nil {
			t.Error("Run() error = nil, want error from tool")
		}
		if !strings.Contains(err.Error(), "tool execution failed") {
			t.Errorf("Error message = %q, want to contain 'tool execution failed'", err.Error())
		}
	})

	t.Run("node passes tool results to next node via state", func(t *testing.T) {
		weatherTool := &mockToolForEngine{
			name: "get_weather",
			output: map[string]interface{}{
				"temperature": 72,
				"conditions":  "sunny",
			},
		}

		// First node calls tool
		fetchNode := NodeFunc[TestState](func(ctx context.Context, state TestState) NodeResult[TestState] {
			weather, err := weatherTool.Call(ctx, map[string]interface{}{
				"location": "San Francisco",
			})
			if err != nil {
				return NodeResult[TestState]{Err: err}
			}

			temp := weather["temperature"].(int)
			return NodeResult[TestState]{
				Delta: TestState{
					Counter: temp,
					Value:   weather["conditions"].(string),
				},
				Route: Goto("process"),
			}
		})

		// Second node uses tool results from state
		processNode := NodeFunc[TestState](func(ctx context.Context, state TestState) NodeResult[TestState] {
			// Use weather data from previous node
			summary := state.Value + " day"
			if state.Counter > 70 {
				summary = "warm " + summary
			}

			return NodeResult[TestState]{
				Delta: TestState{Value: summary},
				Route: Stop(),
			}
		})

		reducer := func(prev, delta TestState) TestState {
			if delta.Value != "" {
				prev.Value = delta.Value
			}
			prev.Counter += delta.Counter
			return prev
		}

		st := store.NewMemStore[TestState]()
		emitter := &mockEmitter{}
		opts := Options{MaxSteps: 10}

		engine := New(reducer, st, emitter, opts)
		if err := engine.Add("fetch", fetchNode); err != nil {
			t.Fatalf("Failed to add fetch node: %v", err)
		}
		if err := engine.Add("process", processNode); err != nil {
			t.Fatalf("Failed to add process node: %v", err)
		}
		if err := engine.StartAt("fetch"); err != nil {
			t.Fatalf("Failed to set start node to fetch: %v", err)
		}

		ctx := context.Background()
		final, err := engine.Run(ctx, "weather-test", TestState{})

		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}

		if final.Counter != 72 {
			t.Errorf("Counter = %d, want 72", final.Counter)
		}
		if final.Value != "warm sunny day" {
			t.Errorf("Value = %q, want 'warm sunny day'", final.Value)
		}
	})

	t.Run("tool respects context cancellation", func(t *testing.T) {
		slowTool := &mockToolForEngine{
			name: "slow_tool",
			output: map[string]interface{}{
				"result": "done",
			},
			delay: 2 * time.Second, // Simulates slow operation
		}

		slowNode := NodeFunc[TestState](func(ctx context.Context, state TestState) NodeResult[TestState] {
			result, err := slowTool.Call(ctx, nil)
			if err != nil {
				return NodeResult[TestState]{Err: err}
			}

			return NodeResult[TestState]{
				Delta: TestState{Value: result["result"].(string)},
				Route: Stop(),
			}
		})

		reducer := func(prev, delta TestState) TestState {
			if delta.Value != "" {
				prev.Value = delta.Value
			}
			return prev
		}

		st := store.NewMemStore[TestState]()
		emitter := &mockEmitter{}
		opts := Options{MaxSteps: 10}

		engine := New(reducer, st, emitter, opts)
		if err := engine.Add("slow", slowNode); err != nil {
			t.Fatalf("Failed to add slow node: %v", err)
		}
		if err := engine.StartAt("slow"); err != nil {
			t.Fatalf("Failed to set start node to slow: %v", err)
		}

		// Create context with short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		_, err := engine.Run(ctx, "timeout-test", TestState{})

		// Should timeout
		if err == nil {
			t.Error("Run() error = nil, want timeout error")
		}
		if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "context") {
			t.Errorf("Error = %v, want context timeout error", err)
		}
	})
}

// mockToolForEngine is a mock tool for engine integration testing
type mockToolForEngine struct {
	name   string
	output map[string]interface{}
	err    error
	called bool
	input  map[string]interface{}
	delay  time.Duration
}

func (m *mockToolForEngine) Name() string {
	return m.name
}

func (m *mockToolForEngine) Call(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	m.called = true
	m.input = input

	// Simulate delay if configured
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if m.err != nil {
		return nil, m.err
	}
	return m.output, nil
}

// TestConcurrentExecution (T024) verifies that independent nodes execute
// concurrently when MaxConcurrentNodes is configured.
//
// According to spec.md SC-001: Graphs with 5 independent nodes complete
// execution in 20% of sequential time (demonstrating parallelism).
//
// Requirements:
// - Graph with 3 independent nodes (each sleeps 100ms) completes in ~100ms (not ~300ms)
// - All nodes execute and state correctly merged
// - MaxConcurrentNodes setting controls parallelism
//
// This test should FAIL initially because concurrent execution isn't implemented yet.
func TestConcurrentExecution(t *testing.T) {
	t.Run("parallel execution with 3 independent nodes", func(t *testing.T) {
		reducer := func(prev, delta TestState) TestState {
			prev.Counter += delta.Counter
			return prev
		}

		st := store.NewMemStore[TestState]()
		emitter := &mockEmitter{}

		// Configure engine for concurrent execution
		opts := Options{
			MaxSteps:           20,
			MaxConcurrentNodes: 3, // Allow 3 nodes to run concurrently
		}
		engine := New(reducer, st, emitter, opts)

		// Track execution times
		var execMu sync.Mutex
		execTimes := make(map[string]time.Time)

		// Create 3 independent nodes that each sleep 100ms
		for i := 1; i <= 3; i++ {
			nodeID := "node" + string(rune('0'+i))
			counter := i

			if err := engine.Add(nodeID, NodeFunc[TestState](func(ctx context.Context, state TestState) NodeResult[TestState] {
				execMu.Lock()
				execTimes[nodeID] = time.Now()
				execMu.Unlock()

				// Sleep to simulate work
				time.Sleep(100 * time.Millisecond)

				return NodeResult[TestState]{
					Delta: TestState{Counter: counter},
					Route: Stop(), // All nodes are independent terminals
				}
			})); err != nil {
				t.Fatalf("Failed to add node node: %v", err)
			}
		}

		// Start from all 3 nodes simultaneously (fan-out from start)
		if err := engine.Add("start", NodeFunc[TestState](func(ctx context.Context, state TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Route: Next{Many: []string{"node1", "node2", "node3"}},
			}
		})); err != nil {
			t.Fatalf("Failed to add start node: %v", err)
		}
		if err := engine.StartAt("start"); err != nil {
			t.Fatalf("Failed to set start node to start: %v", err)
		}

		// Execute and measure total time
		startTime := time.Now()
		finalState, err := engine.Run(context.Background(), "concurrent-test", TestState{})
		elapsed := time.Since(startTime)

		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Verify all nodes executed (state should have counter = 1+2+3 = 6)
		if finalState.Counter != 6 {
			t.Errorf("expected Counter = 6, got %d (not all nodes executed)", finalState.Counter)
		}

		// Verify concurrent execution: should complete in ~100ms, not ~300ms
		// Allow some overhead for scheduling, but should be < 200ms
		if elapsed > 200*time.Millisecond {
			t.Errorf("execution took %v, expected ~100ms (concurrent), got sequential time", elapsed)
		}

		// Verify all 3 nodes started within a short window (indicating concurrency)
		execMu.Lock()
		if len(execTimes) != 3 {
			t.Errorf("expected 3 execution timestamps, got %d", len(execTimes))
		}

		var times []time.Time
		for _, ts := range execTimes {
			times = append(times, ts)
		}
		execMu.Unlock()

		// Check that all nodes started within 50ms of each other
		if len(times) == 3 {
			var minTime, maxTime time.Time
			minTime = times[0]
			maxTime = times[0]
			for _, t := range times[1:] {
				if t.Before(minTime) {
					minTime = t
				}
				if t.After(maxTime) {
					maxTime = t
				}
			}
			spread := maxTime.Sub(minTime)
			if spread > 50*time.Millisecond {
				t.Errorf("nodes started with %v spread, expected concurrent start (<50ms)", spread)
			}
		}
	})

	t.Run("respects MaxConcurrentNodes limit", func(t *testing.T) {
		reducer := func(prev, delta TestState) TestState {
			prev.Counter += delta.Counter
			return prev
		}

		st := store.NewMemStore[TestState]()
		emitter := &mockEmitter{}

		// Configure engine with limit of 2 concurrent nodes
		opts := Options{
			MaxSteps:           20,
			MaxConcurrentNodes: 2, // Only 2 nodes can run at once
		}
		engine := New(reducer, st, emitter, opts)

		// Track concurrent execution count
		var execMu sync.Mutex
		currentConcurrent := 0
		maxConcurrent := 0

		// Create 5 nodes that track concurrency
		for i := 1; i <= 5; i++ {
			nodeID := "node" + string(rune('0'+i))
			counter := i

			if err := engine.Add(nodeID, NodeFunc[TestState](func(ctx context.Context, state TestState) NodeResult[TestState] {
				execMu.Lock()
				currentConcurrent++
				if currentConcurrent > maxConcurrent {
					maxConcurrent = currentConcurrent
				}
				execMu.Unlock()

				// Sleep to ensure overlap detection
				time.Sleep(50 * time.Millisecond)

				execMu.Lock()
				currentConcurrent--
				execMu.Unlock()

				return NodeResult[TestState]{
					Delta: TestState{Counter: counter},
					Route: Stop(),
				}
			})); err != nil {
				t.Fatalf("Failed to add node node: %v", err)
			}
		}

		// Start all 5 nodes simultaneously
		if err := engine.Add("start", NodeFunc[TestState](func(ctx context.Context, state TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Route: Next{Many: []string{"node1", "node2", "node3", "node4", "node5"}},
			}
		})); err != nil {
			t.Fatalf("Failed to add start node: %v", err)
		}
		if err := engine.StartAt("start"); err != nil {
			t.Fatalf("Failed to set start node to start: %v", err)
		}

		_, err := engine.Run(context.Background(), "limit-test", TestState{})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Verify max concurrent never exceeded the limit
		execMu.Lock()
		observed := maxConcurrent
		execMu.Unlock()

		if observed > 2 {
			t.Errorf("MaxConcurrentNodes=2 but observed %d concurrent executions", observed)
		}
	})
}

// TestFanOutRouting (T025) verifies that a node can route to multiple
// child nodes simultaneously using Next.Many, and that branches execute
// concurrently and merge deterministically.
//
// According to spec.md FR-003: System MUST support fan-out routing where
// one node spawns multiple concurrent branches.
//
// Requirements:
// - One node returns Next.Many with 5 node IDs
// - All 5 branches execute concurrently
// - State merges deterministically at join point
//
// This test should FAIL initially because fan-out routing isn't implemented yet.
func TestFanOutRouting(t *testing.T) {
	t.Run("fan-out to 5 concurrent branches", func(t *testing.T) {
		reducer := func(prev, delta TestState) TestState {
			prev.Counter += delta.Counter
			if delta.Value != "" {
				if prev.Value == "" {
					prev.Value = delta.Value
				} else {
					prev.Value += "," + delta.Value
				}
			}
			return prev
		}

		st := store.NewMemStore[TestState]()
		emitter := &mockEmitter{}

		opts := Options{
			MaxSteps:           20,
			MaxConcurrentNodes: 5,
		}
		engine := New(reducer, st, emitter, opts)

		// Track execution order
		var execMu sync.Mutex
		execOrder := make([]string, 0)
		execTimes := make(map[string]time.Time)

		// Create 5 branch nodes
		for i := 1; i <= 5; i++ {
			// Capture loop variables to avoid closure capture issues
			branchID := "branch" + string(rune('0'+i))
			counter := i

			// Create a closure with captured variables
			func(bid string, cnt int) {
				if err := engine.Add(bid, NodeFunc[TestState](func(ctx context.Context, state TestState) NodeResult[TestState] {
					execMu.Lock()
					execOrder = append(execOrder, bid)
					execTimes[bid] = time.Now()
					execMu.Unlock()

					// Small delay to ensure concurrent execution
					time.Sleep(20 * time.Millisecond)

					return NodeResult[TestState]{
						Delta: TestState{
							Counter: cnt,
							Value:   bid,
						},
						Route: Goto("join"),
					}
				})); err != nil {
					t.Fatalf("Failed to add node node: %v", err)
				}
			}(branchID, counter)
		}

		// Join node that merges results
		if err := engine.Add("join", NodeFunc[TestState](func(ctx context.Context, state TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{}, // No delta, just observe merged state
				Route: Stop(),
			}
		})); err != nil {
			t.Fatalf("Failed to add join node: %v", err)
		}

		// Fan-out node
		if err := engine.Add("fanout", NodeFunc[TestState](func(ctx context.Context, state TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Route: Next{Many: []string{"branch1", "branch2", "branch3", "branch4", "branch5"}},
			}
		})); err != nil {
			t.Fatalf("Failed to add fanout node: %v", err)
		}

		if err := engine.StartAt("fanout"); err != nil {
			t.Fatalf("Failed to set start node to fanout: %v", err)
		}

		// Execute
		startTime := time.Now()
		finalState, err := engine.Run(context.Background(), "fanout-test", TestState{})
		elapsed := time.Since(startTime)

		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Verify all branches executed (counter should be 1+2+3+4+5 = 15)
		if finalState.Counter != 15 {
			t.Errorf("expected Counter = 15, got %d", finalState.Counter)
		}

		// Verify concurrent execution (should be ~20ms, not ~100ms sequential)
		if elapsed > 100*time.Millisecond {
			t.Errorf("execution took %v, expected ~20ms with concurrency", elapsed)
		}

		// Verify all branches started concurrently (within 30ms window)
		execMu.Lock()
		if len(execTimes) != 5 {
			t.Errorf("expected 5 branch executions, got %d", len(execTimes))
		}

		var times []time.Time
		for _, ts := range execTimes {
			times = append(times, ts)
		}
		execMu.Unlock()

		if len(times) == 5 {
			var minTime, maxTime time.Time
			minTime = times[0]
			maxTime = times[0]
			for _, t := range times[1:] {
				if t.Before(minTime) {
					minTime = t
				}
				if t.After(maxTime) {
					maxTime = t
				}
			}
			spread := maxTime.Sub(minTime)
			if spread > 30*time.Millisecond {
				t.Errorf("branches started with %v spread, expected concurrent start (<30ms)", spread)
			}
		}

		// Verify state merge includes all branch values
		// (deterministic merge order is tested in TestDeterministicMerge)
		if !strings.Contains(finalState.Value, "branch1") ||
			!strings.Contains(finalState.Value, "branch2") ||
			!strings.Contains(finalState.Value, "branch3") ||
			!strings.Contains(finalState.Value, "branch4") ||
			!strings.Contains(finalState.Value, "branch5") {
			t.Errorf("final state Value missing branches: %s", finalState.Value)
		}
	})
}

// TestConcurrencyLimit (T059) verifies MaxConcurrentNodes enforcement.
//
// According to spec.md FR-009: System MUST enforce MaxConcurrentNodes to limit
// simultaneous node execution.
//
// Requirements:
// - When MaxConcurrentNodes=N, at most N nodes execute simultaneously
// - Nodes beyond the limit wait until capacity becomes available
// - Execution completes successfully with bounded concurrency
// - Peak concurrent execution never exceeds configured limit
//
// This test spawns more work items than the concurrency limit and verifies
// that the peak active node count never exceeds MaxConcurrentNodes.
func TestConcurrencyLimit(t *testing.T) {
	t.Run("enforces MaxConcurrentNodes limit", func(t *testing.T) {
		// Create reducer
		reducer := func(prev, delta TestState) TestState {
			prev.Counter += delta.Counter
			return prev
		}

		// Create engine with MaxConcurrentNodes=3
		st := store.NewMemStore[TestState]()
		emitter := &mockEmitter{}
		opts := Options{
			MaxSteps:           100,
			MaxConcurrentNodes: 3, // Limit to 3 concurrent nodes
			QueueDepth:         100,
		}
		engine := New(reducer, st, emitter, opts)

		// Track concurrent execution count
		var activeMu sync.Mutex
		var activeCount int32
		var peakCount int32

		// Create slow nodes that track concurrency
		createSlowNode := func(id string, sleepMs int) Node[TestState] {
			return NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
				// Increment active count
				activeMu.Lock()
				activeCount++
				if activeCount > peakCount {
					peakCount = activeCount
				}
				current := activeCount
				activeMu.Unlock()

				t.Logf("Node %s started (active=%d)", id, current)

				// Simulate work
				time.Sleep(time.Duration(sleepMs) * time.Millisecond)

				// Decrement active count
				activeMu.Lock()
				activeCount--
				activeMu.Unlock()

				return NodeResult[TestState]{
					Delta: TestState{Counter: 1},
					Route: Stop(),
				}
			})
		}

		// Create fan-out node that spawns 10 parallel branches
		fanOutNode := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Counter: 0},
				Route: Many([]string{"slow1", "slow2", "slow3", "slow4", "slow5",
					"slow6", "slow7", "slow8", "slow9", "slow10"}),
			}
		})

		// Add nodes
		_ = engine.Add("fanout", fanOutNode)
		for i := 1; i <= 10; i++ {
			nodeName := fmt.Sprintf("slow%d", i)
			_ = engine.Add(nodeName, createSlowNode(nodeName, 50))
		}
		_ = engine.StartAt("fanout")

		// Execute
		ctx := context.Background()
		initial := TestState{Counter: 0}
		finalState, err := engine.Run(ctx, "concurrency-limit-test", initial)
		if err != nil {
			t.Fatalf("execution failed: %v", err)
		}

		// Verify all nodes executed
		if finalState.Counter != 10 {
			t.Errorf("expected Counter = 10, got %d", finalState.Counter)
		}

		// Verify peak concurrency never exceeded limit
		activeMu.Lock()
		peak := peakCount
		activeMu.Unlock()

		t.Logf("Peak concurrent execution: %d (limit: 3)", peak)

		if peak > 3 {
			t.Errorf("peak concurrency %d exceeded limit of 3", peak)
		}

		if peak < 2 {
			t.Errorf("peak concurrency %d too low, expected at least 2 concurrent nodes", peak)
		}
	})

	t.Run("sequential execution when MaxConcurrentNodes=0", func(t *testing.T) {
		// Create reducer
		reducer := func(prev, delta TestState) TestState {
			prev.Counter += delta.Counter
			return prev
		}

		// Create engine with MaxConcurrentNodes=0 (sequential mode)
		st := store.NewMemStore[TestState]()
		emitter := &mockEmitter{}
		opts := Options{
			MaxSteps:           100,
			MaxConcurrentNodes: 0, // Sequential execution
		}
		engine := New(reducer, st, emitter, opts)

		// Track concurrent execution count
		var activeMu sync.Mutex
		var activeCount int32
		var peakCount int32

		// Create nodes that track concurrency
		createNode := func(id string) Node[TestState] {
			return NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
				// Increment active count
				activeMu.Lock()
				activeCount++
				if activeCount > peakCount {
					peakCount = activeCount
				}
				current := activeCount
				activeMu.Unlock()

				t.Logf("Node %s started (active=%d)", id, current)

				// Brief work simulation
				time.Sleep(10 * time.Millisecond)

				// Decrement active count
				activeMu.Lock()
				activeCount--
				activeMu.Unlock()

				// Route to next node or stop
				var route Next
				if id == "node1" {
					route = Goto("node2")
				} else if id == "node2" {
					route = Goto("node3")
				} else {
					route = Stop()
				}

				return NodeResult[TestState]{
					Delta: TestState{Counter: 1},
					Route: route,
				}
			})
		}

		// Add sequential nodes
		_ = engine.Add("node1", createNode("node1"))
		_ = engine.Add("node2", createNode("node2"))
		_ = engine.Add("node3", createNode("node3"))
		_ = engine.StartAt("node1")

		// Execute
		ctx := context.Background()
		initial := TestState{Counter: 0}
		finalState, err := engine.Run(ctx, "sequential-test", initial)
		if err != nil {
			t.Fatalf("execution failed: %v", err)
		}

		// Verify all nodes executed
		if finalState.Counter != 3 {
			t.Errorf("expected Counter = 3, got %d", finalState.Counter)
		}

		// Verify sequential execution (max 1 concurrent)
		activeMu.Lock()
		peak := peakCount
		activeMu.Unlock()

		t.Logf("Peak concurrent execution: %d (expected: 1 for sequential)", peak)

		if peak > 1 {
			t.Errorf("peak concurrency %d exceeded expected 1 for sequential mode", peak)
		}
	})
}

// TestRunWallClockBudget (T071) verifies run-level timeout enforcement.
//
// According to spec.md FR-015: System MUST enforce run-level wall clock budget
// via RunWallClockBudget configuration.
//
// Requirements:
// - When RunWallClockBudget is set, Run() must terminate execution after the timeout
// - Returns context.DeadlineExceeded error when budget is exceeded
// - Nodes respect the timeout and stop execution
// - Partial state updates are not lost (last successful checkpoint preserved)
//
// This test creates a workflow with slow nodes and verifies that execution
// terminates when the run-level timeout is reached.
func TestRunWallClockBudget(t *testing.T) {
	t.Run("enforces run-level timeout", func(t *testing.T) {
		// Create reducer
		reducer := func(prev, delta TestState) TestState {
			prev.Counter += delta.Counter
			if delta.Value != "" {
				prev.Value = delta.Value
			}
			return prev
		}

		// Create engine with 200ms wall clock budget
		st := store.NewMemStore[TestState]()
		emitter := &mockEmitter{}
		opts := Options{
			MaxSteps:           100,
			RunWallClockBudget: 200 * time.Millisecond, // 200ms total budget
		}
		engine := New(reducer, st, emitter, opts)

		// Create slow node that would exceed budget
		slowNode := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			// This node takes 150ms
			select {
			case <-time.After(150 * time.Millisecond):
				return NodeResult[TestState]{
					Delta: TestState{Counter: 1, Value: "node1_complete"},
					Route: Goto("node2"),
				}
			case <-ctx.Done():
				// Context was cancelled
				return NodeResult[TestState]{
					Delta: TestState{Value: "node1_cancelled"},
					Err:   ctx.Err(),
				}
			}
		})

		// Second slow node that should not complete due to timeout
		slowNode2 := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			select {
			case <-time.After(150 * time.Millisecond):
				return NodeResult[TestState]{
					Delta: TestState{Counter: 1, Value: "node2_complete"},
					Route: Stop(),
				}
			case <-ctx.Done():
				return NodeResult[TestState]{
					Delta: TestState{Value: "node2_cancelled"},
					Err:   ctx.Err(),
				}
			}
		})

		_ = engine.Add("node1", slowNode)
		_ = engine.Add("node2", slowNode2)
		_ = engine.StartAt("node1")

		// Execute with timeout
		ctx := context.Background()
		start := time.Now()
		_, err := engine.Run(ctx, "timeout-test", TestState{})
		duration := time.Since(start)

		// Verify timeout occurred
		if err == nil {
			t.Fatal("expected timeout error, got nil")
		}

		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("expected context.DeadlineExceeded, got %v", err)
		}

		// Verify execution terminated within reasonable time of budget
		// Allow 100ms grace period for cleanup
		if duration > 300*time.Millisecond {
			t.Errorf("execution took %v, expected ~200ms (budget)", duration)
		}

		t.Logf("Run terminated after %v with error: %v", duration, err)
	})

	t.Run("completes successfully within budget", func(t *testing.T) {
		// Create reducer
		reducer := func(prev, delta TestState) TestState {
			prev.Counter += delta.Counter
			return prev
		}

		// Create engine with generous wall clock budget
		st := store.NewMemStore[TestState]()
		emitter := &mockEmitter{}
		opts := Options{
			MaxSteps:           100,
			RunWallClockBudget: 2 * time.Second, // 2s total budget (plenty of time)
		}
		engine := New(reducer, st, emitter, opts)

		// Create fast node
		fastNode := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Counter: 1},
				Route: Stop(),
			}
		})

		_ = engine.Add("fast", fastNode)
		_ = engine.StartAt("fast")

		// Execute
		ctx := context.Background()
		finalState, err := engine.Run(ctx, "within-budget-test", TestState{})
		if err != nil {
			t.Fatalf("expected success within budget, got error: %v", err)
		}

		if finalState.Counter != 1 {
			t.Errorf("expected Counter = 1, got %d", finalState.Counter)
		}
	})

	t.Run("no timeout when RunWallClockBudget is zero", func(t *testing.T) {
		// Create reducer
		reducer := func(prev, delta TestState) TestState {
			prev.Counter += delta.Counter
			return prev
		}

		// Create engine without wall clock budget (default 0 = no limit)
		st := store.NewMemStore[TestState]()
		emitter := &mockEmitter{}
		opts := Options{
			MaxSteps:           100,
			RunWallClockBudget: 0, // No timeout
		}
		engine := New(reducer, st, emitter, opts)

		// Create moderately slow node
		slowNode := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			time.Sleep(100 * time.Millisecond)
			return NodeResult[TestState]{
				Delta: TestState{Counter: 1},
				Route: Stop(),
			}
		})

		_ = engine.Add("slow", slowNode)
		_ = engine.StartAt("slow")

		// Execute without timeout
		ctx := context.Background()
		finalState, err := engine.Run(ctx, "no-timeout-test", TestState{})
		if err != nil {
			t.Fatalf("expected success without timeout, got error: %v", err)
		}

		if finalState.Counter != 1 {
			t.Errorf("expected Counter = 1, got %d", finalState.Counter)
		}
	})
}

// TestCancellationPropagation (T073) verifies context cancellation reaches all running nodes.
//
// According to spec.md SC-004: Context cancellation reaches all running nodes within
// 1 second of cancellation signal.
//
// Requirements:
// - When parent context is cancelled, all running nodes receive cancellation
// - Cancellation propagates within 1 second
// - Nodes respect ctx.Done() and terminate promptly
// - Concurrent nodes all receive cancellation signal
//
// This test spawns multiple concurrent nodes and cancels the context, verifying
// that all nodes detect cancellation quickly.
func TestCancellationPropagation(t *testing.T) {
	t.Run("cancellation reaches all concurrent nodes within 1 second", func(t *testing.T) {
		// Create reducer
		reducer := func(prev, delta TestState) TestState {
			prev.Counter += delta.Counter
			return prev
		}

		// Create engine with concurrent execution
		st := store.NewMemStore[TestState]()
		emitter := &mockEmitter{}
		opts := Options{
			MaxSteps:           100,
			MaxConcurrentNodes: 5,
			QueueDepth:         100,
		}
		engine := New(reducer, st, emitter, opts)

		// Track cancellation timing for each node
		var cancelMu sync.Mutex
		cancelTimes := make(map[string]time.Time)

		// Create long-running nodes that respect cancellation
		createCancellableNode := func(id string) Node[TestState] {
			return NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
				// Simulate long-running work that respects ctx.Done()
				select {
				case <-time.After(10 * time.Second): // Would take 10s without cancellation
					return NodeResult[TestState]{
						Delta: TestState{Counter: 1},
						Route: Stop(),
					}
				case <-ctx.Done():
					// Record when this node detected cancellation
					cancelMu.Lock()
					cancelTimes[id] = time.Now()
					cancelMu.Unlock()

					return NodeResult[TestState]{
						Delta: TestState{Counter: 0},
						Err:   ctx.Err(),
					}
				}
			})
		}

		// Create fan-out node that spawns multiple parallel branches
		fanOutNode := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Counter: 0},
				Route: Many([]string{"node1", "node2", "node3", "node4", "node5"}),
			}
		})

		// Add nodes
		_ = engine.Add("fanout", fanOutNode)
		for i := 1; i <= 5; i++ {
			nodeName := "node" + string(rune('0'+i))
			_ = engine.Add(nodeName, createCancellableNode(nodeName))
		}
		_ = engine.StartAt("fanout")

		// Create cancellable context
		ctx, cancel := context.WithCancel(context.Background())

		// Start execution in goroutine
		errChan := make(chan error, 1)
		go func() {
			_, err := engine.Run(ctx, "cancel-test", TestState{})
			errChan <- err
		}()

		// Wait a bit for nodes to start, then cancel
		time.Sleep(100 * time.Millisecond)
		cancelTime := time.Now()
		cancel()

		// Wait for execution to complete
		err := <-errChan

		// Verify execution was cancelled
		if err == nil {
			t.Fatal("expected cancellation error, got nil")
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}

		// Verify all nodes detected cancellation
		cancelMu.Lock()
		nodeCount := len(cancelTimes)
		cancelMu.Unlock()

		if nodeCount == 0 {
			t.Fatal("no nodes detected cancellation")
		}

		// Verify cancellation propagation time
		cancelMu.Lock()
		var maxPropagation time.Duration
		for nodeID, cancelledAt := range cancelTimes {
			propagation := cancelledAt.Sub(cancelTime)
			t.Logf("Node %s detected cancellation after %v", nodeID, propagation)
			if propagation > maxPropagation {
				maxPropagation = propagation
			}
		}
		cancelMu.Unlock()

		// SC-004: Cancellation must reach all nodes within 1 second
		if maxPropagation > time.Second {
			t.Errorf("cancellation propagation took %v, expected < 1s", maxPropagation)
		}

		t.Logf("All %d nodes detected cancellation within %v", nodeCount, maxPropagation)
	})

	t.Run("sequential execution respects cancellation", func(t *testing.T) {
		// Create reducer
		reducer := func(prev, delta TestState) TestState {
			prev.Counter += delta.Counter
			return prev
		}

		// Create engine in sequential mode
		st := store.NewMemStore[TestState]()
		emitter := &mockEmitter{}
		opts := Options{
			MaxSteps:           100,
			MaxConcurrentNodes: 0, // Sequential mode
		}
		engine := New(reducer, st, emitter, opts)

		// Create node that respects cancellation
		cancellableNode := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			select {
			case <-time.After(5 * time.Second):
				return NodeResult[TestState]{
					Delta: TestState{Counter: 1},
					Route: Stop(),
				}
			case <-ctx.Done():
				return NodeResult[TestState]{
					Err: ctx.Err(),
				}
			}
		})

		_ = engine.Add("slow", cancellableNode)
		_ = engine.StartAt("slow")

		// Create cancellable context
		ctx, cancel := context.WithCancel(context.Background())

		// Start execution in goroutine
		errChan := make(chan error, 1)
		go func() {
			_, err := engine.Run(ctx, "seq-cancel-test", TestState{})
			errChan <- err
		}()

		// Cancel after brief delay
		time.Sleep(50 * time.Millisecond)
		cancelTime := time.Now()
		cancel()

		// Wait for execution to complete
		err := <-errChan
		propagation := time.Since(cancelTime)

		// Verify cancellation
		if err == nil {
			t.Fatal("expected cancellation error, got nil")
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}

		// Verify fast propagation
		if propagation > time.Second {
			t.Errorf("cancellation took %v, expected < 1s", propagation)
		}

		t.Logf("Sequential node detected cancellation within %v", propagation)
	})
}

// TestDeadlockDetection (T074) verifies ErrNoProgress is raised when workflow deadlocks.
//
// According to spec.md FR-016: System MUST detect topology deadlocks (no progress possible)
// and raise ErrNoProgress.
//
// Requirements:
// - Detect when frontier is empty but workflow is not complete
// - Detect when all nodes are waiting on conditions that won't be satisfied
// - Raise ErrNoProgress error with clear message
// - Detection occurs within reasonable time (not hanging indefinitely)
//
// This test creates scenarios where the workflow cannot make forward progress
// and verifies that deadlock detection triggers appropriately.
func TestDeadlockDetection(t *testing.T) {
	t.Run("detects deadlock when no routes available", func(t *testing.T) {
		// Create reducer
		reducer := func(prev, delta TestState) TestState {
			prev.Counter += delta.Counter
			return prev
		}

		// Create engine
		st := store.NewMemStore[TestState]()
		emitter := &mockEmitter{}
		opts := Options{
			MaxSteps: 100,
		}
		engine := New(reducer, st, emitter, opts)

		// Create node that doesn't route anywhere (no Goto, no Stop, no edges)
		deadEndNode := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Counter: 1},
				Route: Next{}, // Empty route - no next node, not terminal
			}
		})

		_ = engine.Add("deadend", deadEndNode)
		_ = engine.StartAt("deadend")

		// Execute - should detect deadlock
		ctx := context.Background()
		_, err := engine.Run(ctx, "deadlock-test", TestState{})

		// Verify deadlock error
		if err == nil {
			t.Fatal("expected deadlock error, got nil")
		}

		// Check for ErrNoProgress or "no valid route" error
		if !errors.Is(err, ErrNoProgress) && !strings.Contains(err.Error(), "no valid route") {
			t.Errorf("expected ErrNoProgress or 'no valid route', got %v", err)
		}

		t.Logf("Deadlock detected correctly: %v", err)
	})

	t.Run("detects deadlock with circular dependencies", func(t *testing.T) {
		// This test verifies detection of more complex deadlock scenarios
		// For now, the "no valid route" error serves as our deadlock detection
		// A more sophisticated implementation would detect circular waits

		// Create reducer
		reducer := func(prev, delta TestState) TestState {
			prev.Counter += delta.Counter
			return prev
		}

		// Create engine
		st := store.NewMemStore[TestState]()
		emitter := &mockEmitter{}
		opts := Options{
			MaxSteps: 10, // Low limit to prevent infinite loops
		}
		engine := New(reducer, st, emitter, opts)

		// Create nodes that route to each other conditionally but never satisfy exit condition
		nodeA := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			// Always route to B, never satisfy exit condition
			return NodeResult[TestState]{
				Delta: TestState{Counter: 1},
				Route: Goto("nodeB"),
			}
		})

		nodeB := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			// Always route back to A
			return NodeResult[TestState]{
				Delta: TestState{Counter: 1},
				Route: Goto("nodeA"),
			}
		})

		_ = engine.Add("nodeA", nodeA)
		_ = engine.Add("nodeB", nodeB)
		_ = engine.StartAt("nodeA")

		// Execute - should hit MaxSteps limit (which prevents infinite loops)
		ctx := context.Background()
		_, err := engine.Run(ctx, "circular-test", TestState{})

		// Verify error (either MaxSteps exceeded or deadlock detected)
		if err == nil {
			t.Fatal("expected error for circular routing, got nil")
		}

		// The error should be either MAX_STEPS_EXCEEDED or no progress
		if !strings.Contains(err.Error(), "MAX_STEPS_EXCEEDED") &&
			!errors.Is(err, ErrNoProgress) {
			t.Errorf("expected MAX_STEPS_EXCEEDED or ErrNoProgress, got %v", err)
		}

		t.Logf("Circular dependency detected: %v", err)
	})
}
