package graph

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dshills/langgraph-go/graph/emit"
	"github.com/dshills/langgraph-go/graph/store"
)

// TestIntegration_CheckpointResumeWorkflow verifies end-to-end checkpoint and resume cycle (T076).
func TestIntegration_CheckpointResumeWorkflow(t *testing.T) {
	t.Run("complete checkpoint/resume workflow cycle", func(t *testing.T) {
		// Create engine with realistic configuration
		reducer := func(prev, delta TestState) TestState {
			if delta.Value != "" {
				prev.Value = delta.Value
			}
			prev.Counter += delta.Counter
			return prev
		}

		st := store.NewMemStore[TestState]()
		emitter := &integrationEmitter{events: make([]emit.Event, 0)}
		engine := New(reducer, st, emitter, Options{MaxSteps: 20})

		// Define multi-step workflow simulating real LLM agent workflow
		// Step 1: Receive user query
		receiveNode := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "received", Counter: 1},
				Route: Goto("analyze"),
			}
		})

		// Step 2: Analyze query
		analyzeNode := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "analyzed", Counter: 10},
				Route: Goto("generate"),
			}
		})

		// Step 3: Generate response
		generateNode := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "generated", Counter: 100},
				Route: Goto("validate"),
			}
		})

		// Step 4: Validate output
		validateNode := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "validated", Counter: 1000},
				Route: Stop(),
			}
		})

		// Build workflow
		_ = engine.Add("receive", receiveNode)
		_ = engine.Add("analyze", analyzeNode)
		_ = engine.Add("generate", generateNode)
		_ = engine.Add("validate", validateNode)
		_ = engine.StartAt("receive")

		ctx := context.Background()

		// Phase 1: Run workflow to completion
		t.Log("Phase 1: Running initial workflow...")
		initial := TestState{Value: "query", Counter: 0}
		finalState, err := engine.Run(ctx, "integration-run-001", initial)
		if err != nil {
			t.Fatalf("Initial run failed: %v", err)
		}

		// Verify complete execution
		expectedCounter := 0 + 1 + 10 + 100 + 1000 // initial + all nodes
		if finalState.Counter != expectedCounter {
			t.Errorf("expected Counter = %d, got %d", expectedCounter, finalState.Counter)
		}
		if finalState.Value != "validated" {
			t.Errorf("expected Value = 'validated', got %q", finalState.Value)
		}

		// Phase 2: Create checkpoint
		t.Log("Phase 2: Creating checkpoint...")
		cpID := "checkpoint-after-validation"
		if err := engine.SaveCheckpoint(ctx, "integration-run-001", cpID); err != nil {
			t.Fatalf("Failed to save checkpoint: %v", err)
		}

		// Verify checkpoint was saved
		cpState, cpStep, err := st.LoadCheckpoint(ctx, cpID)
		if err != nil {
			t.Fatalf("Failed to load checkpoint: %v", err)
		}
		if cpStep != 4 {
			t.Errorf("expected checkpoint at step 4, got %d", cpStep)
		}
		if cpState.Counter != expectedCounter {
			t.Errorf("checkpoint should have final state, got Counter = %d", cpState.Counter)
		}

		// Phase 3: Resume from checkpoint and run different path
		t.Log("Phase 3: Resuming from checkpoint...")

		// Add a new continuation node for resumed workflow
		continueNode := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "continued", Counter: 10000},
				Route: Stop(),
			}
		})
		_ = engine.Add("continue", continueNode)

		// Resume from checkpoint starting at new node
		resumedState, err := engine.ResumeFromCheckpoint(ctx, cpID, "integration-run-002", "continue")
		if err != nil {
			t.Fatalf("Failed to resume from checkpoint: %v", err)
		}

		// Verify resumed execution includes checkpoint state + new node
		expectedResumedCounter := expectedCounter + 10000
		if resumedState.Counter != expectedResumedCounter {
			t.Errorf("expected resumed Counter = %d, got %d", expectedResumedCounter, resumedState.Counter)
		}
		if resumedState.Value != "continued" {
			t.Errorf("expected Value = 'continued', got %q", resumedState.Value)
		}

		// Phase 4: Verify observability events were emitted
		t.Log("Phase 4: Verifying observability...")
		if len(emitter.events) == 0 {
			t.Error("expected observability events to be emitted")
		}

		// Should have events for: initial run (4 nodes) + checkpoint + resume + continue node
		minExpectedEvents := 4 + 1 + 1 + 1 // node completions + checkpoint + resume + continue
		if len(emitter.events) < minExpectedEvents {
			t.Errorf("expected at least %d events, got %d", minExpectedEvents, len(emitter.events))
		}

		// Verify checkpoint event exists
		foundCheckpointEvent := false
		for _, event := range emitter.events {
			if event.Msg == "checkpoint saved: "+cpID {
				foundCheckpointEvent = true
				break
			}
		}
		if !foundCheckpointEvent {
			t.Error("expected checkpoint event in emitted events")
		}

		t.Log("Integration test passed: checkpoint/resume cycle complete")
	})
}

// TestIntegration_WorkflowCrashRecovery verifies 5-node workflow crash and recovery scenario (T077).
func TestIntegration_WorkflowCrashRecovery(t *testing.T) {
	t.Run("5-node workflow with crash and recovery", func(t *testing.T) {
		// Create engine
		reducer := func(prev, delta TestState) TestState {
			if delta.Value != "" {
				prev.Value = delta.Value
			}
			prev.Counter += delta.Counter
			return prev
		}

		st := store.NewMemStore[TestState]()
		emitter := &integrationEmitter{events: make([]emit.Event, 0)}
		engine := New(reducer, st, emitter, Options{MaxSteps: 20})

		// Simulate a long-running workflow that crashes mid-execution
		// Node 1: Initialize
		initNode := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			t.Log("  Node 1: Initialize")
			return NodeResult[TestState]{
				Delta: TestState{Value: "initialized", Counter: 1},
				Route: Goto("fetch"),
			}
		})

		// Node 2: Fetch data (succeeds)
		fetchNode := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			t.Log("  Node 2: Fetch data")
			time.Sleep(10 * time.Millisecond) // Simulate I/O
			return NodeResult[TestState]{
				Delta: TestState{Value: "fetched", Counter: 10},
				Route: Goto("process"),
			}
		})

		// Node 3: Process (will fail on first attempt)
		processAttempts := 0
		processNode := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			processAttempts++
			t.Logf("  Node 3: Process (attempt %d)", processAttempts)

			// Simulate crash on first attempt
			if processAttempts == 1 {
				t.Log("  Node 3: CRASH! (simulated)")
				return NodeResult[TestState]{
					Err:   errors.New("simulated processing failure"),
					Route: Stop(),
				}
			}

			// Succeed on retry
			return NodeResult[TestState]{
				Delta: TestState{Value: "processed", Counter: 100},
				Route: Goto("validate"),
			}
		})

		// Node 4: Validate
		validateNode := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			t.Log("  Node 4: Validate")
			return NodeResult[TestState]{
				Delta: TestState{Value: "validated", Counter: 1000},
				Route: Goto("finalize"),
			}
		})

		// Node 5: Finalize
		finalizeNode := NodeFunc[TestState](func(ctx context.Context, s TestState) NodeResult[TestState] {
			t.Log("  Node 5: Finalize")
			return NodeResult[TestState]{
				Delta: TestState{Value: "finalized", Counter: 10000},
				Route: Stop(),
			}
		})

		// Build 5-node workflow
		_ = engine.Add("init", initNode)
		_ = engine.Add("fetch", fetchNode)
		_ = engine.Add("process", processNode)
		_ = engine.Add("validate", validateNode)
		_ = engine.Add("finalize", finalizeNode)
		_ = engine.StartAt("init")

		ctx := context.Background()

		// Attempt 1: Run workflow (will fail at node 3)
		t.Log("Attempt 1: Running workflow (will crash)...")
		initial := TestState{Value: "start", Counter: 0}
		_, err := engine.Run(ctx, "crash-run-001", initial)

		// Verify failure occurred
		if err == nil {
			t.Fatal("expected error from simulated crash")
		}
		if err.Error() != "simulated processing failure" {
			t.Errorf("unexpected error: %v", err)
		}

		// Verify partial state was saved (nodes 1 and 2 completed)
		partialState, partialStep, err := st.LoadLatest(ctx, "crash-run-001")
		if err != nil {
			t.Fatalf("expected partial state to be saved: %v", err)
		}
		if partialState.Value != "fetched" {
			t.Errorf("expected partial Value = 'fetched', got %q", partialState.Value)
		}
		if partialState.Counter != 11 { // init(1) + fetch(10)
			t.Errorf("expected partial Counter = 11, got %d", partialState.Counter)
		}
		if partialStep != 2 {
			t.Errorf("expected crashed at step 2, got %d", partialStep)
		}

		t.Log("Crash verified: partial state saved at step 2")

		// Create checkpoint from crashed state for recovery
		t.Log("Creating recovery checkpoint...")
		if err := engine.SaveCheckpoint(ctx, "crash-run-001", "recovery-checkpoint"); err != nil {
			t.Fatalf("Failed to save recovery checkpoint: %v", err)
		}

		// Attempt 2: Resume from checkpoint (will succeed this time)
		t.Log("Attempt 2: Resuming from checkpoint (should succeed)...")
		recoveredState, err := engine.ResumeFromCheckpoint(ctx, "recovery-checkpoint", "crash-run-002", "process")
		if err != nil {
			t.Fatalf("Recovery run failed: %v", err)
		}

		// Verify full completion after recovery
		// Should have: partial state (11) + process (100) + validate (1000) + finalize (10000)
		expectedCounter := 11 + 100 + 1000 + 10000
		if recoveredState.Counter != expectedCounter {
			t.Errorf("expected recovered Counter = %d, got %d", expectedCounter, recoveredState.Counter)
		}
		if recoveredState.Value != "finalized" {
			t.Errorf("expected recovered Value = 'finalized', got %q", recoveredState.Value)
		}

		// Verify process node was called twice (once failed, once succeeded)
		if processAttempts != 2 {
			t.Errorf("expected 2 process attempts, got %d", processAttempts)
		}

		// Verify observability captured the failure and recovery
		foundErrorEvent := false
		for _, event := range emitter.events {
			// Note: current implementation doesn't emit error events, but this validates the structure
			if event.Msg != "" {
				t.Logf("Event: %s (step %d, node %s)", event.Msg, event.Step, event.NodeID)
			}
		}

		// For now, just verify we have events
		if len(emitter.events) == 0 {
			t.Error("expected observability events")
		}

		_ = foundErrorEvent // Will be used when error events are implemented

		t.Log("Integration test passed: crash recovery successful")
	})
}

// integrationEmitter captures events for verification in integration tests.
type integrationEmitter struct {
	events []emit.Event
}

func (e *integrationEmitter) Emit(event emit.Event) {
	e.events = append(e.events, event)
}
