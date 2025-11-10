# T004: Skipped Tests Analysis

This document provides a comprehensive review of skipped tests to understand what features need to be implemented.

## Overview

Three test files contain skipped tests across three user stories:
- **US1 (Sequential Retries)**: `graph/replay_test.go:483` - 1 test
- **US2 (Node Timeouts)**: `graph/policy_test.go:47,103,116,130` - 4 tests  
- **US3 (Backpressure)**: `graph/scheduler_test.go:191,452,467` - 3 tests

---

## US1: Sequential Retries (Deterministic Replay)

### Test File: `graph/replay_test.go`

#### Test 1: TestDeterministicRetryDelays (Line 483)

**Status**: SKIPPED

**Skip Reason**:
```
"Test expects node-level retry tracking with delta merging on failures. 
Engine-level retry only merges on success. Retry logic WORKS (all other tests pass), 
but this test needs redesign for engine-level semantics"
```

**What It Tests**:
- Validates that retry delays are identical across 100 consecutive executions
- Uses same RunID across all runs to ensure deterministic RNG seeding
- Verifies retry delays are captured and merged into state through reducer

**Acceptance Criteria**:
1. Node can fail up to N times before succeeding
2. Each failure attempt captures retry delay via RNG jitter
3. Same RunID produces identical delay sequences across 100 runs
4. Retry deltas merge into accumulated state via reducer function
5. Final state hash is byte-identical across all runs

**Test Setup/Fixtures**:
```go
type TestState struct {
    RetryCount    int               // Number of retry attempts
    RetryDelays   []time.Duration   // Sequence of delays experienced
    ExecutionHash string            // Hash of final execution
}
```

**Implementation Requirements**:
1. **RNG per RunID**: Engine must seed context RNG from RunID (already done in Phase 2)
2. **Retry Delay Tracking**: Engine must compute deterministic backoff delays:
   - Formula: `baseDelay * 2^attempt + jitter(0, baseDelay)`
   - Jitter sourced from context RNG for determinism
3. **Delta Merging**: Reducer function must merge retry deltas even on failures
   - Current implementation only merges on success
   - Need to update engine to merge deltas for each retry attempt
4. **Deterministic Ordering**: Multiple runs with same RunID must produce byte-identical states

**Key Challenge**:
The test expects the engine to merge node deltas on every attempt (including failures), 
but the current engine only merges deltas on success. This requires semantic change to 
retry handling: either the test needs redesign OR the engine needs to track intermediate 
deltas during retries.

**Dependencies/Prerequisites**:
- RNG seeding from RunID (FR-020) - T054-T055 status: COMPLETE
- Exponential backoff formula - T083 status: not yet verified
- Deterministic jitter from context RNG
- Engine delta merging on retry (NOT just on success)

**Link to User Story**: T010 (Implement Sequential Retry Policy)

---

## US2: Node Timeouts (Per-Node Policies)

### Test File: `graph/policy_test.go`

#### Test 1: TestNodeTimeout - "enforces per-node timeout" (Line 52)

**Status**: ACTIVE (NOT SKIPPED)

**What It Tests**:
- Per-node timeout enforcement via `NodePolicy.Timeout`
- Node execution must be cancelled when timeout is exceeded
- Only the timed-out node is cancelled, not the entire workflow
- Other nodes continue normally
- Returns `context.DeadlineExceeded` error

**Acceptance Criteria**:
1. Node with 100ms timeout attempting 500ms work must be cancelled at ~100ms
2. Execution time must be ~100ms (not full 500ms)
3. Node receives context cancellation signal via `ctx.Done()`
4. Error indicates timeout via `errors.Is(err, context.DeadlineExceeded)` or "timeout"/"NODE_TIMEOUT" in message

**Test Setup**:
```go
type timedNode[S any] struct {
    fn      func(context.Context, S) graph.NodeResult[S]
    timeout time.Duration
}

func (n timedNode[S]) Policy() graph.NodePolicy {
    return graph.NodePolicy{
        Timeout: n.timeout,  // Per-node timeout value
    }
}
```

**Implementation Requirements**:
1. **NodePolicy Interface**: Nodes can implement optional `Policy() graph.NodePolicy` method
2. **Timeout Context**: Engine must create child context with deadline based on:
   - Node's `NodePolicy.Timeout` if > 0
   - Otherwise `Options.DefaultNodeTimeout` if > 0
   - Otherwise no timeout
3. **Cancellation Handling**: Pass timeout context to node `Run()` method
4. **Error Propagation**: Timeout errors must be returned as `NodeResult.Err`

**Dependencies/Prerequisites**:
- `NodePolicy` type definition
- `DefaultNodeTimeout` in `Options`
- Context deadline enforcement in scheduler

**Link to User Story**: T021 (Implement Timeout Policies)

---

#### Test 2: TestNodeTimeout - "uses DefaultNodeTimeout when Policy().Timeout is zero" (Line 129)

**Status**: SKIPPED at line 130

**Skip Reason**: 
```
"Pending implementation of per-node timeout enforcement (T076)"
```

**What It Tests**:
- Engine fallback behavior when node doesn't specify explicit timeout
- Uses global `Options.DefaultNodeTimeout` instead

**Acceptance Criteria**:
1. Node with no explicit timeout (Policy().Timeout == 0)
2. Engine creates context with `DefaultNodeTimeout` deadline
3. Node is cancelled after ~100ms (using default)
4. Works same as explicit timeout

**Test Setup**:
- Engine with `DefaultNodeTimeout=100ms`
- Node with no explicit timeout
- Node attempts to run for 500ms

**Implementation Requirements**:
- Same as Test 1, but with fallback logic for zero timeout

**Dependencies/Prerequisites**:
- Test 1 dependencies plus timeout fallback logic

**Link to User Story**: T021 (Implement Timeout Policies)

---

#### Test 3: TestNodeTimeout - "different nodes have independent timeouts" (Line 142)

**Status**: SKIPPED at line 143

**Skip Reason**:
```
"Pending implementation of per-node timeout enforcement (T076)"
```

**What It Tests**:
- Multiple nodes with different timeouts in same workflow
- Each node's timeout is independent
- One node timing out doesn't affect others

**Acceptance Criteria**:
1. Node A with 50ms timeout times out
2. Node B with 200ms timeout completes successfully
3. Workflow continues after Node A timeout
4. Error from A doesn't cancel B

**Test Setup**:
- Workflow: A(50ms timeout) -> B(200ms timeout)
- Both attempt reasonable work

**Implementation Requirements**:
- Context management must isolate timeouts per node
- Timeout of one node must not cascade to others

**Link to User Story**: T021 (Implement Timeout Policies)

---

#### Test 4: TestNodeTimeout - "no timeout when Policy().Timeout and DefaultNodeTimeout are zero" (Line 156)

**Status**: SKIPPED at line 157

**Skip Reason**:
```
"Pending implementation of per-node timeout enforcement (T076)"
```

**What It Tests**:
- Nodes can run indefinitely when no timeout configured
- Both explicit and default timeout are zero

**Acceptance Criteria**:
1. Engine with `DefaultNodeTimeout=0`
2. Node with `Policy().Timeout=0`
3. Node runs for 100ms without cancellation
4. Completes normally

**Implementation Requirements**:
- Check both timeout sources
- Skip timeout handling if both are zero
- Don't create deadline context if no timeout configured

**Link to User Story**: T021 (Implement Timeout Policies)

---

## US3: Backpressure (Queue-Based Flow Control)

### Test File: `graph/scheduler_test.go`

#### Test 1: TestBackpressureBlock - "enqueue to full frontier blocks" (Line 189)

**Status**: SKIPPED at line 192

**Skip Reason**:
```
"Backpressure testing deferred to Phase 5 (US3)"
```

**What It Tests**:
- Frontier queue blocks when reaching capacity
- Enqueue operation blocks until space available

**Acceptance Criteria**:
1. Frontier with capacity 5
2. Fill to capacity (5 items)
3. Attempt 6th enqueue - must block
4. Enqueue succeeds once space freed (via dequeue)

**Test Setup**:
- `NewFrontier[S](ctx, capacity=5, ...)` creates bounded queue
- Enqueue items until full
- Try enqueue in separate goroutine
- Verify it blocks
- Dequeue to free space
- Verify enqueue unblocks

**Implementation Requirements**:
1. **Frontier as bounded channel**: Underlying channel with fixed capacity
2. **Backpressure mechanism**: Enqueue blocks on full channel
3. **Channel semantics**: 
   - `Enqueue(ctx, item)` blocks when full
   - Respects context cancellation
   - Returns after item sent or context error
4. **Capacity checking**: `frontier.Len()` returns current queue size

**Dependencies/Prerequisites**:
- `Frontier[S]` type with bounded queue
- `WorkItem[S]` type with OrderKey, StepID, NodeID, State
- Channel-based queueing semantics

**Link to User Story**: T031 (Implement Backpressure/QueueDepth)

---

#### Test 2: TestBackpressureTimeout - "backpressure timeout triggers checkpoint and pause" (Line 449)

**Status**: SKIPPED at line 453

**Skip Reason**:
```
"Backpressure timeout implementation pending (T064)"
```

**What It Tests**:
- Engine detects sustained backpressure (enqueue blocking > timeout)
- Checkpoints current state before pausing
- Returns error indicating backpressure timeout
- Execution can resume from checkpoint

**Acceptance Criteria**:
1. Engine with `BackpressureTimeout=500ms`
2. Create condition where enqueue blocks > 500ms
3. Engine detects timeout condition
4. Saves checkpoint with frontier state
5. Returns `ErrBackpressureTimeout` or similar
6. Can resume from saved checkpoint

**Test Setup**:
- Small QueueDepth to trigger backpressure quickly
- Nodes producing work faster than consumption
- Verify checkpoint saved
- Verify resumable state

**Implementation Requirements**:
1. **BackpressureTimeout option**: Add to `Options` struct
2. **Backpressure detection**: Track time blocking on enqueue
3. **Checkpoint logic**: Before pausing, save:
   - Current state
   - Frontier queue contents (in OrderKey order)
   - Step counter
   - Current node ID
4. **Pause/Resume**: Return error allowing resume
5. **Observability**: Emit event about backpressure condition

**Dependencies/Prerequisites**:
- `Options.BackpressureTimeout` field
- Store checkpoint API
- Backpressure timeout detection mechanism
- Error type `ErrBackpressureTimeout`

**Note**: Test is documentation of expected behavior, implementation to follow T064

**Link to User Story**: T031 (Implement Backpressure/QueueDepth)

---

#### Test 3: TestBackpressureTimeout - "backpressure timeout emits observability event" (Line 465)

**Status**: SKIPPED at line 468

**Skip Reason**:
```
"Backpressure event emission pending (T069)"
```

**What It Tests**:
- Engine emits observability event when backpressure timeout occurs
- Event includes metadata (queue depth, timeout duration, etc.)

**Acceptance Criteria**:
1. Backpressure timeout condition triggered
2. Emitter receives "backpressure_timeout" event
3. Event includes:
   - Queue depth at timeout
   - Timeout duration elapsed
   - Current state information
   - Frontier size
   - Available workers count

**Test Setup**:
- Engine with custom emitter capturing events
- Trigger backpressure timeout
- Check emitted events

**Implementation Requirements**:
1. **Event emission**: Call `emitter.Emit(event)` on backpressure timeout
2. **Event type**: Define backpressure timeout event structure
3. **Metadata collection**: Capture queue/system state when timeout occurs
4. **Observability integration**: Work with existing `Emitter` interface

**Dependencies/Prerequisites**:
- Event emission framework
- Emitter interface enhancements
- Event type definitions

**Link to User Story**: T031 (Implement Backpressure/QueueDepth)

---

## Test Dependency Matrix

```
T004 (This Task) - Initial Analysis
  ├─ US1: Sequential Retries (T010)
  │  └─ TestDeterministicRetryDelays (T037)
  │     ├─ RNG seeding (FR-020) - T054-T055: DONE
  │     ├─ Exponential backoff (FR-009) - T083: TODO
  │     └─ Delta merging on retry - ENGINE CHANGE NEEDED
  │
  ├─ US2: Node Timeouts (T021)
  │  ├─ TestNodeTimeout (multiple tests at lines 52, 129, 142, 156)
  │  │  ├─ NodePolicy interface
  │  │  ├─ Context deadline handling
  │  │  └─ DefaultNodeTimeout option
  │  │
  │  └─ These tests guide T076 (Timeout Implementation)
  │
  └─ US3: Backpressure (T031)
     ├─ TestBackpressureBlock (line 189)
     │  └─ Frontier bounded queue semantics
     │
     ├─ TestBackpressureTimeout (line 449)
     │  ├─ BackpressureTimeout option
     │  ├─ Checkpoint on timeout
     │  └─ Resume capability
     │
     └─ TestBackpressureEventEmission (line 465)
        └─ Observability events
```

---

## Implementation Priority

Based on test analysis and spec review:

### Phase 1: US2 Node Timeouts (HIGHEST PRIORITY)
- **Why**: Existing tests validate immediate requirements
- **Effort**: Medium (context deadline management)
- **Tasks**: 
  1. Define `NodePolicy` interface
  2. Add `DefaultNodeTimeout` to Options
  3. Implement timeout context in scheduler
  4. Handle `context.DeadlineExceeded` errors

### Phase 2: US3 Backpressure (MEDIUM PRIORITY)
- **Why**: Framework for production stability
- **Effort**: High (queueing semantics + checkpointing)
- **Tasks**:
  1. Implement bounded Frontier queue
  2. Add backpressure detection
  3. Checkpoint on backpressure timeout
  4. Emit backpressure events

### Phase 3: US1 Deterministic Retry Delays (LOWER PRIORITY)
- **Why**: Clarification needed on semantics
- **Effort**: Medium (test needs redesign)
- **Considerations**:
  - Current test expects node-level delta merging on every attempt
  - Engine only merges on success
  - Either redesign test OR change engine semantics
  - Impact on all retry logic

---

## Key Findings

### Finding 1: Test Design vs Implementation Mismatch (US1)
The TestDeterministicRetryDelays test reveals a semantic gap:
- **Test expects**: Delta merging on every retry attempt (failure + retry delay tracking)
- **Engine implements**: Delta merging only on success
- **Resolution needed**: Either update test semantics or update engine behavior

### Finding 2: Timeout Tests Are Well-Specified (US2)
The policy tests provide clear requirements:
1. Per-node timeout via `NodePolicy.Timeout`
2. Fallback to `DefaultNodeTimeout`
3. No timeout when both are zero
4. Independent per-node timeouts
5. Clear error handling expectations

### Finding 3: Backpressure Has Three Distinct Concerns (US3)
1. **Basic backpressure**: Queue blocks on full (line 189)
2. **Backpressure timeout**: Checkpoint on sustained blocking (line 449)
3. **Observability**: Event emission for monitoring (line 465)

These should be implemented in order.

### Finding 4: RNG Infrastructure Complete
- RNG seeding from RunID: DONE
- Context-based RNG access: DONE
- Deterministic sequences: WORKING
- Ready for deterministic retry delays once test semantics clarified

---

## Recommendation

Focus implementation in this order:

1. **T076 (Node Timeout)**: Highest impact, well-defined requirements
2. **T064 (Backpressure Timeout)**: Foundation for US3
3. **Clarify US1 Test Semantics**: Understand if test needs redesign
4. **T083 (Exponential Backoff)**: Once US1 semantics clarified

This sequence unblocks T021 (Timeouts) and T031 (Backpressure) while allowing T010 (Retries) to proceed with clear requirements.
