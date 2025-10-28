package store

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestSQLiteStore_SaveLoadStep verifies SaveStep and LoadLatest work correctly (T057, T072).
func TestSQLiteStore_SaveLoadStep(t *testing.T) {
	ctx := context.Background()
	store := newTestSQLiteStore(t)
	defer store.Close()

	// Test 1: Save a single step
	state1 := TestState{Value: "first", Counter: 1}
	err := store.SaveStep(ctx, "run-001", 1, "node-a", state1)
	if err != nil {
		t.Fatalf("SaveStep failed: %v", err)
	}

	// Test 2: Load the step back
	loadedState, step, err := store.LoadLatest(ctx, "run-001")
	if err != nil {
		t.Fatalf("LoadLatest failed: %v", err)
	}
	if step != 1 {
		t.Errorf("expected step = 1, got %d", step)
	}
	if loadedState.Value != "first" {
		t.Errorf("expected Value = 'first', got %q", loadedState.Value)
	}
	if loadedState.Counter != 1 {
		t.Errorf("expected Counter = 1, got %d", loadedState.Counter)
	}

	// Test 3: Save multiple steps
	state2 := TestState{Value: "second", Counter: 2}
	state3 := TestState{Value: "third", Counter: 3}
	_ = store.SaveStep(ctx, "run-001", 2, "node-b", state2)
	_ = store.SaveStep(ctx, "run-001", 3, "node-c", state3)

	// Test 4: LoadLatest returns highest step number
	loadedState, step, err = store.LoadLatest(ctx, "run-001")
	if err != nil {
		t.Fatalf("LoadLatest failed: %v", err)
	}
	if step != 3 {
		t.Errorf("expected step = 3, got %d", step)
	}
	if loadedState.Value != "third" {
		t.Errorf("expected Value = 'third', got %q", loadedState.Value)
	}

	// Test 5: Out-of-order saves (save step 5, then step 4)
	state4 := TestState{Value: "fourth", Counter: 4}
	state5 := TestState{Value: "fifth", Counter: 5}
	_ = store.SaveStep(ctx, "run-001", 5, "node-e", state5)
	_ = store.SaveStep(ctx, "run-001", 4, "node-d", state4)

	// LoadLatest should still return step 5
	loadedState, step, err = store.LoadLatest(ctx, "run-001")
	if err != nil {
		t.Fatalf("LoadLatest failed: %v", err)
	}
	if step != 5 {
		t.Errorf("expected step = 5 (highest), got %d", step)
	}
	if loadedState.Value != "fifth" {
		t.Errorf("expected Value = 'fifth', got %q", loadedState.Value)
	}

	// Test 6: LoadLatest on nonexistent run returns ErrNotFound
	_, _, err = store.LoadLatest(ctx, "nonexistent-run")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound for nonexistent run, got: %v", err)
	}

	// Test 7: Multiple separate runs don't interfere
	stateRun2 := TestState{Value: "run2", Counter: 100}
	_ = store.SaveStep(ctx, "run-002", 1, "node-x", stateRun2)

	loadedRun2, stepRun2, err := store.LoadLatest(ctx, "run-002")
	if err != nil {
		t.Fatalf("LoadLatest for run-002 failed: %v", err)
	}
	if stepRun2 != 1 {
		t.Errorf("expected step = 1 for run-002, got %d", stepRun2)
	}
	if loadedRun2.Value != "run2" {
		t.Errorf("expected Value = 'run2', got %q", loadedRun2.Value)
	}

	// Verify run-001 is still correct
	loadedRun1, stepRun1, _ := store.LoadLatest(ctx, "run-001")
	if stepRun1 != 5 {
		t.Errorf("run-001 step changed unexpectedly: got %d", stepRun1)
	}
	if loadedRun1.Value != "fifth" {
		t.Errorf("run-001 state changed unexpectedly: got %q", loadedRun1.Value)
	}
}

// TestSQLiteStore_CheckpointV2 verifies SaveCheckpointV2 and LoadCheckpointV2 (T058, T073).
func TestSQLiteStore_CheckpointV2(t *testing.T) {
	ctx := context.Background()
	store := newTestSQLiteStore(t)
	defer store.Close()

	// Test 1: Save a checkpoint with full context
	checkpoint1 := CheckpointV2[TestState]{
		RunID:          "run-001",
		StepID:         1,
		State:          TestState{Value: "checkpoint1", Counter: 10},
		Frontier:       []string{"node-a", "node-b"},
		RNGSeed:        12345,
		RecordedIOs:    []string{"io1", "io2"},
		IdempotencyKey: "idem-key-001",
		Timestamp:      time.Now(),
		Label:          "after-validation",
	}

	err := store.SaveCheckpointV2(ctx, checkpoint1)
	if err != nil {
		t.Fatalf("SaveCheckpointV2 failed: %v", err)
	}

	// Test 2: Load the checkpoint back
	loaded, err := store.LoadCheckpointV2(ctx, "run-001", 1)
	if err != nil {
		t.Fatalf("LoadCheckpointV2 failed: %v", err)
	}

	if loaded.RunID != "run-001" {
		t.Errorf("expected RunID = 'run-001', got %q", loaded.RunID)
	}
	if loaded.StepID != 1 {
		t.Errorf("expected StepID = 1, got %d", loaded.StepID)
	}
	if loaded.State.Value != "checkpoint1" {
		t.Errorf("expected State.Value = 'checkpoint1', got %q", loaded.State.Value)
	}
	if loaded.RNGSeed != 12345 {
		t.Errorf("expected RNGSeed = 12345, got %d", loaded.RNGSeed)
	}
	if loaded.Label != "after-validation" {
		t.Errorf("expected Label = 'after-validation', got %q", loaded.Label)
	}

	// Verify frontier
	frontierSlice, ok := loaded.Frontier.([]interface{})
	if !ok {
		t.Fatalf("expected Frontier to be []interface{}, got %T", loaded.Frontier)
	}
	if len(frontierSlice) != 2 {
		t.Errorf("expected Frontier length = 2, got %d", len(frontierSlice))
	}

	// Test 3: Save another checkpoint for same run, different step
	checkpoint2 := CheckpointV2[TestState]{
		RunID:          "run-001",
		StepID:         2,
		State:          TestState{Value: "checkpoint2", Counter: 20},
		Frontier:       []string{"node-c"},
		RNGSeed:        67890,
		RecordedIOs:    []string{"io3"},
		IdempotencyKey: "idem-key-002",
		Timestamp:      time.Now(),
		Label:          "",
	}

	err = store.SaveCheckpointV2(ctx, checkpoint2)
	if err != nil {
		t.Fatalf("SaveCheckpointV2 (checkpoint2) failed: %v", err)
	}

	// Test 4: Load both checkpoints correctly
	loaded1, _ := store.LoadCheckpointV2(ctx, "run-001", 1)
	loaded2, _ := store.LoadCheckpointV2(ctx, "run-001", 2)

	if loaded1.State.Counter != 10 {
		t.Errorf("checkpoint1 Counter changed: got %d", loaded1.State.Counter)
	}
	if loaded2.State.Counter != 20 {
		t.Errorf("expected checkpoint2 Counter = 20, got %d", loaded2.State.Counter)
	}

	// Test 5: LoadCheckpointV2 on nonexistent checkpoint returns ErrNotFound
	_, err = store.LoadCheckpointV2(ctx, "run-001", 99)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound for nonexistent checkpoint, got: %v", err)
	}

	_, err = store.LoadCheckpointV2(ctx, "nonexistent-run", 1)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound for nonexistent run, got: %v", err)
	}
}

// TestSQLiteStore_Idempotency verifies idempotency key checking (T059, T074).
func TestSQLiteStore_Idempotency(t *testing.T) {
	ctx := context.Background()
	store := newTestSQLiteStore(t)
	defer store.Close()

	// Test 1: CheckIdempotency on unused key returns false
	exists, err := store.CheckIdempotency(ctx, "unused-key")
	if err != nil {
		t.Fatalf("CheckIdempotency failed: %v", err)
	}
	if exists {
		t.Error("expected unused key to return false")
	}

	// Test 2: Save checkpoint with idempotency key
	checkpoint := CheckpointV2[TestState]{
		RunID:          "run-001",
		StepID:         1,
		State:          TestState{Value: "test", Counter: 1},
		Frontier:       []string{},
		RNGSeed:        123,
		RecordedIOs:    []string{},
		IdempotencyKey: "test-idem-key",
		Timestamp:      time.Now(),
		Label:          "",
	}

	err = store.SaveCheckpointV2(ctx, checkpoint)
	if err != nil {
		t.Fatalf("SaveCheckpointV2 failed: %v", err)
	}

	// Test 3: CheckIdempotency on used key returns true
	exists, err = store.CheckIdempotency(ctx, "test-idem-key")
	if err != nil {
		t.Fatalf("CheckIdempotency (used key) failed: %v", err)
	}
	if !exists {
		t.Error("expected used key to return true")
	}

	// Test 4: Saving with duplicate idempotency key fails
	checkpoint2 := CheckpointV2[TestState]{
		RunID:          "run-001",
		StepID:         2,
		State:          TestState{Value: "duplicate", Counter: 2},
		Frontier:       []string{},
		RNGSeed:        456,
		RecordedIOs:    []string{},
		IdempotencyKey: "test-idem-key", // Same key
		Timestamp:      time.Now(),
		Label:          "",
	}

	err = store.SaveCheckpointV2(ctx, checkpoint2)
	if err == nil {
		t.Fatal("expected SaveCheckpointV2 to fail with duplicate idempotency key")
	}

	// Test 5: Original checkpoint still loads correctly
	loaded, err := store.LoadCheckpointV2(ctx, "run-001", 1)
	if err != nil {
		t.Fatalf("LoadCheckpointV2 failed: %v", err)
	}
	if loaded.State.Value != "test" {
		t.Errorf("expected original checkpoint unchanged, got Value = %q", loaded.State.Value)
	}

	// Test 6: Different run can use same idempotency key pattern (shouldn't conflict)
	// Note: This assumes idempotency keys are globally unique in the system
	checkpoint3 := CheckpointV2[TestState]{
		RunID:          "run-002",
		StepID:         1,
		State:          TestState{Value: "different-run", Counter: 3},
		Frontier:       []string{},
		RNGSeed:        789,
		RecordedIOs:    []string{},
		IdempotencyKey: "test-idem-key-2", // Different key
		Timestamp:      time.Now(),
		Label:          "",
	}

	err = store.SaveCheckpointV2(ctx, checkpoint3)
	if err != nil {
		t.Fatalf("SaveCheckpointV2 for different run failed: %v", err)
	}

	// Verify both keys are tracked
	exists1, _ := store.CheckIdempotency(ctx, "test-idem-key")
	exists2, _ := store.CheckIdempotency(ctx, "test-idem-key-2")
	if !exists1 || !exists2 {
		t.Error("expected both idempotency keys to be tracked")
	}
}

// TestSQLiteStore_Outbox verifies transactional outbox pattern (T060, T075).
func TestSQLiteStore_Outbox(t *testing.T) {
	ctx := context.Background()
	store := newTestSQLiteStore(t)
	defer store.Close()

	// Note: MemStore and SQLiteStore don't have a public AddEvent method yet.
	// This test validates PendingEvents and MarkEventsEmitted work correctly
	// assuming events are added through SaveCheckpointV2 or another mechanism.

	// For now, we'll test the query/mark cycle with manual insertion
	// This is a simplified test - full integration test would use actual event flow

	// Test 1: PendingEvents on empty outbox returns empty list
	events, err := store.PendingEvents(ctx, 10)
	if err != nil {
		t.Fatalf("PendingEvents failed: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 pending events, got %d", len(events))
	}

	// Test 2: Manually insert test events
	insertEventQuery := `
		INSERT INTO events_outbox (id, run_id, event_data, emitted_at, created_at)
		VALUES (?, ?, ?, ?, ?)
	`

	event1JSON := `{"type":"step_start","meta":{"event_id":"evt-001"}}`
	event2JSON := `{"type":"step_end","meta":{"event_id":"evt-002"}}`
	event3JSON := `{"type":"checkpoint","meta":{"event_id":"evt-003"}}`

	_, _ = store.db.ExecContext(ctx, insertEventQuery, "evt-001", "run-001", event1JSON, nil, time.Now())
	_, _ = store.db.ExecContext(ctx, insertEventQuery, "evt-002", "run-001", event2JSON, nil, time.Now())
	_, _ = store.db.ExecContext(ctx, insertEventQuery, "evt-003", "run-002", event3JSON, nil, time.Now())

	// Test 3: PendingEvents returns all pending (emitted_at IS NULL)
	events, err = store.PendingEvents(ctx, 10)
	if err != nil {
		t.Fatalf("PendingEvents failed: %v", err)
	}
	if len(events) != 3 {
		t.Errorf("expected 3 pending events, got %d", len(events))
	}

	// Test 4: PendingEvents respects limit
	events, err = store.PendingEvents(ctx, 2)
	if err != nil {
		t.Fatalf("PendingEvents (limit=2) failed: %v", err)
	}
	if len(events) != 2 {
		t.Errorf("expected 2 events with limit=2, got %d", len(events))
	}

	// Test 5: Mark some events as emitted
	err = store.MarkEventsEmitted(ctx, []string{"evt-001", "evt-002"})
	if err != nil {
		t.Fatalf("MarkEventsEmitted failed: %v", err)
	}

	// Test 6: PendingEvents now returns only unemitted events
	events, err = store.PendingEvents(ctx, 10)
	if err != nil {
		t.Fatalf("PendingEvents (after marking) failed: %v", err)
	}
	if len(events) != 1 {
		t.Errorf("expected 1 pending event after marking 2 as emitted, got %d", len(events))
	}
	if events[0].Meta["event_id"] != "evt-003" {
		t.Errorf("expected remaining event to be evt-003, got %v", events[0].Meta["event_id"])
	}

	// Test 7: MarkEventsEmitted is idempotent
	err = store.MarkEventsEmitted(ctx, []string{"evt-001"})
	if err != nil {
		t.Fatalf("MarkEventsEmitted (idempotent) failed: %v", err)
	}

	events, _ = store.PendingEvents(ctx, 10)
	if len(events) != 1 {
		t.Errorf("idempotent mark changed event count: got %d", len(events))
	}

	// Test 8: MarkEventsEmitted with empty list is no-op
	err = store.MarkEventsEmitted(ctx, []string{})
	if err != nil {
		t.Fatalf("MarkEventsEmitted (empty) failed: %v", err)
	}

	// Test 9: Mark remaining event
	err = store.MarkEventsEmitted(ctx, []string{"evt-003"})
	if err != nil {
		t.Fatalf("MarkEventsEmitted (evt-003) failed: %v", err)
	}

	// Test 10: All events marked, pending list is empty
	events, err = store.PendingEvents(ctx, 10)
	if err != nil {
		t.Fatalf("PendingEvents (final) failed: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 pending events after marking all, got %d", len(events))
	}
}

// TestSQLiteStore_ConcurrentReads verifies concurrent read operations (T061, T076).
func TestSQLiteStore_ConcurrentReads(t *testing.T) {
	ctx := context.Background()
	store := newTestSQLiteStore(t)
	defer store.Close()

	// Setup: Create multiple runs with steps
	for runNum := 1; runNum <= 10; runNum++ {
		runID := fmt.Sprintf("run-%03d", runNum)
		for step := 1; step <= 5; step++ {
			state := TestState{
				Value:   fmt.Sprintf("run%d-step%d", runNum, step),
				Counter: runNum*10 + step,
			}
			_ = store.SaveStep(ctx, runID, step, fmt.Sprintf("node-%d", step), state)
		}
	}

	// Test: Concurrent reads from multiple goroutines
	const numReaders = 20
	var wg sync.WaitGroup
	errors := make(chan error, numReaders)

	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func(readerID int) {
			defer wg.Done()

			// Each reader reads multiple runs
			for runNum := 1; runNum <= 10; runNum++ {
				runID := fmt.Sprintf("run-%03d", runNum)

				state, step, err := store.LoadLatest(ctx, runID)
				if err != nil {
					errors <- fmt.Errorf("reader %d: LoadLatest failed: %w", readerID, err)
					return
				}

				// Verify data correctness
				if step != 5 {
					errors <- fmt.Errorf("reader %d: expected step=5 for %s, got %d", readerID, runID, step)
					return
				}

				expectedValue := fmt.Sprintf("run%d-step5", runNum)
				if state.Value != expectedValue {
					errors <- fmt.Errorf("reader %d: expected Value=%q, got %q", readerID, expectedValue, state.Value)
					return
				}

				expectedCounter := runNum*10 + 5
				if state.Counter != expectedCounter {
					errors <- fmt.Errorf("reader %d: expected Counter=%d, got %d", readerID, expectedCounter, state.Counter)
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Error(err)
	}
}

// TestSQLiteStore_CloseAndReopen verifies persistence across close/reopen (bonus test).
func TestSQLiteStore_CloseAndReopen(t *testing.T) {
	ctx := context.Background()

	// Create a temporary file path (but let SQLite create it)
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	// Test 1: Create store and save data
	store1, err := NewSQLiteStore[TestState](dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}

	state1 := TestState{Value: "persistent", Counter: 42}
	_ = store1.SaveStep(ctx, "run-001", 1, "node-a", state1)

	checkpoint := CheckpointV2[TestState]{
		RunID:          "run-001",
		StepID:         1,
		State:          state1,
		Frontier:       []string{"node-b"},
		RNGSeed:        999,
		RecordedIOs:    []string{},
		IdempotencyKey: "persist-key",
		Timestamp:      time.Now(),
		Label:          "test-checkpoint",
	}
	_ = store1.SaveCheckpointV2(ctx, checkpoint)

	// Close the store
	err = store1.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Test 2: Reopen store and verify data persists
	store2, err := NewSQLiteStore[TestState](dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore (reopen) failed: %v", err)
	}
	defer store2.Close()

	// Load step
	loadedState, step, err := store2.LoadLatest(ctx, "run-001")
	if err != nil {
		t.Fatalf("LoadLatest after reopen failed: %v", err)
	}
	if loadedState.Value != "persistent" {
		t.Errorf("expected Value='persistent' after reopen, got %q", loadedState.Value)
	}
	if step != 1 {
		t.Errorf("expected step=1 after reopen, got %d", step)
	}

	// Load checkpoint
	loadedCheckpoint, err := store2.LoadCheckpointV2(ctx, "run-001", 1)
	if err != nil {
		t.Fatalf("LoadCheckpointV2 after reopen failed: %v", err)
	}
	if loadedCheckpoint.Label != "test-checkpoint" {
		t.Errorf("expected Label='test-checkpoint' after reopen, got %q", loadedCheckpoint.Label)
	}

	// Check idempotency key
	exists, err := store2.CheckIdempotency(ctx, "persist-key")
	if err != nil {
		t.Fatalf("CheckIdempotency after reopen failed: %v", err)
	}
	if !exists {
		t.Error("expected idempotency key to persist after reopen")
	}
}

// TestSQLiteStore_LegacyCheckpoint verifies legacy SaveCheckpoint/LoadCheckpoint (bonus test).
func TestSQLiteStore_LegacyCheckpoint(t *testing.T) {
	ctx := context.Background()
	store := newTestSQLiteStore(t)
	defer store.Close()

	// Test 1: Save legacy checkpoint
	state := TestState{Value: "legacy", Counter: 100}
	err := store.SaveCheckpoint(ctx, "cp-001", state, 5)
	if err != nil {
		t.Fatalf("SaveCheckpoint failed: %v", err)
	}

	// Test 2: Load legacy checkpoint
	loadedState, step, err := store.LoadCheckpoint(ctx, "cp-001")
	if err != nil {
		t.Fatalf("LoadCheckpoint failed: %v", err)
	}
	if step != 5 {
		t.Errorf("expected step=5, got %d", step)
	}
	if loadedState.Value != "legacy" {
		t.Errorf("expected Value='legacy', got %q", loadedState.Value)
	}

	// Test 3: Update existing checkpoint
	state2 := TestState{Value: "updated", Counter: 200}
	err = store.SaveCheckpoint(ctx, "cp-001", state2, 10)
	if err != nil {
		t.Fatalf("SaveCheckpoint (update) failed: %v", err)
	}

	loadedState, step, err = store.LoadCheckpoint(ctx, "cp-001")
	if err != nil {
		t.Fatalf("LoadCheckpoint (after update) failed: %v", err)
	}
	if step != 10 {
		t.Errorf("expected updated step=10, got %d", step)
	}
	if loadedState.Value != "updated" {
		t.Errorf("expected Value='updated', got %q", loadedState.Value)
	}

	// Test 4: LoadCheckpoint on nonexistent checkpoint returns ErrNotFound
	_, _, err = store.LoadCheckpoint(ctx, "nonexistent-cp")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

// TestSQLiteStore_ClosedStoreErrors verifies operations fail after Close (bonus test).
func TestSQLiteStore_ClosedStoreErrors(t *testing.T) {
	ctx := context.Background()
	store := newTestSQLiteStore(t)

	// Close the store
	err := store.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// All operations should return errors
	state := TestState{Value: "test", Counter: 1}

	err = store.SaveStep(ctx, "run-001", 1, "node-a", state)
	if err == nil {
		t.Error("expected SaveStep to fail on closed store")
	}

	_, _, err = store.LoadLatest(ctx, "run-001")
	if err == nil {
		t.Error("expected LoadLatest to fail on closed store")
	}

	err = store.SaveCheckpoint(ctx, "cp-001", state, 1)
	if err == nil {
		t.Error("expected SaveCheckpoint to fail on closed store")
	}

	_, _, err = store.LoadCheckpoint(ctx, "cp-001")
	if err == nil {
		t.Error("expected LoadCheckpoint to fail on closed store")
	}

	checkpoint := CheckpointV2[TestState]{
		RunID:          "run-001",
		StepID:         1,
		State:          state,
		Frontier:       []string{},
		RNGSeed:        123,
		RecordedIOs:    []string{},
		IdempotencyKey: "key",
		Timestamp:      time.Now(),
		Label:          "",
	}
	err = store.SaveCheckpointV2(ctx, checkpoint)
	if err == nil {
		t.Error("expected SaveCheckpointV2 to fail on closed store")
	}

	_, err = store.LoadCheckpointV2(ctx, "run-001", 1)
	if err == nil {
		t.Error("expected LoadCheckpointV2 to fail on closed store")
	}

	_, err = store.CheckIdempotency(ctx, "key")
	if err == nil {
		t.Error("expected CheckIdempotency to fail on closed store")
	}

	_, err = store.PendingEvents(ctx, 10)
	if err == nil {
		t.Error("expected PendingEvents to fail on closed store")
	}

	err = store.MarkEventsEmitted(ctx, []string{"evt-001"})
	if err == nil {
		t.Error("expected MarkEventsEmitted to fail on closed store")
	}

	// Double close should be safe (no-op)
	err = store.Close()
	if err != nil {
		t.Error("expected double Close to succeed (no-op)")
	}
}

// TestSQLiteStore_InterfaceCompliance verifies SQLiteStore implements Store interface.
func TestSQLiteStore_InterfaceCompliance(t *testing.T) {
	var _ Store[TestState] = (*SQLiteStore[TestState])(nil)
}

// newTestSQLiteStore creates an in-memory SQLite store for testing.
func newTestSQLiteStore(t *testing.T) *SQLiteStore[TestState] {
	store, err := NewSQLiteStore[TestState](":memory:")
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}
	return store
}
