// Package store provides persistence implementations for graph state.
package store

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

// T191: MySQL Integration Test with Real Database.
//
// This test validates the MySQLStore implementation against a real MySQL database.
// It tests the complete workflow persistence and resumption scenario.
//
// Prerequisites:
// - MySQL server running (local, Docker, or cloud).
// - TEST_MYSQL_DSN environment variable set with connection string.
// - Database user has CREATE, INSERT, SELECT, UPDATE, DELETE permissions.
//
// Example DSN: "user:password@tcp(localhost:3306)/test_db?parseTime=true".
//
// To run this test:
// export TEST_MYSQL_DSN="user:password@tcp(localhost:3306)/test_db?parseTime=true".
// go test -v -run TestMySQLIntegration ./graph/store.

// WorkflowState represents a realistic workflow state for testing.
type WorkflowState struct {
	WorkflowID string
	Steps      int
	Status     string
	Data       map[string]interface{}
	Timestamp  time.Time
}

func TestMySQLIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("Skipping MySQL integration test: Set TEST_MYSQL_DSN environment variable to run")
	}

	t.Run("complete workflow lifecycle with checkpoints", func(t *testing.T) {
		ctx := context.Background()

		// Create store.
		store, err := NewMySQLStore[WorkflowState](dsn)
		if err != nil {
			t.Fatalf("Failed to create MySQLStore: %v", err)
		}
		defer func() { _ = store.Close() }()

		// Test scenario: 5-node workflow that crashes after node 3,
		// then resumes from checkpoint to complete.

		// Phase 1: Execute nodes 1-3, save checkpoint.
		runID := fmt.Sprintf("integration-test-%d", time.Now().UnixNano())

		// Node 1.
		state1 := WorkflowState{
			WorkflowID: runID,
			Steps:      1,
			Status:     "processing",
			Data:       map[string]interface{}{"node": "start"},
			Timestamp:  time.Now(),
		}
		err = store.SaveStep(ctx, runID, 1, "node1", state1)
		if err != nil {
			t.Fatalf("Failed to save step 1: %v", err)
		}

		// Node 2.
		state2 := state1
		state2.Steps = 2
		state2.Data = map[string]interface{}{"node": "process", "count": 42}
		state2.Timestamp = time.Now()
		err = store.SaveStep(ctx, runID, 2, "node2", state2)
		if err != nil {
			t.Fatalf("Failed to save step 2: %v", err)
		}

		// Node 3 - save checkpoint before crash.
		state3 := state2
		state3.Steps = 3
		state3.Data = map[string]interface{}{"node": "transform", "count": 42, "transformed": true}
		state3.Timestamp = time.Now()
		err = store.SaveStep(ctx, runID, 3, "node3", state3)
		if err != nil {
			t.Fatalf("Failed to save step 3: %v", err)
		}

		// Save checkpoint at node 3.
		checkpointID := fmt.Sprintf("%s-before-crash", runID)
		err = store.SaveCheckpoint(ctx, checkpointID, state3, 3)
		if err != nil {
			t.Fatalf("Failed to save checkpoint: %v", err)
		}

		// Verify we can load the latest state.
		loadedState, loadedStep, err := store.LoadLatest(ctx, runID)
		if err != nil {
			t.Fatalf("Failed to load latest state: %v", err)
		}
		if loadedStep != 3 {
			t.Errorf("LoadLatest step = %d, want 3", loadedStep)
		}
		if loadedState.Steps != 3 {
			t.Errorf("LoadLatest state.Steps = %d, want 3", loadedState.Steps)
		}

		// Simulate crash - close store.
		store.Close()

		// Phase 2: Resume from checkpoint.
		t.Log("Simulating process restart...")

		// Create new store instance (simulating process restart).
		store2, err := NewMySQLStore[WorkflowState](dsn)
		if err != nil {
			t.Fatalf("Failed to create MySQLStore after restart: %v", err)
		}
		defer func() { _ = store2.Close() }()

		// Load from checkpoint.
		checkpointState, checkpointStep, err := store2.LoadCheckpoint(ctx, checkpointID)
		if err != nil {
			t.Fatalf("Failed to load checkpoint: %v", err)
		}

		// Verify checkpoint data.
		if checkpointStep != 3 {
			t.Errorf("Checkpoint step = %d, want 3", checkpointStep)
		}
		if checkpointState.Steps != 3 {
			t.Errorf("Checkpoint state.Steps = %d, want 3", checkpointState.Steps)
		}
		if checkpointState.Status != "processing" {
			t.Errorf("Checkpoint state.Status = %q, want %q", checkpointState.Status, "processing")
		}

		// Verify data integrity.
		if transformed, ok := checkpointState.Data["transformed"].(bool); !ok || !transformed {
			t.Error("Checkpoint state.Data missing 'transformed' field or incorrect value")
		}
		if count, ok := checkpointState.Data["count"].(float64); !ok || count != 42 {
			t.Errorf("Checkpoint state.Data['count'] = %v, want 42", checkpointState.Data["count"])
		}

		// Resume execution: Node 4.
		state4 := checkpointState
		state4.Steps = 4
		state4.Data = map[string]interface{}{
			"node":        "validate",
			"count":       42,
			"transformed": true,
			"validated":   true,
		}
		state4.Timestamp = time.Now()
		err = store2.SaveStep(ctx, runID, 4, "node4", state4)
		if err != nil {
			t.Fatalf("Failed to save step 4: %v", err)
		}

		// Node 5 - complete.
		state5 := state4
		state5.Steps = 5
		state5.Status = "completed"
		state5.Data = map[string]interface{}{
			"node":        "complete",
			"count":       42,
			"transformed": true,
			"validated":   true,
			"result":      "success",
		}
		state5.Timestamp = time.Now()
		err = store2.SaveStep(ctx, runID, 5, "node5", state5)
		if err != nil {
			t.Fatalf("Failed to save step 5: %v", err)
		}

		// Verify final state.
		finalState, finalStep, err := store2.LoadLatest(ctx, runID)
		if err != nil {
			t.Fatalf("Failed to load final state: %v", err)
		}

		if finalStep != 5 {
			t.Errorf("Final step = %d, want 5", finalStep)
		}
		if finalState.Status != "completed" {
			t.Errorf("Final state.Status = %q, want %q", finalState.Status, "completed")
		}
		if finalState.Steps != 5 {
			t.Errorf("Final state.Steps = %d, want 5", finalState.Steps)
		}

		// Verify result data.
		if result, ok := finalState.Data["result"].(string); !ok || result != "success" {
			t.Errorf("Final state.Data['result'] = %v, want %q", finalState.Data["result"], "success")
		}

		t.Log("✅ Integration test passed: 5-node workflow survived crash and resumed from checkpoint")
	})

	t.Run("concurrent workflow execution", func(t *testing.T) {
		ctx := context.Background()

		store, err := NewMySQLStore[WorkflowState](dsn)
		if err != nil {
			t.Fatalf("Failed to create MySQLStore: %v", err)
		}
		defer func() { _ = store.Close() }()

		// Test 3 concurrent workflows.
		workflows := []string{"workflow-A", "workflow-B", "workflow-C"}
		done := make(chan error, len(workflows))

		for _, wfID := range workflows {
			go func(workflowID string) {
				// Execute 3 steps.
				for step := 1; step <= 3; step++ {
					state := WorkflowState{
						WorkflowID: workflowID,
						Steps:      step,
						Status:     "running",
						Data:       map[string]interface{}{"step": step},
						Timestamp:  time.Now(),
					}
					err := store.SaveStep(ctx, workflowID, step, fmt.Sprintf("node%d", step), state)
					if err != nil {
						done <- fmt.Errorf("workflow %s step %d failed: %w", workflowID, step, err)
						return
					}
					time.Sleep(10 * time.Millisecond) // Simulate work
				}
				done <- nil
			}(wfID)
		}

		// Wait for all workflows to complete.
		for i := 0; i < len(workflows); i++ {
			if err := <-done; err != nil {
				t.Errorf("Concurrent workflow failed: %v", err)
			}
		}

		// Verify each workflow's final state.
		for _, wfID := range workflows {
			state, step, err := store.LoadLatest(ctx, wfID)
			if err != nil {
				t.Errorf("Failed to load state for %s: %v", wfID, err)
				continue
			}
			if step != 3 {
				t.Errorf("Workflow %s final step = %d, want 3", wfID, step)
			}
			if state.Steps != 3 {
				t.Errorf("Workflow %s state.Steps = %d, want 3", wfID, state.Steps)
			}
		}

		t.Log("✅ Concurrent execution test passed: 3 workflows executed independently")
	})

	t.Run("checkpoint isolation between workflows", func(t *testing.T) {
		ctx := context.Background()

		store, err := NewMySQLStore[WorkflowState](dsn)
		if err != nil {
			t.Fatalf("Failed to create MySQLStore: %v", err)
		}
		defer func() { _ = store.Close() }()

		// Create two workflows with same checkpoint labels.
		workflow1 := fmt.Sprintf("checkpoint-test-1-%d", time.Now().UnixNano())
		workflow2 := fmt.Sprintf("checkpoint-test-2-%d", time.Now().UnixNano())

		state1 := WorkflowState{
			WorkflowID: workflow1,
			Steps:      1,
			Status:     "workflow1",
			Data:       map[string]interface{}{"source": "workflow1"},
			Timestamp:  time.Now(),
		}

		state2 := WorkflowState{
			WorkflowID: workflow2,
			Steps:      2,
			Status:     "workflow2",
			Data:       map[string]interface{}{"source": "workflow2"},
			Timestamp:  time.Now(),
		}

		// Save checkpoints with same label.
		checkpoint1ID := fmt.Sprintf("%s-milestone", workflow1)
		checkpoint2ID := fmt.Sprintf("%s-milestone", workflow2)

		err = store.SaveCheckpoint(ctx, checkpoint1ID, state1, 1)
		if err != nil {
			t.Fatalf("Failed to save checkpoint for workflow1: %v", err)
		}

		err = store.SaveCheckpoint(ctx, checkpoint2ID, state2, 2)
		if err != nil {
			t.Fatalf("Failed to save checkpoint for workflow2: %v", err)
		}

		// Load checkpoint for workflow1.
		loaded1, step1, err := store.LoadCheckpoint(ctx, checkpoint1ID)
		if err != nil {
			t.Fatalf("Failed to load checkpoint for workflow1: %v", err)
		}

		// Load checkpoint for workflow2.
		loaded2, step2, err := store.LoadCheckpoint(ctx, checkpoint2ID)
		if err != nil {
			t.Fatalf("Failed to load checkpoint for workflow2: %v", err)
		}

		// Verify isolation.
		if step1 != 1 {
			t.Errorf("Workflow1 checkpoint step = %d, want 1", step1)
		}
		if step2 != 2 {
			t.Errorf("Workflow2 checkpoint step = %d, want 2", step2)
		}

		if loaded1.Status != "workflow1" {
			t.Errorf("Workflow1 checkpoint status = %q, want %q", loaded1.Status, "workflow1")
		}
		if loaded2.Status != "workflow2" {
			t.Errorf("Workflow2 checkpoint status = %q, want %q", loaded2.Status, "workflow2")
		}

		// Verify data isolation.
		if source1, ok := loaded1.Data["source"].(string); !ok || source1 != "workflow1" {
			t.Error("Workflow1 checkpoint data corrupted or mixed with workflow2")
		}
		if source2, ok := loaded2.Data["source"].(string); !ok || source2 != "workflow2" {
			t.Error("Workflow2 checkpoint data corrupted or mixed with workflow1")
		}

		t.Log("✅ Checkpoint isolation test passed: Workflows maintain independent checkpoints")
	})
}
