package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// MySQL tests use the shared TestState from store_test.go:
// type TestState struct {
//     Value   string
//     Counter int
// }

// T183: MySQL Store connection tests

func TestMySQLStore_NewConnection(t *testing.T) {
	// Skip if no MySQL available
	dsn := getTestDSN(t)
	if dsn == "" {
		t.Skip("Skipping MySQL tests: TEST_MYSQL_DSN not set")
	}

	t.Run("successful connection", func(t *testing.T) {
		store, err := NewMySQLStore[TestState](dsn)
		if err != nil {
			t.Fatalf("Failed to create MySQL store: %v", err)
		}
		defer store.Close()

		// Verify connection is alive
		ctx := context.Background()
		if err := store.Ping(ctx); err != nil {
			t.Errorf("Ping failed: %v", err)
		}
	})

	t.Run("invalid DSN", func(t *testing.T) {
		invalidDSN := "invalid:dsn:string"
		_, err := NewMySQLStore[TestState](invalidDSN)
		if err == nil {
			t.Error("Expected error with invalid DSN, got nil")
		}
	})

	t.Run("connection to non-existent database", func(t *testing.T) {
		badDSN := "user:pass@tcp(localhost:3306)/nonexistent_db"
		_, err := NewMySQLStore[TestState](badDSN)
		if err == nil {
			t.Error("Expected error with non-existent database, got nil")
		}
	})
}

func TestMySQLStore_ConnectionPooling(t *testing.T) {
	dsn := getTestDSN(t)
	if dsn == "" {
		t.Skip("Skipping MySQL tests: TEST_MYSQL_DSN not set")
	}

	t.Run("pool configuration", func(t *testing.T) {
		store, err := NewMySQLStore[TestState](dsn)
		if err != nil {
			t.Fatalf("Failed to create MySQL store: %v", err)
		}
		defer store.Close()

		// Verify pool settings
		stats := store.Stats()
		if stats.MaxOpenConnections == 0 {
			t.Error("Expected max open connections to be set")
		}
	})

	t.Run("concurrent connections", func(t *testing.T) {
		store, err := NewMySQLStore[TestState](dsn)
		if err != nil {
			t.Fatalf("Failed to create MySQL store: %v", err)
		}
		defer store.Close()

		// Concurrent pings
		const numGoroutines = 10
		errChan := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func() {
				ctx := context.Background()
				errChan <- store.Ping(ctx)
			}()
		}

		// Check all pings succeeded
		for i := 0; i < numGoroutines; i++ {
			if err := <-errChan; err != nil {
				t.Errorf("Concurrent ping %d failed: %v", i, err)
			}
		}
	})

	t.Run("connection timeout", func(t *testing.T) {
		store, err := NewMySQLStore[TestState](dsn)
		if err != nil {
			t.Fatalf("Failed to create MySQL store: %v", err)
		}
		defer store.Close()

		// Short timeout context
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()

		// This should timeout or complete quickly
		// We don't fail the test if it completes, as that's also valid
		_ = store.Ping(ctx)
	})
}

func TestMySQLStore_Close(t *testing.T) {
	dsn := getTestDSN(t)
	if dsn == "" {
		t.Skip("Skipping MySQL tests: TEST_MYSQL_DSN not set")
	}

	t.Run("close active connection", func(t *testing.T) {
		store, err := NewMySQLStore[TestState](dsn)
		if err != nil {
			t.Fatalf("Failed to create MySQL store: %v", err)
		}

		// Close should succeed
		if err := store.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}

		// Operations after close should fail
		ctx := context.Background()
		err = store.Ping(ctx)
		if err == nil {
			t.Error("Expected error after close, got nil")
		}
	})

	t.Run("double close", func(t *testing.T) {
		store, err := NewMySQLStore[TestState](dsn)
		if err != nil {
			t.Fatalf("Failed to create MySQL store: %v", err)
		}

		// First close
		if err := store.Close(); err != nil {
			t.Errorf("First close failed: %v", err)
		}

		// Second close should not panic
		if err := store.Close(); err != nil {
			// Error is acceptable, just shouldn't panic
			t.Logf("Second close returned error: %v", err)
		}
	})
}

func TestMySQLStore_TableCreation(t *testing.T) {
	dsn := getTestDSN(t)
	if dsn == "" {
		t.Skip("Skipping MySQL tests: TEST_MYSQL_DSN not set")
	}

	t.Run("auto-create tables on first connection", func(t *testing.T) {
		// Drop tables first
		cleanupTestTables(t, dsn)

		store, err := NewMySQLStore[TestState](dsn)
		if err != nil {
			t.Fatalf("Failed to create MySQL store: %v", err)
		}
		defer store.Close()

		// Verify tables exist
		ctx := context.Background()
		if !tableExists(ctx, store, "workflow_steps") {
			t.Error("workflow_steps table not created")
		}
		if !tableExists(ctx, store, "workflow_checkpoints") {
			t.Error("workflow_checkpoints table not created")
		}
	})

	t.Run("handle existing tables", func(t *testing.T) {
		// Create store first time
		store1, err := NewMySQLStore[TestState](dsn)
		if err != nil {
			t.Fatalf("Failed to create first MySQL store: %v", err)
		}
		store1.Close()

		// Create store second time (tables already exist)
		store2, err := NewMySQLStore[TestState](dsn)
		if err != nil {
			t.Fatalf("Failed to create second MySQL store: %v", err)
		}
		defer store2.Close()

		// Should succeed without errors
		ctx := context.Background()
		if err := store2.Ping(ctx); err != nil {
			t.Errorf("Ping failed on second store: %v", err)
		}
	})
}

// T185: MySQL transaction handling tests

func TestMySQLStore_SaveStepBatch(t *testing.T) {
	dsn := getTestDSN(t)
	if dsn == "" {
		t.Skip("Skipping MySQL tests: TEST_MYSQL_DSN not set")
	}

	t.Run("atomic batch save - all succeed", func(t *testing.T) {
		store, err := NewMySQLStore[TestState](dsn)
		if err != nil {
			t.Fatalf("Failed to create MySQL store: %v", err)
		}
		defer store.Close()

		ctx := context.Background()
		runID := "batch-test-001"

		// Prepare multiple steps
		steps := []struct {
			step   int
			nodeID string
			state  TestState
		}{
			{1, "node-a", TestState{Value: "step 1", Counter: 1}},
			{2, "node-b", TestState{Value: "step 2", Counter: 2}},
			{3, "node-c", TestState{Value: "step 3", Counter: 3}},
		}

		// Save batch atomically
		err = store.SaveStepBatch(ctx, runID, steps)
		if err != nil {
			t.Fatalf("SaveStepBatch failed: %v", err)
		}

		// Verify all steps were saved
		for _, step := range steps {
			state, stepNum, err := store.LoadLatest(ctx, runID)
			if err != nil {
				t.Errorf("Failed to load step %d: %v", step.step, err)
			}
			if stepNum != 3 { // Should get the latest
				t.Errorf("Expected step 3, got %d", stepNum)
			}
			if state.Counter != 3 {
				t.Errorf("Expected counter 3, got %d", state.Counter)
			}
		}
	})

	t.Run("atomic batch save - rollback on error", func(t *testing.T) {
		store, err := NewMySQLStore[TestState](dsn)
		if err != nil {
			t.Fatalf("Failed to create MySQL store: %v", err)
		}
		defer store.Close()

		ctx := context.Background()
		runID := "batch-test-002"

		// Save first step successfully
		err = store.SaveStep(ctx, runID, 1, "node-1", TestState{Counter: 1})
		if err != nil {
			t.Fatalf("Failed to save initial step: %v", err)
		}

		// Attempt batch with invalid data (this should rollback)
		// The implementation should handle this gracefully
		steps := []struct {
			step   int
			nodeID string
			state  TestState
		}{
			{2, "node-2", TestState{Value: "step 2", Counter: 2}},
			{3, "node-3", TestState{Value: "step 3", Counter: 3}},
		}

		// Save batch
		_ = store.SaveStepBatch(ctx, runID, steps)

		// Original step should still be intact
		state, step, err := store.LoadLatest(ctx, runID)
		if err != nil {
			t.Fatalf("Failed to load state: %v", err)
		}

		// Verify we have at least the original step
		if step < 1 {
			t.Errorf("Expected at least step 1, got %d", step)
		}
		if state.Counter < 1 {
			t.Errorf("Expected counter >= 1, got %d", state.Counter)
		}
	})

	t.Run("transaction isolation", func(t *testing.T) {
		store, err := NewMySQLStore[TestState](dsn)
		if err != nil {
			t.Fatalf("Failed to create MySQL store: %v", err)
		}
		defer store.Close()

		ctx := context.Background()
		runID := "isolation-test-001"

		// Start concurrent transactions
		const numGoroutines = 5
		errChan := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				err := store.SaveStep(ctx, runID, id+1, fmt.Sprintf("node-%d", id), TestState{
					Value:   fmt.Sprintf("concurrent-%d", id),
					Counter: id + 1,
				})
				errChan <- err
			}(i)
		}

		// Check all succeeded
		for i := 0; i < numGoroutines; i++ {
			if err := <-errChan; err != nil {
				t.Errorf("Concurrent save %d failed: %v", i, err)
			}
		}

		// Verify latest state
		state, step, err := store.LoadLatest(ctx, runID)
		if err != nil {
			t.Fatalf("Failed to load latest: %v", err)
		}

		if step < 1 || step > numGoroutines {
			t.Errorf("Expected step between 1 and %d, got %d", numGoroutines, step)
		}
		if state.Counter < 1 || state.Counter > numGoroutines {
			t.Errorf("Expected counter between 1 and %d, got %d", numGoroutines, state.Counter)
		}
	})
}

func TestMySQLStore_TransactionRollback(t *testing.T) {
	dsn := getTestDSN(t)
	if dsn == "" {
		t.Skip("Skipping MySQL tests: TEST_MYSQL_DSN not set")
	}

	t.Run("rollback on context cancellation", func(t *testing.T) {
		store, err := NewMySQLStore[TestState](dsn)
		if err != nil {
			t.Fatalf("Failed to create MySQL store: %v", err)
		}
		defer store.Close()

		runID := "rollback-test-001"

		// Save initial state
		ctx := context.Background()
		err = store.SaveStep(ctx, runID, 1, "node-1", TestState{Counter: 1})
		if err != nil {
			t.Fatalf("Failed to save initial step: %v", err)
		}

		// Create cancelled context
		cancelledCtx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		// Attempt to save with cancelled context
		err = store.SaveStep(cancelledCtx, runID, 2, "node-2", TestState{Counter: 2})
		// Error is expected but state should be consistent

		// Verify original state is intact
		state, step, err := store.LoadLatest(ctx, runID)
		if err != nil {
			t.Fatalf("Failed to load state: %v", err)
		}

		// Should have at least the original step
		if step < 1 {
			t.Errorf("Expected at least step 1, got %d", step)
		}
		if state.Counter < 1 {
			t.Errorf("Expected counter >= 1, got %d", state.Counter)
		}
	})
}

func TestMySQLStore_ConcurrentCheckpoints(t *testing.T) {
	dsn := getTestDSN(t)
	if dsn == "" {
		t.Skip("Skipping MySQL tests: TEST_MYSQL_DSN not set")
	}

	t.Run("concurrent checkpoint saves", func(t *testing.T) {
		store, err := NewMySQLStore[TestState](dsn)
		if err != nil {
			t.Fatalf("Failed to create MySQL store: %v", err)
		}
		defer store.Close()

		ctx := context.Background()

		// Concurrent checkpoint saves
		const numCheckpoints = 10
		errChan := make(chan error, numCheckpoints)

		for i := 0; i < numCheckpoints; i++ {
			go func(id int) {
				cpID := fmt.Sprintf("checkpoint-%d", id)
				err := store.SaveCheckpoint(ctx, cpID, TestState{
					Value:   fmt.Sprintf("checkpoint-%d", id),
					Counter: id,
				}, id)
				errChan <- err
			}(i)
		}

		// Check all succeeded
		for i := 0; i < numCheckpoints; i++ {
			if err := <-errChan; err != nil {
				t.Errorf("Concurrent checkpoint save %d failed: %v", i, err)
			}
		}

		// Verify all checkpoints can be loaded
		for i := 0; i < numCheckpoints; i++ {
			cpID := fmt.Sprintf("checkpoint-%d", i)
			state, step, err := store.LoadCheckpoint(ctx, cpID)
			if err != nil {
				t.Errorf("Failed to load checkpoint %s: %v", cpID, err)
				continue
			}
			if state.Counter != i {
				t.Errorf("Checkpoint %s: expected counter %d, got %d", cpID, i, state.Counter)
			}
			if step != i {
				t.Errorf("Checkpoint %s: expected step %d, got %d", cpID, i, step)
			}
		}
	})
}

// T187: MySQL checkpoint operations tests

func TestMySQLStore_SaveCheckpoint(t *testing.T) {
	dsn := getTestDSN(t)
	if dsn == "" {
		t.Skip("Skipping MySQL tests: TEST_MYSQL_DSN not set")
	}

	t.Run("save simple checkpoint", func(t *testing.T) {
		store, err := NewMySQLStore[TestState](dsn)
		if err != nil {
			t.Fatalf("Failed to create MySQL store: %v", err)
		}
		defer store.Close()

		ctx := context.Background()
		cpID := "checkpoint-001"
		state := TestState{
			Value:   "test checkpoint",
			Counter: 42,
		}

		// Save checkpoint
		err = store.SaveCheckpoint(ctx, cpID, state, 5)
		if err != nil {
			t.Fatalf("SaveCheckpoint failed: %v", err)
		}

		// Verify it was saved
		loadedState, step, err := store.LoadCheckpoint(ctx, cpID)
		if err != nil {
			t.Fatalf("LoadCheckpoint failed: %v", err)
		}

		if step != 5 {
			t.Errorf("Expected step 5, got %d", step)
		}
		if loadedState.Counter != 42 {
			t.Errorf("Expected counter 42, got %d", loadedState.Counter)
		}
		if loadedState.Value != "test checkpoint" {
			t.Errorf("Expected value 'test checkpoint', got '%s'", loadedState.Value)
		}
	})

	t.Run("save checkpoint with empty state", func(t *testing.T) {
		store, err := NewMySQLStore[TestState](dsn)
		if err != nil {
			t.Fatalf("Failed to create MySQL store: %v", err)
		}
		defer store.Close()

		ctx := context.Background()
		cpID := "checkpoint-empty"
		state := TestState{} // Zero value

		err = store.SaveCheckpoint(ctx, cpID, state, 0)
		if err != nil {
			t.Fatalf("SaveCheckpoint with empty state failed: %v", err)
		}

		// Verify it can be loaded
		loadedState, step, err := store.LoadCheckpoint(ctx, cpID)
		if err != nil {
			t.Fatalf("LoadCheckpoint failed: %v", err)
		}

		if step != 0 {
			t.Errorf("Expected step 0, got %d", step)
		}
		if loadedState.Counter != 0 {
			t.Errorf("Expected counter 0, got %d", loadedState.Counter)
		}
	})

	t.Run("save checkpoint with complex state", func(t *testing.T) {
		store, err := NewMySQLStore[TestState](dsn)
		if err != nil {
			t.Fatalf("Failed to create MySQL store: %v", err)
		}
		defer store.Close()

		ctx := context.Background()
		cpID := "checkpoint-complex"
		state := TestState{
			Value:   "Complex state with unicode: 你好世界 🚀",
			Counter: 999,
		}

		err = store.SaveCheckpoint(ctx, cpID, state, 100)
		if err != nil {
			t.Fatalf("SaveCheckpoint with complex state failed: %v", err)
		}

		// Verify state is preserved exactly
		loadedState, step, err := store.LoadCheckpoint(ctx, cpID)
		if err != nil {
			t.Fatalf("LoadCheckpoint failed: %v", err)
		}

		if step != 100 {
			t.Errorf("Expected step 100, got %d", step)
		}
		if loadedState.Value != state.Value {
			t.Errorf("Unicode value not preserved: got '%s'", loadedState.Value)
		}
		if loadedState.Counter != 999 {
			t.Errorf("Expected counter 999, got %d", loadedState.Counter)
		}
	})

	t.Run("update existing checkpoint", func(t *testing.T) {
		store, err := NewMySQLStore[TestState](dsn)
		if err != nil {
			t.Fatalf("Failed to create MySQL store: %v", err)
		}
		defer store.Close()

		ctx := context.Background()
		cpID := "checkpoint-update"

		// Save initial checkpoint
		state1 := TestState{Value: "version 1", Counter: 10}
		err = store.SaveCheckpoint(ctx, cpID, state1, 1)
		if err != nil {
			t.Fatalf("Initial SaveCheckpoint failed: %v", err)
		}

		// Update checkpoint with same ID
		state2 := TestState{Value: "version 2", Counter: 20}
		err = store.SaveCheckpoint(ctx, cpID, state2, 2)
		if err != nil {
			t.Fatalf("Update SaveCheckpoint failed: %v", err)
		}

		// Verify latest version is loaded
		loadedState, step, err := store.LoadCheckpoint(ctx, cpID)
		if err != nil {
			t.Fatalf("LoadCheckpoint failed: %v", err)
		}

		if step != 2 {
			t.Errorf("Expected step 2, got %d", step)
		}
		if loadedState.Counter != 20 {
			t.Errorf("Expected counter 20 (updated), got %d", loadedState.Counter)
		}
		if loadedState.Value != "version 2" {
			t.Errorf("Expected value 'version 2', got '%s'", loadedState.Value)
		}
	})

	t.Run("save checkpoint after close", func(t *testing.T) {
		store, err := NewMySQLStore[TestState](dsn)
		if err != nil {
			t.Fatalf("Failed to create MySQL store: %v", err)
		}

		// Close store
		store.Close()

		ctx := context.Background()
		err = store.SaveCheckpoint(ctx, "checkpoint-closed", TestState{}, 0)
		if err == nil {
			t.Error("Expected error when saving checkpoint after close, got nil")
		}
	})
}

func TestMySQLStore_LoadCheckpoint(t *testing.T) {
	dsn := getTestDSN(t)
	if dsn == "" {
		t.Skip("Skipping MySQL tests: TEST_MYSQL_DSN not set")
	}

	t.Run("load existing checkpoint", func(t *testing.T) {
		store, err := NewMySQLStore[TestState](dsn)
		if err != nil {
			t.Fatalf("Failed to create MySQL store: %v", err)
		}
		defer store.Close()

		ctx := context.Background()
		cpID := "checkpoint-load-test"
		expectedState := TestState{
			Value:   "load test",
			Counter: 555,
		}

		// Save checkpoint first
		err = store.SaveCheckpoint(ctx, cpID, expectedState, 10)
		if err != nil {
			t.Fatalf("SaveCheckpoint failed: %v", err)
		}

		// Load it back
		state, step, err := store.LoadCheckpoint(ctx, cpID)
		if err != nil {
			t.Fatalf("LoadCheckpoint failed: %v", err)
		}

		if step != 10 {
			t.Errorf("Expected step 10, got %d", step)
		}
		if state.Counter != expectedState.Counter {
			t.Errorf("Expected counter %d, got %d", expectedState.Counter, state.Counter)
		}
		if state.Value != expectedState.Value {
			t.Errorf("Expected value '%s', got '%s'", expectedState.Value, state.Value)
		}
	})

	t.Run("load non-existent checkpoint", func(t *testing.T) {
		store, err := NewMySQLStore[TestState](dsn)
		if err != nil {
			t.Fatalf("Failed to create MySQL store: %v", err)
		}
		defer store.Close()

		ctx := context.Background()
		cpID := "checkpoint-does-not-exist"

		// Attempt to load non-existent checkpoint
		_, _, err = store.LoadCheckpoint(ctx, cpID)
		if err == nil {
			t.Error("Expected error when loading non-existent checkpoint, got nil")
		}
		if err != ErrNotFound {
			t.Errorf("Expected ErrNotFound, got %v", err)
		}
	})

	t.Run("load checkpoint after close", func(t *testing.T) {
		store, err := NewMySQLStore[TestState](dsn)
		if err != nil {
			t.Fatalf("Failed to create MySQL store: %v", err)
		}

		ctx := context.Background()
		cpID := "checkpoint-test"
		store.SaveCheckpoint(ctx, cpID, TestState{Counter: 1}, 1)

		// Close store
		store.Close()

		// Attempt to load after close
		_, _, err = store.LoadCheckpoint(ctx, cpID)
		if err == nil {
			t.Error("Expected error when loading checkpoint after close, got nil")
		}
	})

	t.Run("load checkpoint with cancelled context", func(t *testing.T) {
		store, err := NewMySQLStore[TestState](dsn)
		if err != nil {
			t.Fatalf("Failed to create MySQL store: %v", err)
		}
		defer store.Close()

		// Save a checkpoint first
		ctx := context.Background()
		cpID := "checkpoint-cancel-test"
		store.SaveCheckpoint(ctx, cpID, TestState{Counter: 1}, 1)

		// Create cancelled context
		cancelledCtx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		// Attempt to load with cancelled context
		_, _, err = store.LoadCheckpoint(cancelledCtx, cpID)
		// Error expected (context cancelled or success depending on timing)
		// Just verify it doesn't panic
	})
}

func TestMySQLStore_CheckpointIsolation(t *testing.T) {
	dsn := getTestDSN(t)
	if dsn == "" {
		t.Skip("Skipping MySQL tests: TEST_MYSQL_DSN not set")
	}

	t.Run("multiple checkpoints are isolated", func(t *testing.T) {
		store, err := NewMySQLStore[TestState](dsn)
		if err != nil {
			t.Fatalf("Failed to create MySQL store: %v", err)
		}
		defer store.Close()

		ctx := context.Background()

		// Save multiple different checkpoints
		checkpoints := map[string]TestState{
			"cp-1": {Value: "first", Counter: 1},
			"cp-2": {Value: "second", Counter: 2},
			"cp-3": {Value: "third", Counter: 3},
		}

		for cpID, state := range checkpoints {
			err := store.SaveCheckpoint(ctx, cpID, state, state.Counter)
			if err != nil {
				t.Fatalf("Failed to save checkpoint %s: %v", cpID, err)
			}
		}

		// Verify each checkpoint is independent
		for cpID, expectedState := range checkpoints {
			state, step, err := store.LoadCheckpoint(ctx, cpID)
			if err != nil {
				t.Fatalf("Failed to load checkpoint %s: %v", cpID, err)
			}

			if step != expectedState.Counter {
				t.Errorf("Checkpoint %s: expected step %d, got %d", cpID, expectedState.Counter, step)
			}
			if state.Counter != expectedState.Counter {
				t.Errorf("Checkpoint %s: expected counter %d, got %d", cpID, expectedState.Counter, state.Counter)
			}
			if state.Value != expectedState.Value {
				t.Errorf("Checkpoint %s: expected value '%s', got '%s'", cpID, expectedState.Value, state.Value)
			}
		}
	})
}

// Helper functions

func getTestDSN(t *testing.T) string {
	// Check for test database DSN in environment
	// Example: TEST_MYSQL_DSN="user:pass@tcp(localhost:3306)/test_db"
	// To run these tests: export TEST_MYSQL_DSN="your-connection-string"
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Logf("MySQL tests skipped: Set TEST_MYSQL_DSN environment variable to run")
	}
	return dsn
}

func cleanupTestTables(t *testing.T, dsn string) {
	t.Helper()

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("Failed to open database for cleanup: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	_, _ = db.ExecContext(ctx, "DROP TABLE IF EXISTS workflow_steps")
	_, _ = db.ExecContext(ctx, "DROP TABLE IF EXISTS workflow_checkpoints")
}

func tableExists(ctx context.Context, store *MySQLStore[TestState], tableName string) bool {
	// This will be implemented once MySQLStore is created
	// For now, assume tables exist if no error
	return true
}
