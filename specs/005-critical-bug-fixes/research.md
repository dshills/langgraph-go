# Research: Critical Concurrency Bug Fixes

**Feature**: 005-critical-bug-fixes  
**Date**: 2025-10-29  
**Purpose**: Research Go concurrency best practices for fixing 4 critical bugs in concurrent workflow execution

---

## R1: Channel Buffer Sizing for Worker Pool Results

### Decision: Results channel buffer = MaxConcurrentNodes * 2

**Rationale**:
- Each worker can produce exactly one result (success or error)
- Worst case: all workers fail simultaneously
- 2x buffer provides safety margin for completion coordination
- Prevents blocking on error delivery while maintaining bounded memory
- Standard Go pattern: buffer size = max concurrent producers

**Alternatives Considered**:

**Alternative A: Unbuffered channel**
- Pros: Minimal memory, immediate backpressure
- Cons: Workers must block until result is consumed, reduces parallelism
- Rejected: Blocking workers defeats concurrency benefits

**Alternative B: Buffer = MaxConcurrentNodes exactly**
- Pros: Minimal memory
- Cons: No safety margin, completion goroutine may race with error sends
- Rejected: Tight coupling between buffer and worker count risks edge cases

**Alternative C: Large fixed buffer (e.g., 1000)**
- Pros: Never blocks
- Cons: Wastes memory for small MaxConcurrentNodes, masks backpressure issues
- Rejected: Wasteful and doesn't scale with configuration

**Code Example**:
```go
// In runConcurrent():
results := make(chan nodeResult[S], e.opts.MaxConcurrentNodes*2)
```

**Performance Impact**:
- Memory: +16 bytes per buffer slot * 2 * MaxConcurrentNodes
- For MaxConcurrentNodes=8: ~256 bytes (negligible)
- For MaxConcurrentNodes=100: ~3.2KB (acceptable)
- No throughput impact (channels are fast)

---

## R2: Thread-Safe RNG with Deterministic Seeding

### Decision: Per-worker RNG instances with derived seeds from base RNG

**Rationale**:
- math/rand.Rand is NOT thread-safe (documented in Go stdlib)
- Creating separate Rand per goroutine is standard Go practice
- Deriving seeds from base RNG maintains determinism
- Base RNG seeded with SHA-256(runID) ensures reproducibility
- Worker-specific offsets (workerID) ensure unique but deterministic seeds

**Alternatives Considered**:

**Alternative A: Mutex-protected shared RNG**
- Pros: Single RNG instance, simple
- Cons: Serializes all RNG calls, severe performance bottleneck
- Rejected: Defeats concurrency benefits, 10-100x slower

**Alternative B: Global RNG with lock-free access**
- Pros: No explicit synchronization
- Cons: Not thread-safe, causes data races (current bug!)
- Rejected: This is the bug we're fixing

**Alternative C: crypto/rand for each call**
- Pros: Thread-safe, cryptographically secure
- Cons: 1000x slower, non-deterministic (breaks replay)
- Rejected: Performance and determinism requirements

**Alternative D: math/rand/v2 (Go 1.22+)**
- Pros: Thread-safe global RNG
- Cons: Requires Go 1.22+, project targets Go 1.21+
- Deferred: Consider for future when Go 1.22 is minimum version

**Code Example**:
```go
func (e *Engine[S]) runConcurrent(ctx context.Context, runID string, initial S) (S, error) {
    // Create base RNG with deterministic seed
    baseRNG := initRNG(runID)  // SHA-256(runID) → seed
    
    for i := 0; i < maxWorkers; i++ {
        wg.Add(1)
        
        // Derive unique but deterministic seed for this worker
        workerSeed := baseRNG.Int63()
        
        go func(workerID int) {
            defer wg.Done()
            
            // Each worker gets its own RNG
            workerRNG := rand.New(rand.NewSource(workerSeed + int64(workerID)))
            nodeCtx := context.WithValue(workerCtx, RNGKey, workerRNG)
            
            // Worker loop uses nodeCtx for RNG access
            // ...
        }(i)
    }
}
```

**Determinism Validation**:
```go
// Test: Same runID → same worker seeds → same random sequences
runID := "test-run-001"
rng1 := initRNG(runID)
seed1_worker0 := rng1.Int63()

rng2 := initRNG(runID)
seed2_worker0 := rng2.Int63()

assert(seed1_worker0 == seed2_worker0)  // Deterministic
```

**Performance Impact**:
- Memory: +24 bytes per worker (rand.Rand instance)
- For MaxConcurrentNodes=8: ~192 bytes
- CPU: No contention, each worker uses independent RNG
- Throughput: Neutral to slight improvement (no lock contention)

---

## R3: Frontier Heap/Channel Synchronization Pattern

### Decision: Heap as single source of truth, channel as notification mechanism

**Rationale**:
- Heap (container/heap) provides O(log n) priority ordering
- Channel provides blocking/awakening for idle workers
- Keeping both synchronized is error-prone (current bug)
- Channel should be notification-only (empty struct)
- Heap Pop after channel receive ensures OrderKey ordering

**Alternatives Considered**:

**Alternative A: Dual-structure (current implementation)**
- Pros: Both heap and channel hold data
- Cons: Synchronization bugs, complex invariants
- Rejected: This is the bug we're fixing

**Alternative B: Condition variable instead of channel**
- Pros: No buffer, no synchronization drift
- Cons: Requires manual broadcast/signal, more complex than channels
- Rejected: Channels are more idiomatic in Go

**Alternative C: Channel-only (no heap)**
- Pros: Simple, no sync issues
- Cons: Cannot maintain priority ordering (OrderKey)
- Rejected: Breaks determinism guarantee

**Alternative D: Heap-only with condition variable**
- Pros: Single data structure
- Cons: Less idiomatic than channels in Go
- Considered but channels preferred for Go idioms

**Code Example**:
```go
type Frontier[S any] struct {
    heap  workHeap[S]           // Priority queue (ordered by OrderKey)
    queue chan struct{}          // Notification channel (empty struct)
    mu    sync.Mutex
}

func (f *Frontier[S]) Enqueue(ctx context.Context, item WorkItem[S]) error {
    // Add to heap
    f.mu.Lock()
    heap.Push(&f.heap, item)
    heapLen := f.heap.Len()
    f.mu.Unlock()
    
    // Send notification (non-blocking if queue full = backpressure)
    select {
    case f.queue <- struct{}{}:  // Empty struct (zero bytes)
        return nil
    case <-ctx.Done():
        return ctx.Err()
    case <-time.After(f.backpressureTimeout):
        return ErrBackpressureTimeout
    }
}

func (f *Frontier[S]) Dequeue(ctx context.Context) (WorkItem[S], error) {
    // Wait for notification
    select {
    case <-ctx.Done():
        return zero, ctx.Err()
    case <-f.queue:
        // Notification received, pop from heap
        f.mu.Lock()
        defer f.mu.Unlock()
        
        if f.heap.Len() == 0 {
            // Should never happen if synchronized correctly
            return zero, fmt.Errorf("frontier desync: notification without work")
        }
        
        item := heap.Pop(&f.heap).(WorkItem[S])
        return item, nil
    }
}
```

**Synchronization Guarantee**:
- Enqueue: heap.Push THEN queue send
- Dequeue: queue receive THEN heap.Pop
- Invariant: queue notifications ≤ heap items (notifications may be lost on backpressure, but heap is source of truth)

**Performance Impact**:
- Empty struct{} channel: 0 bytes per notification (vs WorkItem[S] currently)
- Reduced contention: one lock per enqueue/dequeue (same as before)
- Throughput: Neutral (synchronization cost unchanged)
- Memory: Reduced (no duplicate work item storage in channel)

---

## R4: Completion Detection Without Polling or Races

### Decision: Atomic completion flag + explicit signaling on last worker idle

**Rationale**:
- Polling every 10ms has inherent race window
- Atomic flag provides race-free completion state
- Signal-based detection is immediate (no delay)
- Combine frontier emptiness check with worker idle check atomically

**Alternatives Considered**:

**Alternative A: Polling with ticker (current)**
- Pros: Simple, no complex coordination
- Cons: Race window, 10ms delay, continuous CPU usage
- Rejected: This is the bug we're fixing

**Alternative B: Channel-based completion signal**
- Pros: No polling, immediate notification
- Cons: Who sends the signal? Last worker needs to know it's last
- Complexity: Determining "last worker" is rac

y

**Alternative C: WaitGroup + atomic flag**
- Pros: WaitGroup tracks worker lifecycle, atomic flag prevents races
- Cons: Requires careful coordination
- Selected: Best balance of safety and performance

**Alternative D: Condition variable with broadcast**
- Pros: Standard synchronization primitive
- Cons: More complex than atomics, requires mutex
- Rejected: Atomics are simpler for binary state

**Code Example**:
```go
func (e *Engine[S]) runConcurrent(ctx context.Context, runID string, initial S) (S, error) {
    var completionFlag atomic.Bool
    var inflightCounter atomic.Int32
    
    // Helper: Check completion atomically
    checkCompletion := func() bool {
        if e.frontier.Len() == 0 && inflightCounter.Load() == 0 {
            // Atomically set completion flag
            return completionFlag.CompareAndSwap(false, true)
        }
        return false
    }
    
    // Workers check completion after each dequeue failure
    go func() {
        defer wg.Done()
        for {
            item, err := e.frontier.Dequeue(workerCtx)
            if err != nil {
                // Dequeue failed (context canceled or frontier empty)
                if checkCompletion() {
                    // This worker detected completion
                    cancel()
                }
                return
            }
            
            inflightCounter.Add(1)
            // ... execute node
            inflightCounter.Add(-1)
            
            // Check completion after node finishes
            if checkCompletion() {
                cancel()
                return
            }
        }
    }()
}
```

**Race-Free Guarantee**:
- CompareAndSwap ensures exactly one worker triggers completion
- Atomic operations on inflightCounter prevent torn reads
- Completion checked at two points: dequeue failure and after node execution
- No polling loop needed

**Performance Impact**:
- CPU: Eliminates 10ms ticker overhead
- Latency: Immediate completion detection (no 10ms delay)
- Throughput: Slight improvement (no polling goroutine)
- Memory: Neutral (atomic.Bool is same size as bool)

---

## Summary of Research Findings

### Key Decisions

1. **Results Channel**: 2x MaxConcurrentNodes buffer prevents deadlock with minimal memory cost
2. **RNG**: Per-worker instances with derived seeds maintains thread safety and determinism
3. **Frontier**: Heap-only data storage, channel for notifications prevents synchronization bugs
4. **Completion**: Atomic flags with explicit checks eliminates polling races

### Performance Expectations

| Metric | Baseline | After Fixes | Change |
|--------|----------|-------------|---------|
| Throughput | 100% | 97-100% | -0 to -3% |
| Memory per workflow | 100% | 100-105% | +0 to +5% |
| Race conditions | N/A | 0 | Fixed |
| Completion latency | 0-10ms | 0-1ms | Improved |

### Risk Assessment

**Low Risk Fixes**:
- Results channel buffer increase (trivial change)
- Per-worker RNG (well-established pattern)

**Medium Risk Fixes**:
- Frontier synchronization (requires careful testing)
- Completion detection (interaction with many goroutines)

**Mitigation**: Comprehensive test suite with race detector, stress tests, and determinism validation.

---

**Research Complete**: All technical unknowns resolved. Ready for Phase 1 design artifacts.
