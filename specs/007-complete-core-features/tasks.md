# Implementation Tasks: Complete Core Features for Production Readiness

**Branch**: `007-complete-core-features` | **Created**: 2025-10-30  
**Spec**: [spec.md](./spec.md) | **Review**: [incomplete-functionality-review.md](../incomplete-functionality-review.md)

## Overview

This task breakdown implements 3 missing core features identified in the comprehensive codebase review:
- **US1 (P1)**: Sequential Execution with Retries - Enable retry policies for sequential workflows  
- **US2 (P2)**: Per-Node Timeout Control - Enforce fine-grained timeout limits
- **US3 (P3)**: Backpressure Visibility - Add monitoring for queue saturation

**Target**: Complete 11 currently skipped tests and achieve production readiness for v0.2.0 GA

## Dependencies

### User Story Completion Order

```
US1 (Sequential Retries) → Independent, no dependencies
US2 (Node Timeouts)      → Independent, no dependencies  
US3 (Backpressure)       → Independent, no dependencies
```

All three user stories are **independent** and can be implemented in parallel or any order.

### Suggested MVP

**MVP Scope**: US1 only (Sequential Retries)
- **Rationale**: Highest priority (P1), blocking feature for production workflows
- **Value**: Enables resilient sequential workflows (financial transactions, audit trails)
- **Effort**: 1-2 days
- **Tests**: Unskips TestDeterministicRetryDelays, validates deterministic retry behavior

## Phase 1: Setup & Analysis

**Goal**: Understand existing implementations and prepare for enhancement

- [ ] T001 [P] Review concurrent retry implementation in graph/engine.go:runConcurrent (lines 920-1250)
- [ ] T002 [P] Review sequential execution path in graph/engine.go (locate runSequential or sequential execution logic)
- [ ] T003 [P] Analyze existing retry tests in graph/retry_integration_test.go for pattern reference
- [ ] T004 [P] Review skipped tests to understand requirements: graph/replay_test.go:483, graph/policy_test.go:47,103,116,130, graph/scheduler_test.go:191,452,467

## Phase 2: User Story 1 - Sequential Execution with Retries (P1)

**Story Goal**: Enable retry policies for sequential workflows (MaxConcurrentNodes: 0)

**Independent Test**: Create sequential workflow with retry policy, trigger transient failure, verify automatic retry with deterministic backoff

**Acceptance**:
- Sequential workflow retries up to configured limit with exponential backoff
- Retry delays are 100% deterministic across runs with same runID
- TestDeterministicRetryDelays passes (currently skipped)

### Implementation Tasks

- [ ] T005 [US1] Locate sequential execution method (likely in graph/engine.go, search for MaxConcurrentNodes == 0 check)
- [ ] T006 [US1] Add retry loop to sequential execution matching concurrent pattern in graph/engine.go
- [ ] T007 [US1] Implement deterministic retry backoff using existing RNG infrastructure in graph/engine.go
- [ ] T008 [US1] Extract retry attempt number into context (similar to AttemptKey pattern) in graph/engine.go
- [ ] T009 [US1] Add retry error handling (distinguish transient vs permanent) in graph/engine.go
- [ ] T010 [US1] Remove t.Skip() from TestDeterministicRetryDelays in graph/replay_test.go:483
- [ ] T011 [US1] Run TestDeterministicRetryDelays and verify 100% deterministic retry delays across 100 runs
- [ ] T012 [US1] Add example demonstrating sequential retry in examples/ directory (new file)
- [ ] T013 [US1] Update CLAUDE.md documenting sequential retry capability

## Phase 3: User Story 2 - Per-Node Timeout Control (P2)

**Story Goal**: Enforce NodePolicy.Timeout limits during node execution

**Independent Test**: Create workflow with mixed timeout policies, verify fast nodes timeout quickly while slow nodes run longer

**Acceptance**:
- NodePolicy.Timeout enforced during node execution
- DefaultNodeTimeout used as fallback when NodePolicy.Timeout is zero
- 4 skipped tests in policy_test.go pass

### Implementation Tasks

- [ ] T014 [P] [US2] Review NodePolicy interface in graph/policy.go and timeout field usage
- [ ] T015 [P] [US2] Review Options.DefaultNodeTimeout in graph/options.go:173
- [ ] T016 [US2] Locate node execution point in graph/engine.go (where node.Run() is called in both concurrent and sequential paths)
- [ ] T017 [US2] Wrap node.Run() with timeout context in concurrent execution (graph/engine.go runConcurrent method)
- [ ] T018 [US2] Wrap node.Run() with timeout context in sequential execution (graph/engine.go sequential method)
- [ ] T019 [US2] Implement timeout precedence logic (node timeout < default timeout < global timeout) in graph/engine.go
- [ ] T020 [US2] Create timeout error with node ID and duration in graph/engine.go
- [ ] T021 [US2] Remove t.Skip() from 4 tests in graph/policy_test.go:47,103,116,130
- [ ] T022 [US2] Run timeout tests and verify correct enforcement
- [ ] T023 [US2] Add example demonstrating per-node timeouts in examples/ directory (new file or update existing)
- [ ] T024 [US2] Update CLAUDE.md documenting timeout configuration

## Phase 4: User Story 3 - Backpressure Visibility (P3)

**Story Goal**: Emit metrics and events when workflow queue reaches capacity

**Independent Test**: Saturate work queue, verify backpressure metrics increment and events are emitted

**Acceptance**:
- Backpressure metrics increment when queue fills
- Events emitted with queue depth, wait time, node ID
- 3 skipped tests in scheduler_test.go pass

### Implementation Tasks

- [ ] T025 [P] [US3] Review Metrics interface IncrementBackpressure() method in graph/options.go
- [ ] T026 [P] [US3] Review Frontier.Enqueue backpressure handling in graph/scheduler.go
- [ ] T027 [US3] Add backpressure metric call when Enqueue blocks in graph/scheduler.go
- [ ] T028 [US3] Create backpressure event structure matching emit.Event format
- [ ] T029 [US3] Emit backpressure event through emitter when queue saturates in graph/scheduler.go
- [ ] T030 [US3] Add queue depth and wait duration to backpressure event metadata
- [ ] T031 [US3] Remove t.Skip() from 3 tests in graph/scheduler_test.go:191,452,467  
- [ ] T032 [US3] Run backpressure tests and verify metrics/events are emitted
- [ ] T033 [US3] Update prometheus_monitoring example to show backpressure metrics in examples/prometheus_monitoring/main.go
- [ ] T034 [US3] Update CLAUDE.md documenting backpressure monitoring

## Phase 5: Polish & Verification

**Goal**: Ensure all changes maintain quality standards and documentation is current

- [ ] T035 [P] Run full test suite: go test ./... and verify all tests pass
- [ ] T036 [P] Run golangci-lint and verify zero lint issues
- [ ] T037 [P] Run benchmarks and verify < 5% performance regression: go test -bench=. -benchmem ./...
- [ ] T038 Verify all 11 previously skipped tests now pass (1 in replay_test.go, 4 in policy_test.go, 3 in scheduler_test.go, 3 integration)
- [ ] T039 Update incomplete-functionality-review.md marking US1-US3 as complete
- [ ] T040 Run mcp-pr code review on all changes before committing
- [ ] T041 Create PR with summary of completed features and test results

## Parallel Execution Opportunities

### US1 Tasks (Sequential Retries)
Can run in parallel:
- T005-T006 (analysis and implementation prep)
- T012-T013 (documentation tasks)

Must run sequentially:
- T006 → T007 → T008 → T009 (retry logic implementation chain)
- T009 → T010 → T011 (implementation → test enablement → validation)

### US2 Tasks (Node Timeouts)
Can run in parallel:
- T014-T015 (review existing code)
- T023-T024 (documentation tasks)

Must run sequentially:
- T016 → T017 → T018 → T019 → T020 (timeout implementation chain)
- T020 → T021 → T022 (implementation → test enablement → validation)

### US3 Tasks (Backpressure)
Can run in parallel:
- T025-T026 (review existing code)
- T028 (event structure design)
- T033-T034 (documentation tasks)

Must run sequentially:
- T027 → T029 → T030 (metric and event emission chain)
- T030 → T031 → T032 (implementation → test enablement → validation)

### Cross-Story Parallelization
All three user stories (US1, US2, US3) can be implemented completely in parallel since they:
- Modify different code paths
- Have no shared dependencies
- Are independently testable

**Suggested Parallel Strategy**: Assign US1, US2, US3 to different developers or run sequentially based on priority (US1 → US2 → US3)

## Implementation Strategy

### MVP-First Approach

**Phase 1 MVP** (US1 only - 1-2 days):
1. Implement T005-T011 (Sequential retry logic)
2. Validate with TestDeterministicRetryDelays
3. Ship to unblock production workflows requiring sequential + retry

**Phase 2 Enhancement** (US2 - 2-3 days):
4. Implement T014-T022 (Per-node timeouts)
5. Validate with policy_test.go tests
6. Improves production efficiency with fine-grained timeout control

**Phase 3 Complete** (US3 - 2-3 days):
7. Implement T025-T032 (Backpressure monitoring)
8. Validate with scheduler_test.go tests
9. Adds production observability for capacity planning

### Testing Strategy

**TDD Approach** (Constitution Principle III):
- Each user story's tests are already written (currently skipped)
- Implementation follows Red→Green→Refactor:
  1. Unskip test (Red - should fail)
  2. Implement feature (Green - make it pass)
  3. Refactor for clarity (maintain passing state)

**Test Validation Points**:
- After T011: TestDeterministicRetryDelays must pass
- After T022: All 4 policy_test.go tests must pass
- After T032: All 3 scheduler_test.go backpressure tests must pass
- After T038: Full test suite passes with zero skipped tests

## Task Summary

**Total Tasks**: 41
- Setup & Analysis: 4 tasks
- US1 (Sequential Retries): 9 tasks
- US2 (Node Timeouts): 11 tasks
- US3 (Backpressure): 10 tasks
- Polish & Verification: 7 tasks

**Parallel Opportunities**: 12 tasks can run in parallel (marked with [P])

**Independent Test Criteria**:
- US1: Sequential workflow retries deterministically  
- US2: Per-node timeouts enforced correctly
- US3: Backpressure metrics and events emitted

**Estimated Effort**:
- US1: 1-2 days (MVP)
- US2: 2-3 days
- US3: 2-3 days
- Total: 5-8 days for complete implementation

**Success Metrics**:
- 11 skipped tests now passing
- Zero lint issues maintained
- < 5% performance regression
- 100% determinism maintained
