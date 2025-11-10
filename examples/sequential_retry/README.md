# Sequential Retry Example

This example demonstrates sequential workflow execution with automatic retry policy configuration in LangGraph-Go.

## Overview

Sequential execution (`MaxConcurrentNodes: 0`) runs nodes one at a time in deterministic order. When combined with retry policies, this enables predictable, resumable workflows with transient failure handling.

## What is Sequential Execution?

Sequential execution means nodes run one at a time, in order:

```
Node A (attempt 0 → retry → retry → success)
  ↓
Node B (attempt 0 → success)
  ↓
Node C (attempt 0 → retry → success)
  ↓
Workflow complete
```

### Sequential vs Concurrent

| Aspect | Sequential (MaxConcurrentNodes: 0) | Concurrent (MaxConcurrentNodes: N) |
|--------|-----------------------------------|-----------------------------------|
| **Execution** | One node at a time | Multiple nodes in parallel |
| **Order** | Deterministic sequence | Non-deterministic (worker pool) |
| **Retry** | In-place for loop | Re-enqueue work item |
| **State** | Single state copy | State copies per branch |
| **Use Case** | Linear workflows, debugging | Fan-out, high throughput |

## RetryPolicy Configuration

Configure automatic retries using `NodePolicy.RetryPolicy`:

```go
func (n MyNode) Policy() graph.NodePolicy {
    return graph.NodePolicy{
        Timeout: 5 * time.Second,
        RetryPolicy: &graph.RetryPolicy{
            MaxAttempts: 5,                      // Total attempts (initial + retries)
            BaseDelay:   100 * time.Millisecond, // Starting backoff delay
            MaxDelay:    2 * time.Second,        // Maximum backoff cap
            Retryable: func(err error) bool {
                // Classify errors as retryable or permanent
                var transientErr TransientError
                return errors.As(err, &transientErr)
            },
        },
    }
}
```

### RetryPolicy Fields

#### MaxAttempts (int)
- **Total number of execution attempts** (includes initial attempt)
- Must be >= 1
- `MaxAttempts: 1` = no retries (initial attempt only)
- `MaxAttempts: 3` = up to 2 retries after initial attempt

#### BaseDelay (time.Duration)
- **Starting delay** for exponential backoff
- First retry waits: `BaseDelay * 2^0 + jitter`
- Second retry waits: `BaseDelay * 2^1 + jitter`
- Example: `BaseDelay: 100ms` → retries at ~100ms, ~200ms, ~400ms, etc.

#### MaxDelay (time.Duration)
- **Maximum backoff cap** to prevent unbounded growth
- Must be >= BaseDelay (if both are > 0)
- Set to 0 for no cap (use cautiously)
- Example: `MaxDelay: 30s` → backoff capped at 30 seconds

#### Retryable (func(error) bool)
- **Error classification predicate**
- Returns `true` if error should trigger a retry
- Returns `false` for permanent failures
- If `nil`, all errors are non-retryable

## Exponential Backoff with Jitter

Retry delays follow the formula:

```
delay = min(BaseDelay * 2^attempt, MaxDelay) + jitter(0, BaseDelay)
```

### Example Backoff Sequence

With `BaseDelay: 1s`, `MaxDelay: 30s`:

| Attempt | Exponential | Capped | Jitter Range | Total Delay |
|---------|-------------|--------|--------------|-------------|
| 0 (initial) | - | - | - | 0s (immediate) |
| 1 (1st retry) | 1s * 2^0 = 1s | 1s | 0-1s | 1-2s |
| 2 (2nd retry) | 1s * 2^1 = 2s | 2s | 0-1s | 2-3s |
| 3 (3rd retry) | 1s * 2^2 = 4s | 4s | 0-1s | 4-5s |
| 4 (4th retry) | 1s * 2^3 = 8s | 8s | 0-1s | 8-9s |
| 10 (10th retry) | 1s * 2^10 = 1024s | 30s (capped) | 0-1s | 30-31s |

### Why Jitter?

Jitter adds randomness to prevent **thundering herd** problems:
- Multiple nodes retry at exactly the same time
- Overload the recovering service
- Jitter spreads retries across a time window

### Deterministic Jitter

Jitter is **deterministic** based on runID:
- Same runID → same RNG seed → same jitter values
- Enables exact replay for debugging
- Different runIDs → different jitter patterns

```go
// Engine initializes RNG from runID
rng := initRNG(runID)  // Seeded from SHA256(runID)

// Node receives RNG via context
if rngVal := ctx.Value(graph.RNGKey); rngVal != nil {
    rng := rngVal.(*rand.Rand)
    // Use for deterministic random behavior
}
```

## Retry Attempt Tracking

Access the current retry attempt number via context:

```go
func (n MyNode) Run(ctx context.Context, state S) graph.NodeResult[S] {
    attempt := 0
    if attemptVal := ctx.Value(graph.AttemptKey); attemptVal != nil {
        attempt = attemptVal.(int)
    }

    if attempt > 0 {
        log.Printf("Retry attempt %d", attempt)
    }

    // ... node logic ...
}
```

### Attempt Numbering

- **Attempt 0**: Initial execution (not a retry)
- **Attempt 1**: First retry
- **Attempt 2**: Second retry
- **MaxAttempts: 3**: Allows attempts 0, 1, 2 (2 retries after initial)

## Error Classification

The `Retryable` predicate classifies errors:

```go
// Define custom error types
type TransientError struct { Message string }
func (e TransientError) Error() string { return e.Message }

type PermanentError struct { Message string }
func (e PermanentError) Error() string { return e.Message }

// Classification predicate
Retryable: func(err error) bool {
    // Check error type
    var transientErr TransientError
    if errors.As(err, &transientErr) {
        return true  // Retry transient errors
    }

    var permErr PermanentError
    if errors.As(err, &permErr) {
        return false  // Don't retry permanent errors
    }

    // Default: don't retry unknown errors
    return false
}
```

### Common Retryable Errors

**Network Errors:**
- Connection refused
- Connection timeout
- Temporary DNS failures
- Socket errors

**HTTP Status Codes:**
- 429 Too Many Requests
- 503 Service Unavailable
- 504 Gateway Timeout

**Database Errors:**
- Deadlock detected
- Connection lost
- Transaction timeout

### Common Non-Retryable Errors

**Validation Errors:**
- Invalid input format
- Schema violations
- Business rule failures

**HTTP Status Codes:**
- 400 Bad Request
- 401 Unauthorized
- 403 Forbidden
- 404 Not Found

**Application Errors:**
- Configuration errors
- Programming errors (nil pointer, etc.)
- Data corruption

## Example Scenarios

### Scenario 1: Transient Failure with Successful Retry

```go
type flakyNode struct {
    failUntilAttempt int
}

func (n flakyNode) Run(ctx context.Context, state S) graph.NodeResult[S] {
    attempt := ctx.Value(graph.AttemptKey).(int)

    if attempt < n.failUntilAttempt {
        return graph.NodeResult[S]{
            Err: TransientError{Message: "service unavailable"},
        }
    }

    return graph.NodeResult[S]{
        Delta: S{Status: "success"},
        Route: graph.Stop(),
    }
}

func (n flakyNode) Policy() graph.NodePolicy {
    return graph.NodePolicy{
        RetryPolicy: &graph.RetryPolicy{
            MaxAttempts: 5,
            BaseDelay:   100 * time.Millisecond,
            MaxDelay:    2 * time.Second,
            Retryable: func(err error) bool {
                var transientErr TransientError
                return errors.As(err, &transientErr)
            },
        },
    }
}
```

**Execution Flow:**
1. Attempt 0: Fails with TransientError → backoff ~100-200ms
2. Attempt 1: Fails with TransientError → backoff ~200-300ms
3. Attempt 2: Succeeds → continue workflow

### Scenario 2: Permanent Failure (No Retry)

```go
func (n validationNode) Run(ctx context.Context, state S) graph.NodeResult[S] {
    return graph.NodeResult[S]{
        Err: PermanentError{Message: "invalid configuration"},
    }
}

func (n validationNode) Policy() graph.NodePolicy {
    return graph.NodePolicy{
        RetryPolicy: &graph.RetryPolicy{
            MaxAttempts: 3,
            BaseDelay:   100 * time.Millisecond,
            MaxDelay:    1 * time.Second,
            Retryable: func(err error) bool {
                var permErr PermanentError
                return !errors.As(err, &permErr)  // Don't retry permanent errors
            },
        },
    }
}
```

**Execution Flow:**
1. Attempt 0: Fails with PermanentError
2. `Retryable()` returns `false`
3. Workflow fails immediately (no retries)

### Scenario 3: Max Attempts Exceeded

```go
func (n networkNode) Run(ctx context.Context, state S) graph.NodeResult[S] {
    // Always fails (network down)
    return graph.NodeResult[S]{
        Err: TransientError{Message: "network timeout"},
    }
}

func (n networkNode) Policy() graph.NodePolicy {
    return graph.NodePolicy{
        RetryPolicy: &graph.RetryPolicy{
            MaxAttempts: 2,  // Only 1 retry
            BaseDelay:   50 * time.Millisecond,
            MaxDelay:    500 * time.Millisecond,
            Retryable: func(err error) bool {
                var transientErr TransientError
                return errors.As(err, &transientErr)
            },
        },
    }
}
```

**Execution Flow:**
1. Attempt 0: Fails with TransientError → backoff ~50-100ms
2. Attempt 1: Fails with TransientError
3. Max attempts (2) reached → `graph.ErrMaxAttemptsExceeded`

## Running the Example

```bash
cd examples/sequential_retry
go run main.go
```

### Expected Output

```
=== Sequential Retry Policy Example ===

=== Scenario 1: Transient Failure with Successful Retry ===
Configuration:
- Node will fail on attempts 0 and 1 (transient errors)
- Node will succeed on attempt 2
- MaxAttempts: 5, BaseDelay: 100ms, MaxDelay: 2s
- Sequential execution (MaxConcurrentNodes: 0)

Execution time: ~450ms
Status: SUCCESS

Execution trace:
  1. Attempt 0: Service temporarily unavailable (will retry)
  2. Attempt 1: Service temporarily unavailable (will retry)
  3. Attempt 2: Success! Service is now available
  4. Validation complete - workflow succeeded after retries

Expected behavior:
- Attempt 0 fails → backoff ~100-200ms
- Attempt 1 fails → backoff ~200-300ms
- Attempt 2 succeeds
- Total backoff: ~300-500ms (deterministic based on runID)

=== Scenario 2: Permanent Failure (Non-Retryable) ===
Configuration:
- Node returns PermanentError (non-retryable)
- RetryPolicy.Retryable returns false for PermanentError
- Expected: Immediate failure without retries

Execution time: ~5ms
Error (expected): permanent error: invalid configuration

Execution trace:
  1. Attempt 0: Invalid configuration detected

Result: Failed immediately (no retries for permanent errors)

=== Scenario 3: Max Retry Attempts Exceeded ===
Configuration:
- Node always fails with TransientError (retryable)
- MaxAttempts: 2 (initial attempt + 1 retry)
- Expected: Fails with ErrMaxAttemptsExceeded after 2 attempts

Execution time: ~80ms
Error (expected): max retry attempts exceeded

Execution trace:
  1. Attempt 0 (seed=1): Network timeout
  2. Attempt 1 (seed=1): Network timeout

Result: Max attempts reached (2 total attempts)

=== Scenario 4: Node Without Retry Policy ===
Configuration:
- Node has no RetryPolicy
- Expected: Single execution (no retries)

Execution time: ~1ms
Status: SUCCESS

Execution trace:
  1. No retry policy - completes on first attempt

Result: Completed immediately (no retry policy)
```

## Deterministic Replay

Sequential retry execution is fully deterministic:

```go
// Run 1 with runID "workflow-123"
state1, err1 := engine.Run(ctx, "workflow-123", initialState)

// Run 2 with same runID "workflow-123"
state2, err2 := engine.Run(ctx, "workflow-123", initialState)

// Guaranteed to be identical:
// - Same retry attempts
// - Same backoff delays (same jitter from seeded RNG)
// - Same final state
// - Same error (if any)
```

### Why Determinism Matters

1. **Debugging**: Reproduce exact failure scenarios
2. **Testing**: Verify retry behavior is consistent
3. **Auditing**: Replay historical executions
4. **Checkpointing**: Resume from exact state

## Best Practices

### 1. Configure Appropriate MaxAttempts

```go
// Short-lived operations (API calls)
RetryPolicy: &graph.RetryPolicy{
    MaxAttempts: 3,  // 2 retries
    BaseDelay:   100 * time.Millisecond,
    MaxDelay:    1 * time.Second,
}

// Long-lived operations (batch processing)
RetryPolicy: &graph.RetryPolicy{
    MaxAttempts: 5,  // 4 retries
    BaseDelay:   1 * time.Second,
    MaxDelay:    30 * time.Second,
}
```

### 2. Use Appropriate BaseDelay

```go
// Fast retries (network operations)
BaseDelay: 100 * time.Millisecond

// Moderate retries (external APIs)
BaseDelay: 1 * time.Second

// Slow retries (batch jobs)
BaseDelay: 5 * time.Second
```

### 3. Set Reasonable MaxDelay

```go
// Prevent unbounded backoff
MaxDelay: 30 * time.Second  // Cap at 30 seconds

// Match timeout constraints
Timeout:  60 * time.Second
MaxDelay: 10 * time.Second  // Leave room for multiple attempts
```

### 4. Classify Errors Carefully

```go
Retryable: func(err error) bool {
    // Retry network errors
    if isNetworkError(err) {
        return true
    }

    // Retry specific HTTP status codes
    if httpErr, ok := err.(HTTPError); ok {
        return httpErr.StatusCode == 429 ||  // Too Many Requests
               httpErr.StatusCode == 503 ||  // Service Unavailable
               httpErr.StatusCode == 504     // Gateway Timeout
    }

    // Don't retry validation errors
    var validationErr ValidationError
    if errors.As(err, &validationErr) {
        return false
    }

    // Conservative default: don't retry unknown errors
    return false
}
```

### 5. Log Retry Attempts

```go
func (n MyNode) Run(ctx context.Context, state S) graph.NodeResult[S] {
    attempt := 0
    if attemptVal := ctx.Value(graph.AttemptKey); attemptVal != nil {
        attempt = attemptVal.(int)
    }

    if attempt > 0 {
        log.Printf("Retrying %s (attempt %d)", n.Name, attempt)
    }

    // ... node logic ...
}
```

### 6. Consider Total Workflow Time

```go
// Calculate maximum retry time
maxRetryTime := computeMaxRetryTime(
    maxAttempts:  5,
    baseDelay:    1 * time.Second,
    maxDelay:     30 * time.Second,
)
// Result: ~1s + 2s + 4s + 8s = ~15s (plus jitter)

// Ensure workflow timeout accommodates retries
Options{
    MaxSteps: 100,
    // Add buffer for retries across multiple nodes
}
```

## Related Documentation

- `graph.NodePolicy` - Per-node configuration
- `graph.RetryPolicy` - Retry policy structure
- `graph.Options.MaxConcurrentNodes` - Sequential vs concurrent mode
- `graph.ErrMaxAttemptsExceeded` - Retry exhaustion error
- `graph/engine.go:763-843` - Sequential retry implementation
- `graph/policy.go:113-136` - `computeBackoff()` function
- `examples/node_timeouts/` - Timeout configuration example

## Testing Retry Behavior

```go
func TestNodeRetries(t *testing.T) {
    attemptCount := 0
    node := NodeFunc[State](func(ctx context.Context, state State) NodeResult[State] {
        attemptCount++
        attempt := ctx.Value(graph.AttemptKey).(int)

        // Verify attempt counter matches
        if attempt != attemptCount-1 {
            t.Errorf("attempt mismatch: got %d, want %d", attempt, attemptCount-1)
        }

        // Fail first 2 attempts, succeed on 3rd
        if attemptCount < 3 {
            return NodeResult[State]{
                Err: TransientError{Message: "temporary failure"},
            }
        }

        return NodeResult[State]{
            Delta: State{Status: "success"},
            Route: graph.Stop(),
        }
    })

    // ... configure engine and run ...

    if attemptCount != 3 {
        t.Errorf("expected 3 attempts, got %d", attemptCount)
    }
}
```

## Troubleshooting

### Problem: Node retries forever

**Cause**: `MaxDelay` set to 0 (no cap) with high `MaxAttempts`

**Solution**: Set a reasonable `MaxDelay`:
```go
MaxDelay: 30 * time.Second,  // Cap at 30 seconds
```

### Problem: Retries happen for validation errors

**Cause**: `Retryable` predicate returns `true` for all errors

**Solution**: Classify errors properly:
```go
Retryable: func(err error) bool {
    var validationErr ValidationError
    if errors.As(err, &validationErr) {
        return false  // Don't retry validation errors
    }
    // ... classify other errors ...
}
```

### Problem: Backoff too short (thundering herd)

**Cause**: `BaseDelay` too small

**Solution**: Increase `BaseDelay` and `MaxDelay`:
```go
BaseDelay: 1 * time.Second,  // Start at 1 second
MaxDelay:  30 * time.Second, // Cap at 30 seconds
```

### Problem: Non-deterministic replay

**Cause**: Not using context RNG for random behavior

**Solution**: Use RNG from context:
```go
func (n MyNode) Run(ctx context.Context, state S) NodeResult[S] {
    // Get deterministic RNG
    rng := ctx.Value(graph.RNGKey).(*rand.Rand)

    // Use for random behavior
    randomValue := rng.Intn(100)

    // ... rest of logic ...
}
```

## Advanced Usage

### Custom Error Classification

```go
// Define error matcher interface
type Matcher interface {
    Matches(error) bool
}

// Implement matchers
type HTTPStatusMatcher struct {
    Codes []int
}

func (m HTTPStatusMatcher) Matches(err error) bool {
    if httpErr, ok := err.(HTTPError); ok {
        for _, code := range m.Codes {
            if httpErr.StatusCode == code {
                return true
            }
        }
    }
    return false
}

// Use in Retryable predicate
Retryable: func(err error) bool {
    retryableStatuses := HTTPStatusMatcher{
        Codes: []int{429, 503, 504},
    }
    return retryableStatuses.Matches(err)
}
```

### Circuit Breaker Pattern

```go
type CircuitBreakerNode struct {
    failures    int
    threshold   int
    resetTime   time.Time
}

func (n *CircuitBreakerNode) Run(ctx context.Context, state S) NodeResult[S] {
    // Check if circuit is open
    if n.failures >= n.threshold && time.Now().Before(n.resetTime) {
        return NodeResult[S]{
            Err: PermanentError{Message: "circuit breaker open"},
        }
    }

    // Attempt operation
    err := n.doWork()
    if err != nil {
        n.failures++
        n.resetTime = time.Now().Add(30 * time.Second)
        return NodeResult[S]{Err: err}
    }

    // Success - reset circuit
    n.failures = 0
    return NodeResult[S]{Route: graph.Stop()}
}
```

### Adaptive Retry Delays

```go
func (n AdaptiveNode) Policy() graph.NodePolicy {
    // Adjust delays based on recent performance
    baseDelay := n.calculateBaseDelay()

    return graph.NodePolicy{
        RetryPolicy: &graph.RetryPolicy{
            MaxAttempts: 5,
            BaseDelay:   baseDelay,
            MaxDelay:    30 * time.Second,
            Retryable:   n.classifyError,
        },
    }
}

func (n *AdaptiveNode) calculateBaseDelay() time.Duration {
    // Increase delay if seeing many failures
    if n.recentFailureRate() > 0.5 {
        return 2 * time.Second
    }
    return 500 * time.Millisecond
}
```
