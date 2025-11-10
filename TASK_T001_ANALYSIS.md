# Task T001: Concurrent Retry Implementation Analysis

## Overview
This document analyzes the concurrent retry implementation in `graph/engine.go:runConcurrent` (lines 1114-1246) to serve as a reference pattern for sequential retry implementation in T006-T009.

## Key Code Components

### Method Signature
```go
func (e *Engine[S]) runConcurrent(ctx context.Context, runID string, initial S) (S, error)
```

Location: `graph/engine.go:952-1350`

Worker pool pattern with concurrent node execution supporting automatic retries on transient failures.

---

## Concurrent Retry Pattern Summary

### 1. Node Policy Retrieval (Lines 1115-1119)
```go
var policy *NodePolicy
if policyProvider, ok := nodeImpl.(interface{ Policy() NodePolicy }); ok {
    p := policyProvider.Policy()
    policy = &p
}
```
- Nodes implement optional `Policy() NodePolicy` interface method
- Allows per-node configuration of timeout and retry behavior
- Policy can be nil if node doesn't implement the interface

### 2. Work-Item-Specific RNG (Lines 1121-1126)
```go
itemSeed := baseSeed ^ int64(item.OrderKey)
itemRNG := rand.New(rand.NewSource(itemSeed))
```
- **Deterministic replay**: Same OrderKey always produces same RNG sequence
- BaseSeed derived from runID hash (SHA256) for per-run determinism
- Workers can execute in any order but retry backoff is reproducible

### 3. Node Execution with Timeout (Lines 1135-1158)
```go
result, timeoutErr := executeNodeWithTimeout(nodeCtx, nodeImpl, item.NodeID, item.State, policy, e.opts.DefaultNodeTimeout)

if timeoutErr != nil {
    if result.Err == nil {
        result.Err = timeoutErr
    } else {
        result.Err = fmt.Errorf("%w (node also returned: %v)", timeoutErr, result.Err)
    }
}
```
- Wraps execution with context timeout
- Preserves both timeout and node errors if both occur
- Timeout enforced via `executeNodeWithTimeout` in `timeout.go`

### 4. Error Handling with Retry Logic (Lines 1160-1246)

#### Transient vs. Permanent Error Detection (Lines 1178-1199)
```go
if policy != nil && policy.RetryPolicy != nil {
    retryPol := policy.RetryPolicy
    
    // Validate retry policy configuration
    if err := retryPol.Validate(); err != nil {
        sendErrorAndCancel(fmt.Errorf("retry policy validation failed for node %s: %w", item.NodeID, err))
        return
    }
    
    // Check if error is retryable using predicate (T084)
    isRetryable := retryPol.Retryable != nil && retryPol.Retryable(result.Err)
    
    // Calculate remaining retry attempts (T089)
    remainingRetries := retryPol.MaxAttempts - item.Attempt - 1
}
```

Key distinction:
- **Retryable**: Determined by `RetryPolicy.Retryable` predicate function
- **Transient errors** (retryable): Network failures, timeouts, rate limits (HTTP 429, 503, 504)
- **Permanent errors**: Non-retryable, cause immediate termination
- `MaxAttempts` includes initial attempt (3 = 1 initial + 2 retries)

#### Retry Decision (Lines 1199-1238)
```go
if isRetryable && remainingRetries > 0 {
    // Increment retry metrics
    if e.metrics != nil {
        e.metrics.IncrementRetries(runID, item.NodeID, "error")
    }
    
    // Compute backoff delay (T086, T090)
    delay := computeBackoff(item.Attempt, retryPol.BaseDelay, retryPol.MaxDelay, rng)
    
    // Apply backoff delay before re-enqueueing
    time.Sleep(delay)
    
    // Create retry work item with incremented attempt (T088)
    retryItem := WorkItem[S]{
        StepID:       item.StepID,      // Same step ID (retry, not new step)
        OrderKey:     item.OrderKey,    // Preserve order key for determinism
        NodeID:       item.NodeID,      // Same node
        State:        item.State,       // Same input state
        Attempt:      item.Attempt + 1, // Increment attempt counter
        ParentNodeID: item.ParentNodeID,
        EdgeIndex:    item.EdgeIndex,
    }
    
    // Re-enqueue for retry
    if err := e.frontier.Enqueue(workerCtx, retryItem); err != nil {
        sendErrorAndCancel(result.Err)
        return
    }
    
    // Exit function, outer loop dequeues next item
    return
}
```

Critical design points:
- Retryable errors trigger re-enqueuing, not inline retry loop
- WorkItem reused with `Attempt+1`
- OrderKey preserved for deterministic merge order
- StepID unchanged (retry doesn't increment step)
- Backoff applied BEFORE re-enqueue via `time.Sleep(delay)`

#### Max Attempts Exceeded (Lines 1240-1245)
```go
if remainingRetries <= 0 {
    // Max attempts exceeded (T089)
    sendErrorAndCancel(ErrMaxAttemptsExceeded)
    return
}
```
- Error sent to results channel for caller
- Execution cancelled via `cancel()`

### 5. Backoff Calculation (Lines 1205-1211)
Located in `graph/policy.go:113-136`:

```go
func computeBackoff(attempt int, base, maxDelay time.Duration, rng *rand.Rand) time.Duration {
    // Compute exponential delay: base * 2^attempt
    exponentialDelay := base * (1 << attempt)
    
    // Cap at maxDelay to prevent unbounded growth
    if exponentialDelay > maxDelay {
        exponentialDelay = maxDelay
    }
    
    // Add jitter: random value between 0 and base
    var jitter time.Duration
    if rng != nil {
        jitter = time.Duration(rng.Int63n(int64(base)))
    } else {
        jitter = time.Duration(rand.Int63n(int64(base)))
    }
    
    return exponentialDelay + jitter
}
```

Backoff formula:
- `delay = min(base * 2^attempt, maxDelay) + jitter(0, base)`
- Exponential growth: 2^attempt for each retry
- Jitter range: 0 to base (prevents thundering herd)
- Example with base=1s, maxDelay=30s:
  - attempt 0: 1s + jitter(0-1s) = 1-2s
  - attempt 1: 2s + jitter(0-1s) = 2-3s
  - attempt 2: 4s + jitter(0-1s) = 4-5s
  - attempt 10: 30s + jitter(0-1s) = 30-31s (capped)

### 6. Error Reporting (Lines 1167-1176)
```go
sendErrorAndCancel := func(err error) {
    select {
    case results <- nodeResult[S]{err: err}:
        // Error sent successfully
    case <-ctx.Done():
        // Parent context canceled before send completed
    }
    cancel()
}
```
- Non-blocking send with context cancellation fallback
- Results channel buffered at `maxWorkers*2` to prevent deadlock
- All critical errors go through this function

---

## Sequential Retry Pattern (for reference, lines 763-832)

The existing sequential retry implementation uses a simpler inline loop:

```go
for attempt := 0; attempt <= maxRetries; attempt++ {
    attemptCtx := context.WithValue(ctx, AttemptKey, attempt)
    
    result, timeoutErr = executeNodeWithTimeout(attemptCtx, nodeImpl, currentNode, currentState, policy, e.opts.DefaultNodeTimeout)
    
    if timeoutErr != nil {
        result.Err = timeoutErr
    }
    
    if result.Err == nil {
        break
    }
    
    if attempt < maxRetries {
        // Compute backoff (basic implementation)
        shift := attempt
        if shift > 10 {
            shift = 10
        }
        backoff := baseDelay * time.Duration(1<<uint(shift))
        if backoff > maxBackoff {
            backoff = maxBackoff
        }
        
        // Add jitter
        var jitter time.Duration
        if rng != nil {
            jitter = time.Duration(rng.Intn(50)) * time.Millisecond
        }
        delay := backoff + jitter
        
        select {
        case <-time.After(delay):
            continue
        case <-ctx.Done():
            return zero, ctx.Err()
        }
    }
    
    e.emitError(runID, currentNode, step-1, result.Err)
    return zero, result.Err
}
```

**Key differences**:
- Sequential: Inline retry loop with blocking delays
- Concurrent: Re-enqueue pattern via work item frontier
- Sequential uses `maxRetries` (retry count), concurrent uses `MaxAttempts` (total attempts)

---

## Data Structures

### RetryPolicy (graph/policy.go:36-56)
```go
type RetryPolicy struct {
    MaxAttempts int                     // Total attempts (1-based including initial)
    BaseDelay   time.Duration           // Base delay for exponential backoff
    MaxDelay    time.Duration           // Maximum delay cap
    Retryable   func(error) bool        // Predicate to determine if error is retryable
}
```

### NodePolicy (graph/policy.go:16-29)
```go
type NodePolicy struct {
    Timeout            time.Duration              // Per-node timeout override
    RetryPolicy        *RetryPolicy               // Automatic retry configuration
    IdempotencyKeyFunc func(state any) string    // Idempotency key generation
}
```

### WorkItem (used internally)
```go
type WorkItem[S any] struct {
    StepID       int    // Monotonic step counter
    OrderKey     uint64 // Deterministic ordering for concurrent merge
    NodeID       string // Node to execute
    State        S      // Input state
    Attempt      int    // Zero-based attempt number
    ParentNodeID string // Previous node (for tracing)
    EdgeIndex    int    // Edge index (for fan-out)
}
```

---

## Critical Design Patterns for Sequential Implementation

### Pattern 1: Attempt Counting
- **Concurrent**: Zero-based `item.Attempt` (0 = first execution)
  - `remainingRetries = MaxAttempts - item.Attempt - 1`
- **Sequential**: Should use same pattern for consistency

### Pattern 2: Policy Validation
```go
if err := retryPol.Validate(); err != nil {
    // Wrap validation error with context
    return fmt.Errorf("retry policy validation failed for node %s: %w", item.NodeID, err)
}
```
- Always validate policy before attempting retries
- Prevents silent failures from misconfiguration

### Pattern 3: Deterministic RNG for Backoff
```go
// Concurrent
itemSeed := baseSeed ^ int64(item.OrderKey)
itemRNG := rand.New(rand.NewSource(itemSeed))
delay := computeBackoff(item.Attempt, retryPol.BaseDelay, retryPol.MaxDelay, rng)

// Sequential should use runID-derived RNG (already initialized in context)
rng := initRNG(runID)
ctx = context.WithValue(ctx, RNGKey, rng)
```
- RNG seeded from runID ensures deterministic replay
- Same RNG instance reused across attempts

### Pattern 4: Metrics and Observability
```go
if e.metrics != nil {
    e.metrics.IncrementRetries(runID, item.NodeID, "error")
}
```
- Track retry attempts for observability
- Helps identify problematic nodes in production

### Pattern 5: Error Context Preservation
```go
if result.Err == nil {
    result.Err = timeoutErr
} else {
    result.Err = fmt.Errorf("%w (node also returned: %v)", timeoutErr, result.Err)
}
```
- Preserve both timeout and node errors
- Provides complete error context for debugging

---

## Context Keys Used

From `graph/engine.go:37-50`:

```go
const (
    RunIDKey    contextKey = "langgraph.run_id"
    StepIDKey   contextKey = "langgraph.step_id"
    NodeIDKey   contextKey = "langgraph.node_id"
    OrderKeyKey contextKey = "langgraph.order_key"
    RNGKey      contextKey = "langgraph.rng"
    AttemptKey  contextKey = "langgraph.attempt"
)
```

- `RNGKey`: Access deterministic RNG from context in nodes
- `AttemptKey`: Current retry attempt number for node introspection
- All context keys are private type (`contextKey`) to prevent collisions

---

## Helper Functions

### initRNG (deterministic seeding)
Location: `graph/engine.go` (search for definition)
- Seeds RNG from runID hash (SHA256)
- Ensures replay produces identical random sequences

### computeBackoff (exponential backoff with jitter)
Location: `graph/policy.go:113-136`
- Formula: `min(base * 2^attempt, maxDelay) + jitter(0, base)`
- Prevents thundering herd via randomized jitter
- Capped exponential growth prevents overflow

### executeNodeWithTimeout (timeout enforcement)
Location: `graph/timeout.go:48-84`
- Precedence: NodePolicy.Timeout > DefaultNodeTimeout > unlimited
- Merges timeout errors with node errors if both occur
- Uses context.WithTimeout for cancellation

### (RetryPolicy).Validate (policy validation)
Location: `graph/policy.go:143-151`
- Checks: `MaxAttempts >= 1` and `MaxDelay >= BaseDelay` (if both > 0)
- Called before attempting retries to fail fast on configuration errors

---

## Summary: Key Insights for Sequential Implementation

1. **Use NodePolicy.RetryPolicy** instead of Engine.Retries to align with concurrent pattern
2. **Validate policy before retrying** to detect misconfigurations early
3. **Use work-item-derived RNG** for deterministic backoff jitter (concurrent pattern preserved)
4. **Preserve OrderKey semantics** in sequential mode for future migration to concurrent
5. **Track metrics** via `e.metrics.IncrementRetries()` for observability
6. **Merge timeout and node errors** to preserve complete error context
7. **Use computeBackoff()** function directly (no reimplementation)
8. **Attempt numbering**: Zero-based, use `MaxAttempts - attempt - 1` for remaining count
9. **Emit events** for node start, end, and errors for complete observability trail
10. **Handle context cancellation** during backoff delays via select statement

---

## Files Modified

- `graph/engine.go`: runConcurrent method (lines 952-1350) and Run method (lines 649-907)
- `graph/policy.go`: RetryPolicy, NodePolicy, computeBackoff, Validate
- `graph/timeout.go`: executeNodeWithTimeout, getNodeTimeout

## Related Tasks

- **T006-T009**: Sequential retry implementation (reference this analysis)
- **T087-T090**: Concurrent retry implementation (already complete)
- **US1**: Reliable workflow execution (sequential mode)
- **US2**: Deterministic replay validation
