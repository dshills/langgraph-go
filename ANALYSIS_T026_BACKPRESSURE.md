# T026 Analysis: Frontier.Enqueue Backpressure Handling

## Summary

The `Frontier[S]` structure in `graph/scheduler.go` implements backpressure control for the work queue using a combination of a priority heap (for deterministic ordering) and a buffered channel (for capacity enforcement). When the frontier queue reaches its configured capacity, `Enqueue()` blocks the caller until space becomes available or context cancellation occurs, providing natural backpressure that prevents unbounded memory growth.

## Frontier Interface

The `Frontier[S]` type is not explicitly defined as an interface, but it provides the following core methods:

```go
// graph/scheduler.go, line 114-157
type Frontier[S any] struct {
    heap                workHeap[S]         // Priority queue for deterministic ordering (single source of truth)
    queue               chan struct{}       // Notification channel (empty struct, no data)
    capacity            int                 // Maximum queue depth
    ctx                 context.Context     // Context for cancellation
    mu                  sync.Mutex          // Protects heap and len operations

    // Metrics tracking (T068) - use atomic operations for thread-safe updates
    totalEnqueued       atomic.Int64        // Total work items enqueued
    totalDequeued       atomic.Int64        // Total work items dequeued
    backpressureEvents  atomic.Int32        // Count of backpressure triggers
    peakQueueDepth      atomic.Int32        // Maximum queue depth observed

    // US3: Observability parameters for backpressure monitoring
    runID               string              // Run identifier for event/metric attribution
    metrics             *PrometheusMetrics  // Optional Prometheus metrics collector
    emitter             emit.Emitter        // Optional event emitter
}
```

**Key Design (BUG-003 fix, T019-T021):**
- **Heap**: Single source of truth for work item storage and ordering
- **Channel**: Notification-only (carries empty struct), not a data carrier
- **Architecture**: Enqueue pushes to heap THEN sends notification to channel; Dequeue waits for notification THEN pops from heap
- **Benefit**: Prevents heap/channel desynchronization and reduces memory usage

## Enqueue Implementation

### Signature
```go
func (f *Frontier[S]) Enqueue(ctx context.Context, item WorkItem[S]) error
```

### Location
File: `/Users/dshills/Development/projects/langgraph-go/graph/scheduler.go`  
Lines: 172-208

### Detailed Behavior

```go
// Simplified flow (see lines 172-208 for full implementation)
func (f *Frontier[S]) Enqueue(ctx context.Context, item WorkItem[S]) error {
    // 1. Check context first for fast failure
    if ctx.Err() != nil {
        return ctx.Err()
    }

    // 2. Add to heap under lock (deterministic ordering)
    f.mu.Lock()
    heap.Push(&f.heap, item)
    currentDepth := int32(f.heap.Len())
    f.mu.Unlock()

    // 3. Update metrics: track peak queue depth (T068)
    for {
        oldPeak := f.peakQueueDepth.Load()
        if currentDepth <= oldPeak || f.peakQueueDepth.CompareAndSwap(oldPeak, currentDepth) {
            break
        }
    }

    // 4. US3 (T027-T030): Backpressure observability with metrics and events
    // When queue depth reaches capacity, emit backpressure event
    if currentDepth >= int32(f.capacity) {
        f.backpressureEvents.Add(1)
        if f.metrics != nil {
            f.metrics.IncrementBackpressure(f.runID, "queue_full")
        }
        waitStart := time.Now()
        if f.emitter != nil {
            go f.emitter.Emit(emit.Event{
                RunID: f.runID, Step: item.StepID, NodeID: item.NodeID,
                Msg: "backpressure",
                Meta: map[string]interface{}{
                    "queue_depth": currentDepth,
                    "capacity": f.capacity,
                    "node_id": item.NodeID,
                    "order_key": item.OrderKey,
                }})
        }
        
        // Block until space available or context cancelled
        select {
        case <-ctx.Done():
            return ctx.Err()
        case f.queue <- struct{}{}:  // THIS BLOCKS IF CHANNEL FULL
            f.totalEnqueued.Add(1)
            if time.Since(waitStart) > time.Millisecond && f.emitter != nil {
                go f.emitter.Emit(emit.Event{
                    RunID: f.runID, Step: item.StepID, NodeID: item.NodeID,
                    Msg: "backpressure_resolved",
                    Meta: map[string]interface{}{
                        "wait_duration_ms": time.Since(waitStart).Milliseconds(),
                        "queue_depth": currentDepth,
                    }})
            }
            return nil
        }
    }
    
    // 5. Normal path: send to channel (same select pattern)
    select {
    case <-ctx.Done():
        return ctx.Err()
    case f.queue <- struct{}{}:
        f.totalEnqueued.Add(1)
        return nil
    }
}
```

### Key Features

1. **Fast Path Check**: Early context validation before lock acquisition
2. **Heap Protection**: Mutex-protected heap operations ensure thread-safe insertion
3. **Deterministic Ordering**: Items stored in priority queue by OrderKey
4. **Atomic Peak Tracking**: Lock-free peak queue depth recording (T068)
5. **Backpressure Detection**: Automatic detection when `queue_depth >= capacity`
6. **Observability Events**: Async emission of backpressure and backpressure_resolved events
7. **Metrics Integration**: Increments backpressure_events_total counter
8. **Context Awareness**: Respects context cancellation during blocking

## Queue Capacity Configuration

### Configuration Method
```go
// graph/options.go, lines 106-121
func WithQueueDepth(n int) Option {
    return func(cfg *engineConfig) error {
        cfg.opts.QueueDepth = n
        return nil
    }
}
```

### Default and Tuning
- **Default**: 1024 work items
- **Recommendation**: `MaxConcurrentNodes × 100` for initial estimate
- **Rationale**: Provides buffer for bursty work production while limiting memory growth
- **Use Cases**:
  - Large fan-outs: Increase to 2048+ to absorb parallel branch explosions
  - Memory-constrained: Decrease to 512 or less
  - Deterministic workflows: Fine-tune based on typical fan-out patterns

### Integration with Engine

```go
// graph/engine.go, line 984 (initial work item enqueue)
if err := e.frontier.Enqueue(ctx, initialItem); err != nil {
    return zero, err
}

// Engine initialization (concurrent mode):
// Engine.Run() initializes frontier with:
queueDepth := e.opts.QueueDepth
if queueDepth == 0 {
    queueDepth = 1024  // Default
}
e.frontier = NewFrontier[S](ctx, queueDepth, runID, e.opts.Metrics, e.emitter)
```

## Blocking Behavior

### When Does Enqueue Block?

1. **Channel Full Condition**: When `currentDepth >= capacity` (line 196-208)
   - Item is already in heap
   - Send operation on buffered channel blocks
   - Channel buffer has exactly `capacity` slots
   - All slots occupied by pending Dequeue notifications

2. **Context Cancellation During Block**: 
   - `select` statement (line 203-208) provides escape hatch
   - If context is cancelled while blocked, returns `ctx.Err()`
   - Allows graceful shutdown during backpressure

### Blocking Mechanism Details

```go
// The buffered channel provides bounded capacity:
queue: make(chan struct{}, capacity)

// When full, this send operation blocks:
case f.queue <- struct{}{}:  // Line 205 (normal path) or 205 (backpressure path)
```

**Key Property**: The channel send operation is atomic from the goroutine perspective. 
Once the send unblocks (space available), the item is guaranteed to be:
- In the heap (already pushed)
- Notified in the channel (notification sent)
- Ready for dequeue

### Release of Backpressure

Backpressure is relieved when:
1. A worker goroutine calls `Dequeue()`
2. `Dequeue()` receives notification from channel
3. A slot becomes available in the buffered channel
4. A blocked `Enqueue()` call can send notification
5. New work item becomes available for processing

```go
// graph/scheduler.go, lines 210-226 (Dequeue)
func (f *Frontier[S]) Dequeue(ctx context.Context) (WorkItem[S], error) {
    // ...
    case <-f.queue:  // Receives notification (empty struct discarded)
        f.mu.Lock()
        defer f.mu.Unlock()
        if f.heap.Len() == 0 {
            return zero, context.Canceled
        }
        item := heap.Pop(&f.heap).(WorkItem[S])
        f.totalDequeued.Add(1)
        return item, nil
    // ...
}
```

## Call Sites

### Primary Callsites in Engine

1. **Initial Work Enqueue** (graph/engine.go:984)
   ```go
   // First work item to start execution
   if err := e.frontier.Enqueue(ctx, initialItem); err != nil {
       return zero, err
   }
   ```

2. **Retry Enqueue** (graph/engine.go:1228)
   ```go
   // Re-enqueue after backoff delay when retryable error occurs
   if err := e.frontier.Enqueue(workerCtx, retryItem); err != nil {
       sendErrorAndCancel(result.Err)
       return
   }
   ```

3. **Fan-out Branch Enqueue** (graph/engine.go:1302)
   ```go
   // Enqueue work items for parallel branches (Next.Many)
   for edgeIdx, branchID := range result.Route.Many {
       branchItem := WorkItem[S]{...}
       if err := e.frontier.Enqueue(workerCtx, branchItem); err != nil {
           results <- nodeResult[S]{err: err}
           cancel()
           return
       }
   }
   ```

4. **Single Next Node Enqueue** (graph/engine.go:1327, 1363)
   ```go
   // Enqueue next node after Goto routing decision
   nextItem := WorkItem[S]{...}
   if err := e.frontier.Enqueue(workerCtx, nextItem); err != nil {
       results <- nodeResult[S]{err: err}
       cancel()
       return
   }
   ```

5. **Checkpoint Restoration** (graph/engine.go:2098)
   ```go
   // Enqueue work items when resuming from checkpoint
   for _, item := range frontierItems {
       if err := e.frontier.Enqueue(ctx, item); err != nil {
           return zero, &EngineError{...}
       }
   }
   ```

### Test Callsites

- `graph/scheduler_test.go`: Lines 134, 226, 254, 312, 331
- `graph/scheduler_standalone_test.go`: Lines 61, 96, 106, 120, 157

### Caller Context Analysis

All production callers are from **worker goroutines** running under:
- `workerCtx` (derived from engine context with timeout)
- Concurrent execution mode only
- After node execution completes (in result routing phase)
- During retry scheduling (with backoff delay applied)
- From checkpoint restoration

**No blocking is intentional** - if Enqueue blocks, it blocks a worker goroutine,
reducing concurrency temporarily. This is correct behavior: backpressure should
slow down work production to match consumption rate.

## Integration Points for Backpressure Observability

### 1. Metrics Integration (T068)

**Counter**: `backpressure_events_total`
- Incremented when `currentDepth >= capacity` (line 196)
- Labels: `run_id`, `reason` (always "queue_full")
- Prometheus metric: `langgraph_backpressure_events_total{run_id="...", reason="queue_full"}`

```go
// Line 197-198 in scheduler.go
f.backpressureEvents.Add(1)
if f.metrics != nil {
    f.metrics.IncrementBackpressure(f.runID, "queue_full")
}
```

**Retrieved via**: `Frontier[S].Metrics()` method (line 268-280)
```go
func (f *Frontier[S]) Metrics() SchedulerMetrics {
    return SchedulerMetrics{
        QueueDepth:         currentQueueDepth,
        QueueCapacity:      int32(f.capacity),
        TotalEnqueued:      f.totalEnqueued.Load(),
        TotalDequeued:      f.totalDequeued.Load(),
        BackpressureEvents: f.backpressureEvents.Load(),  // T068
        PeakQueueDepth:     f.peakQueueDepth.Load(),      // T068
    }
}
```

### 2. Event Emission (US3, T027-T030)

**Backpressure Event** (line 199-207)
```go
if f.emitter != nil {
    go f.emitter.Emit(emit.Event{
        RunID: f.runID,
        Step: item.StepID,
        NodeID: item.NodeID,
        Msg: "backpressure",
        Meta: map[string]interface{}{
            "queue_depth": currentDepth,
            "capacity": f.capacity,
            "node_id": item.NodeID,
            "order_key": item.OrderKey,
        }})
}
```

**Backpressure Resolved Event** (line 209-214)
```go
if time.Since(waitStart) > time.Millisecond && f.emitter != nil {
    go f.emitter.Emit(emit.Event{
        RunID: f.runID,
        Step: item.StepID,
        NodeID: item.NodeID,
        Msg: "backpressure_resolved",
        Meta: map[string]interface{}{
            "wait_duration_ms": time.Since(waitStart).Milliseconds(),
            "queue_depth": currentDepth,
        }})
}
```

### 3. Peak Queue Depth Tracking (T068)

Lock-free atomic Compare-And-Swap loop (line 185-191):
```go
for {
    oldPeak := f.peakQueueDepth.Load()
    if currentDepth <= oldPeak || f.peakQueueDepth.CompareAndSwap(oldPeak, currentDepth) {
        break
    }
}
```

Benefits:
- Non-blocking metric collection
- Safe for concurrent access
- Prevents lock contention on the heap mutex
- Tracks maximum observed queue depth across entire run

## Recommendations for Backpressure Detection and Implementation

### 1. Integrate Metrics Monitoring
```go
// Best practice: enable metrics collection
metrics := graph.NewPrometheusMetrics(registry)
engine := graph.New(
    reducer, store, emitter,
    graph.WithMaxConcurrent(16),
    graph.WithQueueDepth(2048),
    graph.WithMetrics(metrics),
)

// Periodically check backpressure events:
// SELECT langgraph_backpressure_events_total WHERE reason='queue_full'
// If rate > 0, increase QueueDepth or MaxConcurrentNodes
```

### 2. Event-Driven Alerting
```go
// Implement custom emitter to track backpressure events
type AlertingEmitter struct {
    delegate emit.Emitter
    alert    AlertFunc
}

func (ae *AlertingEmitter) Emit(event emit.Event) {
    if event.Msg == "backpressure" {
        meta := event.Meta.(map[string]interface{})
        queueDepth := meta["queue_depth"].(int32)
        capacity := meta["capacity"].(int)
        utilization := float64(queueDepth) / float64(capacity)
        
        if utilization > 0.8 {  // Alert at 80% capacity
            ae.alert(fmt.Sprintf(
                "Queue saturation: %d/%d (%.1f%%)",
                queueDepth, capacity, utilization*100))
        }
    }
    ae.delegate.Emit(event)
}
```

### 3. Tuning Guidance
```
For workflows with measured fan-out F and concurrency C:

Initial Settings:
  QueueDepth = C × 100  (provides 100-item buffer per worker)
  MaxConcurrentNodes = C

Backpressure Observed?
  → Increase QueueDepth to 2×, 5×, or 10× initial value
  → OR Increase MaxConcurrentNodes if CPU/memory allows
  → Consider parallelizing node execution (horizontal scaling)

Monitor:
  1. backpressure_events_total counter (should be 0 or near-0)
  2. peak_queue_depth metric (should be << capacity)
  3. backpressure event duration (should be < 100ms typically)
  4. node execution latency (should not spike during backpressure)
```

### 4. Context-Aware Cancellation Handling
The current implementation correctly handles context cancellation:
```go
// When context is cancelled during Enqueue block:
select {
case <-ctx.Done():
    return ctx.Err()  // Unblocks gracefully
case f.queue <- struct{}{}:
    return nil
}
```

This enables:
- Graceful shutdown: Cancel context when scaling down
- Timeout enforcement: Use context with deadline
- Per-request timeouts: Each worker has its own workerCtx

### 5. Distributed Tracing Integration
For OpenTelemetry integration, backpressure events should:
```go
// Emit backpressure span
span, ctx := tracer.Start(ctx, "queue_backpressure",
    trace.WithAttributes(
        attribute.String("runID", f.runID),
        attribute.Int("queue_depth", int(currentDepth)),
        attribute.Int("capacity", f.capacity),
    ))
defer span.End()
// ... wait for space ...
span.AddEvent("backpressure_resolved")
```

### 6. Load Testing and Benchmarking
```go
// Test backpressure behavior under load
// file: graph/scheduler_test.go (extend existing tests)

func TestBackpressureBlocking(t *testing.T) {
    // Create frontier with small capacity
    frontier := NewFrontier[int](ctx, 10, "test-run", nil, nil)
    
    // Enqueue 10 items quickly (fills channel)
    for i := 0; i < 10; i++ {
        item := WorkItem[int]{...}
        frontier.Enqueue(ctx, item)
    }
    
    // Next enqueue should block
    start := time.Now()
    done := make(chan struct{})
    go func() {
        frontier.Enqueue(ctx, WorkItem[int]{...})
        done <- struct{}{}
    }()
    
    // Verify blocking (no result for 10ms)
    select {
    case <-done:
        t.Fatal("expected enqueue to block")
    case <-time.After(10 * time.Millisecond):
        // Good: still blocking
    }
    
    // Dequeue one item to unblock
    frontier.Dequeue(ctx)
    
    // Now enqueue should complete
    select {
    case <-done:
        duration := time.Since(start)
        if duration < 10*time.Millisecond {
            t.Fatalf("blocked for less than expected: %v", duration)
        }
    case <-time.After(100 * time.Millisecond):
        t.Fatal("expected enqueue to unblock after dequeue")
    }
}
```

## Summary Table

| Aspect | Details |
|--------|---------|
| **Frontier Type** | `Frontier[S]` struct (not explicit interface) |
| **Blocking Mechanism** | Buffered channel `queue chan struct{}` with capacity limit |
| **Capacity Configuration** | `WithQueueDepth(n)` option, default 1024 |
| **Blocking Condition** | `heap.Len() >= capacity` when sending to full channel |
| **Release Condition** | `Dequeue()` receives notification, freeing channel slot |
| **Context Handling** | `select` with `ctx.Done()` allows cancellation during block |
| **Observability** | Metrics counter + events (backpressure, backpressure_resolved) |
| **Atomic Metrics** | `totalEnqueued`, `totalDequeued`, `backpressureEvents`, `peakQueueDepth` |
| **Thread Safety** | Heap operations protected by mutex; metrics use atomics |
| **Determinism** | OrderKey ensures consistent ordering across concurrent dequeues |
| **Default Tuning** | `QueueDepth = MaxConcurrentNodes × 100` recommended |

## References

- **Specification**: `specs/005-critical-bug-fixes/` (BUG-003 fix, T019-T021)
- **Observability Spec**: T027-T030 (US3: Backpressure Observability)
- **Metrics Spec**: T032-T039 (Prometheus Integration)
- **Engine Integration**: `graph/engine.go` lines 984, 1228, 1302, 1327, 1363
- **Options Configuration**: `graph/options.go` lines 106-121
- **Test Coverage**: `graph/scheduler_test.go`, `graph/scheduler_standalone_test.go`
