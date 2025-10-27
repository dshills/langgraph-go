# LangGraph-Go

A Go-native orchestration framework for building **stateful, graph-based LLM and tool workflows**.

> **⚠️ Alpha Software Warning**
>
> LangGraph-Go is currently in **alpha** stage. The API is not yet stable and may change significantly between releases. While the core functionality is complete and tested, we recommend:
> - **Not using in production** without thorough testing
> - **Pinning to specific versions** in your go.mod
> - **Expecting breaking changes** until v1.0.0
> - **Reporting issues** to help us improve stability
>
> We welcome early adopters and contributors! Please see [CONTRIBUTING.md](./CONTRIBUTING.md) for how to get involved.

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
# Install latest version (may include breaking changes)
go get github.com/dshills/langgraph-go

# Recommended: Pin to a specific version
go get github.com/dshills/langgraph-go@v0.1.0
```

**Note:** Since this is alpha software, we recommend pinning to a specific version in your `go.mod` to avoid unexpected breaking changes.

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
  ├── edge.go        # Edge and predicate types
  ├── state.go       # State management
  ├── store/         # Persistence layer
  │   ├── store.go   # Store interface
  │   ├── memory.go  # In-memory store (for testing)
  │   ├── mysql.go   # MySQL/Aurora store
  │   └── mysql/     # MySQL implementation details
  ├── emit/          # Event system
  │   ├── emitter.go # Emitter interface
  │   ├── event.go   # Event types
  │   ├── log.go     # Logging emitter
  │   ├── buffered.go# Buffered emitter
  │   └── null.go    # Null emitter
  ├── model/         # LLM integrations
  │   ├── chat.go    # ChatModel interface
  │   ├── openai/    # OpenAI adapter
  │   ├── anthropic/ # Anthropic Claude adapter
  │   ├── google/    # Google Gemini adapter
  │   └── mock.go    # Mock for testing
  └── tool/          # Tool abstractions
      ├── tool.go    # Tool interface
      ├── http.go    # HTTP tool implementation
      └── mock.go    # Mock tool for testing
```

## Features

- ✅ **Stateful Execution** - Checkpoint and resume workflows
- ✅ **Conditional Routing** - Dynamic control flow based on state
- ✅ **Parallel Execution** - Fan-out to concurrent nodes
- ✅ **LLM Integration** - OpenAI, Anthropic, Google Gemini
- ✅ **Tool Support** - HTTP tools and custom tool integration
- ✅ **Persistence** - MySQL/Aurora store for production use
- ✅ **Event Tracing** - Comprehensive observability with multiple emitters
- ✅ **Type Safety** - Go generics for compile-time safety
- ✅ **Production Ready** - Error handling, retries, timeouts

## Examples

See the [`examples/`](./examples) directory for complete, runnable examples:

- **`chatbot/`** - Customer support chatbot with intent detection
- **`checkpoint/`** - Checkpoint and resume workflows
- **`routing/`** - Conditional routing based on state
- **`parallel/`** - Parallel execution with fan-out/fan-in
- **`llm/`** - Multi-provider LLM integration (OpenAI, Anthropic, Google)
- **`tools/`** - Tool calling and integration
- **`data-pipeline/`** - Data processing pipeline
- **`research-pipeline/`** - Multi-stage research workflow
- **`interactive-workflow/`** - Interactive user input workflow
- **`tracing/`** - Event tracing and observability
- **`benchmarks/`** - Performance benchmarking

All examples can be built with:
```bash
make examples
./build/<example-name>
```

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

The project includes a comprehensive Makefile for common development tasks:

```bash
# Build the library
make build

# Build all examples
make examples

# Run tests
make test

# Run tests with verbose output
make test-verbose

# Run tests with coverage report
make test-cover

# Run benchmarks
make bench

# Format code
make fmt

# Run go vet
make vet

# Run linter (if golangci-lint is available)
make lint

# Clean build artifacts
make clean

# Install dependencies
make install

# Build everything
make all

# Show all available targets
make help
```

Or use standard Go commands:

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

MIT License

## Project Status

**Current Version:** v0.1.0-alpha

✅ **Core Framework Complete** - All 5 user stories have been implemented and tested:

- ✅ **US1**: Stateful workflow with checkpointing
- ✅ **US2**: Conditional routing and dynamic control flow
- ✅ **US3**: Parallel execution with fan-out/fan-in
- ✅ **US4**: Multi-provider LLM integration
- ✅ **US5**: Comprehensive event tracing and observability

**Current Phase**: Alpha release - API stabilization and community feedback

**Roadmap to v1.0.0:**
- Gather feedback from early adopters
- Stabilize public API based on real-world usage
- Address any critical bugs or issues
- Complete API documentation
- Add more examples and use cases

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](./CONTRIBUTING.md) for development guidelines including our Test-Driven Development workflow.

Please note that this project is released with a [Code of Conduct](./CODE_OF_CONDUCT.md). By participating in this project you agree to abide by its terms.

## Security

For security concerns, please see our [Security Policy](./SECURITY.md).

## Acknowledgments

This project is inspired by [LangGraph](https://github.com/langchain-ai/langgraph) by LangChain AI, a powerful framework for building stateful, multi-actor applications with LLMs. LangGraph-Go brings these concepts to the Go ecosystem, redesigned from the ground up to leverage Go's type system, generics, and concurrency primitives.

**Key differences from the original LangGraph:**
- **Type-safe** - Uses Go generics for compile-time type safety
- **Go-native** - Idiomatic Go design patterns and error handling
- **Concurrency-first** - Built on goroutines and channels for efficient parallelism
- **Minimal dependencies** - Pure Go core with optional external adapters

We are grateful to the LangChain AI team for pioneering the graph-based workflow approach for LLM applications. Check out the original Python implementation at https://github.com/langchain-ai/langgraph.
