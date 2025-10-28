# Conflict Resolution Policies

LangGraph-Go provides configurable conflict resolution strategies for handling concurrent state updates in multi-branch workflows. This guide explains when conflicts occur, available policies, and how to choose the right strategy for your use case.

## Table of Contents

- [Overview](#overview)
- [When Conflicts Occur](#when-conflicts-occur)
- [Built-in Policies](#built-in-policies)
  - [ConflictFail (Default)](#conflictfail-default)
  - [LastWriterWins](#lastwriterwins)
- [Choosing a Policy](#choosing-a-policy)
- [Custom Conflict Handlers](#custom-conflict-handlers)
- [Future: CRDT Integration](#future-crdt-integration)
- [Best Practices](#best-practices)

## Overview

**Conflict resolution** determines how the engine handles simultaneous state updates from parallel branches. When multiple nodes complete and merge their deltas, the conflict policy decides how to reconcile overlapping changes.

**Key Concepts:**

- **State Delta**: Partial state update returned by a node
- **Reducer**: Function that merges deltas into accumulated state
- **Conflict**: When multiple deltas modify the same state field
- **Policy**: Strategy for resolving conflicts deterministically

## When Conflicts Occur

Conflicts arise in parallel execution scenarios:

### Example: Parallel Data Collection

```go
// Graph with 3 parallel branches fetching data
engine.Add("start", startNode)
engine.Add("fetchA", fetchANode)  // Updates state.DataA
engine.Add("fetchB", fetchBNode)  // Updates state.DataB
engine.Add("fetchC", fetchCNode)  // Updates state.DataA (conflict!)
engine.Add("merge", mergeNode)

engine.StartAt("start")
engine.Connect("start", "fetchA")
engine.Connect("start", "fetchB")
engine.Connect("start", "fetchC")
engine.Connect("fetchA", "merge")
engine.Connect("fetchB", "merge")
engine.Connect("fetchC", "merge")
```

**Conflict occurs if:**
- `fetchA` sets `state.DataA = "value-from-A"`
- `fetchC` sets `state.DataA = "value-from-C"`
- Both deltas arrive at reducer simultaneously

**How engine handles it:**
1. Engine detects overlapping field updates
2. Applies configured conflict policy
3. Merges deltas deterministically based on order keys

## Built-in Policies

### ConflictFail (Default)

**Behavior**: Raises an error when conflicting updates are detected.

**Use When:**
- Conflicts indicate bugs in workflow design
- You want strict validation of parallel logic
- State correctness is critical (financial, legal)

**Example:**

```go
// Configure ConflictFail (default)
opts := graph.Options{
    ConflictPolicy: graph.ConflictFail,
}

engine := graph.New(reducer, store, emitter, opts)

// Execution with conflict
// Node A returns: Delta{Counter: 10}
// Node B returns: Delta{Counter: 20}
// Result: Error "conflict detected: field 'Counter' updated by multiple branches"
```

**Error Details:**

```go
var ErrMergeConflict = errors.New("merge conflict detected")

// Error includes:
// - Conflicting field name
// - Node IDs that caused conflict
// - Order keys of conflicting deltas
// - Original values
```

**Benefits:**
- ✅ Fail-fast on design errors
- ✅ Prevents silent data corruption
- ✅ Forces explicit conflict handling

**Drawbacks:**
- ❌ Requires careful workflow design
- ❌ May reject valid concurrent updates

### LastWriterWins

**Behavior**: Accepts the delta with the highest order key (last in deterministic order).

**Use When:**
- Conflicts are acceptable and expected
- Latest value is always preferred (caching, status updates)
- Workflow designed for idempotent updates

**Example:**

```go
// Configure LastWriterWins
opts := graph.Options{
    ConflictPolicy: graph.LastWriterWins,
}

engine := graph.New(reducer, store, emitter, opts)

// Execution with conflict
// Node A (order_key: 0x1234) returns: Delta{Status: "processing"}
// Node B (order_key: 0x5678) returns: Delta{Status: "complete"}
// Result: state.Status = "complete" (higher order key wins)
```

**Determinism Guarantee:**

Order keys are deterministically computed:
```
order_key = SHA256(parent_path || node_id || edge_index)
```

Same graph topology always produces same order keys, so "last writer" is predictable.

**Benefits:**
- ✅ Allows parallel updates to same fields
- ✅ Deterministic based on topology
- ✅ Simple to understand and debug

**Drawbacks:**
- ❌ Losing values may contain important data
- ❌ No semantic merge logic
- ❌ Requires careful node design

**Example Use Case: Status Aggregation**

```go
type WorkflowState struct {
    Status   string
    Progress int
    Errors   []string
}

func reducer(prev, delta WorkflowState) WorkflowState {
    // LastWriterWins for Status and Progress
    if delta.Status != "" {
        prev.Status = delta.Status
    }
    if delta.Progress > 0 {
        prev.Progress = delta.Progress
    }

    // Append for Errors (no conflict)
    prev.Errors = append(prev.Errors, delta.Errors...)
    return prev
}

// With LastWriterWins policy:
// - Status updates don't fail
// - Latest status reflects final state
// - Errors accumulate without conflicts
```

## Choosing a Policy

Use this decision tree:

```
┌─────────────────────────────────────────────┐
│ Are concurrent updates to same field valid? │
└────────────────┬────────────────────────────┘
                 │
        ┌────────┴────────┐
        │                 │
       YES               NO
        │                 │
        ↓                 ↓
┌────────────────┐  ┌─────────────┐
│ LastWriterWins │  │ ConflictFail│
└────────────────┘  └─────────────┘
        │                 │
        ↓                 ↓
Does order matter?    Design workflow to
        │             avoid conflicts
   ┌────┴────┐
   │         │
  YES       NO
   │         │
   ↓         ↓
Design     Use
edges      separate
properly   fields
```

### Policy Selection Guide

| Scenario | Policy | Reason |
|----------|--------|--------|
| Financial transactions | `ConflictFail` | Correctness critical |
| Status updates (idempotent) | `LastWriterWins` | Latest status preferred |
| Counter increments | Custom reducer | Need additive merge |
| Data aggregation | `ConflictFail` | Ensure all data captured |
| Cache updates | `LastWriterWins` | Latest value acceptable |
| Error collection | Use arrays + append | Avoid conflicts entirely |
| Configuration merging | Custom reducer | Semantic merge required |

## Custom Conflict Handlers

For complex conflict resolution, implement custom logic in your reducer:

### Example: Additive Counter Merge

```go
type State struct {
    Counter int
    Data    map[string]string
}

func additiveReducer(prev, delta State) State {
    // Additive merge for Counter (no conflict)
    prev.Counter += delta.Counter

    // Map merge (LastWriterWins for keys)
    if prev.Data == nil {
        prev.Data = make(map[string]string)
    }
    for k, v := range delta.Data {
        prev.Data[k] = v
    }

    return prev
}

// With ConflictFail policy:
// - Counter updates never conflict (additive)
// - Map key updates handled by reducer
// - Other field conflicts still detected
```

### Example: Semantic Merge with Priorities

```go
type Priority int

const (
    PriorityLow Priority = 1
    PriorityMed Priority = 5
    PriorityHigh Priority = 10
)

type State struct {
    Message  string
    Priority Priority
}

func priorityReducer(prev, delta State) State {
    // Keep message with highest priority
    if delta.Priority > prev.Priority {
        prev.Message = delta.Message
        prev.Priority = delta.Priority
    }
    return prev
}

// High-priority branches always win
// No conflicts, semantic merge
```

### Example: CRDT-style Set Merge

```go
type State struct {
    Tags map[string]bool // Set of unique tags
}

func setMergeReducer(prev, delta State) State {
    // Union merge - no conflicts
    if prev.Tags == nil {
        prev.Tags = make(map[string]bool)
    }
    for tag := range delta.Tags {
        prev.Tags[tag] = true
    }
    return prev
}

// All tag additions preserved
// No LastWriterWins needed
```

## Future: CRDT Integration

**Planned Feature (v0.4.0)**: Native support for Conflict-free Replicated Data Types (CRDTs).

### Vision

```go
// Future API (not yet implemented)
import "github.com/dshills/langgraph-go/crdt"

type State struct {
    Counter crdt.GCounter  // Grow-only counter
    Set     crdt.ORSet     // Observed-remove set
    Map     crdt.LWWMap    // Last-write-wins map
}

opts := graph.Options{
    ConflictPolicy: graph.CRDTPolicy,
}

// Automatic conflict-free merging
// No manual reducer logic needed
```

### CRDT Types Planned

- **GCounter**: Grow-only counter (additive merge)
- **PNCounter**: Positive-negative counter (supports decrement)
- **GSet**: Grow-only set (union merge)
- **ORSet**: Observed-remove set (add/remove operations)
- **LWWMap**: Last-write-wins map (timestamp-based)
- **MVRegister**: Multi-value register (keeps all concurrent values)

### Use Cases

- Distributed analytics with additive counters
- Tag collections with set merging
- Collaborative editing with last-write-wins
- Event logs with append-only semantics

**Status**: Under design - feedback welcome on GitHub issues!

## Best Practices

### 1. Design for Conflict Avoidance

**✅ Good: Independent Fields**

```go
type State struct {
    DataA string  // Updated only by fetchA
    DataB string  // Updated only by fetchB
    DataC string  // Updated only by fetchC
}

// No conflicts - each branch owns its field
```

**❌ Bad: Shared Field**

```go
type State struct {
    Result string  // Updated by all branches - conflicts!
}

// Requires conflict policy or redesign
```

### 2. Use Arrays for Multi-Source Data

**✅ Good: Append to Array**

```go
type State struct {
    Results []string
}

func reducer(prev, delta State) State {
    prev.Results = append(prev.Results, delta.Results...)
    return prev
}

// All results preserved, no conflicts
```

**❌ Bad: Single Value**

```go
type State struct {
    Result string  // Only one branch wins
}
```

### 3. Start with ConflictFail

Begin with strict validation:

```go
// Development
opts := graph.Options{
    ConflictPolicy: graph.ConflictFail,
}

// Catches design issues early
// Refine after understanding conflicts
```

If conflicts occur, decide:
1. Redesign workflow to avoid conflicts
2. Switch to `LastWriterWins` if acceptable
3. Implement custom reducer logic

### 4. Test Conflict Scenarios

```go
func TestConflictResolution(t *testing.T) {
    // Arrange: Graph with conflicting updates
    engine := setupEngineWithConflict()

    // Act: Execute workflow
    final, err := engine.Run(ctx, "test-run", initialState)

    // Assert: Verify policy behavior
    if opts.ConflictPolicy == graph.ConflictFail {
        assert.Error(t, err)
        assert.ErrorIs(t, err, graph.ErrMergeConflict)
    } else {
        assert.NoError(t, err)
        // Verify LastWriterWins produced expected result
        assert.Equal(t, expectedValue, final.Field)
    }
}
```

### 5. Document Conflict Decisions

In production workflows, document why conflicts are acceptable:

```go
// ConflictPolicy: LastWriterWins
// Rationale: Status field tracks final state only.
// Intermediate statuses from parallel branches are
// not critical - latest status reflects true state.
// Tested: test_status_updates_parallel.go
opts := graph.Options{
    ConflictPolicy: graph.LastWriterWins,
}
```

## Related Documentation

- [State Management Guide](./guides/03-state-management.md) - Reducer patterns
- [Parallel Execution Guide](./guides/06-parallel.md) - Fan-out/fan-in patterns
- [Concurrency Model](./concurrency.md) - Deterministic ordering
- [Determinism Guarantees](./determinism-guarantees.md) - Order key computation

## Summary

**Conflict policies provide predictable behavior for parallel state updates:**

| Policy | Behavior | Use Case |
|--------|----------|----------|
| `ConflictFail` | Error on conflict | Strict validation, critical correctness |
| `LastWriterWins` | Highest order key wins | Idempotent updates, status tracking |
| Custom Reducer | Application logic | Semantic merge, additive operations |

**Recommendations:**

1. ✅ Start with `ConflictFail` in development
2. ✅ Design workflows to avoid conflicts when possible
3. ✅ Use arrays/maps for multi-source data
4. ✅ Switch to `LastWriterWins` only when acceptable
5. ✅ Implement custom reducers for complex merge logic
6. ✅ Test conflict scenarios explicitly
7. ✅ Document conflict resolution decisions

Proper conflict policy selection ensures deterministic, predictable behavior in concurrent workflows while maintaining data integrity.
