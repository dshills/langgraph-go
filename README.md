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
    "time"

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

    // Build the workflow with functional options (recommended)
    st := store.NewMemStore[Session]()
    emitter := emit.NewLogEmitter(os.Stdout, false)

    engine := graph.New(
        reduce, st, emitter,
        graph.WithMaxConcurrent(8),
        graph.WithDefaultNodeTimeout(10*time.Second),
    )
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
go get github.com/dshills/langgraph-go@v0.3.0  # Latest: Production hardening + comprehensive docs
go get github.com/dshills/langgraph-go@v0.2.0  # Concurrent execution + deterministic replay
go get github.com/dshills/langgraph-go@v0.1.0  # Initial release
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

### Concurrent Execution

**New in v0.2.0**: Execute independent nodes in parallel with deterministic results:

```go
// Enable concurrent execution with functional options (recommended in v0.3.0)
engine := graph.New(
    reducer, store, emitter,
    graph.WithMaxConcurrent(10),   // Execute up to 10 nodes in parallel
    graph.WithQueueDepth(1000),     // Frontier queue capacity
)

// Or use Options struct (backward compatible)
opts := graph.Options{
    MaxConcurrentNodes: 10,
    QueueDepth:         1000,
}
engine := graph.New(reducer, store, emitter, opts)

// Graph with parallel branches executes concurrently
// Fan-out: one node spawns multiple parallel branches
engine.Add("start", startNode)
engine.Add("branchA", branchANode) // Executes in parallel
engine.Add("branchB", branchBNode) // Executes in parallel
engine.Add("branchC", branchCNode) // Executes in parallel
engine.Add("merge", mergeNode)     // Results merge deterministically

// Results are deterministic regardless of completion order
final, err := engine.Run(ctx, runID, initialState)
```

**Key Features:**

- ⚡ **Performance**: 2-5x speedup for workflows with independent nodes
- 🎯 **Deterministic**: Same results regardless of execution order
- 🔄 **Replay**: Record and replay executions exactly for debugging
- 🛡️ **Backpressure**: Automatic queue management prevents resource exhaustion
- 📊 **Observability**: Built-in metrics for queue depth and active workers

**Deterministic Replay** for debugging production issues:

```go
// Original execution automatically recorded
final, err := engine.Run(ctx, "prod-run-123", initialState)

// Load checkpoint from production
checkpoint, _ := store.LoadCheckpointV2(ctx, "prod-run-123", "")

// Replay execution exactly (no external API calls)
opts := graph.Options{ReplayMode: true}
replayResult, _ := engine.ReplayRun(ctx, "debug-replay", checkpoint)

// Identical results, full debugging context
```

See [Concurrency Guide](./docs/concurrency.md) and [Replay Guide](./docs/replay.md) for details.

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
  │   ├── sqlite.go  # SQLite store (dev/small prod)
  │   ├── mysql.go   # MySQL/Aurora store (production)
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

### Core Features
- ✅ **Stateful Execution** - Checkpoint and resume workflows
- ✅ **Conditional Routing** - Dynamic control flow based on state
- ✅ **Parallel Execution** - Fan-out to concurrent nodes
- ✅ **LLM Integration** - OpenAI, Anthropic, Google Gemini
- ✅ **Tool Support** - HTTP tools and custom tool integration
- ✅ **Type Safety** - Go generics for compile-time safety

### Performance & Reliability (v0.2.0)
- ✅ **Concurrent Execution** - Worker pool with deterministic ordering
- ✅ **Deterministic Replay** - Record and replay executions exactly
- ✅ **Backpressure Control** - Automatic queue management
- ✅ **Retry Policies** - Exponential backoff with jitter

### Production Hardening (v0.3.0)
- ✅ **SQLite Store** - Zero-config persistence for development
- ✅ **Prometheus Metrics** - Production monitoring (6 metrics)
- ✅ **Cost Tracking** - LLM token and cost accounting
- ✅ **Functional Options** - Ergonomic API with backward compatibility
- ✅ **Typed Errors** - errors.Is() compatible error handling
- ✅ **Formal Guarantees** - Documented determinism and exactly-once semantics
- ✅ **Contract Tests** - CI-validated correctness proofs

## Examples

See the [`examples/`](./examples) directory for complete, runnable examples:

- **`sqlite_quickstart/`** - **⭐ Start here!** Zero-config persistence
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

**📚 [Complete Documentation Index](./docs/README.md)** - Organized guide to all documentation

### Getting Started

- [Quick Start Guide](./docs/quickstart.md) - Get up and running in 5 minutes
- [Architecture Overview](./docs/architecture.md) - System design and components
- [Error Handling](./docs/error-handling.md) - Typed errors and recovery patterns

### Core Concepts (v0.2.0)

- [Concurrency Model](./docs/concurrency.md) - Worker pool, ordering, backpressure, determinism
- [Deterministic Replay](./docs/replay.md) - Record and replay executions with guardrails
- [Store Guarantees](./docs/store-guarantees.md) - Exactly-once semantics and atomic commits
- [Determinism Guarantees](./docs/determinism-guarantees.md) - Formal contracts and mathematical proofs

### Production Features (v0.3.0)

- [Observability](./docs/observability.md) - Prometheus metrics, OpenTelemetry, cost tracking
- [Testing Contracts](./docs/testing-contracts.md) - Contract tests and CI integration
- [Conflict Policies](./docs/conflict-policies.md) - State merge strategies
- [Human-in-the-Loop](./docs/human-in-the-loop.md) - Approval workflows and pause/resume
- [Streaming Support](./docs/streaming.md) - Current status and workarounds
- [Why Go?](./docs/why-go.md) - Go vs Python LangGraph comparison

### Migration Guides

- [v0.1 → v0.2 Migration](./docs/migration-v0.2.md) - Upgrade from v0.1.x
- [v0.2 → v0.3 Migration](./CHANGELOG.md#030---2025-10-28) - See CHANGELOG for v0.3.0 changes

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

## Testing

LangGraph-Go has comprehensive test coverage including contract tests that validate production guarantees.

### Run All Tests

```bash
# Run all tests
go test ./...

# Run with race detector (recommended)
go test -race ./...

# Run with coverage
go test -cover -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Contract Tests

Contract tests validate critical system guarantees:

```bash
# Determinism contracts
go test -v -race -run TestReplayMismatchDetection ./graph/
go test -v -race -run TestMergeOrderingWithRandomDelays ./graph/

# Exactly-once semantics
go test -v -race -run TestConcurrentStateUpdates ./graph/
go test -v -race -run TestIdempotencyAcrossStores ./graph/store/

# Backpressure
go test -v -race -run TestBackpressureBlocking ./graph/

# RNG determinism
go test -v -race -run TestRNGDeterminism ./graph/
```

### MySQL Integration Tests

Some tests require MySQL for persistence validation:

```bash
# Start MySQL (Docker)
docker run -d -p 3306:3306 \
  -e MYSQL_ROOT_PASSWORD=testpassword \
  -e MYSQL_DATABASE=langgraph_test \
  mysql:8.0

# Run with MySQL
export TEST_MYSQL_DSN="root:testpassword@tcp(127.0.0.1:3306)/langgraph_test?parseTime=true"
go test -v -race ./graph/...
```

### What Contract Tests Prove

- **Determinism**: Same inputs produce same outputs across replays
- **Exactly-Once**: State updates happen exactly once, never duplicated or lost
- **Backpressure**: System handles overload gracefully without crashes
- **RNG Determinism**: Random workflows are replayable for debugging
- **Cross-Store Consistency**: All Store implementations uphold the same contracts

See [docs/testing-contracts.md](./docs/testing-contracts.md) for detailed test documentation.

### CI/CD

All tests run automatically on push/PR via GitHub Actions:
- Platforms: Linux, macOS, Windows
- Go versions: 1.21, 1.22, 1.23
- Race detector enabled
- Coverage uploaded to Codecov

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
