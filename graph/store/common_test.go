package store_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dshills/langgraph-go/graph/store"
)

// TestIdempotencyAcrossStores (T087) verifies that idempotency enforcement works
// consistently across all Store implementations: MemStore, MySQLStore, SQLiteStore.
//
// According to spec.md FR-009: All Store implementations MUST enforce idempotency
// to prevent duplicate checkpoint commits.
//
// Requirements:
// - MemStore enforces idempotency
// - MySQLStore enforces idempotency
// - SQLiteStore enforces idempotency
// - All stores reject duplicate idempotency keys
// - All stores allow progression with unique keys
//
// This test ensures that the idempotency contract is consistently implemented
// across all persistence backends, providing exactly-once guarantees regardless
// of which store is used in production.
func TestIdempotencyAcrossStores(t *testing.T) {
	type TestState struct {
		Counter int    `json:"counter"`
		Message string `json:"message"`
	}

	// Test data
	runID := "idempotency-test-" + time.Now().Format("20060102-150405")
	state1 := TestState{Counter: 1, Message: "first"}
	state2 := TestState{Counter: 2, Message: "second"}

	// Compute idempotency keys
	key1 := "sha256:abc123def456ghi789" // Simulated key for step 1
	key2 := "sha256:jkl012mno345pqr678" // Simulated key for step 2 (different)

	checkpoint1 := store.CheckpointV2[TestState]{
		RunID:          runID,
		StepID:         1,
		State:          state1,
		Frontier:       []interface{}{},
		RNGSeed:        12345,
		RecordedIOs:    []interface{}{},
		IdempotencyKey: key1,
		Timestamp:      time.Now(),
	}

	checkpoint2 := store.CheckpointV2[TestState]{
		RunID:          runID,
		StepID:         2,
		State:          state2,
		Frontier:       []interface{}{},
		RNGSeed:        67890,
		RecordedIOs:    []interface{}{},
		IdempotencyKey: key2,
		Timestamp:      time.Now(),
	}

	checkpoint1Duplicate := store.CheckpointV2[TestState]{
		RunID:          runID,
		StepID:         3, // Different step
		State:          TestState{Counter: 999, Message: "duplicate"},
		Frontier:       []interface{}{},
		RNGSeed:        99999,
		RecordedIOs:    []interface{}{},
		IdempotencyKey: key1, // DUPLICATE KEY
		Timestamp:      time.Now(),
	}

	// Test scenarios for each store
	testScenarios := []struct {
		name      string
		storeFunc func(*testing.T) (store.Store[TestState], func())
	}{
		{
			name: "MemStore",
			storeFunc: func(t *testing.T) (store.Store[TestState], func()) {
				st := store.NewMemStore[TestState]()
				return st, func() { /* no cleanup needed */ }
			},
		},
		{
			name: "SQLiteStore",
			storeFunc: func(t *testing.T) (store.Store[TestState], func()) {
				// Create temporary database file
				tmpDir := t.TempDir()
				dbPath := filepath.Join(tmpDir, "test.db")

				st, err := store.NewSQLiteStore[TestState](dbPath)
				if err != nil {
					t.Fatalf("Failed to create SQLiteStore: %v", err)
				}

				return st, func() {
					st.Close()
				}
			},
		},
		{
			name: "MySQLStore",
			storeFunc: func(t *testing.T) (store.Store[TestState], func()) {
				dsn := os.Getenv("TEST_MYSQL_DSN")
				if dsn == "" {
					t.Skip("Skipping MySQL test: TEST_MYSQL_DSN not set")
				}

				st, err := store.NewMySQLStore[TestState](dsn)
				if err != nil {
					t.Fatalf("Failed to create MySQLStore: %v", err)
				}

				return st, func() {
					st.Close()
				}
			},
		},
	}

	for _, scenario := range testScenarios {
		t.Run(scenario.name, func(t *testing.T) {
			ctx := context.Background()
			st, cleanup := scenario.storeFunc(t)
			defer cleanup()

			// Test 1: First checkpoint should succeed
			err := st.SaveCheckpointV2(ctx, checkpoint1)
			if err != nil {
				t.Fatalf("First checkpoint save failed: %v", err)
			}

			// Verify idempotency key was recorded
			exists, err := st.CheckIdempotency(ctx, key1)
			if err != nil {
				t.Fatalf("CheckIdempotency failed: %v", err)
			}
			if !exists {
				t.Error("Idempotency key was not recorded after save")
			}

			// Test 2: Duplicate key should be rejected
			err = st.SaveCheckpointV2(ctx, checkpoint1Duplicate)
			if err == nil {
				t.Fatal("Duplicate idempotency key was not rejected")
			}
			t.Logf("✓ Duplicate key correctly rejected with error: %v", err)

			// Test 3: Verify duplicate checkpoint was NOT saved
			_, err = st.LoadCheckpointV2(ctx, runID, 3)
			if !errors.Is(err, store.ErrNotFound) {
				t.Errorf("Duplicate checkpoint should not exist, got error: %v", err)
			}

			// Test 4: Verify first checkpoint still exists unchanged
			loaded, err := st.LoadCheckpointV2(ctx, runID, 1)
			if err != nil {
				t.Fatalf("Failed to load first checkpoint: %v", err)
			}
			if loaded.State.Counter != state1.Counter {
				t.Errorf("First checkpoint was modified: got Counter=%d, want=%d",
					loaded.State.Counter, state1.Counter)
			}

			// Test 5: Second checkpoint with different key should succeed
			err = st.SaveCheckpointV2(ctx, checkpoint2)
			if err != nil {
				t.Fatalf("Second checkpoint with different key failed: %v", err)
			}

			// Verify second key was recorded
			exists, err = st.CheckIdempotency(ctx, key2)
			if err != nil {
				t.Fatalf("CheckIdempotency for key2 failed: %v", err)
			}
			if !exists {
				t.Error("Second idempotency key was not recorded")
			}

			// Test 6: Verify both checkpoints exist
			loaded1, err := st.LoadCheckpointV2(ctx, runID, 1)
			if err != nil {
				t.Fatalf("Failed to load checkpoint 1: %v", err)
			}
			if loaded1.State.Counter != state1.Counter {
				t.Errorf("Checkpoint 1 state mismatch: got=%d, want=%d",
					loaded1.State.Counter, state1.Counter)
			}

			loaded2, err := st.LoadCheckpointV2(ctx, runID, 2)
			if err != nil {
				t.Fatalf("Failed to load checkpoint 2: %v", err)
			}
			if loaded2.State.Counter != state2.Counter {
				t.Errorf("Checkpoint 2 state mismatch: got=%d, want=%d",
					loaded2.State.Counter, state2.Counter)
			}

			// Test 7: Verify both keys still exist
			for _, key := range []string{key1, key2} {
				exists, err := st.CheckIdempotency(ctx, key)
				if err != nil {
					t.Errorf("CheckIdempotency for key %s failed: %v", key, err)
				}
				if !exists {
					t.Errorf("Idempotency key %s missing after all operations", key)
				}
			}

			t.Logf("✓ %s idempotency enforcement validated", scenario.name)
		})
	}
}

// TestStoreContractConsistency verifies that all Store implementations behave
// consistently for core operations.
func TestStoreContractConsistency(t *testing.T) {
	type SimpleState struct {
		Value int `json:"value"`
	}

	testScenarios := []struct {
		name      string
		storeFunc func(*testing.T) (store.Store[SimpleState], func())
	}{
		{
			name: "MemStore",
			storeFunc: func(t *testing.T) (store.Store[SimpleState], func()) {
				return store.NewMemStore[SimpleState](), func() {}
			},
		},
		{
			name: "SQLiteStore",
			storeFunc: func(t *testing.T) (store.Store[SimpleState], func()) {
				tmpDir := t.TempDir()
				dbPath := filepath.Join(tmpDir, "test.db")
				st, err := store.NewSQLiteStore[SimpleState](dbPath)
				if err != nil {
					t.Fatalf("Failed to create SQLiteStore: %v", err)
				}
				return st, func() {
					st.Close()
				}
			},
		},
		{
			name: "MySQLStore",
			storeFunc: func(t *testing.T) (store.Store[SimpleState], func()) {
				dsn := os.Getenv("TEST_MYSQL_DSN")
				if dsn == "" {
					t.Skip("Skipping MySQL test: TEST_MYSQL_DSN not set")
				}
				st, err := store.NewMySQLStore[SimpleState](dsn)
				if err != nil {
					t.Fatalf("Failed to create MySQLStore: %v", err)
				}
				return st, func() {
					st.Close()
				}
			},
		},
	}

	for _, scenario := range testScenarios {
		t.Run(scenario.name+"/SaveLoadCheckpointV2", func(t *testing.T) {
			ctx := context.Background()
			st, cleanup := scenario.storeFunc(t)
			defer cleanup()

			runID := "consistency-test-" + scenario.name
			checkpoint := store.CheckpointV2[SimpleState]{
				RunID:          runID,
				StepID:         1,
				State:          SimpleState{Value: 42},
				Frontier:       []interface{}{},
				RNGSeed:        123,
				RecordedIOs:    []interface{}{},
				IdempotencyKey: "sha256:test123",
				Timestamp:      time.Now(),
			}

			// Save checkpoint
			err := st.SaveCheckpointV2(ctx, checkpoint)
			if err != nil {
				t.Fatalf("SaveCheckpointV2 failed: %v", err)
			}

			// Load checkpoint
			loaded, err := st.LoadCheckpointV2(ctx, runID, 1)
			if err != nil {
				t.Fatalf("LoadCheckpointV2 failed: %v", err)
			}

			// Verify fields
			if loaded.RunID != checkpoint.RunID {
				t.Errorf("RunID mismatch: got=%s, want=%s", loaded.RunID, checkpoint.RunID)
			}
			if loaded.StepID != checkpoint.StepID {
				t.Errorf("StepID mismatch: got=%d, want=%d", loaded.StepID, checkpoint.StepID)
			}
			if loaded.State.Value != checkpoint.State.Value {
				t.Errorf("State.Value mismatch: got=%d, want=%d", loaded.State.Value, checkpoint.State.Value)
			}
			if loaded.RNGSeed != checkpoint.RNGSeed {
				t.Errorf("RNGSeed mismatch: got=%d, want=%d", loaded.RNGSeed, checkpoint.RNGSeed)
			}
			if loaded.IdempotencyKey != checkpoint.IdempotencyKey {
				t.Errorf("IdempotencyKey mismatch: got=%s, want=%s", loaded.IdempotencyKey, checkpoint.IdempotencyKey)
			}
		})

		t.Run(scenario.name+"/LoadNonexistentCheckpoint", func(t *testing.T) {
			ctx := context.Background()
			st, cleanup := scenario.storeFunc(t)
			defer cleanup()

			// Load nonexistent checkpoint
			_, err := st.LoadCheckpointV2(ctx, "nonexistent-run", 999)
			if !errors.Is(err, store.ErrNotFound) {
				t.Errorf("Expected ErrNotFound, got: %v", err)
			}
		})
	}
}
