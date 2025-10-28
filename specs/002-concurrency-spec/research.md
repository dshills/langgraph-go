# Research: Concurrent Graph Execution with Deterministic Replay

**Feature**: Concurrent Graph Execution with Deterministic Replay
**Date**: 2025-10-28
**Phase**: Phase 0 - Research

## Research Questions

This document addresses all unknowns and clarifications needed from the Technical Context and specification.

---

## 1. Deterministic Ordering Mechanism

**Question**: How do we guarantee deterministic ordering of concurrent node execution across replays?

**Decision**: Use `(step_id, order_key)` tuple where `order_key = hash(path_hash, edge_index)`

**Rationale**:
- Path hash captures the execution path that led to this work item (parent node ID + routing decision)
- Edge index provides stable ordering when multiple edges originate from same node
- Hash function (SHA-256) ensures collision-resistant deterministic ordering
- Sort work items by order_key before execution to ensure consistent scheduling
- This approach enables deterministic replay without recording exact goroutine scheduling

**Alternatives Considered**:
- **Timestamp-based ordering**: Rejected because wall-clock times are non-deterministic across replays
- **Global sequence number**: Rejected because requires centralized counter with locking overhead
- **Lexicographic node ID sorting**: Rejected because doesn't capture execution path context

**Implementation Notes**:
- Order key computed at edge traversal time
- Work queue maintains sorted order (heap or sorted slice)
- Replay uses same order key computation to achieve identical scheduling

---

## 2. Work Queue Implementation

**Question**: What data structure should we use for the frontier work queue to support deterministic ordering, bounded capacity, and efficient insertion/removal?

**Decision**: Use buffered channel with priority queue wrapper

**Rationale**:
- Buffered channel provides bounded capacity with built-in blocking (backpressure)
- Priority queue (heap) wrapper sorts work items by order_key before channel insertion
- Go's `container/heap` provides efficient O(log n) insertion/removal
- Channel blocking semantics integrate naturally with context cancellation
- Simple implementation without external dependencies

**Alternatives Considered**:
- **Pure channel**: Rejected because doesn't provide ordering guarantees
- **Sync.Map + goroutine pool**: Rejected due to complexity and lack of deterministic dequeuing
- **Third-party queue library**: Rejected to maintain dependency minimalism (Principle V)

**Implementation Notes**:
```go
type workItem[S any] struct {
    stepID    int
    orderKey  uint64  // Sortable hash
    nodeID    string
    state     S
    attempt   int
}

type frontier[S any] struct {
    heap   workHeap[S]      // Priority queue
    queue  chan workItem[S] // Bounded channel
    ctx    context.Context
}
```

---

## 3. State Copy-on-Write for Fan-Out

**Question**: How do we efficiently create isolated state copies for fan-out branches without deep copy overhead?

**Decision**: Use explicit deep copy with optional copy-on-write optimization for large states

**Rationale**:
- Go doesn't have built-in copy-on-write for structs
- Deep copy via serialization (JSON marshal/unmarshal) is simple and works for any serializable state
- For performance-critical cases, users can implement custom copy logic via optional `StateCopier` interface
- Explicit copying makes memory ownership clear and avoids shared-state bugs

**Alternatives Considered**:
- **Structural sharing with immutable data structures**: Rejected due to complexity and non-idiomatic Go
- **Pointer-based state with manual tracking**: Rejected due to high error potential and complexity
- **Single shared state with locks**: Rejected because violates determinism (lock contention affects replay)

**Implementation Notes**:
```go
// Default: JSON-based deep copy
func deepCopy[S any](state S) (S, error) {
    data, err := json.Marshal(state)
    if err != nil {
        return state, err
    }
    var copy S
    err = json.Unmarshal(data, &copy)
    return copy, err
}

// Optional optimization interface
type StateCopier[S any] interface {
    Copy(S) (S, error)
}
```

---

## 4. Idempotency Key Generation

**Question**: How do we generate idempotency keys to prevent duplicate step commits during exactly-once execution?

**Decision**: Hash of `(run_id, step_id, sorted_work_items, reduced_state_hash)`

**Rationale**:
- Run ID + Step ID uniquely identifies the scheduler tick
- Sorted work items capture the frontier inputs
- Reduced state hash captures the deterministic output
- Combined hash ensures that identical inputs produce identical keys
- Collision-resistant hashing (SHA-256) prevents accidental duplicates

**Alternatives Considered**:
- **UUID per step**: Rejected because not deterministic across replays
- **Simple step counter**: Rejected because doesn't capture state or frontier changes
- **State-only hash**: Rejected because doesn't prevent retry loops with same state

**Implementation Notes**:
```go
func computeIdempotencyKey[S any](runID string, stepID int, items []workItem[S], state S) string {
    h := sha256.New()
    h.Write([]byte(runID))
    binary.Write(h, binary.BigEndian, int64(stepID))
    for _, item := range items {
        h.Write([]byte(item.nodeID))
        binary.Write(h, binary.BigEndian, item.orderKey)
    }
    stateJSON, _ := json.Marshal(state)
    h.Write(stateJSON)
    return hex.EncodeToString(h.Sum(nil))
}
```

---

## 5. I/O Recording for Deterministic Replay

**Question**: What format should we use to record external I/O for replay, and where should recordings be stored?

**Decision**: Store as JSON in checkpoint alongside state, indexed by `(node_id, attempt)`

**Rationale**:
- JSON is human-readable for debugging and inspection
- Storing in checkpoint ensures atomic commit with state (no orphaned recordings)
- Indexing by (node_id, attempt) allows retrieval during replay
- Compact enough for most LLM/tool interactions (typically <100KB per call)
- Supports versioning if recording format changes

**Alternatives Considered**:
- **Separate blob storage**: Rejected because requires managing external storage lifecycle
- **Binary format (protobuf/msgpack)**: Rejected because loses human-readability for debugging
- **In-memory only**: Rejected because doesn't survive process restarts

**Implementation Notes**:
```go
type RecordedIO struct {
    NodeID    string          `json:"node_id"`
    Attempt   int             `json:"attempt"`
    Request   json.RawMessage `json:"request"`
    Response  json.RawMessage `json:"response"`
    Hash      string          `json:"hash"`      // For mismatch detection
    Timestamp time.Time       `json:"timestamp"` // For debugging
}

type Checkpoint struct {
    RunID       string                  `json:"run_id"`
    StepID      int                     `json:"step_id"`
    State       json.RawMessage         `json:"state"`
    Frontier    []workItem              `json:"frontier"`
    RNGSeed     int64                   `json:"rng_seed"`
    RecordedIOs []RecordedIO            `json:"recorded_ios"`
    IdempotencyKey string               `json:"idempotency_key"`
}
```

---

## 6. Backpressure Timeout Strategy

**Question**: What is a reasonable default for `BackpressureTimeout` and how should it be configured?

**Decision**: Default 30 seconds, configurable via `Options.BackpressureTimeout`

**Rationale**:
- 30 seconds is long enough to avoid false positives during temporary load spikes
- Short enough to prevent indefinite blocking when a node is genuinely stuck
- Configurable for different workload characteristics (fast nodes vs slow LLM calls)
- When timeout is reached, system saves checkpoint and returns error to allow investigation

**Alternatives Considered**:
- **No timeout (block indefinitely)**: Rejected because can cause deadlocks
- **Very short timeout (<5s)**: Rejected because too sensitive to normal load variations
- **Adaptive timeout**: Rejected as premature optimization (YAGNI)

**Implementation Notes**:
```go
type Options struct {
    MaxConcurrentNodes   int           // Default: 8
    QueueDepth           int           // Default: 1024
    BackpressureTimeout  time.Duration // Default: 30s
    DefaultNodeTimeout   time.Duration // Default: 30s
    RunWallClockBudget   time.Duration // Default: 10m
}
```

---

## 7. Retry Backoff Algorithm

**Question**: What backoff strategy should we use for node retries?

**Decision**: Exponential backoff with jitter: `delay = base * 2^attempt + jitter(0, base)`

**Rationale**:
- Exponential backoff reduces load on failing services (best practice for rate limits)
- Jitter prevents thundering herd when multiple nodes retry simultaneously
- Configurable base delay allows tuning for different failure modes
- Matches industry-standard retry patterns (AWS SDK, gRPC)

**Alternatives Considered**:
- **Fixed delay**: Rejected because doesn't adapt to persistent failures
- **Linear backoff**: Rejected because too slow for transient failures
- **Exponential without jitter**: Rejected because can cause synchronized retries

**Implementation Notes**:
```go
type RetryPolicy struct {
    MaxAttempts int
    BaseDelay   time.Duration // Default: 1s
    MaxDelay    time.Duration // Default: 30s
    Retryable   func(error) bool
}

func computeBackoff(attempt int, base, max time.Duration) time.Duration {
    delay := base * (1 << attempt) // 2^attempt
    if delay > max {
        delay = max
    }
    jitter := time.Duration(rand.Int63n(int64(base)))
    return delay + jitter
}
```

---

## 8. Cancellation Propagation Mechanism

**Question**: How do we ensure cancellation reaches all running nodes within the 1-second requirement?

**Decision**: Use context.Context with derived contexts per node, check ctx.Done() in tight loops

**Rationale**:
- Go's context package provides built-in cancellation propagation
- Derived contexts per node allow per-node timeouts while respecting parent cancellation
- Checking ctx.Done() in scheduler loop ensures fast detection
- 1-second budget allows for graceful node shutdown

**Alternatives Considered**:
- **Signal-based cancellation**: Rejected because OS-specific and doesn't integrate with Go context
- **Polling with longer intervals**: Rejected because violates 1-second requirement
- **Forced goroutine termination**: Not possible in Go (runtime.Goexit only works within goroutine)

**Implementation Notes**:
```go
// Engine creates derived contexts
nodeCtx, cancel := context.WithTimeout(ctx, nodeTimeout)
defer cancel()

// Nodes must check ctx.Done() regularly
select {
case <-ctx.Done():
    return NodeResult{Err: ctx.Err()}
case result := <-workCh:
    // Process result
}
```

---

## 9. Transactional Outbox Pattern

**Question**: Should the transactional outbox for event emission be owned by the Engine or delegated to Store implementations?

**Decision**: Delegate to Store implementations with optional Engine-level default

**Rationale**:
- Different stores have different transaction semantics (MySQL transactions vs in-memory atomicity)
- Store knows how to batch writes efficiently (single DB transaction for state + events)
- Engine provides simple slice-based outbox for MemStore (in-memory testing)
- Production stores (MySQL) implement outbox table with async polling/draining

**Alternatives Considered**:
- **Engine-only outbox**: Rejected because doesn't leverage store-specific transaction capabilities
- **No outbox (immediate emission)**: Rejected because violates exactly-once guarantee for events
- **External message queue**: Rejected due to added operational complexity

**Implementation Notes**:
```go
// Store interface extension
type Store[S any] interface {
    SaveStep(ctx context.Context, runID string, step int, nodeID string, state S, events []Event) error
    LoadLatest(ctx context.Context, runID string) (state S, step int, nodeID string, _ error)
    // Outbox methods (optional, implemented by production stores)
    PendingEvents(ctx context.Context, limit int) ([]Event, error)
    MarkEventsEmitted(ctx context.Context, eventIDs []string) error
}
```

---

## 10. Seeded RNG Implementation

**Question**: What RNG should we use to ensure deterministic replay, and how should seeds be managed?

**Decision**: Use `math/rand` with per-run seed derived from `hash(run_id)`

**Rationale**:
- `math/rand` is deterministic when seeded (unlike `crypto/rand`)
- Per-run seed ensures different runs don't repeat random sequences
- Hash of run_id provides stable seed across replays of same run
- Simple API: pass `*rand.Rand` to nodes via context values

**Alternatives Considered**:
- **crypto/rand**: Rejected because non-deterministic (uses OS entropy)
- **Global rand.Rand**: Rejected because concurrent access requires locking
- **Per-node seed**: Rejected because complex to manage and may cause seed exhaustion

**Implementation Notes**:
```go
// Create per-run RNG
seed := hashRunIDToSeed(runID)
rng := rand.New(rand.NewSource(seed))

// Store in context
ctx = context.WithValue(ctx, rngKey, rng)

// Nodes retrieve RNG
rng := ctx.Value(rngKey).(*rand.Rand)
randomValue := rng.Intn(100)
```

---

## 11. Conflict Resolution Policies

**Question**: How should we implement the three conflict policies (Fail, LastWriterWins, CRDT)?

**Decision**:
- **ConflictFail**: Default - return error when reducer detects conflicting deltas
- **ConflictLastWriterWins**: Reducer overwrites prev with delta fields
- **ConflictCRDT**: Not implemented initially (future enhancement)

**Rationale**:
- Fail-fast is safest default (prevents silent data loss)
- LastWriterWins is simple to implement and sufficient for many use cases
- CRDT requires complex types (G-Counter, PN-Counter, LWW-Map) - defer until demand is proven
- Users can implement custom CRDT logic in their reducer if needed

**Alternatives Considered**:
- **Always LastWriterWins**: Rejected because silently hides merge conflicts
- **Require CRDT for all concurrent operations**: Rejected due to complexity
- **Automatic conflict detection in reducer**: Rejected because requires reflection and tight coupling

**Implementation Notes**:
```go
type ConflictPolicy int
const (
    ConflictFail ConflictPolicy = iota  // Default
    ConflictLastWriterWins
    ConflictCRDT // Not implemented in Phase 1
)

// Example reducer with conflict detection
func reducerWithPolicy(prev, delta MyState, policy ConflictPolicy) (MyState, error) {
    if policy == ConflictFail && prev.Version != delta.Version {
        return prev, fmt.Errorf("conflict detected: version mismatch")
    }
    // Merge logic...
    return merged, nil
}
```

---

## 12. Performance Benchmarking Strategy

**Question**: How do we validate that concurrent execution meets performance goals (1000+ concurrent nodes, <10ms overhead)?

**Decision**: Use Go benchmark tests with varying concurrency levels and synthetic workloads

**Rationale**:
- `go test -bench` provides reliable, repeatable measurements
- Synthetic nodes (sleep-based) isolate scheduler overhead from external I/O latency
- Benchmark across concurrency levels (1, 10, 100, 1000 nodes) to detect scaling issues
- Profile with `go tool pprof` to identify bottlenecks

**Alternatives Considered**:
- **Production load testing only**: Rejected because hard to isolate variables
- **Third-party benchmarking frameworks**: Rejected due to dependency minimalism
- **Manual timing with time.Now()**: Rejected because less accurate than benchmark framework

**Implementation Notes**:
```go
func BenchmarkConcurrentExecution(b *testing.B) {
    for _, concurrency := range []int{1, 10, 100, 1000} {
        b.Run(fmt.Sprintf("nodes=%d", concurrency), func(b *testing.B) {
            engine := setupEngineWithNNodes(concurrency)
            b.ResetTimer()
            for i := 0; i < b.N; i++ {
                engine.Run(ctx, fmt.Sprintf("run-%d", i), initialState)
            }
        })
    }
}
```

---

## Summary of Decisions

All research questions have been resolved with concrete technical decisions. Key takeaways:

1. **Determinism**: Achieved through order keys, seeded RNG, recorded I/O, and hash-based idempotency
2. **Performance**: Buffered channels + heap for work queue, JSON-based deep copy with optional optimization
3. **Reliability**: Exponential backoff with jitter, context-based cancellation, transactional outbox
4. **Simplicity**: Leveraging Go stdlib (context, heap, sha256), minimal new dependencies

Ready to proceed to Phase 1 (Design & Contracts).
