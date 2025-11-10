# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**LangGraph-Go** is a Go-native orchestration framework for building stateful, graph-based LLM and tool workflows. It models reasoning pipelines as graphs of nodes where each node represents a computation step (LLM call, tool, or logic block), and data/state flows through edges between them.

This project is currently in the specification phase and uses the Specify framework for implementation planning.

## Core Architecture

### Graph-Based Execution Model

The framework centers around a directed graph where:
- **Nodes**: Processing units (LLM calls, tools, or functions) that implement `Node[S any]` interface
- **Edges**: Define control flow between nodes with optional predicates
- **State**: Shared context (generic type `S`) that evolves through node outputs
- **Reducers**: Functions of type `func(prev S, delta S) S` that merge partial state changes deterministically
- **Engine**: Orchestrates execution, handles routing, persistence, and observability

### State Management

State is strongly-typed using Go generics. Each node returns a `NodeResult[S]` containing:
- `Delta S`: Partial state update to be merged via the reducer
- `Route Next`: Next hop(s) for execution (supports `Goto(nodeID)`, `Stop()`, or fan-out via `Many []string`)
- `Events []Event`: Observability events
- `Err error`: Node-level error handling

The reducer function merges deltas into accumulated state, enabling deterministic replay and resumable execution.

### Package Structure

```
/graph
  engine.go        // Engine, graph wiring, execution runner
  node.go          // Node interface, NodeFunc, Next, Edge
  state.go         // Reducer definitions and helpers
  store/
    memory.go      // In-memory store (for testing)
    mysql.go       // Aurora/MySQL persistence implementation
  emit/
    log.go         // Stdout logger
    otel.go        // OpenTelemetry emitter
  model/
    chat.go        // ChatModel interface + adapters
    openai.go      // OpenAI adapter
    ollama.go      // Local LLM adapter
  tool/
    tool.go        // Tool interface
    http.go        // Example HTTP tool
```

## Development Commands

### Codebase Navigation

**gocontext** is the primary tool for exploring this codebase:

```bash
# Index the codebase (run at project start or after major changes)
mcp__gocontext__index_codebase(path="/Users/dshills/Development/projects/langgraph-go", include_tests=true)

# Search code with natural language queries
mcp__gocontext__search_code(path="/Users/dshills/Development/projects/langgraph-go", query="error handling patterns")

# Search for DDD patterns (aggregates, entities, repositories, etc.)
mcp__gocontext__search_code(
  path="/Users/dshills/Development/projects/langgraph-go",
  query="repository pattern",
  filters={"ddd_patterns": ["repository"]}
)

# Check indexing status
mcp__gocontext__get_status(path="/Users/dshills/Development/projects/langgraph-go")
```

Use gocontext for exploratory code search instead of grep/ripgrep. It provides semantic search with natural language queries, DDD pattern filtering, and context-aware results.

### Go Tooling
```bash
# Build the project
go build ./...

# Run tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run a specific test
go test -v -run TestName ./path/to/package

# Format code
go fmt ./...

# Run linter (if golangci-lint is available)
golangci-lint run

# Run security checks (if gosec is available)
gosec ./...

# Tidy dependencies
go mod tidy

# Vendor dependencies
go mod vendor
```

### Specify Framework Commands

This project uses the Specify framework for specification-driven development:

```bash
# Create or update feature specification
/speckit.specify

# Generate implementation plan
/speckit.plan

# Generate task breakdown
/speckit.tasks

# Execute implementation from tasks.md
/speckit.implement

# Analyze cross-artifact consistency
/speckit.analyze

# Generate custom checklist
/speckit.checklist

# Ask clarification questions
/speckit.clarify
```

## Design Principles

1. **Deterministic Replay**: Every run can be resumed or re-simulated from checkpoints
2. **Type Safety**: Strongly-typed state management using Go generics
3. **Low Dependencies**: Pure Go core with optional external adapters
4. **Composable**: Support for loops, branches, and fan-out patterns
5. **Production-Ready**: Built-in checkpointing, persistence, and observability

## Key Interfaces

### Node Interface
```go
type Node[S any] interface {
    Run(ctx context.Context, state S) NodeResult[S]
}
```

Nodes can be implemented as structs with the `Run` method or as `NodeFunc[S]` for function-based nodes.

### Store Interface
```go
type Store[S any] interface {
    SaveStep(ctx context.Context, runID string, step int, nodeID string, state S) error
    LoadLatest(ctx context.Context, runID string) (state S, step int, nodeID string, _ error)
    SaveCheckpoint(ctx context.Context, runID, label string, state S, step int, nodeID string) error
    LoadCheckpoint(ctx context.Context, runID, label string) (state S, step int, nodeID string, _ error)
}
```

Enables persistent execution with resumption from checkpoints.

### ChatModel Interface
```go
type ChatModel interface {
    Chat(ctx context.Context, messages []Message, tools []ToolSpec) (ChatOut, error)
}
```

Abstraction for LLM providers (OpenAI, Anthropic, Ollama, Google).

### Tool Interface
```go
type Tool interface {
    Name() string
    Call(ctx context.Context, input any) (any, error)
}
```

## Testing Strategy

- Use `NewMemStore[S]()` for in-memory state during unit tests
- Test node logic independently with mocked state
- Test graph execution flows with small state types
- Verify reducer functions merge state correctly
- Test error handling and retry logic
- Validate checkpoint save/restore cycles

## LLM Integration

The project depends on official SDK clients:
- `github.com/anthropics/anthropic-sdk-go` - Anthropic Claude
- `github.com/openai/openai-go` - OpenAI GPT models
- `github.com/google/generative-ai-go` - Google Gemini

Adapters implement the `ChatModel` interface to provide a unified API across providers.

## Concurrency Model

Nodes can return multiple next hops via `Next{Many: []string{...}}` to enable parallel execution. Branches execute concurrently with isolated state copies and merge at a join node using the reducer function.

## Node Configuration & Timeouts

### Per-Node Timeouts

Nodes can configure execution timeouts via the optional `Policy()` method:

```go
type MyNode struct{}

func (n MyNode) Run(ctx context.Context, state S) NodeResult[S] {
    // Node implementation
}

func (n MyNode) Policy() NodePolicy {
    return NodePolicy{
        Timeout: 30 * time.Second, // Per-node timeout
    }
}
```

### Timeout Precedence

Timeout enforcement follows this precedence order:

1. **Per-Node Timeout** (highest priority)
   - Set via `NodePolicy.Timeout` on individual nodes
   - Overrides `DefaultNodeTimeout` and global timeouts
   - Example: `Policy().Timeout = 2 * time.Minute`

2. **DefaultNodeTimeout** (fallback)
   - Set via `Options.DefaultNodeTimeout` when creating the engine
   - Used when a node's `Policy().Timeout` is zero
   - Example: `Options{DefaultNodeTimeout: 30 * time.Second}`

3. **No Timeout** (when both are zero)
   - Node can run indefinitely
   - Use cautiously to avoid hanging workflows

### Timeout Error Handling

When a node exceeds its timeout:
- Returns `context.DeadlineExceeded` error
- Error wrapped as `EngineError` with code `NODE_TIMEOUT`
- Node's `Run()` method receives cancellation via `ctx.Done()`
- Other nodes are not affected (timeout is per-node)

### Example

See `examples/node_timeouts/` for a complete demonstration of per-node timeout configuration.

## Backpressure & Queue Management

### Overview

Backpressure occurs when the execution frontier's work queue reaches its configured capacity. This is a normal operating condition that prevents unbounded memory growth when node execution latency exceeds task enqueueing rate. The framework automatically handles backpressure by blocking further task submission until queue space becomes available.

### Queue Capacity Configuration

Queue capacity is configured via the `WithQueueDepth(n)` option when creating the engine:

```go
engine := New(
    reducer, store, emitter,
    WithMaxConcurrent(16),
    WithQueueDepth(2048),  // Set queue capacity to 2048 items
)
```

Default queue depth is 1024 items. Higher values consume more memory but tolerate larger bursts of concurrent node submissions. Lower values apply stricter backpressure at lower queue utilization.

### Backpressure Metrics

When backpressure occurs, the framework emits metrics to track queue health:

```go
// Prometheus metric incremented when queue reaches capacity
metrics.IncrementBackpressure(runID, "queue_full")

// Tracks number of times backpressure condition was triggered
// Use this to identify sustained capacity issues
```

Monitor this metric to detect bottlenecks:
- **Frequent backpressure**: Indicates nodes are not draining the queue fast enough. Consider increasing `MaxConcurrent` or `QueueDepth`
- **Rare backpressure**: Normal behavior; system is responding to load spikes
- **Persistent backpressure**: Node execution may have degraded latency; investigate node logs

### Backpressure Events

Two observability events mark the backpressure lifecycle:

**"backpressure" Event** (emitted when queue reaches capacity):
```json
{
  "msg": "backpressure",
  "meta": {
    "queue_depth": 2048,
    "capacity": 2048,
    "node_id": "process_step",
    "order_key": "item-123"
  }
}
```

**"backpressure_resolved" Event** (emitted when queue frees up after wait):
```json
{
  "msg": "backpressure_resolved",
  "meta": {
    "wait_duration_ms": 42,
    "queue_depth": 2048
  }
}
```

Wait duration is only emitted if the wait exceeded 1 millisecond. Very fast queue drains may not emit this event.

### Monitoring Recommendations

**In Development:**
- Log backpressure events at INFO level to identify queue saturation patterns
- Monitor `queue_depth` in "backpressure" events to see peak queue utilization
- Check `wait_duration_ms` in resolved events to quantify blocking impact

**In Production:**
- Set up Prometheus alerting on `backpressure_total` metric:
  ```
  rate(langgraph_backpressure_total[1m]) > 10  # More than 10 backpressures per minute
  ```
- Track max queue depth during normal operations; if approaching configured capacity, increase `QueueDepth`
- Monitor correlation between backpressure events and node latency spikes
- Use wait durations to quantify impact: high wait durations indicate resource contention

### Tuning Strategy

1. **Monitor baseline behavior**: Run with default `QueueDepth(1024)` and observe backpressure metrics
2. **Identify bottleneck**: Check if backpressure is caused by:
   - Slow node execution: Increase `DefaultNodeTimeout` to identify timeouts
   - Insufficient concurrency: Try increasing `WithMaxConcurrent(n)`
   - Queue saturation: Increase `WithQueueDepth(n)`
3. **Test changes in staging**: Always validate tuning with realistic load before production deployment
4. **Monitor after changes**: Ensure tuning reduces backpressure frequency without increasing memory consumption

### Example Configuration

```go
// Conservative: Lower latency, more frequent backpressure
engine := New(
    reducer, store, emitter,
    WithQueueDepth(512),        // Smaller queue
    WithMaxConcurrent(8),       // Strict concurrency
    WithBackpressureTimeout(5 * time.Second),
)

// Aggressive: Higher memory, tolerates bursts
engine := New(
    reducer, store, emitter,
    WithQueueDepth(4096),       // Larger queue
    WithMaxConcurrent(32),      // More parallel nodes
    WithBackpressureTimeout(30 * time.Second),
)
```

## Error Handling

- Node errors (`NodeResult.Err`) trigger retry logic or route to error handling nodes
- Retry attempts configurable via `Options.Retries`
- `LastError` field in state enables downstream error handling logic
- Engine enforces `MaxSteps` limit to prevent infinite loops

## Reference Documentation

- Full specification: `specs/SPEC.md`
- Specify templates: `.specify/templates/`
- Project constitution: `.specify/memory/constitution.md` (currently template only)

## Active Technologies
- Go 1.21+ (requires generics support) + Go standard library only (core framework), optional adapters for OpenTelemetry SDK, MySQL driver (002-concurrency-spec)
- Store interface supports in-memory (testing) and MySQL/Aurora (production) implementations (002-concurrency-spec)
- Go 1.21+ (requires generics, math/rand, sync/atomic) (005-critical-bug-fixes)
- N/A (bug fixes in execution engine, not persistence layer) (005-critical-bug-fixes)

## Recent Changes
- 002-concurrency-spec: Added Go 1.21+ (requires generics support) + Go standard library only (core framework), optional adapters for OpenTelemetry SDK, MySQL driver
