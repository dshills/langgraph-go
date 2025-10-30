# Implementation Plan: Critical Concurrency Bug Fixes

**Branch**: `005-critical-bug-fixes` | **Date**: 2025-10-29 | **Spec**: [spec.md](./spec.md)  
**Input**: Feature specification from `/specs/005-critical-bug-fixes/spec.md`

## Summary

This feature addresses 4 critical concurrency bugs identified in the comprehensive code review that cause deadlocks, race conditions, and non-deterministic behavior in concurrent workflow execution. The fixes ensure reliable error delivery, thread-safe random number generation, deterministic work item ordering, and accurate completion detection while maintaining backward compatibility and deterministic replay guarantees.

## Technical Context

**Language/Version**: Go 1.21+ (requires generics, math/rand, sync/atomic)  
**Primary Dependencies**: 
- Go standard library: sync, sync/atomic, context, container/heap
- No external dependencies for core fixes  
**Storage**: N/A (bug fixes in execution engine, not persistence layer)  
**Testing**: go test with -race flag (race detector), benchmark tests for performance validation  
**Target Platform**: Cross-platform (Linux, macOS, Windows) - Go runtime  
**Project Type**: Library/framework (graph execution engine)  
**Performance Goals**: 
- Zero race conditions (race detector clean)
- <5% throughput degradation
- <10% memory overhead increase  
**Constraints**: 
- Backward compatibility with existing public APIs (no breaking changes)
- Deterministic replay must remain functional
- All existing tests must pass  
**Scale/Scope**: 
- 4 critical bugs across 3 files (engine.go, scheduler.go, policy.go)
- ~200-300 lines of code changes
- 10-15 new test cases

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### I. Type Safety & Determinism

**Compliance**: ✅ PASS

- Fixes preserve generic type system (Engine[S], WorkItem[S])
- Per-worker RNG maintains deterministic replay with derived seeds
- Frontier heap ordering preserves OrderKey determinism
- No changes to state serialization or reducer patterns

**Justification**: Bug fixes strengthen determinism by eliminating race conditions that could cause non-deterministic results.

### II. Interface-First Design

**Compliance**: ✅ PASS

- No changes to public interfaces (Node[S], Store[S], Emitter)
- Internal implementation fixes only
- Maintains interface contracts

**Justification**: Pure bug fixes with no API surface changes.

### III. Production Readiness

**Compliance**: ✅ PASS (Strengthened)

- Fixes eliminate critical production blockers (deadlocks, races)
- Race detector validation ensures thread safety
- Performance benchmarks prevent regressions
- Existing observability (metrics, events) remains functional

**Justification**: These fixes are prerequisites for production deployment at scale.

### IV. Test Coverage & Quality

**Compliance**: ✅ PASS

- New tests required for each bug fix
- Race detector tests validate thread safety
- Stress tests validate completion detection
- Performance benchmarks prevent regressions

**Expected Coverage**: +5-10% coverage in graph package (currently 57.8%)

### V. Documentation Standards

**Compliance**: ✅ PASS

- Code comments updated to explain concurrency patterns
- godoc preserved and enhanced
- Changelog entry documenting bug fixes
- No new public APIs requiring documentation

### Development Workflow

**Compliance**: ✅ PASS

- Will use mcp-pr code review before commits
- All tests with race detector before merge
- Performance benchmarks before merge

**GATE STATUS**: ✅ ALL GATES PASSED - Proceed to Phase 0

## Project Structure

### Documentation (this feature)

```text
specs/005-critical-bug-fixes/
├── spec.md              # Feature specification
├── plan.md              # This file
├── research.md          # Phase 0: Concurrency patterns research
├── data-model.md        # N/A (no data model changes for bug fixes)
├── quickstart.md        # N/A (internal fixes, no user-facing changes)
├── contracts/           # N/A (no API contract changes)
└── checklists/
    └── requirements.md  # Specification quality validation
```

### Source Code (repository root)

```text
graph/
├── engine.go            # FIX: Results channel sizing, completion detection, RNG per-worker
├── scheduler.go         # FIX: Frontier heap/channel synchronization
├── policy.go            # REVIEW: RNG fallback pattern
├── engine_test.go       # NEW: Race condition tests, stress tests
├── scheduler_test.go    # NEW: Frontier ordering tests
└── concurrency_test.go  # NEW: Dedicated concurrency validation tests

specs/005-critical-bug-fixes/
└── research.md          # Concurrency patterns, Go race detector best practices
```

**Structure Decision**: Single library project with internal bug fixes. No new packages or external dependencies. Changes localized to existing graph/ package files.

## Complexity Tracking

> This section intentionally left empty - no constitution violations to justify.

All fixes align with constitution principles and strengthen production readiness.

---

## Phase 0: Research & Discovery

### Unknowns to Resolve


1. **Optimal Results Channel Buffer Size**: Determine correct sizing formula for results channel to prevent deadlock without excessive memory overhead
2. **RNG Seed Derivation Strategy**: Best practice for deriving per-worker seeds from base seed while maintaining deterministic replay
3. **Frontier Synchronization Pattern**: Best approach for heap+channel coordination (notification-only vs dual-queue)
4. **Completion Detection Signal**: Most reliable pattern for detecting workflow completion without polling or races

### Research Tasks

#### R1: Go Channel Buffer Sizing Patterns
**Question**: What is the industry-standard approach for sizing result channels in worker pool patterns?

**Research Focus**:
- Go concurrency patterns documentation
- Worker pool implementations in production Go code
- Backpressure and buffering strategies
- Trade-offs: memory vs blocking behavior

**Expected Output**: Recommended buffer size formula with rationale

---

#### R2: Thread-Safe RNG in Go Concurrent Systems
**Question**: How should deterministic RNG be safely shared across goroutines while maintaining reproducibility?

**Research Focus**:
- math/rand thread safety documentation
- Per-goroutine RNG patterns
- Seed derivation for independent but deterministic RNGs
- Alternatives to math/rand (crypto/rand, math/rand/v2 in Go 1.22+)

**Expected Output**: Pattern for per-worker RNG with deterministic seeding

---

#### R3: Priority Queue + Channel Synchronization
**Question**: What is the correct pattern for combining heap-based priority queues with channel-based work distribution?

**Research Focus**:
- container/heap best practices
- Channel vs condition variable for work notification
- Synchronization patterns preventing desynchronization
- Performance comparison: notification-only vs dual-structure

**Expected Output**: Recommended synchronization pattern with code example

---

#### R4: Completion Detection in Concurrent Work Pools
**Question**: What is the most reliable way to detect completion without polling or race conditions?

**Research Focus**:
- WaitGroup patterns for worker coordination
- Atomic counters vs channels for state tracking
- Completion signaling without polling
- Avoiding race between "last worker finishing" and "completion check"

**Expected Output**: Recommended completion detection pattern

---

### Research Output

**Destination**: `research.md` in this directory

**Format**:
```markdown
## Decision: [What was chosen]

**Rationale**: [Why this approach]

**Alternatives Considered**:
- Alternative A: [pros/cons]
- Alternative B: [pros/cons]

**Code Example**: [Illustrative pattern]

**Performance Impact**: [Expected impact on throughput/memory]
```

---

## Phase 1: Design Artifacts

### Data Model

**Status**: N/A for this feature

**Justification**: Bug fixes do not introduce new data entities. Existing entities (WorkItem, nodeResult, Frontier) are modified internally without schema changes.

### API Contracts

**Status**: N/A for this feature

**Justification**: No public API changes. All fixes are internal implementation improvements maintaining existing method signatures and behaviors.

### Quickstart Guide

**Status**: N/A for this feature

**Justification**: Internal bug fixes do not require user-facing documentation changes. Existing usage patterns remain unchanged.

---

## Phase 2: Implementation Strategy

*NOTE: This section provides guidance for `/speckit.tasks` command. Detailed tasks will be generated in tasks.md.*

### Implementation Approach

**Fix Order** (by dependency):

1. **BUG-002 (RNG Thread Safety)** - FIRST
   - No dependencies
   - Affects retry backoff
   - Required for deterministic replay

2. **BUG-001 (Results Channel Deadlock)** - SECOND  
   - No dependencies
   - Critical for error reporting
   - Most straightforward fix

3. **BUG-003 (Frontier Ordering)** - THIRD
   - Independent of other fixes
   - Affects work distribution
   - Requires careful synchronization changes

4. **BUG-004 (Completion Detection)** - FOURTH
   - May depend on BUG-003 fix for validation
   - Requires understanding of worker lifecycle
   - Most complex interaction to test

### Testing Strategy

**Per-Bug Test Requirements**:

1. **Unit Tests**: Isolated test for each bug fix
   - Mock/stub dependencies
   - Reproduce trigger condition
   - Verify fix prevents issue

2. **Race Detector Tests**: Run with `-race` flag
   - Validate thread safety
   - Must pass cleanly (zero races)

3. **Stress Tests**: High-load validation
   - 100+ concurrent workers
   - 1000+ work items
   - Rapid context cancellations
   - Simultaneous errors

4. **Determinism Tests**: Replay validation
   - Same workflow 100+ times
   - Verify identical outputs
   - Validate OrderKey compliance

5. **Performance Benchmarks**: Regression prevention
   - Baseline current throughput
   - Measure post-fix throughput
   - Verify <5% degradation

### Risk Mitigation

**High Risk Areas**:
- Frontier synchronization changes could break ordering guarantees
- Channel buffer size changes could mask other issues
- Per-worker RNG could break determinism if seeds not derived correctly

**Mitigation**:
- Extensive determinism testing (100+ identical runs)
- Frontier ordering validation with 10,000+ items
- Seed derivation verification against test vectors

---

## Dependencies

**Prerequisites**:
- Current test suite passes (baseline)
- Race detector available (`go test -race`)
- Benchmark framework functional

**Blocks**:
- Architectural refactoring (should wait for bug fixes)
- Performance optimization work (needs stable baseline)
- Production deployment (blocked until fixes merged)

---

## Success Validation

### Validation Checklist

After implementation, verify:

- [ ] All 4 bugs have corresponding unit tests that fail before fix, pass after
- [ ] Race detector reports zero races: `go test -race ./graph/...`
- [ ] Determinism test: 100 identical runs produce identical outputs
- [ ] Stress test: 1000 concurrent executions without hangs or panics
- [ ] Performance benchmark: <5% throughput degradation
- [ ] Memory profile: <10% allocation increase
- [ ] All existing tests pass
- [ ] Code review via mcp-pr
- [ ] Documentation updated (comments, changelog)

### Acceptance Criteria Mapping

| Success Criterion | Validation Method |
|-------------------|-------------------|
| SC-001: 1000 consecutive executions | Stress test loop |
| SC-002: Zero race conditions | `go test -race` |
| SC-003: Byte-identical replay | Determinism test |
| SC-004: 100% error delivery | Error injection test |
| SC-005: 100% OrderKey compliance | Frontier ordering test |
| SC-006: 100% completion accuracy | Completion detection test |
| SC-007: <5% throughput impact | Benchmark comparison |
| SC-008: <10% memory increase | Memory profiling |

---

**Next Command**: `/speckit.tasks` to generate actionable task breakdown
