package graph

import (
	"context"
	"errors"
	"testing"

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
	events []emit.Event
}

func (m *mockEmitter) Emit(event emit.Event) {
	if m.events == nil {
		m.events = make([]emit.Event, 0)
	}
	m.events = append(m.events, event)
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

// Helper functions for MaxSteps tests
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

// failingStore is a test store that always fails SaveStep
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
