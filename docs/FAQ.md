# Frequently Asked Questions (FAQ)

Common questions about LangGraph-Go and workflow development.

## General Questions

### What is LangGraph-Go?

LangGraph-Go is a Go-native framework for building stateful, graph-based workflows. It's designed for orchestrating LLM calls, tools, and complex multi-step processes with features like:
- Type-safe state management using Go generics
- Built-in checkpointing and resumption
- Parallel execution support
- Provider-agnostic LLM integration
- Comprehensive event tracing

### Why use LangGraph-Go instead of simple function calls?

Use LangGraph-Go when you need:
- **State persistence** - Resume workflows after crashes
- **Complex routing** - Conditional logic, loops, branching
- **Parallel execution** - Run independent tasks concurrently
- **Observability** - Track every execution step
- **Provider switching** - Swap LLM providers without code changes
- **Deterministic replay** - Reproduce execution from checkpoints

For simple one-shot operations, plain functions are simpler.

### How is this different from Python's LangGraph?

**Similarities**:
- Graph-based workflow model
- State management with reducers
- Checkpointing support
- LLM integration

**Differences**:
- **Type safety**: Go's generics provide compile-time type checking
- **Performance**: Native Go performance (no GIL, efficient concurrency)
- **Deployment**: Single binary, no Python runtime needed
- **Concurrency**: Native goroutines for parallel execution

### What Go version is required?

Go 1.21 or later (requires generics support).

---

## Getting Started

### How do I install LangGraph-Go?

```bash
go get github.com/dshills/langgraph-go
```

See [Getting Started Guide](./guides/01-getting-started.md) for a complete walkthrough.

### What's the simplest possible workflow?

```go
// 1. Define state
type State struct { Result string }

// 2. Define reducer
reducer := func(prev, delta State) State {
    if delta.Result != "" {
        prev.Result = delta.Result
    }
    return prev
}

// 3. Create node
node := graph.NodeFunc[State](func(ctx context.Context, s State) graph.NodeResult[State] {
    return graph.NodeResult[State]{
        Delta: State{Result: "done"},
        Route: graph.Stop(),
    }
})

// 4. Run workflow
engine := graph.New(reducer, store.NewMemStore[State](), emit.NewNullEmitter(), graph.Options{})
engine.Add("work", node)
engine.StartAt("work")
final, _ := engine.Run(context.Background(), "run-001", State{})
```

### Do I need a database to use LangGraph-Go?

No. For development and testing, use the in-memory store:

```go
store := store.NewMemStore[State]()
```

For production persistence, use a database-backed store (MySQL, PostgreSQL, etc.).

---

## State Management

### Why do I need a reducer?

The reducer merges partial state updates (deltas) from nodes into the accumulated state. This enables:
- **Partial updates** - Nodes only specify changed fields
- **Parallel merging** - Combine results from parallel branches
- **Deterministic replay** - Reproduce execution from checkpoints

### Can I mutate state directly in nodes?

❌ **No.** Nodes should return state changes via `Delta`, not mutate the input state:

```go
// ❌ BAD: Mutates input state
func badNode(ctx context.Context, s State) graph.NodeResult[State] {
    s.Count++ // Don't do this!
    return graph.NodeResult[State]{Route: graph.Goto("next")}
}

// ✅ GOOD: Returns delta
func goodNode(ctx context.Context, s State) graph.NodeResult[State] {
    return graph.NodeResult[State]{
        Delta: State{Count: 1}, // Reducer increments
        Route: graph.Goto("next"),
    }
}
```

### What types can I use for state?

State must be:
- **A struct** (not primitives or interfaces)
- **JSON-serializable** (for checkpointing)
- **Copyable** (for parallel execution)

```go
✅ type State struct { Count int; Items []string }
✅ type State struct { Map map[string]int; Data []byte }

❌ type State int                    // Not a struct
❌ type State interface{}            // Not concrete
❌ type State struct { Ch chan int } // Not JSON-serializable
```

### How do I handle large data in state?

Store references, not the data itself:

```go
// ❌ BAD: Stores megabytes of HTML
type State struct {
    HTMLPages []string // Megabytes!
}

// ✅ GOOD: Stores references
type State struct {
    PageIDs []string // Fetch from DB as needed
}
```

---

## Workflow Design

### How do I create a loop?

Use conditional edges to route back to earlier nodes:

```go
engine.Connect("process", "process", func(s State) bool {
    return s.Iterations < 10 && !s.Done
})

engine.Connect("process", "complete", func(s State) bool {
    return s.Iterations >= 10 || s.Done
})
```

See [Conditional Routing](./guides/05-routing.md).

### How do I run tasks in parallel?

Use fan-out routing:

```go
func fanOut(ctx context.Context, s State) graph.NodeResult[State] {
    return graph.NodeResult[State]{
        Route: graph.Next{
            Many: []string{"task1", "task2", "task3"},
        },
    }
}
```

See [Parallel Execution](./guides/06-parallel.md).

### How do I handle errors?

Three options:

**1. Return error (halts workflow)**:
```go
return graph.NodeResult[State]{
    Err: fmt.Errorf("validation failed"),
}
```

**2. Route to error handler**:
```go
if err := validate(s); err != nil {
    return graph.NodeResult[State]{
        Delta: State{LastError: err.Error()},
        Route: graph.Goto("error-handler"),
    }
}
```

**3. Configure retries**:
```go
opts := graph.Options{Retries: 3}
```

See [Building Workflows](./guides/02-building-workflows.md#error-handling).

### Can I have multiple entry points?

No. Each engine has one entry point set via `StartAt()`. For multiple workflows, create multiple engines.

### How do I prevent infinite loops?

Set `MaxSteps` in options:

```go
opts := graph.Options{MaxSteps: 100}
```

If a workflow exceeds this limit, `Run()` returns an error.

---

## LLM Integration

### Which LLM providers are supported?

- OpenAI (GPT-4, GPT-3.5)
- Anthropic (Claude)
- Google (Gemini)

All implement the unified `ChatModel` interface.

### Can I use local LLMs (Ollama)?

The framework supports any provider that implements the `ChatModel` interface. An Ollama adapter can be created by implementing:

```go
type ChatModel interface {
    Chat(ctx context.Context, messages []Message, tools []ToolSpec) (ChatOut, error)
}
```

### How do I switch between providers?

Use the same interface for all providers:

```go
var llm model.ChatModel

if useOpenAI {
    llm = openai.NewChatModel(key, "gpt-4o")
} else {
    llm = anthropic.NewChatModel(key, "claude-sonnet-4-5-20250929")
}

out, err := llm.Chat(ctx, messages, nil)
```

See [LLM Integration](./guides/07-llm-integration.md).

### How do I use tools with LLMs?

Define tools with JSON Schema and pass to `Chat()`:

```go
tools := []model.ToolSpec{
    {
        Name:        "calculator",
        Description: "Performs arithmetic",
        Schema: map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "operation": map[string]interface{}{"type": "string"},
                "a": map[string]interface{}{"type": "number"},
                "b": map[string]interface{}{"type": "number"},
            },
        },
    },
}

out, err := llm.Chat(ctx, messages, tools)

// Check if LLM wants to call tools
if len(out.ToolCalls) > 0 {
    // Execute tools and send results back
}
```

---

## Checkpoints & Persistence

### How do I save workflow progress?

It's automatic! Every step is saved via the `Store`:

```go
store := store.NewMemStore[State]() // Or MySQL/PostgreSQL
engine := graph.New(reducer, store, emitter, opts)

// Each node execution triggers SaveStep()
engine.Run(ctx, "run-001", initialState)
```

### How do I resume a workflow?

Load the latest state and re-run:

```go
lastState, lastStep, err := store.LoadLatest(ctx, "run-001")
if err != nil {
    // No previous run, start fresh
    engine.Run(ctx, "run-001", initialState)
} else {
    // Resume from last state
    engine.Run(ctx, "run-001", lastState)
}
```

See [Checkpoints & Resume](./guides/04-checkpoints.md).

### How do I create manual checkpoints?

Use named checkpoints:

```go
// Inside a node
cpID := "checkpoint-after-validation"
store.SaveCheckpoint(ctx, cpID, s, s.StepCount)

// Later: restore from checkpoint
state, step, _ := store.LoadCheckpoint(ctx, cpID)
```

### Can I use SQLite for persistence?

Currently, LangGraph-Go provides:
- `MemStore` (in-memory)
- `MySQLStore` (MySQL/Aurora)

For SQLite, implement the `Store[S]` interface:

```go
type Store[S any] interface {
    SaveStep(ctx context.Context, runID string, step int, nodeID string, state S) error
    LoadLatest(ctx context.Context, runID string) (S, int, error)
    SaveCheckpoint(ctx context.Context, cpID string, state S, step int) error
    LoadCheckpoint(ctx context.Context, cpID string) (S, int, error)
}
```

---

## Observability

### How do I debug workflows?

Use `LogEmitter` during development:

```go
emitter := emit.NewLogEmitter(os.Stdout, false) // Text mode
engine := graph.New(reducer, store, emitter, opts)
```

This prints every event to stdout.

### How do I analyze workflow execution?

Use `BufferedEmitter` to capture events:

```go
buffered := emit.NewBufferedEmitter()
engine := graph.New(reducer, store, buffered, opts)

engine.Run(ctx, "run-001", initialState)

// Query events
allEvents := buffered.GetHistory("run-001")
errors := buffered.GetHistoryWithFilter("run-001", emit.HistoryFilter{Msg: "error"})
```

See [Event Tracing](./guides/08-event-tracing.md).

### What events are emitted?

Standard events:
- `node_start` - Node begins execution
- `node_end` - Node completes (includes state delta)
- `routing_decision` - Shows next node(s)
- `error` - Node encountered error

### How do I integrate with OpenTelemetry?

Use the OpenTelemetry emitter (when available):

```go
import "github.com/dshills/langgraph-go/graph/emit/otel"

otelEmitter := otel.NewOtelEmitter(tracerProvider)
engine := graph.New(reducer, store, otelEmitter, opts)
```

---

## Performance

### How fast is LangGraph-Go?

Performance depends on:
- Node execution time (LLM calls, I/O)
- State size (affects copying for parallel execution)
- Store implementation (in-memory vs database)
- Emitter overhead (NullEmitter has zero overhead)

For microbenchmarks, see `graph/benchmark_test.go`.

### How do I optimize performance?

1. **Use parallel execution** for independent tasks
2. **Minimize state size** (store references, not data)
3. **Use NullEmitter** in production if observability not needed
4. **Batch operations** instead of per-item nodes
5. **Cache LLM responses** for identical queries

### Does parallel execution actually run concurrently?

Yes! Each branch in a fan-out executes in a separate goroutine. Branches run truly concurrently and merge deterministically via the reducer.

### What's the overhead of checkpointing?

Checkpointing cost depends on the `Store` implementation:
- **MemStore**: Fast (in-memory)
- **MySQLStore**: Network + serialization overhead
- **Custom stores**: Varies

Checkpointing happens after every step, so minimize state size.

---

## Testing

### How do I test workflows?

Use `MemStore` and `BufferedEmitter` for tests:

```go
func TestWorkflow(t *testing.T) {
    store := store.NewMemStore[State]()
    emitter := emit.NewBufferedEmitter()
    engine := graph.New(reducer, store, emitter, graph.Options{})

    // Build workflow
    engine.Add("A", nodeA)
    engine.Add("B", nodeB)
    engine.StartAt("A")

    // Execute
    final, err := engine.Run(context.Background(), "test-001", initialState)

    // Assert
    if err != nil {
        t.Fatal(err)
    }
    if final.Result != "expected" {
        t.Errorf("got %s, want expected", final.Result)
    }

    // Verify events
    events := emitter.GetHistory("test-001")
    if len(events) != expectedCount {
        t.Errorf("got %d events, want %d", len(events), expectedCount)
    }
}
```

### How do I mock LLMs for testing?

Create a mock `ChatModel`:

```go
type MockLLM struct {
    responses []string
    callCount int
}

func (m *MockLLM) Chat(ctx context.Context, messages []model.Message, tools []model.ToolSpec) (model.ChatOut, error) {
    if m.callCount >= len(m.responses) {
        return model.ChatOut{}, errors.New("no more responses")
    }
    response := m.responses[m.callCount]
    m.callCount++
    return model.ChatOut{Text: response}, nil
}

// Use in tests
mockLLM := &MockLLM{responses: []string{"Test response"}}
```

### Can I use table-driven tests?

Yes:

```go
func TestReducer(t *testing.T) {
    tests := []struct {
        name  string
        prev  State
        delta State
        want  State
    }{
        {"append items", State{Items: []string{"a"}}, State{Items: []string{"b"}}, State{Items: []string{"a", "b"}}},
        {"increment counter", State{Count: 5}, State{Count: 3}, State{Count: 8}},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := reducer(tt.prev, tt.delta)
            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("reducer() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

---

## Troubleshooting

### "MaxSteps exceeded"

**Problem**: Workflow hit the step limit.

**Solutions**:
1. Check for infinite loops in routing logic
2. Increase `Options.MaxSteps` if workflow is legitimately long
3. Add explicit `graph.Stop()` conditions

### "No valid route found"

**Problem**: Node returned `Next{}` but no edge matched.

**Solutions**:
1. Use explicit routing: `graph.Goto("node-name")`
2. Ensure edge predicates can match the current state
3. Add a default edge: `engine.Connect("node", "default", nil)`

### State changes not appearing

**Problem**: State modifications not visible in final state.

**Solutions**:
1. Verify reducer actually merges the fields
2. Ensure nodes populate `Delta`, not mutate input state
3. Check that reducer returns the modified state

### Parallel branches not merging

**Problem**: Results from parallel branches missing.

**Solutions**:
1. Ensure all parallel nodes route to terminal (`graph.Stop()`)
2. Verify reducer merges parallel deltas correctly
3. Check that parallel nodes don't overwrite each other's results

### Context deadline exceeded

**Problem**: Workflow times out.

**Solutions**:
1. Increase context timeout
2. Optimize slow nodes (especially LLM calls)
3. Use parallel execution for independent tasks
4. Add timeout handling in nodes

---

## Migration & Compatibility

### Can I migrate from Python LangGraph?

Conceptually yes, but requires rewriting in Go:
1. Convert Python classes to Go structs
2. Implement `Node[S]` interface for each node
3. Convert state dict to typed Go struct
4. Translate conditional logic to Go functions

No automated migration tool currently exists.

### Is the API stable?

The framework is in active development. Breaking changes may occur before v1.0.

Pin to specific versions:
```bash
go get github.com/dshills/langgraph-go@v0.x.x
```

### How do I upgrade to new versions?

1. Check CHANGELOG.md for breaking changes
2. Update import in go.mod
3. Run tests to verify compatibility
4. Adjust code for breaking changes if any

---

## Contributing

### How can I contribute?

See [CONTRIBUTING.md](../CONTRIBUTING.md) for:
- Code style guidelines
- Testing requirements
- Pull request process
- Development workflow

### Where do I report bugs?

GitHub Issues: https://github.com/dshills/langgraph-go/issues

Please include:
- Go version (`go version`)
- Framework version
- Minimal reproducible example
- Expected vs actual behavior

### How do I request features?

Open a GitHub issue with:
- Use case description
- Proposed API (if applicable)
- Why existing features don't solve the problem

---

## Additional Resources

- [Getting Started](./guides/01-getting-started.md)
- [API Reference](./api/)
- [Examples](../examples/)
- [GoDoc](https://pkg.go.dev/github.com/dshills/langgraph-go)
