# Testing Contracts

This document describes the contract tests that validate LangGraph-Go's production guarantees for determinism, exactly-once semantics, concurrency, and backpressure.

## Overview

Contract tests are critical integration tests that prove the system upholds its documented guarantees under various conditions. These tests serve as:

1. **Production Confidence**: Prove the system behaves correctly under real-world conditions
2. **Regression Prevention**: Catch breaking changes before they reach production
3. **Documentation**: Demonstrate expected behavior through executable examples
4. **Debugging Aid**: Identify the root cause when guarantees are violated

## Test Categories

### 1. Determinism Contracts

**Purpose**: Prove that graph execution produces identical results across replays

**Tests**:

#### TestReplayMismatchDetection (`graph/determinism_test.go`)
- **What it tests**: Parameter drift detection during replay
- **Why it matters**: Non-deterministic nodes break replay debugging
- **How it works**: Runs a node twice with different behavior, verifies mismatch is detected
- **Expected behavior**: `ErrReplayMismatch` raised when outputs differ

#### TestMergeOrderingWithRandomDelays (`graph/determinism_test.go`)
- **What it tests**: Deterministic merge order despite timing variations
- **Why it matters**: Production workflows must produce consistent results regardless of CPU load or network latency
- **How it works**: Creates 5 parallel branches with random 0-100ms delays, verifies merge order matches edge indices
- **Expected behavior**: Merge order is deterministic across 10 runs, all branches complete exactly once

**Key Guarantee**: Same inputs → Same outputs, regardless of execution timing

---

### 2. Exactly-Once Semantics

**Purpose**: Prove that state updates happen exactly once, never duplicated or lost

**Tests**:

#### TestConcurrentStateUpdates (`graph/exactly_once_test.go`)
- **What it tests**: Concurrent state updates don't lose data
- **Why it matters**: Financial transactions, inventory updates, and other critical operations must never be lost
- **How it works**: 100 goroutines each increment a counter by 1
- **Expected behavior**: Final counter value = 100 (no lost updates)

#### TestAtomicStepCommit (`graph/exactly_once_test.go`)
- **What it tests**: Checkpoint commits are atomic (all-or-nothing)
- **Why it matters**: Partial commits can leave the system in an inconsistent state
- **How it works**: Saves checkpoint with state+frontier+idempotency key, verifies all persisted or all rolled back
- **Expected behavior**: Atomic commit or complete rollback, never partial

#### TestIdempotencyEnforcement (`graph/exactly_once_test.go`)
- **What it tests**: Duplicate idempotency keys are rejected
- **Why it matters**: Prevents duplicate execution of non-idempotent operations (e.g., charging a credit card twice)
- **How it works**: Attempts to save checkpoint with duplicate idempotency key
- **Expected behavior**: Second save rejected, first checkpoint unchanged

#### TestNoDuplicatesUnderConcurrency (`graph/exactly_once_test.go`)
- **What it tests**: No duplicates even under high concurrency with retries
- **Why it matters**: Production systems retry failed operations; idempotency must hold under race conditions
- **How it works**: 100 concurrent workflows × 10 steps × 3 retries = 3000 attempts, but only 1000 commits
- **Expected behavior**: Exactly 1000 checkpoints saved, zero duplicates

**Key Guarantee**: State mutations happen exactly once, enforced by idempotency keys

---

### 3. Backpressure & Flow Control

**Purpose**: Prove that the system handles overload gracefully without crashes or memory exhaustion

**Tests**:

#### TestBackpressureBlocking (`graph/scheduler_test.go`)
- **What it tests**: Queue blocks when at capacity, unblocks when space available
- **Why it matters**: Prevents memory exhaustion when producers outpace consumers
- **How it works**: QueueDepth=1, enqueue 3 items, verify second blocks until first dequeued
- **Expected behavior**: Enqueue blocks at capacity, unblocks after dequeue, respects context cancellation

**Key Guarantee**: Bounded queue depth prevents runaway memory growth

---

### 4. RNG Determinism

**Purpose**: Prove that random number generation is deterministic for replay scenarios

**Tests**:

#### TestRNGDeterminism (`graph/replay_test.go`)
- **What it tests**: Same seed produces identical random sequences across replays
- **Why it matters**: Workflows using randomness (e.g., sampling, Monte Carlo) must be replayable for debugging
- **How it works**: Generate 100 random values with same seed 10 times, verify all sequences identical
- **Expected behavior**: 10 replays produce byte-identical random sequences

**Key Guarantee**: RNG seeded from RunID enables deterministic replay of stochastic workflows

---

### 5. Cross-Store Consistency

**Purpose**: Prove that all Store implementations uphold the same contracts

**Tests**:

#### TestIdempotencyAcrossStores (`graph/store/common_test.go`)
- **What it tests**: MemStore, MySQLStore, and SQLiteStore all enforce idempotency
- **Why it matters**: Switching between stores (dev → staging → prod) must not change behavior
- **How it works**: Runs identical idempotency test against all three stores
- **Expected behavior**: All stores reject duplicate keys, all stores allow progression with unique keys

**Key Guarantee**: Store interface contract is consistently implemented across all backends

---

## Running Contract Tests

### Run All Contract Tests
```bash
go test -v -race ./graph/...
```

### Run Specific Contract Test
```bash
# Determinism
go test -v -race -run TestReplayMismatchDetection ./graph/

# Exactly-once
go test -v -race -run TestConcurrentStateUpdates ./graph/

# Backpressure
go test -v -race -run TestBackpressureBlocking ./graph/

# RNG Determinism
go test -v -race -run TestRNGDeterminism ./graph/

# Cross-store
go test -v -race -run TestIdempotencyAcrossStores ./graph/store/
```

### Run With Coverage
```bash
go test -v -race -coverprofile=coverage.out ./graph/...
go tool cover -html=coverage.out
```

### MySQL Integration Tests
```bash
# Start MySQL (Docker)
docker run -d -p 3306:3306 \
  -e MYSQL_ROOT_PASSWORD=testpassword \
  -e MYSQL_DATABASE=langgraph_test \
  mysql:8.0

# Run with MySQL
export TEST_MYSQL_DSN="root:testpassword@tcp(127.0.0.1:3306)/langgraph_test?parseTime=true"
go test -v -race ./graph/...
```

---

## CI/CD Integration

Contract tests run automatically on every push and pull request via GitHub Actions:

- **Platforms**: Linux, macOS, Windows
- **Go versions**: 1.21, 1.22, 1.23
- **Race detector**: Enabled for all contract tests
- **Coverage**: Uploaded to Codecov
- **MySQL**: Integrated for exactly-once tests (Linux only)

See `.github/workflows/contract-tests.yml` for full CI configuration.

---

## Interpreting Test Failures

### TestReplayMismatchDetection Failures
**Symptom**: Test fails to detect parameter drift  
**Root cause**: Node using non-deterministic sources (time.Now, crypto/rand, filesystem)  
**Fix**: Use context-provided RNG, record external I/O, make nodes pure functions

### TestMergeOrderingWithRandomDelays Failures
**Symptom**: Merge order varies across runs  
**Root cause**: Order key computation not deterministic, or scheduler not respecting order keys  
**Fix**: Verify ComputeOrderKey is pure, verify Frontier.Dequeue sorts by OrderKey

### TestConcurrentStateUpdates Failures
**Symptom**: Final counter < 100 (lost updates)  
**Root cause**: Reducer not called for all deltas, or state mutations not atomic  
**Fix**: Verify reducer is invoked for every node output, check for race conditions with `-race`

### TestBackpressureBlocking Failures
**Symptom**: Enqueue doesn't block when queue full  
**Root cause**: Channel capacity not enforced, or backpressure detection not implemented  
**Fix**: Verify Frontier uses buffered channel with correct capacity, verify Enqueue blocks on channel send

### TestRNGDeterminism Failures
**Symptom**: Sequences differ across replays  
**Root cause**: RNG not seeded from RunID, or nodes using global rand instead of context RNG  
**Fix**: Verify initRNG derives seed from RunID, verify nodes extract RNG from context

### TestIdempotencyAcrossStores Failures
**Symptom**: One store allows duplicates while others reject  
**Root cause**: Store implementation not checking idempotency key before commit  
**Fix**: Verify SaveCheckpointV2 calls CheckIdempotency, verify unique constraint on idempotency_keys table

---

## Adding New Contract Tests

When adding a new production guarantee, follow this process:

1. **Document the Guarantee**: Add to spec.md with FR-XXX identifier
2. **Write the Test**: Create test that FAILS if guarantee violated
3. **Add to CI**: Include test in `.github/workflows/contract-tests.yml`
4. **Document Here**: Add test description to this file
5. **Update README**: Add test to testing section

Example test template:

```go
// TestNewGuarantee (TXXX) verifies that [description]
//
// According to spec.md FR-XXX: [requirement]
//
// Requirements:
// - [requirement 1]
// - [requirement 2]
//
// This test proves that [what it proves]
func TestNewGuarantee(t *testing.T) {
    t.Run("scenario 1", func(t *testing.T) {
        // Setup
        // Execute
        // Assert
    })
}
```

---

## Performance Benchmarks

Contract tests are also benchmarked to catch performance regressions:

```bash
go test -bench=. -benchmem ./graph/...
```

Key metrics tracked:
- `BenchmarkOrderKeyGeneration`: Order key computation must be < 1µs
- `BenchmarkFrontierEnqueue`: Enqueue must be < 10µs
- `BenchmarkReducerExecution`: Reducer must be < 100µs per merge

See `graph/benchmark_test.go` for full benchmark suite.

---

## Related Documentation

- [Architecture](architecture.md) - System design overview
- [Concurrency](concurrency.md) - Concurrent execution model
- [Replay](replay.md) - Deterministic replay system
- [Store Guarantees](store-guarantees.md) - Persistence contracts
- [Observability](observability.md) - Monitoring and debugging

---

## FAQ

**Q: Why use contract tests instead of unit tests?**  
A: Unit tests verify isolated components. Contract tests verify system-level guarantees that emerge from component interactions. Both are necessary.

**Q: How often should contract tests run?**  
A: Every commit. Contract tests are fast (< 2 minutes) and catch critical bugs early.

**Q: Can I skip contract tests in development?**  
A: No. Contract tests prevent subtle bugs that only appear in production under load or timing variations.

**Q: What's the difference between contract tests and integration tests?**  
A: Contract tests validate specific guarantees (determinism, exactly-once). Integration tests validate feature workflows (create order, process payment). Contract tests focus on non-functional requirements.

**Q: Why test with race detector?**  
A: Go's race detector catches data races that cause non-deterministic behavior and lost updates. Critical for concurrent systems.

**Q: How do I debug a failing contract test?**  
A: 
1. Run test with `-v` flag for verbose output
2. Check test logs for specific assertion failure
3. Add `t.Logf()` statements to trace execution
4. Run with `-race` to detect concurrency bugs
5. Use debugger (Delve) to step through execution

---

**Last Updated**: 2025-10-28  
**Maintained By**: LangGraph-Go Core Team
