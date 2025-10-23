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
    "github.com/dshills/langgraph-go/graph"
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
    if delta.Steps != 0 {
        prev.Steps = delta.Steps
    }
    return prev
}

func main() {
    // Create nodes
    process := graph.NodeFunc[Session](func(ctx context.Context, s Session) graph.NodeResult[Session] {
        return graph.NodeResult[Session]{
            Delta: Session{Answer: "Processed: " + s.Query, Steps: s.Steps + 1},
            Route: graph.Stop(),
        }
    })

    // Build the workflow
    store := graph.NewMemStore[Session]()
    emitter := graph.NewLogEmitter()

    engine := graph.New[Session](reduce, store, emitter, graph.Options{MaxSteps: 10})
    engine.Add("process", process)
    engine.StartAt("process")

    // Execute
    ctx := context.Background()
    final, err := engine.Run(ctx, "run-001", Session{Query: "Hello LangGraph!"})
    if err != nil {
        panic(err)
    }

    println(final.Answer) // Output: Processed: Hello LangGraph!
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

- [Architecture Overview](./CLAUDE.md)
- [Constitution](`./.specify/memory/constitution.md`) - Development principles
- [Specification](./specs/SPEC.md) - Technical specification
- [Contributing](./CONTRIBUTING.md) - Development workflow

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

🚧 **Early Development** - This project is actively being developed following specification-driven development practices. The core framework is being implemented incrementally based on prioritized user stories.

Current focus: **User Story 1 (P1)** - Stateful workflow with checkpointing

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](./CONTRIBUTING.md) for development guidelines including our Test-Driven Development workflow.

## Acknowledgments

Inspired by [LangGraph](https://github.com/langchain-ai/langgraph), redesigned for Go's type system and concurrency primitives.
