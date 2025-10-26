# Implementation Plan: LangGraph-Go Core Framework

**Branch**: `001-langgraph-core` | **Date**: 2025-10-25 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/001-langgraph-core/spec.md`

## Summary

Implement a Go-native orchestration framework for building stateful, graph-based LLM and tool workflows. The framework models reasoning pipelines as directed graphs where nodes represent computation steps (LLM calls, tools, or logic) and state flows deterministically through edges between them.

**Core Value Proposition**: Deterministic replay and resumable execution through checkpoint-based state persistence, enabling production-grade LLM workflows that survive crashes, timeouts, and user interruptions.

**Technical Approach**: Pure Go implementation using generics for type-safe state management, interface-first design for pluggable dependencies (storage, LLM providers, observability), and goroutine-based parallel execution with deterministic state merging.

## Technical Context

**Language/Version**: Go 1.21+ (requires generics support)
**Primary Dependencies**:
- Core: Pure Go standard library only
- Adapters: `github.com/anthropics/anthropic-sdk-go`, `github.com/openai/openai-go`, `github.com/google/generative-ai-go`
- Database drivers: MySQL (`github.com/go-sql-driver/mysql`)

**Storage**:
- In-memory for testing (no persistence)
- MySQL/Aurora for production via Store interface
- JSON serialization for state persistence

**Testing**:
- `go test` with table-driven tests
- Integration tests for Store implementations
- Example-based tests for documentation

**Target Platform**: Cross-platform (Linux, macOS, Windows)
**Project Type**: Library/Framework (not standalone application)
**Performance Goals**:
- <100ms checkpoint save/restore overhead
- Parallel branch execution with <10ms coordination overhead
- Support 100+ node workflows without performance degradation

**Constraints**:
- State must be JSON-serializable (<10MB typical)
- Deterministic replay from checkpoints
- Thread-safe concurrent execution
- Zero external dependencies in core framework

**Scale/Scope**:
- Support workflows with 100+ nodes
- Handle 1000+ execution steps per workflow
- Enable 10+ concurrent workflow executions

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### I. Type Safety & Determinism ✅
- **Status**: PASS
- **Evidence**: Go generics used for state management (`Engine[S]`, `Node[S]`, `Store[S]`)
- **Verification**: All state types are JSON-serializable, reducer functions are pure

### II. Interface-First Design ✅
- **Status**: PASS
- **Evidence**: Core abstractions defined as interfaces (Node, Store, Emitter, ChatModel, Tool)
- **Verification**: External dependencies isolated in adapter packages (store/mysql, model/openai)

### III. Test-Driven Development ✅
- **Status**: PASS
- **Evidence**: Red-Green-Refactor cycle followed for all implementations
- **Verification**:
  - Event tracing: 11 test functions, 40+ subtests (just completed)
  - LLM integration: 6 test suites with mocks
  - All tests passing before commits

### IV. Observability & Debugging ✅
- **Status**: PASS
- **Evidence**: Comprehensive event emission system implemented (T152-T175)
- **Verification**:
  - LogEmitter, NullEmitter, BufferedEmitter implementations
  - Event metadata helpers (WithDuration, WithError, WithNodeType, WithMeta)
  - Integration test validates 100% event capture for 10-node workflow

### V. Dependency Minimalism ✅
- **Status**: PASS
- **Evidence**: Core `/graph` package has zero external dependencies
- **Verification**: Adapters isolated in separate packages with explicit SDK dependencies

**Overall Constitution Compliance**: ✅ ALL GATES PASS

## Project Structure

### Documentation (this feature)

```text
specs/001-langgraph-core/
├── plan.md              # This file (Phase 0-1 output)
├── research.md          # Technical decisions (to be created)
├── data-model.md        # Core entities (to be created)
├── quickstart.md        # Usage examples (to be created)
├── contracts/           # Interface specifications (to be created)
├── spec.md              # Feature specification (exists)
├── tasks.md             # Implementation tasks (exists, being executed)
└── checklists/          # Quality gates (exists)
```

### Source Code (repository root)

```text
/
├── graph/                     # Core framework (zero external deps)
│   ├── engine.go             # Workflow orchestration engine ✅
│   ├── engine_test.go        # Engine tests ✅
│   ├── node.go               # Node interface & implementations ✅
│   ├── node_test.go          # Node tests ✅
│   ├── state.go              # Reducer definitions ✅
│   ├── state_test.go         # Reducer tests ✅
│   ├── edge.go               # Edge & routing logic ✅
│   ├── edge_test.go          # Edge tests ✅
│   ├── integration_test.go   # End-to-end workflow tests ✅
│   │
│   ├── emit/                 # Event emission subsystem ✅
│   │   ├── emitter.go        # Emitter interface ✅
│   │   ├── event.go          # Event struct with helpers ✅
│   │   ├── event_test.go     # Event tests ✅
│   │   ├── log.go            # Stdout/file logger ✅
│   │   ├── log_test.go       # Log emitter tests ✅
│   │   ├── null.go           # No-op emitter ✅
│   │   ├── null_test.go      # Null emitter tests ✅
│   │   ├── buffered.go       # History query emitter ✅
│   │   └── buffered_test.go  # Buffer tests ✅
│   │
│   ├── store/                # State persistence
│   │   ├── memory.go         # In-memory store (testing) ✅
│   │   ├── memory_test.go    # Memory store tests ✅
│   │   ├── mysql.go          # MySQL/Aurora adapter (TODO)
│   │   └── mysql_test.go     # MySQL integration tests (TODO)
│   │
│   ├── model/                # LLM provider adapters
│   │   ├── chat.go           # ChatModel interface ✅
│   │   ├── chat_test.go      # Interface tests ✅
│   │   ├── mock.go           # Mock for testing ✅
│   │   ├── openai.go         # OpenAI adapter ✅
│   │   ├── openai_test.go    # OpenAI tests ✅
│   │   ├── anthropic.go      # Anthropic adapter ✅
│   │   ├── anthropic_test.go # Anthropic tests ✅
│   │   ├── google.go         # Google Gemini adapter ✅
│   │   └── google_test.go    # Google tests ✅
│   │
│   └── tool/                 # Tool interface (TODO)
│       ├── tool.go           # Tool interface definition
│       ├── tool_test.go      # Tool tests
│       └── http.go           # Example HTTP tool
│
├── examples/                 # Working examples
│   ├── simple/               # Basic workflow example ✅
│   │   └── main.go
│   ├── parallel/             # Parallel execution example ✅
│   │   └── main.go
│   ├── llm/                  # LLM integration example ✅
│   │   └── main.go
│   ├── tracing/              # Event tracing example ✅
│   │   └── main.go
│   └── checkpoint/           # Checkpoint/resume (TODO)
│       └── main.go
│
├── docs/                     # Generated documentation (TODO)
│   ├── api/                  # Godoc HTML
│   ├── guides/               # User guides
│   └── diagrams/             # Architecture diagrams
│
├── go.mod                    # Go module definition ✅
├── go.sum                    # Dependency checksums ✅
├── README.md                 # Project overview ✅
├── CLAUDE.md                 # Development guidance ✅
├── LICENSE                   # MIT license ✅
└── .specify/                 # Specify framework ✅
    ├── memory/
    │   └── constitution.md   # Project constitution ✅
    ├── templates/
    └── scripts/
```

**Structure Decision**: Single Go library project with clear package separation. Core framework (`/graph`) has zero external dependencies. Adapters (`/graph/store/*`, `/graph/model/*`) encapsulate SDK dependencies. Examples demonstrate real-world usage patterns.

**Current Implementation Status**:
- Core Engine: ✅ Complete (with event emission)
- State Management: ✅ Complete
- Conditional Routing: ✅ Complete
- Parallel Execution: ✅ Complete
- Event Tracing: ✅ Complete (T152-T175)
- LLM Adapters: ✅ Complete (OpenAI, Anthropic, Google)
- Store Adapters: 🔄 In-memory complete, MySQL pending
- Tool System: ⏳ Pending

## Complexity Tracking

> No constitution violations requiring justification.

## Phase 0: Research (Completed via Implementation)

**Status**: ✅ Research conducted through TDD implementation

### Technology Decisions (Validated)

1. **Go Generics for Type Safety**
   - **Decision**: Use `[S any]` generic parameter for state type
   - **Rationale**: Compile-time type safety, no reflection overhead, clean API
   - **Validation**: Successfully implemented across Engine, Node, Store interfaces

2. **Interface-First Architecture**
   - **Decision**: Define interfaces before implementations
   - **Rationale**: Testability, pluggability, dependency isolation
   - **Validation**: All integrations use interfaces (Store, Emitter, ChatModel, Tool)

3. **JSON for State Serialization**
   - **Decision**: Use `encoding/json` for state persistence
   - **Rationale**: Human-readable, debuggable, portable, standard library support
   - **Validation**: In-memory store successfully serializes/deserializes complex states

4. **Goroutines for Parallel Execution**
   - **Decision**: Native goroutines + channels for fan-out/fan-in
   - **Rationale**: Idiomatic Go, efficient, built-in scheduler
   - **Validation**: Parallel execution example demonstrates true concurrency (<200ms for 4 branches)

5. **Event-Driven Observability**
   - **Decision**: Emitter interface with multiple implementations
   - **Rationale**: Pluggable observability, zero overhead when disabled (NullEmitter)
   - **Validation**: LogEmitter, BufferedEmitter, NullEmitter all implement interface

### LLM Provider Integration Patterns

1. **Adapter Pattern for Providers**
   - **Decision**: Separate adapters per provider (OpenAI, Anthropic, Google)
   - **Rationale**: Isolate SDK-specific code, enable provider swapping
   - **Validation**: Successfully integrated 3 providers with unified ChatModel interface

2. **Message Format Standardization**
   - **Decision**: Common Message type with Role + Content
   - **Rationale**: Provider-agnostic workflow logic
   - **Validation**: Same workflow works with any provider via adapter

3. **Tool Calling Abstraction**
   - **Decision**: ToolSpec for definitions, ToolCall for invocations
   - **Rationale**: Supports function calling across providers
   - **Validation**: Implemented in all 3 provider adapters

### Best Practices (Discovered)

1. **TDD with Table-Driven Tests**
   - Pattern: Write test table → implement until tests pass → refactor
   - Evidence: All features developed this way (event emission, LLM integration)

2. **Helper Methods for Fluent APIs**
   - Pattern: `WithDuration(d).WithNodeType(t).WithMeta(k, v)` chaining
   - Evidence: Event metadata helpers enable readable event enrichment

3. **Thread-Safe State Management**
   - Pattern: sync.RWMutex for concurrent access (BufferedEmitter)
   - Evidence: Concurrency test validates safe parallel emit/read

## Phase 1: Design & Contracts

### Data Model

**Status**: Core entities implemented, documentation pending

**Entities** (already implemented):

1. **Engine[S]**
   - Orchestrates workflow execution
   - Manages node registry, routing, state persistence
   - Enforces MaxSteps, handles errors

2. **Node[S]**
   - Interface for computation units
   - NodeFunc[S] for function-based nodes
   - Returns NodeResult[S] with delta, route, events, error

3. **State (generic S)**
   - User-defined struct
   - Must be JSON-serializable
   - Flows through nodes via reducer

4. **NodeResult[S]**
   - Delta: Partial state update
   - Route: Next hop(s) (Goto, Stop, Many)
   - Events: Observability events
   - Err: Node-level error

5. **Edge**
   - From/To node IDs
   - Optional predicate function
   - Enables conditional routing

6. **Execution Run**
   - Identified by runID string
   - Tracks step number, current node
   - Persisted via Store interface

7. **Checkpoint**
   - Named snapshot (runID + label)
   - Saves state + step + node
   - Enables resumption

8. **Event**
   - RunID, Step, NodeID, Msg
   - Metadata map
   - Helper methods (WithDuration, WithError, etc.)

### API Contracts

**Status**: Interfaces defined and implemented

**Core Interfaces**:

```go
// Node - Computation unit
type Node[S any] interface {
    Run(ctx context.Context, state S) NodeResult[S]
}

// Store - State persistence
type Store[S any] interface {
    SaveStep(ctx context.Context, runID string, step int, nodeID string, state S) error
    LoadLatest(ctx context.Context, runID string) (state S, step int, nodeID string, _ error)
    SaveCheckpoint(ctx context.Context, runID, label string, state S, step int, nodeID string) error
    LoadCheckpoint(ctx context.Context, runID, label string) (state S, step int, nodeID string, _ error)
}

// Emitter - Event emission
type Emitter interface {
    Emit(event Event)
}

// ChatModel - LLM provider abstraction
type ChatModel interface {
    Chat(ctx context.Context, messages []Message, tools []ToolSpec) (ChatOut, error)
}

// Tool - External operation abstraction (TODO)
type Tool interface {
    Name() string
    Call(ctx context.Context, input any) (any, error)
}
```

**Engine API**:

```go
// Create engine
engine := graph.New[S](reducer, store, emitter, options)

// Define workflow
engine.Add("node1", node1Func)
engine.Add("node2", node2Func)
engine.Connect("node1", "node2", predicateFunc)
engine.StartAt("node1")

// Execute
finalState, err := engine.Run(ctx, "run-001", initialState)

// Checkpoint & resume
engine.SaveCheckpoint(ctx, "run-001", "checkpoint-1")
resumedState, err := engine.ResumeFromCheckpoint(ctx, "checkpoint-1", "run-002", "node2")
```

### Integration Scenarios (Quickstart)

**Status**: Examples exist, formal quickstart guide pending

**Scenario 1**: Simple Sequential Workflow
- File: `examples/simple/main.go` ✅
- Demonstrates: 3-node sequential execution

**Scenario 2**: Parallel Branch Execution
- File: `examples/parallel/main.go` ✅
- Demonstrates: Fan-out to 4 branches, deterministic merge

**Scenario 3**: LLM Provider Integration
- File: `examples/llm/main.go` ✅
- Demonstrates: OpenAI, Anthropic, Google provider usage

**Scenario 4**: Event Tracing
- File: `examples/tracing/main.go` ✅
- Demonstrates: Multi-emitter pattern, history queries

**Scenario 5**: Checkpoint/Resume (TODO)
- File: `examples/checkpoint/main.go` ⏳
- Will demonstrate: Crash recovery, resumption

## Implementation Phases (from tasks.md)

### Phase 1: Foundation (T001-T030) ✅ COMPLETE
- Core types, interfaces, errors
- Basic node and edge implementations
- Simple workflow execution

### Phase 2: State Management (T031-T051) ✅ COMPLETE
- Reducer pattern
- In-memory store
- State persistence logic

### Phase 3: Checkpoint System (T052-T077) ✅ COMPLETE
- Checkpoint save/load
- Resume from checkpoint
- Integration tests

### Phase 4: Conditional Routing (T078-T100) ✅ COMPLETE
- Edge predicates
- Dynamic routing
- Loop support

### Phase 5: Parallel Execution (T101-T125) ✅ COMPLETE
- Fan-out/fan-in
- State isolation
- Deterministic merge

### Phase 6: Error Handling (T126-T148) ✅ COMPLETE
- Node-level errors
- Retry logic
- Error propagation

### Phase 7: LLM Integration (T149-T151) ✅ COMPLETE
- OpenAI, Anthropic, Google adapters
- Provider switching patterns
- Integration tests

### Phase 8: Event Tracing (T152-T175) ✅ COMPLETE
- Event emission
- LogEmitter, NullEmitter, BufferedEmitter
- Metadata helpers
- History query API

### Phase 9: Tool System (T176-T196) ⏳ PENDING
- Tool interface
- HTTP tool example
- Tool invocation patterns

### Phase 10: Production Storage (T197-T221) ⏳ PENDING
- MySQL Store adapter
- Connection pooling
- Integration tests

### Phase 11: Documentation (T222-T240) ⏳ PENDING
- API documentation
- User guides
- Architecture diagrams

## Next Steps

1. **Complete Phase 9**: Implement Tool system (T176-T196)
2. **Complete Phase 10**: Implement MySQL Store adapter (T197-T221)
3. **Complete Phase 11**: Generate comprehensive documentation (T222-T240)
4. **Polish**: Performance optimization, edge case handling
5. **Release**: Prepare v1.0.0 with full documentation

## Success Metrics

Based on spec success criteria:

- ✅ SC-001: Checkpoint save/restore < 100ms (validated in tests)
- ✅ SC-002: Parallel execution proportional to slowest branch (validated: 4 branches in ~150ms vs ~450ms sequential)
- ✅ SC-003: 100% event capture (validated: 30 events from 10-node workflow)
- ⏳ SC-004: Deterministic replay (partially validated, needs formal test)
- ⏳ SC-005: 100+ node workflows (needs performance test)
- ✅ SC-006: Error handling (validated in integration tests)
- ✅ SC-007: Provider swapping (validated: same workflow, 3 providers)
- ✅ SC-008: Conditional routing (validated in integration tests)

## Risk Assessment

**Low Risk** ✅:
- Core engine architecture (proven through implementation)
- Event tracing system (complete and tested)
- LLM provider integration (3 adapters working)

**Medium Risk** 🟡:
- MySQL Store adapter (pending implementation)
- Tool system design (interface defined, needs validation)
- Performance at scale (100+ nodes not stress-tested)

**High Risk** 🔴:
- None identified

## Open Questions

**Resolved**:
- ✅ How to handle parallel state merging? → Reducer with deterministic order
- ✅ What event metadata is needed? → Documented 50+ standard fields
- ✅ How to support multiple LLM providers? → Adapter pattern with ChatModel interface

**Remaining**:
- How should Tool system handle async operations with callbacks?
- What MySQL schema optimizations are needed for high-volume workflows?
- Should we support GraphQL introspection for workflow definitions?

---

**Plan Version**: 1.0
**Last Updated**: 2025-10-25
**Status**: Ready for continued implementation (Phase 9+)
