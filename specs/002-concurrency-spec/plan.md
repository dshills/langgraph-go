# Implementation Plan: Concurrent Graph Execution with Deterministic Replay

**Branch**: `002-concurrency-spec` | **Date**: 2025-10-28 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/002-concurrency-spec/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

This feature implements concurrent execution of graph nodes with deterministic replay guarantees. The primary requirements are: (1) parallel execution of independent nodes to reduce total workflow time, (2) deterministic state merging via pure reducers, (3) checkpoint-based replay that produces identical results without re-executing external calls, (4) bounded concurrency with backpressure control, and (5) cancellation/timeout enforcement. The technical approach uses goroutines with work queues, deterministic ordering via `(step_id, order_key)` tuples, recorded I/O for replay, and atomic checkpoint commits to ensure exactly-once state advancement.

## Technical Context

**Language/Version**: Go 1.21+ (requires generics support)
**Primary Dependencies**: Go standard library only (core framework), optional adapters for OpenTelemetry SDK, MySQL driver
**Storage**: Store interface supports in-memory (testing) and MySQL/Aurora (production) implementations
**Testing**: Go testing framework (`go test`), table-driven tests, integration tests with MySQL, benchmark tests
**Target Platform**: Linux/macOS/Windows servers, containerized environments (Docker/Kubernetes)
**Project Type**: Single library project with example applications
**Performance Goals**: Support 1000+ concurrent node executions, <10ms scheduler overhead per step, deterministic replay <100ms for 1000-step workflows
**Constraints**: Atomic checkpoint commits required, bounded queue depth to prevent memory exhaustion, cancellation propagation <1 second, zero duplicate step commits
**Scale/Scope**: Handle graphs with 100+ nodes, 10,000+ steps per run, 10,000 concurrent runs in production, support fan-out of 100+ branches

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### Principle I: Type Safety & Determinism ✅
- **Status**: PASS
- **Evidence**: Feature requires deterministic replay via pure reducers, seeded RNG for stable random values, recorded I/O for external calls, and atomic checkpoint commits. All state transitions will use generic type `S` with explicit `NodeResult[S]` returns.
- **Requirements Met**: FR-004 (pure reducers), FR-007 (deterministic replay), FR-020 (seeded PRNG), FR-023 (reducer purity enforcement)

### Principle II: Interface-First Design ✅
- **Status**: PASS
- **Evidence**: Concurrency features will extend existing interfaces (`Node[S]`, `Store[S]`, `Emitter`) without breaking changes. New abstractions (scheduler, frontier queue, work item) will be internal to engine implementation. Store interface will be extended to support atomic multi-entity commits.
- **Requirements Met**: Existing Store interface will be enhanced with batch operations while maintaining backward compatibility

### Principle III: Test-Driven Development (NON-NEGOTIABLE) ✅
- **Status**: PASS - REQUIRES STRICT ENFORCEMENT
- **Evidence**: Feature requires comprehensive test coverage for deterministic replay, merge order, backpressure, retries, cancellation, and exactly-once guarantees (FR-008, SC-002, SC-005, SC-006). All tests must be written BEFORE implementation.
- **Requirements Met**: TDD cycle will be enforced for all 30 functional requirements. Integration tests required for concurrent execution scenarios.
- **Action Items**:
  - Write failing tests for deterministic ordering (FR-002)
  - Write failing tests for replay mismatch detection (FR-008)
  - Write failing tests for atomic checkpoint commits (FR-018, FR-019)
  - Write failing tests for backpressure blocking (FR-011)
  - Write benchmark tests for concurrent execution (SC-001)

### Principle IV: Observability & Debugging ✅
- **Status**: PASS
- **Evidence**: Feature explicitly requires OpenTelemetry spans (FR-026), metrics for queue depth/step latency/active nodes (FR-027), event emission for all node executions, and structured event data for debugging concurrent workflows.
- **Requirements Met**: FR-026 (OpenTelemetry spans), FR-027 (metrics), FR-029 (async event emission)

### Principle V: Dependency Minimalism ✅
- **Status**: PASS
- **Evidence**: Core concurrency logic will use only Go standard library (context, sync, channels). OpenTelemetry and store-specific dependencies remain optional in adapter packages.
- **Requirements Met**: No new external dependencies in core `/graph` package. Existing adapter pattern preserved.

### Go Idioms & Best Practices ✅
- **Status**: PASS
- **Evidence**:
  - Concurrency: Goroutines for parallel execution (FR-001), channels for work queues, context.Context for cancellation (FR-013, FR-022)
  - Error Handling: Explicit error returns in NodeResult, retry policies (FR-017)
  - Generics: Consistent `[S any]` throughout Engine, Node, Store
- **Requirements Met**: All Go idioms will be followed (gofmt, context cancellation, explicit errors)

### Development Workflow: Code Review ✅
- **Status**: PASS - MANDATORY PRE-COMMIT REVIEW
- **Evidence**: All concurrency code changes MUST be reviewed with `mcp-pr` before commits due to complexity and risk of race conditions, deadlocks, and non-deterministic behavior.
- **Action Items**:
  - Run `mcp-pr review_unstaged` before each commit
  - Focus review on: race conditions, deadlock potential, determinism guarantees, error handling in concurrent paths
  - Document any intentional deviations with clear rationale

### GATE RESULT: ✅ PASS - Proceed to Phase 0 Research

No constitutional violations detected. Feature aligns with all core principles. Special attention required for:
1. TDD enforcement (Principle III) - concurrency bugs are difficult to debug after implementation
2. Pre-commit review (Development Workflow) - concurrent code requires careful review
3. Determinism testing (Principle I) - must validate replay guarantees rigorously

## Project Structure

### Documentation (this feature)

```text
specs/[###-feature]/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
graph/
├── engine.go             # Enhanced with concurrent scheduler, frontier queue
├── engine_test.go        # Tests for concurrent execution, determinism
├── node.go               # Node interface (no changes)
├── node_test.go          # Tests for node execution with concurrency
├── edge.go               # Edge interface (no changes)
├── edge_test.go          # Edge tests
├── state.go              # Reducer type (no changes)
├── state_test.go         # Reducer tests with concurrent scenarios
├── scheduler.go          # NEW: Work queue, order key generation, frontier management
├── scheduler_test.go     # NEW: Tests for scheduling logic, deterministic ordering
├── replay.go             # NEW: I/O recording, replay engine, mismatch detection
├── replay_test.go        # NEW: Tests for deterministic replay scenarios
├── checkpoint.go         # NEW: Checkpoint management, idempotency keys
├── checkpoint_test.go    # NEW: Tests for atomic commits, exactly-once
├── policy.go             # NEW: NodePolicy, RetryPolicy, SideEffectPolicy types
├── policy_test.go        # NEW: Tests for retry logic, timeout enforcement
├── benchmark_test.go     # Enhanced with concurrency benchmarks
├── integration_test.go   # Enhanced with concurrent execution scenarios
├── store/
│   ├── store.go          # Enhanced Store interface with batch operations
│   ├── memory.go         # Enhanced MemStore with atomic batch commits
│   ├── memory_test.go    # Tests for concurrent access
│   ├── mysql.go          # Enhanced MySQLStore with transactional outbox
│   └── mysql_test.go     # Integration tests with concurrent runs
├── emit/
│   ├── emitter.go        # Enhanced with async event draining
│   ├── log.go            # Log emitter (no changes needed)
│   ├── otel.go           # Enhanced with span attributes for concurrency
│   └── otel_test.go      # Tests for concurrent event emission
├── model/
│   └── [no changes]      # ChatModel adapters unchanged
└── tool/
    └── [no changes]      # Tool interface unchanged

examples/
├── concurrent_workflow/  # NEW: Example demonstrating parallel execution
│   └── main.go
├── replay_demo/          # NEW: Example demonstrating checkpoint replay
│   └── main.go
└── [existing examples]

docs/
├── concurrency.md        # NEW: Concurrency model documentation
├── replay.md             # NEW: Deterministic replay guide
└── [existing docs]
```

**Structure Decision**: Single library project structure. All concurrency features are implemented within the existing `/graph` package to maintain backward compatibility. New files (`scheduler.go`, `replay.go`, `checkpoint.go`, `policy.go`) are added alongside existing engine implementation. Store and Emitter interfaces are enhanced with backward-compatible extensions. Examples demonstrate new concurrency capabilities. This structure preserves the existing architecture while adding concurrent execution capabilities as internal engine enhancements.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

No constitutional violations detected. This section is not applicable.

---

## Post-Phase 1 Constitution Re-Check

*Required after completing design artifacts (research.md, data-model.md, contracts/)*

### Principle I: Type Safety & Determinism ✅
- **Status**: PASS
- **Design Evidence**:
  - All entities use generic type parameter `[S any]` (WorkItem, Checkpoint, Engine)
  - Deterministic ordering via hash-based `order_key` (research.md #1)
  - Seeded RNG with `math/rand` (research.md #10)
  - Recorded I/O stored in Checkpoint for replay (data-model.md)
  - Idempotency keys prevent duplicate commits (data-model.md)
- **Validation**: Design preserves determinism guarantees

### Principle II: Interface-First Design ✅
- **Status**: PASS
- **Design Evidence**:
  - Enhanced Store interface with backward-compatible V2 methods (contracts/engine_api.md)
  - Enhanced Emitter interface with batch operations (contracts/engine_api.md)
  - Node interface extended with optional Policy() and Effects() methods
  - New types (WorkItem, NodePolicy, RetryPolicy) are value types, not interfaces
  - All external dependencies (OpenTelemetry) remain in adapter packages
- **Validation**: Interface enhancements maintain backward compatibility, no breaking changes

### Principle III: Test-Driven Development ✅
- **Status**: PASS - ENFORCED
- **Design Evidence**:
  - Test helpers defined in contracts (NewTestEngine, NewConcurrentTestEngine)
  - TDD action items documented in initial Constitution Check
  - Benchmark test strategy defined in research.md #12
  - Integration test requirements specified for concurrent scenarios
- **Validation**: Design includes testability hooks. TDD must be enforced during implementation.
- **Critical Tests Required**:
  - Deterministic ordering (FR-002)
  - Replay mismatch detection (FR-008)
  - Atomic checkpoint commits (FR-018, FR-019)
  - Backpressure blocking (FR-011)
  - Concurrent execution performance (SC-001)

### Principle IV: Observability & Debugging ✅
- **Status**: PASS
- **Design Evidence**:
  - SchedulerMetrics entity tracks performance counters (data-model.md)
  - New event types defined (EventStepStart, EventBackpressure, EventReplayMismatch)
  - Emitter.EmitBatch() for efficient event emission (contracts/engine_api.md)
  - Context values propagated for debugging (RunIDKey, StepIDKey, AttemptKey)
  - Checkpoint includes timestamp, duration for forensics
- **Validation**: Design provides comprehensive observability hooks

### Principle V: Dependency Minimalism ✅
- **Status**: PASS
- **Design Evidence**:
  - Core uses only Go stdlib: context, sync, container/heap, crypto/sha256
  - No new external dependencies added to `/graph` package
  - OpenTelemetry remains optional in `/graph/emit/otel.go`
  - MySQL driver remains optional in `/graph/store/mysql.go`
- **Validation**: Zero new dependencies. Design maintains stdlib-only core.

### Go Idioms & Best Practices ✅
- **Status**: PASS
- **Design Evidence**:
  - Buffered channels for work queue (idiomatic concurrency)
  - context.Context for cancellation propagation
  - Explicit error returns (no panics in library code)
  - Generics usage consistent with existing patterns
  - JSON serialization for portability
- **Validation**: Design follows Go conventions throughout

### Development Workflow: Code Review ✅
- **Status**: PASS - MANDATORY ENFORCEMENT
- **Design Evidence**:
  - Pre-commit review explicitly documented in initial check
  - Focus areas defined: race conditions, deadlocks, determinism
  - Quickstart includes troubleshooting for common concurrency issues
- **Validation**: Review process defined. Must be enforced during implementation.

### FINAL GATE RESULT: ✅ PASS - Ready for Phase 2 (Task Breakdown)

**Summary**: All design artifacts (research.md, data-model.md, contracts/, quickstart.md) have been reviewed against constitution principles. No violations detected. Design maintains:
- ✅ Backward compatibility (no breaking changes)
- ✅ Type safety via generics
- ✅ Deterministic replay guarantees
- ✅ Interface-first approach
- ✅ Zero new dependencies
- ✅ Comprehensive observability
- ✅ TDD readiness with test helpers

**Critical Success Factors**:
1. **TDD Enforcement**: Must write tests BEFORE implementation (Principle III)
2. **Pre-commit Review**: Must use `mcp-pr` for all concurrency code (Development Workflow)
3. **Determinism Validation**: Must verify replay produces identical results (Principle I)

**Ready to proceed to `/speckit.tasks` for implementation task breakdown.**
