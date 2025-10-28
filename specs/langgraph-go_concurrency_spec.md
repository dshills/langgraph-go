# Concurrency & Determinism Spec (langgraph-go)

## 0) Goals (non-negotiable)
- **Deterministic replay** for any successful run: same inputs → same *state deltas*, *routing*, and *merge outcomes* without re-hitting external services.
- **Bounded, observable concurrency** with clear backpressure and cancellation semantics.
- **Exactly-once advancement** of the run frontier and checkpoints (no duplicate step commits).
- **Simple, testable contracts** for node authors.

---

## 1) Terminology
- **Run**: One execution instance of a graph, identified by `run_id`.
- **Step**: A scheduler “tick” that consumes a **frontier** of runnable nodes and produces a new frontier.
- **Frontier**: Set of `(node_id, state_snapshot, meta)` work items ready to execute.
- **Branch**: Subpath created by fan-out routing.
- **Reducer**: Pure function `func(prev, delta S) (S, error)` applied to state.
- **Checkpoint**: Durable, transactional record of `{run_id, step_id, state S, frontier, rng_seed, recorded_io}`.

---

## 2) Execution Model

### 2.1 Unit of work
- A **work item** is `(step_id, order_key, node_id, state_ref, inputs, budget)`.
- `order_key` is a deterministic tuple: `(path_hash, edge_index)`.

**Guarantee:** Within the same `step_id`, items are started in ascending `(order_key)`.

### 2.2 Scheduling
- The scheduler processes items in **waves**:
  1. Dequeue up to `MaxConcurrentNodes` items from the frontier queue (deterministic ordering).
  2. Dispatch each to a worker with a **per-run `context.Context`**.
  3. Nodes emit **state deltas** and **routing commands** (`Goto`, `Stop`, `Fork([...])`).
  4. The engine merges deltas and computes the next frontier.

- **Fan-out**: `Fork` yields multiple successor items, each with a **copy-on-write** state snapshot and a unique `order_key`.

- **Fan-in**: Merges deltas deterministically (ascending `(order_key)` of producers).

### 2.3 Parallelism limits
```go
type EngineOptions struct {
    MaxConcurrentNodes    int
    MaxPerNodeConcurrency int
    QueueDepth            int
    DefaultNodeTimeout    time.Duration
    RunWallClockBudget    time.Duration
}
```

### 2.4 Backpressure
- When `frontier` queue is full, **admission blocks** until capacity frees or `ctx` cancels.
- If blocked longer than `BackpressureTimeout`, checkpoint and pause.

---

## 3) Cancellation & Timeouts

### 3.1 Propagation
Each node’s `ctx` includes:
- `deadline = min(run_deadline, node_deadline)`
- `values`: `{ run_id, step_id, node_id, order_key, attempt }`

### 3.2 Policies
```go
type NodePolicy struct {
    Timeout      time.Duration
    Retry        RetryPolicy
    Idempotency  IdempotencyKeyFunc
}
type RetryPolicy struct {
    MaxAttempts int
    Backoff     func(n int) time.Duration
    Retryable   func(error) bool
}
```

### 3.3 Cancellation sources
- **User cancel**
- **Budget exceeded**
- **Topology deadlock** → `ErrNoProgress`

---

## 4) State Merging (Fan-in)

### 4.1 Delta model
```go
type Reducer[S any] interface {
    Apply(prev S, delta S) (S, error)
}
```

### 4.2 Merge order
Deterministic ascending `(order_key)` order.

### 4.3 ConflictPolicy
```go
type ConflictPolicy int
const (
    ConflictFail ConflictPolicy = iota
    ConflictLastWriterWins
    ConflictCRDT
)
```

### 4.4 Determinism guard
Reducers must be **pure** and **deterministic**.

---

## 5) Deterministic Replay

### 5.1 What’s recorded
Checkpoint includes:
- run_id, step_id, rng_seed
- Frontier, State snapshot
- Recorded I/O, Event hashes

### 5.2 RNG
Seeded PRNG per run, stable across replays.

### 5.3 Replay rules
Strict replay reuses recorded I/O and RNG seed; mismatch triggers `ErrReplayMismatch`.

---

## 6) External I/O & Side Effects
```go
type SideEffectPolicy struct {
    Recordable bool
    RequiresIdempotency bool
}
type Node interface {
    Policy() NodePolicy
    Effects() SideEffectPolicy
    Run(ctx context.Context, s S) (delta S, route Route, err error)
}
```

Recorded data: request/response, hashes, metadata.

---

## 7) Checkpointing & Exactly-Once

### 7.1 Atomic step commit
Store must commit state, frontier, and outbox atomically.

### 7.2 Idempotency key
Hash of work items and reduced state ensures no double-apply.

### 7.3 Outbox for emitters
Transactional event log drained asynchronously.

---

## 8) Invariants
1. No progress → fail with `ErrNoProgress`
2. Bounded concurrency
3. Stable order
4. Pure reducer
5. Deterministic conflicts
6. Bounded queues
7. Timely cancellation

---

## 9) Observability
- OpenTelemetry spans (run, step, node)
- Prometheus metrics (`queue_depth`, `step_latency_ms`, etc.)
- Structured JSON logs

---

## 10) API Skeletons
(see conversation for full detail)

---

## 11) Streaming Guarantees
- Token stream order preserved per node.
- Cross-node no interleaving guarantee.

---

## 12) Testing Matrix
Covers deterministic replay, merge order, backpressure, retries, cancellation, conflicts, exactly-once, randomness, deadlocks.

---

## 13) Node Authoring Rules
- Honor `ctx.Done()`.
- Keep `Run` pure except declared side effects.
- Use provided RNG.
- Return deltas, not full state.

---

## 14) Defaults
```go
MaxConcurrentNodes    = 8
QueueDepth            = 1024
DefaultNodeTimeout    = 30 * time.Second
RunWallClockBudget    = 10 * time.Minute
ConflictPolicy        = ConflictFail
```

---

## 15) Open Questions
- CRDT built-ins?
- Transactional outbox ownership?
- Async human-in-loop pause/resume API?
