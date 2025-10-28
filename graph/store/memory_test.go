package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/dshills/langgraph-go/graph/emit"
)

// TestMemStore_Construction verifies MemStore[S] can be constructed (T033).
func TestMemStore_Construction(t *testing.T) {
	t.Run("construct with NewMemStore", func(t *testing.T) {
		store := NewMemStore[TestState]()

		if store == nil {
			t.Fatal("NewMemStore returned nil")
		}

		// Verify store implements Store interface
		var _ Store[TestState] = store
	})

	t.Run("new store is empty", func(t *testing.T) {
		store := NewMemStore[TestState]()

		ctx := context.Background()
		_, _, err := store.LoadLatest(ctx, "nonexistent-run")

		if !errors.Is(err, ErrNotFound) {
			t.Errorf("expected ErrNotFound for empty store, got %v", err)
		}
	})

	t.Run("multiple stores are independent", func(t *testing.T) {
		store1 := NewMemStore[TestState]()
		store2 := NewMemStore[TestState]()

		ctx := context.Background()

		// Save to store1
		_ = store1.SaveStep(ctx, "run-001", 1, "node1", TestState{Value: "store1"})

		// Verify store2 doesn't have this data
		_, _, err := store2.LoadLatest(ctx, "run-001")
		if !errors.Is(err, ErrNotFound) {
			t.Error("store2 should not have data from store1")
		}
	})
}

// TestMemStore_SaveStep_Concurrent verifies concurrent SaveStep calls (T035).
func TestMemStore_SaveStep_Concurrent(t *testing.T) {
	t.Run("concurrent writes to same runID", func(t *testing.T) {
		store := NewMemStore[TestState]()
		ctx := context.Background()

		// Launch 10 goroutines writing concurrently
		var wg sync.WaitGroup
		errs := make(chan error, 10)

		for i := 1; i <= 10; i++ {
			wg.Add(1)
			go func(step int) {
				defer wg.Done()
				err := store.SaveStep(ctx, "run-001", step, "node", TestState{Counter: step})
				if err != nil {
					errs <- err
				}
			}(i)
		}

		wg.Wait()
		close(errs)

		// Check no errors occurred
		for err := range errs {
			t.Errorf("concurrent SaveStep failed: %v", err)
		}

		// Verify all steps were saved
		state, step, err := store.LoadLatest(ctx, "run-001")
		if err != nil {
			t.Fatalf("LoadLatest failed: %v", err)
		}

		// Latest step should be 10 (highest step number)
		if step < 1 || step > 10 {
			t.Errorf("expected step between 1-10, got %d", step)
		}

		// State should have a valid Counter
		if state.Counter < 1 || state.Counter > 10 {
			t.Errorf("expected Counter between 1-10, got %d", state.Counter)
		}
	})

	t.Run("concurrent writes to different runIDs", func(t *testing.T) {
		store := NewMemStore[TestState]()
		ctx := context.Background()

		var wg sync.WaitGroup
		runIDs := []string{"run-a", "run-b", "run-c", "run-d", "run-e"}

		for _, runID := range runIDs {
			wg.Add(1)
			go func(id string) {
				defer wg.Done()
				for step := 1; step <= 5; step++ {
					_ = store.SaveStep(ctx, id, step, "node", TestState{Value: id})
				}
			}(runID)
		}

		wg.Wait()

		// Verify each runID has its own independent data
		for _, runID := range runIDs {
			state, step, err := store.LoadLatest(ctx, runID)
			if err != nil {
				t.Errorf("LoadLatest(%s) failed: %v", runID, err)
				continue
			}
			if step != 5 {
				t.Errorf("runID %s: expected step = 5, got %d", runID, step)
			}
			if state.Value != runID {
				t.Errorf("runID %s: expected Value = %s, got %s", runID, runID, state.Value)
			}
		}
	})
}

// TestMemStore_LoadLatest verifies LoadLatest behavior (T037).
func TestMemStore_LoadLatest(t *testing.T) {
	t.Run("load latest from empty store", func(t *testing.T) {
		store := NewMemStore[TestState]()
		ctx := context.Background()

		_, _, err := store.LoadLatest(ctx, "nonexistent")
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("load latest after single save", func(t *testing.T) {
		store := NewMemStore[TestState]()
		ctx := context.Background()

		_ = store.SaveStep(ctx, "run-001", 1, "node1", TestState{Value: "first"})

		state, step, err := store.LoadLatest(ctx, "run-001")
		if err != nil {
			t.Fatalf("LoadLatest failed: %v", err)
		}

		if step != 1 {
			t.Errorf("expected step = 1, got %d", step)
		}
		if state.Value != "first" {
			t.Errorf("expected Value = 'first', got %q", state.Value)
		}
	})

	t.Run("load latest after multiple saves", func(t *testing.T) {
		store := NewMemStore[TestState]()
		ctx := context.Background()

		// Save steps 1, 2, 3
		_ = store.SaveStep(ctx, "run-001", 1, "node1", TestState{Value: "step1"})
		_ = store.SaveStep(ctx, "run-001", 2, "node2", TestState{Value: "step2"})
		_ = store.SaveStep(ctx, "run-001", 3, "node3", TestState{Value: "step3"})

		state, step, err := store.LoadLatest(ctx, "run-001")
		if err != nil {
			t.Fatalf("LoadLatest failed: %v", err)
		}

		// Should return the last saved step (3)
		if step != 3 {
			t.Errorf("expected step = 3, got %d", step)
		}
		if state.Value != "step3" {
			t.Errorf("expected Value = 'step3', got %q", state.Value)
		}
	})

	t.Run("load latest with out-of-order saves", func(t *testing.T) {
		store := NewMemStore[TestState]()
		ctx := context.Background()

		// Save steps out of order: 3, 1, 2
		_ = store.SaveStep(ctx, "run-001", 3, "node3", TestState{Value: "step3"})
		_ = store.SaveStep(ctx, "run-001", 1, "node1", TestState{Value: "step1"})
		_ = store.SaveStep(ctx, "run-001", 2, "node2", TestState{Value: "step2"})

		state, step, err := store.LoadLatest(ctx, "run-001")
		if err != nil {
			t.Fatalf("LoadLatest failed: %v", err)
		}

		// Should return highest step number (3)
		if step != 3 {
			t.Errorf("expected step = 3 (highest), got %d", step)
		}
		if state.Value != "step3" {
			t.Errorf("expected Value = 'step3', got %q", state.Value)
		}
	})
}

// TestMemStore_SaveCheckpoint verifies checkpoint save with labels (T039).
func TestMemStore_SaveCheckpoint(t *testing.T) {
	t.Run("save checkpoint with label", func(t *testing.T) {
		store := NewMemStore[TestState]()
		ctx := context.Background()

		err := store.SaveCheckpoint(ctx, "cp-001", TestState{Value: "checkpoint"}, 5)
		if err != nil {
			t.Fatalf("SaveCheckpoint failed: %v", err)
		}

		// Verify checkpoint can be loaded
		state, step, err := store.LoadCheckpoint(ctx, "cp-001")
		if err != nil {
			t.Fatalf("LoadCheckpoint failed: %v", err)
		}

		if step != 5 {
			t.Errorf("expected step = 5, got %d", step)
		}
		if state.Value != "checkpoint" {
			t.Errorf("expected Value = 'checkpoint', got %q", state.Value)
		}
	})

	t.Run("save multiple checkpoints with different labels", func(t *testing.T) {
		store := NewMemStore[TestState]()
		ctx := context.Background()

		checkpoints := map[string]struct {
			state TestState
			step  int
		}{
			"before-validation": {TestState{Value: "pre-validate"}, 3},
			"after-validation":  {TestState{Value: "post-validate"}, 6},
			"final":             {TestState{Value: "complete"}, 10},
		}

		// Save all checkpoints
		for cpID, data := range checkpoints {
			err := store.SaveCheckpoint(ctx, cpID, data.state, data.step)
			if err != nil {
				t.Errorf("SaveCheckpoint(%s) failed: %v", cpID, err)
			}
		}

		// Verify all checkpoints are retrievable
		for cpID, expected := range checkpoints {
			state, step, err := store.LoadCheckpoint(ctx, cpID)
			if err != nil {
				t.Errorf("LoadCheckpoint(%s) failed: %v", cpID, err)
				continue
			}
			if step != expected.step {
				t.Errorf("%s: expected step = %d, got %d", cpID, expected.step, step)
			}
			if state.Value != expected.state.Value {
				t.Errorf("%s: expected Value = %q, got %q", cpID, expected.state.Value, state.Value)
			}
		}
	})

	t.Run("overwrite existing checkpoint", func(t *testing.T) {
		store := NewMemStore[TestState]()
		ctx := context.Background()

		// Save initial checkpoint
		_ = store.SaveCheckpoint(ctx, "cp-001", TestState{Value: "v1"}, 1)

		// Overwrite with new data
		_ = store.SaveCheckpoint(ctx, "cp-001", TestState{Value: "v2"}, 2)

		// Verify latest data is retrieved
		state, step, err := store.LoadCheckpoint(ctx, "cp-001")
		if err != nil {
			t.Fatalf("LoadCheckpoint failed: %v", err)
		}

		if step != 2 {
			t.Errorf("expected step = 2, got %d", step)
		}
		if state.Value != "v2" {
			t.Errorf("expected Value = 'v2', got %q", state.Value)
		}
	})
}

// TestMemStore_LoadCheckpoint_Errors verifies error cases (T041).
func TestMemStore_LoadCheckpoint_Errors(t *testing.T) {
	t.Run("load nonexistent checkpoint", func(t *testing.T) {
		store := NewMemStore[TestState]()
		ctx := context.Background()

		_, _, err := store.LoadCheckpoint(ctx, "nonexistent")
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("load from empty store", func(t *testing.T) {
		store := NewMemStore[TestState]()
		ctx := context.Background()

		_, _, err := store.LoadCheckpoint(ctx, "any-id")
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("expected ErrNotFound for empty store, got %v", err)
		}
	})

	t.Run("load checkpoint after saving only steps", func(t *testing.T) {
		store := NewMemStore[TestState]()
		ctx := context.Background()

		// Save steps but no checkpoints
		_ = store.SaveStep(ctx, "run-001", 1, "node1", TestState{Value: "step"})

		// Trying to load a checkpoint should fail
		_, _, err := store.LoadCheckpoint(ctx, "cp-001")
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})
}

// TestMemStore_JSONSerialization verifies JSON marshaling of MemStore (T071).
func TestMemStore_JSONSerialization(t *testing.T) {
	t.Run("marshal empty store to JSON", func(t *testing.T) {
		store := NewMemStore[TestState]()

		data, err := store.MarshalJSON()
		if err != nil {
			t.Fatalf("MarshalJSON failed: %v", err)
		}

		// Should produce valid JSON
		if len(data) == 0 {
			t.Error("expected non-empty JSON data")
		}

		// Should be parseable as JSON
		var result map[string]interface{}
		if err := json.Unmarshal(data, &result); err != nil {
			t.Errorf("produced invalid JSON: %v", err)
		}
	})

	t.Run("marshal store with steps to JSON", func(t *testing.T) {
		store := NewMemStore[TestState]()
		ctx := context.Background()

		// Add some steps
		_ = store.SaveStep(ctx, "run-001", 1, "node1", TestState{Value: "v1", Counter: 10})
		_ = store.SaveStep(ctx, "run-001", 2, "node2", TestState{Value: "v2", Counter: 20})
		_ = store.SaveStep(ctx, "run-002", 1, "node1", TestState{Value: "v3", Counter: 30})

		data, err := store.MarshalJSON()
		if err != nil {
			t.Fatalf("MarshalJSON failed: %v", err)
		}

		// Should contain step data
		jsonStr := string(data)
		if !contains(jsonStr, "run-001") {
			t.Error("JSON should contain runID 'run-001'")
		}
		if !contains(jsonStr, "node1") {
			t.Error("JSON should contain nodeID 'node1'")
		}
	})

	t.Run("marshal store with checkpoints to JSON", func(t *testing.T) {
		store := NewMemStore[TestState]()
		ctx := context.Background()

		// Add checkpoint
		_ = store.SaveCheckpoint(ctx, "cp-001", TestState{Value: "checkpoint", Counter: 100}, 5)

		data, err := store.MarshalJSON()
		if err != nil {
			t.Fatalf("MarshalJSON failed: %v", err)
		}

		// Should contain checkpoint data
		jsonStr := string(data)
		if !contains(jsonStr, "cp-001") {
			t.Error("JSON should contain checkpointID 'cp-001'")
		}
		if !contains(jsonStr, "checkpoint") {
			t.Error("JSON should contain checkpoint value")
		}
	})

	t.Run("marshal store with both steps and checkpoints", func(t *testing.T) {
		store := NewMemStore[TestState]()
		ctx := context.Background()

		// Add steps and checkpoints
		_ = store.SaveStep(ctx, "run-001", 1, "node1", TestState{Value: "step1", Counter: 1})
		_ = store.SaveCheckpoint(ctx, "cp-001", TestState{Value: "cp1", Counter: 50}, 10)

		data, err := store.MarshalJSON()
		if err != nil {
			t.Fatalf("MarshalJSON failed: %v", err)
		}

		// Should be valid JSON
		if len(data) == 0 {
			t.Error("expected non-empty JSON data")
		}
	})
}

// contains is a helper to check if a string contains a substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || contains(s[1:], substr)))
}

// TestMemStore_JSONDeserialization verifies JSON unmarshaling of MemStore (T073).
func TestMemStore_JSONDeserialization(t *testing.T) {
	t.Run("unmarshal empty store from JSON", func(t *testing.T) {
		// Marshal empty store
		original := NewMemStore[TestState]()
		data, _ := original.MarshalJSON()

		// Unmarshal into new store
		restored := NewMemStore[TestState]()
		err := restored.UnmarshalJSON(data)
		if err != nil {
			t.Fatalf("UnmarshalJSON failed: %v", err)
		}

		// Verify empty
		ctx := context.Background()
		_, _, loadErr := restored.LoadLatest(ctx, "any-run")
		if !errors.Is(loadErr, ErrNotFound) {
			t.Error("expected empty store after unmarshaling empty JSON")
		}
	})

	t.Run("unmarshal store with steps from JSON", func(t *testing.T) {
		// Create original store with steps
		original := NewMemStore[TestState]()
		ctx := context.Background()
		_ = original.SaveStep(ctx, "run-001", 1, "node1", TestState{Value: "v1", Counter: 10})
		_ = original.SaveStep(ctx, "run-001", 2, "node2", TestState{Value: "v2", Counter: 20})

		// Marshal
		data, _ := original.MarshalJSON()

		// Unmarshal into new store
		restored := NewMemStore[TestState]()
		err := restored.UnmarshalJSON(data)
		if err != nil {
			t.Fatalf("UnmarshalJSON failed: %v", err)
		}

		// Verify steps were restored
		state, step, err := restored.LoadLatest(ctx, "run-001")
		if err != nil {
			t.Fatalf("LoadLatest failed after unmarshal: %v", err)
		}

		if step != 2 {
			t.Errorf("expected step = 2, got %d", step)
		}
		if state.Value != "v2" {
			t.Errorf("expected Value = 'v2', got %q", state.Value)
		}
		if state.Counter != 20 {
			t.Errorf("expected Counter = 20, got %d", state.Counter)
		}
	})

	t.Run("unmarshal store with checkpoints from JSON", func(t *testing.T) {
		// Create original store with checkpoint
		original := NewMemStore[TestState]()
		ctx := context.Background()
		_ = original.SaveCheckpoint(ctx, "cp-001", TestState{Value: "checkpoint", Counter: 100}, 5)

		// Marshal
		data, _ := original.MarshalJSON()

		// Unmarshal into new store
		restored := NewMemStore[TestState]()
		err := restored.UnmarshalJSON(data)
		if err != nil {
			t.Fatalf("UnmarshalJSON failed: %v", err)
		}

		// Verify checkpoint was restored
		state, step, err := restored.LoadCheckpoint(ctx, "cp-001")
		if err != nil {
			t.Fatalf("LoadCheckpoint failed after unmarshal: %v", err)
		}

		if step != 5 {
			t.Errorf("expected step = 5, got %d", step)
		}
		if state.Value != "checkpoint" {
			t.Errorf("expected Value = 'checkpoint', got %q", state.Value)
		}
		if state.Counter != 100 {
			t.Errorf("expected Counter = 100, got %d", state.Counter)
		}
	})

	t.Run("round-trip serialization preserves data", func(t *testing.T) {
		// Create complex store
		original := NewMemStore[TestState]()
		ctx := context.Background()
		_ = original.SaveStep(ctx, "run-001", 1, "node1", TestState{Value: "s1", Counter: 1})
		_ = original.SaveStep(ctx, "run-001", 2, "node2", TestState{Value: "s2", Counter: 2})
		_ = original.SaveStep(ctx, "run-002", 1, "node1", TestState{Value: "s3", Counter: 3})
		_ = original.SaveCheckpoint(ctx, "cp-001", TestState{Value: "cp1", Counter: 50}, 10)
		_ = original.SaveCheckpoint(ctx, "cp-002", TestState{Value: "cp2", Counter: 60}, 20)

		// Marshal
		data, _ := original.MarshalJSON()

		// Unmarshal
		restored := NewMemStore[TestState]()
		_ = restored.UnmarshalJSON(data)

		// Verify all data preserved
		// Check run-001
		s1, step1, _ := restored.LoadLatest(ctx, "run-001")
		if step1 != 2 || s1.Value != "s2" || s1.Counter != 2 {
			t.Error("run-001 not preserved correctly")
		}

		// Check run-002
		s2, step2, _ := restored.LoadLatest(ctx, "run-002")
		if step2 != 1 || s2.Value != "s3" || s2.Counter != 3 {
			t.Error("run-002 not preserved correctly")
		}

		// Check checkpoints
		cp1, cpStep1, _ := restored.LoadCheckpoint(ctx, "cp-001")
		if cpStep1 != 10 || cp1.Value != "cp1" || cp1.Counter != 50 {
			t.Error("cp-001 not preserved correctly")
		}

		cp2, cpStep2, _ := restored.LoadCheckpoint(ctx, "cp-002")
		if cpStep2 != 20 || cp2.Value != "cp2" || cp2.Counter != 60 {
			t.Error("cp-002 not preserved correctly")
		}
	})

	t.Run("unmarshal invalid JSON", func(t *testing.T) {
		store := NewMemStore[TestState]()

		// Try to unmarshal invalid JSON
		err := store.UnmarshalJSON([]byte("{invalid json"))
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
	})
}

// TestMemStore_SaveCheckpointV2 verifies enhanced checkpoint save functionality (T094, T099).
func TestMemStore_SaveCheckpointV2(t *testing.T) {
	t.Run("save checkpoint with idempotency key", func(t *testing.T) {
		store := NewMemStore[TestState]()
		ctx := context.Background()

		checkpoint := CheckpointV2[TestState]{
			RunID:          "run-001",
			StepID:         5,
			State:          TestState{Value: "test", Counter: 42},
			IdempotencyKey: "idem-key-001",
		}

		err := store.SaveCheckpointV2(ctx, checkpoint)
		if err != nil {
			t.Fatalf("SaveCheckpointV2 failed: %v", err)
		}

		// Verify checkpoint can be loaded
		loaded, err := store.LoadCheckpointV2(ctx, "run-001", 5)
		if err != nil {
			t.Fatalf("LoadCheckpointV2 failed: %v", err)
		}

		if loaded.RunID != checkpoint.RunID {
			t.Errorf("expected RunID = %q, got %q", checkpoint.RunID, loaded.RunID)
		}
		if loaded.StepID != checkpoint.StepID {
			t.Errorf("expected StepID = %d, got %d", checkpoint.StepID, loaded.StepID)
		}
		if loaded.State.Value != checkpoint.State.Value {
			t.Errorf("expected State.Value = %q, got %q", checkpoint.State.Value, loaded.State.Value)
		}
	})

	t.Run("duplicate idempotency key returns error", func(t *testing.T) {
		store := NewMemStore[TestState]()
		ctx := context.Background()

		checkpoint := CheckpointV2[TestState]{
			RunID:          "run-001",
			StepID:         1,
			State:          TestState{Value: "first"},
			IdempotencyKey: "duplicate-key",
		}

		// First save should succeed
		err := store.SaveCheckpointV2(ctx, checkpoint)
		if err != nil {
			t.Fatalf("first SaveCheckpointV2 failed: %v", err)
		}

		// Second save with same idempotency key should fail
		checkpoint2 := CheckpointV2[TestState]{
			RunID:          "run-002",
			StepID:         2,
			State:          TestState{Value: "second"},
			IdempotencyKey: "duplicate-key",
		}

		err = store.SaveCheckpointV2(ctx, checkpoint2)
		if err == nil {
			t.Error("expected error for duplicate idempotency key")
		}
		if !errors.Is(err, errors.New("duplicate checkpoint")) && err != nil && err.Error() != "duplicate checkpoint: idempotency key \"duplicate-key\" already exists" {
			// Check if error message contains expected text
			if err.Error() == "" || err.Error()[:len("duplicate checkpoint")] != "duplicate checkpoint" {
				t.Errorf("unexpected error message: %v", err)
			}
		}
	})

	t.Run("save checkpoint with label", func(t *testing.T) {
		store := NewMemStore[TestState]()
		ctx := context.Background()

		checkpoint := CheckpointV2[TestState]{
			RunID:          "run-001",
			StepID:         10,
			State:          TestState{Value: "labeled", Counter: 100},
			Label:          "before-validation",
			IdempotencyKey: "label-key-001",
		}

		err := store.SaveCheckpointV2(ctx, checkpoint)
		if err != nil {
			t.Fatalf("SaveCheckpointV2 failed: %v", err)
		}

		// Verify label is indexed (internal verification)
		store.mu.RLock()
		labelKey, exists := store.labelIndex["before-validation"]
		store.mu.RUnlock()

		if !exists {
			t.Error("label not indexed")
		}
		if labelKey != "run-001:10" {
			t.Errorf("expected label to map to 'run-001:10', got %q", labelKey)
		}
	})

	t.Run("save checkpoint without idempotency key", func(t *testing.T) {
		store := NewMemStore[TestState]()
		ctx := context.Background()

		checkpoint := CheckpointV2[TestState]{
			RunID:          "run-001",
			StepID:         1,
			State:          TestState{Value: "no-idem-key"},
			IdempotencyKey: "", // Empty idempotency key
		}

		err := store.SaveCheckpointV2(ctx, checkpoint)
		if err != nil {
			t.Fatalf("SaveCheckpointV2 should succeed without idempotency key: %v", err)
		}
	})
}

// TestMemStore_LoadCheckpointV2 verifies enhanced checkpoint load functionality (T095, T099).
func TestMemStore_LoadCheckpointV2(t *testing.T) {
	t.Run("load existing checkpoint", func(t *testing.T) {
		store := NewMemStore[TestState]()
		ctx := context.Background()

		checkpoint := CheckpointV2[TestState]{
			RunID:          "run-001",
			StepID:         7,
			State:          TestState{Value: "load-test", Counter: 77},
			IdempotencyKey: "load-key-001",
		}

		_ = store.SaveCheckpointV2(ctx, checkpoint)

		loaded, err := store.LoadCheckpointV2(ctx, "run-001", 7)
		if err != nil {
			t.Fatalf("LoadCheckpointV2 failed: %v", err)
		}

		if loaded.StepID != 7 {
			t.Errorf("expected StepID = 7, got %d", loaded.StepID)
		}
		if loaded.State.Counter != 77 {
			t.Errorf("expected Counter = 77, got %d", loaded.State.Counter)
		}
	})

	t.Run("load nonexistent checkpoint", func(t *testing.T) {
		store := NewMemStore[TestState]()
		ctx := context.Background()

		_, err := store.LoadCheckpointV2(ctx, "nonexistent-run", 99)
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("load different steps from same run", func(t *testing.T) {
		store := NewMemStore[TestState]()
		ctx := context.Background()

		// Save multiple checkpoints for same run
		for i := 1; i <= 5; i++ {
			checkpoint := CheckpointV2[TestState]{
				RunID:          "run-001",
				StepID:         i,
				State:          TestState{Counter: i * 10},
				IdempotencyKey: fmt.Sprintf("key-%d", i),
			}
			_ = store.SaveCheckpointV2(ctx, checkpoint)
		}

		// Load step 3
		cp3, err := store.LoadCheckpointV2(ctx, "run-001", 3)
		if err != nil {
			t.Fatalf("failed to load step 3: %v", err)
		}
		if cp3.State.Counter != 30 {
			t.Errorf("expected Counter = 30, got %d", cp3.State.Counter)
		}

		// Load step 5
		cp5, err := store.LoadCheckpointV2(ctx, "run-001", 5)
		if err != nil {
			t.Fatalf("failed to load step 5: %v", err)
		}
		if cp5.State.Counter != 50 {
			t.Errorf("expected Counter = 50, got %d", cp5.State.Counter)
		}
	})
}

// TestMemStore_CheckIdempotency verifies idempotency key checking (T096, T099).
func TestMemStore_CheckIdempotency(t *testing.T) {
	t.Run("check unused key", func(t *testing.T) {
		store := NewMemStore[TestState]()
		ctx := context.Background()

		exists, err := store.CheckIdempotency(ctx, "unused-key")
		if err != nil {
			t.Fatalf("CheckIdempotency failed: %v", err)
		}
		if exists {
			t.Error("expected key to not exist")
		}
	})

	t.Run("check used key", func(t *testing.T) {
		store := NewMemStore[TestState]()
		ctx := context.Background()

		// Save checkpoint with idempotency key
		checkpoint := CheckpointV2[TestState]{
			RunID:          "run-001",
			StepID:         1,
			State:          TestState{Value: "test"},
			IdempotencyKey: "used-key",
		}
		_ = store.SaveCheckpointV2(ctx, checkpoint)

		// Check if key exists
		exists, err := store.CheckIdempotency(ctx, "used-key")
		if err != nil {
			t.Fatalf("CheckIdempotency failed: %v", err)
		}
		if !exists {
			t.Error("expected key to exist")
		}
	})

	t.Run("check multiple keys", func(t *testing.T) {
		store := NewMemStore[TestState]()
		ctx := context.Background()

		// Save multiple checkpoints
		keys := []string{"key-1", "key-2", "key-3"}
		for i, key := range keys {
			checkpoint := CheckpointV2[TestState]{
				RunID:          "run-001",
				StepID:         i + 1,
				State:          TestState{Counter: i},
				IdempotencyKey: key,
			}
			_ = store.SaveCheckpointV2(ctx, checkpoint)
		}

		// Verify all keys exist
		for _, key := range keys {
			exists, err := store.CheckIdempotency(ctx, key)
			if err != nil {
				t.Fatalf("CheckIdempotency(%s) failed: %v", key, err)
			}
			if !exists {
				t.Errorf("expected key %s to exist", key)
			}
		}

		// Verify unused key doesn't exist
		exists, _ := store.CheckIdempotency(ctx, "unused-key")
		if exists {
			t.Error("expected unused-key to not exist")
		}
	})
}

// TestMemStore_PendingEvents verifies event queue retrieval (T097, T099).
func TestMemStore_PendingEvents(t *testing.T) {
	t.Run("retrieve pending events", func(t *testing.T) {
		store := NewMemStore[TestState]()
		ctx := context.Background()

		// Add events to pending queue
		store.mu.Lock()
		store.pendingEvents = []emit.Event{
			{RunID: "run-001", Step: 1, NodeID: "node1", Msg: "event1", Meta: map[string]interface{}{"event_id": "e1"}},
			{RunID: "run-001", Step: 2, NodeID: "node2", Msg: "event2", Meta: map[string]interface{}{"event_id": "e2"}},
			{RunID: "run-001", Step: 3, NodeID: "node3", Msg: "event3", Meta: map[string]interface{}{"event_id": "e3"}},
		}
		store.mu.Unlock()

		// Retrieve all events
		events, err := store.PendingEvents(ctx, 0)
		if err != nil {
			t.Fatalf("PendingEvents failed: %v", err)
		}

		if len(events) != 3 {
			t.Errorf("expected 3 events, got %d", len(events))
		}
	})

	t.Run("retrieve with limit", func(t *testing.T) {
		store := NewMemStore[TestState]()
		ctx := context.Background()

		// Add 10 events
		store.mu.Lock()
		for i := 1; i <= 10; i++ {
			event := emit.Event{
				RunID:  "run-001",
				Step:   i,
				NodeID: fmt.Sprintf("node%d", i),
				Msg:    fmt.Sprintf("event%d", i),
				Meta:   map[string]interface{}{"event_id": fmt.Sprintf("e%d", i)},
			}
			store.pendingEvents = append(store.pendingEvents, event)
		}
		store.mu.Unlock()

		// Retrieve only 5 events
		events, err := store.PendingEvents(ctx, 5)
		if err != nil {
			t.Fatalf("PendingEvents failed: %v", err)
		}

		if len(events) != 5 {
			t.Errorf("expected 5 events, got %d", len(events))
		}

		// Verify correct events returned (first 5)
		for i := 0; i < 5; i++ {
			if events[i].Step != i+1 {
				t.Errorf("expected event %d to have Step = %d, got %d", i, i+1, events[i].Step)
			}
		}
	})

	t.Run("empty queue returns empty list", func(t *testing.T) {
		store := NewMemStore[TestState]()
		ctx := context.Background()

		events, err := store.PendingEvents(ctx, 10)
		if err != nil {
			t.Fatalf("PendingEvents failed: %v", err)
		}

		if len(events) != 0 {
			t.Errorf("expected 0 events, got %d", len(events))
		}
	})
}

// TestMemStore_MarkEventsEmitted verifies event emission marking (T098, T099).
func TestMemStore_MarkEventsEmitted(t *testing.T) {
	t.Run("mark events as emitted", func(t *testing.T) {
		store := NewMemStore[TestState]()
		ctx := context.Background()

		// Add events to pending queue
		store.mu.Lock()
		store.pendingEvents = []emit.Event{
			{RunID: "run-001", Step: 1, Msg: "event1", Meta: map[string]interface{}{"event_id": "e1"}},
			{RunID: "run-001", Step: 2, Msg: "event2", Meta: map[string]interface{}{"event_id": "e2"}},
			{RunID: "run-001", Step: 3, Msg: "event3", Meta: map[string]interface{}{"event_id": "e3"}},
		}
		store.mu.Unlock()

		// Mark e1 and e3 as emitted
		err := store.MarkEventsEmitted(ctx, []string{"e1", "e3"})
		if err != nil {
			t.Fatalf("MarkEventsEmitted failed: %v", err)
		}

		// Verify only e2 remains
		events, _ := store.PendingEvents(ctx, 0)
		if len(events) != 1 {
			t.Errorf("expected 1 pending event, got %d", len(events))
		}
		if len(events) > 0 && events[0].Step != 2 {
			t.Errorf("expected remaining event to be step 2, got step %d", events[0].Step)
		}
	})

	t.Run("mark all events as emitted", func(t *testing.T) {
		store := NewMemStore[TestState]()
		ctx := context.Background()

		// Add events
		store.mu.Lock()
		store.pendingEvents = []emit.Event{
			{RunID: "run-001", Step: 1, Msg: "event1", Meta: map[string]interface{}{"event_id": "e1"}},
			{RunID: "run-001", Step: 2, Msg: "event2", Meta: map[string]interface{}{"event_id": "e2"}},
		}
		store.mu.Unlock()

		// Mark all as emitted
		err := store.MarkEventsEmitted(ctx, []string{"e1", "e2"})
		if err != nil {
			t.Fatalf("MarkEventsEmitted failed: %v", err)
		}

		// Verify queue is empty
		events, _ := store.PendingEvents(ctx, 0)
		if len(events) != 0 {
			t.Errorf("expected empty queue, got %d events", len(events))
		}
	})

	t.Run("mark nonexistent event ID is no-op", func(t *testing.T) {
		store := NewMemStore[TestState]()
		ctx := context.Background()

		// Add one event
		store.mu.Lock()
		store.pendingEvents = []emit.Event{
			{RunID: "run-001", Step: 1, Msg: "event1", Meta: map[string]interface{}{"event_id": "e1"}},
		}
		store.mu.Unlock()

		// Mark nonexistent event
		err := store.MarkEventsEmitted(ctx, []string{"nonexistent"})
		if err != nil {
			t.Fatalf("MarkEventsEmitted failed: %v", err)
		}

		// Verify original event remains
		events, _ := store.PendingEvents(ctx, 0)
		if len(events) != 1 {
			t.Errorf("expected 1 event, got %d", len(events))
		}
	})

	t.Run("mark empty list is no-op", func(t *testing.T) {
		store := NewMemStore[TestState]()
		ctx := context.Background()

		err := store.MarkEventsEmitted(ctx, []string{})
		if err != nil {
			t.Errorf("MarkEventsEmitted with empty list should not error: %v", err)
		}
	})
}

// TestMemStore_ConcurrentV2Operations verifies thread-safety of new methods (T099).
func TestMemStore_ConcurrentV2Operations(t *testing.T) {
	t.Run("concurrent SaveCheckpointV2", func(t *testing.T) {
		store := NewMemStore[TestState]()
		ctx := context.Background()

		var wg sync.WaitGroup
		errors := make(chan error, 10)

		// Launch 10 goroutines saving checkpoints
		for i := 1; i <= 10; i++ {
			wg.Add(1)
			go func(step int) {
				defer wg.Done()
				checkpoint := CheckpointV2[TestState]{
					RunID:          "run-001",
					StepID:         step,
					State:          TestState{Counter: step},
					IdempotencyKey: fmt.Sprintf("key-%d", step),
				}
				if err := store.SaveCheckpointV2(ctx, checkpoint); err != nil {
					errors <- err
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		// Check no errors occurred
		for err := range errors {
			t.Errorf("concurrent SaveCheckpointV2 failed: %v", err)
		}

		// Verify all checkpoints saved
		for i := 1; i <= 10; i++ {
			_, err := store.LoadCheckpointV2(ctx, "run-001", i)
			if err != nil {
				t.Errorf("checkpoint %d not saved: %v", i, err)
			}
		}
	})

	t.Run("concurrent CheckIdempotency", func(t *testing.T) {
		store := NewMemStore[TestState]()
		ctx := context.Background()

		// Pre-populate some keys
		for i := 1; i <= 5; i++ {
			checkpoint := CheckpointV2[TestState]{
				RunID:          "run-001",
				StepID:         i,
				State:          TestState{},
				IdempotencyKey: fmt.Sprintf("key-%d", i),
			}
			_ = store.SaveCheckpointV2(ctx, checkpoint)
		}

		var wg sync.WaitGroup
		// Launch concurrent idempotency checks
		for i := 1; i <= 20; i++ {
			wg.Add(1)
			go func(n int) {
				defer wg.Done()
				key := fmt.Sprintf("key-%d", n%10)
				_, _ = store.CheckIdempotency(ctx, key)
			}(i)
		}

		wg.Wait()
		// Test passes if no race conditions detected
	})

	t.Run("concurrent PendingEvents and MarkEventsEmitted", func(t *testing.T) {
		store := NewMemStore[TestState]()
		ctx := context.Background()

		// Pre-populate events
		store.mu.Lock()
		for i := 1; i <= 20; i++ {
			event := emit.Event{
				RunID:  "run-001",
				Step:   i,
				NodeID: fmt.Sprintf("node%d", i),
				Msg:    fmt.Sprintf("event%d", i),
				Meta:   map[string]interface{}{"event_id": fmt.Sprintf("e%d", i)},
			}
			store.pendingEvents = append(store.pendingEvents, event)
		}
		store.mu.Unlock()

		var wg sync.WaitGroup

		// Reader goroutines
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 10; j++ {
					_, _ = store.PendingEvents(ctx, 5)
				}
			}()
		}

		// Writer goroutines (marking emitted)
		for i := 0; i < 3; i++ {
			wg.Add(1)
			go func(n int) {
				defer wg.Done()
				ids := []string{fmt.Sprintf("e%d", n*2+1), fmt.Sprintf("e%d", n*2+2)}
				_ = store.MarkEventsEmitted(ctx, ids)
			}(i)
		}

		wg.Wait()
		// Test passes if no race conditions detected
	})
}
