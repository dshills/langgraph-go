// Package graph provides the core graph execution engine for LangGraph-Go.
package graph

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/dshills/langgraph-go/graph/emit"
	"github.com/dshills/langgraph-go/graph/model"
	"github.com/dshills/langgraph-go/graph/store"
)

// TestIntegration_CheckpointResumeWorkflow verifies end-to-end checkpoint and resume cycle (T076).
func TestIntegration_CheckpointResumeWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Run("complete checkpoint/resume workflow cycle", func(t *testing.T) {
		// Create engine with realistic configuration.
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

		// Define multi-step workflow simulating real LLM agent workflow.
		// Step 1: Receive user query.
		receiveNode := NodeFunc[TestState](func(_ context.Context, _ TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "received", Counter: 1},
				Route: Goto("analyze"),
			}
		})

		// Step 2: Analyze query.
		analyzeNode := NodeFunc[TestState](func(_ context.Context, _ TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "analyzed", Counter: 10},
				Route: Goto("generate"),
			}
		})

		// Step 3: Generate response.
		generateNode := NodeFunc[TestState](func(_ context.Context, _ TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "generated", Counter: 100},
				Route: Goto("validate"),
			}
		})

		// Step 4: Validate output.
		validateNode := NodeFunc[TestState](func(_ context.Context, _ TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "validated", Counter: 1000},
				Route: Stop(),
			}
		})

		// Build workflow.
		_ = engine.Add("receive", receiveNode)
		_ = engine.Add("analyze", analyzeNode)
		_ = engine.Add("generate", generateNode)
		_ = engine.Add("validate", validateNode)
		_ = engine.StartAt("receive")

		ctx := context.Background()

		// Phase 1: Run workflow to completion.
		t.Log("Phase 1: Running initial workflow...")
		initial := TestState{Value: "query", Counter: 0}
		finalState, err := engine.Run(ctx, "integration-run-001", initial)
		if err != nil {
			t.Fatalf("Initial run failed: %v", err)
		}

		// Verify complete execution.
		expectedCounter := 0 + 1 + 10 + 100 + 1000 // initial + all nodes
		if finalState.Counter != expectedCounter {
			t.Errorf("expected Counter = %d, got %d", expectedCounter, finalState.Counter)
		}
		if finalState.Value != "validated" {
			t.Errorf("expected Value = 'validated', got %q", finalState.Value)
		}

		// Phase 2: Create checkpoint.
		t.Log("Phase 2: Creating checkpoint...")
		cpID := "checkpoint-after-validation"
		if err := engine.SaveCheckpoint(ctx, "integration-run-001", cpID); err != nil {
			t.Fatalf("Failed to save checkpoint: %v", err)
		}

		// Verify checkpoint was saved.
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

		// Phase 3: Resume from checkpoint and run different path.
		t.Log("Phase 3: Resuming from checkpoint...")

		// Add a new continuation node for resumed workflow.
		continueNode := NodeFunc[TestState](func(_ context.Context, _ TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "continued", Counter: 10000},
				Route: Stop(),
			}
		})
		_ = engine.Add("continue", continueNode)

		// Resume from checkpoint starting at new node.
		resumedState, err := engine.ResumeFromCheckpoint(ctx, cpID, "integration-run-002", "continue")
		if err != nil {
			t.Fatalf("Failed to resume from checkpoint: %v", err)
		}

		// Verify resumed execution includes checkpoint state + new node.
		expectedResumedCounter := expectedCounter + 10000
		if resumedState.Counter != expectedResumedCounter {
			t.Errorf("expected resumed Counter = %d, got %d", expectedResumedCounter, resumedState.Counter)
		}
		if resumedState.Value != "continued" {
			t.Errorf("expected Value = 'continued', got %q", resumedState.Value)
		}

		// Phase 4: Verify observability events were emitted.
		t.Log("Phase 4: Verifying observability...")
		if len(emitter.events) == 0 {
			t.Error("expected observability events to be emitted")
		}

		// Should have events for: initial run (4 nodes) + checkpoint + resume + continue node.
		minExpectedEvents := 4 + 1 + 1 + 1 // node completions + checkpoint + resume + continue
		if len(emitter.events) < minExpectedEvents {
			t.Errorf("expected at least %d events, got %d", minExpectedEvents, len(emitter.events))
		}

		// Verify checkpoint event exists.
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
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Run("5-node workflow with crash and recovery", func(t *testing.T) {
		// Create engine.
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

		// Simulate a long-running workflow that crashes mid-execution.
		// Node 1: Initialize.
		initNode := NodeFunc[TestState](func(_ context.Context, _ TestState) NodeResult[TestState] {
			t.Log("  Node 1: Initialize")
			return NodeResult[TestState]{
				Delta: TestState{Value: "initialized", Counter: 1},
				Route: Goto("fetch"),
			}
		})

		// Node 2: Fetch data (succeeds).
		fetchNode := NodeFunc[TestState](func(_ context.Context, _ TestState) NodeResult[TestState] {
			t.Log("  Node 2: Fetch data")
			time.Sleep(10 * time.Millisecond) // Simulate I/O
			return NodeResult[TestState]{
				Delta: TestState{Value: "fetched", Counter: 10},
				Route: Goto("process"),
			}
		})

		// Node 3: Process (will fail on first attempt).
		processAttempts := 0
		processNode := NodeFunc[TestState](func(_ context.Context, _ TestState) NodeResult[TestState] {
			processAttempts++
			t.Logf("  Node 3: Process (attempt %d)", processAttempts)

			// Simulate crash on first attempt.
			if processAttempts == 1 {
				t.Log("  Node 3: CRASH! (simulated)")
				return NodeResult[TestState]{
					Err:   errors.New("simulated processing failure"),
					Route: Stop(),
				}
			}

			// Succeed on retry.
			return NodeResult[TestState]{
				Delta: TestState{Value: "processed", Counter: 100},
				Route: Goto("validate"),
			}
		})

		// Node 4: Validate.
		validateNode := NodeFunc[TestState](func(_ context.Context, _ TestState) NodeResult[TestState] {
			t.Log("  Node 4: Validate")
			return NodeResult[TestState]{
				Delta: TestState{Value: "validated", Counter: 1000},
				Route: Goto("finalize"),
			}
		})

		// Node 5: Finalize.
		finalizeNode := NodeFunc[TestState](func(_ context.Context, _ TestState) NodeResult[TestState] {
			t.Log("  Node 5: Finalize")
			return NodeResult[TestState]{
				Delta: TestState{Value: "finalized", Counter: 10000},
				Route: Stop(),
			}
		})

		// Build 5-node workflow.
		_ = engine.Add("init", initNode)
		_ = engine.Add("fetch", fetchNode)
		_ = engine.Add("process", processNode)
		_ = engine.Add("validate", validateNode)
		_ = engine.Add("finalize", finalizeNode)
		_ = engine.StartAt("init")

		ctx := context.Background()

		// Attempt 1: Run workflow (will fail at node 3).
		t.Log("Attempt 1: Running workflow (will crash)...")
		initial := TestState{Value: "start", Counter: 0}
		_, err := engine.Run(ctx, "crash-run-001", initial)

		// Verify failure occurred.
		if err == nil {
			t.Fatal("expected error from simulated crash")
		}
		if err.Error() != "simulated processing failure" {
			t.Errorf("unexpected error: %v", err)
		}

		// Verify partial state was saved (nodes 1 and 2 completed).
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

		// Create checkpoint from crashed state for recovery.
		t.Log("Creating recovery checkpoint...")
		if err := engine.SaveCheckpoint(ctx, "crash-run-001", "recovery-checkpoint"); err != nil {
			t.Fatalf("Failed to save recovery checkpoint: %v", err)
		}

		// Attempt 2: Resume from checkpoint (will succeed this time).
		t.Log("Attempt 2: Resuming from checkpoint (should succeed)...")
		recoveredState, err := engine.ResumeFromCheckpoint(ctx, "recovery-checkpoint", "crash-run-002", "process")
		if err != nil {
			t.Fatalf("Recovery run failed: %v", err)
		}

		// Verify full completion after recovery.
		// Should have: partial state (11) + process (100) + validate (1000) + finalize (10000).
		expectedCounter := 11 + 100 + 1000 + 10000
		if recoveredState.Counter != expectedCounter {
			t.Errorf("expected recovered Counter = %d, got %d", expectedCounter, recoveredState.Counter)
		}
		if recoveredState.Value != "finalized" {
			t.Errorf("expected recovered Value = 'finalized', got %q", recoveredState.Value)
		}

		// Verify process node was called twice (once failed, once succeeded).
		if processAttempts != 2 {
			t.Errorf("expected 2 process attempts, got %d", processAttempts)
		}

		// Verify observability captured the failure and recovery.
		foundErrorEvent := false
		for _, event := range emitter.events {
			// Note: current implementation doesn't emit error events, but this validates the structure.
			if event.Msg != "" {
				t.Logf("Event: %s (step %d, node %s)", event.Msg, event.Step, event.NodeID)
			}
		}

		// For now, just verify we have events.
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

// TODO: Implement in Phase 8
func (e *integrationEmitter) EmitBatch(_ context.Context, events []emit.Event) error {
	for _, event := range events {
		e.Emit(event)
	}
	return nil
}

// TODO: Implement in Phase 8
func (e *integrationEmitter) Flush(_ context.Context) error {
	return nil
}

// TestIntegration_ConfidenceBasedRouting verifies confidence-based conditional routing (T099).
func TestIntegration_ConfidenceBasedRouting(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Run("route based on confidence score with multiple paths", func(t *testing.T) {
		reducer := func(prev, delta TestState) TestState {
			if delta.Value != "" {
				prev.Value = delta.Value
			}
			prev.Counter += delta.Counter
			return prev
		}

		st := store.NewMemStore[TestState]()
		emitter := &integrationEmitter{events: make([]emit.Event, 0)}
		engine := New(reducer, st, emitter, Options{MaxSteps: 10})

		// Simulate LLM agent workflow with confidence-based routing.
		// Node: generate - Creates response with confidence score.
		generateNode := NodeFunc[TestState](func(_ context.Context, _ TestState) NodeResult[TestState] {
			// Simulate low confidence (Counter will represent confidence * 100).
			return NodeResult[TestState]{
				Delta: TestState{Value: "generated", Counter: 65}, // 0.65 confidence
				Route: Next{},                                     // Use edge routing based on confidence
			}
		})

		// Node: refine - Improves response.
		refineNode := NodeFunc[TestState](func(_ context.Context, _ TestState) NodeResult[TestState] {
			// Improve confidence.
			return NodeResult[TestState]{
				Delta: TestState{Value: "refined", Counter: 20}, // +0.20 confidence
				Route: Next{},                                   // Re-evaluate confidence
			}
		})

		// Node: validate - Final validation for high confidence.
		validateNode := NodeFunc[TestState](func(_ context.Context, _ TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "validated", Counter: 1},
				Route: Stop(),
			}
		})

		// Node: fallback - Handles very low confidence.
		fallbackNode := NodeFunc[TestState](func(_ context.Context, _ TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "fallback", Counter: 100},
				Route: Stop(),
			}
		})

		_ = engine.Add("generate", generateNode)
		_ = engine.Add("refine", refineNode)
		_ = engine.Add("validate", validateNode)
		_ = engine.Add("fallback", fallbackNode)
		_ = engine.StartAt("generate")

		// Edge predicates based on confidence thresholds.
		// generate → fallback (if confidence < 50).
		veryLowConfidence := func(s TestState) bool {
			return s.Counter < 50
		}
		_ = engine.Connect("generate", "fallback", veryLowConfidence)

		// generate → refine (if 50 <= confidence < 80).
		lowConfidence := func(s TestState) bool {
			return s.Counter >= 50 && s.Counter < 80
		}
		_ = engine.Connect("generate", "refine", lowConfidence)

		// generate → validate (if confidence >= 80).
		highConfidence := func(s TestState) bool {
			return s.Counter >= 80
		}
		_ = engine.Connect("generate", "validate", highConfidence)

		// refine → validate (after refinement, confidence should be high enough).
		_ = engine.Connect("refine", "validate", nil)

		ctx := context.Background()
		finalState, err := engine.Run(ctx, "confidence-run-001", TestState{})

		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Should have gone: generate (65) → refine (+20 = 85) → validate (+1 = 86).
		if finalState.Counter != 86 {
			t.Errorf("expected Counter = 86, got %d", finalState.Counter)
		}
		if finalState.Value != "validated" {
			t.Errorf("expected Value = 'validated', got %q", finalState.Value)
		}

		// Verify routing path via events.
		expectedNodes := []string{"generate", "refine", "validate"}
		if len(emitter.events) < len(expectedNodes) {
			t.Errorf("expected at least %d events, got %d", len(expectedNodes), len(emitter.events))
		}

		t.Log("Integration test passed: confidence-based routing worked correctly")
	})
}

// TestIntegration_LoopWithExitCondition verifies loop with conditional exit (T100).
func TestIntegration_LoopWithExitCondition(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Run("loop until condition met with max attempts protection", func(t *testing.T) {
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

		// Simulate iterative refinement loop.
		// Node: process - Incremental processing.
		processNode := NodeFunc[TestState](func(_ context.Context, _ TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "processing", Counter: 1},
				Route: Next{}, // Use edge routing to decide continue or exit
			}
		})

		// Node: validate - Check if done.
		validateNode := NodeFunc[TestState](func(_ context.Context, _ TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "checking", Counter: 0},
				Route: Next{}, // Use edge routing
			}
		})

		// Node: complete - Success exit.
		completeNode := NodeFunc[TestState](func(_ context.Context, _ TestState) NodeResult[TestState] {
			return NodeResult[TestState]{
				Delta: TestState{Value: "completed", Counter: 100},
				Route: Stop(),
			}
		})

		_ = engine.Add("process", processNode)
		_ = engine.Add("validate", validateNode)
		_ = engine.Add("complete", completeNode)
		_ = engine.StartAt("process")

		// process → validate (always check after processing).
		_ = engine.Connect("process", "validate", nil)

		// validate → complete (if Counter >= 5, we're done).
		exitCondition := func(s TestState) bool {
			return s.Counter >= 5
		}
		_ = engine.Connect("validate", "complete", exitCondition)

		// validate → process (if Counter < 5, keep looping).
		continueCondition := func(s TestState) bool {
			return s.Counter < 5
		}
		_ = engine.Connect("validate", "process", continueCondition)

		ctx := context.Background()
		finalState, err := engine.Run(ctx, "loop-exit-run-001", TestState{})

		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Should loop 5 times: process(1), validate, process(1), ..., complete(100).
		// Final: 5 (from processing) + 100 (from complete) = 105.
		if finalState.Counter != 105 {
			t.Errorf("expected Counter = 105, got %d", finalState.Counter)
		}
		if finalState.Value != "completed" {
			t.Errorf("expected Value = 'completed', got %q", finalState.Value)
		}

		// Verify loop iterations.
		// Should have: process, validate, process, validate, ..., complete.
		// At least 11 node executions (5 process + 5 validate + 1 complete).
		minEvents := 11
		if len(emitter.events) < minEvents {
			t.Errorf("expected at least %d events (loop iterations), got %d", minEvents, len(emitter.events))
		}

		t.Log("Integration test passed: loop with exit condition worked correctly")
	})
}

// TestIntegration_ParallelExecution tests 4-branch parallel execution workflow (T118).
func TestIntegration_ParallelExecution(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Run("4-branch parallel fan-out workflow", func(t *testing.T) {
		// State for parallel processing with result collection.
		type ParallelState struct {
			Input    string
			Results  []string
			Count    int
			Complete bool
		}

		// Reducer merges results from parallel branches.
		reducer := func(prev, delta ParallelState) ParallelState {
			if delta.Input != "" {
				prev.Input = delta.Input
			}
			if len(delta.Results) > 0 {
				prev.Results = append(prev.Results, delta.Results...)
			}
			prev.Count += delta.Count
			if delta.Complete {
				prev.Complete = delta.Complete
			}
			return prev
		}

		st := store.NewMemStore[ParallelState]()
		emitter := &mockEmitter{}
		opts := Options{MaxSteps: 100}
		engine := New(reducer, st, emitter, opts)

		// Fanout node routes to 4 parallel branches.
		fanout := NodeFunc[ParallelState](func(_ context.Context, _ ParallelState) NodeResult[ParallelState] {
			return NodeResult[ParallelState]{
				Route: Next{Many: []string{"branch1", "branch2", "branch3", "branch4"}},
			}
		})

		// Branch 1: Fast processing (50ms).
		branch1 := NodeFunc[ParallelState](func(_ context.Context, _ ParallelState) NodeResult[ParallelState] {
			time.Sleep(50 * time.Millisecond)
			return NodeResult[ParallelState]{
				Delta: ParallelState{
					Results: []string{"branch1-result"},
					Count:   1,
				},
				Route: Stop(),
			}
		})

		// Branch 2: Medium processing (75ms).
		branch2 := NodeFunc[ParallelState](func(_ context.Context, _ ParallelState) NodeResult[ParallelState] {
			time.Sleep(75 * time.Millisecond)
			return NodeResult[ParallelState]{
				Delta: ParallelState{
					Results: []string{"branch2-result"},
					Count:   1,
				},
				Route: Stop(),
			}
		})

		// Branch 3: Slow processing (100ms).
		branch3 := NodeFunc[ParallelState](func(_ context.Context, _ ParallelState) NodeResult[ParallelState] {
			time.Sleep(100 * time.Millisecond)
			return NodeResult[ParallelState]{
				Delta: ParallelState{
					Results: []string{"branch3-result"},
					Count:   1,
				},
				Route: Stop(),
			}
		})

		// Branch 4: Very fast processing (25ms).
		branch4 := NodeFunc[ParallelState](func(_ context.Context, _ ParallelState) NodeResult[ParallelState] {
			time.Sleep(25 * time.Millisecond)
			return NodeResult[ParallelState]{
				Delta: ParallelState{
					Results: []string{"branch4-result"},
					Count:   1,
				},
				Route: Stop(),
			}
		})

		// Build workflow.
		if err := engine.Add("fanout", fanout); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := engine.Add("branch1", branch1); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := engine.Add("branch2", branch2); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := engine.Add("branch3", branch3); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := engine.Add("branch4", branch4); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := engine.StartAt("fanout"); err != nil {
			t.Fatalf("StartAt failed: %v", err)
		}

		// Execute workflow.
		ctx := context.Background()
		start := time.Now()
		finalState, err := engine.Run(ctx, "parallel-integration-001", ParallelState{Input: "test"})
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Verify all 4 branches executed.
		if finalState.Count != 4 {
			t.Errorf("expected Count = 4, got %d", finalState.Count)
		}

		// Verify all results collected.
		if len(finalState.Results) != 4 {
			t.Fatalf("expected 4 results, got %d", len(finalState.Results))
		}

		// Verify deterministic merge order (lexicographic by nodeID).
		expectedOrder := []string{"branch1-result", "branch2-result", "branch3-result", "branch4-result"}
		for i, expected := range expectedOrder {
			if finalState.Results[i] != expected {
				t.Errorf("position %d: expected %q, got %q", i, expected, finalState.Results[i])
			}
		}

		// Verify parallel execution (should take ~100ms, not ~250ms sequential).
		if elapsed > 200*time.Millisecond {
			t.Errorf("parallel execution took %v, expected < 200ms (likely ran sequentially)", elapsed)
		}

		t.Logf("Integration test passed: 4 parallel branches completed in %v", elapsed)
	})
}

// TestIntegration_ParallelErrorHandling tests error handling in parallel branches (T119).
func TestIntegration_ParallelErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Run("error in one parallel branch stops execution", func(t *testing.T) {
		type ParallelState struct {
			Results []string
			Count   int
		}

		reducer := func(prev, delta ParallelState) ParallelState {
			if len(delta.Results) > 0 {
				prev.Results = append(prev.Results, delta.Results...)
			}
			prev.Count += delta.Count
			return prev
		}

		st := store.NewMemStore[ParallelState]()
		emitter := &mockEmitter{}
		opts := Options{MaxSteps: 100}
		engine := New(reducer, st, emitter, opts)

		// Fanout to 3 branches.
		fanout := NodeFunc[ParallelState](func(_ context.Context, _ ParallelState) NodeResult[ParallelState] {
			return NodeResult[ParallelState]{
				Route: Next{Many: []string{"success1", "failing", "success2"}},
			}
		})

		// Success branch 1.
		success1 := NodeFunc[ParallelState](func(_ context.Context, _ ParallelState) NodeResult[ParallelState] {
			time.Sleep(30 * time.Millisecond)
			return NodeResult[ParallelState]{
				Delta: ParallelState{Results: []string{"success1"}, Count: 1},
				Route: Stop(),
			}
		})

		// Failing branch.
		failing := NodeFunc[ParallelState](func(_ context.Context, _ ParallelState) NodeResult[ParallelState] {
			time.Sleep(50 * time.Millisecond)
			return NodeResult[ParallelState]{
				Err: &EngineError{
					Message: "simulated branch failure",
					Code:    "BRANCH_FAIL",
				},
			}
		})

		// Success branch 2.
		success2 := NodeFunc[ParallelState](func(_ context.Context, _ ParallelState) NodeResult[ParallelState] {
			time.Sleep(40 * time.Millisecond)
			return NodeResult[ParallelState]{
				Delta: ParallelState{Results: []string{"success2"}, Count: 1},
				Route: Stop(),
			}
		})

		// Build workflow.
		if err := engine.Add("fanout", fanout); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := engine.Add("success1", success1); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := engine.Add("failing", failing); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := engine.Add("success2", success2); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := engine.StartAt("fanout"); err != nil {
			t.Fatalf("StartAt failed: %v", err)
		}

		// Execute workflow (should fail).
		ctx := context.Background()
		_, err := engine.Run(ctx, "parallel-error-001", ParallelState{})

		// Verify error was returned.
		if err == nil {
			t.Fatal("expected error from failing branch, got nil")
		}

		// Verify error message.
		if !strings.Contains(err.Error(), "simulated branch failure") {
			t.Errorf("expected error about 'simulated branch failure', got %q", err.Error())
		}

		t.Logf("Integration test passed: parallel error handling works correctly (error: %v)", err)
	})

	t.Run("multiple parallel branches fail", func(t *testing.T) {
		type ParallelState struct {
			Count int
		}

		reducer := func(prev, delta ParallelState) ParallelState {
			prev.Count += delta.Count
			return prev
		}

		st := store.NewMemStore[ParallelState]()
		emitter := &mockEmitter{}
		opts := Options{MaxSteps: 100}
		engine := New(reducer, st, emitter, opts)

		// Fanout to 4 branches.
		fanout := NodeFunc[ParallelState](func(_ context.Context, _ ParallelState) NodeResult[ParallelState] {
			return NodeResult[ParallelState]{
				Route: Next{Many: []string{"fail1", "success", "fail2", "fail3"}},
			}
		})

		// Failing branch 1.
		fail1 := NodeFunc[ParallelState](func(_ context.Context, _ ParallelState) NodeResult[ParallelState] {
			time.Sleep(20 * time.Millisecond)
			return NodeResult[ParallelState]{
				Err: &EngineError{Message: "error1", Code: "ERR1"},
			}
		})

		// Success branch.
		success := NodeFunc[ParallelState](func(_ context.Context, _ ParallelState) NodeResult[ParallelState] {
			time.Sleep(30 * time.Millisecond)
			return NodeResult[ParallelState]{
				Delta: ParallelState{Count: 1},
				Route: Stop(),
			}
		})

		// Failing branch 2.
		fail2 := NodeFunc[ParallelState](func(_ context.Context, _ ParallelState) NodeResult[ParallelState] {
			time.Sleep(40 * time.Millisecond)
			return NodeResult[ParallelState]{
				Err: &EngineError{Message: "error2", Code: "ERR2"},
			}
		})

		// Failing branch 3.
		fail3 := NodeFunc[ParallelState](func(_ context.Context, _ ParallelState) NodeResult[ParallelState] {
			time.Sleep(10 * time.Millisecond)
			return NodeResult[ParallelState]{
				Err: &EngineError{Message: "error3", Code: "ERR3"},
			}
		})

		// Build workflow.
		if err := engine.Add("fanout", fanout); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := engine.Add("fail1", fail1); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := engine.Add("success", success); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := engine.Add("fail2", fail2); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := engine.Add("fail3", fail3); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := engine.StartAt("fanout"); err != nil {
			t.Fatalf("StartAt failed: %v", err)
		}

		// Execute workflow (should fail with one of the errors).
		ctx := context.Background()
		_, err := engine.Run(ctx, "multi-error-001", ParallelState{})

		// Verify at least one error was returned.
		if err == nil {
			t.Fatal("expected error from failing branches, got nil")
		}

		// Verify it's one of the expected errors.
		errMsg := err.Error()
		hasErr1 := strings.Contains(errMsg, "error1")
		hasErr2 := strings.Contains(errMsg, "error2")
		hasErr3 := strings.Contains(errMsg, "error3")

		if !hasErr1 && !hasErr2 && !hasErr3 {
			t.Errorf("expected one of the branch errors, got %q", errMsg)
		}

		t.Logf("Integration test passed: multiple failures handled (returned: %v)", err)
	})
}

// TestIntegration_MultiProviderWorkflow tests workflow with multiple LLM providers (T150).
func TestIntegration_MultiProviderWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	ctx := context.Background()

	t.Run("provider switching based on task", func(t *testing.T) {
		// Setup: Create mock models for different providers.
		openaiMock := &model.MockChatModel{
			Responses: []model.ChatOut{
				{Text: "OpenAI response"},
			},
		}

		anthropicMock := &model.MockChatModel{
			Responses: []model.ChatOut{
				{Text: "Claude response with detailed reasoning"},
			},
		}

		googleMock := &model.MockChatModel{
			Responses: []model.ChatOut{
				{Text: "Gemini quick response"},
			},
		}

		// Use different providers for different tasks.
		messages := []model.Message{
			{Role: model.RoleUser, Content: "Hello!"},
		}

		// Task 1: Quick response from Gemini.
		out1, err := googleMock.Chat(ctx, messages, nil)
		if err != nil {
			t.Fatalf("Gemini failed: %v", err)
		}
		if out1.Text != "Gemini quick response" {
			t.Errorf("unexpected Gemini response: %s", out1.Text)
		}

		// Task 2: Detailed reasoning from Claude.
		out2, err := anthropicMock.Chat(ctx, messages, nil)
		if err != nil {
			t.Fatalf("Claude failed: %v", err)
		}
		if out2.Text != "Claude response with detailed reasoning" {
			t.Errorf("unexpected Claude response: %s", out2.Text)
		}

		// Task 3: General task with OpenAI.
		out3, err := openaiMock.Chat(ctx, messages, nil)
		if err != nil {
			t.Fatalf("OpenAI failed: %v", err)
		}
		if out3.Text != "OpenAI response" {
			t.Errorf("unexpected OpenAI response: %s", out3.Text)
		}

		// Verify each provider was called once.
		if openaiMock.CallCount() != 1 {
			t.Errorf("OpenAI call count: expected 1, got %d", openaiMock.CallCount())
		}
		if anthropicMock.CallCount() != 1 {
			t.Errorf("Anthropic call count: expected 1, got %d", anthropicMock.CallCount())
		}
		if googleMock.CallCount() != 1 {
			t.Errorf("Google call count: expected 1, got %d", googleMock.CallCount())
		}
	})

	t.Run("fallback to secondary provider on error", func(t *testing.T) {
		// Primary provider fails.
		primaryMock := &model.MockChatModel{
			Err: errors.New("rate limit exceeded"),
		}

		// Secondary provider succeeds.
		secondaryMock := &model.MockChatModel{
			Responses: []model.ChatOut{
				{Text: "Fallback response"},
			},
		}

		messages := []model.Message{
			{Role: model.RoleUser, Content: "Test"},
		}

		// Try primary first.
		_, err := primaryMock.Chat(ctx, messages, nil)
		if err == nil {
			t.Fatal("expected primary to fail")
		}

		// Fallback to secondary.
		out, err := secondaryMock.Chat(ctx, messages, nil)
		if err != nil {
			t.Fatalf("secondary failed: %v", err)
		}

		if out.Text != "Fallback response" {
			t.Errorf("unexpected fallback response: %s", out.Text)
		}
	})

	t.Run("multi-turn conversation with provider", func(t *testing.T) {
		mockModel := &model.MockChatModel{
			Responses: []model.ChatOut{
				{Text: "First response"},
				{Text: "Second response"},
				{Text: "Third response"},
			},
		}

		// Turn 1.
		messages1 := []model.Message{
			{Role: model.RoleUser, Content: "First question"},
		}
		out1, err := mockModel.Chat(ctx, messages1, nil)
		if err != nil {
			t.Fatalf("turn 1 failed: %v", err)
		}
		if out1.Text != "First response" {
			t.Errorf("turn 1: expected 'First response', got %q", out1.Text)
		}

		// Turn 2 - conversation builds.
		messages2 := []model.Message{
			{Role: model.RoleUser, Content: "First question"},
			{Role: model.RoleAssistant, Content: "First response"},
			{Role: model.RoleUser, Content: "Second question"},
		}
		out2, err := mockModel.Chat(ctx, messages2, nil)
		if err != nil {
			t.Fatalf("turn 2 failed: %v", err)
		}
		if out2.Text != "Second response" {
			t.Errorf("turn 2: expected 'Second response', got %q", out2.Text)
		}

		// Turn 3.
		messages3 := []model.Message{
			{Role: model.RoleUser, Content: "First question"},
			{Role: model.RoleAssistant, Content: "First response"},
			{Role: model.RoleUser, Content: "Second question"},
			{Role: model.RoleAssistant, Content: "Second response"},
			{Role: model.RoleUser, Content: "Third question"},
		}
		out3, err := mockModel.Chat(ctx, messages3, nil)
		if err != nil {
			t.Fatalf("turn 3 failed: %v", err)
		}
		if out3.Text != "Third response" {
			t.Errorf("turn 3: expected 'Third response', got %q", out3.Text)
		}

		// Verify conversation history was tracked.
		if mockModel.CallCount() != 3 {
			t.Errorf("expected 3 calls, got %d", mockModel.CallCount())
		}

		// Verify message history accumulates.
		lastCall := mockModel.Calls[2]
		if len(lastCall.Messages) != 5 {
			t.Errorf("turn 3: expected 5 messages in history, got %d", len(lastCall.Messages))
		}
	})

	t.Run("tool calling workflow", func(t *testing.T) {
		// Setup model that returns tool calls.
		mockModel := &model.MockChatModel{
			Responses: []model.ChatOut{
				{
					// First response: model decides to use a tool.
					ToolCalls: []model.ToolCall{
						{
							Name:  "search",
							Input: map[string]interface{}{"query": "weather in Paris"},
						},
					},
				},
				{
					// Second response: model uses tool result.
					Text: "Based on the search results, the weather in Paris is sunny, 22°C.",
				},
			},
		}

		tools := []model.ToolSpec{
			{
				Name:        "search",
				Description: "Search for information",
				Schema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query": map[string]interface{}{
							"type":        "string",
							"description": "Search query",
						},
					},
					"required": []string{"query"},
				},
			},
		}

		// Turn 1: User asks question, model decides to use tool.
		messages1 := []model.Message{
			{Role: model.RoleUser, Content: "What's the weather in Paris?"},
		}

		out1, err := mockModel.Chat(ctx, messages1, tools)
		if err != nil {
			t.Fatalf("turn 1 failed: %v", err)
		}

		if len(out1.ToolCalls) != 1 {
			t.Fatalf("expected 1 tool call, got %d", len(out1.ToolCalls))
		}

		toolCall := out1.ToolCalls[0]
		if toolCall.Name != "search" {
			t.Errorf("expected tool 'search', got %q", toolCall.Name)
		}

		// Simulate tool execution.
		query, ok := toolCall.Input["query"].(string)
		if !ok || query != "weather in Paris" {
			t.Errorf("expected query 'weather in Paris', got %v", toolCall.Input["query"])
		}

		// Turn 2: Send tool result back (in a real workflow).
		messages2 := []model.Message{
			{Role: model.RoleUser, Content: "What's the weather in Paris?"},
			{Role: model.RoleAssistant, Content: "Let me search for that information."},
			{Role: model.RoleUser, Content: "The search returned: Sunny, 22°C"},
		}

		out2, err := mockModel.Chat(ctx, messages2, tools)
		if err != nil {
			t.Fatalf("turn 2 failed: %v", err)
		}

		if out2.Text == "" {
			t.Error("expected text response after tool use")
		}

		// Verify the workflow.
		if mockModel.CallCount() != 2 {
			t.Errorf("expected 2 calls, got %d", mockModel.CallCount())
		}
	})

	t.Run("context cancellation stops workflow", func(t *testing.T) {
		mockModel := &model.MockChatModel{
			Responses: []model.ChatOut{
				{Text: "Response"},
			},
		}

		// Cancel context before call.
		cancelCtx, cancel := context.WithCancel(ctx)
		cancel()

		messages := []model.Message{
			{Role: model.RoleUser, Content: "Test"},
		}

		_, err := mockModel.Chat(cancelCtx, messages, nil)
		if err == nil {
			t.Fatal("expected context.Canceled error")
		}

		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}

		// Model should not have been called.
		if mockModel.CallCount() != 0 {
			t.Errorf("expected 0 calls with canceled context, got %d", mockModel.CallCount())
		}
	})

	t.Run("provider comparison for same task", func(t *testing.T) {
		// Setup multiple providers with different characteristics.
		fastProvider := &model.MockChatModel{
			Responses: []model.ChatOut{
				{Text: "Quick answer: 42"},
			},
		}

		detailedProvider := &model.MockChatModel{
			Responses: []model.ChatOut{
				{Text: "The answer is 42. This is derived from..."},
			},
		}

		messages := []model.Message{
			{Role: model.RoleUser, Content: "What is the answer?"},
		}

		// Get responses from both.
		quickOut, err := fastProvider.Chat(ctx, messages, nil)
		if err != nil {
			t.Fatalf("fast provider failed: %v", err)
		}

		detailedOut, err := detailedProvider.Chat(ctx, messages, nil)
		if err != nil {
			t.Fatalf("detailed provider failed: %v", err)
		}

		// Both should answer, but with different verbosity.
		if quickOut.Text == "" {
			t.Error("fast provider returned empty response")
		}

		if detailedOut.Text == "" {
			t.Error("detailed provider returned empty response")
		}

		if len(detailedOut.Text) <= len(quickOut.Text) {
			t.Error("expected detailed response to be longer")
		}
	})

	t.Run("retry logic with provider failure", func(t *testing.T) {
		// Provider that fails on first call.
		failingMock := &model.MockChatModel{
			Err: errors.New("temporary failure"),
		}

		// Provider that succeeds.
		successMock := &model.MockChatModel{
			Responses: []model.ChatOut{
				{Text: "Success after retry"},
			},
		}

		messages := []model.Message{
			{Role: model.RoleUser, Content: "Test"},
		}

		// Attempt 1: Fail.
		_, err := failingMock.Chat(ctx, messages, nil)
		if err == nil {
			t.Fatal("expected first attempt to fail")
		}

		// Attempt 2: Succeed (simulating retry with different provider or after retry delay).
		out, err := successMock.Chat(ctx, messages, nil)
		if err != nil {
			t.Fatalf("expected retry to succeed, got %v", err)
		}

		if out.Text != "Success after retry" {
			t.Errorf("unexpected response: %s", out.Text)
		}

		// Verify both were called.
		if failingMock.CallCount() != 1 {
			t.Errorf("expected 1 call to failing mock, got %d", failingMock.CallCount())
		}
		if successMock.CallCount() != 1 {
			t.Errorf("expected 1 call to success mock, got %d", successMock.CallCount())
		}
	})
}

// TestIntegration_ProviderSelectionStrategy tests logic for choosing the right provider (T150).
func TestIntegration_ProviderSelectionStrategy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	ctx := context.Background()

	t.Run("select based on task type", func(t *testing.T) {
		providers := map[string]model.ChatModel{
			"fast":     &model.MockChatModel{Responses: []model.ChatOut{{Text: "Fast"}}},
			"detailed": &model.MockChatModel{Responses: []model.ChatOut{{Text: "Detailed"}}},
			"coding":   &model.MockChatModel{Responses: []model.ChatOut{{Text: "Code"}}},
		}

		// Strategy: simple tasks → fast, reasoning → detailed, code → coding.
		selectProvider := func(taskType string) model.ChatModel {
			switch taskType {
			case "simple":
				return providers["fast"]
			case "reasoning":
				return providers["detailed"]
			case "code":
				return providers["coding"]
			default:
				return providers["fast"]
			}
		}

		messages := []model.Message{
			{Role: model.RoleUser, Content: "Test"},
		}

		// Test each strategy.
		tests := []struct {
			taskType         string
			expectedResponse string
		}{
			{"simple", "Fast"},
			{"reasoning", "Detailed"},
			{"code", "Code"},
			{"unknown", "Fast"}, // Default fallback
		}

		for _, tt := range tests {
			provider := selectProvider(tt.taskType)
			out, err := provider.Chat(ctx, messages, nil)
			if err != nil {
				t.Errorf("%s: unexpected error: %v", tt.taskType, err)
				continue
			}

			if out.Text != tt.expectedResponse {
				t.Errorf("%s: expected %q, got %q", tt.taskType, tt.expectedResponse, out.Text)
			}
		}
	})
}

// TestIntegration_EventTracingCapture verifies comprehensive event capture for 10-node workflow (T174).
//
// This integration test validates that the event tracing system correctly.
// captures all events from a 10-node workflow execution, including:
// - Node start/end events.
// - Routing decisions.
// - State changes.
// - Event filtering and querying.
func TestIntegration_EventTracingCapture(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Run("10-node sequential workflow with full event capture", func(t *testing.T) {
		// Create buffered emitter to capture events.
		emitter := emit.NewBufferedEmitter()

		// Define state.
		reducer := func(prev, delta TestState) TestState {
			if delta.Value != "" {
				prev.Value = delta.Value
			}
			prev.Counter += delta.Counter
			return prev
		}

		// Create engine.
		st := store.NewMemStore[TestState]()
		opts := Options{MaxSteps: 100}
		engine := New(reducer, st, emitter, opts)

		// Create 10 nodes that form a sequential chain.
		for i := 0; i < 10; i++ {
			nodeID := fmt.Sprintf("node%d", i)
			nextNode := fmt.Sprintf("node%d", i+1)
			if i == 9 {
				nextNode = "" // Last node stops
			}

			// Capture i in closure.
			currentValue := i
			currentNodeID := nodeID
			currentNextNode := nextNode

			node := NodeFunc[TestState](func(_ context.Context, _ TestState) NodeResult[TestState] {
				// Each node sets its value in the state.
				result := NodeResult[TestState]{
					Delta: TestState{Value: fmt.Sprintf("value%d", currentValue), Counter: 1},
				}

				if currentNextNode != "" {
					result.Route = Goto(currentNextNode)
				} else {
					result.Route = Stop()
				}

				return result
			})

			if err := engine.Add(currentNodeID, node); err != nil {
				t.Fatalf("Failed to add %s: %v", currentNodeID, err)
			}
		}

		if err := engine.StartAt("node0"); err != nil {
			t.Fatalf("Failed to set entry point: %v", err)
		}

		// Execute workflow.
		ctx := context.Background()
		initialState := TestState{}

		finalState, err := engine.Run(ctx, "integration-trace-001", initialState)
		if err != nil {
			t.Fatalf("Workflow execution failed: %v", err)
		}

		// Verify final state.
		if finalState.Counter != 10 {
			t.Errorf("expected Counter = 10, got %d", finalState.Counter)
		}
		if finalState.Value != "value9" {
			t.Errorf("expected Value = 'value9', got %q", finalState.Value)
		}

		// Test 1: Verify all node_start events captured.
		t.Run("captures all node_start events", func(t *testing.T) {
			nodeStarts := emitter.GetHistoryWithFilter("integration-trace-001", emit.HistoryFilter{Msg: "node_start"})
			if len(nodeStarts) != 10 {
				t.Errorf("expected 10 node_start events, got %d", len(nodeStarts))
			}

			// Verify node execution order.
			for i, event := range nodeStarts {
				expectedNodeID := fmt.Sprintf("node%d", i)
				if event.NodeID != expectedNodeID {
					t.Errorf("node_start %d: expected NodeID = %q, got %q", i, expectedNodeID, event.NodeID)
				}
				if event.Step != i {
					t.Errorf("node_start %d: expected Step = %d, got %d", i, i, event.Step)
				}
			}
		})

		// Test 2: Verify all node_end events captured.
		t.Run("captures all node_end events", func(t *testing.T) {
			nodeEnds := emitter.GetHistoryWithFilter("integration-trace-001", emit.HistoryFilter{Msg: "node_end"})
			if len(nodeEnds) != 10 {
				t.Errorf("expected 10 node_end events, got %d", len(nodeEnds))
			}

			// Verify each node_end has delta metadata.
			for i, event := range nodeEnds {
				if event.Meta == nil {
					t.Errorf("node_end %d: expected Meta to be non-nil", i)
					continue
				}
				if event.Meta["delta"] == nil {
					t.Errorf("node_end %d: expected delta in Meta", i)
				}
			}
		})

		// Test 3: Verify all routing_decision events captured.
		t.Run("captures all routing_decision events", func(t *testing.T) {
			routingDecisions := emitter.GetHistoryWithFilter("integration-trace-001", emit.HistoryFilter{Msg: "routing_decision"})
			if len(routingDecisions) != 10 {
				t.Errorf("expected 10 routing_decision events, got %d", len(routingDecisions))
			}

			// Verify routing decisions.
			for i, event := range routingDecisions {
				if event.Meta == nil {
					t.Errorf("routing_decision %d: expected Meta to be non-nil", i)
					continue
				}

				if i < 9 {
					// First 9 nodes route to the next node.
					expectedNext := fmt.Sprintf("node%d", i+1)
					if nextNode, ok := event.Meta["next_node"].(string); !ok || nextNode != expectedNext {
						t.Errorf("routing_decision %d: expected next_node = %q, got %v", i, expectedNext, event.Meta["next_node"])
					}
				} else {
					// Last node should be terminal.
					if terminal, ok := event.Meta["terminal"].(bool); !ok || !terminal {
						t.Errorf("routing_decision %d: expected terminal = true, got %v", i, event.Meta["terminal"])
					}
				}
			}
		})

		// Test 4: Verify total event count.
		t.Run("total event count", func(t *testing.T) {
			allEvents := emitter.GetHistory("integration-trace-001")
			// Should have: 10 node_start + 10 node_end + 10 routing_decision = 30 events.
			if len(allEvents) != 30 {
				t.Errorf("expected 30 total events, got %d", len(allEvents))
			}
		})

		// Test 5: Verify events are in chronological order.
		t.Run("events are in chronological order", func(t *testing.T) {
			allEvents := emitter.GetHistory("integration-trace-001")

			// Verify events are ordered by step and type.
			// Expected pattern for each step: node_start, node_end, routing_decision.
			for i := 0; i < 10; i++ {
				baseIdx := i * 3

				// Check node_start.
				if allEvents[baseIdx].Msg != "node_start" {
					t.Errorf("step %d: expected node_start at index %d, got %s", i, baseIdx, allEvents[baseIdx].Msg)
				}
				if allEvents[baseIdx].Step != i {
					t.Errorf("step %d: node_start has wrong step number: %d", i, allEvents[baseIdx].Step)
				}

				// Check node_end.
				if allEvents[baseIdx+1].Msg != "node_end" {
					t.Errorf("step %d: expected node_end at index %d, got %s", i, baseIdx+1, allEvents[baseIdx+1].Msg)
				}
				if allEvents[baseIdx+1].Step != i {
					t.Errorf("step %d: node_end has wrong step number: %d", i, allEvents[baseIdx+1].Step)
				}

				// Check routing_decision.
				if allEvents[baseIdx+2].Msg != "routing_decision" {
					t.Errorf("step %d: expected routing_decision at index %d, got %s", i, baseIdx+2, allEvents[baseIdx+2].Msg)
				}
				if allEvents[baseIdx+2].Step != i {
					t.Errorf("step %d: routing_decision has wrong step number: %d", i, allEvents[baseIdx+2].Step)
				}
			}
		})

		// Test 6: Verify filtering by step range works.
		t.Run("filtering by step range works", func(t *testing.T) {
			// Get events from steps 3-7.
			minStep := 3
			maxStep := 7
			filter := emit.HistoryFilter{MinStep: &minStep, MaxStep: &maxStep}
			filteredEvents := emitter.GetHistoryWithFilter("integration-trace-001", filter)

			// Should have 5 steps * 3 events per step = 15 events.
			if len(filteredEvents) != 15 {
				t.Errorf("expected 15 events for steps 3-7, got %d", len(filteredEvents))
			}

			// Verify all events are within the step range.
			for _, event := range filteredEvents {
				if event.Step < 3 || event.Step > 7 {
					t.Errorf("found event outside step range 3-7: step %d", event.Step)
				}
			}
		})

		// Test 7: Verify filtering by node ID works.
		t.Run("filtering by node ID works", func(t *testing.T) {
			// Get all events from node5.
			filter := emit.HistoryFilter{NodeID: "node5"}
			node5Events := emitter.GetHistoryWithFilter("integration-trace-001", filter)

			// Should have: node_start, node_end, routing_decision = 3 events.
			if len(node5Events) != 3 {
				t.Errorf("expected 3 events for node5, got %d", len(node5Events))
			}

			// Verify all events are from node5.
			for _, event := range node5Events {
				if event.NodeID != "node5" {
					t.Errorf("expected NodeID = 'node5', got %q", event.NodeID)
				}
			}
		})

		t.Log("Integration test passed: 10-node workflow with comprehensive event tracing")
	})
}

// TestQueueDepthMetrics (T062) verifies metrics tracking for queue depth and concurrency.
//
// According to spec.md FR-027: System MUST expose metrics for queue_depth, step_latency_ms,
// active_nodes, and step_count.
//
// Requirements:
// - Queue depth is tracked as nodes are enqueued/dequeued.
// - Active node count reflects concurrent execution.
// - Metrics are accessible for monitoring and observability.
// - Metrics remain accurate under high concurrency.
//
// This test creates a workflow with fan-out and monitors metrics throughout execution.
func TestQueueDepthMetrics(t *testing.T) {
	t.Run("tracks queue depth and active nodes during execution", func(t *testing.T) {
		// Note: This test will be fully implemented after T067-T068 add SchedulerMetrics.
		// For now, we verify that the structure for metrics exists.

		t.Skip("SchedulerMetrics implementation pending (T067-T068)")

		// Expected test flow:
		// 1. Create engine with metrics tracking enabled.
		// 2. Create workflow with fan-out (spawn 10 parallel branches).
		// 3. Execute workflow and collect metrics snapshots.
		// 4. Verify queue_depth increases when nodes spawn work.
		// 5. Verify queue_depth decreases as work is consumed.
		// 6. Verify active_nodes stays within MaxConcurrentNodes limit.
		// 7. Verify step_count increments correctly.
		// 8. Verify metrics are accessible via Frontier.Metrics() or similar API.
	})

	t.Run("metrics accurately reflect backpressure conditions", func(t *testing.T) {
		// Note: This test will be implemented after backpressure metrics are added.

		t.Skip("Backpressure metrics pending (T068-T069)")

		// Expected test flow:
		// 1. Create engine with small QueueDepth.
		// 2. Create nodes that produce work faster than consumed.
		// 3. Monitor queue_depth metric as it reaches capacity.
		// 4. Verify metrics show queue is full when backpressure triggers.
		// 5. Verify backpressure event is emitted with correct metrics.
		// 6. Verify queue_depth decreases when work is consumed.
	})

	t.Run("metrics remain accurate under high concurrency", func(t *testing.T) {
		// Note: This test will verify thread-safe metric updates.

		t.Skip("Concurrent metrics validation pending (T067-T068)")

		// Expected test flow:
		// 1. Create engine with MaxConcurrentNodes=10.
		// 2. Spawn 100 parallel work items.
		// 3. Collect metrics snapshots from multiple goroutines.
		// 4. Verify no race conditions in metric updates (run with -race).
		// 5. Verify final metrics match expected values.
		// 6. Verify active_nodes never exceeds MaxConcurrentNodes.
	})
}
