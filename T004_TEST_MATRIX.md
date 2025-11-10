# T004: Skipped Tests - Quick Reference Matrix

## Test Inventory

| User Story | Test File | Line | Test Name | Status | Skip Reason | Priority |
|-----------|-----------|------|-----------|--------|------------|----------|
| US1 | replay_test.go | 483 | TestDeterministicRetryDelays | SKIPPED | Semantic mismatch (delta merging) | LOW |
| US2 | policy_test.go | 52 | TestNodeTimeout - enforces timeout | ACTIVE | - | HIGH |
| US2 | policy_test.go | 129 | TestNodeTimeout - DefaultNodeTimeout | SKIPPED | Pending T076 impl | HIGH |
| US2 | policy_test.go | 142 | TestNodeTimeout - independent timeouts | SKIPPED | Pending T076 impl | HIGH |
| US2 | policy_test.go | 156 | TestNodeTimeout - no timeout | SKIPPED | Pending T076 impl | HIGH |
| US3 | scheduler_test.go | 189 | TestBackpressureBlock - enqueue blocks | SKIPPED | Phase 5 (US3) deferral | MEDIUM |
| US3 | scheduler_test.go | 449 | TestBackpressureTimeout - checkpoint | SKIPPED | Pending T064 impl | MEDIUM |
| US3 | scheduler_test.go | 465 | TestBackpressureTimeout - events | SKIPPED | Pending T069 impl | MEDIUM |

---

## US1: Sequential Retries Analysis

### TestDeterministicRetryDelays (Line 483)

**Status**: SKIPPED - Semantic Issue

**What It Validates**:
- Retry delays are identical across 100 executions with same RunID
- RNG produces deterministic jitter values
- State accumulates retry delay history
- Final state hash is byte-identical

**Test State Type**:
```go
type TestState struct {
    RetryCount    int
    RetryDelays   []time.Duration  // Captures each retry delay
    ExecutionHash string
}
```

**Acceptance Criteria**:
| # | Criterion | Status | Notes |
|---|-----------|--------|-------|
| 1 | Node fails N times then succeeds | READY | Retry logic works |
| 2 | Retry delays computed from RNG | PARTIAL | RNG seeding done, backoff formula missing |
| 3 | 100 runs produce identical delays | BLOCKED | Engine doesn't merge failure deltas |
| 4 | Final state hash identical | BLOCKED | Depends on criterion 3 |
| 5 | Delays recorded in state via reducer | BLOCKED | Semantic mismatch |

**The Problem**:
```
Test expects:         Engine implements:
└─ Run attempt 1     └─ Run attempt 1
   └─ FAIL            └─ FAIL
      └─ Merge delta  └─ (no merge, retry)
   └─ Run attempt 2   └─ Run attempt 2
      └─ FAIL         └─ FAIL
         └─ Merge delta  └─ (no merge, retry)
      └─ Run attempt 3  └─ Run attempt 3
         └─ SUCCESS     └─ SUCCESS
            └─ Merge all deltas
                         └─ Merge only success delta
```

**Dependencies**:
- RNG seeding from RunID: ✓ DONE (T054-T055)
- Exponential backoff formula: ✗ NOT DONE (T083)
- Engine delta merging on failure: ✗ NOT DONE (requires semantic change)

**Linked Task**: T010 (Implement Sequential Retry Policy)

**Recommended Action**: Clarify test semantics
- Option A: Redesign test to focus on RNG determinism only
- Option B: Change engine to merge deltas on all attempts
- RECOMMENDATION: Option A (cleaner separation of concerns)

---

## US2: Node Timeout Policies Analysis

### All Four TestNodeTimeout Subtests

**Overall Status**: HIGH PRIORITY - Well-Defined Requirements

**Feature Requirements**:

| Requirement | Test Coverage | Implementation Status |
|-------------|----------------|----------------------|
| Per-node timeout via NodePolicy.Timeout | Line 52, 142 | NOT STARTED |
| Fallback to DefaultNodeTimeout | Line 129 | NOT STARTED |
| No timeout when both are zero | Line 156 | NOT STARTED |
| Context deadline enforcement | All tests | NOT STARTED |
| Independent node timeouts | Line 142 | NOT STARTED |
| Error on deadline exceeded | Line 52 | NOT STARTED |

**Test Breakdown**:

#### Test 1: enforces per-node timeout (Line 52) - ACTIVE

**What It Tests**:
- Node with 100ms timeout that attempts 500ms work
- Must be cancelled at ~100ms, not 500ms
- Returns context.DeadlineExceeded error

**Acceptance Criteria**:
- [ ] Execution time ≈ 100ms (within grace period)
- [ ] Error indicates timeout (DeadlineExceeded or contains "timeout"/"NODE_TIMEOUT")
- [ ] Node can detect cancellation via ctx.Done()

**Setup Details**:
```go
timedNode {
    fn:      nodeFunction
    timeout: 100*time.Millisecond  // Per-node timeout
}
```

**Key Code Pattern**:
```go
select {
case <-time.After(500*ms):
    // Should NOT reach here
case <-ctx.Done():
    // Will be cancelled at ~100ms
    return NodeResult{
        Err: ctx.Err(),  // context.DeadlineExceeded
    }
}
```

**Implementation Hints**:
1. Engine must check if node implements `Policy()` method
2. Extract `NodePolicy.Timeout` if > 0
3. Create context: `context.WithDeadline(parentCtx, now.Add(timeout))`
4. Pass timeout context to `node.Run(ctx, state)`
5. Node error handling returns timeout error

---

#### Test 2: DefaultNodeTimeout fallback (Line 129) - SKIPPED

**What It Tests**:
- When node has no explicit timeout (Policy().Timeout == 0)
- Engine uses Options.DefaultNodeTimeout instead
- Same timeout enforcement as Test 1

**Acceptance Criteria**:
- [ ] Engine checks NodePolicy.Timeout first
- [ ] Falls back to Options.DefaultNodeTimeout if zero
- [ ] Timeout applied same way as explicit timeout
- [ ] No timeout if both are zero

**Setup**:
```go
Engine Options:
  DefaultNodeTimeout: 100*ms

Node:
  Policy().Timeout: 0  // Not set, use default
```

**Implementation Logic**:
```go
timeout := node.Policy().Timeout
if timeout == 0 {
    timeout = options.DefaultNodeTimeout  // Fallback
}
if timeout > 0 {
    ctx = context.WithDeadline(ctx, now.Add(timeout))
}
```

---

#### Test 3: Independent per-node timeouts (Line 142) - SKIPPED

**What It Tests**:
- Multiple nodes with different timeouts
- NodeA has 50ms timeout (times out)
- NodeB has 200ms timeout (completes)
- Timeout of one doesn't affect others

**Acceptance Criteria**:
- [ ] NodeA times out after ~50ms
- [ ] NodeB continues and completes
- [ ] Workflow doesn't cascade failures
- [ ] Error from A doesn't cancel B

**Test Pattern**:
```
Workflow: A(50ms) -> B(200ms)
Expected: A times out, B succeeds
         (not: A times out, B also cancelled)
```

**Implementation Requirement**:
- Create NEW context for EACH node with its own deadline
- Don't reuse contexts between nodes
- Each timeout is isolated

---

#### Test 4: No timeout when both zero (Line 156) - SKIPPED

**What It Tests**:
- When NodePolicy.Timeout == 0 AND Options.DefaultNodeTimeout == 0
- Node can run indefinitely (no timeout)
- Completes normally

**Acceptance Criteria**:
- [ ] Node with no timeout completes normally
- [ ] No context deadline applied
- [ ] Can run for extended duration

**Setup**:
```go
Options.DefaultNodeTimeout: 0
Node.Policy().Timeout:      0
```

**Implementation Logic**:
```go
if timeout == 0 && defaultTimeout == 0 {
    // Skip timeout context creation
    ctx = parentCtx  // Use parent as-is
}
```

---

### US2 Implementation Checklist

**Types to Define**:
- [ ] `NodePolicy` struct with `Timeout time.Duration`
- [ ] `Options.DefaultNodeTimeout` field

**Interface Changes**:
- [ ] Add optional `Policy() graph.NodePolicy` to Node interface

**Scheduler Changes**:
- [ ] Before calling `node.Run()`:
  - Compute effective timeout (per-node or default)
  - Create deadline context if timeout > 0
  - Pass timeout context to Run()

**Error Handling**:
- [ ] Catch `context.DeadlineExceeded`
- [ ] Return as node error

**Edge Cases**:
- [ ] Zero timeout (both sources) = no deadline
- [ ] Partial timeout (one source) = use available
- [ ] Node doesn't implement Policy() = use default

**Tests to Pass**:
- [ ] Line 52: Basic enforcement
- [ ] Line 129: DefaultNodeTimeout fallback
- [ ] Line 142: Independent timeouts
- [ ] Line 156: No timeout with both zero

**Estimated Effort**: 3-5 days

---

## US3: Backpressure Analysis

### Test Overview

| Test | Line | Focus | Dependency | Linked Task |
|------|------|-------|------------|-------------|
| TestBackpressureBlock | 189 | Queue blocking | Frontier bounded queue | T064 |
| TestBackpressureTimeout | 449 | Checkpoint on sustained block | Backpressure detection | T064 |
| TestBackpressureEvents | 465 | Observability emission | Event framework | T069 |

---

### Test 1: TestBackpressureBlock (Line 189) - SKIPPED

**What It Tests**:
- Frontier queue blocks when reaching capacity
- Enqueue blocks until space available (dequeue frees it)
- Basic backpressure mechanism

**Acceptance Criteria**:
- [ ] Enqueue fills queue to capacity
- [ ] Next enqueue blocks (doesn't return immediately)
- [ ] Dequeue unblocks the blocked enqueue
- [ ] Final queue state is consistent

**Test Setup**:
```go
frontier := NewFrontier[S](ctx, capacity=5, ...)

// Fill to capacity
for i := 0; i < 5; i++ {
    frontier.Enqueue(ctx, item)  // Returns immediately
}

// This should block
go func() {
    frontier.Enqueue(ctx, extraItem)  // Blocks here
    ...
}()

// Give goroutine time to block
time.Sleep(50*ms)

// Verify still blocked
select {
case <-enqueueErr:
    fail("should still be blocked")
default:
    ok("correctly blocked")
}

// Now dequeue to free space
frontier.Dequeue(ctx)

// Verify blocked goroutine unblocks
<-enqueueErr  // Should complete now
```

**Implementation Requirements**:
1. Frontier uses bounded channel (not unlimited buffer)
2. Enqueue sends to channel (blocks if full)
3. Dequeue receives from channel (unblocks sender)
4. Context cancellation is respected

**Key Code Pattern**:
```go
type Frontier[S any] struct {
    queue chan WorkItem[S]  // Bounded channel
}

func (f *Frontier[S]) Enqueue(ctx context.Context, item WorkItem[S]) error {
    select {
    case f.queue <- item:
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}

func (f *Frontier[S]) Dequeue(ctx context.Context) (WorkItem[S], error) {
    select {
    case item := <-f.queue:
        return item, nil
    case <-ctx.Done():
        return WorkItem[S]{}, ctx.Err()
    }
}
```

---

### Test 2: TestBackpressureTimeout (Line 449) - SKIPPED

**What It Tests**:
- When enqueue blocks longer than BackpressureTimeout
- Engine detects condition and saves checkpoint
- Pauses execution and returns error
- Can resume from checkpoint

**Acceptance Criteria**:
- [ ] Engine detects backpressure timeout (enqueue blocked > threshold)
- [ ] Checkpoint saved with:
  - Current state
  - Frontier contents (in OrderKey order)
  - Current step/node
- [ ] Returns ErrBackpressureTimeout
- [ ] Execution can resume from checkpoint

**Test Pattern**:
```
Setup:
  QueueDepth: small (triggers backpressure quickly)
  BackpressureTimeout: 500ms

Scenario:
  Nodes produce work faster than it can be consumed
  → Frontier fills up
  → Enqueue blocks
  → Blocks > 500ms
  → Engine detects timeout

Expected:
  1. Checkpoint saved
  2. Error returned: ErrBackpressureTimeout
  3. Caller can resume from checkpoint

Resume Flow:
  1. Load checkpoint
  2. Restore state/frontier
  3. Continue execution
```

**Implementation Needs**:
1. `Options.BackpressureTimeout` field
2. Track time spent blocking on enqueue
3. Checkpoint API (already exists)
4. Pause/resume capability
5. Error type: `ErrBackpressureTimeout`

**Pseudo-code**:
```go
func (e *Engine) enqueueWithBackpressure(item WorkItem) error {
    timeout := e.options.BackpressureTimeout
    
    // If no backpressure timeout configured, don't check
    if timeout <= 0 {
        return e.frontier.Enqueue(ctx, item)
    }
    
    // Track blocking time
    startBlock := time.Now()
    
    select {
    case e.frontier.queue <- item:
        return nil  // Successfully enqueued
    
    case <-time.After(timeout):
        // Backpressure timeout exceeded
        // Save checkpoint before pausing
        state, step, nodeID := e.getCurrentState()
        e.store.SaveCheckpoint(ctx, e.runID, "backpressure", state, step, nodeID)
        return ErrBackpressureTimeout
    
    case <-ctx.Done():
        return ctx.Err()
    }
}
```

---

### Test 3: TestBackpressureEvents (Line 465) - SKIPPED

**What It Tests**:
- Backpressure timeout emits observability event
- Event includes metadata (queue depth, timeout, etc.)

**Acceptance Criteria**:
- [ ] "backpressure_timeout" event emitted
- [ ] Event metadata includes:
  - Queue depth at timeout
  - Timeout duration
  - Current frontier size
  - Available worker count
  - Run ID / Node ID / Step

**Event Structure**:
```go
type BackpressureTimeoutEvent struct {
    Type string        // "backpressure_timeout"
    QueueDepth int
    TimeoutDuration time.Duration
    FrontierSize int
    AvailableWorkers int
    RunID string
    NodeID string
    Step int
    Timestamp time.Time
}
```

**Implementation**:
```go
if backpressureTimeout {
    e.emitter.Emit(BackpressureTimeoutEvent{
        Type: "backpressure_timeout",
        QueueDepth: e.options.QueueDepth,
        TimeoutDuration: e.options.BackpressureTimeout,
        FrontierSize: e.frontier.Len(),
        AvailableWorkers: e.availableWorkers(),
        RunID: e.runID,
        NodeID: currentNodeID,
        Step: e.step,
        Timestamp: time.Now(),
    })
}
```

---

## US3 Implementation Roadmap

### Phase 1: Basic Queue (Prerequisite)
**Status**: Foundation for Phase 2-3

**What**:
- [ ] Frontier with bounded channel
- [ ] Enqueue/Dequeue blocking semantics
- [ ] OrderKey-based priority queue (already done)

**Test Coverage**: TestBackpressureBlock (line 209-432)

**Effort**: 2-3 days

---

### Phase 2: Backpressure Timeout (T064)
**Status**: Core backpressure feature

**What**:
- [ ] Add `BackpressureTimeout` to Options
- [ ] Track enqueue blocking duration
- [ ] Checkpoint on timeout
- [ ] Return `ErrBackpressureTimeout`

**Tests**: TestBackpressureTimeout (line 449-463)

**Dependencies**: Phase 1 (basic queue)

**Effort**: 3-4 days

---

### Phase 3: Observability Events (T069)
**Status**: Monitoring/debugging support

**What**:
- [ ] Emit backpressure timeout events
- [ ] Collect metadata
- [ ] Send to emitter

**Tests**: TestBackpressureEvents (line 465-475)

**Dependencies**: Phase 2 (timeout detection)

**Effort**: 1-2 days

---

## Implementation Priority Summary

### Rank 1: US2 Node Timeouts
- **Why**: Well-defined, high value, unblocks T021
- **Effort**: 3-5 days
- **Start**: Immediately after review
- **Blocker**: None

### Rank 2: US3 Backpressure (Phase 1 + 2)
- **Why**: Production stability, prevents memory exhaustion
- **Effort**: 5-7 days (phases 1-2)
- **Start**: After US2 baseline
- **Blocker**: Depends on US2 completion

### Rank 3: US1 Retry Delays
- **Why**: Semantic clarification needed first
- **Effort**: 1 day clarify + 2-3 days implement
- **Start**: After US1 requirements confirmed
- **Blocker**: Team decision on semantics

---

## Files Generated

1. **T004_SKIPPED_TESTS_ANALYSIS.md** - Full detailed breakdown (474 lines)
2. **T004_FINDINGS_SUMMARY.md** - Executive summary with recommendations
3. **T004_TEST_MATRIX.md** - This quick reference guide

**All files saved to**: `/Users/dshills/Development/projects/langgraph-go/`
