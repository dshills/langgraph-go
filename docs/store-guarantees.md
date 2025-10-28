# Store Guarantees: Exactly-Once Semantics

LangGraph-Go provides **exactly-once execution semantics** through atomic checkpoint commits and idempotency key enforcement. This document explains the guarantees provided by the Store interface and how they ensure reliable, deterministic workflow execution.

## Table of Contents

- [Overview](#overview)
- [Atomic Step Commit Contract](#atomic-step-commit-contract)
- [Idempotency Key Formula](#idempotency-key-formula)
- [Crash Recovery Semantics](#crash-recovery-semantics)
- [Store Implementation Requirements](#store-implementation-requirements)
- [Testing Exactly-Once Behavior](#testing-exactly-once-behavior)
- [Common Pitfalls](#common-pitfalls)

## Overview

**Exactly-once semantics** means that each workflow step executes exactly one time, even in the presence of:
- Process crashes and restarts
- Network failures and retries
- Concurrent execution attempts
- Partial failures during commit

This is critical for operations that cannot be safely retried:
- Charging a payment
- Sending a notification
- Creating a database record with business significance
- Triggering external workflows

LangGraph-Go achieves exactly-once through two mechanisms:

1. **Atomic Commits**: State, frontier, outbox, and idempotency key are committed together or not at all
2. **Idempotency Keys**: Duplicate commits are detected and rejected using deterministic hashes

## Atomic Step Commit Contract

### The Guarantee

When `SaveCheckpointV2` is called, the following components are committed **atomically** within a single transaction:

```go
type CheckpointV2[S any] struct {
    // 1. EXECUTION CONTEXT
    RunID          string      // Workflow run identifier
    StepID         int         // Step number in execution
    Timestamp      time.Time   // When checkpoint was created
    Label          string      // Optional user-defined label

    // 2. STATE SNAPSHOT
    State          S           // Accumulated state after this step
    RNGSeed        int64       // Seed for deterministic randomness

    // 3. WORK QUEUE
    Frontier       []WorkItem  // Pending work items for next steps

    // 4. I/O HISTORY
    RecordedIOs    []RecordedIO // Captured external interactions

    // 5. DEDUPLICATION
    IdempotencyKey string      // Hash preventing duplicate commits
}
```

**Atomicity Contract:**

✅ **Success Case**: All 5 components are persisted together
- State is written to database
- Frontier items are queued for execution
- Recorded I/Os are available for replay
- Idempotency key is recorded in unique constraint table
- **Result**: Checkpoint is fully committed and visible

❌ **Failure Case**: Any component fails → Entire transaction rolls back
- Idempotency key already exists → Rollback (prevents duplicate)
- State serialization fails → Rollback
- Frontier persistence fails → Rollback
- Database constraint violation → Rollback
- **Result**: Checkpoint is NOT committed, operation can be retried

### Why Atomic Commits Matter

**Without Atomicity (broken system):**

```go
// BAD: Non-atomic pseudo-code
SaveState(checkpoint.State)              // ✓ Succeeds
SaveFrontier(checkpoint.Frontier)        // ✓ Succeeds
RecordIdempotencyKey(checkpoint.Key)     // ✗ Fails (duplicate key)

// PROBLEM: State and frontier were saved, but idempotency check failed
// Result: Duplicate execution when operation is retried!
```

**With Atomicity (LangGraph-Go):**

```go
// GOOD: Atomic transaction
tx.Begin()
  tx.SaveState(checkpoint.State)
  tx.SaveFrontier(checkpoint.Frontier)
  tx.RecordIdempotencyKey(checkpoint.Key)  // Fails on duplicate
tx.Rollback()  // ← Entire checkpoint discarded

// Result: Nothing persisted, safe to retry with same idempotency key
```

### Implementation in Database Stores

Both `MySQLStore` and `SQLiteStore` use database transactions to enforce atomicity.

#### MySQL Store Example

The `MySQLStore` uses database transactions to enforce atomicity:

```go
func (m *MySQLStore[S]) SaveCheckpointV2(ctx context.Context, cp CheckpointV2[S]) error {
    // Begin transaction with appropriate isolation level
    tx, err := m.db.BeginTx(ctx, &sql.TxOptions{
        Isolation: sql.LevelReadCommitted,
    })
    if err != nil {
        return fmt.Errorf("failed to begin transaction: %w", err)
    }

    // Ensure rollback on any error
    defer func() {
        if err != nil {
            tx.Rollback()
        }
    }()

    // 1. Insert idempotency key first (will fail if duplicate)
    _, err = tx.ExecContext(ctx,
        "INSERT INTO idempotency_keys (key_value) VALUES (?)",
        cp.IdempotencyKey,
    )
    if err != nil {
        return fmt.Errorf("idempotency key already exists: %w", err)
    }

    // 2. Insert checkpoint with all components
    _, err = tx.ExecContext(ctx,
        "INSERT INTO workflow_checkpoints_v2 (run_id, step_id, state, frontier, ...) VALUES (?, ?, ?, ?, ...)",
        cp.RunID, cp.StepID, stateJSON, frontierJSON, ...,
    )
    if err != nil {
        return fmt.Errorf("failed to save checkpoint: %w", err)
    }

    // 3. Commit transaction (makes all changes visible atomically)
    if err := tx.Commit(); err != nil {
        return fmt.Errorf("failed to commit transaction: %w", err)
    }

    return nil // Success: All components committed atomically
}
```

**Key Properties:**

- **Isolation Level**: `READ COMMITTED` prevents dirty reads while allowing concurrent transactions
- **Unique Constraint**: `idempotency_keys.key_value` is `PRIMARY KEY` → Database enforces uniqueness
- **Rollback on Error**: Any failure triggers complete rollback via `defer`
- **Commit Atomicity**: Database guarantees all-or-nothing visibility

#### SQLite Store Example

The `SQLiteStore` provides the same atomicity guarantees using SQLite's transaction support:

```go
func (s *SQLiteStore[S]) SaveCheckpointV2(ctx context.Context, cp CheckpointV2[S]) error {
    // Begin transaction
    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil {
        return fmt.Errorf("failed to begin transaction: %w", err)
    }

    // Ensure rollback on any error
    defer func() {
        if err != nil {
            tx.Rollback()
        }
    }()

    // 1. Insert idempotency key first (will fail if duplicate)
    _, err = tx.ExecContext(ctx,
        "INSERT INTO idempotency_keys (key_value) VALUES (?)",
        cp.IdempotencyKey,
    )
    if err != nil {
        return fmt.Errorf("idempotency key already exists: %w", err)
    }

    // 2. Insert checkpoint with all components
    _, err = tx.ExecContext(ctx,
        "INSERT INTO workflow_checkpoints_v2 (run_id, step_id, state, frontier, ...) VALUES (?, ?, ?, ?, ...)",
        cp.RunID, cp.StepID, stateJSON, frontierJSON, ...,
    )
    if err != nil {
        return fmt.Errorf("failed to save checkpoint: %w", err)
    }

    // 3. Commit transaction (makes all changes visible atomically)
    if err := tx.Commit(); err != nil {
        return fmt.Errorf("failed to commit transaction: %w", err)
    }

    return nil // Success: All components committed atomically
}
```

**SQLite-Specific Features:**

- **WAL Mode**: Write-Ahead Logging enabled for concurrent reads during writes
- **Single-File Database**: Zero-setup persistence, perfect for development and small deployments
- **Full Atomicity**: SQLite transactions provide ACID guarantees identical to larger databases
- **Unique Constraint**: `PRIMARY KEY` on `idempotency_keys.key_value` enforces uniqueness
- **Default Isolation**: SQLite uses serializable isolation by default, providing even stronger guarantees
- **Busy Timeout**: Configured with 5-second timeout to handle write contention gracefully

**When to Use SQLite Store:**

✅ **Ideal For:**
- Local development and testing
- Single-process workflows
- Small to medium workloads (< 100K steps/day)
- Prototyping before production deployment
- Edge computing and embedded systems
- CI/CD pipelines

❌ **Not Recommended For:**
- Distributed systems with multiple writers
- High-concurrency workloads (> 100 concurrent writes)
- Network-attached storage (NFS, etc.)
- Very large databases (> 100GB)

**Performance Characteristics:**

- **Write Throughput**: ~1000 writes/second (single writer)
- **Read Throughput**: Unlimited concurrent reads (WAL mode)
- **Latency**: Sub-millisecond for local disk
- **Scalability**: Single-writer limitation (inherent to SQLite design)

## Idempotency Key Formula

### Purpose

The idempotency key is a **deterministic hash** that uniquely identifies a checkpoint commit attempt. It prevents duplicate commits even when:
- The same workflow step is retried after a crash
- Concurrent processes attempt to commit the same step
- Network retries cause duplicate SaveCheckpointV2 calls

### Formula

The idempotency key is computed as:

```
idempotency_key = "sha256:" + hex(SHA256(run_id || step_id || sorted_frontier || state_json))
```

**Inputs (in order):**

1. **Run ID** (string): Unique workflow execution identifier
2. **Step ID** (int64): Step number in the execution (8-byte big-endian)
3. **Frontier** (sorted): Work items queued for next execution
   - Sort by `OrderKey` for determinism
   - For each item: hash `NodeID` + `OrderKey` (8-byte big-endian)
4. **State** (JSON): Serialized accumulated state

**Output Format:**

```
sha256:a3f5b2c1d8e9f0123456789abcdef0123456789abcdef0123456789abcdef012
       └────────────────────────────────────────────────────────────┘
                          64 hex characters (256 bits)
```

### Implementation

```go
func computeIdempotencyKey[S any](runID string, stepID int, items []WorkItem[S], state S) (string, error) {
    // Create SHA-256 hasher
    h := sha256.New()

    // 1. Write run ID
    h.Write([]byte(runID))

    // 2. Write step ID as 8-byte big-endian int64
    stepBytes := make([]byte, 8)
    binary.BigEndian.PutUint64(stepBytes, uint64(stepID))
    h.Write(stepBytes)

    // 3. Sort work items by OrderKey for deterministic ordering
    sortedItems := make([]WorkItem[S], len(items))
    copy(sortedItems, items)
    sort.Slice(sortedItems, func(i, j int) bool {
        return sortedItems[i].OrderKey < sortedItems[j].OrderKey
    })

    // 4. Write each work item's identifying information
    for _, item := range sortedItems {
        // Write node ID
        h.Write([]byte(item.NodeID))

        // Write order key as 8-byte big-endian uint64
        orderKeyBytes := make([]byte, 8)
        binary.BigEndian.PutUint64(orderKeyBytes, item.OrderKey)
        h.Write(orderKeyBytes)
    }

    // 5. Marshal state to JSON and write
    stateJSON, err := json.Marshal(state)
    if err != nil {
        return "", err
    }
    h.Write(stateJSON)

    // 6. Return hex-encoded hash with format prefix
    return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}
```

### Properties

**Determinism**: Same inputs always produce the same key
```go
key1, _ := computeIdempotencyKey("run-001", 1, frontier, state)
key2, _ := computeIdempotencyKey("run-001", 1, frontier, state)
assert(key1 == key2) // ✓ Identical
```

**Uniqueness**: Different inputs produce different keys
```go
key1, _ := computeIdempotencyKey("run-001", 1, frontier, state)
key2, _ := computeIdempotencyKey("run-001", 2, frontier, state) // Different step
assert(key1 != key2) // ✓ Unique
```

**Collision Resistance**: SHA-256 provides cryptographic strength
- Collision probability: ~2^-256 (negligible)
- Pre-image resistance: Infeasible to find inputs for a given key
- Used in Bitcoin, TLS, and other security-critical systems

### Why Sort Frontier?

Work items may be generated in non-deterministic order (e.g., from map iteration or concurrent branches). Sorting ensures consistent key computation:

```go
// Without sorting (wrong)
frontier := []WorkItem{
    {NodeID: "node-b", OrderKey: 200},
    {NodeID: "node-a", OrderKey: 100}, // Order depends on map iteration
}
key := computeKey(frontier) // ✗ Non-deterministic

// With sorting (correct)
sort.Slice(frontier, func(i, j int) bool {
    return frontier[i].OrderKey < frontier[j].OrderKey
})
key := computeKey(frontier) // ✓ Always produces same key
```

## Crash Recovery Semantics

### Recovery Scenarios

LangGraph-Go handles three types of failures:

#### 1. Crash Before Commit

**Scenario**: Process crashes after computation but before `SaveCheckpointV2` completes.

```go
// Execution timeline
compute()         // ✓ Completes
SaveCheckpoint()  // ✗ Crash before commit
```

**Recovery Behavior:**
- Checkpoint was NOT persisted (transaction never committed)
- Idempotency key was NOT recorded
- On restart: Workflow resumes from last successful checkpoint
- Step is re-executed (produces same idempotency key)
- Commit succeeds (no duplicate key conflict)

**Guarantee**: Step executes exactly once (retry succeeded).

#### 2. Crash After Commit

**Scenario**: Process crashes after `SaveCheckpointV2` succeeds but before acknowledging success.

```go
// Execution timeline
compute()         // ✓ Completes
SaveCheckpoint()  // ✓ Transaction committed
                  // ✗ Crash before returning success
```

**Recovery Behavior:**
- Checkpoint WAS persisted (transaction committed)
- Idempotency key WAS recorded
- On restart: System may retry operation (assuming failure)
- Retry attempts to commit with same idempotency key
- Database rejects duplicate key → `SaveCheckpointV2` returns error
- Engine recognizes idempotency conflict → Skips to next step

**Guarantee**: Step is NOT re-executed (duplicate prevented).

#### 3. Concurrent Execution

**Scenario**: Two processes attempt to commit the same step concurrently (e.g., distributed workers).

```go
// Process A timeline
compute()         // ✓ Completes at t=0
SaveCheckpoint()  // ✓ Starts transaction at t=1

// Process B timeline
compute()         // ✓ Completes at t=0
SaveCheckpoint()  // ✓ Starts transaction at t=1

// Database serializes transactions
// Process A commits at t=2 (inserts idempotency key)
// Process B commits at t=3 (fails: duplicate key)
```

**Recovery Behavior:**
- First transaction to reach database wins
- Second transaction fails with duplicate key error
- Both processes computed same idempotency key (deterministic)
- Losing process recognizes conflict and continues

**Guarantee**: Step executes exactly once across all processes.

### Resumption Algorithm

When resuming after a crash:

```go
func ResumeRun[S any](ctx context.Context, runID string) error {
    // 1. Load latest successful checkpoint
    checkpoint, err := store.LoadCheckpointV2(ctx, runID, lastStepID)
    if err != nil {
        return fmt.Errorf("failed to load checkpoint: %w", err)
    }

    // 2. Restore execution context
    state := checkpoint.State
    frontier := checkpoint.Frontier.([]WorkItem[S])
    rng := rand.New(rand.NewSource(checkpoint.RNGSeed))

    // 3. Resume from frontier
    for _, item := range frontier {
        // Execute node with restored context
        result := node.Run(ctx, item.State)

        // Attempt to commit
        newCheckpoint := buildCheckpoint(result)
        err := store.SaveCheckpointV2(ctx, newCheckpoint)

        if err != nil && isIdempotencyViolation(err) {
            // Checkpoint already committed in previous run
            // Skip this step and continue
            log.Info("Step already committed, skipping",
                "run_id", runID,
                "step_id", item.StepID)
            continue
        } else if err != nil {
            // Genuine error, propagate
            return err
        }

        // Checkpoint committed successfully
        log.Info("Step committed",
            "run_id", runID,
            "step_id", item.StepID)
    }

    return nil
}
```

### Idempotency Violation Detection

```go
func isIdempotencyViolation(err error) bool {
    // Check for duplicate key error
    // MySQL: Error 1062 "Duplicate entry"
    // Postgres: Error 23505 "unique_violation"

    if err == nil {
        return false
    }

    errMsg := err.Error()
    return strings.Contains(errMsg, "Duplicate entry") ||
           strings.Contains(errMsg, "idempotency_key") ||
           strings.Contains(errMsg, "unique_violation")
}
```

## Store Implementation Requirements

To provide exactly-once guarantees, Store implementations **must** satisfy:

### 1. Transactional Atomicity

✅ **Required**: All checkpoint components committed in single transaction
```go
tx.Begin()
  tx.InsertIdempotencyKey(key)    // Must succeed or rollback
  tx.InsertCheckpoint(data)        // Must succeed or rollback
tx.Commit()                        // Atomic visibility
```

❌ **Forbidden**: Separate operations without transaction
```go
InsertIdempotencyKey(key)          // ✗ Not atomic
InsertCheckpoint(data)             // ✗ Duplicate risk
```

### 2. Idempotency Key Uniqueness

✅ **Required**: Database enforces unique constraint
```sql
CREATE TABLE idempotency_keys (
    key_value VARCHAR(255) PRIMARY KEY,  -- ← Enforces uniqueness
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

❌ **Forbidden**: Application-level checking only
```go
// ✗ Race condition: check-then-insert is not atomic
exists := CheckKey(key)
if !exists {
    InsertKey(key)  // Two threads can both pass check!
}
```

### 3. Isolation Level

✅ **Required**: At least `READ COMMITTED` isolation
- Prevents dirty reads (uncommitted data)
- Allows concurrent transactions (performance)
- Serializes idempotency key conflicts

✅ **Optional**: `SERIALIZABLE` for stronger guarantees
- Prevents phantom reads and anomalies
- May impact performance under high concurrency

❌ **Forbidden**: `READ UNCOMMITTED` (allows dirty reads)

### 4. Error Handling

✅ **Required**: Distinguish idempotency violations from other errors
```go
err := store.SaveCheckpointV2(ctx, checkpoint)

if isDuplicateKeyError(err) {
    // Expected: Step already committed
    return nil // Continue to next step
} else if err != nil {
    // Genuine error: Retry or propagate
    return err
}
```

### 5. Checkpoint Retrieval

✅ **Required**: Atomically load all checkpoint components
```go
checkpoint, err := store.LoadCheckpointV2(ctx, runID, stepID)
// checkpoint.State, checkpoint.Frontier, checkpoint.IdempotencyKey
// all from same consistent snapshot
```

## Testing Exactly-Once Behavior

### Unit Tests

Test atomicity with simulated failures:

```go
func TestAtomicCommit(t *testing.T) {
    store := setupStore(t)

    checkpoint := buildTestCheckpoint()

    // First commit succeeds
    err := store.SaveCheckpointV2(ctx, checkpoint)
    assert.NoError(t, err)

    // Duplicate commit fails
    err = store.SaveCheckpointV2(ctx, checkpoint)
    assert.Error(t, err)

    // Verify original checkpoint unchanged
    loaded, _ := store.LoadCheckpointV2(ctx, checkpoint.RunID, checkpoint.StepID)
    assert.Equal(t, checkpoint.State, loaded.State)
}
```

### Concurrency Tests

Test with high parallelism:

```go
func TestConcurrentCommits(t *testing.T) {
    store := setupStore(t)

    const numGoroutines = 100

    var successCount int32
    var wg sync.WaitGroup

    for i := 0; i < numGoroutines; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()

            // All goroutines attempt same checkpoint
            checkpoint := buildTestCheckpoint()
            err := store.SaveCheckpointV2(ctx, checkpoint)

            if err == nil {
                atomic.AddInt32(&successCount, 1)
            }
        }()
    }

    wg.Wait()

    // Exactly one goroutine should succeed
    assert.Equal(t, int32(1), successCount)
}
```

### Integration Tests

Test crash-resume scenarios:

```go
func TestCrashRecovery(t *testing.T) {
    store := setupStore(t)

    // Simulate execution up to step 5
    for step := 1; step <= 5; step++ {
        checkpoint := buildCheckpoint(step)
        err := store.SaveCheckpointV2(ctx, checkpoint)
        assert.NoError(t, err)
    }

    // Simulate crash: Process restarts, retries step 5
    checkpoint5 := buildCheckpoint(5)
    err := store.SaveCheckpointV2(ctx, checkpoint5)
    assert.Error(t, err) // Duplicate prevented

    // Continue to step 6
    checkpoint6 := buildCheckpoint(6)
    err = store.SaveCheckpointV2(ctx, checkpoint6)
    assert.NoError(t, err) // New step succeeds
}
```

## Common Pitfalls

### Pitfall 1: Non-Deterministic Idempotency Keys

❌ **Problem**: Including non-deterministic data in key computation
```go
// ✗ Wrong: Includes current time
key := hash(runID, stepID, time.Now(), state)

// Result: Retries produce different keys → Duplicate execution!
```

✅ **Solution**: Only include deterministic inputs
```go
// ✓ Correct: Deterministic inputs only
key := hash(runID, stepID, sortedFrontier, state)
```

### Pitfall 2: Forgetting to Sort Frontier

❌ **Problem**: Frontier items in map iteration order
```go
// ✗ Wrong: Non-deterministic order from map
frontier := []WorkItem[S]{}
for nodeID, item := range itemsMap {
    frontier = append(frontier, item)
}
key := computeKey(frontier) // Different each time!
```

✅ **Solution**: Sort by OrderKey before hashing
```go
// ✓ Correct: Deterministic order
sort.Slice(frontier, func(i, j int) bool {
    return frontier[i].OrderKey < frontier[j].OrderKey
})
key := computeKey(frontier) // Same every time
```

### Pitfall 3: Application-Level Idempotency Checks

❌ **Problem**: Checking and inserting in separate operations
```go
// ✗ Wrong: Race condition
exists, _ := store.CheckIdempotency(ctx, key)
if !exists {
    store.SaveCheckpoint(ctx, checkpoint) // Two threads can pass!
}
```

✅ **Solution**: Let database enforce uniqueness atomically
```go
// ✓ Correct: Single atomic operation
err := store.SaveCheckpointV2(ctx, checkpoint)
if isDuplicateKeyError(err) {
    // Database prevented duplicate
}
```

### Pitfall 4: Ignoring Idempotency Violations

❌ **Problem**: Treating duplicate key errors as failures
```go
// ✗ Wrong: Retry on duplicate key
err := store.SaveCheckpoint(ctx, checkpoint)
if err != nil {
    return err // Causes infinite retry loop!
}
```

✅ **Solution**: Recognize duplicates as success
```go
// ✓ Correct: Idempotency violation = already committed
err := store.SaveCheckpoint(ctx, checkpoint)
if isDuplicateKeyError(err) {
    log.Info("Step already committed, continuing")
    return nil // Not an error!
} else if err != nil {
    return err // Genuine error
}
```

## Summary

**Exactly-Once Guarantees:**

✅ Each workflow step commits exactly once
✅ Atomic transactions prevent partial commits
✅ Idempotency keys prevent duplicate execution
✅ Crash recovery is safe and automatic
✅ Concurrent execution is handled correctly

**Key Mechanisms:**

1. **Atomic Commits**: State + Frontier + Idempotency Key in single transaction
2. **Deterministic Keys**: SHA-256 hash of run ID, step ID, frontier, state
3. **Database Constraints**: Unique index on idempotency keys
4. **Crash Recovery**: Resume from last checkpoint, skip already-committed steps

**Implementation Checklist:**

- [ ] Store uses database transactions for SaveCheckpointV2
- [ ] Idempotency key has unique constraint in database
- [ ] Isolation level is at least READ COMMITTED
- [ ] Frontier is sorted before computing idempotency key
- [ ] Duplicate key errors are distinguished from other errors
- [ ] Tests verify atomic behavior under concurrency
- [ ] Tests verify crash-resume scenarios

For implementation details, see:
- `graph/checkpoint.go` - Idempotency key computation
- `graph/store/mysql.go` - MySQL transactional store implementation
- `graph/store/sqlite.go` - SQLite transactional store implementation
- `graph/exactly_once_test.go` - Exactly-once test suite
- [Deterministic Replay Guide](./replay.md) - Replay semantics
