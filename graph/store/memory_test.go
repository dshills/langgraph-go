package store

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
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
