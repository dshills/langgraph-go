# Quickstart: Concurrent Graph Execution

**Feature**: Concurrent Graph Execution with Deterministic Replay
**Audience**: Developers implementing concurrent workflows
**Time**: 15 minutes

## Overview

This guide demonstrates how to build a concurrent LangGraph-Go workflow that executes independent nodes in parallel while maintaining deterministic replay guarantees.

---

## Prerequisites

- Go 1.21+
- LangGraph-Go v0.2.0+ installed
- Basic understanding of graph-based workflows

```bash
go get github.com/dshills/langgraph-go@v0.2.0
```

---

## Scenario

Build a research workflow that:
1. Fetches data from 3 independent APIs in parallel
2. Merges results deterministically
3. Can be replayed exactly for debugging

**Sequential time**: ~3 seconds (1s per API call)
**Concurrent time**: ~1 second (all APIs called in parallel)

---

## Step 1: Define State

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/dshills/langgraph-go/graph"
    "github.com/dshills/langgraph-go/graph/store"
    "github.com/dshills/langgraph-go/graph/emit"
)

// ResearchState holds workflow state
type ResearchState struct {
    Query   string   `json:"query"`
    Papers  []string `json:"papers"`
    Tweets  []string `json:"tweets"`
    News    []string `json:"news"`
    Summary string   `json:"summary"`
}
```

---

## Step 2: Create Concurrent Nodes

```go
// FetchPapersNode calls academic API
type FetchPapersNode struct {
    graph.DefaultPolicy
}

func (n *FetchPapersNode) Run(ctx context.Context, state ResearchState) graph.NodeResult[ResearchState] {
    fmt.Println("Fetching papers...")
    time.Sleep(1 * time.Second) // Simulate API call

    return graph.NodeResult[ResearchState]{
        Delta: ResearchState{Papers: []string{"Paper 1", "Paper 2"}},
        Next:  graph.Goto("merge"),
    }
}

// Declare recordable I/O for replay
func (n *FetchPapersNode) Effects() graph.SideEffectPolicy {
    return graph.SideEffectPolicy{
        Recordable:          true,
        RequiresIdempotency: true,
    }
}

// FetchTweetsNode calls social media API
type FetchTweetsNode struct {
    graph.DefaultPolicy
}

func (n *FetchTweetsNode) Run(ctx context.Context, state ResearchState) graph.NodeResult[ResearchState] {
    fmt.Println("Fetching tweets...")
    time.Sleep(1 * time.Second)

    return graph.NodeResult[ResearchState]{
        Delta: ResearchState{Tweets: []string{"Tweet 1", "Tweet 2"}},
        Next:  graph.Goto("merge"),
    }
}

func (n *FetchTweetsNode) Effects() graph.SideEffectPolicy {
    return graph.SideEffectPolicy{Recordable: true, RequiresIdempotency: true}
}

// FetchNewsNode calls news API
type FetchNewsNode struct {
    graph.DefaultPolicy
}

func (n *FetchNewsNode) Run(ctx context.Context, state ResearchState) graph.NodeResult[ResearchState] {
    fmt.Println("Fetching news...")
    time.Sleep(1 * time.Second)

    return graph.NodeResult[ResearchState]{
        Delta: ResearchState{News: []string{"News 1", "News 2"}},
        Next:  graph.Goto("merge"),
    }
}

func (n *FetchNewsNode) Effects() graph.SideEffectPolicy {
    return graph.SideEffectPolicy{Recordable: true, RequiresIdempotency: true}
}

// MergeNode combines results
type MergeNode struct {
    graph.DefaultPolicy
}

func (n *MergeNode) Run(ctx context.Context, state ResearchState) graph.NodeResult[ResearchState] {
    fmt.Println("Merging results...")

    summary := fmt.Sprintf(
        "Found %d papers, %d tweets, %d news articles",
        len(state.Papers), len(state.Tweets), len(state.News),
    )

    return graph.NodeResult[ResearchState]{
        Delta: ResearchState{Summary: summary},
        Next:  graph.Stop(),
    }
}
```

---

## Step 3: Define Deterministic Reducer

```go
// Reducer merges state deltas deterministically
func researchReducer(prev, delta ResearchState) ResearchState {
    // Merge query (last write wins)
    if delta.Query != "" {
        prev.Query = delta.Query
    }

    // Append arrays deterministically
    prev.Papers = append(prev.Papers, delta.Papers...)
    prev.Tweets = append(prev.Tweets, delta.Tweets...)
    prev.News = append(prev.News, delta.News...)

    // Update summary (last write wins)
    if delta.Summary != "" {
        prev.Summary = delta.Summary
    }

    return prev
}
```

---

## Step 4: Build Concurrent Graph

```go
func main() {
    ctx := context.Background()

    // Configure concurrent execution
    opts := graph.Options{
        MaxSteps:           100,
        MaxConcurrentNodes: 3,   // Run 3 nodes in parallel
        QueueDepth:         1024,
        DefaultNodeTimeout: 10 * time.Second,
        RunWallClockBudget: 5 * time.Minute,
    }

    // Create engine with in-memory store
    memStore := store.NewMemStore[ResearchState]()
    logEmitter := emit.NewLogEmitter()
    engine := graph.New(researchReducer, memStore, logEmitter, opts)

    // Add nodes
    engine.Add("fetch_papers", &FetchPapersNode{})
    engine.Add("fetch_tweets", &FetchTweetsNode{})
    engine.Add("fetch_news", &FetchNewsNode{})
    engine.Add("merge", &MergeNode{})

    // Add edges for fan-out (all 3 fetches start at "start")
    engine.AddEdge("start", "fetch_papers", nil)
    engine.AddEdge("start", "fetch_tweets", nil)
    engine.AddEdge("start", "fetch_news", nil)

    // All fetches converge to merge
    engine.AddEdge("fetch_papers", "merge", nil)
    engine.AddEdge("fetch_tweets", "merge", nil)
    engine.AddEdge("fetch_news", "merge", nil)

    // Start at virtual "start" node
    engine.StartAt("start")

    // Execute
    start := time.Now()
    initial := ResearchState{Query: "LLM applications"}
    final, err := engine.Run(ctx, "research-001", initial)
    elapsed := time.Since(start)

    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }

    fmt.Printf("\nFinal state: %+v\n", final)
    fmt.Printf("Execution time: %v (expected ~1s for concurrent, ~3s for sequential)\n", elapsed)
}
```

---

## Step 5: Run and Verify Concurrency

```bash
go run main.go
```

**Expected output**:
```
Fetching papers...
Fetching tweets...
Fetching news...
Merging results...

Final state: {
    Query: "LLM applications",
    Papers: ["Paper 1", "Paper 2"],
    Tweets: ["Tweet 1", "Tweet 2"],
    News: ["News 1", "News 2"],
    Summary: "Found 2 papers, 2 tweets, 2 news articles"
}
Execution time: ~1s (expected ~1s for concurrent, ~3s for sequential)
```

**Key observation**: All 3 fetch operations run simultaneously, reducing total time to ~1 second instead of 3 seconds sequential.

---

## Step 6: Add Retry Policy

Enhance nodes with automatic retry for transient failures:

```go
type RobustFetchNode struct {
    graph.DefaultPolicy
}

func (n *RobustFetchNode) Policy() graph.NodePolicy {
    return graph.NodePolicy{
        Timeout: 10 * time.Second,
        RetryPolicy: &graph.RetryPolicy{
            MaxAttempts: 3,
            BaseDelay:   time.Second,
            MaxDelay:    10 * time.Second,
            Retryable: func(err error) bool {
                // Only retry on timeout/rate limit errors
                return strings.Contains(err.Error(), "timeout") ||
                       strings.Contains(err.Error(), "rate limit")
            },
        },
    }
}

func (n *RobustFetchNode) Run(ctx context.Context, state ResearchState) graph.NodeResult[ResearchState] {
    // Simulate transient failure
    attempt := ctx.Value(graph.AttemptKey).(int)
    if attempt < 2 {
        return graph.NodeResult[ResearchState]{
            Err: fmt.Errorf("timeout: API unavailable"),  // Will retry
        }
    }

    // Success on 3rd attempt
    return graph.NodeResult[ResearchState]{
        Delta: ResearchState{Papers: []string{"Paper 1", "Paper 2"}},
        Next:  graph.Goto("merge"),
    }
}
```

---

## Step 7: Test Deterministic Replay

```go
func testReplay() {
    ctx := context.Background()

    // Step 1: Original execution (records I/O)
    opts := graph.Options{
        MaxConcurrentNodes: 3,
        ReplayMode:         false,  // Record mode
    }

    memStore := store.NewMemStore[ResearchState]()
    engine := buildEngine(opts, memStore)

    initial := ResearchState{Query: "LLM applications"}
    original, err := engine.Run(ctx, "replay-test", initial)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Original result: %+v\n", original)

    // Step 2: Replay execution (uses recorded I/O, no external calls)
    replayOpts := graph.Options{
        MaxConcurrentNodes: 3,
        ReplayMode:         true,   // Replay mode
        StrictReplay:       true,   // Fail on I/O mismatch
    }

    replayEngine := buildEngine(replayOpts, memStore)
    replayed, err := replayEngine.ReplayRun(ctx, "replay-test")
    if err != nil {
        panic(err)
    }

    fmt.Printf("Replayed result: %+v\n", replayed)

    // Verify identical results
    if original.Summary != replayed.Summary {
        panic("Replay mismatch!")
    }

    fmt.Println("✅ Replay successful: Results match exactly")
}
```

---

## Step 8: Checkpoint and Resume

```go
func testCheckpointResume() {
    ctx := context.Background()
    memStore := store.NewMemStore[ResearchState]()

    // Run until merge step
    opts := graph.Options{MaxConcurrentNodes: 3}
    engine := buildEngine(opts, memStore)

    initial := ResearchState{Query: "checkpoint test"}
    _, err := engine.Run(ctx, "checkpoint-test", initial)
    if err != nil {
        panic(err)
    }

    // Load checkpoint from step 2 (after fetches, before merge)
    checkpoint, err := memStore.LoadCheckpointV2(ctx, "checkpoint-test", 2)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Resuming from checkpoint at step %d\n", checkpoint.StepID)

    // Resume execution
    final, err := engine.RunWithCheckpoint(ctx, checkpoint)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Final state after resume: %+v\n", final)
}
```

---

## Key Concepts Demonstrated

### 1. Fan-Out Parallelism

```go
// Single entry point spawns 3 parallel branches
engine.AddEdge("start", "fetch_papers", nil)
engine.AddEdge("start", "fetch_tweets", nil)
engine.AddEdge("start", "fetch_news", nil)
```

All 3 fetch nodes execute concurrently, bounded by `MaxConcurrentNodes`.

---

### 2. Deterministic Merge

```go
// Reducer merges deltas in deterministic order
func researchReducer(prev, delta ResearchState) ResearchState {
    prev.Papers = append(prev.Papers, delta.Papers...)  // Order preserved
    // ...
}
```

Even though nodes finish in non-deterministic order, reducer applies deltas in deterministic `order_key` sequence.

---

### 3. Replay Without External Calls

```go
// Original execution
opts := graph.Options{ReplayMode: false}  // Records I/O
engine.Run(ctx, "run-001", initial)

// Replay execution
replayOpts := graph.Options{ReplayMode: true}  // Uses recorded I/O
replayEngine.ReplayRun(ctx, "run-001")  // No API calls!
```

---

### 4. Automatic Retries

```go
RetryPolicy: &graph.RetryPolicy{
    MaxAttempts: 3,
    BaseDelay:   time.Second,
    Retryable: func(err error) bool {
        return strings.Contains(err.Error(), "timeout")
    },
}
```

Engine automatically retries with exponential backoff + jitter.

---

## Performance Tuning

### Concurrency Level

```go
// Start with CPU count for I/O-bound workloads
MaxConcurrentNodes: runtime.NumCPU()

// Or match external service capacity
MaxConcurrentNodes: 10  // If API allows 10 concurrent requests
```

### Queue Depth

```go
// Rule of thumb: 100x concurrency level
QueueDepth: MaxConcurrentNodes * 100
```

### Timeouts

```go
DefaultNodeTimeout: 30 * time.Second  // Per-node timeout
RunWallClockBudget: 10 * time.Minute  // Total run timeout
```

---

## Troubleshooting

### Issue: Nodes execute sequentially despite MaxConcurrentNodes > 1

**Cause**: Nodes are not actually independent (dependencies via edges)

**Solution**: Verify graph topology. Use fan-out pattern:
```go
// ❌ Sequential
engine.AddEdge("A", "B", nil)
engine.AddEdge("B", "C", nil)

// ✅ Parallel
engine.AddEdge("start", "A", nil)
engine.AddEdge("start", "B", nil)
engine.AddEdge("start", "C", nil)
```

---

### Issue: Replay fails with ErrReplayMismatch

**Cause**: Node produced different output than recorded

**Solution**: Ensure nodes are deterministic. Use seeded RNG from context:
```go
// ❌ Non-deterministic
randomValue := rand.Intn(100)

// ✅ Deterministic
rng := ctx.Value(graph.RNGKey).(*rand.Rand)
randomValue := rng.Intn(100)
```

---

### Issue: Backpressure timeout errors

**Cause**: Frontier queue full for too long

**Solution**: Increase `QueueDepth` or `BackpressureTimeout`:
```go
opts := graph.Options{
    QueueDepth:          2048,  // Increase capacity
    BackpressureTimeout: 60 * time.Second,  // More patience
}
```

---

## Next Steps

1. **Explore Examples**: Check `examples/concurrent_workflow/` for more patterns
2. **Read Documentation**: See `docs/concurrency.md` for deep dive
3. **Production Deployment**: Use `store/mysql.go` for persistent checkpoints
4. **Observability**: Configure OpenTelemetry emitter for metrics/tracing
5. **Advanced Patterns**: Implement custom conflict resolution in reducer

---

## Summary

You've learned to:
- ✅ Execute graph nodes concurrently for performance
- ✅ Maintain deterministic state merging
- ✅ Replay executions without external calls
- ✅ Add automatic retry policies
- ✅ Save and resume from checkpoints

**Key Takeaway**: LangGraph-Go concurrency is deterministic by design. Same inputs always produce same outputs, even when execution order varies.

---

## Full Code Example

Complete working code: `examples/concurrent_workflow/main.go`

```bash
cd examples/concurrent_workflow
go run main.go
```

---

## Further Reading

- [Concurrency Model Documentation](../../../docs/concurrency.md)
- [Deterministic Replay Guide](../../../docs/replay.md)
- [API Reference](./contracts/engine_api.md)
- [Data Model](./data-model.md)
- [Research Decisions](./research.md)
