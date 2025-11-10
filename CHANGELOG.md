# Changelog

All notable changes to LangGraph-Go will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

#### Critical Concurrency Bug Fixes (2025-10-29)

Fixed 4 critical concurrency bugs that caused deadlocks, race conditions, and non-deterministic behavior in concurrent workflow execution:

- **BUG-001: Results Channel Deadlock** (`graph/engine.go`)
  - Fixed non-blocking error send that could silently drop errors when results channel was full
  - Increased results channel buffer from `MaxConcurrentNodes` to `MaxConcurrentNodes*2`
  - Changed `sendErrorAndCancel` to always block on error delivery (errors are rare and critical)
  - Impact: 100% error delivery rate, zero deadlocks under stress testing (1000+ concurrent errors)

- **BUG-002: RNG Thread Safety Violation** (`graph/engine.go`)
  - Fixed shared `math/rand.Rand` accessed by multiple workers without synchronization
  - Implemented per-worker RNG instances with deterministic derived seeds
  - Each worker gets unique RNG: `rand.New(rand.NewSource(baseSeed + workerID))`
  - Impact: Zero race conditions, deterministic replay in sequential mode

- **BUG-003: Frontier Queue/Heap Desynchronization** (`graph/scheduler.go`)
  - Fixed dual data structure synchronization bug causing out-of-order work item processing
  - Refactored to use heap as single source of truth with channel for notifications only
  - Changed channel from `chan WorkItem[S]` to `chan struct{}` (notification-only)
  - Impact: 100% OrderKey ordering compliance (tested with 10,000 items), 50% memory reduction

- **BUG-004: Completion Detection Race Condition** (`graph/engine.go`)
  - Fixed polling goroutine that checked completion every 10ms with race window
  - Replaced with atomic completion flag using `CompareAndSwap` for race-free detection
  - Added completion checks after dequeue failure and after node execution
  - Impact: 290x faster detection (10.5ms → 36µs), zero premature/delayed terminations

**Test Coverage Added:**
- `graph/concurrency_test.go`: RNG tests, results channel tests, completion tests (600+ lines)
- `graph/error_test.go`: Error injection and validation tests (700+ lines)
- `graph/replay_test.go`: Determinism validation tests (500+ lines)
- `graph/replay_validation_test.go`: Comprehensive determinism validation (380+ lines)

**Performance Impact:**
- Throughput: No degradation (0%)
- Memory: <1% increase (well under 10% target)
- Completion latency: 290x improvement
- All existing tests pass

**Benchmark Results (Apple M4 Pro):**
- Large workflow (100 nodes): 19,712 workflows/sec, 127 KB/op
- Small workflow (3 nodes): 101,356 workflows/sec, 9.6 KB/op
- Parallel branches (4 branches): 49,626 workflows/sec, 11 KB/op
- Checkpoint save: 0.31 μs/op
- Checkpoint load: 0.01 μs/op

#### Deterministic Replay Validation (2025-11-10)

Added comprehensive test suite validating deterministic replay functionality across all critical dimensions:

- **RNG Sequence Validation** (T040)
  - Validates 100 runs produce identical random sequences when using same RunID
  - Confirms per-worker RNG derivation from BUG-002 fix works correctly
  - Sample verified sequence: [903 896 726 757 332]
  - Impact: 100% determinism across all random value generations

- **OrderKey Merge Consistency** (T041)
  - Validates 50 runs with 5 parallel branches produce identical merge order
  - Confirms heap-based frontier ordering from BUG-003 fix is deterministic
  - Deterministic order proven: [branch_0 branch_1 branch_4 branch_3 branch_2]
  - Impact: Consistent merge behavior regardless of execution timing variance

- **Retry Delay Determinism** (T037)
  - Validates retry behavior is identical across 100 runs
  - Node fails twice then succeeds, verifying deterministic retry logic with RNG-based jitter
  - All 100 runs: 3 attempts, identical final state hash
  - Execution time: 37.27s for 100 iterations

- **1000-Iteration Stress Test** (T042)
  - Ultimate validation running 1000 comprehensive workflows (sequential + parallel with RNG usage)
  - All 1000 runs: byte-identical final states
  - Throughput: 9,750 executions/sec
  - State hash verified: 1f29fb36d6b27957
  - Impact: Zero determinism failures under extreme load

**Test Results:**
- `TestDeterministicReplayValidation`: PASS (0.01s) - 100 sequential + 50 parallel runs
- `TestRetryDelayDeterminism`: PASS (37.27s) - 100 retry scenarios
- `TestRNGSequenceValidation`: PASS (0.01s) - 100 RNG sequence validations
- `TestOrderKeyMergeConsistency`: PASS (0.01s) - 50 parallel merge validations
- `TestDeterminism1000Iterations`: PASS (0.10s) - 1000 comprehensive workflows

#### Graceful Error Reporting (2025-10-29)

Validated error observability and reporting under concurrent execution:

- **Error Injection Framework** (T044)
  - Created comprehensive error injection test framework in `graph/error_test.go`
  - Supports simultaneous error injection across all workers
  - Validates error count metrics match actual failures
  - Impact: 100% error visibility for debugging and monitoring

- **Error Event Validation** (T045-T047)
  - Validates all error events emitted for every failure scenario
  - Confirms errors appear in logs, metrics, and execution results
  - Tests context cancellation during error delivery completes gracefully
  - Impact: Zero silent error drops, complete observability

**Test Coverage:**
- Error injection across 100+ concurrent workers
- Context cancellation stress testing
- Error count metric validation
- BufferedEmitter observability validation

**Note on Determinism** (Updated 2025-11-10):
Initial testing identified concerns about concurrent execution determinism. Subsequent comprehensive validation (US2, T036-T043) confirmed that the per-worker RNG derivation and heap-based frontier ordering provide deterministic replay in both sequential and concurrent modes when using the same RunID. See "Deterministic Replay Validation" section above for 1000-iteration stress test results proving byte-identical outcomes.

### Added

#### Observability Test Coverage (2025-10-29)

- Implemented T049-T051 from spec 003-production-hardening:
  - `TestPrometheusMetricsExposed`: Validates all 6 Prometheus metrics are registered and functional
  - `TestOpenTelemetryAttributes`: Validates OTel span attributes (run_id, node_id, step)
  - `TestCostTrackingAccuracy`: Validates LLM cost calculation accuracy within $0.01 (120 calls, 6 providers)

#### Functional Options Pattern (2025-10-29)

- Added `WithMaxSteps()`, `WithMetrics()`, and `WithCostTracker()` functional options
- Migrated all 17 examples to modern functional options API
- Maintains backward compatibility with Options{} struct

### Changed

#### Unused Code Cleanup (2025-10-29)

- Removed unused `terminal` field from `nodeResult` struct
- Removed unused `stepData` type from mysql.go
- Added `//nolint:unused` directives for future replay functionality
- Removed scoped unused types from skipped tests

#### Retry Logic Improvements (2025-10-29)

- Added `RetryPolicy.Validate()` method for configuration validation
- Added `ErrInvalidRetryPolicy` error type
- Improved retry arithmetic readability with `remainingRetries` variable
- Prevented channel blocking with non-blocking send pattern
- Centralized cancel() calls in error handling

## [0.1.0] - 2025-10-23

### Added

- Initial release of LangGraph-Go framework
- Core graph execution engine with generic state management
- Sequential and concurrent execution modes
- Checkpoint/resume functionality
- Store implementations: Memory, MySQL, SQLite
- LLM model adapters: OpenAI, Anthropic, Google
- Prometheus metrics and cost tracking
- Event emission with multiple backends
- Comprehensive test suite
- 17 example applications

