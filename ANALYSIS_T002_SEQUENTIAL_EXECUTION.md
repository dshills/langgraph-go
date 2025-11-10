# Task T002: Sequential Execution Path Analysis

## Overview
This document provides a comprehensive analysis of the sequential execution logic in `graph/engine.go` and identifies where retry support needs to be added for task T006.

**Analysis Date:** November 10, 2025
**File:** `/Users/dshills/Development/projects/langgraph-go/graph/engine.go`

---

## Key Findings

### 1. Sequential Execution Entry Point

**Method:** `Engine[S].Run()`
**Location:** Lines 649-907 in `graph/engine.go`

The `Run()` method implements the main workflow execution orchestrator. It automatically routes to sequential or concurrent execution based on configuration:

```go
func (e *Engine[S]) Run(ctx context.Context, runID string, initial S) (S, error)
```

### 2. Sequential Execution Path Trigger

**Condition:** Lines 705-715
```go
// Initialize Frontier for concurrent execution if MaxConcurrentNodes > 0 (T034)
if e.opts.MaxConcurrentNodes > 0 {
    queueDepth := e.opts.QueueDepth
    if queueDepth == 0 {
        queueDepth = 1024 // Default queue depth
    }
    e.frontier = NewFrontier[S](ctx, queueDepth, runID, e.opts.Metrics, e.emitter)

    // Use concurrent execution path (T035)
    return e.runConcurrent(ctx, runID, initial)
}

// Initialize execution state (sequential execution path)
currentState := initial
currentNode := e.startNode
step := 0
```

**Key Point:** Sequential execution activates when `MaxConcurrentNodes == 0` (default backward-compatible mode)

### 3. Sequential Execution Loop Structure

**Location:** Lines 717-906

The sequential execution is implemented as a single `for` loop with the following flow:

```
for {
    step++
    → Check MaxSteps limit (T060)
    → Check context cancellation
    → Get current node implementation
    → Emit node_start event
    → Get node policy (retry/timeout config)
    → Execute with retry logic (LINES 763-832)
    → Merge state update
    → Persist state
    → Emit node_end event
    → Handle routing (Terminal/Many/To/Edge-based)
    → Continue to next iteration
}
```

### 4. Node Execution with Retry Support

**Method Location:** Lines 763-832
**Key Variable:** `maxRetries = e.opts.Retries`

The sequential path **already has retry support implemented**:

```go
// Execute node with retry support for sequential execution (US1: T005-T009)
var result NodeResult[S]
maxRetries := e.opts.Retries // Number of retry attempts (0 = no retries)

for attempt := 0; attempt <= maxRetries; attempt++ {
    // Add retry attempt to context for nodes to access
    attemptCtx := context.WithValue(ctx, AttemptKey, attempt)

    // Execute node with timeout enforcement (US2: T017, T018)
    var timeoutErr error
    result, timeoutErr = executeNodeWithTimeout(attemptCtx, nodeImpl, currentNode, currentState, policy, e.opts.DefaultNodeTimeout)
    if timeoutErr != nil {
        // Timeout occurred - treat as node error
        result.Err = timeoutErr
    }

    // If node succeeded, break out of retry loop
    if result.Err == nil {
        break
    }

    // Node failed - check if we should retry
    if attempt < maxRetries {
        // Compute exponential backoff with jitter (deterministic)
        // [backoff calculation code]
        
        // Apply backoff delay with context cancellation support
        select {
        case <-time.After(delay):
            // Backoff completed, continue to retry
        case <-ctx.Done():
            // Context canceled during backoff
            return zero, ctx.Err()
        }

        // Continue to next retry attempt
        continue
    }

    // Max retries exceeded - emit final error and return
    e.emitError(runID, currentNode, step-1, result.Err)
    return zero, result.Err
}
```

### 5. Node Execution Call Site

**Location:** Line 773 (wrapped by timeout enforcement)

```go
result, timeoutErr = executeNodeWithTimeout(attemptCtx, nodeImpl, currentNode, currentState, policy, e.opts.DefaultNodeTimeout)
```

**Underlying Call:** The `executeNodeWithTimeout` function in `graph/timeout.go` (lines 30-84) calls:
```go
result := node.Run(timeoutCtx, state)  // Line 70 in timeout.go
```

### 6. State Merge and Persistence

**Location:** Lines 834-846

After successful node execution (all retries succeeded):

```go
// Merge state update (only on success)
currentState = e.reducer(currentState, result.Delta)

// Persist state after node execution (T058)
if err := e.store.SaveStep(ctx, runID, step, currentNode, currentState); err != nil {
    return zero, &EngineError{
        Message: "failed to save step: " + err.Error(),
        Code:    "STORE_ERROR",
    }
}

// Emit node_end event with delta (T155)
e.emitNodeEnd(runID, currentNode, step-1, result.Delta)
```

### 7. Routing Decision Handling

**Location:** Lines 848-905

After successful execution, the engine determines the next node:

```go
// Determine next node from routing decision
if result.Route.Terminal {
    // Emit routing_decision event for Stop (T157)
    // Workflow complete
    return currentState, nil
}

// Handle parallel execution (fan-out) - T104-T108
if len(result.Route.Many) > 0 {
    // Execute branches in parallel with isolated state copies
    parallelState, err := e.executeParallel(ctx, result.Route.Many, currentState)
    if err != nil {
        return zero, err
    }
    currentState = parallelState
    return currentState, nil
}

if result.Route.To != "" {
    // Single next node (Goto)
    currentNode = result.Route.To
    continue
}

// If no explicit route, fall back to edge-based routing (T079)
nextNode := e.evaluateEdges(currentNode, currentState)
if nextNode == "" {
    // No matching edge found - workflow cannot continue
    return zero, &EngineError{
        Message: "no valid route from node: " + currentNode,
        Code:    "NO_ROUTE",
    }
}

currentNode = nextNode
continue
```

### 8. Error Handling Patterns

The sequential execution path includes several error handling patterns:

1. **Node Execution Errors (with Retries)**
   - Caught within retry loop (lines 767-832)
   - Exponential backoff applied before retry
   - Max retries enforced
   - Final error emitted when retries exhausted

2. **Timeout Errors**
   - Handled by `executeNodeWithTimeout` (timeout.go:48-84)
   - Treated as node error within retry loop
   - Timeout duration determined by precedence: NodePolicy.Timeout > DefaultNodeTimeout > 0

3. **Context Cancellation**
   - Checked at loop start (lines 735-739)
   - Checked during backoff delay (line 820)
   - Propagated immediately as error

4. **Store/Persistence Errors**
   - Fatal errors that stop execution (lines 838-843)
   - No retry logic (by design)

5. **Routing Errors**
   - "NO_ROUTE" error when no valid next node found (lines 892-895)

### 9. Current Retry Infrastructure

The sequential execution already leverages:

**From `policy.go`:**
- `NodePolicy` struct with `Timeout` field
- `RetryPolicy` struct (MaxAttempts, BaseDelay, MaxDelay, Retryable predicate)
- `computeBackoff()` function for deterministic exponential backoff
- `RetryPolicy.Validate()` for configuration validation

**From `engine.go`:**
- `e.opts.Retries` - global retry count
- `AttemptKey` - context key for passing attempt number
- `RNGKey` - context key for deterministic random number generation

**From `timeout.go`:**
- `getNodeTimeout()` - timeout precedence logic
- `executeNodeWithTimeout()` - wrapper for node execution with timeout

### 10. Differences from Concurrent Execution

**Concurrent Path** (`runConcurrent`, lines 952+):
- Uses worker goroutines bounded by `MaxConcurrentNodes`
- Uses Frontier scheduler for work distribution
- Retry logic on lines 1136-1150 (parallel structure)
- Similar timeout enforcement and backoff

**Sequential Path**:
- Single-threaded loop
- Cleaner state management (no branching)
- Direct state mutations
- Simpler error propagation

Both paths implement retry logic identically with respect to:
- Backoff calculation
- Jitter application
- Retry attempt tracking
- Timeout enforcement

---

## Summary for T006 Implementation

### Existing Foundation
The sequential execution path in `Engine[S].Run()` **already has complete retry support**:
- Lines 763-832: Full retry loop with exponential backoff
- Timeout enforcement via `executeNodeWithTimeout()`
- Deterministic jitter using seeded RNG
- Node policy support for per-node configuration

### What T006 Needs to Address
Since the basic sequential retry logic is complete, T006 should focus on:

1. **Per-Node Retry Policy Support**
   - Use `NodePolicy.RetryPolicy` instead of global `e.opts.Retries`
   - Implement `RetryPolicy.Retryable()` predicate logic
   - Support per-node MaxAttempts and backoff configuration

2. **Policy Extraction** (already partially done at line 758)
   ```go
   var policy *NodePolicy
   if policyProvider, ok := nodeImpl.(interface{ Policy() NodePolicy }); ok {
       p := policyProvider.Policy()
       policy = &p
   }
   ```

3. **Policy-Based Retry Decision**
   - Check if error is retryable via `policy.RetryPolicy.Retryable()`
   - Use policy-specific MaxAttempts
   - Use policy-specific backoff parameters

4. **Idempotency Support**
   - Implement idempotency key generation from `policy.IdempotencyKeyFunc`
   - Check store for duplicate execution
   - Return cached result if already executed

### Code Locations for T006 Changes
- **Lines 765-766**: Replace global `e.opts.Retries` with policy-based logic
- **Lines 767-832**: Extend retry loop to use `policy.RetryPolicy` fields
- **Line 773**: Add policy-based decision before calling `executeNodeWithTimeout`
- **New Logic Needed**: Idempotency key check before node.Run()

---

## Appendix: Key Interfaces and Types

### NodePolicy (policy.go:16-29)
```go
type NodePolicy struct {
    Timeout time.Duration
    RetryPolicy *RetryPolicy
    IdempotencyKeyFunc func(state any) string
}
```

### RetryPolicy (policy.go:36-56)
```go
type RetryPolicy struct {
    MaxAttempts int
    BaseDelay time.Duration
    MaxDelay time.Duration
    Retryable func(error) bool
}
```

### NodeResult[S] (node.go)
```go
type NodeResult[S] struct {
    Delta S              // Partial state update
    Route Next           // Next hop(s)
    Events []Event       // Observability events
    Err error           // Node-level error
}
```

### Options.Retries (options.go)
- Global default retry count
- Used when no per-node policy defined
- Can be 0 (no retries) to any positive integer

---

## References
- Engine implementation: `/Users/dshills/Development/projects/langgraph-go/graph/engine.go` (lines 649-907)
- Timeout logic: `/Users/dshills/Development/projects/langgraph-go/graph/timeout.go` (lines 30-84)
- Policy definitions: `/Users/dshills/Development/projects/langgraph-go/graph/policy.go` (full file)
- Node interface: `/Users/dshills/Development/projects/langgraph-go/graph/node.go`
