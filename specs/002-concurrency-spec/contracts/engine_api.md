# Engine API Contract: Concurrent Execution

**Feature**: Concurrent Graph Execution with Deterministic Replay
**Package**: `github.com/dshills/langgraph-go/graph`
**Version**: v0.2.0 (requires MINOR bump due to new functionality)

## Overview

This document defines the Go API contracts for concurrent graph execution. These are programmatic interfaces, not REST/GraphQL endpoints. The concurrency features extend the existing `Engine[S]` type with new configuration options and enhanced Store/Emitter interfaces.

---

## Types

### Options (Enhanced)

```go
// Options configures Engine execution behavior
type Options struct {
    // Existing fields
    MaxSteps int    // Maximum workflow steps (default: 0 = unlimited)
    Retries  int    // Node retry attempts (deprecated, use NodePolicy.RetryPolicy)

    // NEW: Concurrency configuration
    MaxConcurrentNodes   int           // Max parallel node execution (default: 8)
    QueueDepth           int           // Frontier queue capacity (default: 1024)
    BackpressureTimeout  time.Duration // Queue full timeout (default: 30s)
    DefaultNodeTimeout   time.Duration // Per-node timeout (default: 30s)
    RunWallClockBudget   time.Duration // Total run timeout (default: 10m)

    // NEW: Replay configuration
    ReplayMode           bool          // Enable replay from recorded I/O (default: false)
    StrictReplay         bool          // Fail on I/O mismatch (default: true)
}
```

**Backward Compatibility**: All new fields have sensible defaults. Existing code works without changes.

---

### WorkItem (NEW)

```go
// WorkItem represents a schedulable unit in the execution frontier
type WorkItem[S any] struct {
    StepID       int    // Step number in run
    OrderKey     uint64 // Deterministic sort key
    NodeID       string // Node to execute
    State        S      // State snapshot
    Attempt      int    // Retry counter (0 = first attempt)
    ParentNodeID string // Node that spawned this work
    EdgeIndex    int    // Edge index from parent
}
```

**Usage**: Internal to engine scheduler. Not directly created by users.

---

### NodePolicy (NEW)

```go
// NodePolicy configures node execution behavior
type NodePolicy struct {
    Timeout            time.Duration      // Max execution time (0 = use Options.DefaultNodeTimeout)
    RetryPolicy        *RetryPolicy       // Retry configuration (nil = no retries)
    IdempotencyKeyFunc func(S any) string // Custom idempotency key (nil = use default)
}

// Node interface (enhanced)
type Node[S any] interface {
    Run(ctx context.Context, state S) NodeResult[S]

    // NEW: Optional policy configuration
    Policy() NodePolicy // Return zero value for defaults
}
```

**Backward Compatibility**: Existing nodes without `Policy()` method continue to work. Interface is satisfied by embedding `DefaultPolicy` struct:

```go
type MyNode struct {
    DefaultPolicy // Embeds Policy() method returning zero value
}
```

---

### RetryPolicy (NEW)

```go
// RetryPolicy configures automatic retry behavior
type RetryPolicy struct {
    MaxAttempts int                    // Max attempts (including initial) (default: 3)
    BaseDelay   time.Duration          // Initial backoff delay (default: 1s)
    MaxDelay    time.Duration          // Maximum backoff cap (default: 30s)
    Retryable   func(error) bool       // Predicate for retryable errors (default: all errors)
}

// Example usage
policy := &RetryPolicy{
    MaxAttempts: 5,
    BaseDelay:   time.Second,
    MaxDelay:    time.Minute,
    Retryable: func(err error) bool {
        // Only retry on specific errors
        return strings.Contains(err.Error(), "timeout") ||
               strings.Contains(err.Error(), "rate limit")
    },
}
```

---

### SideEffectPolicy (NEW)

```go
// SideEffectPolicy declares node's I/O characteristics for replay
type SideEffectPolicy struct {
    Recordable          bool // Can I/O be captured? (default: false)
    RequiresIdempotency bool // Requires idempotency key? (default: false)
}

// Node interface (enhanced)
type Node[S any] interface {
    Run(ctx context.Context, state S) NodeResult[S]
    Policy() NodePolicy

    // NEW: Optional side effect declaration
    Effects() SideEffectPolicy // Return zero value for pure nodes
}
```

**Usage Example**:
```go
// LLM node with recordable I/O
func (n *LLMNode) Effects() SideEffectPolicy {
    return SideEffectPolicy{
        Recordable:          true,
        RequiresIdempotency: true,
    }
}
```

---

### Checkpoint (NEW)

```go
// Checkpoint represents a durable execution snapshot
type Checkpoint[S any] struct {
    RunID           string        // Unique run identifier
    StepID          int           // Step number at checkpoint
    State           S             // Current state
    Frontier        []WorkItem[S] // Pending work items
    RNGSeed         int64         // Seeded RNG state
    RecordedIOs     []RecordedIO  // Captured I/O for replay
    IdempotencyKey  string        // Duplicate prevention key
    Timestamp       time.Time     // Creation time
    Label           string        // Optional user label
}
```

---

### RecordedIO (NEW)

```go
// RecordedIO captures external interaction for replay
type RecordedIO struct {
    NodeID    string          // Node that performed I/O
    Attempt   int             // Retry attempt number
    Request   json.RawMessage // Serialized request
    Response  json.RawMessage // Serialized response
    Hash      string          // SHA-256 of response for verification
    Timestamp time.Time       // When I/O occurred
    Duration  time.Duration   // How long I/O took
}
```

---

## Enhanced Store Interface

```go
// Store persists workflow state and checkpoints
type Store[S any] interface {
    // Existing methods (unchanged)
    SaveStep(ctx context.Context, runID string, step int, nodeID string, state S) error
    LoadLatest(ctx context.Context, runID string) (state S, step int, nodeID string, _ error)

    // Existing checkpoint methods (unchanged)
    SaveCheckpoint(ctx context.Context, runID, label string, state S, step int, nodeID string) error
    LoadCheckpoint(ctx context.Context, runID, label string) (state S, step int, nodeID string, _ error)

    // NEW: Enhanced checkpoint with full context
    SaveCheckpointV2(ctx context.Context, checkpoint Checkpoint[S]) error
    LoadCheckpointV2(ctx context.Context, runID string, stepID int) (Checkpoint[S], error)
    LoadCheckpointByLabel(ctx context.Context, runID, label string) (Checkpoint[S], error)

    // NEW: Idempotency check
    CheckIdempotency(ctx context.Context, key string) (exists bool, _ error)

    // NEW: Transactional outbox (optional, for event exactly-once)
    PendingEvents(ctx context.Context, limit int) ([]Event, error)
    MarkEventsEmitted(ctx context.Context, eventIDs []string) error
}
```

**Migration Path**: New methods have default implementations in `store/memory.go`. Existing stores can opt-in to enhanced features.

---

## Enhanced Emitter Interface

```go
// Emitter receives observability events
type Emitter interface {
    // Existing method (unchanged)
    Emit(ctx context.Context, event Event) error

    // NEW: Batch emission for performance
    EmitBatch(ctx context.Context, events []Event) error

    // NEW: Flush pending events
    Flush(ctx context.Context) error
}

// Event types (NEW)
const (
    EventStepStart      = "step.start"
    EventStepComplete   = "step.complete"
    EventBackpressure   = "backpressure.block"
    EventRetryAttempt   = "node.retry"
    EventReplayMismatch = "replay.mismatch"
    EventCancellation   = "run.cancelled"
)
```

---

## Engine Methods (Enhanced)

### Run (Enhanced)

```go
// Run executes the workflow from start node
// ENHANCED: Now supports concurrent execution and replay
func (e *Engine[S]) Run(ctx context.Context, runID string, initial S) (S, error)
```

**Behavior Changes**:
- Executes nodes concurrently up to `Options.MaxConcurrentNodes`
- Respects context cancellation and `Options.RunWallClockBudget`
- Automatically checkpoints at each step
- Uses recorded I/O if `Options.ReplayMode = true`

**Backward Compatibility**: Existing code continues to work. Concurrency disabled if `MaxConcurrentNodes = 0` (sequential execution).

---

### RunWithCheckpoint (NEW)

```go
// RunWithCheckpoint resumes execution from a specific checkpoint
func (e *Engine[S]) RunWithCheckpoint(ctx context.Context, checkpoint Checkpoint[S]) (S, error)
```

**Usage Example**:
```go
// Save checkpoint
engine.Run(ctx, "run-001", initialState)

// Later: resume from checkpoint
checkpoint, _ := store.LoadCheckpointByLabel(ctx, "run-001", "before_summary")
finalState, err := engine.RunWithCheckpoint(ctx, checkpoint)
```

---

### ReplayRun (NEW)

```go
// ReplayRun replays a previous execution using recorded I/O
func (e *Engine[S]) ReplayRun(ctx context.Context, runID string) (S, error)
```

**Usage Example**:
```go
// Original execution with recording
opts := Options{ReplayMode: false}  // Record I/O
engine := New(reducer, store, emitter, opts)
engine.Run(ctx, "run-001", initialState)

// Replay without hitting external services
replayOpts := Options{ReplayMode: true, StrictReplay: true}
replayEngine := New(reducer, store, emitter, replayOpts)
replayedState, err := replayEngine.ReplayRun(ctx, "run-001")
```

---

## Context Values (NEW)

Engine propagates metadata via context.Context:

```go
// Context keys (exported for node access)
const (
    RunIDKey      = "langgraph.run_id"
    StepIDKey     = "langgraph.step_id"
    NodeIDKey     = "langgraph.node_id"
    OrderKeyKey   = "langgraph.order_key"
    AttemptKey    = "langgraph.attempt"
    RNGKey        = "langgraph.rng"
)

// Example: Node accessing context values
func (n *MyNode) Run(ctx context.Context, state MyState) NodeResult[MyState] {
    runID := ctx.Value(RunIDKey).(string)
    attempt := ctx.Value(AttemptKey).(int)
    rng := ctx.Value(RNGKey).(*rand.Rand)

    randomValue := rng.Intn(100)  // Deterministic across replays
    // ...
}
```

---

## Error Types (NEW)

```go
// ErrReplayMismatch indicates recorded I/O doesn't match live execution
var ErrReplayMismatch = errors.New("replay mismatch: recorded I/O hash mismatch")

// ErrNoProgress indicates topology deadlock
var ErrNoProgress = errors.New("no progress: no runnable nodes in frontier")

// ErrBackpressureTimeout indicates queue full for too long
var ErrBackpressureTimeout = errors.New("backpressure timeout: frontier queue full")

// ErrIdempotencyViolation indicates duplicate step commit attempt
var ErrIdempotencyViolation = errors.New("idempotency violation: checkpoint already committed")

// ErrMaxAttemptsExceeded indicates retry limit reached
var ErrMaxAttemptsExceeded = errors.New("max retry attempts exceeded")
```

---

## Usage Examples

### Example 1: Basic Concurrent Execution

```go
// Configure concurrent execution
opts := graph.Options{
    MaxSteps:           100,
    MaxConcurrentNodes: 10,   // Run 10 nodes in parallel
    QueueDepth:         1024,
    DefaultNodeTimeout: 30 * time.Second,
}

// Create engine
reducer := func(prev, delta MyState) MyState {
    // Merge logic
    return merged
}

store := store.NewMemStore[MyState]()
emitter := emit.NewLogEmitter()
engine := graph.New(reducer, store, emitter, opts)

// Add nodes
engine.Add("fetch_user", fetchUserNode)
engine.Add("fetch_orders", fetchOrdersNode)
engine.Add("merge_data", mergeNode)

// Add edges with fan-out
engine.AddEdge("start", "fetch_user", nil)
engine.AddEdge("start", "fetch_orders", nil)  // Parallel
engine.AddEdge("fetch_user", "merge_data", nil)
engine.AddEdge("fetch_orders", "merge_data", nil)

// Execute
engine.StartAt("start")
final, err := engine.Run(ctx, "run-001", initialState)
```

---

### Example 2: Node with Retry Policy

```go
type APINode struct {
    graph.DefaultPolicy
    client *http.Client
}

func (n *APINode) Policy() graph.NodePolicy {
    return graph.NodePolicy{
        Timeout: 10 * time.Second,
        RetryPolicy: &graph.RetryPolicy{
            MaxAttempts: 3,
            BaseDelay:   time.Second,
            MaxDelay:    10 * time.Second,
            Retryable: func(err error) bool {
                return strings.Contains(err.Error(), "timeout") ||
                       strings.Contains(err.Error(), "503")
            },
        },
    }
}

func (n *APINode) Run(ctx context.Context, state MyState) graph.NodeResult[MyState] {
    resp, err := n.client.Get(state.URL)
    if err != nil {
        return graph.NodeResult[MyState]{Err: err}  // Will retry
    }
    // Process response...
}
```

---

### Example 3: Deterministic Replay

```go
// Original execution (records I/O)
opts := graph.Options{ReplayMode: false}
engine := graph.New(reducer, store, emitter, opts)
final, err := engine.Run(ctx, "run-001", initialState)

// ... time passes, need to debug ...

// Replay exact execution without external calls
replayOpts := graph.Options{
    ReplayMode:   true,
    StrictReplay: true,  // Fail if any I/O mismatch
}
replayEngine := graph.New(reducer, store, emitter, replayOpts)
replayedFinal, err := replayEngine.ReplayRun(ctx, "run-001")

// replayedFinal should exactly match final
```

---

### Example 4: Checkpoint Save and Resume

```go
// Save checkpoint mid-execution
engine.Run(ctx, "run-001", initialState)
// Engine auto-saves checkpoints at each step

// Load and resume later
checkpoint, err := store.LoadCheckpointByLabel(ctx, "run-001", "step_5")
if err != nil {
    log.Fatal(err)
}

final, err := engine.RunWithCheckpoint(ctx, checkpoint)
```

---

## Breaking Changes

None. All enhancements are backward compatible:
- New Options fields have defaults
- New Node methods are optional (default implementations provided)
- Existing Store interface unchanged (V2 methods are additions)
- Sequential execution preserved when `MaxConcurrentNodes = 0`

---

## Migration Guide

### From v0.1.x to v0.2.0

1. **No changes required** for basic usage - existing code continues to work
2. **Opt-in to concurrency**: Set `Options.MaxConcurrentNodes > 0`
3. **Add retry policies**: Implement `Node.Policy()` for automatic retries
4. **Enable replay**: Use `Options.ReplayMode = true` and implement `Node.Effects()`
5. **Use enhanced checkpoints**: Call `Store.SaveCheckpointV2()` for full context

### Example Migration

**Before (v0.1.x)**:
```go
opts := graph.Options{MaxSteps: 100}
engine := graph.New(reducer, store, emitter, opts)
final, err := engine.Run(ctx, "run-001", initialState)
```

**After (v0.2.0 with concurrency)**:
```go
opts := graph.Options{
    MaxSteps:           100,
    MaxConcurrentNodes: 10,  // NEW: Enable concurrency
}
engine := graph.New(reducer, store, emitter, opts)
final, err := engine.Run(ctx, "run-001", initialState)  // Same call!
```

---

## Testing Contracts

### Test Helpers (NEW)

```go
// NewTestEngine creates engine with concurrency disabled for deterministic tests
func NewTestEngine[S any](reducer Reducer[S], opts Options) *Engine[S] {
    opts.MaxConcurrentNodes = 0  // Force sequential
    return New(reducer, store.NewMemStore[S](), emit.NewLogEmitter(), opts)
}

// NewConcurrentTestEngine creates engine for concurrency testing
func NewConcurrentTestEngine[S any](reducer Reducer[S], concurrency int) *Engine[S] {
    opts := Options{MaxConcurrentNodes: concurrency}
    return New(reducer, store.NewMemStore[S](), emit.NewLogEmitter(), opts)
}
```

---

## Performance Considerations

### Concurrency Tuning

- `MaxConcurrentNodes`: Start with CPU count, tune based on I/O vs CPU workload
- `QueueDepth`: Multiply `MaxConcurrentNodes` by 100 for initial estimate
- `BackpressureTimeout`: 30s works for most workloads, reduce for fast-failing systems

### Memory Usage

- Each work item holds state copy (fan-out creates copies)
- Frontier queue bounded by `QueueDepth`
- Recorded I/O accumulates per checkpoint (monitor size)

### Replay Performance

- Replay is typically 100x faster than original execution (no external I/O)
- Large recorded I/O datasets may impact deserialization time
- Consider checkpoint compression for long-running workflows

---

## Security Considerations

- Recorded I/O may contain sensitive data (encrypt at rest)
- Idempotency keys use SHA-256 (cryptographically secure)
- Context cancellation propagates within 1 second (cannot force goroutine termination)
- Store implementations must validate input (SQL injection prevention)

---

## Summary

This API contract provides:
- ✅ Backward-compatible concurrency features
- ✅ Deterministic replay capabilities
- ✅ Flexible retry policies
- ✅ Enhanced checkpointing
- ✅ Observability integration
- ✅ Zero breaking changes

All functionality is opt-in via `Options` configuration.
