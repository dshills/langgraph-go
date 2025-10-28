# Migration Guide: v0.1.x → v0.2.0

This guide helps you upgrade from LangGraph-Go v0.1.x to v0.2.0, which introduces concurrent execution and deterministic replay capabilities.

## Table of Contents

- [Overview](#overview)
- [Breaking Changes](#breaking-changes)
- [New Features](#new-features)
- [Migration Checklist](#migration-checklist)
- [Step-by-Step Migration](#step-by-step-migration)
  - [1. Update Dependencies](#1-update-dependencies)
  - [2. Update Options Configuration](#2-update-options-configuration)
  - [3. Migrate Store Implementation](#3-migrate-store-implementation)
  - [4. Update Emitter Implementation](#4-update-emitter-implementation)
  - [5. Update Node Implementations](#5-update-node-implementations)
  - [6. Test Determinism](#6-test-determinism)
- [Backward Compatibility](#backward-compatibility)
- [Common Migration Scenarios](#common-migration-scenarios)
- [Troubleshooting](#troubleshooting)
- [Getting Help](#getting-help)

## Overview

**v0.2.0 Major Changes:**

✅ **Concurrent Execution**: Execute independent nodes in parallel with configurable worker pool
✅ **Deterministic Replay**: Record and replay executions for debugging and testing
✅ **Enhanced Store Interface**: New V2 checkpoint methods with full execution context
✅ **Batch Event Emission**: EmitBatch and Flush methods for improved performance
✅ **Retry Policies**: Configurable automatic retry with exponential backoff
✅ **Backpressure Management**: Queue depth control and timeout handling

**Impact Level**: Medium

- Most code continues to work without changes (backward compatible)
- New features opt-in via configuration
- Store and Emitter interfaces extended (existing methods unchanged)
- No changes to core Node interface

**Estimated Migration Time**: 2-4 hours for typical projects

## Breaking Changes

### ⚠️ Store Interface Extended

**Impact**: Custom Store implementations must add new methods

**Before (v0.1.x):**
```go
type Store[S any] interface {
    SaveStep(ctx, runID, step, nodeID, state) error
    LoadLatest(ctx, runID) (state, step, error)
    SaveCheckpoint(ctx, cpID, state, step) error
    LoadCheckpoint(ctx, cpID) (state, step, error)
}
```

**After (v0.2.0):**
```go
type Store[S any] interface {
    // Original methods (unchanged)
    SaveStep(ctx, runID, step, nodeID, state) error
    LoadLatest(ctx, runID) (state, step, error)
    SaveCheckpoint(ctx, cpID, state, step) error
    LoadCheckpoint(ctx, cpID) (state, step, error)

    // New methods (required)
    SaveCheckpointV2(ctx, checkpoint CheckpointV2[S]) error
    LoadCheckpointV2(ctx, runID, stepID) (CheckpointV2[S], error)
    CheckIdempotency(ctx, key) (bool, error)
    PendingEvents(ctx, limit) ([]Event, error)
    MarkEventsEmitted(ctx, eventIDs) error
}
```

**Migration**: Implement new methods or use provided implementations:
- `MemStore`: Already updated with new methods
- `MySQLStore`: Already updated with new methods
- Custom stores: Add stub implementations if not using v0.2.0 features

### ⚠️ Emitter Interface Extended

**Impact**: Custom Emitter implementations must add new methods

**Before (v0.1.x):**
```go
type Emitter interface {
    Emit(event Event)
}
```

**After (v0.2.0):**
```go
type Emitter interface {
    Emit(event Event)
    EmitBatch(ctx context.Context, events []Event) error
    Flush(ctx context.Context) error
}
```

**Migration**: Implement new methods or use default implementations:
```go
// Simple implementation for batch methods
func (e *MyEmitter) EmitBatch(ctx context.Context, events []Event) error {
    for _, event := range events {
        e.Emit(event)
    }
    return nil
}

func (e *MyEmitter) Flush(ctx context.Context) error {
    return nil // No-op if not buffering
}
```

## New Features

### Concurrent Execution (Opt-In)

Enable parallel node execution with `MaxConcurrentNodes`:

```go
// v0.1.x (sequential execution only)
opts := graph.Options{
    MaxSteps: 100,
}

// v0.2.0 (opt-in concurrent execution)
opts := graph.Options{
    MaxSteps:           100,
    MaxConcurrentNodes: 10,   // NEW: Enable concurrency
    QueueDepth:         1000, // NEW: Frontier queue size
}
```

**Default Behavior**: `MaxConcurrentNodes: 1` (sequential, same as v0.1.x)

### Deterministic Replay (Opt-In)

Record and replay executions:

```go
// Enable replay mode
opts := graph.Options{
    ReplayMode:   true,  // NEW: Use recorded I/O
    StrictReplay: true,  // NEW: Enforce hash matching
}

// Replay from checkpoint
checkpoint, _ := store.LoadCheckpointV2(ctx, runID, stepID)
final, err := engine.RunWithCheckpoint(ctx, checkpoint)
```

### Retry Policies (Opt-In)

Configure automatic retries per node:

```go
type MyNode struct {
    policy graph.NodePolicy
}

func (n *MyNode) Policy() graph.NodePolicy {
    return graph.NodePolicy{
        Timeout: 30 * time.Second,
        RetryPolicy: &graph.RetryPolicy{
            MaxAttempts: 3,
            BaseDelay:   100 * time.Millisecond,
            MaxDelay:    10 * time.Second,
            Retryable: func(err error) bool {
                // Retry on network errors
                return isNetworkError(err)
            },
        },
    }
}
```

## Migration Checklist

Before starting migration:

- [ ] Review [Concurrency Guide](./concurrency.md) and [Replay Guide](./replay.md)
- [ ] Backup your codebase (git commit or branch)
- [ ] Review custom Store and Emitter implementations
- [ ] Identify nodes using randomness or external I/O
- [ ] Plan testing strategy for determinism

During migration:

- [ ] Update go.mod dependency to v0.2.0
- [ ] Update Options struct with new fields (use defaults initially)
- [ ] Implement new Store methods (or use built-in MemStore/MySQLStore)
- [ ] Implement new Emitter methods (or use built-in emitters)
- [ ] Update nodes using randomness to use context RNG
- [ ] Add retry policies to nodes that need them
- [ ] Test concurrent execution with race detector
- [ ] Verify deterministic replay works

After migration:

- [ ] Run full test suite with `-race` flag
- [ ] Benchmark performance improvements
- [ ] Update documentation and examples
- [ ] Deploy to staging environment
- [ ] Monitor for issues in production

## Step-by-Step Migration

### 1. Update Dependencies

Update your `go.mod`:

```bash
# Update to v0.2.0
go get github.com/dshills/langgraph-go@v0.2.0

# Verify version
go list -m github.com/dshills/langgraph-go
# Output: github.com/dshills/langgraph-go v0.2.0
```

Build to check for compilation errors:

```bash
go build ./...
```

### 2. Update Options Configuration

**Scenario 1**: Using default Options (no changes needed)

```go
// v0.1.x - Still works in v0.2.0
opts := graph.Options{
    MaxSteps: 100,
}
engine := graph.New(reducer, store, emitter, opts)
```

**Scenario 2**: Enable concurrent execution

```go
// v0.2.0 - Enable concurrency
opts := graph.Options{
    MaxSteps:           100,
    MaxConcurrentNodes: 10,   // Start conservative
    QueueDepth:         1000, // 100x concurrency for buffer
}
engine := graph.New(reducer, store, emitter, opts)
```

**Scenario 3**: Full v0.2.0 feature set

```go
opts := graph.Options{
    // Execution limits
    MaxSteps:           1000,
    MaxConcurrentNodes: 10,

    // Queue management
    QueueDepth:          5000,
    BackpressureTimeout: 60 * time.Second,

    // Timeouts
    DefaultNodeTimeout: 30 * time.Second,
    RunWallClockBudget: 5 * time.Minute,

    // Replay (disabled by default)
    ReplayMode:   false,
    StrictReplay: false,
}
engine := graph.New(reducer, store, emitter, opts)
```

### 3. Migrate Store Implementation

**Option A**: Use built-in stores (recommended)

```go
// MemStore (testing/development)
store := store.NewMemStore[MyState]()

// MySQLStore (production)
db, _ := sql.Open("mysql", "user:pass@tcp(host:3306)/db")
store := store.NewMySQLStore[MyState](db)
```

Both built-in stores are fully compatible with v0.2.0.

**Option B**: Update custom Store implementation

Add stub implementations for new methods if not using v0.2.0 features:

```go
// Add to your custom Store[S] implementation

func (s *CustomStore[S]) SaveCheckpointV2(ctx context.Context, checkpoint store.CheckpointV2[S]) error {
    // Minimal implementation: delegate to v1 checkpoint method
    return s.SaveCheckpoint(ctx, checkpoint.Label, checkpoint.State, checkpoint.StepID)
}

func (s *CustomStore[S]) LoadCheckpointV2(ctx context.Context, runID string, stepID int) (store.CheckpointV2[S], error) {
    // Minimal implementation: return error if not supported
    return store.CheckpointV2[S]{}, fmt.Errorf("CheckpointV2 not implemented")
}

func (s *CustomStore[S]) CheckIdempotency(ctx context.Context, key string) (bool, error) {
    // Minimal implementation: always return false (no deduplication)
    return false, nil
}

func (s *CustomStore[S]) PendingEvents(ctx context.Context, limit int) ([]emit.Event, error) {
    // Minimal implementation: no transactional outbox
    return []emit.Event{}, nil
}

func (s *CustomStore[S]) MarkEventsEmitted(ctx context.Context, eventIDs []string) error {
    // Minimal implementation: no-op
    return nil
}
```

**Option C**: Full v0.2.0 Store implementation

See [Store Implementation Guide](./docs/store-implementation.md) for detailed examples.

### 4. Update Emitter Implementation

**Option A**: Use built-in emitters (recommended)

```go
// LogEmitter (development)
emitter := emit.NewLogEmitter(os.Stdout, false)

// BufferedEmitter (production)
baseEmitter := emit.NewLogEmitter(os.Stdout, false)
emitter := emit.NewBufferedEmitter(baseEmitter, 1000)

// NullEmitter (testing)
emitter := emit.NewNullEmitter()
```

All built-in emitters support v0.2.0 batch methods.

**Option B**: Update custom Emitter implementation

Add batch method implementations:

```go
// Add to your custom Emitter implementation

func (e *CustomEmitter) EmitBatch(ctx context.Context, events []emit.Event) error {
    // Simple implementation: emit one by one
    for _, event := range events {
        e.Emit(event)
    }
    return nil
}

func (e *CustomEmitter) Flush(ctx context.Context) error {
    // If your emitter buffers events, flush them here
    // Otherwise, this can be a no-op
    return nil
}
```

### 5. Update Node Implementations

**Scenario 1**: Pure nodes (no changes needed)

```go
// Pure function nodes work as-is
func processNode(ctx context.Context, state MyState) graph.NodeResult[MyState] {
    delta := MyState{Count: state.Count + 1}
    return graph.NodeResult[MyState]{
        Delta: delta,
        Route: graph.Goto("next"),
    }
}
```

**Scenario 2**: Nodes using randomness

```go
// v0.1.x - Non-deterministic
import "math/rand"

func randomNode(ctx context.Context, state MyState) graph.NodeResult[MyState] {
    value := rand.Intn(100) // Different on each execution
    delta := MyState{RandomValue: value}
    return graph.NodeResult[MyState]{Delta: delta, Route: graph.Stop()}
}

// v0.2.0 - Deterministic (use context RNG)
func randomNode(ctx context.Context, state MyState) graph.NodeResult[MyState] {
    rng, ok := ctx.Value(graph.RNGKey).(*rand.Rand)
    if !ok {
        // Fallback for tests
        rng = rand.New(rand.NewSource(42))
    }

    value := rng.Intn(100) // Same on replay
    delta := MyState{RandomValue: value}
    return graph.NodeResult[MyState]{Delta: delta, Route: graph.Stop()}
}
```

**Scenario 3**: Nodes with retry policies

```go
// Add Policy() method to node struct
type APINode struct {
    client *http.Client
}

func (n *APINode) Run(ctx context.Context, state MyState) graph.NodeResult[MyState] {
    // Node implementation
    resp, err := n.client.Get("https://api.example.com/data")
    if err != nil {
        return graph.NodeResult[MyState]{Err: err}
    }
    // ... process response
}

func (n *APINode) Policy() graph.NodePolicy {
    return graph.NodePolicy{
        Timeout: 30 * time.Second,
        RetryPolicy: &graph.RetryPolicy{
            MaxAttempts: 3,
            BaseDelay:   100 * time.Millisecond,
            MaxDelay:    10 * time.Second,
            Retryable: func(err error) bool {
                // Retry on temporary errors
                if netErr, ok := err.(net.Error); ok && netErr.Temporary() {
                    return true
                }
                // Retry on HTTP 429, 503
                if httpErr, ok := err.(*HTTPError); ok {
                    return httpErr.StatusCode == 429 || httpErr.StatusCode == 503
                }
                return false
            },
        },
    }
}
```

### 6. Test Determinism

Add tests to verify deterministic behavior:

```go
func TestDeterministicExecution(t *testing.T) {
    opts := graph.Options{
        MaxConcurrentNodes: 10,
    }

    // Run workflow twice with same inputs
    engine1 := graph.New(reducer, store.NewMemStore[MyState](), emitter, opts)
    // ... build graph
    result1, err1 := engine1.Run(ctx, "run-1", initialState)

    engine2 := graph.New(reducer, store.NewMemStore[MyState](), emitter, opts)
    // ... build graph (identical)
    result2, err2 := engine2.Run(ctx, "run-2", initialState)

    // Verify identical results
    require.NoError(t, err1)
    require.NoError(t, err2)
    assert.Equal(t, result1, result2, "Executions must produce identical results")
}

func TestReplayFromCheckpoint(t *testing.T) {
    // Original execution
    store := store.NewMemStore[MyState]()
    opts := graph.Options{MaxConcurrentNodes: 5}
    engine := graph.New(reducer, store, emitter, opts)
    // ... build graph

    original, err := engine.Run(ctx, "original-run", initialState)
    require.NoError(t, err)

    // Load checkpoint
    checkpoint, err := store.LoadCheckpointV2(ctx, "original-run", 0)
    require.NoError(t, err)

    // Replay execution
    opts.ReplayMode = true
    replayEngine := graph.New(reducer, store, emitter, opts)
    // ... build graph

    replayed, err := replayEngine.RunWithCheckpoint(ctx, checkpoint)
    require.NoError(t, err)

    // Verify identical results
    assert.Equal(t, original, replayed, "Replay must produce identical results")
}

func TestConcurrentExecutionWithRaceDetector(t *testing.T) {
    opts := graph.Options{
        MaxConcurrentNodes: 20, // High concurrency to expose races
    }

    engine := graph.New(reducer, store.NewMemStore[MyState](), emitter, opts)
    // ... build graph with fan-out

    // Run with race detector: go test -race
    final, err := engine.Run(ctx, "race-test", initialState)
    require.NoError(t, err)
    assert.NotNil(t, final)
}
```

Run tests with race detector:

```bash
go test -race ./...
```

## Backward Compatibility

v0.2.0 maintains backward compatibility with v0.1.x:

✅ **Existing code continues to work**:
- Default Options use sequential execution (same as v0.1.x)
- Store and Emitter interfaces extended (original methods unchanged)
- Node interface unchanged
- Reducer function signature unchanged

✅ **Opt-in for new features**:
- Concurrent execution: Set `MaxConcurrentNodes > 1`
- Deterministic replay: Set `ReplayMode = true`
- Retry policies: Implement `Policy()` method on nodes
- Batch emission: Call `EmitBatch()` explicitly

✅ **Gradual migration path**:
1. Update dependencies → code compiles
2. Add stub implementations → tests pass
3. Enable features incrementally → production rollout
4. Full migration over time → leverage all features

## Common Migration Scenarios

### Scenario 1: Simple Workflow (No Changes)

**v0.1.x Code:**
```go
func main() {
    opts := graph.Options{MaxSteps: 100}
    store := store.NewMemStore[MyState]()
    emitter := emit.NewLogEmitter(os.Stdout, false)
    engine := graph.New(reducer, store, emitter, opts)

    engine.Add("process", processNode)
    engine.StartAt("process")

    final, _ := engine.Run(ctx, "run-1", MyState{})
}
```

**v0.2.0 Migration:**
No changes needed! Code works as-is.

### Scenario 2: Enable Concurrency Only

**Migration:**
```go
func main() {
    opts := graph.Options{
        MaxSteps:           100,
        MaxConcurrentNodes: 10,   // NEW: Enable concurrency
        QueueDepth:         1000, // NEW: Queue size
    }
    store := store.NewMemStore[MyState]()
    emitter := emit.NewLogEmitter(os.Stdout, false)
    engine := graph.New(reducer, store, emitter, opts)

    // Rest of code unchanged
}
```

Test with race detector:
```bash
go test -race ./...
```

### Scenario 3: Add Retry Policies

**Migration:**
```go
type APINode struct {
    client *http.Client
}

func (n *APINode) Run(ctx context.Context, state MyState) graph.NodeResult[MyState] {
    // Existing implementation unchanged
}

// NEW: Add Policy method
func (n *APINode) Policy() graph.NodePolicy {
    return graph.NodePolicy{
        RetryPolicy: &graph.RetryPolicy{
            MaxAttempts: 3,
            BaseDelay:   100 * time.Millisecond,
            MaxDelay:    10 * time.Second,
            Retryable:   isRetryable,
        },
    }
}

func isRetryable(err error) bool {
    // Define retryable errors
    return err != nil && strings.Contains(err.Error(), "temporary")
}
```

### Scenario 4: Enable Deterministic Replay

**Migration:**
```go
// 1. Update nodes to use context RNG
func randomNode(ctx context.Context, state MyState) graph.NodeResult[MyState] {
    rng := ctx.Value(graph.RNGKey).(*rand.Rand)
    value := rng.Intn(100)
    // ...
}

// 2. Original execution (records I/O automatically)
opts := graph.Options{MaxSteps: 100}
store := store.NewMySQLStore[MyState](db) // Persists checkpoints
engine := graph.New(reducer, store, emitter, opts)
final, _ := engine.Run(ctx, "prod-run-123", initialState)

// 3. Replay for debugging
checkpoint, _ := store.LoadCheckpointV2(ctx, "prod-run-123", 0)
opts.ReplayMode = true
replayEngine := graph.New(reducer, store, emitter, opts)
replayed, _ := replayEngine.RunWithCheckpoint(ctx, checkpoint)
// Identical to original execution
```

### Scenario 5: Custom Store Migration

**Before (v0.1.x):**
```go
type MyStore[S any] struct {
    db *sql.DB
}

func (s *MyStore[S]) SaveStep(ctx context.Context, runID string, step int, nodeID string, state S) error {
    // Implementation
}

func (s *MyStore[S]) LoadLatest(ctx context.Context, runID string) (S, int, error) {
    // Implementation
}

// ... other v0.1.x methods
```

**After (v0.2.0):**
```go
type MyStore[S any] struct {
    db *sql.DB
}

// Existing methods unchanged
func (s *MyStore[S]) SaveStep(...) error { /* same */ }
func (s *MyStore[S]) LoadLatest(...) (S, int, error) { /* same */ }

// NEW: Add v0.2.0 methods
func (s *MyStore[S]) SaveCheckpointV2(ctx context.Context, checkpoint store.CheckpointV2[S]) error {
    // Option 1: Full implementation with frontier, RNG seed, etc.
    // See MySQLStore implementation for reference

    // Option 2: Minimal delegation to v1 method
    return s.SaveCheckpoint(ctx, checkpoint.Label, checkpoint.State, checkpoint.StepID)
}

func (s *MyStore[S]) LoadCheckpointV2(ctx context.Context, runID string, stepID int) (store.CheckpointV2[S], error) {
    // Option 1: Full implementation
    // Option 2: Return "not implemented" if feature not needed
    return store.CheckpointV2[S]{}, fmt.Errorf("v2 checkpoints not supported")
}

func (s *MyStore[S]) CheckIdempotency(ctx context.Context, key string) (bool, error) {
    // Option 1: Track keys in database
    // Option 2: Return false (no deduplication)
    return false, nil
}

func (s *MyStore[S]) PendingEvents(ctx context.Context, limit int) ([]emit.Event, error) {
    // Option 1: Implement transactional outbox
    // Option 2: Return empty list
    return []emit.Event{}, nil
}

func (s *MyStore[S]) MarkEventsEmitted(ctx context.Context, eventIDs []string) error {
    // Option 1: Mark in database
    // Option 2: No-op
    return nil
}
```

## Troubleshooting

### Build Errors After Update

**Error**: `Store does not implement store.Store (missing method SaveCheckpointV2)`

**Solution**: Add new methods to custom Store implementation (see Scenario 5 above)

### Race Detector Warnings

**Error**: `WARNING: DATA RACE` when running `go test -race`

**Solution**:
1. Check reducer function for shared mutable state
2. Verify nodes don't use global variables
3. Ensure external clients are thread-safe
4. Use sync.Mutex or channels for shared state

### Non-Deterministic Results

**Problem**: Different results across runs with same inputs

**Solution**:
1. Check for randomness using global `rand` (use context RNG)
2. Check for `time.Now()` usage (use state-provided time)
3. Check for map iteration (sort keys before iterating)
4. Check reducer for side effects (make it pure)

### Performance Regression

**Problem**: v0.2.0 slower than v0.1.x

**Solution**:
1. Verify `MaxConcurrentNodes > 1` (default is 1, same as v0.1.x)
2. Check for serialization overhead (profile with pprof)
3. Reduce `QueueDepth` if memory is an issue
4. Benchmark specific bottlenecks

```bash
go test -bench=. -benchmem
go test -cpuprofile=cpu.prof -bench=.
go tool pprof cpu.prof
```

### Replay Mismatch Errors

**Error**: `ErrReplayMismatch: recorded I/O hash mismatch`

**Solution**:
1. Check if node logic changed between original and replay
2. Check if external API responses changed
3. Use lenient replay mode: `StrictReplay: false`
4. Regenerate checkpoint with current code

## Getting Help

### Resources

- [Concurrency Guide](./concurrency.md) - Detailed concurrency documentation
- [Replay Guide](./replay.md) - Deterministic replay guide
- [Examples](../examples/) - Working code examples
- [FAQ](./FAQ.md) - Frequently asked questions

### Support Channels

- **GitHub Issues**: Report bugs or request features
- **Discussions**: Ask questions and share patterns
- **Contributing**: See [CONTRIBUTING.md](../CONTRIBUTING.md)

### Common Questions

**Q: Do I need to migrate immediately?**
A: No. v0.2.0 is backward compatible. Migrate when ready.

**Q: Can I use v0.2.0 without concurrent execution?**
A: Yes. Set `MaxConcurrentNodes: 1` for sequential execution (default).

**Q: Will concurrent execution break my workflow?**
A: Not if your nodes are independent and your reducer is pure. Test with `-race` flag.

**Q: Can I mix v0.1.x and v0.2.0 features?**
A: Yes. All v0.2.0 features are opt-in via configuration.

**Q: What's the performance impact of replay mode?**
A: Replay is faster (no external I/O) but adds checkpoint storage overhead.

## Summary

v0.2.0 Migration Summary:

✅ **Backward Compatible**: Existing code works without changes
✅ **Opt-In Features**: Enable new capabilities via configuration
✅ **Gradual Migration**: Migrate incrementally over time
✅ **Comprehensive Docs**: Detailed guides for all features
✅ **Production Ready**: Battle-tested concurrency and replay

**Next Steps:**

1. Update dependencies: `go get github.com/dshills/langgraph-go@v0.2.0`
2. Review your Store and Emitter implementations
3. Test with race detector: `go test -race ./...`
4. Enable features incrementally
5. Monitor performance and determinism

Welcome to LangGraph-Go v0.2.0! 🚀
