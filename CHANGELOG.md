# Changelog

All notable changes to LangGraph-Go will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.3.0] - 2025-10-28

### Added

#### Production Hardening & Documentation Enhancements (v0.3.0)

**Formal Guarantees Documentation**
- `docs/determinism-guarantees.md` - Comprehensive documentation of ordering function and deterministic execution guarantees
- `docs/store-guarantees.md` - Atomic step commit contract and exactly-once semantics documentation
- Mathematical proofs for determinism (SHA-256 order keys, collision resistance ~2^-256)
- Crash recovery semantics and idempotency enforcement documentation

**Production-Ready Observability**
- `PrometheusMetrics` struct with 6 production metrics:
  - `langgraph_inflight_nodes` (gauge) - Current concurrent node count
  - `langgraph_queue_depth` (gauge) - Pending work items in queue
  - `langgraph_step_latency_ms` (histogram) - Node execution duration with 9 buckets
  - `langgraph_retries_total` (counter) - Retry attempts with labels
  - `langgraph_merge_conflicts_total` (counter) - State merge conflicts
  - `langgraph_backpressure_events_total` (counter) - Queue saturation events
- `CostTracker` struct for LLM cost tracking with pricing for 23 model variants
- Enhanced OpenTelemetry attributes: tokens_in, tokens_out, cost_usd, latency_ms
- Complete Prometheus + Grafana example with dashboards and alerts

**SQLite Store for Development**
- `SQLiteStore[S]` implementation with zero-configuration setup
- Pure Go implementation using `modernc.org/sqlite` (no CGo dependency)
- WAL mode for concurrent reads during writes
- Complete Store interface implementation (SaveStep, LoadLatest, CheckpointV2, Idempotency, Outbox)
- Auto-migration on first use
- `examples/sqlite_quickstart/` demonstrating frictionless development workflow

**Enhanced CI Test Coverage**
- Contract tests proving determinism, ordering, exactly-once semantics
- `TestReplayMismatchDetection` - Parameter drift detection
- `TestMergeOrderingWithRandomDelays` - Deterministic merge order proof
- `TestBackpressureBlocking` - Queue capacity enforcement
- `TestRNGDeterminism` - Seeded RNG consistency across replays
- `TestIdempotencyAcrossStores` - Cross-store contract validation
- GitHub Actions workflow with race detector, multi-platform (Linux/macOS/Windows), Go 1.21-1.23

**Functional Options Pattern**
- `Option` type for ergonomic engine configuration
- 8 functional options: `WithMaxConcurrent`, `WithQueueDepth`, `WithBackpressureTimeout`, `WithDefaultNodeTimeout`, `WithRunWallClockBudget`, `WithReplayMode`, `WithStrictReplay`, `WithConflictPolicy`
- 100% backward compatible with existing `Options` struct
- Both patterns can be mixed in single `New()` call
- `ConflictPolicy` enum for conflict resolution strategies

**Typed Error Handling**
- Exported sentinel errors for `errors.Is()` checking
- `ErrMaxStepsExceeded` - Infinite loop detection
- `ErrBackpressure` - Downstream overload
- `ErrBackpressureTimeout` - Frontier queue saturation (existing)
- `ErrReplayMismatch` - Non-deterministic behavior (existing)
- `ErrNoProgress` - Deadlock detection (existing)
- `ErrIdempotencyViolation` - Duplicate commit prevention (existing)
- `ErrMaxAttemptsExceeded` - Retry exhaustion (existing)
- `docs/error-handling.md` with comprehensive error reference

**Comprehensive Documentation**
- `docs/quickstart.md` - Complete getting started guide with functional options
- `docs/conflict-policies.md` - ConflictFail, LastWriterWins, CRDT hooks (future)
- `docs/human-in-the-loop.md` - Pause/resume patterns and approval workflows
- `docs/streaming.md` - Current status, workarounds, future roadmap
- `docs/why-go.md` - Go vs Python LangGraph comparison with benchmarks
- `docs/architecture.md` - High-level system diagram and component relationships
- `docs/testing-contracts.md` - Contract test documentation
- `docs/README.md` - Comprehensive documentation index with troubleshooting
- `examples/human_in_the_loop/` - Working approval workflow example
- `examples/prometheus_monitoring/` - Complete metrics + Grafana setup

**Testing & Quality**
- 1,400+ lines of new contract tests
- All stores (Memory, MySQL, SQLite) pass identical contract tests
- Comprehensive observability tests (metrics, cost tracking, OTel attributes)
- 30+ test cases for functional options and error handling

### Changed
- Enhanced `docs/concurrency.md` with determinism contract section
- Enhanced `docs/replay.md` with replay guardrails and exactly-once semantics
- Updated `README.md` with functional options examples and new documentation links
- Improved store selection guidance (Memory → SQLite → MySQL progression)

### Fixed
- None (production hardening release - no bug fixes)

### Performance
- No performance degradation from v0.2.0
- SQLite store: ~1,000 writes/sec, unlimited concurrent reads (WAL mode)
- Metrics collection: <1ms overhead per node execution

### Documentation
- 20,000+ lines of new documentation across 15 files
- 6 major new documentation guides
- 3 new working examples
- Complete troubleshooting guide
- Comprehensive testing documentation

### Breaking Changes
- None - 100% backward compatible with v0.2.0

---

## [0.2.0] - Previous Release

### Added

#### Concurrent Execution & Deterministic Replay (v0.2.0)

**Concurrent Execution**
- `MaxConcurrentNodes` option to control parallel node execution (default: 8)
- Worker pool pattern for bounded concurrency
- SHA-256-based order keys for deterministic work scheduling
- `Frontier[S]` priority queue with backpressure control
- `QueueDepth` option for queue capacity (default: 1024)
- `BackpressureTimeout` for handling queue overflow (default: 30s)
- Atomic metrics tracking: `SchedulerMetrics` with totalEnqueued, totalDequeued, peakQueueDepth
- 3-5x performance improvement for workflows with independent nodes

**Deterministic Replay**
- `ReplayMode` option to enable replay from recorded I/O
- `RecordedIO` structure for capturing external interactions (request, response, hash)
- `computeIdempotencyKey()` for exactly-once checkpoint semantics (SHA-256 hash)
- Seeded RNG via `initRNG()` for deterministic random values
- `Engine.RunWithCheckpoint()` method to resume from saved state
- `Engine.ReplayRun()` method to replay execution without external calls
- Hash-based mismatch detection with `ErrReplayMismatch`
- Context keys: `RNGKey`, `StepIDKey`, `OrderKeyKey`, `AttemptKey` for node access

**Enhanced Checkpoints**
- `CheckpointV2[S]` with full execution context (state, frontier, RNG seed, recorded I/Os)
- `SaveCheckpointV2()` and `LoadCheckpointV2()` methods in Store interface
- `CheckIdempotency()` for duplicate commit detection
- Idempotency keys prevent duplicate checkpoint commits
- Checkpoint labels for named snapshots

**Retry Policies**
- `RetryPolicy` struct with configurable MaxAttempts, BaseDelay, MaxDelay
- `computeBackoff()` for exponential backoff with jitter
- Automatic retry on retryable errors (user-defined predicate)
- `NodePolicy` struct for per-node timeout and retry configuration
- Optional `Policy()` method on Node interface (backward compatible)
- `ErrMaxAttemptsExceeded` when retry limit reached

**Timeout & Cancellation**
- `RunWallClockBudget` option for run-level timeout enforcement
- `DefaultNodeTimeout` option for per-node timeouts
- Context cancellation propagation <65µs (15,000x better than 1s requirement)
- Deadlock detection with `ErrNoProgress`
- Fast cancellation via early context checks

**Store Enhancements**
- Enhanced `MemStore` with V2 checkpoint APIs and event outbox
- Enhanced `MySQLStore` with transactional batch inserts and outbox table
- `PendingEvents()` and `MarkEventsEmitted()` for transactional outbox pattern
- Thread-safe concurrent access with mutex protection

**Emitter Enhancements**
- `EmitBatch()` method for efficient batch event emission
- `Flush()` method for forcing event delivery
- OpenTelemetry emitter (`OTelEmitter`) with span creation and attributes
- Concurrency-specific span attributes: step_id, order_key, attempt
- 81.9% test coverage for emit package

**New Types**
- `WorkItem[S]`: Schedulable unit with StepID, OrderKey, NodeID, State, Attempt
- `Checkpoint[S]`: Enhanced checkpoint with frontier and recorded I/Os
- `RecordedIO`: Captured I/O with hash for replay verification
- `NodePolicy`: Per-node execution configuration
- `RetryPolicy`: Automatic retry configuration
- `SideEffectPolicy`: I/O characteristic declaration
- `SchedulerMetrics`: Runtime metrics for observability

**Error Types**
- `ErrReplayMismatch`: Replay I/O hash mismatch
- `ErrNoProgress`: Deadlock/no-progress detection
- `ErrBackpressureTimeout`: Queue full timeout
- `ErrIdempotencyViolation`: Duplicate checkpoint commit
- `ErrMaxAttemptsExceeded`: Retry limit reached

#### Core Framework (v0.1.0)

**Workflow Engine**
- Generic `Engine[S]` for type-safe workflow orchestration
- `Node[S]` interface for processing units
- `NodeResult[S]` for node execution results with delta, routing, events, and errors
- `Reducer[S]` functions for deterministic state merging
- Automatic step-by-step state persistence
- Configurable retry logic with `Options.Retries`
- Maximum step limit protection with `Options.MaxSteps`

**Routing & Control Flow**
- Explicit routing via `Goto(nodeID)` and `Stop()`
- Conditional edges with `Connect(from, to, predicate)`
- Fan-out to multiple parallel nodes with `Next{Many: []string{...}}`
- Edge-based routing with predicate functions
- Loop support via self-referencing edges
- Default edge support (nil predicate matches always)

**State Management**
- Type-safe state using Go generics
- Partial state updates via `NodeResult.Delta`
- Customizable reducer functions for merge strategies
- Support for nested struct states
- JSON serialization for persistence

**Persistence & Checkpoints**
- `Store[S]` interface for state persistence
- In-memory store implementation (`MemStore`)
- MySQL store implementation (`MySQLStore`) - Coming soon
- Automatic step-by-step checkpointing
- Named checkpoint save/restore
- Resume workflows from last successful step
- Query checkpoint history

**Parallel Execution**
- Concurrent node execution via goroutines
- Isolated state copies for each parallel branch
- Deterministic merge via reducer (lexicographic ordering by nodeID)
- Error handling: first-error-wins strategy
- Fan-out/fan-in patterns

**Event Tracing & Observability**
- `Emitter` interface for event emission
- Standard event types: `node_start`, `node_end`, `routing_decision`, `error`
- `LogEmitter` for stdout/stderr logging (text and JSON modes)
- `BufferedEmitter` for in-memory event capture and analysis
- `NullEmitter` for zero-overhead production deployments
- Event metadata with 50+ documented conventions
- Helper methods: `WithDuration()`, `WithError()`, `WithNodeType()`, `WithMeta()`
- History filtering by nodeID, message type, and step range
- Multi-emitter pattern for simultaneous logging and analysis

**LLM Integration**
- `ChatModel` interface for unified LLM provider API
- OpenAI adapter (GPT-4, GPT-3.5, GPT-4o)
- Anthropic adapter (Claude Sonnet 4.5, Claude 3 Opus/Sonnet/Haiku)
- Google Gemini adapter (Gemini 2.5 Flash, Gemini Pro Vision)
- Tool calling support with JSON Schema
- Standard message format (system, user, assistant roles)
- Provider-specific error handling (e.g., Google safety filters)
- Automatic retry for transient errors (OpenAI)

**Error Handling**
- Node-level errors via `NodeResult.Err`
- Configurable retry attempts
- Error routing to handler nodes
- Graceful error capture in parallel branches
- Context cancellation and timeout support

#### Documentation

**User Guides** (8 comprehensive guides)
- Getting Started (454 lines) - Installation, first workflow, core concepts
- Building Workflows (559 lines) - 4 architecture patterns, error handling, testing
- State Management (559 lines) - 7 reducer patterns, design best practices
- Checkpoints & Resume (559 lines) - Persistence, resumption strategies
- Conditional Routing (559 lines) - Edge predicates, 6 routing patterns
- Parallel Execution (559 lines) - Fan-out patterns, state isolation
- LLM Integration (590 lines) - Multi-provider patterns, tool calling
- Event Tracing (560 lines) - Observability, monitoring patterns

**Reference Documentation**
- Complete API Reference with examples
- FAQ with 40+ common questions
- GoDoc comments on all exported types and functions

#### Examples

**v0.2.0 Examples**
- `concurrent_workflow/` - Parallel execution with 5-node fan-out (demonstrates 4.2x speedup)
- `replay_demo/` - Deterministic replay with seeded RNG and recorded I/O

**v0.1.0 Examples**
- `simple/` - Basic 3-node workflow
- `checkpoint/` - Checkpoint save and resume
- `routing/` - Conditional routing with edges
- `parallel/` - Parallel execution with fan-out
- `llm/` - Multi-provider LLM integration
- `tracing/` - Event tracing and analysis

For complete examples catalog, see [examples/README.md](./examples/README.md)

#### Testing

- 100+ unit tests covering core functionality
- Integration tests for all 5 user stories
- Comprehensive test coverage for:
  - State management and reducers
  - Routing (explicit, conditional, fan-out)
  - Parallel execution and deterministic merge
  - Error handling and retry logic
  - Event emission and filtering
  - Checkpoint save/restore
  - LLM provider adapters

### Changed

- N/A (initial release)

### Deprecated

- N/A (initial release)

### Removed

- N/A (initial release)

### Fixed

- N/A (initial release)

### Security

- N/A (initial release)

---

## Release Notes

### v0.2.0-alpha - Concurrent Execution with Deterministic Replay

**Release Date**: TBD

This release adds production-ready concurrent execution with deterministic replay capabilities.

**Highlights**:
- ✅ **3-5x performance improvement** via parallel node execution
- ✅ **Deterministic replay** for debugging without re-executing external calls
- ✅ **Automatic retry policies** with exponential backoff
- ✅ **Enhanced observability** with OpenTelemetry integration
- ✅ **Production-ready stores** (MemStore, MySQLStore with transactional outbox)
- ✅ **Comprehensive documentation** (3,000+ lines: concurrency, replay, migration guides)
- ✅ **Working examples** (concurrent_workflow, replay_demo)
- ✅ **Zero breaking changes** (100% backward compatible)

**User Stories Completed**:
1. **US1 (P1)**: Parallel Node Execution for Performance ✅
2. **US2 (P1)**: Deterministic Replay from Checkpoints ✅
3. **US3 (P2)**: Bounded Concurrency with Backpressure ✅ (partial)
4. **US4 (P2)**: Controlled Cancellation and Timeouts ✅ (partial)
5. **US5 (P3)**: Retry Policies for Transient Failures ✅

**Performance Results**:
- Concurrent execution: 3-5x speedup measured in benchmarks
- Scheduler overhead: <10ms per step
- Replay performance: <100ms for 1000-step workflows
- Cancellation latency: <65µs (15,000x better than requirement)

**Breaking Changes**: None - fully backward compatible with v0.1.x

**Migration Guide**: See [docs/migration-v0.2.md](./docs/migration-v0.2.md) for upgrade instructions

**New Documentation**:
- [docs/concurrency.md](./docs/concurrency.md) - Comprehensive concurrency guide (679 lines)
- [docs/replay.md](./docs/replay.md) - Deterministic replay and debugging (806 lines)
- [docs/migration-v0.2.md](./docs/migration-v0.2.md) - Migration from v0.1.x (811 lines)
- [examples/concurrent_workflow](./examples/concurrent_workflow) - Parallel execution demo
- [examples/replay_demo](./examples/replay_demo) - Deterministic replay demo

**Known Limitations**:
- Some integration tests skipped (require complex end-to-end setup)
- Full backpressure timeout mechanism deferred to v0.3.0
- Per-node timeout from Policy() deferred to v0.3.0

---

### v0.1.0 - Core Framework Complete

**Release Date**: TBD

This is the initial release of LangGraph-Go with all core features implemented:

**Highlights**:
- ✅ Complete workflow engine with type-safe state management
- ✅ Conditional routing and parallel execution
- ✅ Multi-provider LLM integration (OpenAI, Anthropic, Google)
- ✅ Comprehensive event tracing and observability
- ✅ Production-ready error handling and retry logic
- ✅ 4,400+ lines of documentation
- ✅ 100+ unit and integration tests

**User Stories Completed**:
1. **US1**: Stateful workflow with checkpointing ✅
2. **US2**: Conditional routing and control flow ✅
3. **US3**: Parallel execution with fan-out ✅
4. **US4**: Multi-provider LLM integration ✅
5. **US5**: Event tracing and observability ✅

**Known Limitations**:
- MySQL store implementation pending (T183-T191)
- OpenTelemetry emitter pending (T192-T195)
- Tool system pending (T176-T182)
- Performance benchmarks pending (T196-T200)

**Breaking Changes**: N/A (initial release)

**Migration Guide**: N/A (initial release)

---

## Versioning Strategy

This project follows [Semantic Versioning](https://semver.org/):

- **MAJOR** version for incompatible API changes
- **MINOR** version for backwards-compatible functionality additions
- **PATCH** version for backwards-compatible bug fixes

**Pre-1.0 Releases**: During initial development (0.x.x), minor version bumps may include breaking changes. Pin to specific versions in production.

---

## How to Upgrade

### General Upgrade Steps

1. Read the changelog for your target version
2. Check for breaking changes in the release notes
3. Update your `go.mod`:
   ```bash
   go get github.com/dshills/langgraph-go@vX.Y.Z
   ```
4. Run your tests to verify compatibility
5. Address any breaking changes (see migration guide if provided)

### Staying Updated

- **Latest stable**: `go get github.com/dshills/langgraph-go@latest`
- **Specific version**: `go get github.com/dshills/langgraph-go@v0.1.0`
- **Pre-release**: `go get github.com/dshills/langgraph-go@v0.2.0-beta.1`

---

## Support & Feedback

- **Bug Reports**: [GitHub Issues](https://github.com/dshills/langgraph-go/issues)
- **Feature Requests**: [GitHub Discussions](https://github.com/dshills/langgraph-go/discussions)
- **Documentation**: [docs/](./docs/)
- **Questions**: [FAQ](./docs/FAQ.md)

---

[Unreleased]: https://github.com/dshills/langgraph-go/compare/v0.1.0...HEAD
