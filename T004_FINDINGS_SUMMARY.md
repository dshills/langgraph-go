# T004: Skipped Tests Analysis - Executive Summary

## Overview

Analyzed 8 skipped tests across 3 test files to understand feature requirements:

| User Story | Test File | Tests | Status | Priority |
|-----------|-----------|-------|--------|----------|
| US1: Sequential Retries | `replay_test.go` | 1 | SEMANTIC ISSUE | LOW |
| US2: Node Timeouts | `policy_test.go` | 4 | WELL-DEFINED | HIGH |
| US3: Backpressure | `scheduler_test.go` | 3 | DEFERRED | MEDIUM |

---

## Quick Reference: Test Locations

### US1: Deterministic Retry Delays
- **File**: `graph/replay_test.go:483`
- **Test**: `TestDeterministicRetryDelays()`
- **Status**: Needs semantic clarification
- **Issue**: Test expects delta merging on failures; engine only merges on success

### US2: Node Timeout Policies  
- **File**: `graph/policy_test.go`
- **Tests**:
  1. Line 52: `enforces per-node timeout` (ACTIVE - not skipped)
  2. Line 129: `uses DefaultNodeTimeout when Policy().Timeout is zero` (SKIPPED)
  3. Line 142: `different nodes have independent timeouts` (SKIPPED)
  4. Line 156: `no timeout when both timeouts are zero` (SKIPPED)

### US3: Backpressure Queue Control
- **File**: `graph/scheduler_test.go`
- **Tests**:
  1. Line 189: `enqueue to full frontier blocks` (SKIPPED)
  2. Line 449: `backpressure timeout triggers checkpoint and pause` (SKIPPED)
  3. Line 465: `backpressure timeout emits observability event` (SKIPPED)

---

## US1: Sequential Retries - SEMANTIC ISSUE

### Current State
- RNG seeding: COMPLETE (T054-T055)
- Exponential backoff formula: SPECIFIED, not implemented
- Retry logic itself: WORKING (all other retry tests pass)

### The Problem
TestDeterministicRetryDelays expects:
- Engine to merge `NodeResult.Delta` on EVERY retry attempt (even failures)
- State to accumulate retry delays: `RetryDelays []time.Duration`
- Final state after 3 failures + success to have 3 delay entries

Current Engine:
- Only merges deltas when node returns success (Err == nil)
- Doesn't merge deltas from failed attempts
- Retry happens at engine level, not visible in merged state

### What This Test Validates
1. RNG produces deterministic values (WORKING)
2. Retry delays are computed from RNG (NEEDS EXPONENTIAL BACKOFF)
3. Same RunID → identical delay sequences (WOULD WORK if semantics fixed)
4. State accumulates all delays (BLOCKED BY SEMANTIC DIFFERENCE)

### Resolution Options
**Option A**: Redesign test
- Test should verify determinism of RNG jitter only
- Separate concerns: RNG determinism vs delay accumulation
- Simpler and clearer intent

**Option B**: Change engine semantics  
- Merge deltas on every attempt (success and failure)
- Requires careful state management
- Impact on all retry logic flows

### Recommendation
**Option A is preferred**: Redesign test to focus on RNG determinism rather than delta merging semantics. The current test conflates two concerns.

---

## US2: Node Timeout Policies - WELL DEFINED

### Feature Requirements Summary

All 4 tests define consistent requirements:

1. **NodePolicy Interface**
   - Optional `Policy() graph.NodePolicy` method on nodes
   - Returns timeout configuration per node

2. **Timeout Hierarchy**
   - Per-node timeout: `NodePolicy.Timeout`
   - Fallback to: `Options.DefaultNodeTimeout`
   - If both zero: No timeout

3. **Implementation Details**
   - Create context with deadline: `context.WithDeadline(parent, now.Add(timeout))`
   - Pass timeout context to `node.Run(ctx, state)`
   - Node receives cancellation via `ctx.Done()`
   - Timeout error: `context.DeadlineExceeded`

4. **Isolation**
   - Each node gets independent timeout context
   - One node timing out doesn't affect others
   - Only the timed-out node is cancelled

### Test Structure
```
TestNodeTimeout
├─ enforces per-node timeout (ACTIVE)
│  ├─ Validates 100ms timeout enforced when node attempts 500ms work
│  └─ Verifies error is context.DeadlineExceeded
│
├─ uses DefaultNodeTimeout fallback (SKIPPED)
│  └─ When NodePolicy.Timeout == 0, uses Options.DefaultNodeTimeout
│
├─ independent per-node timeouts (SKIPPED)
│  └─ NodeA(50ms) times out; NodeB(200ms) succeeds
│
└─ no timeout when both zero (SKIPPED)
   └─ Both Policy().Timeout and DefaultNodeTimeout == 0 → runs indefinitely
```

### Implementation Effort
- **Complexity**: Medium (context deadline management)
- **Lines of Code**: ~30-40 in scheduler
- **Integration Points**: Node interface, Options, scheduler
- **Tests**: 4 comprehensive test cases provided

### Dependencies
- Context API (standard library)
- Optional interface on Node type
- No external dependencies

---

## US3: Backpressure - DEFERRED BUT WELL DOCUMENTED

### Feature Scope

Three distinct concerns documented by tests:

#### 1. Basic Backpressure (Line 189)
**What**: Frontier queue blocks on full
- **Setup**: Bounded queue with capacity
- **Behavior**: Enqueue blocks when full, unblocks when dequeued
- **Semantics**: Channel-like backpressure
- **Dependencies**: Bounded Frontier queue

#### 2. Backpressure Timeout (Line 449)
**What**: Detect sustained blocking and checkpoint
- **Setup**: Engine with `BackpressureTimeout` option
- **Behavior**: If enqueue blocked > timeout:
  1. Save checkpoint with frontier state
  2. Pause execution
  3. Return `ErrBackpressureTimeout`
- **Resume**: Can be resumed from checkpoint
- **Dependencies**: Checkpoint API, backpressure detection

#### 3. Backpressure Events (Line 465)
**What**: Emit observability events
- **Setup**: Engine with emitter
- **Behavior**: Emit "backpressure_timeout" event with metadata:
  - Queue depth
  - Timeout duration
  - Current state
- **Dependencies**: Event emission framework

### Implementation Roadmap

```
Phase 1: Basic Queue (Foundation)
├─ NewFrontier() with bounded channel
├─ Enqueue/Dequeue with blocking semantics
└─ Tests: TestBackpressureBlock (line 209)

Phase 2: Timeout Detection (T064)
├─ BackpressureTimeout option
├─ Track enqueue blocking duration
├─ Checkpoint on timeout
└─ Tests: TestBackpressureTimeout (line 449)

Phase 3: Observability (T069)
├─ Backpressure timeout events
├─ Metadata collection
└─ Tests: TestBackpressureEventEmission (line 465)
```

### Current Status
- Basic frontier ordering: DONE (BUG-003 fixed)
- OrderKey sorting: DONE
- Bounded queue: NOT YET
- Backpressure timeout: NOT YET
- Event emission: NOT YET

---

## Implementation Priority Recommendation

### Priority 1: US2 Node Timeouts (START NOW)
**Why**: 
- Well-defined requirements ✓
- Limited scope (40 lines engine code)
- High production value
- Unblocks T021

**Effort**: 3-5 days
**Owner**: Assign to implementation team

### Priority 2: US3 Backpressure (START AFTER US2)
**Why**:
- Foundation for production stability
- Prevents memory exhaustion
- Clear phased approach
- Tests document all requirements

**Effort**: 8-12 days (3 phases)
**Owner**: Assign sequentially

### Priority 3: US1 Retry Delays (CLARIFY FIRST)
**Why**:
- Semantic issue needs resolution
- Could be quick if test redesigned
- Lower priority than timeouts/backpressure
- Retry logic already works

**Effort**: 1 day clarification + 2-3 days implementation
**Action**: Clarify test semantics with team

---

## Key Insights

### Insight 1: Infrastructure is Ready for US1
- RNG seeding works ✓
- Deterministic sequences verified ✓
- Only missing: exponential backoff formula + test semantics

### Insight 2: US2 Tests are Actionable
- All requirements explicit in test cases
- No ambiguity in expected behavior
- Test setup shows exactly what needs to work

### Insight 3: US3 Has Clear Phasing
- Basic queue separate from timeout detection
- Timeout detection separate from observability
- Can implement incrementally

### Insight 4: All Tests are Production-Grade
- Each test includes stress/edge cases
- Error handling validated
- Concurrency/cancellation tested
- Ready to guide implementation

---

## Deliverable Location

Full analysis saved to: `/Users/dshills/Development/projects/langgraph-go/T004_SKIPPED_TESTS_ANALYSIS.md`

Contents:
- 8 detailed test breakdowns
- Test-by-test acceptance criteria
- Implementation requirements per feature
- Dependency matrix
- Setup/fixture documentation
- Key findings and recommendations

---

## Next Steps

1. **Clarify US1 semantics** with team (1 meeting)
2. **Start T076 (Node Timeout implementation)** 
   - Use Test 1 (line 52) as acceptance criteria
   - Implement Tests 2-4 requirements
3. **Plan T064 (Backpressure Timeout)** after US2 baseline
4. **Revisit T010 (Retry Delays)** after US1 semantics confirmed
