# Checkpoints & Resume

This guide covers checkpoint-based persistence, workflow resumption, and advanced patterns for building durable, fault-tolerant workflows.

## Overview

LangGraph-Go provides built-in checkpointing to enable:

- **Automatic Resumption**: Resume from the last successful step after crashes
- **Manual Checkpoints**: Save workflow snapshots at critical milestones
- **Branching Workflows**: Explore multiple paths from the same checkpoint
- **Replay & Debugging**: Reconstruct workflow history for analysis
- **Long-Running Workflows**: Persist state across process restarts

## How Checkpointing Works

The `Store[S]` interface provides persistence for workflow state:

```go
type Store[S any] interface {
    // Automatic step tracking
    SaveStep(ctx context.Context, runID string, step int, nodeID string, state S) error
    LoadLatest(ctx context.Context, runID string) (state S, step int, err error)

    // Manual checkpoints
    SaveCheckpoint(ctx context.Context, cpID string, state S, step int) error
    LoadCheckpoint(ctx context.Context, cpID string) (state S, step int, err error)
}
```

### Automatic Step Persistence

Every node execution automatically saves state:

```go
store := store.NewMemStore[State]()
engine := graph.New(reducer, store, emitter, opts)

// Each node execution triggers SaveStep
engine.Run(ctx, "run-001", initialState)

// State is saved after every step:
// - Step 1: nodeA completes → SaveStep("run-001", 1, "nodeA", stateAfterA)
// - Step 2: nodeB completes → SaveStep("run-001", 2, "nodeB", stateAfterB)
// - Step 3: nodeC completes → SaveStep("run-001", 3, "nodeC", stateAfterC)
```

### Manual Checkpoints

Create named snapshots at critical points:

```go
// Inside a node
func checkpointNode(ctx context.Context, s State) graph.NodeResult[State] {
    // Do some work
    result := processData(s)

    // Save a named checkpoint
    cpID := fmt.Sprintf("run-%s-validated", s.RunID)
    store.SaveCheckpoint(ctx, cpID, s, s.StepCount)

    return graph.NodeResult[State]{
        Delta: State{Result: result},
        Route: graph.Goto("next"),
    }
}
```

## Resuming Workflows

### Basic Resumption

Resume from the last saved step:

```go
func resumeWorkflow(runID string) (State, error) {
    store := store.NewMemStore[State]()
    engine := graph.New(reducer, store, emitter, opts)

    // Load latest state
    lastState, lastStep, err := store.LoadLatest(ctx, runID)
    if err == store.ErrNotFound {
        // No previous run found, start fresh
        return engine.Run(ctx, runID, initialState)
    }
    if err != nil {
        return State{}, err
    }

    fmt.Printf("Resuming from step %d\n", lastStep)

    // Resume execution
    return engine.Run(ctx, runID, lastState)
}
```

### Checkpoint-Based Resumption

Resume from a specific checkpoint:

```go
func resumeFromCheckpoint(cpID string) (State, error) {
    store := store.NewMemStore[State]()
    engine := graph.New(reducer, store, emitter, opts)

    // Load checkpoint state
    cpState, cpStep, err := store.LoadCheckpoint(ctx, cpID)
    if err != nil {
        return State{}, err
    }

    fmt.Printf("Resuming from checkpoint '%s' at step %d\n", cpID, cpStep)

    // Create new run from checkpoint
    newRunID := fmt.Sprintf("%s-resumed-%d", cpID, time.Now().Unix())
    return engine.Run(ctx, newRunID, cpState)
}
```

## Store Implementations

### In-Memory Store (Development/Testing)

Fast, simple, no persistence across restarts:

```go
import "github.com/dshills/langgraph-go/graph/store"

// Create in-memory store
st := store.NewMemStore[State]()

// Can serialize to JSON for debugging
data, _ := st.MarshalJSON()
os.WriteFile("workflow-state.json", data, 0644)

// Load from JSON
st2 := store.NewMemStore[State]()
data, _ := os.ReadFile("workflow-state.json")
st2.UnmarshalJSON(data)
```

**Use Cases**:
- Unit tests
- Local development
- Short-lived workflows
- Prototyping

**Limitations**:
- Data lost on process termination
- Not suitable for distributed systems
- Memory usage grows with history

### Database Stores (Production)

Persistent, distributed, durable:

```go
// MySQL/Aurora (recommended for production)
import "github.com/dshills/langgraph-go/graph/store/mysql"

mysqlStore, err := mysql.NewMySQLStore[State](
    "user:pass@tcp(localhost:3306)/workflows",
)
if err != nil {
    log.Fatal(err)
}
defer mysqlStore.Close()

engine := graph.New(reducer, mysqlStore, emitter, opts)
```

**Use Cases**:
- Production workflows
- Distributed systems
- Long-running processes
- Auditing and compliance

**Benefits**:
- Survives process restarts
- Supports concurrent workers
- Query workflow history via SQL
- Built-in replication and backups

## Checkpoint Patterns

### Pattern 1: Milestone Checkpoints

Save at key workflow milestones:

```go
type State struct {
    Phase string
    Data  string
}

// Validation milestone
func validateNode(ctx context.Context, s State) graph.NodeResult[State] {
    if !isValid(s.Data) {
        return graph.NodeResult[State]{Err: errors.New("validation failed")}
    }

    // Save checkpoint after validation
    store.SaveCheckpoint(ctx, "checkpoint-validated", s, s.StepCount)

    return graph.NodeResult[State]{
        Delta: State{Phase: "validated"},
        Route: graph.Goto("process"),
    }
}

// Processing milestone
func processNode(ctx context.Context, s State) graph.NodeResult[State] {
    result := expensiveProcessing(s.Data)

    // Save checkpoint after processing
    store.SaveCheckpoint(ctx, "checkpoint-processed", s, s.StepCount)

    return graph.NodeResult[State]{
        Delta: State{Data: result, Phase: "processed"},
        Route: graph.Goto("finalize"),
    }
}
```

### Pattern 2: Branching Workflows

Explore multiple paths from the same checkpoint:

```go
// Save checkpoint before branching
func branchPoint(ctx context.Context, s State) graph.NodeResult[State] {
    cpID := fmt.Sprintf("branch-point-%d", time.Now().Unix())
    store.SaveCheckpoint(ctx, cpID, s, s.StepCount)

    return graph.NodeResult[State]{
        Delta: State{BranchCheckpoint: cpID},
        Route: graph.Goto("path-a"),
    }
}

// Later: Try path B from the same checkpoint
func retryDifferentPath(cpID string) (State, error) {
    state, step, err := store.LoadCheckpoint(ctx, cpID)
    if err != nil {
        return State{}, err
    }

    // Modify state to take path B
    state.PreferredPath = "path-b"

    // Run from checkpoint
    return engine.Run(ctx, "path-b-attempt", state)
}
```

### Pattern 3: User-Triggered Checkpoints

Allow users to save progress manually:

```go
type State struct {
    UserID      string
    Progress    float64
    CanSave     bool
    LastSaveID  string
}

func saveProgressNode(ctx context.Context, s State) graph.NodeResult[State] {
    if !s.CanSave {
        return graph.NodeResult[State]{
            Route: graph.Goto("next"),
        }
    }

    // Create user-friendly checkpoint ID
    cpID := fmt.Sprintf("user-%s-save-%d", s.UserID, time.Now().Unix())

    store.SaveCheckpoint(ctx, cpID, s, s.StepCount)

    fmt.Printf("Progress saved! Load with checkpoint ID: %s\n", cpID)

    return graph.NodeResult[State]{
        Delta: State{LastSaveID: cpID},
        Route: graph.Goto("next"),
    }
}
```

### Pattern 4: Error Recovery Checkpoints

Save before risky operations:

```go
func riskyOperation(ctx context.Context, s State) graph.NodeResult[State] {
    // Save checkpoint before risky operation
    cpID := fmt.Sprintf("before-risky-%d", time.Now().Unix())
    store.SaveCheckpoint(ctx, cpID, s, s.StepCount)

    // Attempt risky operation
    result, err := callExternalAPI(s.Data)
    if err != nil {
        return graph.NodeResult[State]{
            Delta: State{
                LastError:       err.Error(),
                RecoveryPoint:   cpID,
            },
            Route: graph.Goto("error-handler"),
        }
    }

    return graph.NodeResult[State]{
        Delta: State{Result: result},
        Route: graph.Goto("next"),
    }
}

// Error handler can reload checkpoint and retry
func errorHandler(ctx context.Context, s State) graph.NodeResult[State] {
    if s.RecoveryPoint == "" {
        return graph.NodeResult[State]{
            Err: errors.New("no recovery point available"),
        }
    }

    // Load pre-error state
    recoveryState, _, err := store.LoadCheckpoint(ctx, s.RecoveryPoint)
    if err != nil {
        return graph.NodeResult[State]{Err: err}
    }

    // Decide: retry or fail
    if s.RetryCount < 3 {
        return graph.NodeResult[State]{
            Delta: State{RetryCount: s.RetryCount + 1},
            Route: graph.Goto("risky-operation"),
        }
    }

    return graph.NodeResult[State]{
        Delta: State{Status: "failed"},
        Route: graph.Stop(),
    }
}
```

## Advanced Resumption Patterns

### Resume with State Modification

Modify state before resuming:

```go
func resumeWithFix(runID string, fixData string) (State, error) {
    // Load latest state
    state, step, err := store.LoadLatest(ctx, runID)
    if err != nil {
        return State{}, err
    }

    // Apply fix to state
    state.Data = fixData
    state.FixApplied = true

    fmt.Printf("Resuming from step %d with fix applied\n", step)

    // Resume with modified state
    return engine.Run(ctx, runID, state)
}
```

### Conditional Resumption

Resume only if certain conditions are met:

```go
func smartResume(runID string) (State, error) {
    state, step, err := store.LoadLatest(ctx, runID)
    if err == store.ErrNotFound {
        // Start fresh
        return engine.Run(ctx, runID, initialState)
    }
    if err != nil {
        return State{}, err
    }

    // Check if resumption makes sense
    if state.IsComplete {
        fmt.Println("Workflow already complete")
        return state, nil
    }

    if state.IsFailed {
        return State{}, errors.New("workflow failed, cannot resume")
    }

    if time.Since(state.LastUpdate) > 24*time.Hour {
        return State{}, errors.New("state too old, starting fresh")
    }

    // Looks good, resume
    fmt.Printf("Resuming from step %d\n", step)
    return engine.Run(ctx, runID, state)
}
```

### Multi-Stage Resumption

Resume through multiple checkpoints:

```go
func resumeMultiStage(runID string) (State, error) {
    // Try to load most recent checkpoint
    checkpoints := []string{
        "checkpoint-finalized",
        "checkpoint-processed",
        "checkpoint-validated",
    }

    for _, cpID := range checkpoints {
        fullCpID := fmt.Sprintf("%s-%s", runID, cpID)
        state, step, err := store.LoadCheckpoint(ctx, fullCpID)
        if err == store.ErrNotFound {
            continue // Try next checkpoint
        }
        if err != nil {
            return State{}, err
        }

        fmt.Printf("Resuming from checkpoint %s at step %d\n", cpID, step)
        return engine.Run(ctx, runID, state)
    }

    // No checkpoints found, start from beginning
    return engine.Run(ctx, runID, initialState)
}
```

## Persistence Best Practices

### 1. Use Unique Run IDs

Ensure run IDs are globally unique:

```go
// ❌ BAD: Can collide
runID := "workflow-001"

// ✅ GOOD: Unique with timestamp
runID := fmt.Sprintf("workflow-%s-%d", userID, time.Now().UnixNano())

// ✅ GOOD: UUID
runID := uuid.New().String()
```

### 2. Clean Up Old Data

Prevent unbounded storage growth:

```go
// Periodic cleanup (implement in your Store)
func cleanupOldRuns(store Store[State]) error {
    cutoff := time.Now().Add(-30 * 24 * time.Hour)

    // Delete runs older than 30 days
    return store.DeleteRunsBefore(cutoff)
}
```

### 3. Validate State Before Resuming

Ensure state is still valid:

```go
func validateState(s State) error {
    if s.Version < MinSupportedVersion {
        return errors.New("state version too old")
    }

    if s.SchemaVersion != CurrentSchemaVersion {
        return errors.New("state schema mismatch")
    }

    return nil
}

func safeResume(runID string) (State, error) {
    state, _, err := store.LoadLatest(ctx, runID)
    if err != nil {
        return State{}, err
    }

    if err := validateState(state); err != nil {
        return State{}, fmt.Errorf("invalid state: %w", err)
    }

    return engine.Run(ctx, runID, state)
}
```

### 4. Handle Store Failures Gracefully

Don't crash on persistence errors:

```go
func resilientNode(ctx context.Context, s State) graph.NodeResult[State] {
    result := doWork(s)

    // Try to save checkpoint, but don't fail if it errors
    cpID := fmt.Sprintf("checkpoint-%d", time.Now().Unix())
    if err := store.SaveCheckpoint(ctx, cpID, s, s.StepCount); err != nil {
        log.Printf("WARNING: Failed to save checkpoint: %v", err)
        // Continue anyway
    }

    return graph.NodeResult[State]{
        Delta: State{Result: result},
        Route: graph.Goto("next"),
    }
}
```

## Debugging with Checkpoints

### Replay Workflow History

Reconstruct execution flow:

```go
func replayWorkflow(runID string) error {
    // Get all steps (requires extended Store interface)
    steps, err := store.GetAllSteps(ctx, runID)
    if err != nil {
        return err
    }

    fmt.Printf("Replaying %d steps for run %s\n", len(steps), runID)

    for _, step := range steps {
        fmt.Printf("Step %d: %s\n", step.Step, step.NodeID)
        fmt.Printf("  State: %+v\n", step.State)
    }

    return nil
}
```

### Compare Checkpoint States

Analyze state evolution:

```go
func compareCheckpoints(cpID1, cpID2 string) error {
    state1, _, err := store.LoadCheckpoint(ctx, cpID1)
    if err != nil {
        return err
    }

    state2, _, err := store.LoadCheckpoint(ctx, cpID2)
    if err != nil {
        return err
    }

    fmt.Printf("Checkpoint %s:\n%+v\n\n", cpID1, state1)
    fmt.Printf("Checkpoint %s:\n%+v\n\n", cpID2, state2)

    // Use diff library to show changes
    return nil
}
```

## Testing Resumption Logic

Test that workflows resume correctly:

```go
func TestWorkflowResumption(t *testing.T) {
    store := store.NewMemStore[State]()
    engine := graph.New(reducer, store, emitter, opts)

    // Run workflow partially
    initialState := State{Data: "test"}
    _, err := engine.Run(ctx, "test-run", initialState)
    if err != nil {
        t.Fatal(err)
    }

    // Load latest state
    resumedState, step, err := store.LoadLatest(ctx, "test-run")
    if err != nil {
        t.Fatal(err)
    }

    // Verify state was persisted
    if resumedState.Data != "test-processed" {
        t.Errorf("wrong state: %+v", resumedState)
    }

    if step != 3 {
        t.Errorf("expected step 3, got %d", step)
    }

    // Resume from latest
    finalState, err := engine.Run(ctx, "test-run", resumedState)
    if err != nil {
        t.Fatal(err)
    }

    if !finalState.IsComplete {
        t.Error("workflow should be complete after resumption")
    }
}
```

---

**Next:** Learn dynamic control flow with [Conditional Routing](./05-routing.md) →
