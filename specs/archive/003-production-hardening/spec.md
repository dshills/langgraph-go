# Feature Specification: Production Hardening and Documentation Enhancements

**Feature Branch**: `003-production-hardening`
**Created**: 2025-10-28
**Status**: Draft
**Input**: User description: "review ./specs/spec_additions.md and create a full specification for all recommended items"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Formal Determinism Guarantees (Priority: P1)

A production engineer deploying LangGraph-Go workflows needs explicit, contractual guarantees about deterministic behavior to ensure reproducible results for compliance and debugging purposes.

**Why this priority**: Production systems require formal guarantees, not implied behavior. Without explicit contracts, teams cannot confidently deploy to production or debug issues effectively. This is foundational for trust.

**Independent Test**: Can be fully tested by creating documentation that explicitly states the ordering function, creating tests that prove ordering under various conditions, and verifying that the contract holds across 1000 test runs.

**Acceptance Scenarios**:

1. **Given** the ordering function is documented as "order_key = SHA256(parent_path || node_id || edge_index)", **When** a developer reads the documentation, **Then** they understand exactly how work items are ordered
2. **Given** three parallel branches updating the same state fields, **When** the workflow executes with artificial delays, **Then** the merge order matches static edge indices regardless of completion timing
3. **Given** the same workflow executed 100 times with different timing, **When** comparing final states, **Then** all executions produce byte-identical results

---

### User Story 2 - Exactly-Once Semantics Documentation (Priority: P1)

An operations team deploying workflows to production needs clear documentation of exactly-once guarantees to prevent duplicate operations and ensure data consistency.

**Why this priority**: Exactly-once semantics are critical for financial transactions, inventory updates, and any operations where duplicates cause business problems. Teams need to understand what guarantees exist.

**Independent Test**: Can be tested by documenting the atomic step commit contract, creating tests that verify no duplicates under concurrent access, and validating that the documentation accurately describes behavior.

**Acceptance Scenarios**:

1. **Given** documentation explaining atomic step commits, **When** an engineer reviews it, **Then** they understand that state, frontier, outbox, and idempotency key are committed atomically
2. **Given** 1000 concurrent executions of the same workflow, **When** checking for duplicate step commits, **Then** zero duplicates are found (validated by idempotency keys)
3. **Given** a workflow interrupted mid-execution, **When** resumed from checkpoint, **Then** no operations are duplicated and execution continues cleanly

---

### User Story 3 - Production-Ready Observability (Priority: P1)

A DevOps engineer operating LangGraph-Go workflows in production needs comprehensive observability to monitor performance, detect issues, and track costs.

**Why this priority**: Production systems are invisible without proper observability. Teams need metrics, traces, and costs to operate effectively. This is essential for production readiness.

**Independent Test**: Can be tested by implementing OpenTelemetry integration with specified attributes, verifying metrics are exposed with documented names, and demonstrating cost tracking for LLM calls.

**Acceptance Scenarios**:

1. **Given** a workflow with OpenTelemetry configured, **When** execution completes, **Then** spans are created for run/step/node with attributes (run_id, step_id, node_id, attempt, order_key, tokens_in, tokens_out, latency_ms, cost_usd)
2. **Given** Prometheus scraping the application, **When** checking available metrics, **Then** all documented metrics are available (langgraph_inflight_nodes, langgraph_queue_depth, langgraph_step_latency_ms, langgraph_retries_total, etc.)
3. **Given** a workflow making 100 LLM calls, **When** checking the cost metrics, **Then** total cost in USD is accurately tracked and exposed

---

### User Story 4 - SQLite Store for Frictionless Development (Priority: P2)

A developer learning LangGraph-Go or prototyping workflows wants to get started quickly without setting up MySQL or other production databases.

**Why this priority**: Reduces friction for new users and enables faster iteration during development. While not critical for production (MySQL exists), this significantly improves developer experience and adoption.

**Independent Test**: Can be tested by creating an SQLite store implementation, verifying it passes the same store contract tests as MemStore and MySQLStore, and demonstrating a quickstart that works with zero configuration.

**Acceptance Scenarios**:

1. **Given** a new developer with no database setup, **When** they run a workflow with SQLite store, **Then** it works immediately with no configuration required
2. **Given** a workflow using SQLite store, **When** saving and loading checkpoints, **Then** all Store interface methods work correctly including idempotency checks and outbox
3. **Given** the SQLite store tests, **When** running the full test suite, **Then** 100% of store contract tests pass

---

### User Story 5 - Enhanced CI Test Coverage (Priority: P2)

A framework maintainer wants comprehensive CI tests that prove determinism, ordering, and exactly-once guarantees to prevent regressions as the codebase evolves.

**Why this priority**: Ensures long-term reliability and prevents regressions. These tests serve as executable contracts that validate the core guarantees. Important but not blocking for v0.2.0-alpha release.

**Independent Test**: Can be tested by creating test suites for deterministic replay, merge ordering, backpressure, and RNG determinism, then verifying they catch known failure modes.

**Acceptance Scenarios**:

1. **Given** a deterministic replay test, **When** injecting a parameter drift, **Then** ErrReplayMismatch is raised and the divergent node is identified
2. **Given** a merge ordering test with random delays, **When** comparing merge order to edge indices, **Then** merge order matches edge order 100% of the time
3. **Given** a backpressure test with QueueDepth=1, **When** enqueueing 2+ items, **Then** the second blocks and backpressure events are recorded

---

### User Story 6 - API Ergonomics Improvements (Priority: P3)

A developer building workflows wants a more ergonomic API with functional options, typed errors, and cost tracking to improve code quality and maintainability.

**Why this priority**: Quality-of-life improvements that make the framework more pleasant to use. Not critical for functionality but improves developer satisfaction and reduces boilerplate.

**Independent Test**: Can be tested by refactoring the API to use functional options, exporting typed errors, and adding cost accounting, then verifying all existing code still works with backward compatibility.

**Acceptance Scenarios**:

1. **Given** the functional options pattern, **When** creating an engine with options, **Then** code is more readable: `graph.New(reducer, store, WithMaxConcurrent(8), WithQueueDepth(1024))`
2. **Given** typed exported errors, **When** handling workflow errors, **Then** developers can use `errors.Is(err, graph.ErrReplayMismatch)` for control flow
3. **Given** LLM calls with cost tracking, **When** querying cost metrics, **Then** cost_usd attribute is available in spans and can be aggregated

---

### User Story 7 - Documentation Completions (Priority: P3)

A developer evaluating LangGraph-Go wants complete documentation covering conflict policies, human-in-the-loop patterns, streaming support, and competitive positioning to make informed adoption decisions.

**Why this priority**: Helps with adoption and reduces confusion, but not blocking for existing users. Can be added incrementally based on user questions.

**Independent Test**: Can be tested by creating documentation sections for each topic, validating accuracy against implementation, and confirming users can find answers to common questions.

**Acceptance Scenarios**:

1. **Given** documentation on conflict policies, **When** a developer reads it, **Then** they understand default behavior (ConflictFail) and alternatives (LastWriterWins, CRDT hooks)
2. **Given** documentation on human-in-the-loop, **When** implementing an approval workflow, **Then** developers understand how to pause/resume with external input
3. **Given** a "Why Go vs Python LangGraph" comparison table, **When** evaluating frameworks, **Then** developers can make informed decisions based on type safety, performance, and operational characteristics

---

### Edge Cases

- What happens when the ordering function produces collisions (two work items with same order_key)?
- How does the system handle a workflow where recorded I/O format has changed between record and replay?
- What happens when Prometheus scraping fails or OpenTelemetry export is blocked?
- How does SQLite store handle concurrent access from multiple processes?
- What happens when cost tracking is enabled but LLM provider doesn't return token counts?
- How does the system handle a user providing both old Options struct and new functional options?
- What happens when a developer catches ErrReplayMismatch but doesn't know which node diverged?

## Requirements *(mandatory)*

### Functional Requirements

**Determinism Guarantees (US1)**
- **FR-001**: Documentation MUST explicitly state the ordering function formula: "order_key = SHA256(parent_path || node_id || edge_index)"
- **FR-002**: Documentation MUST explain that work items within a step are started in ascending order_key order
- **FR-003**: Documentation MUST explain that deltas are merged in ascending order_key order
- **FR-004**: Tests MUST prove that merge order equals edge order regardless of completion timing
- **FR-005**: Tests MUST prove that 100 sequential executions with same inputs produce byte-identical states

**Exactly-Once Semantics (US2)**
- **FR-006**: Documentation MUST explain atomic step commit pattern (state + frontier + outbox + idempotency in one transaction)
- **FR-007**: Documentation MUST explain how idempotency keys prevent duplicate commits
- **FR-008**: Documentation MUST provide a "Store Guarantees" guide explaining transactional semantics
- **FR-009**: Tests MUST prove zero duplicate commits across 1000 concurrent executions

**Observability (US3)**
- **FR-010**: System MUST expose Prometheus metrics with documented names: langgraph_inflight_nodes, langgraph_queue_depth, langgraph_step_latency_ms, langgraph_retries_total, langgraph_merge_conflicts_total, langgraph_backpressure_events_total
- **FR-011**: System MUST create OpenTelemetry spans with attributes: run_id, step_id, node_id, attempt, order_key, tokens_in, tokens_out, latency_ms, cost_usd
- **FR-012**: Documentation MUST provide a tracing example showing real-world OpenTelemetry integration
- **FR-013**: Documentation MUST list all metric names and their meanings

**SQLite Store (US4)**
- **FR-014**: System MUST provide a SQLite store implementation passing all Store contract tests
- **FR-015**: SQLite store MUST support all V2 checkpoint APIs (SaveCheckpointV2, LoadCheckpointV2, CheckIdempotency)
- **FR-016**: SQLite store MUST support transactional outbox pattern (PendingEvents, MarkEventsEmitted)
- **FR-017**: SQLite store MUST handle concurrent read access safely
- **FR-018**: Documentation MUST show SQLite as the recommended store for development/testing

**Enhanced Testing (US5)**
- **FR-019**: Test suite MUST include deterministic replay test that detects parameter drift with ErrReplayMismatch
- **FR-020**: Test suite MUST include merge ordering test with random delays proving edge-order preservation
- **FR-021**: Test suite MUST include backpressure test verifying queue blocking and event counting
- **FR-022**: Test suite MUST include RNG determinism test proving checkpoint seed produces identical sequences
- **FR-023**: Test suite MUST be runnable in CI with clear pass/fail output

**API Improvements (US6)**
- **FR-024**: System MUST support functional options pattern: WithMaxConcurrent(n), WithQueueDepth(n), WithDefaultTimeout(d), WithConflictPolicy(p)
- **FR-025**: System MUST export typed errors: ErrNoProgress, ErrReplayMismatch, ErrBackpressure, ErrMaxStepsExceeded
- **FR-026**: System MUST track LLM token counts and costs when available from provider
- **FR-027**: System MUST maintain backward compatibility with existing Options struct approach

**Documentation Completions (US7)**
- **FR-028**: Documentation MUST explain default conflict policy (ConflictFail) and alternatives
- **FR-029**: Documentation MUST explain human-in-the-loop pause/resume patterns
- **FR-030**: Documentation MUST clarify LLM streaming support status
- **FR-031**: Documentation MUST provide comparison table: Go vs Python LangGraph
- **FR-032**: Documentation MUST explain replay guardrails (hard fail on mismatch, divergence detection)

### Key Entities

- **Ordering Function**: Mathematical formula defining deterministic work item ordering. Uses SHA256 hash of (parent_path, node_id, edge_index) to produce uint64 order_key. Critical for replay guarantees.

- **Atomic Step Commit**: Transactional unit persisting state, frontier, outbox, and idempotency key in a single database transaction. Ensures exactly-once semantics and crash recovery.

- **Idempotency Key**: SHA256 hash of (run_id, step_id, sorted_work_items, state_hash) preventing duplicate step commits. Enables safe retries and concurrent access.

- **OpenTelemetry Span**: Trace span representing run/step/node execution with attributes for debugging and performance analysis. Includes concurrency-specific attributes (order_key, attempt) and LLM-specific attributes (tokens, cost).

- **Prometheus Metric**: Named counter/gauge/histogram for monitoring system health. Includes queue depth, latency, retries, backpressure events, and merge conflicts.

- **Store Contract**: Interface guaranteeing transactional semantics, idempotency enforcement, and outbox pattern. Implemented by MemStore, MySQLStore, and SQLiteStore.

- **Conflict Policy**: Strategy for handling concurrent delta updates to the same state fields. Options: ConflictFail (default), LastWriterWins, CRDT hooks.

- **Replay Mismatch**: Condition where recorded I/O hash doesn't match current execution, indicating non-deterministic behavior. Results in ErrReplayMismatch with divergent node identification.

- **Cost Tracking**: Accumulation of LLM API costs (tokens * price_per_token) across workflow execution. Exposed via spans and metrics for budget monitoring.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Documentation explicitly states the ordering function formula and developers can find it within 1 minute of searching
- **SC-002**: 100% of store implementations pass the atomic commit contract tests
- **SC-003**: 100 sequential workflow executions produce byte-identical final states
- **SC-004**: Replay test detects 100% of injected parameter drifts within 1 test run
- **SC-005**: Merge ordering test with random delays produces correct edge-order merging 100% of the time
- **SC-006**: OpenTelemetry spans include all documented attributes (run_id, step_id, order_key, etc.) in 100% of executions
- **SC-007**: All 6 Prometheus metrics are exposed and scrapable within 5 seconds of workflow start
- **SC-008**: SQLite store enables workflow execution within 30 seconds of initial setup (no database configuration needed)
- **SC-009**: Developers can find conflict policy documentation within 2 minutes of searching
- **SC-010**: Functional options pattern reduces engine initialization code by 30% (fewer lines for common configurations)
- **SC-011**: Cost tracking reports total LLM costs within $0.01 accuracy for workflows with 100+ LLM calls
- **SC-012**: "Why Go vs Python" comparison table helps 80% of evaluators make informed decision (measured via user feedback)

## Assumptions

- Developers understand SHA256 hashing and why it provides deterministic ordering
- Production deployments have OpenTelemetry and Prometheus infrastructure available
- SQLite is acceptable for development (single-process access) but not for production (use MySQL/PostgreSQL)
- LLM providers expose token counts in API responses (OpenAI, Anthropic, Google all do)
- Functional options pattern is familiar to Go developers (widely used in stdlib and popular libraries)
- Documentation readers are technical (can understand transactional semantics)
- Cost calculations use standard pricing from provider documentation (prices may change)
- Conflict policy defaults are acceptable for most use cases (explicit conflicts preferred over silent overwrites)

## Scope

### In Scope
- Formal documentation of ordering function and determinism guarantees
- Documentation of atomic step commit and exactly-once semantics
- Store contract documentation with transactional guarantees
- OpenTelemetry emitter with production-ready attributes (run_id, step_id, tokens, cost)
- Prometheus metrics with standardized names (langgraph_* namespace)
- SQLite store implementation for development use
- Enhanced test suite proving determinism, ordering, and exactly-once
- Functional options pattern for Engine construction
- Exported typed errors for error handling
- Cost accounting for LLM API calls
- Conflict policy documentation
- Human-in-the-loop pattern documentation
- Streaming support status documentation
- Competitive positioning (Go vs Python LangGraph)

### Out of Scope
- Distributed tracing across multiple processes (future)
- Custom CRDT implementations (already noted as future work)
- Automatic cost budgeting or alerts (users implement via metrics)
- Built-in Prometheus server (users add to their infrastructure)
- GUI for visualizing traces (use existing OpenTelemetry tools)
- Multi-database support beyond SQLite/MySQL (PostgreSQL future work)
- Breaking changes to existing APIs (maintain backward compatibility)
- Python interop or polyglot support

## Dependencies

- SQLite library (modernc.org/sqlite or mattn/go-sqlite3) for SQLite store
- OpenTelemetry SDK (already added in v0.2.0 for OTelEmitter)
- Prometheus client library (github.com/prometheus/client_golang) for metrics exposure
- LLM provider SDKs already support token counts (OpenAI, Anthropic, Google)
- Documentation tools (markdown, godoc)
- CI infrastructure for running enhanced test suite

## Open Questions

- Should SQLite store use WAL mode for better concurrent read performance?
- Should cost tracking support custom pricing models (e.g., volume discounts)?
- Should we provide a cost budgeting middleware that stops execution at threshold?
- Should human-in-the-loop support webhook callbacks or polling-based resume?
- Should streaming support buffer partial LLM responses in state or use a separate streaming API?
- Should we provide Grafana dashboard templates for the Prometheus metrics?
- Should the "Why Go" comparison include performance benchmarks or just feature comparison?
