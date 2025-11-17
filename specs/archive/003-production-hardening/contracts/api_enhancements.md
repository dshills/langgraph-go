# API Enhancements Contract

**Feature**: Production Hardening
**Package**: `github.com/dshills/langgraph-go/graph`

## Functional Options Pattern

```go
type Option func(*Engine[S]) error

// Concurrency options
func WithMaxConcurrent(n int) Option
func WithQueueDepth(n int) Option  
func WithBackpressureTimeout(d time.Duration) Option

// Timeout options
func WithDefaultNodeTimeout(d time.Duration) Option
func WithRunWallClockBudget(d time.Duration) Option

// Replay options
func WithReplayMode(enabled bool) Option
func WithStrictReplay(enabled bool) Option

// Policy options
func WithConflictPolicy(policy ConflictPolicy) Option

// Usage
engine := graph.New(reducer, store, emitter,
    graph.WithMaxConcurrent(16),
    graph.WithQueueDepth(2048),
    graph.WithConflictPolicy(graph.ConflictFail),
)
```

## Typed Errors

```go
// Exported errors for user error handling
var (
    ErrReplayMismatch    error // Replay I/O mismatch
    ErrNoProgress        error // Deadlock detection
    ErrBackpressure      error // Queue timeout
    ErrMaxStepsExceeded  error // Step limit
    ErrIdempotencyViolation error // Duplicate commit
    ErrMaxAttemptsExceeded error // Retry limit
)

// Usage
if errors.Is(err, graph.ErrReplayMismatch) {
    // Handle replay mismatch
}
```

## Prometheus Metrics

```go
// Metric names (langgraph_* namespace)
const (
    MetricInflightNodes       = "langgraph_inflight_nodes"
    MetricQueueDepth          = "langgraph_queue_depth"
    MetricStepLatencyMs       = "langgraph_step_latency_ms"
    MetricRetries             = "langgraph_retries_total"
    MetricMergeConflicts      = "langgraph_merge_conflicts_total"
    MetricBackpressureEvents  = "langgraph_backpressure_events_total"
)
```

## SQLite Store

```go
// New store implementation
func NewSQLiteStore[S any](dbPath string) (*SQLiteStore[S], error)

// Usage
store, err := store.NewSQLiteStore[MyState]("./workflow.db")
engine := graph.New(reducer, store, emitter, opts)
```
