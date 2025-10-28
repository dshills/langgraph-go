# Feature Specification: Concurrent Graph Execution with Deterministic Replay

**Feature Branch**: `002-concurrency-spec`
**Created**: 2025-10-28
**Status**: Draft
**Input**: User description: "review and write specifications for ./specs/langgraph-go_concurrency_spec.md"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Parallel Node Execution for Performance (Priority: P1)

A developer building an LLM workflow wants to execute multiple independent nodes concurrently (e.g., parallel API calls to different services) to reduce total execution time while maintaining predictable results.

**Why this priority**: Core value proposition of concurrent execution - enables performance optimization without sacrificing correctness. This is the foundation for all other concurrency features.

**Independent Test**: Can be fully tested by creating a graph with 3+ independent nodes, executing them, and verifying that total execution time is significantly less than sequential execution while state merging produces consistent results.

**Acceptance Scenarios**:

1. **Given** a graph with 3 independent nodes that each take 1 second, **When** execution starts with concurrency enabled, **Then** total execution completes in approximately 1 second (not 3 seconds) and all node outputs are correctly merged into final state
2. **Given** a graph with fan-out routing (one node spawning 5 parallel branches), **When** execution reaches the fan-out point, **Then** all 5 branches execute concurrently and merge deterministically at the join point
3. **Given** concurrent nodes producing state deltas, **When** all nodes complete, **Then** the final state is identical regardless of which node finished first

---

### User Story 2 - Deterministic Replay from Checkpoints (Priority: P1)

A developer needs to replay a previous graph execution exactly (for debugging, auditing, or resumption) without re-executing expensive external calls while getting identical state transitions and routing decisions.

**Why this priority**: Critical for production reliability - enables debugging of failures, resumption after crashes, and compliance auditing. Without this, concurrent execution becomes unpredictable and unreliable.

**Independent Test**: Can be tested by executing a graph with recorded I/O, capturing the checkpoint, then replaying from that checkpoint and verifying that state deltas, routing decisions, and final state match exactly without invoking external services.

**Acceptance Scenarios**:

1. **Given** a completed graph execution with checkpoints saved, **When** replaying from any checkpoint, **Then** all subsequent state deltas and routing decisions match the original run exactly
2. **Given** a graph execution that called external APIs with recorded responses, **When** replaying from checkpoint, **Then** recorded API responses are reused without making new external calls
3. **Given** a graph using random number generation with a seeded RNG, **When** replaying from checkpoint, **Then** all random values match the original execution
4. **Given** a replay where recorded I/O hash doesn't match current node output, **When** mismatch is detected, **Then** system raises `ErrReplayMismatch` error

---

### User Story 3 - Bounded Concurrency with Backpressure (Priority: P2)

A developer running graphs in production wants to control resource usage by limiting concurrent node execution and handling scenarios where nodes are queued faster than they can be processed.

**Why this priority**: Essential for production stability - prevents resource exhaustion and enables graceful degradation under load. Lower priority than core concurrency because it's about operational safety rather than core functionality.

**Independent Test**: Can be tested by creating a graph that spawns 100 nodes with a concurrency limit of 5, then verifying that at most 5 nodes execute simultaneously and the system handles queue overflow gracefully.

**Acceptance Scenarios**:

1. **Given** a graph configured with `MaxConcurrentNodes=5`, **When** 20 nodes become ready simultaneously, **Then** only 5 execute concurrently while others queue in deterministic order
2. **Given** a queue at capacity with new nodes ready to execute, **When** attempting to enqueue more work, **Then** system blocks admission until capacity frees or context cancels
3. **Given** a blocked queue for longer than `BackpressureTimeout`, **When** timeout is reached, **Then** system saves checkpoint and pauses execution

---

### User Story 4 - Controlled Cancellation and Timeouts (Priority: P2)

A developer needs to cancel long-running graph executions or enforce time limits on individual nodes to prevent runaway processes and ensure responsive systems.

**Why this priority**: Important for production operations but not core functionality. Users can work around this with manual cancellation for MVP.

**Independent Test**: Can be tested by starting a graph execution with timeout policies, triggering cancellation via context, and verifying that all running nodes receive cancellation signals and cleanup properly.

**Acceptance Scenarios**:

1. **Given** a graph execution with `RunWallClockBudget=30s`, **When** execution exceeds 30 seconds, **Then** all running nodes receive context cancellation and execution terminates gracefully
2. **Given** a node with `Timeout=5s`, **When** node execution exceeds 5 seconds, **Then** only that node is cancelled and error handling is triggered
3. **Given** a running graph, **When** user explicitly cancels via context, **Then** all running nodes receive cancellation signal within 1 second and halt execution
4. **Given** a node that detects topology deadlock (no progress possible), **When** deadlock is detected, **Then** system raises `ErrNoProgress` and terminates

---

### User Story 5 - Retry Policies for Transient Failures (Priority: P3)

A developer wants to configure automatic retries for nodes that fail due to transient issues (network timeouts, rate limits) without having to implement retry logic in every node.

**Why this priority**: Quality-of-life feature that improves robustness but not essential for core functionality. Can be added after MVP.

**Independent Test**: Can be tested by creating a node that fails twice with retryable errors then succeeds, configuring retry policy with exponential backoff, and verifying the node is retried with proper delay intervals.

**Acceptance Scenarios**:

1. **Given** a node configured with `MaxAttempts=3` and exponential backoff, **When** node fails with retryable error, **Then** node is retried up to 3 times with increasing delays between attempts
2. **Given** a node that fails with non-retryable error, **When** failure occurs, **Then** no retry is attempted and error is propagated immediately
3. **Given** a node with idempotency key configured, **When** node is retried, **Then** idempotency key ensures duplicate executions are deduplicated

---

### Edge Cases

- What happens when two concurrent nodes produce conflicting state deltas that the reducer cannot merge?
- How does the system handle a node that never respects `ctx.Done()` and runs indefinitely?
- What happens when a checkpoint save operation fails during step commit?
- How does replay handle schema evolution when state structure has changed between original run and replay?
- What happens when the frontier queue is full and backpressure timeout is reached but checkpoint save fails?
- How does the system handle a reducer function that is non-deterministic or has side effects?
- What happens when recorded I/O during replay is corrupted or missing?
- How does cancellation propagate in deeply nested fan-out branches?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST execute independent nodes concurrently up to configured `MaxConcurrentNodes` limit
- **FR-002**: System MUST process nodes in deterministic order based on `(step_id, order_key)` tuple within same step
- **FR-003**: System MUST support fan-out routing where one node spawns multiple concurrent branches
- **FR-004**: System MUST merge state deltas from concurrent nodes using pure, deterministic reducer functions
- **FR-005**: System MUST maintain merge order based on ascending `order_key` of producing nodes
- **FR-006**: System MUST save checkpoints containing `{run_id, step_id, state, frontier, rng_seed, recorded_io}`
- **FR-007**: System MUST replay executions deterministically by reusing recorded I/O and RNG seed from checkpoints
- **FR-008**: System MUST detect replay mismatches and raise `ErrReplayMismatch` when recorded vs actual output differs
- **FR-009**: System MUST enforce `MaxConcurrentNodes` to limit simultaneous node execution
- **FR-010**: System MUST queue nodes in deterministic order when concurrency limit is reached
- **FR-011**: System MUST implement backpressure by blocking admission when frontier queue reaches `QueueDepth` capacity
- **FR-012**: System MUST checkpoint and pause execution when backpressure blocks longer than `BackpressureTimeout`
- **FR-013**: System MUST propagate context cancellation to all running nodes within 1 second
- **FR-014**: System MUST enforce per-node timeouts via `NodePolicy.Timeout` configuration
- **FR-015**: System MUST enforce run-level wall clock budget via `RunWallClockBudget` configuration
- **FR-016**: System MUST detect topology deadlocks (no progress possible) and raise `ErrNoProgress`
- **FR-017**: System MUST support retry policies with configurable `MaxAttempts`, backoff function, and retryable error predicate
- **FR-018**: System MUST commit state, frontier, and event outbox atomically to store
- **FR-019**: System MUST use idempotency keys (hash of work items and state) to prevent duplicate step commits
- **FR-020**: System MUST provide per-run seeded PRNG that produces stable values across replays
- **FR-021**: System MUST record external I/O (requests, responses, hashes) for replay purposes
- **FR-022**: System MUST propagate `{run_id, step_id, node_id, order_key, attempt}` context values to nodes
- **FR-023**: System MUST enforce reducer function purity (no side effects, deterministic output)
- **FR-024**: System MUST create copy-on-write state snapshots for fan-out branches
- **FR-025**: System MUST support configurable conflict policies (`ConflictFail`, `ConflictLastWriterWins`, `ConflictCRDT`)
- **FR-026**: System MUST emit OpenTelemetry spans for runs, steps, and node executions
- **FR-027**: System MUST expose metrics for `queue_depth`, `step_latency_ms`, `active_nodes`, and `step_count`
- **FR-028**: System MUST preserve token stream order within individual node outputs
- **FR-029**: System MUST drain event outbox asynchronously after transactional commit
- **FR-030**: System MUST validate that node `Run` methods honor `ctx.Done()` signals

### Key Entities

- **Run**: A single execution instance of a graph, uniquely identified by `run_id`. Contains execution state, step history, checkpoints, and configuration.
- **Step**: A scheduler tick that consumes a frontier of runnable nodes and produces a new frontier. Identified by `step_id` within a run. Represents one wave of concurrent node execution.
- **Frontier**: Set of work items `(node_id, state_snapshot, metadata)` that are ready to execute in the next step. Determines which nodes run concurrently.
- **Work Item**: Tuple `(step_id, order_key, node_id, state_ref, inputs, budget)` representing one schedulable unit. `order_key` is `(path_hash, edge_index)` for deterministic ordering.
- **Checkpoint**: Durable, transactional snapshot containing `{run_id, step_id, state, frontier, rng_seed, recorded_io}`. Enables deterministic replay and resumption.
- **State Delta**: Partial state update returned by a node. Merged into accumulated state via reducer function.
- **Reducer**: Pure function `func(prev S, delta S) (S, error)` that merges state deltas deterministically. Must have no side effects.
- **Node Policy**: Configuration for node behavior including `Timeout`, `RetryPolicy`, and `IdempotencyKeyFunc`.
- **Retry Policy**: Configuration for retry behavior including `MaxAttempts`, `Backoff` function, and `Retryable` error predicate.
- **Routing Command**: Node output specifying next execution target(s): `Goto(nodeID)`, `Stop()`, or `Fork([...])` for fan-out.
- **Side Effect Policy**: Declaration of node's external I/O characteristics: `Recordable` (can be captured) and `RequiresIdempotency` flags.
- **Event Outbox**: Transactional log of observability events that is drained asynchronously after checkpoint commit.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Graphs with 5 independent nodes complete execution in 20% of sequential time (demonstrating parallelism)
- **SC-002**: Replayed executions produce identical state deltas and routing decisions 100% of the time without external I/O
- **SC-003**: System enforces concurrency limits such that active node count never exceeds `MaxConcurrentNodes`
- **SC-004**: Context cancellation reaches all running nodes within 1 second of cancellation signal
- **SC-005**: Deterministic ordering guarantees that 100 sequential replays of the same graph produce identical final states
- **SC-006**: Checkpoint atomicity ensures zero duplicate step commits across 1000 concurrent executions
- **SC-007**: Backpressure mechanism prevents queue overflow 100% of the time under sustained overload
- **SC-008**: Reducer purity validation catches 100% of non-deterministic reducers during testing
- **SC-009**: Retry policies successfully recover from 90% of transient failures without developer intervention
- **SC-010**: Observability spans capture 100% of run/step/node execution events with correct parent-child relationships
- **SC-011**: System detects topology deadlocks within 5 seconds of no-progress condition occurring
- **SC-012**: Idempotency keys prevent 100% of duplicate state applications during failure recovery

## Assumptions

- Reducer functions provided by developers are pure and deterministic (validated via testing framework)
- Node implementations respect `ctx.Done()` cancellation signals (documented as contract requirement)
- External I/O is bounded in size for recording purposes (configurable size limits per interaction)
- Store implementation provides transactional atomicity for checkpoint commits
- Network partitions or store failures are handled by retry/timeout mechanisms in store layer
- RNG seed space is sufficient to avoid collisions across runs (64-bit seed provides adequate entropy)
- Order keys use collision-resistant hashing (SHA-256) to ensure deterministic ordering
- Event outbox draining is eventually consistent (tolerable delay between commit and event emission)
- Schema evolution is handled by developer migration logic (not automatic schema versioning)

## Scope

### In Scope
- Concurrent execution of independent nodes within configured limits
- Deterministic state merging via pure reducer functions
- Checkpoint-based deterministic replay with recorded I/O
- Backpressure and queue management
- Cancellation and timeout enforcement
- Retry policies with exponential backoff
- Atomic step commits with idempotency
- OpenTelemetry observability integration
- Conflict detection and configurable resolution policies

### Out of Scope
- Built-in CRDT implementations (noted as open question for future)
- Distributed execution across multiple processes/machines
- Automatic schema migration for state evolution
- Human-in-the-loop pause/resume UI (API only in scope)
- Dynamic graph topology changes during execution
- Automatic performance tuning of concurrency parameters
- Cross-run state sharing or caching
- Built-in rate limiting for external API calls (handle in node layer)

## Dependencies

- Store implementation must support transactional atomicity for multi-entity commits (state + frontier + outbox)
- Go context package for cancellation propagation
- OpenTelemetry SDK for span creation and metric collection
- Cryptographic hash function (SHA-256) for order key and idempotency key generation
- Concurrent-safe data structures for frontier queue management
- Time package for timeout and backoff calculations

## Open Questions

- Should the framework provide built-in CRDT types for common merge patterns (counters, sets, maps)?
- How should the transactional outbox be owned - by the engine or delegated to store implementation?
- What is the API design for async human-in-the-loop pause/resume with external approval workflows?
- Should schema evolution support be provided via automatic migration or developer-driven conversion?
- What are the observability requirements for distributed tracing across process boundaries (future)?
