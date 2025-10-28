# Streaming Support

LangGraph-Go's current approach to streaming, workarounds, and future roadmap.

## Current Status

**Streaming is not yet natively supported in LangGraph-Go v0.2.x.**

The framework currently uses a **batch execution model** where:
- Workflows execute completely before returning final state
- Intermediate results are not streamed to callers
- All node outputs are collected before reducer application

## Why Not Streaming?

The design prioritizes **deterministic replay and exactly-once semantics**:

1. **Checkpoint Consistency**: State must be committed atomically
2. **Deterministic Ordering**: Results merge in deterministic order
3. **Replay Guarantees**: Replays must produce identical outputs
4. **Transaction Boundaries**: State changes are atomic

Streaming would complicate these guarantees, requiring:
- Partial state snapshots
- Stream resumption logic
- Non-deterministic event ordering
- Complex error recovery

## Workarounds

### Pattern 1: Buffer in State

Accumulate results in state, expose via polling:

```go
type StreamableState struct {
    Query       string
    Chunks      []string  // Accumulated LLM chunks
    Complete    bool
    Error       error
}

func streamingNode(ctx context.Context, s StreamableState) graph.NodeResult[StreamableState] {
    var chunks []string

    // Call LLM with streaming
    stream := llm.ChatStream(ctx, s.Query)
    for chunk := range stream {
        chunks = append(chunks, chunk.Text)
    }

    return graph.NodeResult[StreamableState]{
        Delta: StreamableState{
            Chunks:   chunks,
            Complete: true,
        },
        Route: graph.Stop(),
    }
}

// External polling
func pollWorkflow(ctx context.Context, runID string) {
    ticker := time.NewTicker(500 * time.Millisecond)
    defer ticker.Stop()

    for range ticker.C {
        checkpoint, _ := store.LoadCheckpointV2(ctx, runID, "")

        // Display accumulated chunks
        for _, chunk := range checkpoint.State.Chunks {
            fmt.Print(chunk)
        }

        if checkpoint.State.Complete {
            break
        }
    }
}
```

**Pros:**
- ✅ Works with current architecture
- ✅ Maintains determinism guarantees
- ✅ Simple to implement

**Cons:**
- ❌ Polling overhead
- ❌ Latency in chunk delivery
- ❌ Not true streaming

### Pattern 2: Event-Based Progress Updates

Use event emitters for progress reporting:

```go
type ProgressState struct {
    TotalSteps    int
    CompletedSteps int
    CurrentNode   string
    Status        string
}

func progressNode(ctx context.Context, s ProgressState) graph.NodeResult[ProgressState] {
    // Emit progress event
    events := []graph.Event{
        {
            Type: "progress",
            Data: map[string]interface{}{
                "completed": s.CompletedSteps + 1,
                "total":     s.TotalSteps,
                "node":      "processing",
            },
        },
    }

    // Do work
    result := processData(s)

    return graph.NodeResult[ProgressState]{
        Delta:  result,
        Events: events,
        Route:  graph.Goto("next"),
    }
}

// Listen to events
emitter := emit.NewBufferedEmitter(1000)
engine := graph.New(reducer, store, emitter, opts)

go func() {
    for event := range emitter.Events() {
        if event.Type == "progress" {
            fmt.Printf("Progress: %v\n", event.Data)
        }
    }
}()
```

**Pros:**
- ✅ Real-time progress updates
- ✅ Works with existing event system
- ✅ No polling required

**Cons:**
- ❌ Not streaming node outputs
- ❌ Requires async event handling
- ❌ Events may arrive out of order

### Pattern 3: Micro-Batching

Break large tasks into small batches:

```go
type BatchState struct {
    Items       []Item
    Processed   []Result
    BatchSize   int
    BatchIndex  int
}

func batchProcessor(ctx context.Context, s BatchState) graph.NodeResult[BatchState] {
    start := s.BatchIndex * s.BatchSize
    end := min(start+s.BatchSize, len(s.Items))

    if start >= len(s.Items) {
        // All batches complete
        return graph.NodeResult[BatchState]{
            Route: graph.Stop(),
        }
    }

    // Process batch
    batch := s.Items[start:end]
    results := processBatch(batch)

    delta := BatchState{
        Processed:  results,
        BatchIndex: s.BatchIndex + 1,
    }

    return graph.NodeResult[BatchState]{
        Delta: delta,
        Route: graph.Goto("batch-processor"), // Loop back
    }
}

// Caller can poll checkpoint after each batch
func procesWithProgress(ctx context.Context, items []Item) {
    runID := "batch-job-001"
    state := BatchState{
        Items:     items,
        BatchSize: 10,
    }

    go engine.Run(ctx, runID, state)

    // Poll for progress
    ticker := time.NewTicker(1 * time.Second)
    defer ticker.Stop()

    for range ticker.C {
        checkpoint, _ := store.LoadCheckpointV2(ctx, runID, "")
        progress := float64(len(checkpoint.State.Processed)) / float64(len(items))
        fmt.Printf("Progress: %.1f%%\n", progress*100)

        if len(checkpoint.State.Processed) == len(items) {
            break
        }
    }
}
```

**Pros:**
- ✅ Incremental progress visibility
- ✅ Resumable on failure
- ✅ Works with checkpointing

**Cons:**
- ❌ Still requires polling
- ❌ Overhead from checkpoint saves
- ❌ Not suitable for fine-grained streaming

### Pattern 4: External Streaming Proxy

Run LLM streaming outside graph, aggregate in graph:

```go
// External streaming service
func streamLLMResponse(ctx context.Context, query string) <-chan string {
    chunks := make(chan string)

    go func() {
        defer close(chunks)
        stream := llm.ChatStream(ctx, query)

        for chunk := range stream {
            chunks <- chunk.Text
        }
    }()

    return chunks
}

// Graph node aggregates completed stream
func aggregateNode(ctx context.Context, s State) graph.NodeResult[State] {
    // Stream already completed externally
    response := getCompletedResponse(s.StreamID)

    return graph.NodeResult[State]{
        Delta: State{Response: response},
        Route: graph.Stop(),
    }
}

// Usage
func streamToClient(w http.ResponseWriter, query string) {
    // Start streaming immediately
    streamID := uuid.New().String()
    chunks := streamLLMResponse(context.Background(), query)

    // Stream to client
    for chunk := range chunks {
        fmt.Fprintf(w, "data: %s\n\n", chunk)
        w.(http.Flusher).Flush()
    }

    // Start workflow with completed response
    go func() {
        state := State{
            StreamID: streamID,
            Query:    query,
        }
        engine.Run(context.Background(), "workflow-"+streamID, state)
    }()
}
```

**Pros:**
- ✅ True streaming to clients
- ✅ Graph maintains determinism
- ✅ No polling needed

**Cons:**
- ❌ Streaming outside graph (not replayable)
- ❌ Two execution paths (stream + graph)
- ❌ Complex error handling

## Future: Native Streaming Support

**Planned for v0.4.0**: First-class streaming support with deterministic guarantees.

### Design Goals

1. **Stream node outputs incrementally** while maintaining determinism
2. **Replay streaming executions** from checkpoints
3. **Support both push and pull** streaming models
4. **Preserve exactly-once semantics** for streamed events

### Proposed API

```go
// Future API (not yet implemented)

// StreamableNode returns chunks incrementally
type StreamableNode[S any] interface {
    RunStream(ctx context.Context, state S) <-chan StreamResult[S]
}

type StreamResult[S any] {
    Chunk   S           // Partial state update
    Delta   S           // Accumulated delta
    IsFinal bool        // Last chunk
    Err     error
}

// Enable streaming mode
opts := graph.Options{
    StreamingEnabled: true,
}

engine := graph.New(reducer, store, emitter, opts)

// Stream execution
stream := engine.RunStream(ctx, runID, initialState)
for result := range stream {
    fmt.Printf("Chunk: %v\n", result.Chunk)

    if result.IsFinal {
        fmt.Printf("Final state: %v\n", result.Delta)
        break
    }
}
```

### Deterministic Streaming

How streaming will maintain determinism:

1. **Chunked Checkpoints**: Save checkpoint after each chunk
2. **Chunk Ordering**: Assign order keys to chunks within node
3. **Replay Streams**: Replay produces identical chunk sequence
4. **Buffered Merge**: Accumulate chunks before reducer application

### Challenges

- **Checkpoint Overhead**: Frequent checkpoint saves
- **State Bloat**: Storing all chunks in checkpoints
- **Replay Performance**: Replaying long streams
- **Backpressure**: Handling slow consumers

**Status**: Under active design - feedback welcome!

## Recommendations

**For Current Projects:**

1. ✅ Use **Pattern 1 (Buffer in State)** for simple cases
2. ✅ Use **Pattern 2 (Event Progress)** for coarse-grained updates
3. ✅ Use **Pattern 3 (Micro-Batching)** for long-running jobs
4. ✅ Use **Pattern 4 (External Proxy)** when true streaming required

**When Native Streaming Arrives (v0.4.0):**

- Migration path will be provided
- Existing patterns will continue to work
- Opt-in to streaming mode
- Backward compatibility maintained

## Related Documentation

- [State Management](./guides/03-state-management.md) - State design patterns
- [Event Tracing](./guides/08-event-tracing.md) - Event system details
- [Checkpoints & Resume](./guides/04-checkpoints.md) - Checkpoint patterns
- [Concurrency Model](./concurrency.md) - Execution model

## Summary

**Current State (v0.2.x):**
- ❌ No native streaming support
- ✅ Workarounds available (buffering, events, batching, proxy)
- ✅ Maintains determinism and replay guarantees

**Future (v0.4.0):**
- ✅ Native streaming with deterministic guarantees
- ✅ Streamable nodes with incremental output
- ✅ Chunk-level checkpointing and replay
- ✅ Backward compatible migration

**Use workarounds now, migrate to native streaming when available.**

For questions or feature requests, please open an issue on GitHub.
