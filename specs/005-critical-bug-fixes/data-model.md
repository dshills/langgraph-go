# Data Model: Critical Concurrency Bug Fixes

**Feature**: 005-critical-bug-fixes  
**Date**: 2025-10-29

## N/A - No Data Model Changes

**Justification**: This feature consists of internal bug fixes to concurrent execution logic. No new entities are introduced, and existing entities (WorkItem, nodeResult, Frontier) are modified internally without schema changes.

### Existing Entities (No Changes)

These entities remain unchanged in their public interfaces:

- **WorkItem[S]**: Work unit in frontier queue (internal fields may change)
- **nodeResult[S]**: Node execution outcome (internal only)
- **Frontier[S]**: Priority queue scheduler (internal synchronization changes)
- **Engine[S]**: Workflow execution engine (no public API changes)

### Internal Modifications

The following internal structures will be modified:

1. **Per-Worker RNG State**: Each worker goroutine maintains its own `*rand.Rand` instance (not exposed publicly)
2. **Results Channel Capacity**: Increased from MaxConcurrentNodes to MaxConcurrentNodes*2 (internal implementation detail)
3. **Frontier Synchronization**: Heap and channel coordination pattern (internal to scheduler.go)
4. **Completion State**: Atomic flags for race-free detection (internal to engine.go)

None of these changes affect public APIs or user-facing data structures.

---

**Conclusion**: No data-model.md artifact needed for this feature. All changes are internal implementation fixes maintaining existing interfaces and behaviors.
