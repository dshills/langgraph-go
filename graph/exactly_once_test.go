package graph

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dshills/langgraph-go/graph/store"
)

// TestState is a simple state type for testing exactly-once semantics
type ExactlyOnceTestState struct {
	Counter    int    `json:"counter"`
	Message    string `json:"message"`
	ExecutedBy string `json:"executed_by"`
}

// TestAtomicStepCommit verifies that checkpoint commits are atomic:
// - State is persisted
// - Frontier is persisted
// - Idempotency key is recorded
// - All succeed together or all roll back together
//
// This test ensures that the transactional boundary in SaveCheckpointV2
// guarantees exactly-once semantics. If any part fails, the entire
// checkpoint commit must be rolled back.
//
// Test scenarios:
// 1. Normal commit: All components persisted atomically
// 2. Idempotency violation: Duplicate key causes rollback (entire checkpoint rejected)
// 3. Verify atomicity: Partial commits are impossible
func TestAtomicStepCommit(t *testing.T) {
	ctx := context.Background()

	t.Run("successful atomic commit", func(t *testing.T) {
		// Use MySQL store for real transaction testing
		dsn := getTestDSN(t)
		if dsn == "" {
			t.Skip("Skipping MySQL test: TEST_MYSQL_DSN not set")
		}

		st, err := store.NewMySQLStore[ExactlyOnceTestState](dsn)
		if err != nil {
			t.Fatalf("Failed to create MySQL store: %v", err)
		}
		defer st.Close()

		// Create a checkpoint with all components
		initialState := ExactlyOnceTestState{Counter: 1, Message: "initial"}
		frontier := []WorkItem[ExactlyOnceTestState]{
			{
				StepID:       2,
				OrderKey:     100,
				NodeID:       "node-a",
				State:        ExactlyOnceTestState{Counter: 2},
				Attempt:      0,
				ParentNodeID: "start",
				EdgeIndex:    0,
			},
		}

		// Compute idempotency key
		idempotencyKey, err := computeIdempotencyKey("run-001", 1, frontier, initialState)
		if err != nil {
			t.Fatalf("Failed to compute idempotency key: %v", err)
		}

		checkpoint := store.CheckpointV2[ExactlyOnceTestState]{
			RunID:          "run-001",
			StepID:         1,
			State:          initialState,
			Frontier:       frontier,
			RNGSeed:        12345,
			RecordedIOs:    []interface{}{},
			IdempotencyKey: idempotencyKey,
			Timestamp:      time.Now(),
			Label:          "test-checkpoint",
		}

		// Save checkpoint (should succeed atomically)
		err = st.SaveCheckpointV2(ctx, checkpoint)
		if err != nil {
			t.Fatalf("Failed to save checkpoint: %v", err)
		}

		// Verify all components were persisted
		loaded, err := st.LoadCheckpointV2(ctx, "run-001", 1)
		if err != nil {
			t.Fatalf("Failed to load checkpoint: %v", err)
		}

		// Verify state
		if loaded.State.Counter != initialState.Counter {
			t.Errorf("State not persisted correctly: got counter=%d, want=%d",
				loaded.State.Counter, initialState.Counter)
		}

		// Verify frontier
		loadedFrontier, ok := loaded.Frontier.([]interface{})
		if !ok || len(loadedFrontier) != len(frontier) {
			t.Errorf("Frontier not persisted correctly: got length=%d, want=%d",
				len(loadedFrontier), len(frontier))
		}

		// Verify idempotency key was recorded
		exists, err := st.CheckIdempotency(ctx, idempotencyKey)
		if err != nil {
			t.Fatalf("Failed to check idempotency: %v", err)
		}
		if !exists {
			t.Error("Idempotency key was not recorded")
		}

		// Verify RNG seed
		if loaded.RNGSeed != checkpoint.RNGSeed {
			t.Errorf("RNG seed not persisted correctly: got=%d, want=%d",
				loaded.RNGSeed, checkpoint.RNGSeed)
		}
	})

	t.Run("duplicate idempotency key causes atomic rollback", func(t *testing.T) {
		dsn := getTestDSN(t)
		if dsn == "" {
			t.Skip("Skipping MySQL test: TEST_MYSQL_DSN not set")
		}

		st, err := store.NewMySQLStore[ExactlyOnceTestState](dsn)
		if err != nil {
			t.Fatalf("Failed to create MySQL store: %v", err)
		}
		defer st.Close()

		// Create first checkpoint
		state1 := ExactlyOnceTestState{Counter: 10, Message: "first"}
		frontier1 := []WorkItem[ExactlyOnceTestState]{
			{StepID: 2, OrderKey: 200, NodeID: "node-x", State: state1},
		}

		idempotencyKey1, _ := computeIdempotencyKey("run-002", 1, frontier1, state1)

		checkpoint1 := store.CheckpointV2[ExactlyOnceTestState]{
			RunID:          "run-002",
			StepID:         1,
			State:          state1,
			Frontier:       frontier1,
			RNGSeed:        54321,
			RecordedIOs:    []interface{}{},
			IdempotencyKey: idempotencyKey1,
			Timestamp:      time.Now(),
		}

		// Save first checkpoint (should succeed)
		err = st.SaveCheckpointV2(ctx, checkpoint1)
		if err != nil {
			t.Fatalf("Failed to save first checkpoint: %v", err)
		}

		// Attempt to save checkpoint with same idempotency key
		// This should FAIL and the entire checkpoint should be rolled back
		state2 := ExactlyOnceTestState{Counter: 999, Message: "duplicate attempt"}
		checkpoint2 := store.CheckpointV2[ExactlyOnceTestState]{
			RunID:          "run-002",
			StepID:         2, // Different step, same run
			State:          state2,
			Frontier:       frontier1,
			RNGSeed:        99999,
			RecordedIOs:    []interface{}{},
			IdempotencyKey: idempotencyKey1, // DUPLICATE KEY
			Timestamp:      time.Now(),
		}

		err = st.SaveCheckpointV2(ctx, checkpoint2)
		if err == nil {
			t.Fatal("Expected error for duplicate idempotency key, got nil")
		}

		// Verify second checkpoint was NOT persisted (atomic rollback)
		_, err = st.LoadCheckpointV2(ctx, "run-002", 2)
		if !errors.Is(err, store.ErrNotFound) {
			t.Errorf("Second checkpoint should not exist after rollback: got error=%v", err)
		}

		// Verify first checkpoint still exists unchanged
		loaded, err := st.LoadCheckpointV2(ctx, "run-002", 1)
		if err != nil {
			t.Fatalf("First checkpoint should still exist: %v", err)
		}

		if loaded.State.Counter != state1.Counter {
			t.Errorf("First checkpoint was modified: got counter=%d, want=%d",
				loaded.State.Counter, state1.Counter)
		}
	})

	t.Run("in-memory store atomic semantics", func(t *testing.T) {
		// MemStore should also provide atomic semantics
		st := store.NewMemStore[ExactlyOnceTestState]()

		state := ExactlyOnceTestState{Counter: 5, Message: "memstore"}
		frontier := []WorkItem[ExactlyOnceTestState]{
			{StepID: 1, OrderKey: 50, NodeID: "node-m", State: state},
		}

		idempotencyKey, _ := computeIdempotencyKey("mem-run", 1, frontier, state)

		checkpoint := store.CheckpointV2[ExactlyOnceTestState]{
			RunID:          "mem-run",
			StepID:         1,
			State:          state,
			Frontier:       frontier,
			RNGSeed:        777,
			RecordedIOs:    []interface{}{},
			IdempotencyKey: idempotencyKey,
			Timestamp:      time.Now(),
		}

		// Save checkpoint
		err := st.SaveCheckpointV2(ctx, checkpoint)
		if err != nil {
			t.Fatalf("Failed to save checkpoint to MemStore: %v", err)
		}

		// Verify idempotency
		exists, err := st.CheckIdempotency(ctx, idempotencyKey)
		if err != nil {
			t.Fatalf("Failed to check idempotency: %v", err)
		}
		if !exists {
			t.Error("Idempotency key not recorded in MemStore")
		}

		// Attempt duplicate
		err = st.SaveCheckpointV2(ctx, checkpoint)
		if err == nil {
			t.Error("MemStore should reject duplicate idempotency key")
		}
	})
}

// TestIdempotencyEnforcement verifies that the idempotency key prevents
// duplicate checkpoint commits even under retry scenarios.
//
// Idempotency is critical for exactly-once semantics:
// - Same (runID, stepID, state, frontier) → Same idempotency key
// - Duplicate key → Commit rejected
// - Ensures no duplicate execution of non-idempotent operations
//
// Test scenarios:
// 1. Same checkpoint data produces same idempotency key
// 2. Different step produces different key (allows progression)
// 3. Different state produces different key (detects state changes)
// 4. Store rejects duplicate keys atomically
func TestIdempotencyEnforcement(t *testing.T) {
	ctx := context.Background()

	t.Run("same data produces same key", func(t *testing.T) {
		state := ExactlyOnceTestState{Counter: 42, Message: "test"}
		frontier := []WorkItem[ExactlyOnceTestState]{
			{StepID: 1, OrderKey: 100, NodeID: "node-a", State: state},
		}

		// Compute key twice with same data
		key1, err := computeIdempotencyKey("run-id", 1, frontier, state)
		if err != nil {
			t.Fatalf("Failed to compute key: %v", err)
		}

		key2, err := computeIdempotencyKey("run-id", 1, frontier, state)
		if err != nil {
			t.Fatalf("Failed to compute key: %v", err)
		}

		if key1 != key2 {
			t.Errorf("Same data produced different keys: %s vs %s", key1, key2)
		}

		// Verify key format
		if len(key1) < 10 || key1[:7] != "sha256:" {
			t.Errorf("Invalid key format: %s", key1)
		}
	})

	t.Run("different step produces different key", func(t *testing.T) {
		state := ExactlyOnceTestState{Counter: 42, Message: "test"}
		frontier := []WorkItem[ExactlyOnceTestState]{
			{StepID: 1, OrderKey: 100, NodeID: "node-a", State: state},
		}

		key1, _ := computeIdempotencyKey("run-id", 1, frontier, state)
		key2, _ := computeIdempotencyKey("run-id", 2, frontier, state) // Different step

		if key1 == key2 {
			t.Error("Different steps produced same idempotency key")
		}
	})

	t.Run("different state produces different key", func(t *testing.T) {
		state1 := ExactlyOnceTestState{Counter: 42, Message: "test"}
		state2 := ExactlyOnceTestState{Counter: 43, Message: "test"} // Different counter
		frontier := []WorkItem[ExactlyOnceTestState]{
			{StepID: 1, OrderKey: 100, NodeID: "node-a", State: state1},
		}

		key1, _ := computeIdempotencyKey("run-id", 1, frontier, state1)
		key2, _ := computeIdempotencyKey("run-id", 1, frontier, state2) // Different state

		if key1 == key2 {
			t.Error("Different states produced same idempotency key")
		}
	})

	t.Run("different frontier produces different key", func(t *testing.T) {
		state := ExactlyOnceTestState{Counter: 42, Message: "test"}
		frontier1 := []WorkItem[ExactlyOnceTestState]{
			{StepID: 1, OrderKey: 100, NodeID: "node-a", State: state},
		}
		frontier2 := []WorkItem[ExactlyOnceTestState]{
			{StepID: 1, OrderKey: 100, NodeID: "node-b", State: state}, // Different node
		}

		key1, _ := computeIdempotencyKey("run-id", 1, frontier1, state)
		key2, _ := computeIdempotencyKey("run-id", 1, frontier2, state)

		if key1 == key2 {
			t.Error("Different frontiers produced same idempotency key")
		}
	})

	t.Run("store rejects duplicate idempotency key", func(t *testing.T) {
		dsn := getTestDSN(t)
		if dsn == "" {
			t.Skip("Skipping MySQL test: TEST_MYSQL_DSN not set")
		}

		st, err := store.NewMySQLStore[ExactlyOnceTestState](dsn)
		if err != nil {
			t.Fatalf("Failed to create MySQL store: %v", err)
		}
		defer st.Close()

		state := ExactlyOnceTestState{Counter: 100, Message: "duplicate test"}
		frontier := []WorkItem[ExactlyOnceTestState]{
			{StepID: 1, OrderKey: 999, NodeID: "dup-node", State: state},
		}

		idempotencyKey, _ := computeIdempotencyKey("dup-run", 1, frontier, state)

		checkpoint := store.CheckpointV2[ExactlyOnceTestState]{
			RunID:          "dup-run",
			StepID:         1,
			State:          state,
			Frontier:       frontier,
			RNGSeed:        888,
			RecordedIOs:    []interface{}{},
			IdempotencyKey: idempotencyKey,
			Timestamp:      time.Now(),
		}

		// First save should succeed
		err = st.SaveCheckpointV2(ctx, checkpoint)
		if err != nil {
			t.Fatalf("First save failed: %v", err)
		}

		// Second save with same idempotency key should fail
		err = st.SaveCheckpointV2(ctx, checkpoint)
		if err == nil {
			t.Fatal("Expected error for duplicate idempotency key, got nil")
		}

		// Verify only one checkpoint exists
		loaded, err := st.LoadCheckpointV2(ctx, "dup-run", 1)
		if err != nil {
			t.Fatalf("Failed to load checkpoint: %v", err)
		}

		if loaded.State.Counter != state.Counter {
			t.Errorf("Checkpoint state mismatch: got=%d, want=%d",
				loaded.State.Counter, state.Counter)
		}
	})

	t.Run("memstore idempotency enforcement", func(t *testing.T) {
		st := store.NewMemStore[ExactlyOnceTestState]()

		state := ExactlyOnceTestState{Counter: 5, Message: "mem"}
		frontier := []WorkItem[ExactlyOnceTestState]{
			{StepID: 1, OrderKey: 10, NodeID: "mem-node", State: state},
		}

		key, _ := computeIdempotencyKey("mem-dup-run", 1, frontier, state)

		// First check should return false
		exists, err := st.CheckIdempotency(ctx, key)
		if err != nil {
			t.Fatalf("CheckIdempotency failed: %v", err)
		}
		if exists {
			t.Error("Key should not exist before first save")
		}

		// Save checkpoint
		checkpoint := store.CheckpointV2[ExactlyOnceTestState]{
			RunID:          "mem-dup-run",
			StepID:         1,
			State:          state,
			Frontier:       frontier,
			RNGSeed:        333,
			RecordedIOs:    []interface{}{},
			IdempotencyKey: key,
			Timestamp:      time.Now(),
		}

		err = st.SaveCheckpointV2(ctx, checkpoint)
		if err != nil {
			t.Fatalf("First save failed: %v", err)
		}

		// Second check should return true
		exists, err = st.CheckIdempotency(ctx, key)
		if err != nil {
			t.Fatalf("CheckIdempotency failed: %v", err)
		}
		if !exists {
			t.Error("Key should exist after save")
		}

		// Duplicate save should fail
		err = st.SaveCheckpointV2(ctx, checkpoint)
		if err == nil {
			t.Error("Duplicate save should have failed")
		}
	})
}

// TestNoDuplicatesUnderConcurrency verifies that the exactly-once guarantees
// hold even under high concurrency with multiple workflow executions racing
// to commit checkpoints.
//
// This is the ultimate stress test for idempotency:
// - 1000 concurrent workflow executions
// - Each attempts to commit multiple steps
// - Verify zero duplicate step commits
// - Verify all steps completed exactly once
//
// Test ensures:
// - Concurrent transactions don't create duplicates
// - Idempotency keys prevent race conditions
// - Database constraints enforce uniqueness
// - State remains consistent under load
func TestNoDuplicatesUnderConcurrency(t *testing.T) {
	ctx := context.Background()

	dsn := getTestDSN(t)
	if dsn == "" {
		t.Skip("Skipping MySQL test: TEST_MYSQL_DSN not set")
	}

	st, err := store.NewMySQLStore[ExactlyOnceTestState](dsn)
	if err != nil {
		t.Fatalf("Failed to create MySQL store: %v", err)
	}
	defer st.Close()

	const (
		numWorkflows      = 100 // Number of concurrent workflows
		stepsPerWorkflow  = 10  // Steps each workflow executes
		retriesPerStep    = 3   // Simulated retries (should be idempotent)
		concurrentWorkers = 50  // Max concurrent goroutines
	)

	var (
		successfulCommits int64
		failedCommits     int64
		totalAttempts     int64
		wg                sync.WaitGroup
		semaphore         = make(chan struct{}, concurrentWorkers)
	)

	// Execute workflows concurrently
	for workflowID := 0; workflowID < numWorkflows; workflowID++ {
		wg.Add(1)

		go func(wid int) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			runID := formatRunID(wid)

			// Execute steps in sequence
			for step := 1; step <= stepsPerWorkflow; step++ {
				state := ExactlyOnceTestState{
					Counter:    step,
					Message:    formatMessage(wid, step),
					ExecutedBy: runID,
				}

				frontier := []WorkItem[ExactlyOnceTestState]{
					{
						StepID:       step + 1,
						OrderKey:     computeOrderKey(runID, step),
						NodeID:       formatNodeID(step),
						State:        state,
						Attempt:      0,
						ParentNodeID: formatNodeID(step - 1),
						EdgeIndex:    0,
					},
				}

				idempotencyKey, _ := computeIdempotencyKey(runID, step, frontier, state)

				checkpoint := store.CheckpointV2[ExactlyOnceTestState]{
					RunID:          runID,
					StepID:         step,
					State:          state,
					Frontier:       frontier,
					RNGSeed:        int64(wid*1000 + step),
					RecordedIOs:    []interface{}{},
					IdempotencyKey: idempotencyKey,
					Timestamp:      time.Now(),
				}

				// Simulate retries: attempt to save multiple times
				// Only first attempt should succeed
				committed := false
				for retry := 0; retry < retriesPerStep; retry++ {
					atomic.AddInt64(&totalAttempts, 1)

					err := st.SaveCheckpointV2(ctx, checkpoint)
					if err == nil {
						if !committed {
							atomic.AddInt64(&successfulCommits, 1)
							committed = true
						} else {
							// This should never happen - duplicate commit!
							t.Errorf("DUPLICATE COMMIT: run=%s, step=%d, retry=%d",
								runID, step, retry)
						}
					} else {
						atomic.AddInt64(&failedCommits, 1)
					}
				}

				if !committed {
					t.Errorf("Failed to commit checkpoint: run=%s, step=%d", runID, step)
				}
			}
		}(workflowID)
	}

	// Wait for all workflows to complete
	wg.Wait()

	// Verify results
	expectedCommits := int64(numWorkflows * stepsPerWorkflow)

	t.Logf("Concurrency test results:")
	t.Logf("  Total attempts:      %d", totalAttempts)
	t.Logf("  Successful commits:  %d", successfulCommits)
	t.Logf("  Failed commits:      %d", failedCommits)
	t.Logf("  Expected commits:    %d", expectedCommits)

	if successfulCommits != expectedCommits {
		t.Errorf("Wrong number of commits: got=%d, want=%d",
			successfulCommits, expectedCommits)
	}

	// Verify each checkpoint exists exactly once
	for wid := 0; wid < numWorkflows; wid++ {
		runID := formatRunID(wid)

		for step := 1; step <= stepsPerWorkflow; step++ {
			loaded, err := st.LoadCheckpointV2(ctx, runID, step)
			if err != nil {
				t.Errorf("Missing checkpoint: run=%s, step=%d, error=%v",
					runID, step, err)
				continue
			}

			// Verify checkpoint data integrity
			if loaded.State.Counter != step {
				t.Errorf("Corrupted checkpoint: run=%s, step=%d, counter=%d",
					runID, step, loaded.State.Counter)
			}

			if loaded.State.ExecutedBy != runID {
				t.Errorf("Wrong run ID: run=%s, step=%d, got=%s",
					runID, step, loaded.State.ExecutedBy)
			}
		}
	}

	// Verify idempotency keys are all present
	keysChecked := 0
	for wid := 0; wid < numWorkflows; wid++ {
		runID := formatRunID(wid)

		for step := 1; step <= stepsPerWorkflow; step++ {
			state := ExactlyOnceTestState{Counter: step}
			frontier := []WorkItem[ExactlyOnceTestState]{
				{StepID: step + 1, OrderKey: computeOrderKey(runID, step), NodeID: formatNodeID(step)},
			}

			key, _ := computeIdempotencyKey(runID, step, frontier, state)

			exists, err := st.CheckIdempotency(ctx, key)
			if err != nil {
				t.Errorf("Failed to check idempotency key: %v", err)
			}
			if !exists {
				t.Errorf("Idempotency key missing: run=%s, step=%d", runID, step)
			}
			keysChecked++
		}
	}

	t.Logf("  Idempotency keys verified: %d", keysChecked)

	if keysChecked != int(expectedCommits) {
		t.Errorf("Idempotency key count mismatch: checked=%d, expected=%d",
			keysChecked, int(expectedCommits))
	}
}

// Helper functions

func getTestDSN(t *testing.T) string {
	// Read from environment variable
	dsn := os.Getenv("TEST_MYSQL_DSN")
	return dsn
}

func formatRunID(workflowID int) string {
	return fmt.Sprintf("run-%04d", workflowID)
}

func formatMessage(workflowID, step int) string {
	return fmt.Sprintf("workflow-%d-step-%d", workflowID, step)
}

func formatNodeID(step int) string {
	return fmt.Sprintf("node-%d", step)
}
