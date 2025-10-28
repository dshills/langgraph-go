# Tasks: Production Hardening and Documentation Enhancements

**Input**: Design documents from `/specs/003-production-hardening/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/api_enhancements.md

**Tests**: Tests are included as this is a production hardening feature requiring contract validation.

**Organization**: Tasks are grouped by user story (7 stories, P1-P3 priorities) to enable independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

This project follows Go standard layout:
- `graph/` - Core graph execution engine
- `graph/store/` - Store implementations
- `graph/emit/` - Emitter implementations
- `docs/` - Documentation
- `examples/` - Example applications

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Dependency installation and project structure

- [X] T001 Add SQLite dependency (modernc.org/sqlite) to go.mod
- [X] T002 Add Prometheus client dependency (github.com/prometheus/client_golang) to go.mod
- [X] T003 Run go mod tidy to update dependencies

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T004 [P] Export typed errors in graph/errors.go (ErrReplayMismatch, ErrNoProgress, ErrBackpressure, ErrMaxStepsExceeded, ErrIdempotencyViolation, ErrMaxAttemptsExceeded)
- [X] T005 [P] Create metrics types stub in graph/metrics.go (PrometheusMetrics struct with placeholders)
- [X] T006 [P] Create cost tracking stub in graph/cost.go (CostTracker struct with placeholders)

**Checkpoint**: Foundation ready - user story implementation can now begin in parallel

---

## Phase 3: User Story 1 - Formal Determinism Guarantees (Priority: P1) 🎯 MVP

**Goal**: Document and prove deterministic ordering guarantees for production confidence

**Independent Test**: Documentation explicitly states ordering formula, tests prove merge order matches edge order across 100 runs

### Tests for User Story 1

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [X] T007 [P] [US1] Create determinism_test.go with test stub TestDeterministicMergeOrder in graph/determinism_test.go
- [X] T008 [P] [US1] Create test stub TestByteIdenticalReplays in graph/determinism_test.go
- [X] T009 [P] [US1] Create test stub TestOrderKeyCollisionResistance in graph/determinism_test.go

### Implementation for User Story 1

- [X] T010 [US1] Implement TestDeterministicMergeOrder: 3 parallel branches with random delays, verify merge order = edge order
- [X] T011 [US1] Implement TestByteIdenticalReplays: 100 sequential runs with same inputs, verify byte-identical final states
- [X] T012 [US1] Implement TestOrderKeyCollisionResistance: Generate 10,000 order keys, verify zero collisions

### Documentation for User Story 1

- [X] T013 [P] [US1] Create docs/determinism-guarantees.md documenting ordering formula: order_key = SHA256(parent_path || node_id || edge_index)
- [X] T014 [P] [US1] Document dispatch order (ascending order_key) in docs/determinism-guarantees.md
- [X] T015 [P] [US1] Document merge order (ascending order_key) in docs/determinism-guarantees.md
- [X] T016 [US1] Add determinism contract section to docs/concurrency.md referencing docs/determinism-guarantees.md
- [X] T017 [US1] Update README.md with link to determinism guarantees documentation

**Checkpoint**: At this point, User Story 1 should be fully functional - determinism is documented and proven

---

## Phase 4: User Story 2 - Exactly-Once Semantics Documentation (Priority: P1)

**Goal**: Document and prove atomic commit guarantees for production confidence

**Independent Test**: Documentation explains atomic step commit, tests prove zero duplicates across 1000 concurrent executions

### Tests for User Story 2

- [X] T018 [P] [US2] Create exactly_once_test.go with test stub TestAtomicStepCommit in graph/exactly_once_test.go
- [X] T019 [P] [US2] Create test stub TestIdempotencyEnforcement in graph/exactly_once_test.go
- [X] T020 [P] [US2] Create test stub TestNoDuplicatesUnderConcurrency in graph/exactly_once_test.go

### Implementation for User Story 2

- [X] T021 [US2] Implement TestAtomicStepCommit: Verify state+frontier+outbox+idempotency committed atomically or rolled back together
- [X] T022 [US2] Implement TestIdempotencyEnforcement: Attempt duplicate commit with same idempotency key, verify rejection
- [X] T023 [US2] Implement TestNoDuplicatesUnderConcurrency: 1000 concurrent workflow executions, verify zero duplicate step commits

### Documentation for User Story 2

- [X] T024 [P] [US2] Create docs/store-guarantees.md documenting atomic step commit contract (state+frontier+outbox+idempotency)
- [X] T025 [P] [US2] Document idempotency key formula in docs/store-guarantees.md
- [X] T026 [P] [US2] Document crash recovery semantics in docs/store-guarantees.md
- [X] T027 [US2] Add exactly-once semantics section to docs/replay.md referencing docs/store-guarantees.md
- [X] T028 [US2] Update README.md with link to store guarantees documentation

**Checkpoint**: At this point, User Stories 1 AND 2 should both work independently - guarantees documented and proven

---

## Phase 5: User Story 3 - Production-Ready Observability (Priority: P1)

**Goal**: Add comprehensive Prometheus metrics and enhanced OpenTelemetry attributes for production monitoring

**Independent Test**: All 6 metrics exposed and scrapable, spans include all documented attributes

### Tests for User Story 3

- [X] T029 [P] [US3] Create observability_test.go with test stub TestPrometheusMetricsExposed in graph/observability_test.go
- [X] T030 [P] [US3] Create test stub TestOpenTelemetryAttributes in graph/observability_test.go
- [X] T031 [P] [US3] Create test stub TestCostTrackingAccuracy in graph/observability_test.go

### Implementation for User Story 3 - Prometheus Metrics

- [X] T032 [P] [US3] Implement PrometheusMetrics struct with all 6 metrics in graph/metrics.go (inflight_nodes, queue_depth, step_latency_ms, retries_total, merge_conflicts_total, backpressure_events_total)
- [X] T033 [US3] Implement NewPrometheusMetrics constructor with metric registration in graph/metrics.go
- [X] T034 [US3] Implement RecordStepLatency method in graph/metrics.go
- [X] T035 [US3] Implement IncrementRetries method in graph/metrics.go
- [X] T036 [US3] Implement UpdateQueueDepth method in graph/metrics.go
- [X] T037 [US3] Implement UpdateInflightNodes method in graph/metrics.go
- [X] T038 [US3] Implement IncrementMergeConflicts method in graph/metrics.go
- [X] T039 [US3] Implement IncrementBackpressure method in graph/metrics.go

### Implementation for User Story 3 - Cost Tracking

- [X] T040 [P] [US3] Implement CostTracker struct with pricing map in graph/cost.go
- [X] T041 [US3] Implement RecordLLMCall method in graph/cost.go
- [X] T042 [US3] Implement GetTotalCost method in graph/cost.go
- [X] T043 [US3] Implement GetCostByModel method in graph/cost.go

### Implementation for User Story 3 - Integration

- [X] T044 [US3] Integrate PrometheusMetrics into Engine struct in graph/engine.go
- [X] T045 [US3] Integrate CostTracker into Engine struct in graph/engine.go
- [X] T046 [US3] Update Engine.runConcurrent to call metrics updates (UpdateInflightNodes, UpdateQueueDepth, RecordStepLatency) in graph/engine.go
- [X] T047 [US3] Update OTelEmitter to add cost attributes (tokens_in, tokens_out, cost_usd) to spans in graph/emit/otel.go
- [X] T048 [US3] Update OTelEmitter to add concurrency attributes (step_id, order_key, attempt) to spans in graph/emit/otel.go

### Tests Implementation for User Story 3

- [ ] T049 [US3] Implement TestPrometheusMetricsExposed: Start engine, verify all 6 metrics available at /metrics endpoint
- [ ] T050 [US3] Implement TestOpenTelemetryAttributes: Execute workflow, verify spans include run_id, step_id, order_key, attempt, tokens_in, tokens_out, cost_usd
- [ ] T051 [US3] Implement TestCostTrackingAccuracy: Run 100 LLM calls with known token counts, verify cost accuracy within $0.01

### Documentation for User Story 3

- [X] T052 [P] [US3] Enhance docs/observability.md with Prometheus metrics section listing all 6 metrics and their meanings
- [X] T053 [P] [US3] Enhance docs/observability.md with cost tracking section explaining token and cost attributes
- [X] T054 [US3] Create examples/prometheus_monitoring/main.go demonstrating metrics scraping with HTTP endpoint
- [X] T055 [US3] Add prometheus_monitoring example README.md with Grafana dashboard suggestions
- [X] T056 [US3] Update README.md with link to observability documentation and Prometheus example

**Checkpoint**: All P1 user stories complete - production observability fully functional

---

## Phase 6: User Story 4 - SQLite Store for Frictionless Development (Priority: P2)

**Goal**: Implement SQLite store for zero-configuration development workflows

**Independent Test**: SQLite store passes all Store contract tests, quickstart works with no database setup

### Tests for User Story 4

- [X] T057 [P] [US4] Create graph/store/sqlite_test.go with test stub TestSQLiteStoreSaveLoadStep
- [X] T058 [P] [US4] Create test stub TestSQLiteStoreCheckpointV2 in graph/store/sqlite_test.go
- [X] T059 [P] [US4] Create test stub TestSQLiteStoreIdempotency in graph/store/sqlite_test.go
- [X] T060 [P] [US4] Create test stub TestSQLiteStoreOutbox in graph/store/sqlite_test.go
- [X] T061 [P] [US4] Create test stub TestSQLiteStoreConcurrentReads in graph/store/sqlite_test.go

### Implementation for User Story 4

- [X] T062 [US4] Create SQLiteStore struct in graph/store/sqlite.go with db connection field
- [X] T063 [US4] Implement NewSQLiteStore constructor with database initialization and WAL mode in graph/store/sqlite.go
- [X] T064 [US4] Implement schema migration (create tables if not exist) in graph/store/sqlite.go
- [X] T065 [US4] Implement SaveStep method in graph/store/sqlite.go
- [X] T066 [US4] Implement LoadLatest method in graph/store/sqlite.go
- [X] T067 [US4] Implement SaveCheckpointV2 method with transaction in graph/store/sqlite.go
- [X] T068 [US4] Implement LoadCheckpointV2 method in graph/store/sqlite.go
- [X] T069 [US4] Implement CheckIdempotency method in graph/store/sqlite.go
- [X] T070 [US4] Implement PendingEvents method in graph/store/sqlite.go
- [X] T071 [US4] Implement MarkEventsEmitted method in graph/store/sqlite.go

### Tests Implementation for User Story 4

- [X] T072 [US4] Implement TestSQLiteStoreSaveLoadStep: Save step, verify LoadLatest returns correct data
- [X] T073 [US4] Implement TestSQLiteStoreCheckpointV2: Save checkpoint with frontier/RNG/IOs, verify LoadCheckpointV2 restores fully
- [X] T074 [US4] Implement TestSQLiteStoreIdempotency: Attempt duplicate commit, verify CheckIdempotency prevents it
- [X] T075 [US4] Implement TestSQLiteStoreOutbox: Save events, verify PendingEvents returns them, MarkEventsEmitted clears them
- [X] T076 [US4] Implement TestSQLiteStoreConcurrentReads: 10 concurrent readers, verify no errors or corruption

### Documentation for User Story 4

- [X] T077 [P] [US4] Add SQLite store section to docs/store-guarantees.md documenting single-process limitation and WAL mode
- [X] T078 [P] [US4] Create examples/sqlite_quickstart/main.go showing zero-config workflow with SQLite store
- [X] T079 [US4] Add sqlite_quickstart example README.md explaining use case (development/testing)
- [X] T080 [US4] Update docs/quickstart.md to recommend SQLite for development, MySQL for production
- [X] T081 [US4] Update README.md with SQLite store quick start example

**Checkpoint**: User Story 4 complete - SQLite enables frictionless development

---

## Phase 7: User Story 5 - Enhanced CI Test Coverage (Priority: P2)

**Goal**: Create comprehensive CI test suite proving determinism, ordering, exactly-once, and backpressure

**Independent Test**: All contract tests pass in CI, tests catch known failure modes

### Implementation for User Story 5

- [X] T082 [P] [US5] Create test TestReplayMismatchDetection in graph/determinism_test.go: Inject parameter drift, verify ErrReplayMismatch raised
- [X] T083 [P] [US5] Create test TestMergeOrderingWithRandomDelays in graph/determinism_test.go: 5 branches with random 0-100ms delays, verify merge order matches edge indices
- [X] T084 [P] [US5] Create test TestBackpressureBlocking in graph/scheduler_test.go: QueueDepth=1, enqueue 3 items, verify second blocks and backpressure event recorded
- [X] T085 [P] [US5] Create test TestRNGDeterminism in graph/replay_test.go: Same checkpoint seed produces identical random sequences across 10 replays
- [X] T086 [US5] Create test TestConcurrentStateUpdates in graph/exactly_once_test.go: 100 goroutines updating counter, verify final count = 100 (no lost updates)
- [X] T087 [US5] Create test TestIdempotencyAcrossStores in graph/store/common_test.go: Run idempotency test against MemStore, MySQLStore, SQLiteStore - all pass

### CI Integration for User Story 5

- [X] T088 [US5] Create .github/workflows/contract-tests.yml running determinism, exactly-once, observability test suites
- [X] T089 [US5] Add race detector flag to contract tests (-race) in CI workflow
- [X] T090 [US5] Add test coverage reporting to CI workflow (go test -cover)

### Documentation for User Story 5

- [X] T091 [P] [US5] Create docs/testing-contracts.md documenting all contract tests and what they prove
- [X] T092 [US5] Add testing section to README.md explaining how to run contract tests locally

**Checkpoint**: User Story 5 complete - comprehensive test coverage prevents regressions

---

## Phase 8: User Story 6 - API Ergonomics Improvements (Priority: P3)

**Goal**: Add functional options pattern, typed errors (already exported in Phase 2), and maintain backward compatibility

**Independent Test**: Both Options struct and functional options work, all exported errors usable with errors.Is

### Implementation for User Story 6

- [X] T093 [P] [US6] Create Option type (func(*Engine[S]) error) in graph/options.go
- [X] T094 [P] [US6] Implement WithMaxConcurrent(n int) Option in graph/options.go
- [X] T095 [P] [US6] Implement WithQueueDepth(n int) Option in graph/options.go
- [X] T096 [P] [US6] Implement WithBackpressureTimeout(d time.Duration) Option in graph/options.go
- [X] T097 [P] [US6] Implement WithDefaultNodeTimeout(d time.Duration) Option in graph/options.go
- [X] T098 [P] [US6] Implement WithRunWallClockBudget(d time.Duration) Option in graph/options.go
- [X] T099 [P] [US6] Implement WithReplayMode(enabled bool) Option in graph/options.go
- [X] T100 [P] [US6] Implement WithStrictReplay(enabled bool) Option in graph/options.go
- [X] T101 [P] [US6] Implement WithConflictPolicy(policy ConflictPolicy) Option in graph/options.go
- [X] T102 [US6] Update Engine.New to accept variadic ...Option and apply them in graph/engine.go
- [X] T103 [US6] Ensure backward compatibility: Engine.New still accepts Options struct in graph/engine.go

### Tests for User Story 6

- [X] T104 [US6] Create test TestFunctionalOptionsPattern in graph/options_test.go: Create engine with functional options, verify configuration applied
- [X] T105 [US6] Create test TestBackwardCompatibility in graph/options_test.go: Create engine with Options struct, verify still works
- [X] T106 [US6] Create test TestBothPatternsTogether in graph/options_test.go: Mix Options struct + functional options, verify both applied
- [X] T107 [US6] Create test TestTypedErrorHandling in graph/errors_test.go: Verify errors.Is works with all exported errors

### Documentation for User Story 6

- [X] T108 [P] [US6] Update docs/quickstart.md with functional options examples
- [X] T109 [P] [US6] Create docs/error-handling.md documenting all typed errors and when they occur
- [ ] T110 [US6] Update all examples to use functional options pattern (examples/concurrent_workflow, examples/replay_demo, examples/ai_research_assistant)
- [X] T111 [US6] Update README.md with functional options example in quick start section

**Checkpoint**: User Story 6 complete - API more ergonomic while maintaining backward compatibility

---

## Phase 9: User Story 7 - Documentation Completions (Priority: P3)

**Goal**: Complete documentation for conflict policies, human-in-the-loop, streaming, and competitive positioning

**Independent Test**: Users can find answers to all common questions within 2 minutes of searching docs

### Implementation for User Story 7

- [X] T112 [P] [US7] Create docs/conflict-policies.md documenting ConflictFail (default), LastWriterWins, and CRDT hooks (future)
- [X] T113 [P] [US7] Create docs/human-in-the-loop.md documenting pause/resume patterns with external input and approval workflows
- [X] T114 [P] [US7] Create docs/streaming.md documenting current streaming status (not yet supported) and workarounds (buffer in state)
- [X] T115 [P] [US7] Create docs/why-go.md with comparison table: Go vs Python LangGraph (type safety, performance, operational characteristics)
- [X] T116 [US7] Enhance docs/replay.md with guardrails section: hard fail on mismatch, divergent node identification
- [X] T117 [US7] Create docs/architecture.md with high-level system diagram showing Engine, Store, Emitter, Scheduler, Frontier relationships

### Documentation Organization for User Story 7

- [X] T118 [US7] Create docs/README.md as documentation index with links to all guides organized by topic
- [X] T119 [US7] Update main README.md with "Documentation" section linking to docs/README.md
- [X] T120 [US7] Add troubleshooting section to docs/README.md with common issues and solutions

### Examples for User Story 7

- [X] T121 [P] [US7] Create examples/human_in_the_loop/main.go demonstrating approval workflow with pause/resume
- [X] T122 [US7] Add human_in_the_loop example README.md explaining use cases and patterns

**Checkpoint**: User Story 7 complete - documentation comprehensive and easy to navigate

---

## Phase 10: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories and final validation

- [X] T123 [P] Run go fmt ./... to format all code
- [X] T124 [P] Run golangci-lint run ./... to check for linting issues (if available)
- [X] T125 [P] Run gosec ./... to check for security issues (if available)
- [X] T126 Run full test suite: go test ./... with race detector enabled
- [X] T127 Run benchmarks: go test -bench=. ./graph to verify no performance regressions
- [X] T128 [P] Update CHANGELOG.md with v0.2.1 or v0.3.0 release notes
- [X] T129 Update version in README.md badges
- [X] T130 Validate all quickstart.md examples still work with new features
- [X] T131 Run speckit.analyze to verify cross-artifact consistency
- [X] T132 Final code review and cleanup

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3-9)**: All depend on Foundational phase completion
  - P1 stories (US1-US3): Critical for production readiness - sequential recommended
  - P2 stories (US4-US5): Can proceed after P1 complete - parallel possible
  - P3 stories (US6-US7): Can proceed after P1 complete - parallel possible
- **Polish (Phase 10)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1 - Determinism)**: Independent - documents and tests existing behavior
- **User Story 2 (P1 - Exactly-Once)**: Independent - documents and tests existing behavior
- **User Story 3 (P1 - Observability)**: Independent - adds metrics and cost tracking
- **User Story 4 (P2 - SQLite Store)**: Independent - new store implementation
- **User Story 5 (P2 - Enhanced Tests)**: Independent - new test suites proving contracts
- **User Story 6 (P3 - API Ergonomics)**: Independent - adds functional options alongside existing API
- **User Story 7 (P3 - Documentation)**: Independent - documentation completions

### Within Each User Story

- Tests MUST be written and FAIL before implementation
- Implementation tasks follow natural dependencies (structs before methods, integration after components)
- Documentation can proceed in parallel with implementation within same story

### Parallel Opportunities

**Phase 1 (Setup)**:
- T001, T002, T003 can all run in parallel

**Phase 2 (Foundational)**:
- T004, T005, T006 can all run in parallel

**Phase 3 (US1)**:
- Tests: T007, T008, T009 parallel
- Documentation: T013, T014, T015 parallel (after tests)

**Phase 4 (US2)**:
- Tests: T018, T019, T020 parallel
- Documentation: T024, T025, T026 parallel (after tests)

**Phase 5 (US3)**:
- Metrics implementation: T032-T039 parallel (after struct created)
- Cost implementation: T040-T043 parallel
- Documentation: T052, T053 parallel

**Phase 6 (US4)**:
- Tests: T057-T061 parallel
- Store methods: T065-T071 can be partially parallel (different methods)
- Documentation: T077, T078, T079 parallel

**Phase 7 (US5)**:
- Tests: T082-T087 all parallel

**Phase 8 (US6)**:
- Options: T093-T101 all parallel
- Documentation: T108, T109, T110 parallel

**Phase 9 (US7)**:
- Documentation: T112-T117 all parallel
- Examples: T121, T122 parallel

**Phase 10 (Polish)**:
- T123, T124, T125, T128 parallel

**Multi-Story Parallelism**:
After Phase 2 completes, multiple user stories can proceed in parallel:
- Team A: US1 (Determinism docs & tests)
- Team B: US2 (Exactly-once docs & tests)
- Team C: US3 (Observability implementation)

---

## Parallel Example: User Story 3 (Observability)

```bash
# After metrics struct created (T032), launch all metric methods together:
Task: "Implement RecordStepLatency method in graph/metrics.go"
Task: "Implement IncrementRetries method in graph/metrics.go"
Task: "Implement UpdateQueueDepth method in graph/metrics.go"
Task: "Implement UpdateInflightNodes method in graph/metrics.go"
Task: "Implement IncrementMergeConflicts method in graph/metrics.go"
Task: "Implement IncrementBackpressure method in graph/metrics.go"

# Launch cost tracker in parallel:
Task: "Implement CostTracker struct in graph/cost.go"
Task: "Implement RecordLLMCall method in graph/cost.go"
Task: "Implement GetTotalCost method in graph/cost.go"
Task: "Implement GetCostByModel method in graph/cost.go"

# Launch documentation in parallel:
Task: "Enhance docs/observability.md with Prometheus metrics section"
Task: "Enhance docs/observability.md with cost tracking section"
```

---

## Implementation Strategy

### MVP First (User Stories 1-3 Only - All P1)

1. Complete Phase 1: Setup (dependencies)
2. Complete Phase 2: Foundational (error exports, stubs)
3. Complete Phase 3: User Story 1 (Determinism documentation & tests)
4. Complete Phase 4: User Story 2 (Exactly-once documentation & tests)
5. Complete Phase 5: User Story 3 (Observability metrics & cost tracking)
6. **STOP and VALIDATE**: All P1 documentation and observability complete
7. Tag as v0.2.1-alpha and deploy

**Estimated MVP**: ~45 tasks, ~2-3 hours with concurrent agents

### Full Feature (All 7 User Stories)

1. Complete Setup + Foundational → Foundation ready
2. Add US1-US3 (P1 stories) → Production hardening core complete
3. Add US4 (SQLite store) → Development experience improved
4. Add US5 (Enhanced tests) → Regression protection complete
5. Add US6 (API ergonomics) → Developer experience improved
6. Add US7 (Documentation) → Comprehensive documentation complete
7. Polish phase → Final validation
8. Tag as v0.3.0 and deploy

**Estimated Full Feature**: ~132 tasks, ~4-6 hours with concurrent agents

### Parallel Team Strategy

With multiple developers/agents:

1. **Foundation** (all together): Phase 1-2
2. **P1 Stories** (sequential for quality):
   - Developer A: US1 (Determinism)
   - Then Developer A: US2 (Exactly-Once)
   - Then Developers A+B+C: US3 (Observability - 3 sub-components)
3. **P2 Stories** (parallel):
   - Developer A: US4 (SQLite Store)
   - Developer B: US5 (Enhanced Tests)
4. **P3 Stories** (parallel):
   - Developer A: US6 (API Ergonomics)
   - Developer B: US7 (Documentation)
5. **Polish** (all together): Phase 10

---

## Notes

- [P] tasks = different files, no dependencies, can run in parallel
- [Story] label (US1-US7) maps task to specific user story for traceability
- Each user story should be independently completable and testable
- **No tests requested in spec**: Tests included because this is production hardening
- **TDD approach**: Tests written first, ensure they FAIL, then implement
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- All tasks maintain 100% backward compatibility with v0.2.0
- Expected total lines: ~3,500 (docs: ~1,500, SQLite: ~500, metrics: ~300, tests: ~800, options: ~400)

---

## Task Count Summary

- **Phase 1 (Setup)**: 3 tasks
- **Phase 2 (Foundational)**: 3 tasks
- **Phase 3 (US1 - Determinism)**: 11 tasks (3 test stubs + 3 tests + 5 docs)
- **Phase 4 (US2 - Exactly-Once)**: 11 tasks (3 test stubs + 3 tests + 5 docs)
- **Phase 5 (US3 - Observability)**: 28 tasks (3 test stubs + 12 metrics impl + 4 cost impl + 4 integration + 3 tests + 5 docs)
- **Phase 6 (US4 - SQLite Store)**: 25 tasks (5 test stubs + 10 store impl + 5 tests + 5 docs)
- **Phase 7 (US5 - Enhanced Tests)**: 11 tasks (6 tests + 3 CI + 2 docs)
- **Phase 8 (US6 - API Ergonomics)**: 19 tasks (9 options + 3 engine updates + 4 tests + 4 docs)
- **Phase 9 (US7 - Documentation)**: 11 tasks (6 docs + 3 org + 2 examples)
- **Phase 10 (Polish)**: 10 tasks
- **TOTAL**: 132 tasks

**Parallel Opportunities**: 50+ tasks can run in parallel across different phases, enabling significant time savings with concurrent agents.
