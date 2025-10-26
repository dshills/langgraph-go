# LangGraph-Go

A Go-native orchestration framework for building **stateful, graph-based LLM and tool workflows**.

## Overview

LangGraph-Go enables you to build complex AI agent systems with:
- **Deterministic replay** - Resume workflows from any checkpoint
- **Type-safe state** - Strongly-typed state management using Go generics
- **Flexible routing** - Conditional logic, loops, and parallel execution
- **LLM integration** - Unified interface for OpenAI, Anthropic, Google, and local models
- **Production-ready** - Built-in persistence, observability, and error handling

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/dshills/langgraph-go/graph"
    "github.com/dshills/langgraph-go/graph/emit"
    "github.com/dshills/langgraph-go/graph/store"
)

// Define your state type
type Session struct {
    Query  string
    Answer string
    Steps  int
}

// Create a reducer for state merging
func reduce(prev, delta Session) Session {
    if delta.Query != "" {
        prev.Query = delta.Query
    }
    if delta.Answer != "" {
        prev.Answer = delta.Answer
    }
    prev.Steps += delta.Steps
    return prev
}

func main() {
    // Create nodes
    process := graph.NodeFunc[Session](func(ctx context.Context, s Session) graph.NodeResult[Session] {
        return graph.NodeResult[Session]{
            Delta: Session{Answer: "Processed: " + s.Query, Steps: 1},
            Route: graph.Stop(),
        }
    })

    // Build the workflow
    st := store.NewMemStore[Session]()
    emitter := emit.NewLogEmitter(os.Stdout, false)
    opts := graph.Options{MaxSteps: 10}

    engine := graph.New(reduce, st, emitter, opts)
    engine.Add("process", process)
    engine.StartAt("process")

    // Execute
    ctx := context.Background()
    final, err := engine.Run(ctx, "run-001", Session{Query: "Hello LangGraph!"})
    if err != nil {
        panic(err)
    }

    fmt.Println(final.Answer) // Output: Processed: Hello LangGraph!
}
```

## Installation

```bash
go get github.com/dshills/langgraph-go
```

## Core Concepts

### Nodes

Nodes are processing units that receive state, perform computation, and return state modifications:

```go
type Node[S any] interface {
    Run(ctx context.Context, state S) NodeResult[S]
}
```

### State & Reducers

State flows through the workflow and is merged using reducer functions:

```go
type Reducer[S any] func(prev S, delta S) S
```

### Routing

Control flow with conditional routing:

```go
// Explicit routing
graph.Goto("next-node")
graph.Stop()

// Conditional edges
engine.Connect("nodeA", "nodeB", func(s Session) bool {
    return s.Confidence > 0.8
})
```

### Persistence

Automatic checkpointing enables workflow resumption:

```go
// Save checkpoint
engine.SaveCheckpoint("after-step-1")

// Resume from checkpoint
engine.LoadCheckpoint("after-step-1")
final, err := engine.Run(ctx, runID, initialState)
```

## Architecture

```
/graph
  ├── engine.go      # Workflow execution engine
  ├── node.go        # Node abstractions
  ├── state.go       # State management
  ├── store/         # Persistence layer
  │   └── memory.go  # In-memory store (for testing)
  ├── emit/          # Event system
  │   └── log.go     # Logging emitter
  ├── model/         # LLM integrations
  │   ├── openai/
  │   ├── anthropic/
  │   └── google/
  └── tool/          # Tool abstractions
```

## Features

- ✅ **Stateful Execution** - Checkpoint and resume workflows
- ✅ **Conditional Routing** - Dynamic control flow based on state
- ✅ **Parallel Execution** - Fan-out to concurrent nodes
- ✅ **LLM Integration** - OpenAI, Anthropic, Google, Ollama
- ✅ **Event Tracing** - Comprehensive observability
- ✅ **Type Safety** - Go generics for compile-time safety
- ✅ **Production Ready** - Error handling, retries, timeouts

## Examples

See the [`examples/`](./examples) directory for complete examples:
- `simple/` - Basic 3-node workflow
- `checkpoint/` - Checkpoint and resume
- `routing/` - Conditional routing
- `parallel/` - Parallel execution
- `llm/` - LLM provider integration

## Documentation

### User Guides

- [Getting Started](./docs/guides/01-getting-started.md) - Build your first workflow
- [Building Workflows](./docs/guides/02-building-workflows.md) - Patterns and best practices
- [State Management](./docs/guides/03-state-management.md) - Advanced reducer patterns
- [Checkpoints & Resume](./docs/guides/04-checkpoints.md) - Save and resume workflows
- [Conditional Routing](./docs/guides/05-routing.md) - Dynamic control flow
- [Parallel Execution](./docs/guides/06-parallel.md) - Concurrent node execution
- [LLM Integration](./docs/guides/07-llm-integration.md) - Multi-provider LLM support
- [Event Tracing](./docs/guides/08-event-tracing.md) - Observability and monitoring

### Reference

- [API Reference](./docs/api/) - Complete API documentation
- [FAQ](./docs/FAQ.md) - Frequently asked questions

### Project Documentation

- [Architecture Overview](./CLAUDE.md)
- [Technical Specification](./specs/SPEC.md)
- [Contributing Guide](./CONTRIBUTING.md)

## Development

```bash
# Run tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run linter
golangci-lint run

# Build
go build ./...
```

## License

MIT License - see [LICENSE](./LICENSE) for details

## Project Status

✅ **Core Framework Complete** - All 5 user stories have been implemented and tested:

- ✅ **US1**: Stateful workflow with checkpointing
- ✅ **US2**: Conditional routing and dynamic control flow
- ✅ **US3**: Parallel execution with fan-out/fan-in
- ✅ **US4**: Multi-provider LLM integration
- ✅ **US5**: Comprehensive event tracing and observability

**Current Phase**: Documentation and polish (81% complete)

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](./CONTRIBUTING.md) for development guidelines including our Test-Driven Development workflow.

## Acknowledgments

Inspired by [LangGraph](https://github.com/langchain-ai/langgraph), redesigned for Go's type system and concurrency primitives.
