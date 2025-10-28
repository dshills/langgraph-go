# Error Handling

LangGraph-Go provides comprehensive error handling with typed errors that can be checked using `errors.Is()` and `errors.As()`.

## Error Types

### Exported Sentinel Errors

All sentinel errors are exported from the `graph` package and can be checked with `errors.Is()`:

```go
import (
    "errors"
    "github.com/dshills/langgraph-go/graph"
)

_, err := engine.Run(ctx, runID, initial)
if errors.Is(err, graph.ErrMaxStepsExceeded) {
    // Handle max steps error
}
```

### ErrMaxStepsExceeded

**When**: Workflow execution exceeds the `MaxSteps` limit without completing.

**Cause**: Infinite loop or MaxSteps set too low for workflow complexity.

**Resolution**:
- Increase MaxSteps if workflow is legitimately long
- Check for missing loop exit conditions
- Verify conditional edges have exit paths

```go
import "errors"

final, err := engine.Run(ctx, runID, initial)
if errors.Is(err, graph.ErrMaxStepsExceeded) {
    log.Printf("Workflow exceeded maximum steps - check for infinite loop")
    // Increase MaxSteps or fix loop logic
}
```

**Prevention**:
```go
engine := graph.New(
    reducer, store, emitter,
    graph.WithMaxSteps(100), // Set appropriate limit
)
```

### ErrBackpressure

**When**: Downstream processing cannot keep up with current execution rate.

**Cause**: Output buffers full, rate limits exceeded, or consumer too slow.

**Resolution**:
- Reduce input rate
- Increase consumer capacity
- Add buffering or queuing

```go
if errors.Is(err, graph.ErrBackpressure) {
    log.Printf("System under backpressure - reducing load")
    // Implement rate limiting or retry with backoff
}
```

**Note**: This is distinct from `ErrBackpressureTimeout` which is specific to frontier queue overflow.

### ErrBackpressureTimeout

**When**: Frontier queue remains full beyond the configured `BackpressureTimeout`.

**Cause**: Nodes are being enqueued faster than they can be executed.

**Resolution**:
- Increase `MaxConcurrentNodes` to process more nodes in parallel
- Increase `QueueDepth` to buffer more pending work
- Increase `BackpressureTimeout` if temporary spikes are expected
- Checkpoint and resume after reducing load

```go
if errors.Is(err, graph.ErrBackpressureTimeout) {
    log.Printf("Frontier queue saturated - checkpointing and pausing")
    // Save checkpoint, then resume after load reduction
    engine.SaveCheckpoint(ctx, runID, "backpressure-pause")
}
```

**Prevention**:
```go
engine := graph.New(
    reducer, store, emitter,
    graph.WithMaxConcurrent(16),                  // More parallelism
    graph.WithQueueDepth(2048),                   // Larger queue
    graph.WithBackpressureTimeout(60*time.Second), // Longer timeout
)
```

### ErrReplayMismatch

**When**: Recorded I/O hash doesn't match current execution during replay mode.

**Cause**: Node logic changed between recording and replay, causing non-deterministic behavior.

**Resolution**:
- Disable `StrictReplay` for "best effort" replay
- Update recorded I/O by re-running in record mode
- Fix non-deterministic node logic (random values, timestamps, etc.)

```go
if errors.Is(err, graph.ErrReplayMismatch) {
    log.Printf("Replay detected logic change - re-record or disable strict mode")
    // Re-run in record mode or use StrictReplay=false
}
```

**Replay Configuration**:
```go
// Strict replay (default) - fails on mismatch
engine := graph.New(
    reducer, store, emitter,
    graph.WithReplayMode(true),
    graph.WithStrictReplay(true), // Fail on hash mismatch
)

// Best-effort replay - tolerates changes
engine := graph.New(
    reducer, store, emitter,
    graph.WithReplayMode(true),
    graph.WithStrictReplay(false), // Allow minor changes
)
```

See [replay.md](replay.md) for complete replay documentation.

### ErrNoProgress

**When**: Scheduler detects a deadlock with no runnable nodes.

**Cause**:
- All nodes waiting on conditions that will never be satisfied
- Circular dependencies without conditional break
- Missing edges or routing logic

**Resolution**:
- Review edge predicates for always-false conditions
- Check for circular dependencies
- Verify all nodes have valid next hops

```go
if errors.Is(err, graph.ErrNoProgress) {
    log.Printf("Deadlock detected - check graph topology and edge conditions")
    // Review graph structure and predicates
}
```

**Example Deadlock**:
```go
// WRONG: Circular dependency with no exit
engine.Connect("A", "B", nil)
engine.Connect("B", "C", nil)
engine.Connect("C", "A", nil) // Creates infinite loop

// CORRECT: Add exit condition
engine.Connect("A", "B", nil)
engine.Connect("B", "C", nil)
engine.Connect("C", "A", func(s State) bool {
    return s.Counter < 10 // Exit condition
})
engine.Connect("C", "end", func(s State) bool {
    return s.Counter >= 10 // Exit path
})
```

### ErrIdempotencyViolation

**When**: Attempting to commit a checkpoint with a duplicate idempotency key.

**Cause**: Same execution step committed twice (usually during retry or crash recovery).

**Resolution**:
- This is typically not an error - it means checkpoint was already committed
- The store prevents duplicate commits automatically
- No action needed - system maintains exactly-once semantics

```go
if errors.Is(err, graph.ErrIdempotencyViolation) {
    log.Printf("Checkpoint already committed - continuing")
    // Safe to continue - exactly-once guarantee upheld
}
```

**How it Works**:
```go
// Idempotency key computed from: runID + stepID + state + frontier
key := computeIdempotencyKey(runID, stepID, state, frontier)

// Store checks if key exists before commit
if store.CheckIdempotency(key) {
    return ErrIdempotencyViolation // Already committed
}

// Commit atomically with idempotency check
store.SaveCheckpointV2(checkpoint)
```

See [store-guarantees.md](store-guarantees.md) for exactly-once semantics.

### ErrMaxAttemptsExceeded

**When**: Node fails more times than allowed by its retry policy.

**Cause**: Persistent failure that exceeds `MaxAttempts` in retry policy.

**Resolution**:
- Check node error logs to diagnose root cause
- Increase `MaxAttempts` if failures are transient
- Fix underlying issue causing repeated failures
- Add circuit breaker pattern for external dependencies

```go
if errors.Is(err, graph.ErrMaxAttemptsExceeded) {
    log.Printf("Node exhausted retry attempts - check logs for failure cause")
    // Review node error logs and fix root cause
}
```

**Retry Configuration**:
```go
type MyNode struct{}

func (n *MyNode) Policy() graph.NodePolicy {
    return graph.NodePolicy{
        RetryPolicy: &graph.RetryPolicy{
            MaxAttempts: 3,              // Total attempts (initial + retries)
            BaseDelay:   100 * time.Millisecond,
            MaxDelay:    5 * time.Second,
            Retryable: func(err error) bool {
                // Only retry transient errors
                return isTransient(err)
            },
        },
    }
}

func (n *MyNode) Run(ctx context.Context, state State) graph.NodeResult[State] {
    // Node implementation...
}
```

## EngineError Type

`EngineError` is a structured error type with error codes:

```go
type EngineError struct {
    Message string
    Code    string
}
```

**Checking EngineError**:
```go
var engineErr *graph.EngineError
if errors.As(err, &engineErr) {
    log.Printf("Engine error: %s (code: %s)", engineErr.Message, engineErr.Code)

    switch engineErr.Code {
    case "MAX_STEPS_EXCEEDED":
        // Handle max steps
    case "MISSING_REDUCER":
        // Handle configuration error
    case "NODE_NOT_FOUND":
        // Handle graph topology error
    default:
        // Handle unknown error
    }
}
```

**Common Error Codes**:

| Code | Meaning | Resolution |
|------|---------|------------|
| `MAX_STEPS_EXCEEDED` | Exceeded MaxSteps limit | Increase MaxSteps or fix loop |
| `MISSING_REDUCER` | No reducer provided | Pass reducer to New() |
| `MISSING_STORE` | No store provided | Pass store to New() |
| `NO_START_NODE` | StartAt() not called | Call StartAt() before Run() |
| `NODE_NOT_FOUND` | Referenced node doesn't exist | Check node IDs match |
| `DUPLICATE_NODE` | Node ID already registered | Use unique node IDs |
| `NO_ROUTE` | No valid next node | Add edges or explicit routing |
| `CHECKPOINT_SAVE_FAILED` | Checkpoint write failed | Check store health |
| `CHECKPOINT_NOT_FOUND` | Checkpoint doesn't exist | Verify checkpoint ID |
| `STORE_ERROR` | Store operation failed | Check database connectivity |
| `REPLAY_MODE_REQUIRED` | Replay called without ReplayMode | Enable ReplayMode |
| `UNSUPPORTED_CONFLICT_POLICY` | Invalid conflict policy | Use ConflictFail |

## Error Handling Patterns

### Basic Error Handling

```go
final, err := engine.Run(ctx, runID, initial)
if err != nil {
    log.Printf("Workflow failed: %v", err)
    return err
}
```

### Typed Error Handling

```go
import "errors"

final, err := engine.Run(ctx, runID, initial)
if err != nil {
    switch {
    case errors.Is(err, graph.ErrMaxStepsExceeded):
        log.Printf("Workflow exceeded max steps")
        // Handle infinite loop case

    case errors.Is(err, graph.ErrBackpressureTimeout):
        log.Printf("System overloaded")
        // Checkpoint and retry later
        engine.SaveCheckpoint(ctx, runID, "backpressure-pause")

    case errors.Is(err, graph.ErrReplayMismatch):
        log.Printf("Replay mismatch detected")
        // Re-record or disable strict replay

    default:
        log.Printf("Unknown error: %v", err)
        // Generic error handling
    }
    return err
}
```

### Structured Error Logging

```go
var engineErr *graph.EngineError
if errors.As(err, &engineErr) {
    log.Printf("error_code=%s message=%s", engineErr.Code, engineErr.Message)

    // Send to error tracking service
    errorTracker.Report(engineErr.Code, engineErr.Message, map[string]interface{}{
        "run_id": runID,
        "state":  initial,
    })
}
```

### Retry with Exponential Backoff

```go
func runWithRetry(engine *graph.Engine[State], ctx context.Context, runID string, initial State) (State, error) {
    maxRetries := 3
    baseDelay := time.Second

    for attempt := 0; attempt < maxRetries; attempt++ {
        final, err := engine.Run(ctx, runID, initial)
        if err == nil {
            return final, nil
        }

        // Only retry on backpressure
        if !errors.Is(err, graph.ErrBackpressure) {
            return final, err
        }

        // Exponential backoff
        delay := baseDelay * time.Duration(1<<uint(attempt))
        log.Printf("Backpressure detected, retrying in %v (attempt %d/%d)", delay, attempt+1, maxRetries)
        time.Sleep(delay)
    }

    return State{}, fmt.Errorf("max retries exceeded")
}
```

### Circuit Breaker Pattern

```go
type CircuitBreaker struct {
    mu           sync.Mutex
    failures     int
    lastFailure  time.Time
    threshold    int
    resetTimeout time.Duration
    isOpen       bool
}

func (cb *CircuitBreaker) Call(fn func() error) error {
    cb.mu.Lock()
    defer cb.mu.Unlock()

    // Check if circuit should reset
    if cb.isOpen && time.Since(cb.lastFailure) > cb.resetTimeout {
        cb.isOpen = false
        cb.failures = 0
    }

    // Circuit is open - fail fast
    if cb.isOpen {
        return fmt.Errorf("circuit breaker open")
    }

    // Execute function
    err := fn()
    if err != nil {
        cb.failures++
        cb.lastFailure = time.Now()

        // Open circuit if threshold reached
        if cb.failures >= cb.threshold {
            cb.isOpen = true
        }
        return err
    }

    // Success - reset failures
    cb.failures = 0
    return nil
}

// Usage
cb := &CircuitBreaker{threshold: 3, resetTimeout: 30 * time.Second}

err := cb.Call(func() error {
    _, err := engine.Run(ctx, runID, initial)
    return err
})
```

## Context Cancellation

Workflows respect context cancellation for graceful shutdown:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()

final, err := engine.Run(ctx, runID, initial)
if errors.Is(err, context.DeadlineExceeded) {
    log.Printf("Workflow timed out after 5 minutes")
    // Save checkpoint for resumption
    engine.SaveCheckpoint(ctx, runID, "timeout-checkpoint")
}
```

## Node-Level Error Handling

Nodes return errors in `NodeResult.Err`:

```go
func (n *MyNode) Run(ctx context.Context, state State) graph.NodeResult[State] {
    result, err := doWork(ctx, state)
    if err != nil {
        // Return error - engine will handle retry/propagation
        return graph.NodeResult[State]{
            Err: fmt.Errorf("work failed: %w", err),
        }
    }

    return graph.NodeResult[State]{
        Delta: State{Result: result},
        Route: graph.Stop(),
    }
}
```

**With Retry Policy**:
```go
func (n *MyNode) Policy() graph.NodePolicy {
    return graph.NodePolicy{
        RetryPolicy: &graph.RetryPolicy{
            MaxAttempts: 3,
            Retryable: func(err error) bool {
                // Classify errors as retryable
                return errors.Is(err, io.ErrUnexpectedEOF) ||
                       errors.Is(err, syscall.ECONNRESET)
            },
        },
    }
}
```

## Best Practices

1. **Always Check Errors**: Never ignore errors from `Run()`, `Add()`, `StartAt()`, etc.
2. **Use Typed Errors**: Check with `errors.Is()` for known error types
3. **Log Structured**: Include error codes and context for debugging
4. **Set MaxSteps**: Always set to prevent infinite loops
5. **Handle Backpressure**: Implement checkpointing and retry logic
6. **Use Circuit Breakers**: Protect external dependencies from cascading failures
7. **Test Error Paths**: Write tests for error conditions
8. **Monitor Error Rates**: Track error frequency in production
9. **Document Errors**: Document which errors each operation can return

## Testing Error Handling

```go
func TestWorkflowErrors(t *testing.T) {
    tests := []struct {
        name        string
        maxSteps    int
        wantErr     error
    }{
        {
            name:     "exceeds max steps",
            maxSteps: 5,
            wantErr:  graph.ErrMaxStepsExceeded,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            engine := graph.New(
                reducer, store, emitter,
                graph.Options{MaxSteps: tt.maxSteps},
            )

            // Build graph...

            _, err := engine.Run(ctx, runID, initial)
            if !errors.Is(err, tt.wantErr) {
                t.Errorf("got error %v, want %v", err, tt.wantErr)
            }
        })
    }
}
```

## See Also

- [quickstart.md](quickstart.md) - Getting started guide
- [replay.md](replay.md) - Replay and debugging
- [store-guarantees.md](store-guarantees.md) - Exactly-once semantics
- [concurrency.md](concurrency.md) - Concurrent execution
