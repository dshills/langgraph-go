# Data Model: Concurrent Graph Execution with Deterministic Replay

**Feature**: Concurrent Graph Execution with Deterministic Replay
**Date**: 2025-10-28
**Phase**: Phase 1 - Design

## Overview

This document defines the core data structures and their relationships for implementing concurrent graph execution with deterministic replay guarantees. All types are designed to support serialization, deterministic ordering, and exactly-once execution semantics.

---

## Core Entities

### WorkItem

Represents a single schedulable unit of work in the execution frontier.

**Purpose**: Enables deterministic ordering of concurrent node execution and provides all context needed to execute a node.

**Fields**:
- `StepID` (int): Monotonically increasing step number in the run
- `OrderKey` (uint64): Deterministic sort key computed from hash(path_hash, edge_index)
- `NodeID` (string): Identifier of the node to execute
- `State` (generic S): Snapshot of state for this work item
- `Attempt` (int): Retry counter (0 for first execution, 1+ for retries)
- `ParentNodeID` (string): Node that spawned this work item (for path hash computation)
- `EdgeIndex` (int): Index of edge taken from parent (for deterministic ordering)

**Relationships**:
- Contained in Frontier queue
- References Node (by NodeID)
- Contains State snapshot

**Validation Rules**:
- StepID must be ≥ 0
- OrderKey must be deterministically computable
- NodeID must exist in graph
- Attempt must be ≥ 0

**State Transitions**:
- Created → Queued (added to frontier)
- Queued → Executing (dequeued by scheduler)
- Executing → Completed (node finishes successfully)
- Executing → Failed (node returns error, may retry)
- Failed → Queued (retry with incremented Attempt)

---

### Frontier

Work queue containing all nodes ready to execute in the current step.

**Purpose**: Manages bounded, deterministically-ordered queue of pending work with backpressure control.

**Fields**:
- `Heap` (priority queue): Min-heap ordered by WorkItem.OrderKey
- `Queue` (buffered channel): Bounded channel for blocking/backpressure
- `Capacity` (int): Maximum queue size (from Options.QueueDepth)
- `Context` (context.Context): For cancellation propagation

**Relationships**:
- Contains multiple WorkItems
- Owned by Scheduler
- Persisted in Checkpoint

**Validation Rules**:
- Queue size ≤ Capacity at all times
- Items dequeued in ascending OrderKey order
- Blocking admission when at capacity

**Operations**:
- `Enqueue(item WorkItem) error`: Add item (blocks if full, respects context)
- `Dequeue() (WorkItem, error)`: Remove min OrderKey item (blocks if empty)
- `Len() int`: Current queue size

---

### Checkpoint

Durable snapshot of execution state enabling resumption and replay.

**Purpose**: Provides atomic, transactional persistence of run state, frontier, and recorded I/O for exactly-once execution and deterministic replay.

**Fields**:
- `RunID` (string): Unique identifier for this execution
- `StepID` (int): Step number at checkpoint time
- `State` (generic S, JSON): Current accumulated state
- `Frontier` ([]WorkItem, JSON): Work items ready to execute
- `RNGSeed` (int64): Seed for deterministic random number generation
- `RecordedIOs` ([]RecordedIO): Captured external interactions
- `IdempotencyKey` (string): Hash preventing duplicate commits
- `Timestamp` (time.Time): When checkpoint was created
- `Label` (string): Optional user-defined checkpoint name

**Relationships**:
- Contains Frontier snapshot
- Contains multiple RecordedIOs
- References Run (by RunID)
- Stored via Store interface

**Validation Rules**:
- RunID must not be empty
- StepID must be ≥ 0
- IdempotencyKey must be unique per (RunID, StepID)
- State must be JSON-serializable
- RNGSeed must be deterministic for given RunID

**State Transitions**:
- Computed → Validated (idempotency key checked)
- Validated → Committed (atomic store write)
- Committed → Loadable (available for resumption)

---

### RecordedIO

Captured external interaction for deterministic replay.

**Purpose**: Enables replay of graph executions without re-invoking external services by recording request/response pairs.

**Fields**:
- `NodeID` (string): Node that performed the I/O
- `Attempt` (int): Retry attempt number
- `Request` (JSON): Serialized request data
- `Response` (JSON): Serialized response data
- `Hash` (string): SHA-256 hash of response for mismatch detection
- `Timestamp` (time.Time): When I/O was recorded
- `Duration` (time.Duration): How long the I/O took

**Relationships**:
- Contained in Checkpoint
- References Node (by NodeID)
- Indexed by (NodeID, Attempt) for retrieval during replay

**Validation Rules**:
- NodeID must exist in graph
- Attempt must match WorkItem.Attempt
- Hash must match actual response content during replay
- Request and Response must be valid JSON

**Matching Logic**:
- During replay, lookup by (NodeID, Attempt)
- Compare Hash of live response vs recorded Hash
- If mismatch → raise ErrReplayMismatch

---

### NodePolicy

Configuration for node execution behavior.

**Purpose**: Encapsulates timeout, retry, and idempotency policies for individual nodes.

**Fields**:
- `Timeout` (time.Duration): Maximum execution time for this node
- `RetryPolicy` (RetryPolicy): Retry configuration (can be nil)
- `IdempotencyKeyFunc` (func(S) string): Optional custom idempotency key computation

**Relationships**:
- Attached to Node
- Used by Scheduler to enforce policies

**Validation Rules**:
- Timeout must be > 0 if set
- RetryPolicy.MaxAttempts must be ≥ 1

**Defaults**:
- Timeout: Options.DefaultNodeTimeout (30s)
- RetryPolicy: nil (no retries)
- IdempotencyKeyFunc: nil (use default)

---

### RetryPolicy

Retry configuration for transient failures.

**Purpose**: Enables automatic recovery from transient errors without node-level retry logic.

**Fields**:
- `MaxAttempts` (int): Maximum number of execution attempts (including initial)
- `BaseDelay` (time.Duration): Base delay for exponential backoff
- `MaxDelay` (time.Duration): Maximum delay cap
- `Retryable` (func(error) bool): Predicate determining if error is retryable

**Relationships**:
- Contained in NodePolicy
- Applied by Scheduler when node execution fails

**Validation Rules**:
- MaxAttempts must be ≥ 1
- BaseDelay must be > 0
- MaxDelay must be ≥ BaseDelay
- Retryable function must not be nil

**Backoff Formula**:
```
delay = min(BaseDelay * 2^attempt + jitter(0, BaseDelay), MaxDelay)
```

---

### SideEffectPolicy

Declaration of node's external I/O characteristics.

**Purpose**: Informs replay engine whether node I/O should be recorded and replayed.

**Fields**:
- `Recordable` (bool): Whether I/O can be captured for replay
- `RequiresIdempotency` (bool): Whether node requires idempotency key

**Relationships**:
- Attached to Node
- Used by replay engine to decide recording behavior

**Validation Rules**:
- If RequiresIdempotency=true, node must provide IdempotencyKeyFunc

**Examples**:
- LLM call: `{Recordable: true, RequiresIdempotency: true}`
- Pure function: `{Recordable: false, RequiresIdempotency: false}`
- Database write: `{Recordable: false, RequiresIdempotency: true}`

---

### SchedulerState

Internal state of the concurrent scheduler.

**Purpose**: Tracks active goroutines, pending work, and execution metrics for observability and debugging.

**Fields**:
- `ActiveNodes` (int): Number of nodes currently executing
- `QueueDepth` (int): Current frontier queue size
- `CompletedSteps` (int): Total steps completed in this run
- `CancelFunc` (context.CancelFunc): Function to cancel all running nodes
- `Metrics` (SchedulerMetrics): Performance counters

**Relationships**:
- Owned by Engine
- Contains Frontier
- Emits Events via Emitter

**Validation Rules**:
- ActiveNodes must be ≤ Options.MaxConcurrentNodes
- QueueDepth must be ≤ Options.QueueDepth

---

### SchedulerMetrics

Performance and observability metrics for concurrent execution.

**Purpose**: Provides quantitative data for monitoring, alerting, and performance tuning.

**Fields**:
- `TotalSteps` (int): Total steps executed
- `TotalNodesExecuted` (int): Total node executions
- `AvgStepLatencyMs` (float64): Average time per scheduler step
- `MaxQueueDepth` (int): Peak frontier queue size
- `TotalRetries` (int): Total retry attempts
- `TotalBackpressureBlocks` (int): Times admission was blocked

**Relationships**:
- Contained in SchedulerState
- Exported via Emitter as events
- Aggregated across run lifetime

**Update Triggers**:
- After each step completion
- On backpressure block
- On retry attempt
- On run completion

---

## Entity Relationships Diagram

```
Run 1---* Checkpoint
    |     └─ contains ─→ Frontier ([]WorkItem)
    |                    └─ references ─→ Node
    |     └─ contains ─→ RecordedIO
    |                    └─ references ─→ Node
    |
    1─── has ─→ SchedulerState
                 └─ contains ─→ Frontier
                                └─ contains ─→ WorkItem
                 └─ contains ─→ SchedulerMetrics

Node 1--- has ─→ NodePolicy
     |           └─ contains ─→ RetryPolicy
     |
     1--- has ─→ SideEffectPolicy

Engine 1--- owns ─→ Scheduler
       |           └─ manages ─→ Frontier
       |           └─ maintains ─→ SchedulerState
       |
       *─── contains ─→ Node
       |
       1─── uses ─→ Store
       |            └─ persists ─→ Checkpoint
       |
       1─── uses ─→ Emitter
                    └─ emits ─→ Event
```

---

## State Transition Diagrams

### WorkItem Lifecycle

```
[Created]
    ↓
[Queued in Frontier]
    ↓
[Dequeued by Scheduler]
    ↓
[Executing in Goroutine] ─→ [Success] → [State Delta Merged]
    ↓                                          ↓
[Failed with Error]                     [Next WorkItems Created]
    ↓                                          ↓
[Check RetryPolicy]                     [Checkpoint Committed]
    ↓                   ↓
[Non-Retryable]    [Retryable + Attempts < Max]
    ↓                   ↓
[Error Propagated]  [Re-queued with Attempt++]
```

### Checkpoint Lifecycle

```
[Execution Step Completes]
    ↓
[Compute Idempotency Key]
    ↓
[Check for Duplicate] ─→ [Duplicate Detected] → [Skip Commit]
    ↓
[No Duplicate]
    ↓
[Serialize State + Frontier + RecordedIOs]
    ↓
[Atomic Store Write] ─→ [Commit Failure] → [Rollback + Error]
    ↓
[Commit Success]
    ↓
[Checkpoint Available for Resumption]
```

---

## Serialization Formats

All entities use JSON for serialization to support:
- Human readability for debugging
- Cross-language compatibility
- Schema evolution with version fields

### WorkItem JSON Example
```json
{
  "step_id": 42,
  "order_key": 18446744073709551615,
  "node_id": "process_query",
  "state": {"query": "hello", "counter": 5},
  "attempt": 0,
  "parent_node_id": "input_validator",
  "edge_index": 0
}
```

### Checkpoint JSON Example
```json
{
  "run_id": "run-2025-001",
  "step_id": 10,
  "state": {"query": "result", "counter": 10},
  "frontier": [
    {"step_id": 11, "order_key": 12345, "node_id": "summarize", "state": {...}, "attempt": 0}
  ],
  "rng_seed": 42,
  "recorded_ios": [
    {
      "node_id": "call_llm",
      "attempt": 0,
      "request": {"prompt": "hello"},
      "response": {"text": "Hi there!"},
      "hash": "sha256:abc123...",
      "timestamp": "2025-10-28T10:30:00Z",
      "duration_ms": 250
    }
  ],
  "idempotency_key": "sha256:def456...",
  "timestamp": "2025-10-28T10:30:01Z",
  "label": "before_summary"
}
```

---

## Indexing Strategy

### Store Indexes

For efficient checkpoint retrieval and replay:

1. **Primary Index**: `(run_id, step_id)` - lookup latest checkpoint
2. **Label Index**: `(run_id, label)` - named checkpoint retrieval
3. **Idempotency Index**: `idempotency_key` - duplicate detection

### In-Memory Indexes

Scheduler maintains:

1. **Active Nodes**: `map[string]context.CancelFunc` - for cancellation
2. **Recorded I/O Lookup**: `map[(node_id, attempt)]RecordedIO` - for replay

---

## Validation Rules Summary

### WorkItem
- `StepID ≥ 0`
- `OrderKey` deterministically computed
- `NodeID` exists in graph
- `Attempt ≥ 0`

### Checkpoint
- `RunID` not empty
- `StepID ≥ 0`
- `IdempotencyKey` unique per (RunID, StepID)
- `State` JSON-serializable
- `RNGSeed` deterministic

### RetryPolicy
- `MaxAttempts ≥ 1`
- `BaseDelay > 0`
- `MaxDelay ≥ BaseDelay`
- `Retryable` not nil

### NodePolicy
- `Timeout > 0` if set

---

## Concurrency Considerations

### Thread Safety

- **Frontier**: Synchronized via channel operations (blocking sends/receives)
- **SchedulerState**: Protected by mutex for metrics updates
- **Store**: Implementations must be thread-safe for concurrent checkpoint writes
- **Emitter**: Must handle concurrent event emission

### Deadlock Prevention

- Always acquire locks in consistent order: State → Frontier → Emitter
- Use timeouts on all blocking operations
- Context cancellation propagates to all goroutines
- No circular waits (frontier queue is single-producer, multi-consumer)

### Memory Bounds

- Frontier queue bounded by `Options.QueueDepth`
- WorkItem state copied for fan-out (isolated memory)
- RecordedIO cleaned up after checkpoint commit
- Active goroutines limited by `Options.MaxConcurrentNodes`

---

## Future Enhancements (Out of Scope for Phase 1)

- CRDT types for automatic conflict resolution
- Distributed frontier (multi-machine coordination)
- Incremental checkpoint compression
- Checkpoint garbage collection policies
- Custom serialization formats (protobuf, msgpack)

---

## Summary

This data model provides:
1. **Deterministic ordering** via OrderKey and sorted frontier
2. **Exactly-once execution** via idempotency keys
3. **Deterministic replay** via recorded I/O and seeded RNG
4. **Bounded concurrency** via capacity-limited frontier queue
5. **Observability** via metrics and structured events

All entities are designed for serialization, testability, and evolution over time.
