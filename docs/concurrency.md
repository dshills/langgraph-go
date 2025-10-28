# Concurrent Graph Execution

LangGraph-Go v0.2.0 introduces production-ready concurrent execution that enables parallel node processing while maintaining deterministic, reproducible results. This guide explains the concurrency model, configuration options, and best practices.

## Table of Contents

- [Overview](#overview)
- [Concurrency Model](#concurrency-model)
  - [Worker Pool Architecture](#worker-pool-architecture)
  - [Frontier Queue](#frontier-queue)
  - [Deterministic Ordering](#deterministic-ordering)
- [Configuration](#configuration)
  - [Basic Options](#basic-options)
  - [Advanced Tuning](#advanced-tuning)
- [Backpressure Management](#backpressure-management)
  - [Queue Depth Control](#queue-depth-control)
  - [Backpressure Behavior](#backpressure-behavior)
- [State Merging](#state-merging)
  - [Merge Order Guarantees](#merge-order-guarantees)
  - [Conflict Handling](#conflict-handling)
- [Performance Tuning](#performance-tuning)
  - [Choosing MaxConcurrentNodes](#choosing-maxconcurrentnodes)
  - [Queue Depth Sizing](#queue-depth-sizing)
  - [Measuring Speedup](#measuring-speedup)
- [Best Practices](#best-practices)
- [Troubleshooting](#troubleshooting)

## Overview

Concurrent execution allows independent nodes in your workflow to execute in parallel, dramatically reducing total execution time while maintaining correctness guarantees:

```go
// Sequential execution: 15 seconds total
// Node A (5s) → Node B (5s) → Node C (5s)

// Concurrent execution: 5 seconds total
// Node A (5s) ┐
// Node B (5s) ├→ merge → Continue
// Node C (5s) ┘
```

**Key Benefits:**

- **Performance**: Execute independent nodes in parallel for 2-5x speedup
- **Determinism**: Results are identical regardless of execution order
- **Reproducibility**: Same inputs always produce same outputs
- **Production-Ready**: Built-in backpressure, timeouts, and observability

## Concurrency Model

### Worker Pool Architecture

LangGraph-Go uses a bounded worker pool to execute nodes concurrently:

```
┌─────────────────────────────────────────────────┐
│                  Engine                         │
│                                                 │
│  ┌──────────────┐      ┌──────────────────┐   │
│  │   Frontier   │─────▶│   Worker Pool    │   │
│  │  (Priority   │      │  (MaxConcurrent  │   │
│  │   Queue)     │      │    Nodes: 5)     │   │
│  └──────────────┘      └──────────────────┘   │
│         ▲                        │             │
│         │                        ▼             │
│         │              ┌──────────────────┐   │
│         └──────────────│  State Merger    │   │
│                        │  (Reducer Func)  │   │
│                        └──────────────────┘   │
└─────────────────────────────────────────────────┘
```

**Components:**

1. **Frontier Queue**: Priority queue of ready-to-execute work items
2. **Worker Pool**: Fixed-size pool of goroutines executing nodes
3. **State Merger**: Applies node results deterministically via reducer

### Frontier Queue

The frontier is a priority queue of `WorkItem[S]` structs representing nodes ready for execution:

```go
type WorkItem[S any] struct {
    StepID       int    // Execution step number
    OrderKey     uint64 // Deterministic sort key
    NodeID       string // Node to execute
    State        S      // Input state snapshot
    Attempt      int    // Retry counter
    ParentNodeID string // Provenance tracking
    EdgeIndex    int    // Edge taken from parent
}
```

**Work Item Lifecycle:**

1. **Creation**: Node produces routing decision → creates work items
2. **Enqueue**: Items added to frontier with computed order key
3. **Dequeue**: Worker pulls highest-priority item (min order key)
4. **Execution**: Node runs with state snapshot
5. **Merge**: Result delta merged into accumulated state

**Queue Properties:**

- **Bounded**: Configurable capacity via `QueueDepth`
- **Blocking**: Enqueue blocks when full (backpressure)
- **Priority-Ordered**: Items dequeued by ascending `OrderKey`
- **Thread-Safe**: Concurrent enqueue/dequeue operations

### Deterministic Ordering

Order keys ensure consistent execution order across replays:

```go
// OrderKey = Hash(ParentNodeID || EdgeIndex)
orderKey := ComputeOrderKey("nodeA", 0) // → uint64 hash
```

**Order Key Properties:**

1. **Deterministic**: Same parent + edge always produces same key
2. **Total Ordering**: All work items can be consistently sorted
3. **Collision-Resistant**: SHA-256 provides cryptographic strength
4. **Replay-Stable**: Same order keys across execution replays

**Example Ordering:**

```go
// Graph: start → [A, B, C] (fan-out)

// Step 1: Start node creates 3 work items
WorkItem{NodeID: "A", OrderKey: Hash("start", 0)} // 0x1234...
WorkItem{NodeID: "B", OrderKey: Hash("start", 1)} // 0x5678...
WorkItem{NodeID: "C", OrderKey: Hash("start", 2)} // 0x9abc...

// Items dequeued in order key sequence (A, then B, then C)
// Even if C finishes first, results merge in deterministic order
```

## Configuration

### Basic Options

Enable concurrent execution with `MaxConcurrentNodes`:

```go
opts := graph.Options{
    MaxSteps:            100,
    MaxConcurrentNodes:  5,    // Execute up to 5 nodes in parallel
}

engine := graph.New(reducer, store, emitter, opts)
```

**Default Values:**

```go
Options{
    MaxSteps:            100,  // Total step limit
    MaxConcurrentNodes:  1,    // Sequential (no concurrency)
    QueueDepth:          1000, // Frontier queue capacity
    BackpressureTimeout: 30s,  // Block duration before checkpoint
}
```

### Advanced Tuning

Full configuration options for production workloads:

```go
opts := graph.Options{
    // Execution Limits
    MaxSteps:            1000,              // Total execution steps
    MaxConcurrentNodes:  10,                // Concurrent node limit

    // Queue Management
    QueueDepth:          5000,              // Frontier capacity
    BackpressureTimeout: 60 * time.Second, // Backpressure wait time

    // Timeouts
    DefaultNodeTimeout:  30 * time.Second, // Per-node execution limit
    RunWallClockBudget:  5 * time.Minute,  // Total run time limit

    // Replay Options
    ReplayMode:          false,            // Enable replay from checkpoint
    StrictReplay:        true,             // Enforce I/O hash matching
}

engine := graph.New(reducer, store, emitter, opts)
```

**Configuration Guidelines:**

| Workload Type | MaxConcurrentNodes | QueueDepth | Notes |
|---------------|-------------------|------------|-------|
| I/O-bound (API calls) | 20-50 | 10000 | High concurrency, large queue |
| CPU-bound (processing) | 2-8 | 1000 | Match CPU cores |
| Mixed workload | 10-20 | 5000 | Balance between extremes |
| Low-latency | 5-10 | 500 | Quick admission, fast response |

## Backpressure Management

### Queue Depth Control

The frontier queue has a fixed capacity to prevent memory exhaustion:

```go
opts := graph.Options{
    MaxConcurrentNodes:  10,
    QueueDepth:          1000, // Max 1000 queued work items
}
```

**Queue States:**

```
Queue Empty (0 items)
    ↓ enqueue work items
Queue Active (1-999 items) ← Normal operation
    ↓ enqueue reaches capacity
Queue Full (1000 items)
    ↓ enqueue blocks (backpressure)
Backpressure Active
    ↓ timeout reached
Checkpoint & Pause
```

### Backpressure Behavior

When the queue reaches capacity, the engine applies backpressure:

```go
// Backpressure sequence:
1. Queue reaches QueueDepth capacity
2. Enqueue operations block (waiting for dequeue)
3. If blocked > BackpressureTimeout:
   a. Save checkpoint
   b. Emit backpressure event
   c. Pause execution
4. Resume later via checkpoint
```

**Backpressure Example:**

```go
// Scenario: 100 nodes spawned, QueueDepth=20, MaxConcurrent=5

// Step 1: First 20 items enqueue successfully
// Step 2: 5 items dequeue and execute (workers busy)
// Step 3: Next 5 items enqueue (queue now at 20 again)
// Step 4: Enqueue blocks (queue full, backpressure activated)
// Step 5: Wait for workers to finish and dequeue more items
// Step 6: If wait exceeds BackpressureTimeout, checkpoint and pause
```

**Handling Backpressure Events:**

```go
// Monitor backpressure via events
emitter := emit.NewLogEmitter(os.Stdout, true)

// Emitted event when backpressure occurs:
Event{
    Type:      "backpressure",
    RunID:     "run-123",
    StepID:    42,
    NodeID:    "fan-out-node",
    Message:   "Queue full, blocking admission",
    Timestamp: time.Now(),
    Metadata: map[string]interface{}{
        "queue_depth":     1000,
        "active_workers":  10,
        "blocked_duration": "5s",
    },
}
```

## State Merging

### Merge Order Guarantees

Concurrent node results are merged deterministically using order keys:

```go
// Reducer function merges deltas in order key sequence
func reducer(prev, delta MyState) MyState {
    // This is called in deterministic order regardless of
    // which node finished first at runtime
    if delta.Count > 0 {
        prev.Count += delta.Count
    }
    if delta.Message != "" {
        prev.Messages = append(prev.Messages, delta.Message)
    }
    return prev
}
```

**Merge Process:**

```
Step 1: Concurrent Execution
    Node A (finishes 3rd) → Delta{Count: 1, Message: "A"}
    Node B (finishes 1st) → Delta{Count: 2, Message: "B"}
    Node C (finishes 2nd) → Delta{Count: 3, Message: "C"}

Step 2: Order by OrderKey
    Sort deltas: [A, B, C] (by order key, not completion time)

Step 3: Apply Reducer Sequentially
    State{Count: 0, Messages: []}
    + Delta{Count: 1, Message: "A"} → State{Count: 1, Messages: ["A"]}
    + Delta{Count: 2, Message: "B"} → State{Count: 3, Messages: ["A","B"]}
    + Delta{Count: 3, Message: "C"} → State{Count: 6, Messages: ["A","B","C"]}

Final State: {Count: 6, Messages: ["A", "B", "C"]}
```

**Order Guarantees:**

- ✅ Deltas merged in order key sequence (deterministic)
- ✅ Same order across all replays
- ✅ Independent of runtime scheduling
- ✅ Independent of node completion time

### Conflict Handling

When concurrent nodes modify the same state field, the reducer determines resolution:

```go
// Last-Writer-Wins Strategy
func reducer(prev, delta MyState) MyState {
    if delta.Status != "" {
        prev.Status = delta.Status // Overwrites previous
    }
    return prev
}

// Accumulation Strategy
func reducer(prev, delta MyState) MyState {
    prev.Counter += delta.Counter // Additive merge
    return prev
}

// Merge Strategy
func reducer(prev, delta MyState) MyState {
    for k, v := range delta.Tags {
        prev.Tags[k] = v // Merge maps
    }
    return prev
}
```

**Best Practices for Reducers:**

1. **Pure Functions**: No side effects (I/O, mutation, randomness)
2. **Deterministic**: Same inputs always produce same output
3. **Field-Level Logic**: Handle each field explicitly
4. **Commutative** (ideal): Order independence for robustness
5. **Documented**: Clarify conflict resolution strategy

## Performance Tuning

### Choosing MaxConcurrentNodes

Guidelines for setting concurrency limits:

**CPU-Bound Workloads:**

```go
// Rule: MaxConcurrent = NumCPU or NumCPU + 1
maxConcurrent := runtime.NumCPU()

opts := graph.Options{
    MaxConcurrentNodes: maxConcurrent,
}
```

**I/O-Bound Workloads:**

```go
// Rule: MaxConcurrent = 10-50 (experiment with load testing)
opts := graph.Options{
    MaxConcurrentNodes: 20, // Start here, tune based on metrics
}
```

**Mixed Workloads:**

```go
// Rule: MaxConcurrent = 2 * NumCPU
maxConcurrent := 2 * runtime.NumCPU()

opts := graph.Options{
    MaxConcurrentNodes: maxConcurrent,
}
```

### Queue Depth Sizing

Queue depth should accommodate expected fan-out:

```go
// Formula: QueueDepth ≥ AvgFanOut * MaxConcurrentNodes * 2

// Example: Fan-out of 10, concurrency 5
opts := graph.Options{
    MaxConcurrentNodes: 5,
    QueueDepth:         10 * 5 * 2, // = 100
}
```

**Queue Depth Trade-offs:**

| Queue Depth | Memory Usage | Backpressure Frequency | Use Case |
|-------------|--------------|------------------------|----------|
| Small (100-500) | Low | High | Low fan-out, memory-constrained |
| Medium (1000-5000) | Moderate | Occasional | Typical workflows |
| Large (10000+) | High | Rare | High fan-out, bursty loads |

### Measuring Speedup

Benchmark concurrent vs sequential execution:

```go
// benchmark_test.go
func BenchmarkSequential(b *testing.B) {
    opts := graph.Options{
        MaxConcurrentNodes: 1, // Sequential
    }
    engine := setupEngine(opts)

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        engine.Run(ctx, fmt.Sprintf("run-%d", i), initialState)
    }
}

func BenchmarkConcurrent(b *testing.B) {
    opts := graph.Options{
        MaxConcurrentNodes: 10, // Concurrent
    }
    engine := setupEngine(opts)

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        engine.Run(ctx, fmt.Sprintf("run-%d", i), initialState)
    }
}

// Run benchmarks:
// go test -bench=. -benchtime=10s
//
// Example output:
// BenchmarkSequential-8    100    1500ms/op
// BenchmarkConcurrent-8    500    300ms/op   (5x speedup)
```

**Speedup Formula:**

```
Speedup = T_sequential / T_concurrent

// Example with 5 parallel nodes of 1s each:
T_sequential = 5 * 1s = 5s
T_concurrent = 1s (max of parallel) + overhead
Speedup = 5s / 1.1s ≈ 4.5x
```

## Best Practices

### 1. Start Conservative, Scale Up

```go
// Begin with low concurrency
opts := graph.Options{
    MaxConcurrentNodes: 2,
    QueueDepth:         100,
}

// Measure performance and error rates
// Gradually increase concurrency
// Monitor metrics: latency, throughput, errors
```

### 2. Monitor Queue Depth

```go
// Expose metrics for observability
type Metrics struct {
    QueueDepth    int
    ActiveWorkers int
    TotalSteps    int
}

// Alert on sustained high queue depth
if metrics.QueueDepth > 0.8 * opts.QueueDepth {
    log.Warn("Queue nearing capacity, consider increasing QueueDepth")
}
```

### 3. Design Independent Nodes

```go
// ✅ Good: Independent nodes (safe for parallel execution)
nodeA := func(ctx context.Context, s State) NodeResult[State] {
    // Read external API
    delta := State{DataA: fetchDataA()}
    return NodeResult[State]{Delta: delta, Route: Goto("merge")}
}

nodeB := func(ctx context.Context, s State) NodeResult[State] {
    // Read different API (no shared state with nodeA)
    delta := State{DataB: fetchDataB()}
    return NodeResult[State]{Delta: delta, Route: Goto("merge")}
}

// ❌ Bad: Nodes with hidden dependencies
nodeA := func(ctx context.Context, s State) NodeResult[State] {
    writeToSharedCache("key", "valueA") // Side effect!
    delta := State{}
    return NodeResult[State]{Delta: delta, Route: Goto("merge")}
}

nodeB := func(ctx context.Context, s State) NodeResult[State] {
    value := readFromSharedCache("key") // Race condition!
    delta := State{Value: value}
    return NodeResult[State]{Delta: delta, Route: Goto("merge")}
}
```

### 4. Pure Reducers

```go
// ✅ Good: Pure reducer
func reducer(prev, delta State) State {
    if delta.Count > 0 {
        prev.Count += delta.Count // Only modifies state
    }
    return prev
}

// ❌ Bad: Reducer with side effects
func reducer(prev, delta State) State {
    log.Println("Merging state") // I/O side effect!
    prev.Count += delta.Count

    db.Save(prev) // External mutation - breaks determinism!
    return prev
}
```

### 5. Test Concurrency

```go
// Test with high concurrency to expose race conditions
func TestConcurrentExecution(t *testing.T) {
    opts := graph.Options{
        MaxConcurrentNodes: 50, // High concurrency
    }
    engine := graph.New(reducer, store, emitter, opts)

    // Build graph with fan-out
    engine.Add("start", startNode)
    engine.Add("fanout", fanOutNode) // Creates 20 parallel branches
    // ... add more nodes

    // Run with race detector: go test -race
    final, err := engine.Run(ctx, "test-run", initialState)
    require.NoError(t, err)

    // Verify deterministic result
    expected := computeExpectedState()
    assert.Equal(t, expected, final)
}
```

## Troubleshooting

### Queue Backpressure Issues

**Symptoms**: Frequent backpressure events, slow execution

**Diagnosis**:
```go
// Check metrics
log.Printf("Queue depth: %d/%d", metrics.QueueDepth, opts.QueueDepth)
log.Printf("Active workers: %d/%d", metrics.ActiveWorkers, opts.MaxConcurrentNodes)
```

**Solutions**:
1. Increase `QueueDepth` if queue frequently full
2. Increase `MaxConcurrentNodes` if workers always busy
3. Reduce fan-out in graph topology
4. Add join nodes to reduce frontier size

### Non-Deterministic Results

**Symptoms**: Different results across runs with same inputs

**Diagnosis**:
```go
// Check reducer purity
func TestReducerDeterminism(t *testing.T) {
    prev := State{Count: 5}
    delta := State{Count: 3}

    result1 := reducer(prev, delta)
    result2 := reducer(prev, delta)

    assert.Equal(t, result1, result2, "Reducer must be deterministic")
}
```

**Solutions**:
1. Remove side effects from reducer (I/O, logging, mutations)
2. Use context RNG for randomness, not global rand
3. Avoid time.Now() in reducer or nodes
4. Check for data races: `go test -race`

### Poor Speedup

**Symptoms**: Concurrent execution not faster than sequential

**Diagnosis**:
```go
// Profile execution
// go test -cpuprofile=cpu.prof -bench=BenchmarkConcurrent
// go tool pprof cpu.prof

// Check node execution time distribution
log.Printf("Node A duration: %v", durationA)
log.Printf("Node B duration: %v", durationB)
```

**Solutions**:
1. Check if nodes are actually independent (no serialization)
2. Verify MaxConcurrentNodes > 1
3. Profile for bottlenecks (locks, channel contention)
4. Ensure nodes are CPU/I/O bound (not instant)
5. Check if serialization overhead dominates (state copy cost)

### Memory Issues

**Symptoms**: High memory usage, OOM errors

**Diagnosis**:
```go
// Monitor memory usage
var m runtime.MemStats
runtime.ReadMemStats(&m)
log.Printf("Alloc: %d MB", m.Alloc/1024/1024)
log.Printf("Queue depth: %d", metrics.QueueDepth)
```

**Solutions**:
1. Reduce `QueueDepth` to lower memory footprint
2. Reduce `MaxConcurrentNodes` to limit parallel state copies
3. Optimize state struct size (remove large fields)
4. Use state references instead of deep copies where safe

## Related Documentation

- [Deterministic Replay Guide](./replay.md) - Record and replay executions
- [State Management](./guides/03-state-management.md) - Reducer patterns
- [Parallel Execution](./guides/06-parallel.md) - Fan-out workflows
- [Event Tracing](./guides/08-event-tracing.md) - Observability

## Summary

LangGraph-Go's concurrent execution provides:

✅ **Deterministic Parallelism**: Execute nodes concurrently with predictable results
✅ **Bounded Resources**: Control memory and CPU usage via limits
✅ **Backpressure**: Graceful handling of overload scenarios
✅ **Production-Ready**: Built-in timeouts, retries, and observability
✅ **Easy Configuration**: Sensible defaults with tuning options

Start with sequential execution (`MaxConcurrentNodes: 1`), measure performance, and scale up concurrency as needed. Monitor queue depth and active workers to optimize configuration for your workload.
