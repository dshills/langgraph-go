# Quickstart: Production Hardening Features

**Feature**: Production Hardening
**Time**: 10 minutes

## SQLite Store (Zero Configuration)

```go
import "github.com/dshills/langgraph-go/graph/store"

// Create SQLite store (file created automatically)
sqliteStore, err := store.NewSQLiteStore[MyState]("./dev.db")
if err != nil {
    log.Fatal(err)
}

engine := graph.New(reducer, sqliteStore, emitter, opts)
// Checkpoints automatically persisted to dev.db
```

## Functional Options

```go
// Old way (still works)
opts := graph.Options{
    MaxConcurrentNodes: 16,
    QueueDepth: 2048,
}
engine := graph.New(reducer, store, emitter, opts)

// New way (more ergonomic)
engine := graph.New(reducer, store, emitter,
    graph.WithMaxConcurrent(16),
    graph.WithQueueDepth(2048),
    graph.WithConflictPolicy(graph.ConflictFail),
)
```

## Prometheus Metrics

```go
import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics auto-registered during engine creation
http.Handle("/metrics", promhttp.Handler())
go http.ListenAndServe(":9090", nil)

// Metrics available:
// - langgraph_inflight_nodes
// - langgraph_queue_depth
// - langgraph_step_latency_ms
// - langgraph_retries_total
// - langgraph_merge_conflicts_total
// - langgraph_backpressure_events_total
```

## Cost Tracking

```go
// Costs automatically tracked in OpenTelemetry spans
// Access via span attributes:
// - tokens_in: input token count
// - tokens_out: output token count
// - cost_usd: calculated cost

// Query total cost from OTel backend (Jaeger, Tempo, etc.)
```

## Typed Error Handling

```go
import "errors"

finalState, err := engine.Run(ctx, runID, initial)
if err != nil {
    switch {
    case errors.Is(err, graph.ErrReplayMismatch):
        // Handle replay mismatch
    case errors.Is(err, graph.ErrNoProgress):
        // Handle deadlock
    case errors.Is(err, graph.ErrBackpressure):
        // Handle backpressure timeout
    default:
        // Handle other errors
    }
}
```
