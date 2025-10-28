# LangGraph-Go Examples

This directory contains working examples demonstrating LangGraph-Go features and patterns. Each example is self-contained and runnable with `go run`.

## Quick Start

```bash
# Clone the repository
git clone https://github.com/dshills/langgraph-go.git
cd langgraph-go/examples

# Run any example
cd concurrent_workflow
go run main.go
```

## Examples by Feature

### 🚀 Concurrent Execution

#### [concurrent_workflow](./concurrent_workflow) - Parallel Data Fetching Pipeline
**Level**: Intermediate | **Time**: 5 minutes

Demonstrates concurrent execution of 5 independent nodes that fetch data from different sources (academic papers, news, social media, patents, market data). Shows:
- Fan-out parallelism (5 nodes execute simultaneously)
- Deterministic state merging despite non-deterministic completion order
- Performance speedup measurement (sequential ~4.6s vs parallel ~1.1s)
- Side effect policies for recordable I/O
- Resource control with MaxConcurrentNodes

```bash
cd concurrent_workflow
go run main.go

# Expected output:
# - 5 data sources queried in parallel
# - ~4x speedup vs sequential execution
# - Deterministic result merging
```

**Key Concepts**:
- `Options.MaxConcurrentNodes` for parallelism control
- `Next.Many` for fan-out routing
- `SideEffectPolicy.Recordable` for replay support
- Deterministic reducer functions

---

#### [parallel](./parallel) - Simple Parallel Text Processing
**Level**: Beginner | **Time**: 2 minutes

A simpler example showing 4 text processing operations running in parallel. Good starting point before `concurrent_workflow`.

```bash
cd parallel
go run main.go
```

---

### ♻️ Deterministic Replay

#### [replay_demo](./replay_demo) - Checkpoint Save and Replay
**Level**: Advanced | **Time**: 10 minutes

Demonstrates deterministic replay using seeded random number generation and recorded I/O. Implements a dice game workflow where:
- Original execution records API responses and uses seeded RNG
- Replay execution produces identical results without external calls
- Different runIDs produce different (but still deterministic) random sequences
- Checkpoint save/restore for time-travel debugging

```bash
cd replay_demo
go run main.go

# Expected output:
# Part 1: Original execution (makes API calls, records I/O)
# Part 2: Replay execution (uses recorded I/O, no API calls)
# Part 3: Verification (states match exactly)
# Part 4: Different runID (different random sequence)
```

**Key Concepts**:
- Seeded RNG via `ctx.Value(graph.RNGKey)` for deterministic randomness
- `Options.ReplayMode` for record vs replay
- `SideEffectPolicy.Recordable` for I/O capture
- Checkpoint save/restore with `SaveCheckpointV2`/`LoadCheckpointV2`
- State hash verification for replay correctness

**Use Cases**:
- **Debugging**: Replay production failures locally without external dependencies
- **Testing**: Verify workflow logic with recorded API responses
- **Auditing**: Reconstruct exact execution flow from checkpoints
- **Time Travel**: Resume from any checkpoint in execution history

---

### 💾 Checkpoint Management

#### [checkpoint](./checkpoint) - Basic Checkpoint Save/Resume
**Level**: Beginner | **Time**: 3 minutes

Demonstrates basic checkpoint patterns without replay complexity. Shows:
- Saving checkpoints during execution
- Resuming from saved checkpoints
- Checkpoint labeling for named snapshots

```bash
cd checkpoint
go run main.go
```

---

### 🤖 LLM Integration

#### [llm](./llm) - Multi-Provider LLM Support
**Level**: Intermediate | **Time**: 5 minutes

Demonstrates LLM integration with OpenAI, Anthropic, Google, and Ollama providers. Requires API keys.

```bash
cd llm
export OPENAI_API_KEY=your-key
go run main.go
```

---

#### [chatbot](./chatbot) - Interactive LLM Chatbot
**Level**: Intermediate | **Time**: 5 minutes

Build a simple conversational chatbot with memory. Shows:
- Message history management
- Turn-based conversation flow
- Streaming responses

```bash
cd chatbot
export OPENAI_API_KEY=your-key
go run main.go
```

---

### 🛠️ Tool Integration

#### [tools](./tools) - HTTP Tool with Real API Calls
**Level**: Intermediate | **Time**: 5 minutes

Demonstrates the tool system with HTTP tool that makes real API requests. Shows:
- Tool definition and registration
- Parameter validation
- Error handling for external calls
- Response parsing

```bash
cd tools
go run main.go
```

---

### 🔀 Control Flow

#### [routing](./routing) - Conditional Routing with Predicates
**Level**: Beginner | **Time**: 3 minutes

Shows dynamic routing based on state predicates. Demonstrates:
- Edge predicates for conditional transitions
- Multiple exit paths from a node
- State-based routing decisions

```bash
cd routing
go run main.go
```

---

### 📊 Advanced Workflows

#### [research-pipeline](./research-pipeline) - Complex Multi-Stage Pipeline
**Level**: Advanced | **Time**: 10 minutes

Production-like research workflow with multiple stages, LLM calls, and tool invocations. Demonstrates:
- Multi-stage pipeline (gather → analyze → synthesize)
- LLM + tool integration
- Error handling and retries
- State accumulation across stages

```bash
cd research-pipeline
export OPENAI_API_KEY=your-key
go run main.go
```

---

#### [data-pipeline](./data-pipeline) - ETL Pipeline Pattern
**Level**: Intermediate | **Time**: 5 minutes

Shows ETL (Extract, Transform, Load) pattern using LangGraph. Good example of non-LLM workflows:
- Data extraction from multiple sources
- Transformation and validation
- Batch processing patterns

```bash
cd data-pipeline
go run main.go
```

---

#### [interactive-workflow](./interactive-workflow) - Human-in-the-Loop Pattern
**Level**: Advanced | **Time**: 10 minutes

Demonstrates human-in-the-loop workflows with approval steps. Shows:
- Workflow pause for human input
- Resume after approval/rejection
- State persistence during pause
- Interactive decision nodes

```bash
cd interactive-workflow
go run main.go
```

---

### 📈 Observability

#### [tracing](./tracing) - OpenTelemetry Integration
**Level**: Advanced | **Time**: 10 minutes

Demonstrates observability with OpenTelemetry tracing. Shows:
- Span creation for nodes and edges
- Distributed tracing context propagation
- Metrics collection
- Integration with Jaeger/Zipkin

```bash
cd tracing
# Start Jaeger (requires Docker)
docker run -d --name jaeger \
  -p 16686:16686 \
  -p 4318:4318 \
  jaegertracing/all-in-one:latest

go run main.go

# View traces at http://localhost:16686
```

---

### 🏎️ Performance

#### [benchmarks](./benchmarks) - Performance Benchmarking
**Level**: Advanced | **Time**: 5 minutes

Benchmarks for comparing concurrent vs sequential execution, checkpoint overhead, and reducer performance.

```bash
cd benchmarks
go test -bench=. -benchmem
```

---

## Example Patterns by Use Case

### Building Agent Workflows
1. Start with [llm](./llm) for basic LLM integration
2. Add tools with [tools](./tools)
3. Scale with [research-pipeline](./research-pipeline)
4. Add human oversight with [interactive-workflow](./interactive-workflow)

### Data Processing Pipelines
1. Start with [parallel](./parallel) for basic parallelism
2. Scale to [concurrent_workflow](./concurrent_workflow) for real-world patterns
3. Add ETL patterns with [data-pipeline](./data-pipeline)

### Production Deployment
1. Implement observability with [tracing](./tracing)
2. Add checkpoint/resume with [checkpoint](./checkpoint)
3. Enable replay debugging with [replay_demo](./replay_demo)
4. Benchmark performance with [benchmarks](./benchmarks)

---

## Concepts by Example

| Concept | Example(s) |
|---------|-----------|
| **Concurrent execution** | concurrent_workflow, parallel |
| **Deterministic replay** | replay_demo |
| **Checkpoint/resume** | checkpoint, replay_demo |
| **LLM integration** | llm, chatbot, research-pipeline |
| **Tool calling** | tools, research-pipeline |
| **Conditional routing** | routing |
| **State management** | All examples |
| **Error handling** | research-pipeline, tools |
| **Observability** | tracing |
| **Performance tuning** | benchmarks, concurrent_workflow |

---

## Running Examples with Different Configurations

Most examples support configuration via environment variables:

```bash
# Adjust concurrency level
export MAX_CONCURRENT_NODES=4

# Enable replay mode
export REPLAY_MODE=true

# Configure timeouts
export NODE_TIMEOUT=30s
export RUN_TIMEOUT=5m

# Run example
go run main.go
```

---

## Common Patterns

### 1. Concurrent Fan-Out

```go
// Launch multiple nodes in parallel from a single entry point
opts := graph.Options{
    MaxConcurrentNodes: 5,
    QueueDepth:        1024,
}

engine := graph.New(reducer, store, emitter, opts)

// All these nodes execute concurrently
engine.Add("fetch_a", nodeA)
engine.Add("fetch_b", nodeB)
engine.Add("fetch_c", nodeC)

// Fan-out from start
engine.AddEdge("start", "fetch_a", nil)
engine.AddEdge("start", "fetch_b", nil)
engine.AddEdge("start", "fetch_c", nil)
```

See: [concurrent_workflow](./concurrent_workflow), [parallel](./parallel)

---

### 2. Deterministic Replay

```go
// Record mode: Capture I/O for later replay
engine := graph.New(reducer, store, emitter, graph.Options{
    ReplayMode:   false,  // Record
    StrictReplay: true,
})

// Nodes must use seeded RNG from context
rng := ctx.Value(graph.RNGKey).(*rand.Rand)
randomValue := rng.Intn(100)  // Deterministic!

// Replay mode: Use recorded I/O
replayEngine := graph.New(reducer, store, emitter, graph.Options{
    ReplayMode:   true,   // Replay
    StrictReplay: true,
})
```

See: [replay_demo](./replay_demo)

---

### 3. Checkpoint Save/Restore

```go
// Save checkpoint after step
err := store.SaveCheckpointV2(ctx, runID, stepID, "label", state, frontier)

// Load checkpoint
checkpoint, err := store.LoadCheckpointV2(ctx, runID, stepID)

// Resume from checkpoint
finalState, err := engine.RunWithCheckpoint(ctx, checkpoint)
```

See: [checkpoint](./checkpoint), [replay_demo](./replay_demo)

---

### 4. Side Effect Policies

```go
type MyNode struct {
    graph.DefaultPolicy
}

// Declare recordable I/O
func (n *MyNode) Effects() graph.SideEffectPolicy {
    return graph.SideEffectPolicy{
        Recordable:          true,  // Can be replayed
        RequiresIdempotency: true,  // Use idempotency keys
    }
}

// Pure nodes don't need Effects() - default is no side effects
```

See: [concurrent_workflow](./concurrent_workflow), [replay_demo](./replay_demo)

---

### 5. Retry Policies

```go
type RobustNode struct {
    graph.DefaultPolicy
}

func (n *RobustNode) Policy() graph.NodePolicy {
    return graph.NodePolicy{
        Timeout: 30 * time.Second,
        RetryPolicy: &graph.RetryPolicy{
            MaxAttempts: 3,
            BaseDelay:   time.Second,
            MaxDelay:    10 * time.Second,
            Retryable: func(err error) bool {
                return strings.Contains(err.Error(), "timeout")
            },
        },
    }
}
```

See: [research-pipeline](./research-pipeline)

---

## FAQ

### Q: How do I control concurrency level?
**A**: Set `Options.MaxConcurrentNodes`. Default is 8. Set to 0 for sequential execution (backward compatible).

See: [concurrent_workflow](./concurrent_workflow)

---

### Q: How do I make replay work with random values?
**A**: Use the seeded RNG from context: `rng := ctx.Value(graph.RNGKey).(*rand.Rand)`. Never use `math/rand` directly or `crypto/rand` in nodes that need replay.

See: [replay_demo](./replay_demo)

---

### Q: How do I add retry logic?
**A**: Implement `Policy() NodePolicy` on your node and configure `RetryPolicy`. The engine automatically retries with exponential backoff.

See: [research-pipeline](./research-pipeline)

---

### Q: How do I integrate LLMs?
**A**: Use the model adapters in `graph/model/`. Support for OpenAI, Anthropic, Google, and Ollama.

See: [llm](./llm), [chatbot](./chatbot)

---

### Q: How do I add observability?
**A**: Use the OpenTelemetry emitter in `graph/emit/otel.go`. It emits spans for each node execution.

See: [tracing](./tracing)

---

### Q: How do I persist state to a database?
**A**: Use `store.NewMySQLStore()` instead of `store.NewMemStore()`. Requires MySQL/Aurora.

See: Main README for MySQL setup instructions.

---

## Contributing Examples

We welcome new examples! Please:
1. Create a new directory under `examples/`
2. Include a runnable `main.go`
3. Add comments explaining key concepts
4. Update this README with your example
5. Test with `go run main.go`
6. Submit a PR

**Example categories we need**:
- Multi-modal LLM workflows (vision, audio)
- Integration with specific tools (GitHub, Slack, etc.)
- Industry-specific pipelines (finance, healthcare, etc.)
- Advanced error handling patterns
- Custom reducer patterns
- Streaming response handling

---

## Resources

- **Documentation**: [Main README](../README.md)
- **Concurrency Guide**: [docs/concurrency.md](../docs/concurrency.md)
- **Replay Guide**: [docs/replay.md](../docs/replay.md)
- **API Reference**: [docs/api.md](../docs/api.md)
- **Quickstart**: [specs/002-concurrency-spec/quickstart.md](../specs/002-concurrency-spec/quickstart.md)

---

## License

MIT - See [LICENSE](../LICENSE)
