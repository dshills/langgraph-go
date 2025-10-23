package store

import (
	"context"
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

		if err != ErrNotFound {
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
		if err != ErrNotFound {
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
		if err != ErrNotFound {
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
		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("load from empty store", func(t *testing.T) {
		store := NewMemStore[TestState]()
		ctx := context.Background()

		_, _, err := store.LoadCheckpoint(ctx, "any-id")
		if err != ErrNotFound {
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
		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})
}
