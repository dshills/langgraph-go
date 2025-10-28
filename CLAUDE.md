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

## Recent Changes
- 002-concurrency-spec: Added Go 1.21+ (requires generics support) + Go standard library only (core framework), optional adapters for OpenTelemetry SDK, MySQL driver
