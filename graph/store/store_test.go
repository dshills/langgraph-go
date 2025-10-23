package store

import (
	"context"
	"errors"
	"testing"
)

// TestState is a test state type for store tests.
type TestState struct {
	Value   string
	Counter int
}

// TestStore_InterfaceContract verifies Store[S] interface can be implemented (T027).
func TestStore_InterfaceContract(t *testing.T) {
	// Verify interface can be declared
	var _ Store[TestState] = (*mockStore)(nil)
}

// mockStore is a minimal Store implementation for testing the interface contract.
type mockStore struct {
	steps       map[string][]StepRecord[TestState]
	checkpoints map[string]Checkpoint[TestState]
}

func (m *mockStore) SaveStep(ctx context.Context, runID string, step int, nodeID string, state TestState) error {
	if m.steps == nil {
		m.steps = make(map[string][]StepRecord[TestState])
	}
	m.steps[runID] = append(m.steps[runID], StepRecord[TestState]{
		Step:   step,
		NodeID: nodeID,
		State:  state,
	})
	return nil
}

func (m *mockStore) LoadLatest(ctx context.Context, runID string) (TestState, int, error) {
	steps, exists := m.steps[runID]
	if !exists || len(steps) == 0 {
		return TestState{}, 0, ErrNotFound
	}
	latest := steps[len(steps)-1]
	return latest.State, latest.Step, nil
}

func (m *mockStore) SaveCheckpoint(ctx context.Context, cpID string, state TestState, step int) error {
	if m.checkpoints == nil {
		m.checkpoints = make(map[string]Checkpoint[TestState])
	}
	m.checkpoints[cpID] = Checkpoint[TestState]{
		ID:    cpID,
		State: state,
		Step:  step,
	}
	return nil
}

func (m *mockStore) LoadCheckpoint(ctx context.Context, cpID string) (TestState, int, error) {
	cp, exists := m.checkpoints[cpID]
	if !exists {
		return TestState{}, 0, ErrNotFound
	}
	return cp.State, cp.Step, nil
}

// TestStore_SaveStep verifies SaveStep method behavior.
func TestStore_SaveStep(t *testing.T) {
	ctx := context.Background()
	store := &mockStore{}

	err := store.SaveStep(ctx, "run-001", 1, "node1", TestState{Value: "step1"})
	if err != nil {
		t.Fatalf("SaveStep failed: %v", err)
	}

	// Verify step was saved
	steps, exists := store.steps["run-001"]
	if !exists {
		t.Fatal("expected steps to be saved for run-001")
	}
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}
	if steps[0].NodeID != "node1" {
		t.Errorf("expected NodeID = 'node1', got %q", steps[0].NodeID)
	}
	if steps[0].State.Value != "step1" {
		t.Errorf("expected State.Value = 'step1', got %q", steps[0].State.Value)
	}
}

// TestStore_LoadLatest verifies LoadLatest method behavior.
func TestStore_LoadLatest(t *testing.T) {
	ctx := context.Background()
	store := &mockStore{}

	// Save multiple steps
	_ = store.SaveStep(ctx, "run-001", 1, "node1", TestState{Value: "step1"})
	_ = store.SaveStep(ctx, "run-001", 2, "node2", TestState{Value: "step2"})
	_ = store.SaveStep(ctx, "run-001", 3, "node3", TestState{Value: "step3"})

	// Load latest
	state, step, err := store.LoadLatest(ctx, "run-001")
	if err != nil {
		t.Fatalf("LoadLatest failed: %v", err)
	}

	if step != 3 {
		t.Errorf("expected step = 3, got %d", step)
	}
	if state.Value != "step3" {
		t.Errorf("expected State.Value = 'step3', got %q", state.Value)
	}
}

// TestStore_LoadLatest_NotFound verifies error handling for missing runID.
func TestStore_LoadLatest_NotFound(t *testing.T) {
	ctx := context.Background()
	store := &mockStore{}

	_, _, err := store.LoadLatest(ctx, "nonexistent-run")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// TestStore_SaveCheckpoint verifies SaveCheckpoint method behavior.
func TestStore_SaveCheckpoint(t *testing.T) {
	ctx := context.Background()
	store := &mockStore{}

	err := store.SaveCheckpoint(ctx, "cp-001", TestState{Value: "checkpoint"}, 5)
	if err != nil {
		t.Fatalf("SaveCheckpoint failed: %v", err)
	}

	// Verify checkpoint was saved
	cp, exists := store.checkpoints["cp-001"]
	if !exists {
		t.Fatal("expected checkpoint cp-001 to exist")
	}
	if cp.State.Value != "checkpoint" {
		t.Errorf("expected State.Value = 'checkpoint', got %q", cp.State.Value)
	}
	if cp.Step != 5 {
		t.Errorf("expected Step = 5, got %d", cp.Step)
	}
}

// TestStore_LoadCheckpoint verifies LoadCheckpoint method behavior.
func TestStore_LoadCheckpoint(t *testing.T) {
	ctx := context.Background()
	store := &mockStore{}

	// Save checkpoint
	_ = store.SaveCheckpoint(ctx, "cp-001", TestState{Value: "restored"}, 10)

	// Load checkpoint
	state, step, err := store.LoadCheckpoint(ctx, "cp-001")
	if err != nil {
		t.Fatalf("LoadCheckpoint failed: %v", err)
	}

	if step != 10 {
		t.Errorf("expected step = 10, got %d", step)
	}
	if state.Value != "restored" {
		t.Errorf("expected State.Value = 'restored', got %q", state.Value)
	}
}

// TestStore_LoadCheckpoint_NotFound verifies error handling for missing checkpoint.
func TestStore_LoadCheckpoint_NotFound(t *testing.T) {
	ctx := context.Background()
	store := &mockStore{}

	_, _, err := store.LoadCheckpoint(ctx, "nonexistent-cp")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
