# Data Model: Production Hardening

**Feature**: Production Hardening and Documentation Enhancements
**Date**: 2025-10-28
**Phase**: Phase 1 - Design

## Overview

This feature primarily enhances existing v0.2.0 data structures with better documentation and adds minimal new entities. Most data models already exist from feature 002.

---

## New Entities

### PrometheusMetrics

Prometheus metrics collector for runtime monitoring.

**Purpose**: Exposes workflow execution metrics via Prometheus for production monitoring and alerting.

**Fields**:
- `InflightNodesGauge`: Current number of executing nodes
- `QueueDepthGauge`: Current frontier queue depth
- `StepLatencyHistogram`: Distribution of step execution times (milliseconds)
- `RetriesTotalCounter`: Cumulative retry attempts
- `MergeConflictsTotalCounter`: Cumulative merge conflicts detected
- `BackpressureEventsTotalCounter`: Cumulative backpressure triggers

**Operations**:
- `RecordStepLatency(duration)`: Record step execution time
- `IncrementRetries()`: Increment retry counter
- `UpdateQueueDepth(depth)`: Update queue depth gauge
- `UpdateInflightNodes(count)`: Update active nodes gauge

**Relationships**:
- Used by Engine during execution
- Integrated with Emitter for event-driven updates
- Exported via HTTP endpoint for Prometheus scraping

---

### CostTracker

Tracks LLM API costs across workflow execution.

**Purpose**: Provides cost visibility for budget monitoring and optimization.

**Fields**:
- `TotalCostUSD`: Cumulative cost in US dollars
- `TokensIn`: Total input tokens consumed
- `TokensOut`: Total output tokens generated
- `CallsByModel`: Map of model name to call count

**Operations**:
- `RecordLLMCall(model, tokensIn, tokensOut)`: Track individual LLM call
- `GetTotalCost()`: Return cumulative cost
- `GetCostByModel(model)`: Return per-model cost breakdown

**Pricing**:
- Uses static pricing map (updateable via godoc)
- Supports custom pricing calculator override

---

## Enhanced Existing Entities

### Options (functional options added)

Existing struct enhanced with functional option support:

```go
// Existing approach (still supported)
opts := Options{MaxConcurrentNodes: 8, QueueDepth: 1024}

// New functional options approach
engine := New(reducer, store, emitter,
    WithMaxConcurrent(8),
    WithQueueDepth(1024),
    WithConflictPolicy(ConflictFail),
)
```

**Backward Compatibility**: Both patterns supported simultaneously.

---

### Store Interface (SQLite implementation added)

**New Implementation**: `SQLiteStore[S]` in `graph/store/sqlite.go`

**Same Interface**: Implements existing Store[S] interface
- SaveStep, LoadLatest
- SaveCheckpointV2, LoadCheckpointV2
- CheckIdempotency
- PendingEvents, MarkEventsEmitted

**SQLite-Specific**:
- Single file database
- WAL mode for concurrent reads
- Pragma settings for performance
- Auto-migration on first use

---

## Documentation Entities

These are conceptual entities documented but not coded:

### Ordering Contract

Mathematical specification of deterministic ordering:
- Formula: `order_key = SHA256(parent_path || node_id || edge_index)`
- Guarantees: Total ordering, collision resistance, determinism
- Application: Dispatch order and merge order both use ascending order_key

### Atomic Step Commit Contract

Transactional guarantee for exactly-once semantics:
- Components: State + Frontier + Outbox + Idempotency Key
- Atomicity: All-or-nothing commit in single database transaction
- Idempotency: Duplicate commits detected and rejected
- Crash Recovery: Uncommitted work lost, last committed state preserved

---

## Summary

Minimal new data structures (PrometheusMetrics, CostTracker), mostly enhancing existing entities with:
- Better documentation
- Additional implementations (SQLite store)
- Monitoring integration (Prometheus)
- Cost tracking (LLM costs)

All changes maintain backward compatibility with v0.2.0.
