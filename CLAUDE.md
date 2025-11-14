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

## AWS Bedrock LLM Integration

The AWS Bedrock adapter provides seamless integration with AWS Bedrock's LLM offerings, including Claude models via Amazon Bedrock. The adapter supports standard chat operations, streaming responses, multi-region failover, and tool calling.

### Basic Setup Example

```go
import "github.com/dshills/langgraph-go/graph/model/bedrock"

config := bedrock.Config{
    Region:  "us-east-1",
    ModelID: "us.anthropic.claude-3-5-sonnet-20241022-v2:0",  // Use inference profile
    MaxTokens: 4096,
}

adapter, err := bedrock.NewAdapter(context.Background(), config)
if err != nil {
    log.Fatal(err)
}
```

### Model Access Requirements

**Important**: AWS Bedrock requires accounts to request model access before invoking models:

1. **Request Model Access**: Navigate to AWS Bedrock console → Model access → Request access
2. **Fill Use Case Form**: For Claude models, complete the Anthropic use case details form
3. **Wait for Approval**: Model access typically granted within 15 minutes

Without model access, API calls will fail with `ResourceNotFoundException`.

### Inference Profiles

AWS Bedrock supports two model ID formats:

**Direct Model ID** (region-specific):
```go
ModelID: "anthropic.claude-3-5-sonnet-20241022-v2:0"
```

**Inference Profile** (cross-region routing, recommended):
```go
ModelID: "us.anthropic.claude-3-5-sonnet-20241022-v2:0"  // US regions
ModelID: "eu.anthropic.claude-3-5-sonnet-20241022-v2:0"  // EU regions
```

Inference profiles enable cross-region request routing and are **required** in some AWS accounts for on-demand throughput. The adapter automatically detects both formats.

### Multi-Region Failover

The adapter supports automatic failover to backup regions if the primary region is unavailable:

```go
config := bedrock.Config{
    Region: "us-east-1",
    FallbackRegions: []string{"us-west-2", "eu-west-1"},
    ModelID: "us.anthropic.claude-3-5-sonnet-20241022-v2:0",
}
```

When a request fails in the primary region with a retryable error (e.g., throttling, service unavailable), the adapter automatically retries the request in the next available fallback region.

### Streaming Responses

Enable streaming to receive tokens as they are generated:

```go
callback := func(chunk bedrock.StreamChunk) error {
    fmt.Print(chunk.Delta)  // Print tokens as they arrive
    return nil
}

response, err := adapter.ChatStream(ctx, messages, nil, callback)
```

The callback function is invoked for each token chunk. Return an error from the callback to abort streaming early.

### Tool Calling

The adapter supports Bedrock's tool calling capabilities:

```go
tools := []model.ToolSpec{
    {
        Name: "get_weather",
        Description: "Get current weather",
        Schema: map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "location": map[string]interface{}{
                    "type": "string",
                },
            },
            "required": []string{"location"},
        },
    },
}

response, err := adapter.Chat(ctx, messages, tools)
for _, call := range response.ToolCalls {
    fmt.Printf("Tool: %s, Input: %v\n", call.Name, call.Input)
}
```

Tool calls are returned in the `ChatOut.ToolCalls` slice with parsed `Name` and `Input` fields.

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

## Sequential Execution & Retry Policies

### Sequential vs Concurrent Execution

The framework supports both concurrent and sequential execution modes:

**Concurrent Execution** (default):
- Multiple nodes execute in parallel up to `MaxConcurrentNodes` limit
- Enables fan-out patterns via `Next{Many: []string{...}}`
- Results merge deterministically using reducer and ordering keys
- Default: `MaxConcurrentNodes = 8`

**Sequential Execution**:
- Set `MaxConcurrentNodes = 0` in `Options`
- Nodes execute one at a time in graph order
- No parallelism, no fan-out merging complexity
- Simpler mental model for debugging and testing

```go
engine := graph.New(
    reducer, store, emitter,
    graph.WithOptions(graph.Options{
        MaxConcurrentNodes: 0,  // Enable sequential mode
        MaxSteps:           20,
    }),
)
```

Sequential mode is ideal for:
- Debugging complex workflows
- Workflows with strict ordering requirements
- Resource-constrained environments
- Testing retry behavior in isolation

### RetryPolicy Configuration

Nodes can configure automatic retry behavior for transient failures via the `Policy()` method:

```go
type MyNode struct{}

func (n MyNode) Run(ctx context.Context, state S) NodeResult[S] {
    // Node implementation
}

func (n MyNode) Policy() graph.NodePolicy {
    return graph.NodePolicy{
        Timeout: 30 * time.Second,
        RetryPolicy: &graph.RetryPolicy{
            MaxAttempts: 3,                    // Total attempts (1 initial + 2 retries)
            BaseDelay:   time.Second,          // Base delay for backoff
            MaxDelay:    10 * time.Second,     // Maximum delay cap
            Retryable: func(err error) bool {  // Error classification
                errStr := err.Error()
                return strings.Contains(errStr, "timeout") ||
                    strings.Contains(errStr, "rate limit") ||
                    strings.Contains(errStr, "503")
            },
        },
    }
}
```

**RetryPolicy Fields**:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `MaxAttempts` | `int` | Yes | Maximum execution attempts including initial attempt. Must be >= 1. A value of 1 means no retries. |
| `BaseDelay` | `time.Duration` | Yes | Base delay for exponential backoff between retries. |
| `MaxDelay` | `time.Duration` | Yes | Maximum delay cap for exponential backoff. Must be >= BaseDelay. |
| `Retryable` | `func(error) bool` | Optional | Predicate to determine if error is retryable. If nil, all errors are non-retryable. |

**Validation Rules**:
- `MaxAttempts >= 1` (enforced at configuration time)
- `MaxDelay >= BaseDelay` (enforced at configuration time)
- If `RetryPolicy` is nil, node will not retry on errors

### Retry Behavior

When a node returns an error, the retry mechanism:

1. **Evaluates Retryability**: Calls `Retryable(err)` predicate
   - If `false` or `Retryable` is `nil`, fails immediately
   - If `true`, proceeds to step 2

2. **Checks Attempt Count**: Compares current attempt to `MaxAttempts`
   - If attempts exhausted, returns `graph.ErrMaxAttemptsExceeded`
   - Otherwise, proceeds to step 3

3. **Calculates Backoff Delay**: Uses exponential backoff with jitter
   ```
   delay = min(BaseDelay * 2^attempt, MaxDelay) + jitter
   ```
   - `attempt`: 0-based retry attempt (0 for first retry, 1 for second, etc.)
   - `jitter`: Random value in range `[0, BaseDelay)` for thundering herd prevention

4. **Waits with Cancellation**: Sleeps for calculated delay
   - Respects context cancellation during backoff
   - Returns `context.Canceled` or `context.DeadlineExceeded` if context cancelled

5. **Retries Execution**: Re-invokes `node.Run()` with incremented attempt counter

**Deterministic Jitter**:
- Jitter values are computed using a seeded RNG derived from `runID`
- Same `runID` produces identical retry delays across runs
- Enables deterministic replay of workflows with retries

### Accessing Attempt Number

Nodes can determine their current retry attempt via context:

```go
func (n MyNode) Run(ctx context.Context, state S) NodeResult[S] {
    attempt, ok := ctx.Value(graph.AttemptKey).(int)
    if !ok {
        attempt = 0  // First attempt
    }

    log.Printf("Executing attempt %d", attempt+1)

    // Adjust behavior based on attempt
    if attempt > 0 {
        // Use fallback strategy on retries
    }

    // ... node logic
}
```

**AttemptKey Context Value**:
- Type: `int` (0-based)
- Value: `0` for initial execution, `1` for first retry, `2` for second retry, etc.
- Always present in context during node execution
- Use for attempt-aware logging, metrics, or behavior adjustments

### Error Classification

The `Retryable` predicate determines which errors trigger retries:

**Common Retryable Errors**:
- Network timeouts and connection failures
- HTTP 429 (rate limit), 503 (service unavailable), 504 (gateway timeout)
- Database deadlocks and lock timeouts
- Temporary resource exhaustion

**Common Non-Retryable Errors**:
- HTTP 400 (bad request), 401 (unauthorized), 404 (not found)
- Validation errors and invalid input
- Business logic failures (e.g., insufficient funds)
- Permanent configuration errors

**Example Classification Functions**:

```go
// Network-focused retries
func networkRetryable(err error) bool {
    errStr := err.Error()
    return strings.Contains(errStr, "timeout") ||
        strings.Contains(errStr, "connection refused") ||
        strings.Contains(errStr, "temporary failure")
}

// HTTP API retries
func httpRetryable(err error) bool {
    errStr := err.Error()
    return strings.Contains(errStr, "429") ||  // Rate limit
        strings.Contains(errStr, "503") ||     // Service unavailable
        strings.Contains(errStr, "504")        // Gateway timeout
}

// Database retries
func dbRetryable(err error) bool {
    errStr := err.Error()
    return strings.Contains(errStr, "deadlock") ||
        strings.Contains(errStr, "lock timeout") ||
        strings.Contains(errStr, "serialization failure")
}
```

### Retry Examples

**LLM API Call with Retries**:
```go
type LLMNode struct {
    model graph.ChatModel
}

func (n LLMNode) Policy() graph.NodePolicy {
    return graph.NodePolicy{
        Timeout: 30 * time.Second,
        RetryPolicy: &graph.RetryPolicy{
            MaxAttempts: 3,
            BaseDelay:   time.Second,
            MaxDelay:    10 * time.Second,
            Retryable: func(err error) bool {
                errStr := err.Error()
                return strings.Contains(errStr, "timeout") ||
                    strings.Contains(errStr, "rate limit") ||
                    strings.Contains(errStr, "503")
            },
        },
    }
}
```

**External API Call with Aggressive Retries**:
```go
type APIFetchNode struct{}

func (n APIFetchNode) Policy() graph.NodePolicy {
    return graph.NodePolicy{
        Timeout: 15 * time.Second,
        RetryPolicy: &graph.RetryPolicy{
            MaxAttempts: 4,  // API can be flaky
            BaseDelay:   2 * time.Second,
            MaxDelay:    20 * time.Second,
            Retryable: func(err error) bool {
                errStr := err.Error()
                return strings.Contains(errStr, "timeout") ||
                    strings.Contains(errStr, "connection") ||
                    strings.Contains(errStr, "503") ||
                    strings.Contains(errStr, "502")
            },
        },
    }
}
```

**No Retries for Validation Errors**:
```go
type ValidationNode struct{}

func (n ValidationNode) Policy() graph.NodePolicy {
    return graph.NodePolicy{
        Timeout: 5 * time.Second,
        // No RetryPolicy: validation errors are not retryable
    }
}
```

### Reference Examples

- `examples/ai_research_assistant/` - Comprehensive retry configuration for LLM and API nodes
- `examples/node_timeouts/` - Sequential execution with timeout and retry behavior

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
- Go 1.21+ (requires generics support for ChatModel interface compatibility) (008-bedrock-llm-support)
- N/A (adapter is stateless, uses existing Engine state management) (008-bedrock-llm-support)

## Recent Changes
- 002-concurrency-spec: Added Go 1.21+ (requires generics support) + Go standard library only (core framework), optional adapters for OpenTelemetry SDK, MySQL driver
